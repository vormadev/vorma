package tooling

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/lru"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

// matchCacheMaxSize limits the match cache to prevent unbounded memory growth
const matchCacheMaxSize = 10000

// Ignore patterns - these are glob patterns, not path segments
const (
	globGit         = "**/.git"
	globNodeModules = "**/node_modules"
)

// Watcher manages file watching for the dev server
type Watcher struct {
	cfg     *wave.ParsedConfig
	log     *slog.Logger
	fsWatch *fsnotify.Watcher

	watchedDirs sync.Map

	// LRU cache for pattern matching results
	matchCache *lru.Cache[string, bool]

	// Patterns stored as absolute paths with forward slashes
	ignoredDirs    []string
	ignoredFiles   []string
	defaultWatched []wave.WatchedFile

	// Absolute watch root for reference
	absWatchRoot     string
	absPublicStatic  string
	absPrivateStatic string
}

// NewWatcher creates a new file watcher
func NewWatcher(cfg *wave.ParsedConfig, log *slog.Logger) (*Watcher, error) {
	if log == nil {
		log = colorlog.New("wave")
	}

	fsWatch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	absWatchRoot := pathnorm.AbsoluteSlash(cfg.WatchRoot())

	absPublicStatic := ""
	absPrivateStatic := ""
	if cfg.UsingBrowser() {
		absPublicStatic = pathnorm.AbsoluteSlash(cfg.Core.StaticAssetDirs.Public)
		absPrivateStatic = pathnorm.AbsoluteSlash(cfg.Core.StaticAssetDirs.Private)
	}

	w := &Watcher{
		cfg:              cfg,
		log:              log,
		fsWatch:          fsWatch,
		absWatchRoot:     absWatchRoot,
		absPublicStatic:  absPublicStatic,
		absPrivateStatic: absPrivateStatic,
		matchCache:       lru.NewCache[string, bool](matchCacheMaxSize),
	}

	w.setupPatterns()
	return w, nil
}

// norm converts a path to absolute with forward slashes for consistent matching
func (w *Watcher) norm(p string) string {
	return pathnorm.AbsoluteSlash(p)
}

func (w *Watcher) normalizePathOrPatternFromWatchRoot(pathOrPattern string) string {
	if filepath.IsAbs(pathOrPattern) {
		return w.norm(pathOrPattern)
	}

	return w.norm(filepath.Join(w.cfg.WatchRoot(), pathOrPattern))
}

func (w *Watcher) setupPatterns() {
	w.ignoredFiles = []string{
		w.norm(w.cfg.Dist.Binary()),
	}

	// Add dist static as absolute path
	w.ignoredDirs = append(w.ignoredDirs, w.norm(w.cfg.Dist.Static()))
	w.ignoredDirs = append(w.ignoredDirs, w.norm(w.cfg.Dist.Static())+"/**")

	// Only add static asset patterns if not in server-only mode
	if w.cfg.UsingBrowser() {
		publicStatic := filepath.Clean(w.cfg.Core.StaticAssetDirs.Public)
		privateStatic := filepath.Clean(w.cfg.Core.StaticAssetDirs.Private)

		nohashDir := w.norm(filepath.Join(publicStatic, wave.NohashDirname))
		w.ignoredDirs = append(w.ignoredDirs, nohashDir)
		w.ignoredDirs = append(w.ignoredDirs, nohashDir+"/**")

		prehashedDir := w.norm(filepath.Join(publicStatic, wave.PrehashedDirname))
		w.ignoredDirs = append(w.ignoredDirs, prehashedDir)
		w.ignoredDirs = append(w.ignoredDirs, prehashedDir+"/**")

		// Public static files: Wave handles processing and writes filemap.ts directly.
		// No explicit dev build hook command is needed - Vite HMR picks up the TS file change.
		w.defaultWatched = []wave.WatchedFile{
			{
				Pattern: w.norm(publicStatic) + "/**/*",
			},
		}

		// Private static files: Wave handles processing and triggers browser reload.
		w.defaultWatched = append(w.defaultWatched, wave.WatchedFile{
			Pattern: w.norm(privateStatic) + "/**/*",
		})
	}

	// Add framework-injected watch patterns
	w.addFrameworkWatchPatterns()

	// Add framework-injected ignored patterns
	for _, p := range w.cfg.FrameworkIgnoredPatterns {
		normalizedPattern := w.normalizePathOrPatternFromWatchRoot(p)

		// Heuristic: if it ends in /** or looks like a dir, treat as ignored dir
		if strings.HasSuffix(p, "/**") {
			w.ignoredDirs = append(w.ignoredDirs, normalizedPattern)
		} else {
			// It might be a file or a pattern
			w.ignoredFiles = append(w.ignoredFiles, normalizedPattern)
		}
	}

	// For ** patterns, we need to anchor them to watch root
	w.ignoredDirs = append(w.ignoredDirs, w.absWatchRoot+"/"+globGit)
	w.ignoredDirs = append(w.ignoredDirs, w.absWatchRoot+"/"+globNodeModules)

	if w.cfg.Watch != nil {
		for _, p := range w.cfg.Watch.Exclude.Dirs {
			normalizedDirectoryPattern := w.normalizePathOrPatternFromWatchRoot(p)
			w.ignoredDirs = append(w.ignoredDirs, normalizedDirectoryPattern)
			w.ignoredDirs = append(w.ignoredDirs, normalizedDirectoryPattern+"/**")
		}
		for _, p := range w.cfg.Watch.Exclude.Files {
			w.ignoredFiles = append(w.ignoredFiles, w.normalizePathOrPatternFromWatchRoot(p))
		}
	}

	w.joinPatternsWithRoot()
	w.preSortHooks()
}

// addFrameworkWatchPatterns adds patterns injected by frameworks (e.g., Vorma)
func (w *Watcher) addFrameworkWatchPatterns() {
	for _, wf := range w.cfg.FrameworkWatchPatterns {
		// Create a copy with normalized pattern
		normalizedWF := wf
		normalizedWF.Pattern = w.normalizePathOrPatternFromWatchRoot(wf.Pattern)

		// Normalize exclude patterns in hooks
		for i, hook := range normalizedWF.OnChangeHooks {
			for j, excl := range hook.Exclude {
				normalizedWF.OnChangeHooks[i].Exclude[j] = w.normalizePathOrPatternFromWatchRoot(excl)
			}
		}

		w.defaultWatched = append(w.defaultWatched, normalizedWF)
	}
}

func (w *Watcher) joinPatternsWithRoot() {
	if w.cfg.Watch == nil {
		return
	}

	for i, wf := range w.cfg.Watch.Include {
		w.cfg.Watch.Include[i].Pattern = w.normalizePathOrPatternFromWatchRoot(wf.Pattern)

		for j, hook := range wf.OnChangeHooks {
			for k, excl := range hook.Exclude {
				w.cfg.Watch.Include[i].OnChangeHooks[j].Exclude[k] = w.normalizePathOrPatternFromWatchRoot(excl)
			}
		}
	}
}

// preSortHooks pre-sorts hooks for all watched files to avoid repeated sorting during event handling
func (w *Watcher) preSortHooks() {
	if w.cfg.Watch != nil {
		for i := range w.cfg.Watch.Include {
			w.cfg.Watch.Include[i].Sort()
		}
	}

	for i := range w.defaultWatched {
		w.defaultWatched[i].Sort()
	}
}

func (w *Watcher) Events() <-chan fsnotify.Event {
	return w.fsWatch.Events
}

func (w *Watcher) Errors() <-chan error {
	return w.fsWatch.Errors
}

func (w *Watcher) Close() error {
	return w.fsWatch.Close()
}

// AddDir adds a directory and its subdirectories to the watcher
func (w *Watcher) AddDir(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}

		if w.IsIgnoredDir(path) {
			return filepath.SkipDir
		}

		// Use absolute path as key to avoid duplicates
		absPath := w.norm(path)
		if _, exists := w.watchedDirs.Load(absPath); exists {
			return nil
		}

		if err := w.fsWatch.Add(path); err != nil {
			return err
		}

		w.watchedDirs.Store(absPath, true)
		return nil
	})
}

// RemoveStale removes watches for directories that no longer exist
func (w *Watcher) RemoveStale() {
	w.watchedDirs.Range(func(key, _ any) bool {
		path := key.(string)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			w.fsWatch.Remove(path)
			w.watchedDirs.Delete(path)
		}
		return true
	})
}

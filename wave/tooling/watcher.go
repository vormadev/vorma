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

// watcher manages file watching for the dev server
type watcher struct {
	cfg     *wave.ParsedConfig
	log     *slog.Logger
	fsWatch *fsnotify.Watcher

	watchedDirs sync.Map

	// LRU cache for pattern matching results
	matchCache *lru.Cache[string, bool]

	// Patterns stored as absolute paths with forward slashes
	ignoredDirs       []string
	ignoredFiles      []string
	defaultWatched    []wave.WatchedFile
	configuredWatched []wave.WatchedFile

	// Absolute watch root for reference
	absWatchRoot     string
	absPublicStatic  string
	absPrivateStatic string
}

// newWatcher creates a new file watcher
func newWatcher(cfg *wave.ParsedConfig, log *slog.Logger) (*watcher, error) {
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

	w := &watcher{
		cfg:              cfg,
		log:              log,
		fsWatch:          fsWatch,
		absWatchRoot:     absWatchRoot,
		absPublicStatic:  absPublicStatic,
		absPrivateStatic: absPrivateStatic,
		matchCache:       lru.NewCache[string, bool](matchCacheMaxSize),
	}

	if err := w.setupPatterns(); err != nil {
		_ = fsWatch.Close()
		return nil, err
	}
	return w, nil
}

func (w *watcher) setupPatterns() error {
	watcherPlanForSetup, buildWatcherPlanError := w.buildWatcherPlan()
	if buildWatcherPlanError != nil {
		return buildWatcherPlanError
	}
	w.applyWatcherPlan(watcherPlanForSetup)
	return nil
}

// norm converts a path to absolute with forward slashes for consistent matching
func (w *watcher) norm(p string) string {
	return pathnorm.AbsoluteSlash(p)
}

func (w *watcher) normalizeLiteralPathForPattern(path string) string {
	return escapePatternMetaCharactersForDoublestarPattern(w.norm(path))
}

func (w *watcher) normalizePathOrPatternFromWatchRoot(pathOrPattern string) string {
	if filepath.IsAbs(pathOrPattern) {
		return w.norm(pathOrPattern)
	}

	normalizedJoinedPathOrPattern := w.norm(filepath.Join(w.cfg.WatchRoot(), pathOrPattern))
	normalizedWatchRoot := w.norm(w.cfg.WatchRoot())
	normalizedWatchRootPrefix := normalizedWatchRoot + "/"
	escapedWatchRoot := w.normalizeLiteralPathForPattern(w.cfg.WatchRoot())

	if normalizedJoinedPathOrPattern == normalizedWatchRoot {
		return escapedWatchRoot
	}
	if strings.HasPrefix(normalizedJoinedPathOrPattern, normalizedWatchRootPrefix) {
		return escapedWatchRoot + "/" + strings.TrimPrefix(
			normalizedJoinedPathOrPattern,
			normalizedWatchRootPrefix,
		)
	}

	return normalizedJoinedPathOrPattern
}

func escapePatternMetaCharactersForDoublestarPattern(path string) string {
	var escapedPathBuilder strings.Builder
	escapedPathBuilder.Grow(len(path))

	for _, character := range path {
		switch character {
		case '\\', '*', '?', '[', ']', '{', '}':
			escapedPathBuilder.WriteByte('\\')
		}
		escapedPathBuilder.WriteRune(character)
	}

	return escapedPathBuilder.String()
}

func (w *watcher) Events() <-chan fsnotify.Event {
	return w.fsWatch.Events
}

func (w *watcher) Errors() <-chan error {
	return w.fsWatch.Errors
}

func (w *watcher) Close() error {
	return w.fsWatch.Close()
}

// AddDir adds a directory and its subdirectories to the watcher
func (w *watcher) AddDir(root string) error {
	return filepath.WalkDir(root, func(path string, directoryEntry fs.DirEntry, err error) error {
		if err != nil || !directoryEntry.IsDir() {
			return err
		}

		if w.IsIgnoredDir(path) {
			return filepath.SkipDir
		}

		// Use absolute path as key to avoid duplicates
		absolutePath := w.norm(path)
		if _, exists := w.watchedDirs.Load(absolutePath); exists {
			return nil
		}

		if err := w.fsWatch.Add(path); err != nil {
			return err
		}

		w.watchedDirs.Store(absolutePath, true)
		return nil
	})
}

// RemoveStale removes watches for directories that no longer exist
func (w *watcher) RemoveStale() {
	w.watchedDirs.Range(func(key, _ any) bool {
		path := key.(string)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			w.fsWatch.Remove(path)
			w.watchedDirs.Delete(path)
		}
		return true
	})
}

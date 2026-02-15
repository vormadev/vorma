package tooling

import (
	"log/slog"
	"path/filepath"
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
	ignoredDirs    []string
	ignoredFiles   []string
	defaultWatched []wave.WatchedFile

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

	w.setupPatterns()
	return w, nil
}

// norm converts a path to absolute with forward slashes for consistent matching
func (w *watcher) norm(p string) string {
	return pathnorm.AbsoluteSlash(p)
}

func (w *watcher) normalizePathOrPatternFromWatchRoot(pathOrPattern string) string {
	if filepath.IsAbs(pathOrPattern) {
		return w.norm(pathOrPattern)
	}

	return w.norm(filepath.Join(w.cfg.WatchRoot(), pathOrPattern))
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

package tooling

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/lru"
	"github.com/vormadev/vorma/wave"
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
	absWatchRoot    string
	absPublicStatic string
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

	absWatchRoot, err := filepath.Abs(cfg.WatchRoot())
	if err != nil {
		absWatchRoot = cfg.WatchRoot()
	}

	absPublicStatic := ""
	if cfg.UsingBrowser() {
		abs, err := filepath.Abs(filepath.Clean(cfg.Core.StaticAssetDirs.Public))
		if err == nil {
			absPublicStatic = filepath.ToSlash(abs)
		}
	}

	w := &Watcher{
		cfg:             cfg,
		log:             log,
		fsWatch:         fsWatch,
		absWatchRoot:    filepath.ToSlash(absWatchRoot),
		absPublicStatic: absPublicStatic,
		matchCache:      lru.NewCache[string, bool](matchCacheMaxSize),
	}

	w.setupPatterns()
	return w, nil
}

// norm converts a path to absolute with forward slashes for consistent matching
func (w *Watcher) norm(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(abs)
}

func (w *Watcher) setupPatterns() {
	watchRoot := w.cfg.WatchRoot()

	w.ignoredFiles = []string{
		w.norm(w.cfg.Dist.Binary()),
	}

	// Add dist static as absolute path
	w.ignoredDirs = append(w.ignoredDirs, w.norm(w.cfg.Dist.Static()))
	w.ignoredDirs = append(w.ignoredDirs, w.norm(w.cfg.Dist.Static())+"/**")

	// Only add static asset patterns if not in server-only mode
	if w.cfg.UsingBrowser() {
		publicStatic := filepath.Clean(w.cfg.Core.StaticAssetDirs.Public)

		nohashDir := w.norm(filepath.Join(publicStatic, wave.NohashDirname))
		w.ignoredDirs = append(w.ignoredDirs, nohashDir)
		w.ignoredDirs = append(w.ignoredDirs, nohashDir+"/**")

		prehashedDir := w.norm(filepath.Join(publicStatic, wave.PrehashedDirname))
		w.ignoredDirs = append(w.ignoredDirs, prehashedDir)
		w.ignoredDirs = append(w.ignoredDirs, prehashedDir+"/**")

		// Public static files: Wave handles processing and writes filemap.ts directly.
		// No DevBuildHook needed - Vite HMR picks up the TS file change.
		w.defaultWatched = []wave.WatchedFile{
			{
				Pattern: w.norm(publicStatic) + "/**/*",
			},
		}
	}

	// Add framework-injected watch patterns
	w.addFrameworkWatchPatterns()

	// Add framework-injected ignored patterns
	for _, p := range w.cfg.FrameworkIgnoredPatterns {
		// Heuristic: if it ends in /** or looks like a dir, treat as ignored dir
		if strings.HasSuffix(p, "/**") {
			w.ignoredDirs = append(w.ignoredDirs, w.norm(p))
		} else {
			// It might be a file or a pattern
			w.ignoredFiles = append(w.ignoredFiles, w.norm(p))
		}
	}

	// For ** patterns, we need to anchor them to watch root
	w.ignoredDirs = append(w.ignoredDirs, w.absWatchRoot+"/"+globGit)
	w.ignoredDirs = append(w.ignoredDirs, w.absWatchRoot+"/"+globNodeModules)

	if w.cfg.Watch != nil {
		for _, p := range w.cfg.Watch.Exclude.Dirs {
			w.ignoredDirs = append(w.ignoredDirs, w.norm(filepath.Join(watchRoot, p)))
			w.ignoredDirs = append(w.ignoredDirs, w.norm(filepath.Join(watchRoot, p))+"/**")
		}
		for _, p := range w.cfg.Watch.Exclude.Files {
			w.ignoredFiles = append(w.ignoredFiles, w.norm(filepath.Join(watchRoot, p)))
		}
	}

	w.joinPatternsWithRoot()
	w.preSortHooks()
}

// addFrameworkWatchPatterns adds patterns injected by frameworks (e.g., Vorma)
func (w *Watcher) addFrameworkWatchPatterns() {
	for _, wf := range w.cfg.FrameworkWatchPatterns {
		// Normalize the pattern path
		pattern := wf.Pattern
		if !filepath.IsAbs(pattern) {
			pattern = w.absWatchRoot + "/" + filepath.ToSlash(pattern)
		} else {
			pattern = w.norm(pattern)
		}

		// Create a copy with normalized pattern
		normalizedWF := wf
		normalizedWF.Pattern = pattern

		// Normalize exclude patterns in hooks
		for i, hook := range normalizedWF.OnChangeHooks {
			for j, excl := range hook.Exclude {
				normalizedWF.OnChangeHooks[i].Exclude[j] = w.norm(filepath.Join(w.cfg.WatchRoot(), excl))
			}
		}

		w.defaultWatched = append(w.defaultWatched, normalizedWF)
	}
}

func (w *Watcher) joinPatternsWithRoot() {
	if w.cfg.Watch == nil {
		return
	}

	watchRoot := w.cfg.WatchRoot()

	for i, wf := range w.cfg.Watch.Include {
		pattern := wf.Pattern
		if !filepath.IsAbs(pattern) {
			pattern = w.absWatchRoot + "/" + filepath.ToSlash(pattern)
		} else {
			pattern = w.norm(pattern)
		}
		w.cfg.Watch.Include[i].Pattern = pattern

		for j, hook := range wf.OnChangeHooks {
			for k, excl := range hook.Exclude {
				w.cfg.Watch.Include[i].OnChangeHooks[j].Exclude[k] = w.norm(filepath.Join(watchRoot, excl))
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

// MatchPattern checks if a path matches a glob pattern.
// Both pattern and path should already be normalized (absolute + forward slashes).
func (w *Watcher) MatchPattern(pattern, path string) bool {
	key := pattern + "\x00" + path

	if cached, found := w.matchCache.Get(key); found {
		return cached
	}

	matches, err := doublestar.Match(pattern, path)
	if err != nil {
		w.log.Error("Pattern match error", "pattern", pattern, "path", path, "error", err)
		return false
	}

	w.matchCache.Set(key, matches, false)
	return matches
}

// IsIgnored checks if a path matches any of the ignored patterns.
// Normalizes the path before matching.
func (w *Watcher) IsIgnored(path string, patterns []string) bool {
	np := w.norm(path)
	for _, pattern := range patterns {
		if w.MatchPattern(pattern, np) {
			return true
		}
	}
	return false
}

// IsIgnoredFile checks if a file path should be ignored
func (w *Watcher) IsIgnoredFile(path string) bool {
	return w.IsIgnored(path, w.ignoredFiles)
}

// IsIgnoredDir checks if a directory path should be ignored
func (w *Watcher) IsIgnoredDir(path string) bool {
	return w.IsIgnored(path, w.ignoredDirs)
}

// FindWatchedFile finds and merges all matching WatchedFile configs for a path.
// Framework patterns are matched first, then user patterns. Settings are merged
// with "strongest wins" semantics: destructive flags use OR, suppressive flags use AND,
// and hooks are concatenated (framework first, then user).
func (w *Watcher) FindWatchedFile(path string) *wave.WatchedFile {
	np := w.norm(path)

	var matches []*wave.WatchedFile

	// Collect framework/default matches first (these run first in hook order)
	for i := range w.defaultWatched {
		wf := &w.defaultWatched[i]
		if w.MatchPattern(wf.Pattern, np) {
			matches = append(matches, wf)
		}
	}

	// Collect user-defined matches second (these run after framework hooks)
	if w.cfg.Watch != nil {
		for i := range w.cfg.Watch.Include {
			wf := &w.cfg.Watch.Include[i]
			if w.MatchPattern(wf.Pattern, np) {
				matches = append(matches, wf)
			}
		}
	}

	if len(matches) == 0 {
		return nil
	}

	if len(matches) == 1 {
		return matches[0]
	}

	return mergeWatchedFiles(matches)
}

// mergeWatchedFiles merges multiple WatchedFile configs into one.
//
// The principle is simple: union of all work. If ANY matching config requests
// an action, we do it. This prevents user config from accidentally disabling
// framework-critical behavior.
//
// For each possible action, we ask: "would ANY config cause this to happen?"
//
//   - Compile Go binary: yes if RecompileGoBinary=true OR (it's a .go file AND TreatAsNonGo=false)
//   - Restart app: yes if RestartApp=true
//   - Run standard build: yes if RunOnChangeOnly=false
//   - Show notification: yes if SkipRebuildingNotification=false
//   - Full reload (vs revalidate): yes if OnlyRunClientDefinedRevalidateFunc=false
//
// Hooks are concatenated: framework hooks first, then user hooks.
func mergeWatchedFiles(matches []*wave.WatchedFile) *wave.WatchedFile {
	if len(matches) == 0 {
		return nil
	}

	// Start with "no work" defaults
	merged := &wave.WatchedFile{
		Pattern: matches[0].Pattern,

		// These mean "do work" when true - start false, any true wins
		RecompileGoBinary: false,
		RestartApp:        false,

		// These mean "skip work" when true - start true, any false wins
		TreatAsNonGo:                       true,
		RunOnChangeOnly:                    true,
		SkipRebuildingNotification:         true,
		OnlyRunClientDefinedRevalidateFunc: true,
	}

	var allHooks []wave.OnChangeHook

	for _, wf := range matches {
		// "Do X" flags: if any config says do it, we do it
		if wf.RecompileGoBinary {
			merged.RecompileGoBinary = true
		}
		if wf.RestartApp {
			merged.RestartApp = true
		}

		// "Skip X" flags: if any config says DON'T skip, we don't skip
		if !wf.TreatAsNonGo {
			merged.TreatAsNonGo = false
		}
		if !wf.RunOnChangeOnly {
			merged.RunOnChangeOnly = false
		}
		if !wf.SkipRebuildingNotification {
			merged.SkipRebuildingNotification = false
		}
		if !wf.OnlyRunClientDefinedRevalidateFunc {
			merged.OnlyRunClientDefinedRevalidateFunc = false
		}

		allHooks = append(allHooks, wf.OnChangeHooks...)
	}

	merged.OnChangeHooks = allHooks
	merged.Sort()

	return merged
}

// IsPublicStaticFile checks if a path is within the public static directory
func (w *Watcher) IsPublicStaticFile(path string) bool {
	if w.absPublicStatic == "" {
		return false
	}
	np := w.norm(path)
	return strings.HasPrefix(np, w.absPublicStatic+"/")
}

// Debouncer batches rapid file events and ensures callbacks don't overlap.
type Debouncer struct {
	duration time.Duration
	callback func([]fsnotify.Event)
	mu       sync.Mutex
	timer    *time.Timer
	events   []fsnotify.Event
	stopped  bool
	inFlight bool
	pending  []fsnotify.Event
}

func NewDebouncer(d time.Duration, cb func([]fsnotify.Event)) *Debouncer {
	return &Debouncer{duration: d, callback: cb}
}

func (d *Debouncer) Add(evt fsnotify.Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}

	d.events = append(d.events, evt)

	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(d.duration, d.flush)
}

// flush is called by the timer. It checks if a callback is in-flight and either
// runs the callback or queues events for later.
func (d *Debouncer) flush() {
	d.mu.Lock()

	if d.stopped {
		d.mu.Unlock()
		return
	}

	events := d.events
	d.events = nil

	if len(events) == 0 {
		d.mu.Unlock()
		return
	}

	// If a callback is already running, queue these events for when it finishes
	if d.inFlight {
		d.pending = append(d.pending, events...)
		d.mu.Unlock()
		return
	}

	// Mark as in-flight and release lock before callback
	d.inFlight = true
	d.mu.Unlock()

	// Run callback outside of lock
	d.callback(events)

	// After callback completes, check for pending events
	d.mu.Lock()
	d.inFlight = false

	if len(d.pending) > 0 && !d.stopped {
		// Move pending to events and schedule another flush
		d.events = d.pending
		d.pending = nil
		d.timer = time.AfterFunc(d.duration, d.flush)
	}
	d.mu.Unlock()
}

// Stop cancels any pending debounced callback and prevents future events.
// This should be called when the watcher is being closed to prevent
// callbacks from firing during or after cleanup.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stopped = true
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.events = nil
	d.pending = nil
}

// isNonEmptyChmodOnly checks if an event is only a chmod operation on a non-empty file.
// We skip these because they're likely just permission changes, not content changes.
// However, chmod on an empty file might be part of a file creation sequence
// (some editors: create empty → chmod → write), so we don't skip those.
func isNonEmptyChmodOnly(evt fsnotify.Event) bool {
	if evt.Has(fsnotify.Write) || evt.Has(fsnotify.Create) || evt.Has(fsnotify.Remove) ||
		evt.Has(fsnotify.Rename) {
		return false
	}

	info, err := os.Stat(evt.Name)
	if err != nil {
		return false
	}

	return info.Size() > 0
}

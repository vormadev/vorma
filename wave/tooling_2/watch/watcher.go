package watch

import (
	"fmt"
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
	"github.com/vormadev/vorma/wave/internal/waveshared"
	"github.com/vormadev/vorma/wave/tooling_2/toolingshared"
)

// matchCacheMaxSize limits the match cache to prevent unbounded memory growth
const matchCacheMaxSize = 10000

// Ignore patterns - these are glob patterns, not path segments
const (
	globGit         = "**/.git"
	globNodeModules = "**/node_modules"
)

// Watcher manages file watching for the dev server.
type Watcher struct {
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

// NewWatcher creates a new file watcher
func NewWatcher(cfg *wave.ParsedConfig, log *slog.Logger) (*Watcher, error) {
	if log == nil {
		log = colorlog.New("wave")
	}

	fsWatch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	absWatchRoot := waveshared.AbsoluteSlash(cfg.WatchRoot())

	absPublicStatic := ""
	absPrivateStatic := ""
	if cfg.UsingBrowser() {
		absPublicStatic = waveshared.AbsoluteSlash(cfg.Core.StaticAssetDirs.Public)
		absPrivateStatic = waveshared.AbsoluteSlash(cfg.Core.StaticAssetDirs.Private)
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

	if err := w.setupPatterns(); err != nil {
		_ = fsWatch.Close()
		return nil, err
	}
	return w, nil
}

func (w *Watcher) setupPatterns() error {
	watcherPlanForSetup, buildWatcherPlanError := w.buildWatcherPlan()
	if buildWatcherPlanError != nil {
		return buildWatcherPlanError
	}
	w.applyWatcherPlan(watcherPlanForSetup)
	return nil
}

// norm converts a path to absolute with forward slashes for consistent matching
func (w *Watcher) NormalizePath(p string) string {
	return waveshared.AbsoluteSlash(p)
}

func (w *Watcher) normalizeLiteralPathForPattern(path string) string {
	return escapePatternMetaCharactersForDoublestarPattern(w.NormalizePath(path))
}

func (w *Watcher) normalizePathOrPatternFromWatchRoot(pathOrPattern string) string {
	if filepath.IsAbs(pathOrPattern) {
		return w.NormalizePath(pathOrPattern)
	}

	normalizedJoinedPathOrPattern := w.NormalizePath(filepath.Join(w.cfg.WatchRoot(), pathOrPattern))
	normalizedWatchRoot := w.NormalizePath(w.cfg.WatchRoot())
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
	return filepath.WalkDir(root, func(path string, directoryEntry fs.DirEntry, err error) error {
		if err != nil || !directoryEntry.IsDir() {
			return err
		}

		if w.IsIgnoredDir(path) {
			return filepath.SkipDir
		}

		// Use absolute path as key to avoid duplicates
		absolutePath := w.NormalizePath(path)
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

// IsWatchingDir reports whether the watcher currently has a watch entry for the path.
func (w *Watcher) IsWatchingDir(path string) bool {
	normalizedPath := w.NormalizePath(path)
	_, isWatchingPath := w.watchedDirs.Load(normalizedPath)
	return isWatchingPath
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
	normalizedPath := w.NormalizePath(path)
	for _, pattern := range patterns {
		if w.MatchPattern(pattern, normalizedPath) {
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
	normalizedPath := w.NormalizePath(path)

	var matches []*wave.WatchedFile

	// Collect framework/default matches first (these run first in hook order)
	for i := range w.defaultWatched {
		watchedFile := &w.defaultWatched[i]
		if w.MatchPattern(watchedFile.Pattern, normalizedPath) {
			matches = append(matches, watchedFile)
		}
	}

	// Collect user-defined matches second (these run after framework hooks)
	for i := range w.configuredWatched {
		watchedFile := &w.configuredWatched[i]
		if w.MatchPattern(watchedFile.Pattern, normalizedPath) {
			matches = append(matches, watchedFile)
		}
	}

	if len(matches) == 0 {
		return nil
	}

	if len(matches) == 1 {
		return matches[0]
	}

	return MergeWatchedFiles(matches)
}

// MergeWatchedFiles merges multiple WatchedFile configs into one.
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
//   - Revalidate only (vs full reload): yes if OnlyRunClientDefinedRevalidateFunc=true (trump flag)
//
// Hooks are concatenated: framework hooks first, then user hooks.
func MergeWatchedFiles(matches []*wave.WatchedFile) *wave.WatchedFile {
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
		TreatAsNonGo:               true,
		RunOnChangeOnly:            true,
		SkipRebuildingNotification: true,

		// Trump flag: user explicitly overriding browser behavior - any true wins
		OnlyRunClientDefinedRevalidateFunc: false,
	}

	var allHooks []wave.OnChangeHook

	for _, watchedFile := range matches {
		// "Do X" flags: if any config says do it, we do it
		if watchedFile.RecompileGoBinary {
			merged.RecompileGoBinary = true
		}
		if watchedFile.RestartApp {
			merged.RestartApp = true
		}

		// "Skip X" flags: if any config says DON'T skip, we don't skip
		if !watchedFile.TreatAsNonGo {
			merged.TreatAsNonGo = false
		}
		if !watchedFile.RunOnChangeOnly {
			merged.RunOnChangeOnly = false
		}
		if !watchedFile.SkipRebuildingNotification {
			merged.SkipRebuildingNotification = false
		}

		// Trump flag: user explicitly overriding browser behavior
		if watchedFile.OnlyRunClientDefinedRevalidateFunc {
			merged.OnlyRunClientDefinedRevalidateFunc = true
		}

		allHooks = append(allHooks, watchedFile.OnChangeHooks...)
	}

	merged.OnChangeHooks = allHooks
	merged.Sort()

	return merged
}

// IsPublicStaticFile checks if a path is within the public static directory
func (w *Watcher) IsPublicStaticFile(path string) bool {
	return w.isPathWithinStaticDirectory(path, w.absPublicStatic)
}

// IsPrivateStaticFile checks if a path is within the private static directory
func (w *Watcher) IsPrivateStaticFile(path string) bool {
	return w.isPathWithinStaticDirectory(path, w.absPrivateStatic)
}

func (w *Watcher) isPathWithinStaticDirectory(
	path string,
	absoluteStaticDirectoryPath string,
) bool {
	if absoluteStaticDirectoryPath == "" {
		return false
	}
	normalizedPath := w.NormalizePath(path)
	return strings.HasPrefix(normalizedPath, absoluteStaticDirectoryPath+"/")
}

type watcherPlan struct {
	ignoredFiles      []string
	ignoredDirs       []string
	defaultWatched    []wave.WatchedFile
	configuredWatched []wave.WatchedFile
}

func (w *Watcher) buildWatcherPlan() (watcherPlan, error) {
	watcherPlanForSetup := watcherPlan{
		ignoredFiles: []string{
			w.normalizeLiteralPathForPattern(w.cfg.Dist.Binary()),
		},
		ignoredDirs:       make([]string, 0),
		defaultWatched:    make([]wave.WatchedFile, 0),
		configuredWatched: make([]wave.WatchedFile, 0),
	}

	// Add dist static as absolute path.
	normalizedDistStaticPathPattern := w.normalizeLiteralPathForPattern(w.cfg.Dist.Static())
	watcherPlanForSetup.ignoredDirs = append(
		watcherPlanForSetup.ignoredDirs,
		normalizedDistStaticPathPattern,
	)
	watcherPlanForSetup.ignoredDirs = append(
		watcherPlanForSetup.ignoredDirs,
		normalizedDistStaticPathPattern+"/**",
	)

	// Only add static asset patterns if not in server-only mode.
	if w.cfg.UsingBrowser() {
		publicStatic := filepath.Clean(w.cfg.Core.StaticAssetDirs.Public)
		privateStatic := filepath.Clean(w.cfg.Core.StaticAssetDirs.Private)

		nohashDir := w.normalizeLiteralPathForPattern(filepath.Join(publicStatic, wave.NohashDirname))
		watcherPlanForSetup.ignoredDirs = append(
			watcherPlanForSetup.ignoredDirs,
			nohashDir,
			nohashDir+"/**",
		)

		prehashedDir := w.normalizeLiteralPathForPattern(filepath.Join(publicStatic, wave.PrehashedDirname))
		watcherPlanForSetup.ignoredDirs = append(
			watcherPlanForSetup.ignoredDirs,
			prehashedDir,
			prehashedDir+"/**",
		)

		// Public static files: Wave handles processing and writes filemap.ts directly.
		// No explicit dev build hook command is needed - Vite HMR picks up the TS file change.
		publicStaticPatternPrefix := w.normalizeLiteralPathForPattern(publicStatic)
		watcherPlanForSetup.defaultWatched = append(
			watcherPlanForSetup.defaultWatched,
			wave.WatchedFile{
				Pattern: publicStaticPatternPrefix + "/**/*",
			},
		)

		// Private static files: Wave handles processing and triggers browser reload.
		privateStaticPatternPrefix := w.normalizeLiteralPathForPattern(privateStatic)
		watcherPlanForSetup.defaultWatched = append(
			watcherPlanForSetup.defaultWatched,
			wave.WatchedFile{
				Pattern: privateStaticPatternPrefix + "/**/*",
			},
		)
	}

	// Add framework-injected watch patterns.
	for frameworkWatchPatternIndex, watchedFile := range w.cfg.FrameworkWatchPatterns {
		normalizedWatchedFile, normalizeWatchedFileError := w.normalizeWatchedFileForWatcherPlan(
			normalizeWatchedFileForWatcherPlanArgs{
				watchedFile: watchedFile,
				fieldPath: fmt.Sprintf(
					"FrameworkWatchPatterns[%d]",
					frameworkWatchPatternIndex,
				),
			},
		)
		if normalizeWatchedFileError != nil {
			return watcherPlan{}, normalizeWatchedFileError
		}
		watcherPlanForSetup.defaultWatched = append(
			watcherPlanForSetup.defaultWatched,
			normalizedWatchedFile,
		)
	}

	// Add framework-injected ignored patterns.
	for ignoredPatternIndex, ignoredPattern := range w.cfg.FrameworkIgnoredPatterns {
		normalizedPattern := w.normalizePathOrPatternFromWatchRoot(ignoredPattern)
		if validatePatternError := validateWatcherGlobPatternInputForRuntime(
			fmt.Sprintf("FrameworkIgnoredPatterns[%d]", ignoredPatternIndex),
			ignoredPattern,
		); validatePatternError != nil {
			return watcherPlan{}, validatePatternError
		}

		// Framework ignored patterns apply to both file and directory checks.
		watcherPlanForSetup.ignoredFiles = append(
			watcherPlanForSetup.ignoredFiles,
			normalizedPattern,
		)
		if !strings.HasSuffix(ignoredPattern, "/**") {
			watcherPlanForSetup.ignoredFiles = append(
				watcherPlanForSetup.ignoredFiles,
				normalizedPattern+"/**",
			)
		}
		watcherPlanForSetup.ignoredDirs = append(
			watcherPlanForSetup.ignoredDirs,
			normalizedPattern,
		)
		if !strings.HasSuffix(ignoredPattern, "/**") {
			watcherPlanForSetup.ignoredDirs = append(
				watcherPlanForSetup.ignoredDirs,
				normalizedPattern+"/**",
			)
		}
	}

	// For ** patterns, we need to anchor them to watch root.
	normalizedWatchRootPathPattern := w.normalizeLiteralPathForPattern(w.cfg.WatchRoot())
	watcherPlanForSetup.ignoredDirs = append(
		watcherPlanForSetup.ignoredDirs,
		normalizedWatchRootPathPattern+"/"+globGit,
		normalizedWatchRootPathPattern+"/"+globNodeModules,
	)

	if w.cfg.Watch != nil {
		for watchedFileIndex, watchedFile := range w.cfg.Watch.Include {
			normalizedWatchedFile, normalizeWatchedFileError := w.normalizeWatchedFileForWatcherPlan(
				normalizeWatchedFileForWatcherPlanArgs{
					watchedFile: watchedFile,
					fieldPath: fmt.Sprintf(
						"Watch.Include[%d]",
						watchedFileIndex,
					),
				},
			)
			if normalizeWatchedFileError != nil {
				return watcherPlan{}, normalizeWatchedFileError
			}
			watcherPlanForSetup.configuredWatched = append(
				watcherPlanForSetup.configuredWatched,
				normalizedWatchedFile,
			)
		}

		for excludedDirectoryPatternIndex, excludedDirectoryPattern := range w.cfg.Watch.Exclude.Dirs {
			normalizedDirectoryPattern := w.normalizePathOrPatternFromWatchRoot(
				excludedDirectoryPattern,
			)
			if validatePatternError := validateWatcherGlobPatternInputForRuntime(
				fmt.Sprintf("Watch.Exclude.Dirs[%d]", excludedDirectoryPatternIndex),
				excludedDirectoryPattern,
			); validatePatternError != nil {
				return watcherPlan{}, validatePatternError
			}
			watcherPlanForSetup.ignoredDirs = append(
				watcherPlanForSetup.ignoredDirs,
				normalizedDirectoryPattern,
				normalizedDirectoryPattern+"/**",
			)
		}
		for excludedFilePatternIndex, excludedFilePattern := range w.cfg.Watch.Exclude.Files {
			normalizedExcludedFilePattern := w.normalizePathOrPatternFromWatchRoot(excludedFilePattern)
			if validatePatternError := validateWatcherGlobPatternInputForRuntime(
				fmt.Sprintf("Watch.Exclude.Files[%d]", excludedFilePatternIndex),
				excludedFilePattern,
			); validatePatternError != nil {
				return watcherPlan{}, validatePatternError
			}
			watcherPlanForSetup.ignoredFiles = append(
				watcherPlanForSetup.ignoredFiles,
				normalizedExcludedFilePattern,
			)
		}
	}

	return watcherPlanForSetup, nil
}

func (w *Watcher) applyWatcherPlan(
	watcherPlanForSetup watcherPlan,
) {
	w.ignoredFiles = watcherPlanForSetup.ignoredFiles
	w.ignoredDirs = watcherPlanForSetup.ignoredDirs
	w.defaultWatched = watcherPlanForSetup.defaultWatched
	w.configuredWatched = watcherPlanForSetup.configuredWatched
}

type normalizeWatchedFileForWatcherPlanArgs struct {
	watchedFile wave.WatchedFile
	fieldPath   string
}

func (w *Watcher) normalizeWatchedFileForWatcherPlan(
	args normalizeWatchedFileForWatcherPlanArgs,
) (wave.WatchedFile, error) {
	watchedFile := args.watchedFile
	normalizedWatchedFile := watchedFile
	normalizedWatchedFile.Pattern = w.normalizePathOrPatternFromWatchRoot(
		watchedFile.Pattern,
	)
	if validatePatternError := validateWatcherGlobPatternInputForRuntime(
		args.fieldPath+".Pattern",
		watchedFile.Pattern,
	); validatePatternError != nil {
		return wave.WatchedFile{}, validatePatternError
	}

	normalizedOnChangeHooks, normalizeOnChangeHookExcludesError := cloneOnChangeHooksForWatcherPlan(
		cloneOnChangeHooksForWatcherPlanArgs{
			onChangeHooks:                    watchedFile.OnChangeHooks,
			normalizePathOrPatternForExclude: w.normalizePathOrPatternFromWatchRoot,
			fieldPath:                        args.fieldPath + ".OnChangeHooks",
		},
	)
	if normalizeOnChangeHookExcludesError != nil {
		return wave.WatchedFile{}, normalizeOnChangeHookExcludesError
	}
	normalizedWatchedFile.OnChangeHooks = normalizedOnChangeHooks
	normalizedWatchedFile.SortedHooks = DeriveSortedHooksForWatcherPlan(
		normalizedWatchedFile.OnChangeHooks,
	)

	return normalizedWatchedFile, nil
}

type cloneOnChangeHooksForWatcherPlanArgs struct {
	onChangeHooks                    []wave.OnChangeHook
	normalizePathOrPatternForExclude func(string) string
	fieldPath                        string
}

func cloneOnChangeHooksForWatcherPlan(
	args cloneOnChangeHooksForWatcherPlanArgs,
) ([]wave.OnChangeHook, error) {
	onChangeHooks := args.onChangeHooks
	normalizePathOrPatternForExclude := args.normalizePathOrPatternForExclude

	if len(onChangeHooks) == 0 {
		return nil, nil
	}

	clonedOnChangeHooks := make([]wave.OnChangeHook, 0, len(onChangeHooks))
	for hookIndex, onChangeHook := range onChangeHooks {
		clonedOnChangeHook := onChangeHook
		clonedOnChangeHook.Exclude = append([]string(nil), onChangeHook.Exclude...)
		if normalizePathOrPatternForExclude != nil {
			for excludeIndex, excludePattern := range clonedOnChangeHook.Exclude {
				if validatePatternError := validateWatcherGlobPatternInputForRuntime(
					fmt.Sprintf(
						"%s[%d].Exclude[%d]",
						args.fieldPath,
						hookIndex,
						excludeIndex,
					),
					excludePattern,
				); validatePatternError != nil {
					return nil, validatePatternError
				}
				clonedOnChangeHook.Exclude[excludeIndex] = normalizePathOrPatternForExclude(
					excludePattern,
				)
			}
		}
		clonedOnChangeHooks = append(clonedOnChangeHooks, clonedOnChangeHook)
	}

	return clonedOnChangeHooks, nil
}

func DeriveSortedHooksForWatcherPlan(
	onChangeHooks []wave.OnChangeHook,
) *wave.SortedHooks {
	if len(onChangeHooks) == 0 {
		return &wave.SortedHooks{}
	}

	watchedFileForSorting := wave.WatchedFile{
		OnChangeHooks: cloneOnChangeHooksForSortingWithoutNormalization(
			onChangeHooks,
		),
	}
	watchedFileForSorting.Sort()
	return CloneSortedHooksForWatcherPlan(watchedFileForSorting.SortedHooks)
}

func CloneSortedHooksForWatcherPlan(
	sortedHooks *wave.SortedHooks,
) *wave.SortedHooks {
	if sortedHooks == nil {
		return &wave.SortedHooks{}
	}

	return &wave.SortedHooks{
		Pre: cloneOnChangeHooksForSortingWithoutNormalization(sortedHooks.Pre),
		Concurrent: cloneOnChangeHooksForSortingWithoutNormalization(
			sortedHooks.Concurrent,
		),
		ConcurrentNoWait: cloneOnChangeHooksForSortingWithoutNormalization(
			sortedHooks.ConcurrentNoWait,
		),
		Post: cloneOnChangeHooksForSortingWithoutNormalization(sortedHooks.Post),
	}
}

func cloneOnChangeHooksForSortingWithoutNormalization(
	onChangeHooks []wave.OnChangeHook,
) []wave.OnChangeHook {
	clonedOnChangeHooks, _ := cloneOnChangeHooksForWatcherPlan(
		cloneOnChangeHooksForWatcherPlanArgs{
			onChangeHooks: onChangeHooks,
		},
	)
	return clonedOnChangeHooks
}

func validateWatcherGlobPatternInputForRuntime(
	fieldPath string,
	pattern string,
) error {
	return toolingshared.ValidateNamedGlobPatternInput("watcher setup", fieldPath, pattern)
}

// Debouncer batches rapid file events and ensures callbacks do not overlap.
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

// NewDebouncer creates a file-event debouncer for watch pipelines.
func NewDebouncer(
	duration time.Duration,
	callback func([]fsnotify.Event),
) *Debouncer {
	return &Debouncer{duration: duration, callback: callback}
}

func (debouncer *Debouncer) Add(event fsnotify.Event) {
	debouncer.mu.Lock()
	defer debouncer.mu.Unlock()

	if debouncer.stopped {
		return
	}

	debouncer.events = append(debouncer.events, event)

	if debouncer.timer != nil {
		debouncer.timer.Stop()
	}

	debouncer.timer = time.AfterFunc(debouncer.duration, debouncer.flush)
}

// flush is called by the timer. It checks if a callback is in-flight and either
// runs the callback or queues events for later.
func (debouncer *Debouncer) flush() {
	debouncer.mu.Lock()

	if debouncer.stopped {
		debouncer.mu.Unlock()
		return
	}

	events := debouncer.events
	debouncer.events = nil

	if len(events) == 0 {
		debouncer.mu.Unlock()
		return
	}

	// If a callback is already running, queue these events for when it finishes
	if debouncer.inFlight {
		debouncer.pending = append(debouncer.pending, events...)
		debouncer.mu.Unlock()
		return
	}

	// Mark as in-flight and release lock before callback
	debouncer.inFlight = true
	debouncer.mu.Unlock()

	// Run callback outside of lock
	debouncer.callback(events)

	// After callback completes, check for pending events
	debouncer.mu.Lock()
	debouncer.inFlight = false

	if len(debouncer.pending) > 0 && !debouncer.stopped {
		// Move pending to events and schedule another flush
		debouncer.events = debouncer.pending
		debouncer.pending = nil
		debouncer.timer = time.AfterFunc(debouncer.duration, debouncer.flush)
	}
	debouncer.mu.Unlock()
}

// Stop cancels any pending debounced callback and prevents future events.
// This should be called when the watcher is being closed to prevent
// callbacks from firing during or after cleanup.
func (debouncer *Debouncer) Stop() {
	debouncer.mu.Lock()
	defer debouncer.mu.Unlock()

	debouncer.stopped = true
	if debouncer.timer != nil {
		debouncer.timer.Stop()
		debouncer.timer = nil
	}
	debouncer.events = nil
	debouncer.pending = nil
}

// IsNonEmptyChmodOnly checks if an event is only a chmod operation on a non-empty file.
// We skip these because they're likely just permission changes, not content changes.
// However, chmod on an empty file might be part of a file creation sequence
// (some editors: create empty -> chmod -> write), so we don't skip those.
func IsNonEmptyChmodOnly(event fsnotify.Event) bool {
	if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) ||
		event.Has(fsnotify.Rename) {
		return false
	}

	info, err := os.Stat(event.Name)
	if err != nil {
		return false
	}

	return info.Size() > 0
}

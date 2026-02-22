package watch

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/wavecore"
	"github.com/vormadev/vorma/wave/tooling/internal/shared"
	"github.com/vormadev/vorma/wave/tooling/internal/watch/dedup"
)

// Watcher provides filesystem watch streams plus semantic file classification helpers.
type Watcher struct {
	cfg *wave.ParsedConfig
	log *slog.Logger

	watcher *fsnotify.Watcher

	mu                  sync.RWMutex
	watchedDirectories  map[string]struct{}
	includeRules        []includeRule
	excludeDirPatterns  []string
	excludeFilePatterns []string

	publicStaticRoot  string
	privateStaticRoot string

	lastEventByPath map[string]time.Time
}

// includeRule binds one normalized glob to its watched-file semantic metadata.
type includeRule struct {
	pattern     string
	watchedFile *wave.WatchedFile
}

// NewWatcher creates and initializes a recursive watcher for project roots.
func NewWatcher(cfg *wave.ParsedConfig, log *slog.Logger) (*Watcher, error) {
	if cfg == nil {
		return nil, errors.New("watcher config is nil")
	}
	if log == nil {
		log = slog.Default()
	}
	if validatePatternsError := validateWatcherConfigPatterns(cfg); validatePatternsError != nil {
		return nil, validatePatternsError
	}

	fsnotifyWatcher, watcherCreateError := fsnotify.NewWatcher()
	if watcherCreateError != nil {
		return nil, fmt.Errorf(
			"create fsnotify watcher: %w",
			watcherCreateError,
		)
	}

	resolvedIncludeRules := buildIncludeRules(cfg)
	resolvedExcludeDirectoryPatterns := buildExcludeDirectoryPatterns(cfg)
	resolvedExcludeFilePatterns := buildExcludeFilePatterns(cfg)

	watcher := &Watcher{
		cfg:                 cfg,
		log:                 log,
		watcher:             fsnotifyWatcher,
		watchedDirectories:  make(map[string]struct{}),
		includeRules:        resolvedIncludeRules,
		excludeDirPatterns:  resolvedExcludeDirectoryPatterns,
		excludeFilePatterns: resolvedExcludeFilePatterns,
		publicStaticRoot: normalizePath(
			filepath.Clean(cfg.Core.StaticAssetDirs.Public),
		),
		privateStaticRoot: normalizePath(
			filepath.Clean(cfg.Core.StaticAssetDirs.Private),
		),
		lastEventByPath: make(map[string]time.Time),
	}

	if initializeError := watcher.initializeDirectoryWatchPlan(); initializeError != nil {
		_ = fsnotifyWatcher.Close()
		return nil, initializeError
	}

	return watcher, nil
}

// initializeDirectoryWatchPlan installs directory watches for watch root subtree.
func (watcher *Watcher) initializeDirectoryWatchPlan() error {
	watchRoot := watcher.cfg.WatchRoot()
	if strings.TrimSpace(watchRoot) == "" {
		watchRoot = "."
	}
	watchRoot = filepath.Clean(watchRoot)
	watchRootInfo, watchRootStatError := os.Stat(watchRoot)
	if watchRootStatError != nil {
		return fmt.Errorf("watch root %q: %w", watchRoot, watchRootStatError)
	}
	if !watchRootInfo.IsDir() {
		return fmt.Errorf("watch root %q is not a directory", watchRoot)
	}

	if addError := watcher.addDirectoryRecursively(watchRoot); addError != nil {
		return addError
	}
	return nil
}

// Close closes the underlying fsnotify watcher.
func (watcher *Watcher) Close() error {
	if watcher == nil || watcher.watcher == nil {
		return nil
	}
	return watcher.watcher.Close()
}

// Events exposes fsnotify watcher events.
func (watcher *Watcher) Events() <-chan fsnotify.Event {
	if watcher == nil || watcher.watcher == nil {
		return nil
	}
	return watcher.watcher.Events
}

// Errors exposes fsnotify watcher errors.
func (watcher *Watcher) Errors() <-chan error {
	if watcher == nil || watcher.watcher == nil {
		return nil
	}
	return watcher.watcher.Errors
}

// WatchedDirectoryPaths returns a sorted snapshot of currently watched directories.
func (watcher *Watcher) WatchedDirectoryPaths() []string {
	if watcher == nil {
		return nil
	}

	watcher.mu.RLock()
	defer watcher.mu.RUnlock()

	paths := make([]string, 0, len(watcher.watchedDirectories))
	for path := range watcher.watchedDirectories {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// NormalizePath normalizes one path to watcher internal comparison format.
func (watcher *Watcher) NormalizePath(path string) string {
	_ = watcher
	return normalizePath(path)
}

// IsWatchingDir reports whether the watcher currently tracks one directory path.
func (watcher *Watcher) IsWatchingDir(path string) bool {
	if watcher == nil {
		return false
	}
	normalizedPath := normalizePath(path)
	watcher.mu.RLock()
	defer watcher.mu.RUnlock()
	_, exists := watcher.watchedDirectories[normalizedPath]
	return exists
}

// AddDirectoryRecursively adds a directory and all subdirectories to watcher.
func (watcher *Watcher) AddDirectoryRecursively(path string) error {
	if watcher == nil {
		return errors.New("watcher is nil")
	}
	return watcher.addDirectoryRecursively(path)
}

// AddDir adds a directory and all subdirectories to watcher.
func (watcher *Watcher) AddDir(path string) error {
	return watcher.AddDirectoryRecursively(path)
}

// addDirectoryRecursively performs recursive directory registration.
func (watcher *Watcher) addDirectoryRecursively(path string) error {
	rootPath := normalizePath(path)
	if rootPath == "" {
		return nil
	}

	walkError := filepath.WalkDir(
		rootPath,
		func(currentPath string, directoryEntry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			normalizedCurrentPath := normalizePath(currentPath)
			if normalizedCurrentPath == "" {
				return nil
			}

			if directoryEntry.IsDir() {
				if watcher.IsIgnoredDirectory(normalizedCurrentPath) {
					return filepath.SkipDir
				}
				return watcher.addSingleDirectory(normalizedCurrentPath)
			}
			return nil
		},
	)
	if walkError != nil {
		return fmt.Errorf(
			"walk directory %q for watch registration: %w",
			rootPath,
			walkError,
		)
	}
	return nil
}

// addSingleDirectory registers one directory unless already watched.
func (watcher *Watcher) addSingleDirectory(directoryPath string) error {
	watcher.mu.RLock()
	_, alreadyWatched := watcher.watchedDirectories[directoryPath]
	watcher.mu.RUnlock()
	if alreadyWatched {
		return nil
	}

	if addError := watcher.watcher.Add(directoryPath); addError != nil {
		if isKnownNonWatchablePathError(addError) {
			return nil
		}
		return fmt.Errorf("watch directory %q: %w", directoryPath, addError)
	}

	watcher.mu.Lock()
	watcher.watchedDirectories[directoryPath] = struct{}{}
	watcher.mu.Unlock()
	return nil
}

// IsIgnoredDirectory reports whether a directory should be excluded from watch registration.
func (watcher *Watcher) IsIgnoredDirectory(directoryPath string) bool {
	normalizedPath := normalizePath(directoryPath)
	if normalizedPath == "" {
		return true
	}

	// Never watch dist output roots to avoid self-trigger loops.
	if normalizePath(watcher.cfg.Dist.Static()) == normalizedPath {
		return true
	}
	if strings.HasPrefix(
		normalizedPath,
		normalizePath(watcher.cfg.Dist.Static())+"/",
	) {
		return true
	}

	for _, pattern := range watcher.excludeDirPatterns {
		if shared.MatchPathAgainstGlob(normalizedPath, pattern) {
			return true
		}
	}
	return false
}

// IsIgnoredDir reports whether a directory should be excluded from watch registration.
func (watcher *Watcher) IsIgnoredDir(directoryPath string) bool {
	return watcher.IsIgnoredDirectory(directoryPath)
}

// IsIgnoredFile reports whether a file path should be excluded from event processing.
func (watcher *Watcher) IsIgnoredFile(filePath string) bool {
	normalizedPath := normalizePath(filePath)
	if normalizedPath == "" {
		return true
	}

	if filepath.Base(normalizedPath) == shared.LockFileName {
		return true
	}

	for _, pattern := range watcher.excludeFilePatterns {
		if shared.MatchPathAgainstGlob(normalizedPath, pattern) {
			return true
		}
	}

	for _, directoryPattern := range watcher.excludeDirPatterns {
		if shared.MatchPathAgainstGlob(
			normalizedPath,
			directoryPattern,
		) {
			return true
		}
	}

	return false
}

// TrackEvent stores the latest event timestamp for a path.
func (watcher *Watcher) TrackEvent(eventPath string) {
	if watcher == nil {
		return
	}
	normalizedPath := normalizePath(eventPath)
	if normalizedPath == "" {
		return
	}
	watcher.mu.Lock()
	watcher.lastEventByPath[normalizedPath] = time.Now()
	watcher.mu.Unlock()
}

// RemoveStale forgets historical path entries that no longer exist on disk.
func (watcher *Watcher) RemoveStale() {
	if watcher == nil {
		return
	}
	watcher.mu.Lock()
	defer watcher.mu.Unlock()

	for directoryPath := range watcher.watchedDirectories {
		if _, statError := os.Stat(directoryPath); statError != nil &&
			errors.Is(statError, os.ErrNotExist) {
			if watcher.watcher != nil {
				_ = watcher.watcher.Remove(directoryPath)
			}
			delete(watcher.watchedDirectories, directoryPath)
		}
	}

	for path := range watcher.lastEventByPath {
		if _, statError := os.Stat(path); statError != nil &&
			errors.Is(statError, os.ErrNotExist) {
			delete(watcher.lastEventByPath, path)
		}
	}
}

// FindWatchedFile returns watched file semantics matching the path, if any.
func (watcher *Watcher) FindWatchedFile(path string) *wave.WatchedFile {
	normalizedPath := normalizePath(path)
	if normalizedPath == "" {
		return nil
	}

	matches := make([]*wave.WatchedFile, 0)
	for _, rule := range watcher.includeRules {
		if shared.MatchPathAgainstGlob(normalizedPath, rule.pattern) {
			matches = append(matches, rule.watchedFile)
		}
	}
	return dedup.MergeWatchedFiles(matches)
}

// IsPublicStaticFile reports whether path targets configured public static sources.
func (watcher *Watcher) IsPublicStaticFile(path string) bool {
	normalizedPath := normalizePath(path)
	if normalizedPath == "" {
		return false
	}
	if watcher.publicStaticRoot == "" {
		return false
	}
	if normalizedPath == watcher.publicStaticRoot {
		return true
	}
	return strings.HasPrefix(normalizedPath, watcher.publicStaticRoot+"/")
}

// IsPrivateStaticFile reports whether path targets configured private static sources.
func (watcher *Watcher) IsPrivateStaticFile(path string) bool {
	normalizedPath := normalizePath(path)
	if normalizedPath == "" {
		return false
	}
	if watcher.privateStaticRoot == "" {
		return false
	}
	if normalizedPath == watcher.privateStaticRoot {
		return true
	}
	return strings.HasPrefix(normalizedPath, watcher.privateStaticRoot+"/")
}

// EnsureDirectoryWatchForEventPath adds a new directory watch for created directories.
func (watcher *Watcher) EnsureDirectoryWatchForEventPath(path string) {
	if watcher == nil {
		return
	}
	normalizedPath := normalizePath(path)
	if normalizedPath == "" {
		return
	}
	fileInfo, statError := os.Stat(normalizedPath)
	if statError != nil || !fileInfo.IsDir() {
		return
	}
	if addError := watcher.addDirectoryRecursively(normalizedPath); addError != nil {
		watcher.log.Debug(
			"failed to add directory watch dynamically",
			"path",
			normalizedPath,
			"error",
			addError,
		)
	}
}

// buildIncludeRules merges app and framework watched file patterns.
func buildIncludeRules(cfg *wave.ParsedConfig) []includeRule {
	includeRules := make([]includeRule, 0)
	appendRules := func(watchedFiles []wave.WatchedFile) {
		for watchedFileIndex := range watchedFiles {
			watchedFile := watchedFiles[watchedFileIndex]
			normalizedPattern := resolvePathOrPatternFromWatchRoot(
				cfg,
				watchedFile.Pattern,
			)
			if normalizedPattern == "" {
				continue
			}
			watchedFileCopy := watchedFile
			watchedFileCopy.Pattern = normalizedPattern
			includeRules = append(includeRules, includeRule{
				pattern:     normalizedPattern,
				watchedFile: &watchedFileCopy,
			})
		}
	}

	appendRules(cfg.FrameworkWatchPatterns)
	if cfg.Watch != nil {
		appendRules(cfg.Watch.Include)
	}
	return includeRules
}

// buildExcludeDirectoryPatterns builds normalized exclude dir patterns.
func buildExcludeDirectoryPatterns(cfg *wave.ParsedConfig) []string {
	patterns := make([]string, 0)

	appendDirectoryPattern := func(pattern string) {
		normalizedPattern := resolvePathOrPatternFromWatchRoot(cfg, pattern)
		if normalizedPattern == "" {
			return
		}
		patterns = append(patterns, normalizedPattern)
		if !strings.HasSuffix(normalizedPattern, "/**") {
			patterns = append(patterns, normalizedPattern+"/**")
		}
	}

	if cfg.Watch != nil {
		for _, pattern := range cfg.Watch.Exclude.Dirs {
			appendDirectoryPattern(pattern)
		}
	}
	for _, pattern := range cfg.FrameworkIgnoredPatterns {
		appendDirectoryPattern(pattern)
	}

	watchRoot := cfg.WatchRoot()
	if strings.TrimSpace(watchRoot) == "" {
		watchRoot = "."
	}
	patterns = append(
		patterns,
		resolvePathOrPatternFromWatchRoot(cfg, filepath.Join(watchRoot, ".git")),
		resolvePathOrPatternFromWatchRoot(
			cfg,
			filepath.Join(watchRoot, ".git", "**"),
		),
		resolvePathOrPatternFromWatchRoot(cfg, filepath.Join(watchRoot, "**", ".git")),
		resolvePathOrPatternFromWatchRoot(
			cfg,
			filepath.Join(watchRoot, "**", ".git", "**"),
		),
		resolvePathOrPatternFromWatchRoot(cfg, filepath.Join(watchRoot, "node_modules")),
		resolvePathOrPatternFromWatchRoot(
			cfg,
			filepath.Join(watchRoot, "node_modules", "**"),
		),
		resolvePathOrPatternFromWatchRoot(
			cfg,
			filepath.Join(watchRoot, "**", "node_modules"),
		),
		resolvePathOrPatternFromWatchRoot(
			cfg,
			filepath.Join(watchRoot, "**", "node_modules", "**"),
		),
	)

	return dedupeStrings(patterns)
}

// buildExcludeFilePatterns builds normalized exclude file patterns.
func buildExcludeFilePatterns(cfg *wave.ParsedConfig) []string {
	patterns := make([]string, 0)
	if cfg.Watch != nil {
		for _, pattern := range cfg.Watch.Exclude.Files {
			normalizedPattern := resolvePathOrPatternFromWatchRoot(cfg, pattern)
			if normalizedPattern != "" {
				patterns = append(patterns, normalizedPattern)
			}
		}
	}

	// Always ignore generated build outputs and lock files.
	patterns = append(
		patterns,
		normalizeGlob(filepath.Join(cfg.Dist.Static(), "**")),
		normalizeGlob(cfg.Dist.Binary()),
		normalizeGlob(
			filepath.Join(cfg.Dist.Static(), shared.LockFileName),
		),
	)

	return dedupeStrings(patterns)
}

func validateWatcherConfigPatterns(cfg *wave.ParsedConfig) error {
	if cfg == nil {
		return errors.New("watcher config is nil")
	}

	for frameworkWatchPatternIndex, watchedFile := range cfg.FrameworkWatchPatterns {
		if validatePatternError := validateWatcherGlobPatternInputForRuntime(
			fmt.Sprintf(
				"FrameworkWatchPatterns[%d].Pattern",
				frameworkWatchPatternIndex,
			),
			watchedFile.Pattern,
		); validatePatternError != nil {
			return validatePatternError
		}
		if validateHookExcludesError := validateOnChangeHookExcludePatterns(
			fmt.Sprintf(
				"FrameworkWatchPatterns[%d].OnChangeHooks",
				frameworkWatchPatternIndex,
			),
			watchedFile.OnChangeHooks,
		); validateHookExcludesError != nil {
			return validateHookExcludesError
		}
	}

	for frameworkIgnoredPatternIndex, frameworkIgnoredPattern := range cfg.FrameworkIgnoredPatterns {
		if validatePatternError := validateWatcherGlobPatternInputForRuntime(
			fmt.Sprintf(
				"FrameworkIgnoredPatterns[%d]",
				frameworkIgnoredPatternIndex,
			),
			frameworkIgnoredPattern,
		); validatePatternError != nil {
			return validatePatternError
		}
	}

	if cfg.Watch == nil {
		return nil
	}

	for watchIncludePatternIndex, watchedFile := range cfg.Watch.Include {
		if validatePatternError := validateWatcherGlobPatternInputForRuntime(
			fmt.Sprintf("Watch.Include[%d].Pattern", watchIncludePatternIndex),
			watchedFile.Pattern,
		); validatePatternError != nil {
			return validatePatternError
		}
		if validateHookExcludesError := validateOnChangeHookExcludePatterns(
			fmt.Sprintf("Watch.Include[%d].OnChangeHooks", watchIncludePatternIndex),
			watchedFile.OnChangeHooks,
		); validateHookExcludesError != nil {
			return validateHookExcludesError
		}
	}

	for excludedDirectoryPatternIndex, excludedDirectoryPattern := range cfg.Watch.Exclude.Dirs {
		if validatePatternError := validateWatcherGlobPatternInputForRuntime(
			fmt.Sprintf("Watch.Exclude.Dirs[%d]", excludedDirectoryPatternIndex),
			excludedDirectoryPattern,
		); validatePatternError != nil {
			return validatePatternError
		}
	}

	for excludedFilePatternIndex, excludedFilePattern := range cfg.Watch.Exclude.Files {
		if validatePatternError := validateWatcherGlobPatternInputForRuntime(
			fmt.Sprintf("Watch.Exclude.Files[%d]", excludedFilePatternIndex),
			excludedFilePattern,
		); validatePatternError != nil {
			return validatePatternError
		}
	}

	return nil
}

func validateOnChangeHookExcludePatterns(
	fieldPath string,
	onChangeHooks []wave.OnChangeHook,
) error {
	for hookIndex, onChangeHook := range onChangeHooks {
		for excludePatternIndex, excludePattern := range onChangeHook.Exclude {
			if validatePatternError := validateWatcherGlobPatternInputForRuntime(
				fmt.Sprintf(
					"%s[%d].Exclude[%d]",
					fieldPath,
					hookIndex,
					excludePatternIndex,
				),
				excludePattern,
			); validatePatternError != nil {
				return validatePatternError
			}
		}
	}
	return nil
}

func validateWatcherGlobPatternInputForRuntime(
	fieldPath string,
	pattern string,
) error {
	return shared.ValidateNamedGlobPatternInput(
		"watcher setup",
		fieldPath,
		pattern,
	)
}

func resolvePathOrPatternFromWatchRoot(
	cfg *wave.ParsedConfig,
	pathOrPattern string,
) string {
	if strings.TrimSpace(pathOrPattern) == "" {
		return ""
	}
	if filepath.IsAbs(pathOrPattern) {
		return normalizePath(pathOrPattern)
	}

	watchRoot := "."
	if cfg != nil {
		watchRoot = cfg.WatchRoot()
	}
	if strings.TrimSpace(watchRoot) == "" {
		watchRoot = "."
	}

	normalizedWatchRoot := normalizePath(watchRoot)
	normalizedJoinedPathOrPattern := normalizePath(
		filepath.Join(watchRoot, pathOrPattern),
	)
	normalizedWatchRootPrefix := normalizedWatchRoot + "/"
	escapedWatchRoot := escapePatternMetaCharactersForDoublestarPattern(normalizedWatchRoot)

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

// normalizePath returns cleaned slash-normalized path.
func normalizePath(path string) string {
	return wavecore.AbsoluteSlash(path)
}

// normalizeGlob returns slash-normalized glob with trimmed whitespace.
func normalizeGlob(globPattern string) string {
	if strings.TrimSpace(globPattern) == "" {
		return ""
	}
	return wavecore.AbsoluteSlash(globPattern)
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

// dedupeStrings removes duplicates while preserving first appearance order.
func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	deduped := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, alreadySeen := seen[value]; alreadySeen {
			continue
		}
		seen[value] = struct{}{}
		deduped = append(deduped, value)
	}
	return deduped
}

// isKnownNonWatchablePathError reports whether watcher.Add error is non-fatal.
func isKnownNonWatchablePathError(err error) bool {
	if err == nil {
		return false
	}
	errString := strings.ToLower(err.Error())
	if strings.Contains(errString, "no such file or directory") {
		return true
	}
	if strings.Contains(errString, "too many open files") {
		return false
	}
	if strings.Contains(errString, "not a directory") {
		return true
	}
	return false
}

// Debouncer buffers watcher events and emits them in deterministic batches.
type Debouncer struct {
	delay   time.Duration
	onFlush func([]fsnotify.Event)

	mu               sync.Mutex
	buffer           []fsnotify.Event
	timer            *time.Timer
	stopped          bool
	callbackInFlight bool
}

// NewDebouncer creates a debouncer with fixed flush delay.
func NewDebouncer(
	delay time.Duration,
	onFlush func([]fsnotify.Event),
) *Debouncer {
	if delay <= 0 {
		delay = 30 * time.Millisecond
	}
	if onFlush == nil {
		onFlush = func([]fsnotify.Event) {}
	}
	return &Debouncer{
		delay:   delay,
		onFlush: onFlush,
		buffer:  make([]fsnotify.Event, 0, 16),
	}
}

// Add appends an event and resets the flush timer.
func (debouncer *Debouncer) Add(event fsnotify.Event) {
	if debouncer == nil {
		return
	}

	debouncer.mu.Lock()
	defer debouncer.mu.Unlock()

	if debouncer.stopped {
		return
	}

	debouncer.buffer = append(debouncer.buffer, event)
	if debouncer.callbackInFlight {
		return
	}
	if debouncer.timer == nil {
		debouncer.timer = time.AfterFunc(debouncer.delay, debouncer.flush)
		return
	}
	debouncer.timer.Reset(debouncer.delay)
}

// Stop stops the debouncer and drops pending buffered events.
func (debouncer *Debouncer) Stop() {
	if debouncer == nil {
		return
	}

	debouncer.mu.Lock()
	if debouncer.stopped {
		debouncer.mu.Unlock()
		return
	}
	debouncer.stopped = true
	if debouncer.timer != nil {
		debouncer.timer.Stop()
	}
	debouncer.buffer = nil
	debouncer.mu.Unlock()
}

// flush drains buffered events and invokes callback.
func (debouncer *Debouncer) flush() {
	debouncer.mu.Lock()
	if debouncer.stopped {
		debouncer.timer = nil
		debouncer.mu.Unlock()
		return
	}
	if debouncer.callbackInFlight {
		debouncer.timer = time.AfterFunc(debouncer.delay, debouncer.flush)
		debouncer.mu.Unlock()
		return
	}
	batchedEvents := append([]fsnotify.Event(nil), debouncer.buffer...)
	debouncer.buffer = debouncer.buffer[:0]
	debouncer.callbackInFlight = true
	debouncer.timer = nil
	debouncer.mu.Unlock()

	if len(batchedEvents) == 0 {
		debouncer.mu.Lock()
		debouncer.callbackInFlight = false
		debouncer.mu.Unlock()
		return
	}
	debouncer.onFlush(batchedEvents)

	debouncer.mu.Lock()
	debouncer.callbackInFlight = false
	if !debouncer.stopped && len(debouncer.buffer) > 0 && debouncer.timer == nil {
		debouncer.timer = time.AfterFunc(debouncer.delay, debouncer.flush)
	}
	debouncer.mu.Unlock()
}

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
	"github.com/vormadev/vorma/wave/tooling_3/toolingshared"
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

// AddDirectoryRecursively adds a directory and all subdirectories to watcher.
func (watcher *Watcher) AddDirectoryRecursively(path string) error {
	if watcher == nil {
		return errors.New("watcher is nil")
	}
	return watcher.addDirectoryRecursively(path)
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
				return nil
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
		if toolingshared.MatchPathAgainstGlob(normalizedPath, pattern) {
			return true
		}
	}
	return false
}

// IsIgnoredFile reports whether a file path should be excluded from event processing.
func (watcher *Watcher) IsIgnoredFile(filePath string) bool {
	normalizedPath := normalizePath(filePath)
	if normalizedPath == "" {
		return true
	}

	if filepath.Base(normalizedPath) == toolingshared.LockFileName {
		return true
	}

	for _, pattern := range watcher.excludeFilePatterns {
		if toolingshared.MatchPathAgainstGlob(normalizedPath, pattern) {
			return true
		}
	}

	for _, directoryPattern := range watcher.excludeDirPatterns {
		if toolingshared.MatchPathAgainstGlob(
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

	for _, rule := range watcher.includeRules {
		if toolingshared.MatchPathAgainstGlob(normalizedPath, rule.pattern) {
			return rule.watchedFile
		}
	}
	return nil
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
			normalizedPattern := normalizeGlob(watchedFile.Pattern)
			if normalizedPattern == "" {
				continue
			}
			watchedFileCopy := watchedFile
			includeRules = append(includeRules, includeRule{
				pattern:     normalizedPattern,
				watchedFile: &watchedFileCopy,
			})
		}
	}

	if cfg.Watch != nil {
		appendRules(cfg.Watch.Include)
	}
	appendRules(cfg.FrameworkWatchPatterns)
	return includeRules
}

// buildExcludeDirectoryPatterns builds normalized exclude dir patterns.
func buildExcludeDirectoryPatterns(cfg *wave.ParsedConfig) []string {
	patterns := make([]string, 0)

	if cfg.Watch != nil {
		for _, pattern := range cfg.Watch.Exclude.Dirs {
			normalizedPattern := normalizeGlob(pattern)
			if normalizedPattern != "" {
				patterns = append(patterns, normalizedPattern)
			}
		}
	}
	for _, pattern := range cfg.FrameworkIgnoredPatterns {
		normalizedPattern := normalizeGlob(pattern)
		if normalizedPattern != "" {
			patterns = append(patterns, normalizedPattern)
		}
	}

	return dedupeStrings(patterns)
}

// buildExcludeFilePatterns builds normalized exclude file patterns.
func buildExcludeFilePatterns(cfg *wave.ParsedConfig) []string {
	patterns := make([]string, 0)
	if cfg.Watch != nil {
		for _, pattern := range cfg.Watch.Exclude.Files {
			normalizedPattern := normalizeGlob(pattern)
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
			filepath.Join(cfg.Dist.Static(), toolingshared.LockFileName),
		),
	)

	return dedupeStrings(patterns)
}

// normalizePath returns cleaned slash-normalized path.
func normalizePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	cleanedPath := filepath.Clean(path)
	return strings.ReplaceAll(cleanedPath, "\\", "/")
}

// normalizeGlob returns slash-normalized glob with trimmed whitespace.
func normalizeGlob(globPattern string) string {
	if strings.TrimSpace(globPattern) == "" {
		return ""
	}
	return strings.ReplaceAll(strings.TrimSpace(globPattern), "\\", "/")
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
	if strings.Contains(errString, "permission denied") {
		return true
	}
	return false
}

// Debouncer buffers watcher events and emits them in deterministic batches.
type Debouncer struct {
	delay   time.Duration
	onFlush func([]fsnotify.Event)

	mu      sync.Mutex
	buffer  []fsnotify.Event
	timer   *time.Timer
	stopped bool
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
	if debouncer.timer == nil {
		debouncer.timer = time.AfterFunc(debouncer.delay, debouncer.flush)
		return
	}
	debouncer.timer.Reset(debouncer.delay)
}

// Stop stops the debouncer and flushes remaining buffered events.
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
	pendingEvents := append([]fsnotify.Event(nil), debouncer.buffer...)
	debouncer.buffer = nil
	debouncer.mu.Unlock()

	if len(pendingEvents) > 0 {
		debouncer.onFlush(pendingEvents)
	}
}

// flush drains buffered events and invokes callback.
func (debouncer *Debouncer) flush() {
	debouncer.mu.Lock()
	if debouncer.stopped {
		debouncer.mu.Unlock()
		return
	}
	batchedEvents := append([]fsnotify.Event(nil), debouncer.buffer...)
	debouncer.buffer = debouncer.buffer[:0]
	debouncer.mu.Unlock()

	if len(batchedEvents) == 0 {
		return
	}
	debouncer.onFlush(batchedEvents)
}

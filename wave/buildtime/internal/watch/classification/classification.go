// Package classification maps changed filesystem paths to semantic watcher file
// types.
//
// This package exists so event planning can operate on stable categories rather
// than brittle raw path checks spread throughout dev-server code.
package classification

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch/dedup"
	"github.com/vormadev/vorma/wave/waveenv"
)

// EventKind is a normalized watcher event class independent of fsnotify bitmasks.
type EventKind int

const (
	// EventKindUnknown is used when no explicit fsnotify operation bit is set.
	EventKindUnknown EventKind = iota
	// EventKindCreate represents filesystem create events.
	EventKindCreate
	// EventKindWrite represents content writes.
	EventKindWrite
	// EventKindRemove represents file removal.
	EventKindRemove
	// EventKindRename represents path renames.
	EventKindRename
	// EventKindChmod represents chmod-only metadata updates.
	EventKindChmod
)

// PreClassificationDecision controls whether an event enters semantic planning.
type PreClassificationDecision struct {
	IncludeEvent     bool
	IgnoreReason     string
	CanonicalPath    string
	ShouldSkipStat   bool
	LooksLikeTempIO  bool
	LooksLikeLockIO  bool
	UnderlyingKind   EventKind
	FromUnknownEvent bool
}

// PostClassificationDecision controls final inclusion after semantic classification.
type PostClassificationDecision struct {
	IncludeClassifiedEvent bool
	DroppedBecauseIgnored  bool
	DroppedBecauseChmod    bool
}

// PathClassifierDependencies groups callback dependencies for pre-classification.
type PathClassifierDependencies struct {
	IsIgnoredPathFunc func(string) bool
	LockFileName      string
}

// DeriveEventKind converts fsnotify operation flags into one deterministic kind.
func DeriveEventKind(watcherEvent fsnotify.Event) EventKind {
	if watcherEvent.Has(fsnotify.Create) {
		return EventKindCreate
	}
	if watcherEvent.Has(fsnotify.Write) {
		return EventKindWrite
	}
	if watcherEvent.Has(fsnotify.Remove) {
		return EventKindRemove
	}
	if watcherEvent.Has(fsnotify.Rename) {
		return EventKindRename
	}
	if watcherEvent.Has(fsnotify.Chmod) {
		return EventKindChmod
	}
	return EventKindUnknown
}

// NormalizeWatcherEventPath normalizes path shape for consistent matching.
func NormalizeWatcherEventPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	clean := filepath.Clean(path)
	if clean == "." {
		return ""
	}
	return clean
}

// DerivePreClassificationDecision performs cheap filtering before semantic work.
func DerivePreClassificationDecision(
	watcherEvent fsnotify.Event,
	dependencies PathClassifierDependencies,
) PreClassificationDecision {
	normalizedPath := NormalizeWatcherEventPath(watcherEvent.Name)
	kind := DeriveEventKind(watcherEvent)

	decision := PreClassificationDecision{
		IncludeEvent:   true,
		CanonicalPath:  normalizedPath,
		UnderlyingKind: kind,
	}

	if normalizedPath == "" {
		decision.IncludeEvent = false
		decision.IgnoreReason = "empty_path"
		return decision
	}

	if strings.TrimSpace(dependencies.LockFileName) != "" &&
		filepath.Base(normalizedPath) == dependencies.LockFileName {
		decision.IncludeEvent = false
		decision.IgnoreReason = "lock_file"
		decision.LooksLikeLockIO = true
		return decision
	}

	if IsLikelyEditorTemporaryPath(normalizedPath) {
		decision.IncludeEvent = false
		decision.IgnoreReason = "editor_temp_path"
		decision.LooksLikeTempIO = true
		return decision
	}

	if dependencies.IsIgnoredPathFunc != nil &&
		dependencies.IsIgnoredPathFunc(normalizedPath) {
		decision.IncludeEvent = false
		decision.IgnoreReason = "ignored_path"
		return decision
	}

	// Existing directory events are watcher-maintenance noise. Keep only
	// file-path semantics in the classification pipeline.
	if shouldIgnoreDirectoryEvent(normalizedPath, kind) {
		decision.IncludeEvent = false
		decision.IgnoreReason = "directory_event"
		return decision
	}

	if kind == EventKindUnknown {
		decision.FromUnknownEvent = true
		decision.ShouldSkipStat = true
	}

	return decision
}

// DerivePostClassificationDecision removes ignored and chmod-only events.
func DerivePostClassificationDecision(
	ignored bool,
	chmodOnly bool,
) PostClassificationDecision {
	if ignored {
		return PostClassificationDecision{
			IncludeClassifiedEvent: false,
			DroppedBecauseIgnored:  true,
		}
	}
	if chmodOnly {
		return PostClassificationDecision{
			IncludeClassifiedEvent: false,
			DroppedBecauseChmod:    true,
		}
	}
	return PostClassificationDecision{IncludeClassifiedEvent: true}
}

// IsLikelyEditorTemporaryPath catches common editor temp/backup artifacts.
func IsLikelyEditorTemporaryPath(path string) bool {
	base := filepath.Base(path)
	if base == "" {
		return false
	}

	lowerBase := strings.ToLower(base)
	if strings.HasPrefix(lowerBase, ".#") {
		return true
	}
	if strings.HasSuffix(lowerBase, "~") {
		return true
	}
	if strings.HasSuffix(lowerBase, ".tmp") {
		return true
	}
	if strings.HasSuffix(lowerBase, ".swp") ||
		strings.HasSuffix(lowerBase, ".swo") {
		return true
	}
	if strings.HasPrefix(lowerBase, "4913") {
		return true
	}
	return false
}

// DeriveChmodOnlyDecision reports whether a chmod event is pure metadata noise.
func DeriveChmodOnlyDecision(watcherEvent fsnotify.Event) bool {
	return IsNonEmptyChmodOnly(watcherEvent)
}

// IsNonEmptyChmodOnly reports whether an event is chmod-only on a non-empty file.
func IsNonEmptyChmodOnly(watcherEvent fsnotify.Event) bool {
	if !isChmodOnlyOperation(watcherEvent) {
		return false
	}
	fileInfo, statError := os.Stat(watcherEvent.Name)
	if statError != nil {
		return false
	}
	return fileInfo.Size() > 0
}

func isChmodOnlyOperation(watcherEvent fsnotify.Event) bool {
	if !watcherEvent.Has(fsnotify.Chmod) {
		return false
	}
	if watcherEvent.Has(fsnotify.Write) || watcherEvent.Has(fsnotify.Create) {
		return false
	}
	if watcherEvent.Has(fsnotify.Remove) || watcherEvent.Has(fsnotify.Rename) {
		return false
	}
	return true
}

// ShouldSuppressEventForMissingPath returns true for remove/rename path misses.
func ShouldSuppressEventForMissingPath(
	watcherEvent fsnotify.Event,
	statError error,
) bool {
	if statError == nil {
		return false
	}
	if !errors.Is(statError, os.ErrNotExist) {
		return false
	}
	if watcherEvent.Has(fsnotify.Remove) {
		return true
	}
	if watcherEvent.Has(fsnotify.Rename) {
		return true
	}
	return false
}

// IsLikelyDirectoryChange reports whether path currently resolves to a directory.
func IsLikelyDirectoryChange(path string) bool {
	fileInfo, statError := os.Stat(path)
	if statError != nil {
		return false
	}
	return fileInfo.IsDir()
}

func shouldIgnoreDirectoryEvent(path string, eventKind EventKind) bool {
	if !IsLikelyDirectoryChange(path) {
		return false
	}
	if eventKind != EventKindCreate {
		return true
	}

	// Keep create events for non-empty directories so subtree moves/renames can
	// be reconciled even when the watcher emits only directory-level events.
	directoryEntries, readDirectoryError := os.ReadDir(path)
	if readDirectoryError != nil {
		return false
	}
	return len(directoryEntries) == 0
}

// IsSymlinkPath reports whether a path is a symbolic link.
func IsSymlinkPath(path string) bool {
	fileInfo, lstatError := os.Lstat(path)
	if lstatError != nil {
		return false
	}
	return fileInfo.Mode()&os.ModeSymlink != 0
}

// ResolveSymlinkIfPresent resolves symlink targets and returns cleaned path.
func ResolveSymlinkIfPresent(path string) string {
	normalizedPath := NormalizeWatcherEventPath(path)
	if normalizedPath == "" {
		return ""
	}
	resolvedPath, evalError := filepath.EvalSymlinks(normalizedPath)
	if evalError != nil {
		return normalizedPath
	}
	return NormalizeWatcherEventPath(resolvedPath)
}

// CanonicalizePathForEvent attempts best-effort absolute path canonicalization.
func CanonicalizePathForEvent(path string) string {
	normalizedPath := NormalizeWatcherEventPath(path)
	if normalizedPath == "" {
		return ""
	}

	absolutePath, absError := filepath.Abs(normalizedPath)
	if absError != nil {
		return ResolveSymlinkIfPresent(normalizedPath)
	}
	return ResolveSymlinkIfPresent(absolutePath)
}

// IsConfigurationPathChange reports whether watcher path targets config file.
func IsConfigurationPathChange(
	watcherPath string,
	configurationPath string,
) bool {
	normalizedWatcherPath := waveenv.Absolute(watcherPath)
	normalizedConfigurationPath := waveenv.Absolute(configurationPath)
	if normalizedWatcherPath == "" || normalizedConfigurationPath == "" {
		return false
	}
	if waveenv.PathsReferToSameLocation(
		normalizedWatcherPath,
		normalizedConfigurationPath,
	) {
		return true
	}

	watcherMissingAliasKey := dedup.MissingFileAliasKeyForWatcherEventDeduplication(
		normalizedWatcherPath,
	)
	configurationMissingAliasKey := dedup.MissingFileAliasKeyForWatcherEventDeduplication(
		normalizedConfigurationPath,
	)
	return watcherMissingAliasKey != "" &&
		watcherMissingAliasKey == configurationMissingAliasKey
}

// EventKindString maps EventKind to stable log values.
func EventKindString(kind EventKind) string {
	switch kind {
	case EventKindCreate:
		return "create"
	case EventKindWrite:
		return "write"
	case EventKindRemove:
		return "remove"
	case EventKindRename:
		return "rename"
	case EventKindChmod:
		return "chmod"
	default:
		return "unknown"
	}
}

// ShouldLogWatcherAddDirectoryError suppresses expected add-directory failures.
func ShouldLogWatcherAddDirectoryError(addDirectoryWatchError error) bool {
	if addDirectoryWatchError == nil {
		return false
	}
	if os.IsNotExist(addDirectoryWatchError) ||
		errors.Is(addDirectoryWatchError, fs.ErrNotExist) {
		return false
	}
	if errors.Is(addDirectoryWatchError, syscall.ENOTDIR) {
		return false
	}
	return true
}

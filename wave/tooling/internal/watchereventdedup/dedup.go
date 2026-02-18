// Package watchereventdedup deduplicates watcher events by normalized path
// while preserving operation unions across aliasing path shapes.
//
// Why this package exists:
// fsnotify can emit repeated events for one file and can emit multiple path
// representations for one location (for example, symlink aliases or cleaned vs
// uncleaned paths). Without one dedup boundary, downstream phases can run
// duplicate hooks/build work, produce noisy logs, and apply inconsistent
// restart/reload decisions.
package watchereventdedup

import (
	"path/filepath"
	"sort"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

// DeduplicateWatcherEventsByPath merges watcher events by resolved path key and
// returns deterministic path-sorted output.
func DeduplicateWatcherEventsByPath(
	events []fsnotify.Event,
) []fsnotify.Event {
	if len(events) == 0 {
		return nil
	}

	mergedEventOpsByPath := make(map[string]fsnotify.Op, len(events))
	deduplicationIndex := NewWatcherEventDeduplicationIndex()
	for _, event := range events {
		eventPathKey := NormalizeWatcherEventPathForDeduplication(event.Name)
		if existingPathKey := deduplicationIndex.ResolveExistingPathKey(
			mergedEventOpsByPath,
			eventPathKey,
		); existingPathKey != "" {
			eventPathKey = existingPathKey
		} else {
			deduplicationIndex.RecordPathKey(eventPathKey)
		}
		mergedEventOpsByPath[eventPathKey] |= event.Op
	}

	deduplicatedPaths := make([]string, 0, len(mergedEventOpsByPath))
	for eventPath := range mergedEventOpsByPath {
		deduplicatedPaths = append(deduplicatedPaths, eventPath)
	}
	sort.Strings(deduplicatedPaths)

	deduplicatedEvents := make([]fsnotify.Event, 0, len(deduplicatedPaths))
	for _, deduplicatedPath := range deduplicatedPaths {
		deduplicatedEvents = append(deduplicatedEvents, fsnotify.Event{
			Name: deduplicatedPath,
			Op:   mergedEventOpsByPath[deduplicatedPath],
		})
	}

	return deduplicatedEvents
}

// WatcherEventDeduplicationIndex caches canonical and alias probe resolution
// for one deduplication pass.
type WatcherEventDeduplicationIndex struct {
	// CanonicalPathToPathKey maps canonical absolute paths to first-seen keys.
	CanonicalPathToPathKey map[string]string
	// MissingFileAliasKeyToPathKey maps missing-file alias keys to first-seen
	// path keys.
	MissingFileAliasKeyToPathKey map[string]string
	// CanonicalPathByAbsolutePath memoizes canonicalization per absolute path.
	CanonicalPathByAbsolutePath map[string]string
	// MissingAliasKeyByAbsolutePath memoizes missing-file alias keys per path.
	MissingAliasKeyByAbsolutePath map[string]string
}

// NewWatcherEventDeduplicationIndex returns an empty dedup probe index.
func NewWatcherEventDeduplicationIndex() *WatcherEventDeduplicationIndex {
	return &WatcherEventDeduplicationIndex{
		CanonicalPathToPathKey:        make(map[string]string),
		MissingFileAliasKeyToPathKey:  make(map[string]string),
		CanonicalPathByAbsolutePath:   make(map[string]string),
		MissingAliasKeyByAbsolutePath: make(map[string]string),
	}
}

// ResolveExistingPathKey returns an existing path key for normalizedEventPath
// when canonical or alias probes identify a previously-seen key.
func (index *WatcherEventDeduplicationIndex) ResolveExistingPathKey(
	mergedEventOpsByPath map[string]fsnotify.Op,
	normalizedEventPath string,
) string {
	if normalizedEventPath == "" {
		return ""
	}

	if _, alreadyExists := mergedEventOpsByPath[normalizedEventPath]; alreadyExists {
		return normalizedEventPath
	}

	if !IsAbsolutePathForWatcherEventDeduplication(normalizedEventPath) {
		return ""
	}

	return index.resolveExistingPathKeyFromCanonicalAndMissingAliasProbes(
		normalizedEventPath,
	)
}

// RecordPathKey records pathKey for canonical and alias probes on first-seen
// basis.
func (index *WatcherEventDeduplicationIndex) RecordPathKey(pathKey string) {
	if !IsAbsolutePathForWatcherEventDeduplication(pathKey) {
		return
	}

	index.recordPathKeyForCanonicalAndMissingAliasProbes(pathKey)
}

// MissingAliasKeyResolutionEmptySentinel caches negative alias-key probes.
const MissingAliasKeyResolutionEmptySentinel = "\x00"

// ResolveCanonicalPathForAbsolutePath canonicalizes and memoizes absolutePath.
func (index *WatcherEventDeduplicationIndex) ResolveCanonicalPathForAbsolutePath(
	absolutePath string,
) string {
	if !IsAbsolutePathForWatcherEventDeduplication(absolutePath) {
		return ""
	}

	if canonicalPath, exists := index.CanonicalPathByAbsolutePath[absolutePath]; exists {
		return canonicalPath
	}

	canonicalPath := canonicalizeAbsolutePathForWatcherEventDeduplication(absolutePath)
	if canonicalPath == "" {
		return ""
	}
	index.CanonicalPathByAbsolutePath[absolutePath] = canonicalPath
	return canonicalPath
}

// ResolveMissingAliasKeyForAbsolutePath resolves and memoizes missing-file
// alias keys for absolutePath.
func (index *WatcherEventDeduplicationIndex) ResolveMissingAliasKeyForAbsolutePath(
	absolutePath string,
) string {
	if !IsAbsolutePathForWatcherEventDeduplication(absolutePath) {
		return ""
	}

	if cachedAliasKey, exists := index.MissingAliasKeyByAbsolutePath[absolutePath]; exists {
		if cachedAliasKey == MissingAliasKeyResolutionEmptySentinel {
			return ""
		}
		return cachedAliasKey
	}

	missingAliasKey := MissingFileAliasKeyForWatcherEventDeduplication(absolutePath)
	if missingAliasKey == "" {
		index.MissingAliasKeyByAbsolutePath[absolutePath] = MissingAliasKeyResolutionEmptySentinel
		return ""
	}

	index.MissingAliasKeyByAbsolutePath[absolutePath] = missingAliasKey
	return missingAliasKey
}

// resolveExistingPathKeyFromCanonicalAndMissingAliasProbes checks canonical and
// missing-file alias probes in deterministic order.
func (index *WatcherEventDeduplicationIndex) resolveExistingPathKeyFromCanonicalAndMissingAliasProbes(
	absolutePath string,
) string {
	if !IsAbsolutePathForWatcherEventDeduplication(absolutePath) {
		return ""
	}

	if existingPathKey := index.resolveExistingPathKeyFromCanonicalPathProbe(absolutePath); existingPathKey != "" {
		return existingPathKey
	}

	return index.resolveExistingPathKeyFromMissingAliasProbe(absolutePath)
}

// resolveExistingPathKeyFromCanonicalPathProbe resolves a path key from the
// canonical path probe lookup.
func (index *WatcherEventDeduplicationIndex) resolveExistingPathKeyFromCanonicalPathProbe(
	absolutePath string,
) string {
	canonicalPath := index.ResolveCanonicalPathForAbsolutePath(absolutePath)
	return lookupFirstSeenPathKeyByProbeKey(
		index.CanonicalPathToPathKey,
		canonicalPath,
	)
}

// resolveExistingPathKeyFromMissingAliasProbe resolves a path key from the
// missing-file alias probe lookup.
func (index *WatcherEventDeduplicationIndex) resolveExistingPathKeyFromMissingAliasProbe(
	absolutePath string,
) string {
	missingAliasKey := index.ResolveMissingAliasKeyForAbsolutePath(absolutePath)
	return lookupFirstSeenPathKeyByProbeKey(
		index.MissingFileAliasKeyToPathKey,
		missingAliasKey,
	)
}

// recordPathKeyForCanonicalAndMissingAliasProbes records first-seen mappings
// for canonical and missing-file alias probe lookups.
func (index *WatcherEventDeduplicationIndex) recordPathKeyForCanonicalAndMissingAliasProbes(
	pathKey string,
) {
	if !IsAbsolutePathForWatcherEventDeduplication(pathKey) {
		return
	}

	canonicalPath := index.ResolveCanonicalPathForAbsolutePath(pathKey)
	recordFirstSeenPathKeyByProbeKeyIfMissing(
		index.CanonicalPathToPathKey,
		canonicalPath,
		pathKey,
	)

	missingAliasKey := index.ResolveMissingAliasKeyForAbsolutePath(pathKey)
	recordFirstSeenPathKeyByProbeKeyIfMissing(
		index.MissingFileAliasKeyToPathKey,
		missingAliasKey,
		pathKey,
	)
}

// NormalizeWatcherEventPathForDeduplication trims and cleans path shapes used
// as primary dedup keys.
func NormalizeWatcherEventPathForDeduplication(
	eventPath string,
) string {
	cleanedEventPath := pathnorm.TrimAndCleanPath(eventPath)
	if cleanedEventPath == "" {
		return ""
	}

	if IsAbsolutePathForWatcherEventDeduplication(cleanedEventPath) {
		return pathnorm.Absolute(cleanedEventPath)
	}

	return cleanedEventPath
}

// canonicalizeAbsolutePathForWatcherEventDeduplication resolves a canonical path
// identity for location comparisons.
func canonicalizeAbsolutePathForWatcherEventDeduplication(
	absolutePath string,
) string {
	return pathnorm.CanonicalizePathForLocationComparison(absolutePath)
}

// MissingFileAliasKeyForWatcherEventDeduplication resolves a stable alias key
// for missing absolute file paths (canonical-parent + base-name).
func MissingFileAliasKeyForWatcherEventDeduplication(
	absolutePath string,
) string {
	if !IsAbsolutePathForWatcherEventDeduplication(absolutePath) {
		return ""
	}

	baseName := filepath.Base(absolutePath)
	if baseName == "" || baseName == "." || baseName == string(filepath.Separator) {
		return ""
	}

	parentDirectoryPath := pathnorm.AbsoluteDirectory(absolutePath)
	if parentDirectoryPath == "" {
		return ""
	}
	canonicalParentDirectoryPath := canonicalizeAbsolutePathForWatcherEventDeduplication(
		parentDirectoryPath,
	)
	if canonicalParentDirectoryPath == "" {
		return ""
	}

	return canonicalParentDirectoryPath + "\x00" + baseName
}

// IsAbsolutePathForWatcherEventDeduplication reports whether path is an
// absolute non-empty path.
func IsAbsolutePathForWatcherEventDeduplication(path string) bool {
	return path != "" && filepath.IsAbs(path)
}

// lookupFirstSeenPathKeyByProbeKey reads from one probe lookup map
// (canonical-path or missing-alias lookup) and returns the first-seen path key
// associated with probeKey.
func lookupFirstSeenPathKeyByProbeKey(
	firstSeenPathKeyByProbeKey map[string]string,
	probeKey string,
) string {
	if firstSeenPathKeyByProbeKey == nil || probeKey == "" {
		return ""
	}
	return firstSeenPathKeyByProbeKey[probeKey]
}

// recordFirstSeenPathKeyByProbeKeyIfMissing preserves the first-seen path key
// for a probeKey. This prevents later alias paths from replacing the earlier
// path-key label and keeps dedup output stable for one watcher batch.
func recordFirstSeenPathKeyByProbeKeyIfMissing(
	firstSeenPathKeyByProbeKey map[string]string,
	probeKey string,
	pathKey string,
) {
	if firstSeenPathKeyByProbeKey == nil || probeKey == "" || pathKey == "" {
		return
	}
	if _, exists := firstSeenPathKeyByProbeKey[probeKey]; exists {
		return
	}
	firstSeenPathKeyByProbeKey[probeKey] = pathKey
}

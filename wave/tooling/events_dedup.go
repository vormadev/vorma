package tooling

import (
	"path/filepath"
	"sort"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

func deduplicateWatcherEventsByPath(
	events []fsnotify.Event,
) []fsnotify.Event {
	if len(events) == 0 {
		return nil
	}

	mergedEventOpsByPath := make(map[string]fsnotify.Op, len(events))
	deduplicationIndex := newWatcherEventDeduplicationIndex()
	for _, event := range events {
		eventPathKey := normalizeWatcherEventPathForDeduplication(event.Name)
		if existingPathKey := deduplicationIndex.resolveExistingPathKey(
			mergedEventOpsByPath,
			eventPathKey,
		); existingPathKey != "" {
			eventPathKey = existingPathKey
		} else {
			deduplicationIndex.recordPathKey(eventPathKey)
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

type watcherEventDeduplicationIndex struct {
	canonicalPathToPathKey        map[string]string
	missingFileAliasKeyToPathKey  map[string]string
	canonicalPathByAbsolutePath   map[string]string
	missingAliasKeyByAbsolutePath map[string]string
}

func newWatcherEventDeduplicationIndex() *watcherEventDeduplicationIndex {
	return &watcherEventDeduplicationIndex{
		canonicalPathToPathKey:        make(map[string]string),
		missingFileAliasKeyToPathKey:  make(map[string]string),
		canonicalPathByAbsolutePath:   make(map[string]string),
		missingAliasKeyByAbsolutePath: make(map[string]string),
	}
}

func (index *watcherEventDeduplicationIndex) resolveExistingPathKey(
	mergedEventOpsByPath map[string]fsnotify.Op,
	normalizedEventPath string,
) string {
	if normalizedEventPath == "" {
		return ""
	}

	if _, alreadyExists := mergedEventOpsByPath[normalizedEventPath]; alreadyExists {
		return normalizedEventPath
	}

	if !isAbsolutePathForWatcherEventDeduplication(normalizedEventPath) {
		return ""
	}

	return index.resolveExistingPathKeyFromCanonicalAndMissingAliasProbes(
		normalizedEventPath,
	)
}

func (index *watcherEventDeduplicationIndex) recordPathKey(pathKey string) {
	if !isAbsolutePathForWatcherEventDeduplication(pathKey) {
		return
	}

	index.recordPathKeyForCanonicalAndMissingAliasProbes(pathKey)
}

const missingAliasKeyResolutionEmptySentinel = "\x00"

func (index *watcherEventDeduplicationIndex) resolveCanonicalPathForAbsolutePath(
	absolutePath string,
) string {
	if !isAbsolutePathForWatcherEventDeduplication(absolutePath) {
		return ""
	}

	if canonicalPath, exists := index.canonicalPathByAbsolutePath[absolutePath]; exists {
		return canonicalPath
	}

	canonicalPath := canonicalizeAbsolutePathForWatcherEventDeduplication(absolutePath)
	if canonicalPath == "" {
		return ""
	}
	index.canonicalPathByAbsolutePath[absolutePath] = canonicalPath
	return canonicalPath
}

func (index *watcherEventDeduplicationIndex) resolveMissingAliasKeyForAbsolutePath(
	absolutePath string,
) string {
	if !isAbsolutePathForWatcherEventDeduplication(absolutePath) {
		return ""
	}

	if cachedAliasKey, exists := index.missingAliasKeyByAbsolutePath[absolutePath]; exists {
		if cachedAliasKey == missingAliasKeyResolutionEmptySentinel {
			return ""
		}
		return cachedAliasKey
	}

	missingAliasKey := missingFileAliasKeyForWatcherEventDeduplication(absolutePath)
	if missingAliasKey == "" {
		index.missingAliasKeyByAbsolutePath[absolutePath] = missingAliasKeyResolutionEmptySentinel
		return ""
	}

	index.missingAliasKeyByAbsolutePath[absolutePath] = missingAliasKey
	return missingAliasKey
}

func (index *watcherEventDeduplicationIndex) resolveExistingPathKeyFromCanonicalAndMissingAliasProbes(
	absolutePath string,
) string {
	if !isAbsolutePathForWatcherEventDeduplication(absolutePath) {
		return ""
	}

	if existingPathKey := index.resolveExistingPathKeyFromCanonicalPathProbe(absolutePath); existingPathKey != "" {
		return existingPathKey
	}

	return index.resolveExistingPathKeyFromMissingAliasProbe(absolutePath)
}

func (index *watcherEventDeduplicationIndex) resolveExistingPathKeyFromCanonicalPathProbe(
	absolutePath string,
) string {
	canonicalPath := index.resolveCanonicalPathForAbsolutePath(absolutePath)
	return resolveExistingPathKeyFromProbeLookup(
		index.canonicalPathToPathKey,
		canonicalPath,
	)
}

func (index *watcherEventDeduplicationIndex) resolveExistingPathKeyFromMissingAliasProbe(
	absolutePath string,
) string {
	missingAliasKey := index.resolveMissingAliasKeyForAbsolutePath(absolutePath)
	return resolveExistingPathKeyFromProbeLookup(
		index.missingFileAliasKeyToPathKey,
		missingAliasKey,
	)
}

func (index *watcherEventDeduplicationIndex) recordPathKeyForCanonicalAndMissingAliasProbes(
	pathKey string,
) {
	if !isAbsolutePathForWatcherEventDeduplication(pathKey) {
		return
	}

	canonicalPath := index.resolveCanonicalPathForAbsolutePath(pathKey)
	recordPathKeyForProbeLookupIfAbsent(
		index.canonicalPathToPathKey,
		canonicalPath,
		pathKey,
	)

	missingAliasKey := index.resolveMissingAliasKeyForAbsolutePath(pathKey)
	recordPathKeyForProbeLookupIfAbsent(
		index.missingFileAliasKeyToPathKey,
		missingAliasKey,
		pathKey,
	)
}

func resolveExistingPathKeyFromProbeLookup(
	pathKeyByProbe map[string]string,
	probeKey string,
) string {
	if pathKeyByProbe == nil || probeKey == "" {
		return ""
	}
	return pathKeyByProbe[probeKey]
}

func recordPathKeyForProbeLookupIfAbsent(
	pathKeyByProbe map[string]string,
	probeKey string,
	pathKey string,
) {
	if pathKeyByProbe == nil || probeKey == "" || pathKey == "" {
		return
	}
	if _, exists := pathKeyByProbe[probeKey]; exists {
		return
	}
	pathKeyByProbe[probeKey] = pathKey
}

func normalizeWatcherEventPathForDeduplication(
	eventPath string,
) string {
	cleanedEventPath := pathnorm.TrimAndCleanPath(eventPath)
	if cleanedEventPath == "" {
		return ""
	}

	if isAbsolutePathForWatcherEventDeduplication(cleanedEventPath) {
		return pathnorm.Absolute(cleanedEventPath)
	}

	return cleanedEventPath
}

func canonicalizeAbsolutePathForWatcherEventDeduplication(
	absolutePath string,
) string {
	return pathnorm.CanonicalizePathForLocationComparison(absolutePath)
}

func missingFileAliasKeyForWatcherEventDeduplication(
	absolutePath string,
) string {
	if !isAbsolutePathForWatcherEventDeduplication(absolutePath) {
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

func isAbsolutePathForWatcherEventDeduplication(path string) bool {
	return path != "" && filepath.IsAbs(path)
}

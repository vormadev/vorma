package tooling

import "github.com/fsnotify/fsnotify"

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

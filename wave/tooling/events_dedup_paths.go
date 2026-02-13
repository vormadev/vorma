package tooling

import (
	"path/filepath"

	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

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

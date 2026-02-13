package tooling

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/vormadev/vorma/wave"
)

func cleanupStaleStaticDistFiles(
	distDirectoryPath string,
	oldMap wave.FileMap,
	newMap wave.FileMap,
) {
	for key, oldVal := range oldMap {
		newVal, exists := newMap[key]
		if !exists || newVal.DistName != oldVal.DistName {
			removeStaticDistArtifactIfPresent(distDirectoryPath, oldVal.DistName)
		}
	}
}

func removeStaticDistArtifactIfPresent(
	distDirectoryPath string,
	distName string,
) {
	_ = os.Remove(filepath.Join(distDirectoryPath, distName))
}

func removeStaticMapEntriesForChangedRelativePaths(
	staticMap wave.FileMap,
	distDirectoryPath string,
	changedRelativePaths []string,
) bool {
	if len(changedRelativePaths) == 0 || len(staticMap) == 0 {
		return false
	}

	changedRelativePathSet := make(map[string]struct{}, len(changedRelativePaths))
	for _, changedRelativePath := range changedRelativePaths {
		if changedRelativePath == "" {
			continue
		}
		changedRelativePathSet[changedRelativePath] = struct{}{}
	}
	if len(changedRelativePathSet) == 0 {
		return false
	}

	pathsToDelete := make([]string, 0)
	for existingRelativePath := range staticMap {
		if shouldRemoveStaticMapEntryForChangedRelativePathSet(
			existingRelativePath,
			changedRelativePathSet,
		) {
			pathsToDelete = append(pathsToDelete, existingRelativePath)
		}
	}

	if len(pathsToDelete) == 0 {
		return false
	}

	sort.Strings(pathsToDelete)

	for _, relativePathToDelete := range pathsToDelete {
		oldValueForPath := staticMap[relativePathToDelete]
		delete(staticMap, relativePathToDelete)
		removeStaticDistArtifactIfPresent(distDirectoryPath, oldValueForPath.DistName)
	}

	return true
}

func shouldRemoveStaticMapEntryForChangedRelativePathSet(
	existingRelativePath string,
	changedRelativePathSet map[string]struct{},
) bool {
	if _, hasExactMatch := changedRelativePathSet[existingRelativePath]; hasExactMatch {
		return true
	}

	for index, character := range existingRelativePath {
		if character != '/' {
			continue
		}

		if _, hasParentDirectoryMatch := changedRelativePathSet[existingRelativePath[:index]]; hasParentDirectoryMatch {
			return true
		}
	}

	return false
}

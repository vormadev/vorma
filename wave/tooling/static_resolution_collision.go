package tooling

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

func ensureNoStaticLogicalPathCollision(
	existingFileInfo fileInfo,
	candidateFileInfo fileInfo,
) error {
	if existingFileInfo.relPath != candidateFileInfo.relPath {
		return nil
	}

	if pathnorm.PathsReferToSameLocation(existingFileInfo.srcPath, candidateFileInfo.srcPath) {
		return nil
	}

	conflictingSourcePaths := []string{
		pathnorm.Absolute(existingFileInfo.srcPath),
		pathnorm.Absolute(candidateFileInfo.srcPath),
	}
	sort.Strings(conflictingSourcePaths)

	return fmt.Errorf(
		"static source path collision for logical path %q: %q and %q both map to the same output; keep exactly one source file",
		existingFileInfo.relPath,
		conflictingSourcePaths[0],
		conflictingSourcePaths[1],
	)
}

func ensureNoStaticLogicalPathCollisionWithinSourceDirectory(
	sourceDirectoryPath string,
	relativePath string,
) error {
	relativePathWithNativeSeparators := filepath.FromSlash(relativePath)
	candidateFileInfos := []fileInfo{
		{
			srcPath: filepath.Join(sourceDirectoryPath, relativePathWithNativeSeparators),
			relPath: relativePath,
			prehash: false,
		},
		{
			srcPath: filepath.Join(
				sourceDirectoryPath,
				wave.PrehashedDirname,
				relativePathWithNativeSeparators,
			),
			relPath: relativePath,
			prehash: true,
		},
		{
			srcPath: filepath.Join(
				sourceDirectoryPath,
				wave.NohashDirname,
				relativePathWithNativeSeparators,
			),
			relPath: relativePath,
			prehash: true,
		},
	}

	existingSourcePaths := make([]string, 0, len(candidateFileInfos))
	for _, candidateFileInfo := range candidateFileInfos {
		sourceExists, sourceStatError := staticSourceFileExists(candidateFileInfo.srcPath)
		if sourceStatError != nil {
			return sourceStatError
		}
		if sourceExists {
			existingSourcePaths = append(
				existingSourcePaths,
				pathnorm.Absolute(candidateFileInfo.srcPath),
			)
		}
	}

	if len(existingSourcePaths) <= 1 {
		return nil
	}

	sort.Strings(existingSourcePaths)
	return fmt.Errorf(
		"static source path collision for logical path %q: multiple source files map to the same output: %s; keep exactly one source file",
		relativePath,
		strings.Join(existingSourcePaths, ", "),
	)
}

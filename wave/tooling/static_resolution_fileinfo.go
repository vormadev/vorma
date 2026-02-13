package tooling

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/wave"
)

func resolveStaticRelativePathFromSourcePath(
	sourceDirectoryPath string,
	sourcePath string,
) (string, bool, error) {
	relativePath, relativePathError := filepath.Rel(sourceDirectoryPath, sourcePath)
	if relativePathError != nil {
		return "", false, relativePathError
	}
	normalizedRelativePath := filepath.ToSlash(relativePath)
	if normalizedRelativePath == "." ||
		normalizedRelativePath == ".." ||
		strings.HasPrefix(normalizedRelativePath, "../") {
		return "", false, nil
	}
	return normalizedRelativePath, true, nil
}

func buildStaticFileInfoFromRelativePath(
	sourcePath string,
	relativePath string,
) (fileInfo, bool) {
	normalizedRelativePath := relativePath
	prehash := false
	prehashedPrefix := wave.PrehashedDirname + "/"
	nohashPrefix := wave.NohashDirname + "/"
	if strings.HasPrefix(normalizedRelativePath, prehashedPrefix) {
		prehash = true
		normalizedRelativePath = strings.TrimPrefix(normalizedRelativePath, prehashedPrefix)
	} else if strings.HasPrefix(normalizedRelativePath, nohashPrefix) {
		prehash = true
		normalizedRelativePath = strings.TrimPrefix(normalizedRelativePath, nohashPrefix)
	}

	if normalizedRelativePath == "" {
		return fileInfo{}, false
	}

	if _, ignore := staticIgnoreList[filepath.Base(normalizedRelativePath)]; ignore {
		return fileInfo{}, false
	}

	return fileInfo{
		srcPath: sourcePath,
		relPath: normalizedRelativePath,
		prehash: prehash,
	}, true
}

func resolveStaticFileInfoFromSourcePath(
	sourceDirectoryPath string,
	sourcePath string,
) (fileInfo, bool, error) {
	relativePath, isWithinSourceDirectory, relativePathError := resolveStaticRelativePathFromSourcePath(
		sourceDirectoryPath,
		sourcePath,
	)
	if relativePathError != nil {
		return fileInfo{}, false, relativePathError
	}
	if !isWithinSourceDirectory {
		return fileInfo{}, false, nil
	}

	staticFileInfo, shouldProcessFile := buildStaticFileInfoFromRelativePath(sourcePath, relativePath)
	return staticFileInfo, shouldProcessFile, nil
}

func staticSourceFileExists(sourcePath string) (bool, error) {
	sourceInfo, sourceStatError := os.Stat(sourcePath)
	if sourceStatError != nil {
		if os.IsNotExist(sourceStatError) {
			return false, nil
		}
		return false, fmt.Errorf("stat changed file %s: %w", sourcePath, sourceStatError)
	}

	return !sourceInfo.IsDir(), nil
}

package tooling

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
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

type staticChangedPathResolution struct {
	fileInfo     fileInfo
	sourceExists bool
}

type staticChangedPathResolutionProbeFunctions struct {
	normalizeChangedSourcePath                                        func(string) string
	resolveStaticFileInfoFromSourcePath                               func(string, string) (fileInfo, bool, error)
	sourceFileExists                                                  func(string) (bool, error)
	ensureNoStaticLogicalPathCollisionWithinSourceDirectoryForRelPath func(string, string) error
}

type staticChangedPathResolutionProbeResult struct {
	staticFileInfo    fileInfo
	shouldProcessFile bool
}

func resolveStaticChangedPathResolutions(
	sourceDirectoryPath string,
	changedSourcePaths []string,
) (map[string]staticChangedPathResolution, bool, error) {
	return resolveStaticChangedPathResolutionsWithProbeFunctions(
		sourceDirectoryPath,
		changedSourcePaths,
		staticChangedPathResolutionProbeFunctions{
			normalizeChangedSourcePath: pathnorm.CanonicalizePathForLocationComparison,
			resolveStaticFileInfoFromSourcePath: func(
				sourceDirectoryPath string,
				sourcePath string,
			) (fileInfo, bool, error) {
				return resolveStaticFileInfoFromSourcePath(sourceDirectoryPath, sourcePath)
			},
			sourceFileExists: staticSourceFileExists,
			ensureNoStaticLogicalPathCollisionWithinSourceDirectoryForRelPath: func(
				sourceDirectoryPath string,
				relativePath string,
			) error {
				return ensureNoStaticLogicalPathCollisionWithinSourceDirectory(
					sourceDirectoryPath,
					relativePath,
				)
			},
		},
	)
}

func resolveStaticChangedPathResolutionsWithProbeFunctions(
	sourceDirectoryPath string,
	changedSourcePaths []string,
	probeFunctions staticChangedPathResolutionProbeFunctions,
) (map[string]staticChangedPathResolution, bool, error) {
	changedResolutions := make(map[string]staticChangedPathResolution)
	resolveProbeResultByNormalizedChangedSourcePath := make(
		map[string]staticChangedPathResolutionProbeResult,
	)
	sourceExistsByNormalizedChangedSourcePath := make(map[string]bool)
	collisionCheckedByRelativePath := make(map[string]struct{})

	normalizeChangedSourcePath := probeFunctions.normalizeChangedSourcePath
	if normalizeChangedSourcePath == nil {
		normalizeChangedSourcePath = pathnorm.CanonicalizePathForLocationComparison
	}

	normalizedSourceDirectoryPath := normalizeChangedSourcePath(sourceDirectoryPath)
	if normalizedSourceDirectoryPath != "" {
		sourceDirectoryPath = normalizedSourceDirectoryPath
	}
	resolveStaticFileInfoFromSourcePathProbe := probeFunctions.resolveStaticFileInfoFromSourcePath
	if resolveStaticFileInfoFromSourcePathProbe == nil {
		resolveStaticFileInfoFromSourcePathProbe = resolveStaticFileInfoFromSourcePath
	}
	sourceFileExistsProbe := probeFunctions.sourceFileExists
	if sourceFileExistsProbe == nil {
		sourceFileExistsProbe = staticSourceFileExists
	}
	collisionProbe := probeFunctions.ensureNoStaticLogicalPathCollisionWithinSourceDirectoryForRelPath
	if collisionProbe == nil {
		collisionProbe = ensureNoStaticLogicalPathCollisionWithinSourceDirectory
	}

	for _, changedSourcePath := range changedSourcePaths {
		normalizedChangedSourcePath := normalizeChangedSourcePath(changedSourcePath)
		if normalizedChangedSourcePath == "" {
			continue
		}
		if normalizedChangedSourcePath == sourceDirectoryPath {
			return nil, true, nil
		}

		resolutionProbeResult, hasResolutionProbeResult := resolveProbeResultByNormalizedChangedSourcePath[normalizedChangedSourcePath]
		if !hasResolutionProbeResult {
			staticFileInfo, shouldProcessFile, resolveError := resolveStaticFileInfoFromSourcePathProbe(
				sourceDirectoryPath,
				normalizedChangedSourcePath,
			)
			if resolveError != nil {
				return nil, false, fmt.Errorf(
					"resolve static file info for changed path %s: %w",
					changedSourcePath,
					resolveError,
				)
			}

			resolutionProbeResult = staticChangedPathResolutionProbeResult{
				staticFileInfo:    staticFileInfo,
				shouldProcessFile: shouldProcessFile,
			}
			resolveProbeResultByNormalizedChangedSourcePath[normalizedChangedSourcePath] = resolutionProbeResult
		}
		if !resolutionProbeResult.shouldProcessFile {
			continue
		}

		sourceExists, hasSourceExists := sourceExistsByNormalizedChangedSourcePath[normalizedChangedSourcePath]
		if !hasSourceExists {
			sourceExistsForPath, sourceStatError := sourceFileExistsProbe(normalizedChangedSourcePath)
			if sourceStatError != nil {
				return nil, false, sourceStatError
			}

			sourceExists = sourceExistsForPath
			sourceExistsByNormalizedChangedSourcePath[normalizedChangedSourcePath] = sourceExists
		}

		if sourceExists {
			relativePath := resolutionProbeResult.staticFileInfo.relPath
			if _, alreadyChecked := collisionCheckedByRelativePath[relativePath]; !alreadyChecked {
				collisionError := collisionProbe(sourceDirectoryPath, relativePath)
				if collisionError != nil {
					return nil, false, collisionError
				}
				collisionCheckedByRelativePath[relativePath] = struct{}{}
			}
		}

		relativePath := resolutionProbeResult.staticFileInfo.relPath
		existingResolution, alreadyResolved := changedResolutions[relativePath]
		if !alreadyResolved || sourceExists || !existingResolution.sourceExists {
			changedResolutions[relativePath] = staticChangedPathResolution{
				fileInfo:     resolutionProbeResult.staticFileInfo,
				sourceExists: sourceExists,
			}
		}
	}

	return changedResolutions, false, nil
}

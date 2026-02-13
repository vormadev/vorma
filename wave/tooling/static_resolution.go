package tooling

import (
	"fmt"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

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

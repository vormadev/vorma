package tooling

import (
	"os"
	"sort"

	"github.com/vormadev/vorma/wave"
)

func (b *Builder) processStaticFilesForChangedPaths(
	opts staticOpts,
	changedSourcePaths []string,
) error {
	if len(changedSourcePaths) == 0 {
		return b.processStaticFiles(opts)
	}

	changedResolutions, fullBuildRequired, resolutionError := resolveStaticChangedPathResolutions(
		opts.srcDir,
		changedSourcePaths,
	)
	if resolutionError != nil {
		return resolutionError
	}
	if fullBuildRequired {
		return b.processStaticFiles(opts)
	}
	if len(changedResolutions) == 0 {
		return nil
	}

	oldMap, loadError := b.loadFileMapFromPath(opts.gobPath)
	if loadError != nil {
		return b.processStaticFiles(opts)
	}
	if oldMap == nil {
		oldMap = make(wave.FileMap)
	}

	resolvedRelativePaths := make([]string, 0, len(changedResolutions))
	for relativePath := range changedResolutions {
		resolvedRelativePaths = append(resolvedRelativePaths, relativePath)
	}
	sort.Strings(resolvedRelativePaths)

	mapWasChanged := false
	relativePathsNeedingRemoval := make([]string, 0, len(resolvedRelativePaths))
	for _, resolvedRelativePath := range resolvedRelativePaths {
		resolution := changedResolutions[resolvedRelativePath]
		oldValueForPath, hadOldValueForPath := oldMap[resolvedRelativePath]

		if resolution.sourceExists {
			updatedValue, hasUpdatedValue, processError := b.processChangedStaticFile(
				resolution.fileInfo,
				opts,
				oldMap,
			)
			if processError != nil {
				return processError
			}
			if hasUpdatedValue {
				if !hadOldValueForPath || oldValueForPath != updatedValue {
					mapWasChanged = true
				}
				oldMap[resolvedRelativePath] = updatedValue

				if hadOldValueForPath && oldValueForPath.DistName != updatedValue.DistName {
					removeStaticDistArtifactIfPresent(opts.distDir, oldValueForPath.DistName)
				}
				continue
			}
		}
		relativePathsNeedingRemoval = append(relativePathsNeedingRemoval, resolvedRelativePath)
	}

	if removeStaticMapEntriesForChangedRelativePaths(
		oldMap,
		opts.distDir,
		relativePathsNeedingRemoval,
	) {
		mapWasChanged = true
	}

	if !mapWasChanged {
		return nil
	}

	if err := b.saveFileMap(oldMap, opts.gobPath); err != nil {
		return err
	}

	if opts.isPublic {
		return b.savePublicFileMapJS(oldMap)
	}

	return nil
}

func (b *Builder) processChangedStaticFile(
	staticFileInfo fileInfo,
	opts staticOpts,
	oldMap wave.FileMap,
) (wave.FileVal, bool, error) {
	processedValue, processError := b.processStaticFileAndReturnValue(
		staticFileInfo,
		opts,
		oldMap,
	)
	if processError != nil {
		if os.IsNotExist(processError) {
			return wave.FileVal{}, false, nil
		}
		return wave.FileVal{}, false, processError
	}

	return processedValue, true, nil
}

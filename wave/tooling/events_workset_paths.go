package tooling

import "github.com/vormadev/vorma/wave/internal/pathnorm"

func normalizeChangedSourceFilePathForWorkSet(
	filePath string,
) string {
	return pathnorm.Absolute(filePath)
}

func appendNormalizedFilePathIfMissing(
	existingFilePaths []string,
	existingFilePathSet map[string]struct{},
	filePath string,
) ([]string, map[string]struct{}) {
	normalizedFilePath := normalizeChangedSourceFilePathForWorkSet(filePath)
	if normalizedFilePath == "" {
		return existingFilePaths, existingFilePathSet
	}

	if existingFilePathSet == nil {
		existingFilePathSet = make(map[string]struct{}, len(existingFilePaths)+1)
		for _, existingFilePath := range existingFilePaths {
			existingFilePathSet[existingFilePath] = struct{}{}
		}
	}
	if _, alreadyExists := existingFilePathSet[normalizedFilePath]; alreadyExists {
		return existingFilePaths, existingFilePathSet
	}

	existingFilePaths = append(existingFilePaths, normalizedFilePath)
	existingFilePathSet[normalizedFilePath] = struct{}{}
	return existingFilePaths, existingFilePathSet
}

func (buildDecision *buildPhaseDecision) addPublicStaticChangedFilePath(
	filePath string,
) {
	buildDecision.publicStaticChangedFilePaths, buildDecision.publicStaticChangedFilePathSet = appendNormalizedFilePathIfMissing(
		buildDecision.publicStaticChangedFilePaths,
		buildDecision.publicStaticChangedFilePathSet,
		filePath,
	)
}

func (buildDecision *buildPhaseDecision) addPrivateStaticChangedFilePath(
	filePath string,
) {
	buildDecision.privateStaticChangedFilePaths, buildDecision.privateStaticChangedFilePathSet = appendNormalizedFilePathIfMissing(
		buildDecision.privateStaticChangedFilePaths,
		buildDecision.privateStaticChangedFilePathSet,
		filePath,
	)
}

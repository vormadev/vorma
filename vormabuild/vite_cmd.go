package vormabuild

import (
	"fmt"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

type postViteProdBuildDependencies struct {
	toPathsFileStageTwo      func(*vormaruntime.Vorma) (*vormaruntime.PathsFile, error)
	writePathsToDiskStageTwo func(*vormaruntime.Vorma, *vormaruntime.PathsFile) error
	applyBuildIDToVorma      func(*vormaruntime.Vorma, string)
}

func defaultPostViteProdBuildDependencies() postViteProdBuildDependencies {
	return postViteProdBuildDependencies{
		toPathsFileStageTwo:      toPathsFileStageTwo,
		writePathsToDiskStageTwo: writePathsToDiskStageTwo,
		applyBuildIDToVorma:      applyBuildIDToVorma,
	}
}

func normalizePostViteProdBuildDependencies(
	dependencies postViteProdBuildDependencies,
) postViteProdBuildDependencies {
	defaultDependencies := defaultPostViteProdBuildDependencies()

	if dependencies.toPathsFileStageTwo == nil {
		dependencies.toPathsFileStageTwo = defaultDependencies.toPathsFileStageTwo
	}
	if dependencies.writePathsToDiskStageTwo == nil {
		dependencies.writePathsToDiskStageTwo = defaultDependencies.writePathsToDiskStageTwo
	}
	if dependencies.applyBuildIDToVorma == nil {
		dependencies.applyBuildIDToVorma = defaultDependencies.applyBuildIDToVorma
	}

	return dependencies
}

func postViteProdBuild(v *vormaruntime.Vorma) error {
	return postViteProdBuildWithDependencies(v, postViteProdBuildDependencies{})
}

func postViteProdBuildWithDependencies(
	v *vormaruntime.Vorma,
	dependencies postViteProdBuildDependencies,
) error {
	dependencies = normalizePostViteProdBuildDependencies(dependencies)

	pathsFile, err := dependencies.toPathsFileStageTwo(v)
	if err != nil {
		return fmt.Errorf("convert paths to stage two: %w", err)
	}

	if err := dependencies.writePathsToDiskStageTwo(v, pathsFile); err != nil {
		return fmt.Errorf("write stage-two paths: %w", err)
	}

	dependencies.applyBuildIDToVorma(v, pathsFile.BuildID)
	return nil
}

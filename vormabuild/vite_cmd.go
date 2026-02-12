package vormabuild

import (
	"fmt"

	"github.com/vormadev/vorma/vormaruntime"
)

func postViteProdBuild(v *vormaruntime.Vorma) error {
	pathsFile, err := toPathsFileStageTwo(v)
	if err != nil {
		return fmt.Errorf("convert paths to stage two: %w", err)
	}

	if err := writePathsToDiskStageTwo(v, pathsFile); err != nil {
		return fmt.Errorf("write stage-two paths: %w", err)
	}

	applyBuildIDToVorma(v, pathsFile.BuildID)
	return nil
}

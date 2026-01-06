package vorma

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func (v *Vorma) postViteProdBuild() error {
	// Must come after Vite -- only needed in prod (the stage "one" version is fine in dev)
	pf, err := v.toPathsFile_StageTwo()
	if err != nil {
		Log.Error(fmt.Sprintf("error converting paths to paths file: %s", err))
		return err
	}

	pathsAsJSON, err := json.MarshalIndent(pf, "", "\t")

	if err != nil {
		Log.Error(fmt.Sprintf("error marshalling paths to JSON: %s", err))
		return err
	}

	pathsJSONOut_StageTwo := filepath.Join(
		v.Wave.GetStaticPrivateOutDir(),
		"vorma_out",
		VormaPathsStageTwoJSONFileName,
	)
	err = os.WriteFile(pathsJSONOut_StageTwo, pathsAsJSON, os.ModePerm)
	if err != nil {
		Log.Error(fmt.Sprintf("error writing paths to disk: %s", err))
		return err
	}

	return nil
}

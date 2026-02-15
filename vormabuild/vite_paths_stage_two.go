package vormabuild

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/lab/viteutil"
)

type stageTwoPathsWriteDependencies struct {
	marshalStageTwoPathsFile func(*vormaruntime.PathsFile) ([]byte, error)
	writeStageTwoPathsJSON   func(*vormaruntime.Vorma, []byte) error
}

var stageTwoPathsWriteDeps = stageTwoPathsWriteDependencies{
	marshalStageTwoPathsFile: func(pathsFile *vormaruntime.PathsFile) ([]byte, error) {
		return json.MarshalIndent(pathsFile, "", "\t")
	},
	writeStageTwoPathsJSON: func(v *vormaruntime.Vorma, pathsAsJSON []byte) error {
		return writePathsJSONBytesToOutputPath(
			pathsOutputPath(v, vormaruntime.VormaPathsStageTwoJSONFileName),
			pathsAsJSON,
			pathsJSONWriteDependencies{
				makePathsOutputDirectory: os.MkdirAll,
				writePathsJSON:           writeFileAtomically,
			},
			"create stage-two paths output directory",
			"write stage-two paths JSON",
		)
	},
}

func toPathsFileStageTwo(v *vormaruntime.Vorma) (*vormaruntime.PathsFile, error) {
	viteManifest, err := viteutil.ReadManifest(v.Wave.GetViteManifestLocation())
	if err != nil {
		return nil, fmt.Errorf("read vite manifest: %w", err)
	}

	paths := v.GetPathsSnapshot()
	cleanClientEntry := filepath.Clean(v.Config.ClientEntry)
	manifestApplicationResult := applyViteManifestToPaths(
		viteManifest,
		paths,
		cleanClientEntry,
	)

	pathsFile := buildStageTwoPathsFile(
		v,
		paths,
		manifestApplicationResult,
	)

	buildID, err := computeStageTwoBuildID(v, pathsFile)
	if err != nil {
		return nil, err
	}

	applyBuildIDToPathsFile(pathsFile, buildID)
	return pathsFile, nil
}

func writePathsToDiskStageTwo(v *vormaruntime.Vorma, pathsFile *vormaruntime.PathsFile) error {
	pathsAsJSON, err := stageTwoPathsWriteDeps.marshalStageTwoPathsFile(pathsFile)
	if err != nil {
		return fmt.Errorf("marshal stage-two paths file: %w", err)
	}

	if err := stageTwoPathsWriteDeps.writeStageTwoPathsJSON(v, pathsAsJSON); err != nil {
		return fmt.Errorf("write stage-two paths JSON: %w", err)
	}
	return nil
}

func pathsOutputPath(v *vormaruntime.Vorma, fileName string) string {
	return filepath.Join(v.Wave.GetStaticPrivateOutDir(), vormaruntime.VormaOutDirname, fileName)
}

func applyBuildIDToPathsFile(pathsFile *vormaruntime.PathsFile, buildID string) {
	pathsFile.BuildID = buildID
}

func applyBuildIDToVorma(v *vormaruntime.Vorma, buildID string) {
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID(buildID)
	})
}

func buildStageTwoPathsFile(
	v *vormaruntime.Vorma,
	paths map[string]*vormaruntime.Path,
	manifestApplicationResult viteManifestApplicationResult,
) *vormaruntime.PathsFile {
	return &vormaruntime.PathsFile{
		Stage:             "two",
		DepToCSSBundleMap: manifestApplicationResult.depToCSSBundleMap,
		Paths:             paths,
		ClientEntrySrc:    v.Config.ClientEntry,
		ClientEntryOut:    manifestApplicationResult.clientEntryOut,
		ClientEntryDeps:   manifestApplicationResult.clientEntryDeps,
		RouteManifestFile: v.GetRouteManifestFile(),
	}
}

package vormabuild

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/lab/viteutil"
)

type stageTwoPathsWriteDependencies struct {
	marshalStageTwoPathsFile func(*vormaruntime.PathsFile) ([]byte, error)
	writeStageTwoPathsJSON   func(*vormaruntime.Vorma, []byte) error
}

type stageTwoPathsWriteExecutor struct {
	dependencies stageTwoPathsWriteDependencies
}

var defaultStageTwoPathsWriteExecutor = newStageTwoPathsWriteExecutor(
	stageTwoPathsWriteDependencies{},
)

func defaultStageTwoPathsWriteDependencies() stageTwoPathsWriteDependencies {
	return stageTwoPathsWriteDependencies{
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
}

func normalizeStageTwoPathsWriteDependencies(
	dependencies stageTwoPathsWriteDependencies,
) stageTwoPathsWriteDependencies {
	defaultDependencies := defaultStageTwoPathsWriteDependencies()

	if dependencies.marshalStageTwoPathsFile == nil {
		dependencies.marshalStageTwoPathsFile = defaultDependencies.marshalStageTwoPathsFile
	}
	if dependencies.writeStageTwoPathsJSON == nil {
		dependencies.writeStageTwoPathsJSON = defaultDependencies.writeStageTwoPathsJSON
	}

	return dependencies
}

func newStageTwoPathsWriteExecutor(
	dependencies stageTwoPathsWriteDependencies,
) stageTwoPathsWriteExecutor {
	return stageTwoPathsWriteExecutor{
		dependencies: normalizeStageTwoPathsWriteDependencies(dependencies),
	}
}

func toPathsFileStageTwo(v *vormaruntime.Vorma) (*vormaruntime.PathsFile, error) {
	viteManifest, err := viteutil.ReadManifest(v.Wave.ViteManifestLocation())
	if err != nil {
		return nil, fmt.Errorf("read vite manifest: %w", err)
	}

	paths := v.Paths()
	cleanClientEntry := filepath.Clean(v.Config.ClientEntry)
	manifestApplicationResult := applyViteManifestToPaths(
		viteManifest,
		paths,
		cleanClientEntry,
	)
	if err := validateStageTwoManifestCoverage(
		paths,
		cleanClientEntry,
		manifestApplicationResult,
	); err != nil {
		return nil, err
	}

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
	return defaultStageTwoPathsWriteExecutor.writePathsToDiskStageTwo(v, pathsFile)
}

func (executor stageTwoPathsWriteExecutor) writePathsToDiskStageTwo(
	v *vormaruntime.Vorma,
	pathsFile *vormaruntime.PathsFile,
) error {
	pathsAsJSON, err := executor.dependencies.marshalStageTwoPathsFile(pathsFile)
	if err != nil {
		return fmt.Errorf("marshal stage-two paths file: %w", err)
	}

	if err := executor.dependencies.writeStageTwoPathsJSON(v, pathsAsJSON); err != nil {
		return fmt.Errorf("write stage-two paths JSON: %w", err)
	}
	return nil
}

func pathsOutputPath(v *vormaruntime.Vorma, fileName string) string {
	return filepath.Join(v.Wave.StaticPrivateOutDir(), vormaruntime.VormaOutDirname, fileName)
}

func applyBuildIDToPathsFile(pathsFile *vormaruntime.PathsFile, buildID string) {
	pathsFile.BuildID = buildID
}

func applyBuildIDToVorma(v *vormaruntime.Vorma, buildID string) {
	commitRuntimeState(
		v,
		runtimeStateCommitInput{
			shouldCommitBuildID: true,
			buildID:             buildID,
		},
	)
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
		RouteManifestFile: v.RouteManifestFile(),
	}
}

func validateStageTwoManifestCoverage(
	paths map[string]*vormaruntime.Path,
	cleanClientEntry string,
	manifestApplicationResult viteManifestApplicationResult,
) error {
	if strings.TrimSpace(manifestApplicationResult.clientEntryOut) == "" {
		return fmt.Errorf(
			"client entry chunk missing from Vite manifest for %q",
			cleanClientEntry,
		)
	}

	for routePattern, routePath := range paths {
		if routePath == nil || strings.TrimSpace(routePath.SrcPath) == "" {
			continue
		}
		if strings.TrimSpace(routePath.OutPath) != "" {
			continue
		}
		return fmt.Errorf(
			"route chunk missing from Vite manifest for %q (source: %q)",
			routePattern,
			routePath.SrcPath,
		)
	}

	return nil
}

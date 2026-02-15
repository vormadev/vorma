package vormabuild

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/id"
)

type fastRouteRebuildDependencies struct {
	parseClientRoutes             func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error)
	newFastRebuildID              func() (string, error)
	runRouteSyncExecution         func(*vormaruntime.Vorma, routeSyncExecutionOptions) error
	logFastRouteRebuildCompletion func(*vormaruntime.Vorma, time.Time)
}

type fastRouteRebuildArtifactDependencies struct {
	cleanRouteManifestsOnly     func(*vormaruntime.Vorma) error
	writeRouteArtifacts         func(*vormaruntime.LockedVorma) error
	readRouteManifestArtifact   func(string) ([]byte, error)
	writeRouteManifestArtifact  func(string, []byte, os.FileMode) error
	removeRouteManifestArtifact func(string) error
}

type fastRouteRebuildBuildIDDependencies struct {
	generateFastRebuildIDSuffix func() (string, error)
}

var fastRouteRebuildDeps = fastRouteRebuildDependencies{
	parseClientRoutes:             parseClientRoutes,
	newFastRebuildID:              newFastRebuildID,
	runRouteSyncExecution:         runRouteSyncExecution,
	logFastRouteRebuildCompletion: logFastRouteRebuildCompletion,
}

var fastRouteRebuildArtifactDeps = fastRouteRebuildArtifactDependencies{
	cleanRouteManifestsOnly:     cleanRouteManifestsOnly,
	writeRouteArtifacts:         writeRouteArtifacts,
	readRouteManifestArtifact:   os.ReadFile,
	writeRouteManifestArtifact:  writeFileAtomically,
	removeRouteManifestArtifact: os.Remove,
}

var fastRouteRebuildBuildIDDeps = fastRouteRebuildBuildIDDependencies{
	generateFastRebuildIDSuffix: func() (string, error) {
		return id.New(16)
	},
}

// rebuildRoutesOnly is the fast path for rebuilding when only vorma.routes.ts changes.
// Runs in Process A (Dev Server), which has handlers registered for type reflection.
//
// Flow:
//  1. Parse client routes with esbuild
//  2. Generate TypeScript using live reflection (Process A has handlers)
//  3. Write all artifacts to disk
//  4. Wave calls Process B's reload endpoint to sync from disk
//
// Performance: ~50ms vs ~1.5s for full rebuild
func rebuildRoutesOnly(v *vormaruntime.Vorma) error {
	start := time.Now()

	if !v.GetIsDevMode() {
		return errors.New("rebuildRoutesOnly should only be called in dev mode")
	}

	v.Log.Info("START fast route rebuild")

	err := fastRouteRebuildDeps.runRouteSyncExecution(
		v,
		routeSyncExecutionOptions{
			parseClientRoutes:          fastRouteRebuildDeps.parseClientRoutes,
			generateBuildID:            fastRouteRebuildDeps.newFastRebuildID,
			parseClientRoutesErrorText: "parse client routes",
			postSyncHook: func(l *vormaruntime.LockedVorma) error {
				return writeFastRebuildArtifactsAfterRouteSync(v, l)
			},
		},
	)
	if err != nil {
		return err
	}

	fastRouteRebuildDeps.logFastRouteRebuildCompletion(v, start)
	return nil
}

func newFastRebuildID() (string, error) {
	return generateBuildIDWithPrefix(
		"dev_fast_",
		fastRouteRebuildBuildIDDeps.generateFastRebuildIDSuffix,
	)
}

func writeFastRebuildArtifactsAfterRouteSync(
	v *vormaruntime.Vorma,
	l *vormaruntime.LockedVorma,
) error {
	previousRouteManifestFile := l.GetRouteManifestFile()
	previousRouteManifestSnapshot, err := captureFastRebuildRouteManifestArtifactSnapshot(
		v,
		previousRouteManifestFile,
	)
	if err != nil {
		return fmt.Errorf("snapshot current route manifest artifact: %w", err)
	}

	return runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				if err := fastRouteRebuildArtifactDeps.cleanRouteManifestsOnly(v); err != nil {
					return fmt.Errorf("clean route manifests: %w", err)
				}

				return fastRouteRebuildArtifactDeps.writeRouteArtifacts(l)
			},
			rollbackOnFailure: func() error {
				return restoreFastRebuildRouteManifestArtifactSnapshot(
					v,
					previousRouteManifestFile,
					previousRouteManifestSnapshot,
				)
			},
			rollbackErrorContext: "restore route manifest artifact",
			logRollbackFailureAfterPanic: func(rollbackErr error) {
				if v.Log != nil {
					v.Log.Error("restore route manifest artifact after panic failed", "error", rollbackErr)
				}
			},
		},
	)
}

type fastRebuildRouteManifestArtifactSnapshot = buildArtifactFileSnapshot

func captureFastRebuildRouteManifestArtifactSnapshot(
	v *vormaruntime.Vorma,
	manifestFile string,
) (fastRebuildRouteManifestArtifactSnapshot, error) {
	if manifestFile == "" {
		return fastRebuildRouteManifestArtifactSnapshot{}, nil
	}

	manifestFilePath := filepath.Join(v.Wave.GetStaticPublicOutDir(), manifestFile)
	return captureBuildArtifactFileSnapshot(
		manifestFilePath,
		fastRouteRebuildArtifactDeps.readRouteManifestArtifact,
	)
}

func restoreFastRebuildRouteManifestArtifactSnapshot(
	v *vormaruntime.Vorma,
	manifestFile string,
	snapshot fastRebuildRouteManifestArtifactSnapshot,
) error {
	if manifestFile == "" {
		return nil
	}

	manifestFilePath := filepath.Join(v.Wave.GetStaticPublicOutDir(), manifestFile)
	return restoreBuildArtifactFileSnapshot(
		manifestFilePath,
		snapshot,
		fastRouteRebuildArtifactDeps.writeRouteManifestArtifact,
		fastRouteRebuildArtifactDeps.removeRouteManifestArtifact,
	)
}

func logFastRouteRebuildCompletion(v *vormaruntime.Vorma, start time.Time) {
	v.Log.Info("DONE fast route rebuild",
		"buildID", v.GetBuildID(),
		"routes", len(v.GetPathsSnapshot()),
		"duration", time.Since(start),
	)
}

func cleanRouteManifestsOnly(v *vormaruntime.Vorma) error {
	staticPublicOutDir := v.Wave.GetStaticPublicOutDir()
	err := removeMatchingTopLevelFiles(staticPublicOutDir, isGeneratedRouteManifestFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func isGeneratedRouteManifestFilename(fileName string) bool {
	return len(fileName) > len(vormaruntime.VormaRouteManifestPrefix) &&
		strings.HasPrefix(fileName, vormaruntime.VormaRouteManifestPrefix)
}

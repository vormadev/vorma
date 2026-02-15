package vormabuild

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/mux"
)

type routeRegistryBuildDependencies struct {
	marshalRouteManifestJSON    func(any) ([]byte, error)
	writeRouteManifestJSON      func(string, []byte, os.FileMode) error
	removeRouteManifestJSON     func(string) error
	readStageOnePathsArtifact   func(string) ([]byte, error)
	writeStageOnePathsArtifact  func(string, []byte, os.FileMode) error
	removeStageOnePathsArtifact func(string) error
	writeGeneratedTypeScript    func(*vormaruntime.LockedVorma) error
}

var routeRegistryBuildDeps = routeRegistryBuildDependencies{
	marshalRouteManifestJSON:    json.Marshal,
	writeRouteManifestJSON:      writeFileAtomically,
	removeRouteManifestJSON:     os.Remove,
	readStageOnePathsArtifact:   os.ReadFile,
	writeStageOnePathsArtifact:  writeFileAtomically,
	removeStageOnePathsArtifact: os.Remove,
	writeGeneratedTypeScript:    writeGeneratedTS,
}

// writeRouteArtifacts writes all route-related artifacts to disk.
// Includes manifest, paths JSON, and TypeScript generation.
func writeRouteArtifacts(l *vormaruntime.LockedVorma) error {
	v := l.Vorma()
	previousRouteManifestFile := l.GetRouteManifestFile()
	stageOnePathsArtifactPath := stageOnePathsArtifactOutputPath(v)

	stageOnePathsArtifactSnapshot, err := captureStageOnePathsArtifactSnapshot(stageOnePathsArtifactPath)
	if err != nil {
		return fmt.Errorf("snapshot stage-one paths artifact: %w", err)
	}

	manifestFile, err := writeRouteManifestArtifact(l)
	if err != nil {
		return fmt.Errorf("write route manifest: %w", err)
	}

	transactionErr := runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				if err := writePathsToDiskStageOneWithRouteManifest(l, manifestFile); err != nil {
					return fmt.Errorf("write paths JSON: %w", err)
				}

				if err := routeRegistryBuildDeps.writeGeneratedTypeScript(l); err != nil {
					return fmt.Errorf("write generated TypeScript: %w", err)
				}
				return nil
			},
			rollbackOnFailure: func() error {
				return cleanupRouteArtifactsAfterWriteFailure(
					v,
					manifestFile,
					previousRouteManifestFile,
					stageOnePathsArtifactPath,
					stageOnePathsArtifactSnapshot,
				)
			},
			logRollbackFailureAfterPanic: func(rollbackErr error) {
				if v.Log != nil {
					v.Log.Error("cleanup route artifacts after panic failed", "error", rollbackErr)
				}
			},
		},
	)
	if transactionErr != nil {
		return transactionErr
	}

	l.SetRouteManifestFile(manifestFile)
	return nil
}

func writeAndSetRouteManifest(l *vormaruntime.LockedVorma) error {
	manifestFile, err := writeRouteManifestArtifact(l)
	if err != nil {
		return err
	}

	l.SetRouteManifestFile(manifestFile)
	return nil
}

func writeRouteManifestArtifact(l *vormaruntime.LockedVorma) (string, error) {
	v := l.Vorma()
	manifest := generateRouteManifest(l, v.LoadersRouter().NestedRouter)
	manifestFile, err := writeRouteManifestToDisk(v, manifest)
	if err != nil {
		return "", err
	}
	return manifestFile, nil
}

func writeRouteManifestToDisk(v *vormaruntime.Vorma, manifest map[string]int) (string, error) {
	manifestJSON, err := routeRegistryBuildDeps.marshalRouteManifestJSON(manifest)
	if err != nil {
		return "", fmt.Errorf("marshal route manifest: %w", err)
	}

	filename := routeManifestFilename(manifestJSON)

	outPath := filepath.Join(v.Wave.GetStaticPublicOutDir(), filename)
	if err := routeRegistryBuildDeps.writeRouteManifestJSON(outPath, manifestJSON, buildArtifactFileMode); err != nil {
		return "", fmt.Errorf("write route manifest: %w", err)
	}

	return filename, nil
}

type stageOnePathsArtifactSnapshot = buildArtifactFileSnapshot

type routeManifestStateReader interface {
	Vorma() *vormaruntime.Vorma
	GetPaths() map[string]*vormaruntime.Path
}

func stageOnePathsArtifactOutputPath(v *vormaruntime.Vorma) string {
	return pathsOutputPath(v, vormaruntime.VormaPathsStageOneJSONFileName)
}

func captureStageOnePathsArtifactSnapshot(
	stageOnePathsArtifactPath string,
) (stageOnePathsArtifactSnapshot, error) {
	return captureBuildArtifactFileSnapshot(
		stageOnePathsArtifactPath,
		routeRegistryBuildDeps.readStageOnePathsArtifact,
	)
}

func cleanupRouteArtifactsAfterWriteFailure(
	v *vormaruntime.Vorma,
	manifestFile string,
	previousRouteManifestFile string,
	stageOnePathsArtifactPath string,
	stageOnePathsSnapshot stageOnePathsArtifactSnapshot,
) error {
	artifactCleanupErrors := make([]error, 0, 2)
	if shouldRemoveRouteManifestArtifactAfterWriteFailure(manifestFile, previousRouteManifestFile) {
		if err := removeRouteManifestArtifactFile(v, manifestFile); err != nil {
			artifactCleanupErrors = append(
				artifactCleanupErrors,
				fmt.Errorf("cleanup route manifest artifact: %w", err),
			)
		}
	}
	if err := restoreStageOnePathsArtifactFromSnapshot(stageOnePathsArtifactPath, stageOnePathsSnapshot); err != nil {
		artifactCleanupErrors = append(
			artifactCleanupErrors,
			fmt.Errorf("cleanup stage-one paths artifact: %w", err),
		)
	}

	return errors.Join(artifactCleanupErrors...)
}

func shouldRemoveRouteManifestArtifactAfterWriteFailure(
	manifestFile string,
	previousRouteManifestFile string,
) bool {
	return manifestFile != "" && manifestFile != previousRouteManifestFile
}

func removeRouteManifestArtifactFile(v *vormaruntime.Vorma, manifestFile string) error {
	manifestFilePath := filepath.Join(v.Wave.GetStaticPublicOutDir(), manifestFile)
	err := routeRegistryBuildDeps.removeRouteManifestJSON(manifestFilePath)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func restoreStageOnePathsArtifactFromSnapshot(
	stageOnePathsArtifactPath string,
	stageOnePathsSnapshot stageOnePathsArtifactSnapshot,
) error {
	return restoreBuildArtifactFileSnapshot(
		stageOnePathsArtifactPath,
		stageOnePathsSnapshot,
		routeRegistryBuildDeps.writeStageOnePathsArtifact,
		routeRegistryBuildDeps.removeStageOnePathsArtifact,
	)
}

func generateRouteManifest(l routeManifestStateReader, nestedRouter *mux.NestedRouter) map[string]int {
	manifest := make(map[string]int)
	paths := l.GetPaths()

	for _, currentPath := range paths {
		manifest[currentPath.OriginalPattern] = routeManifestServerLoaderFlag(nestedRouter, currentPath.OriginalPattern)
	}

	return manifest
}

func routeManifestFilename(manifestJSON []byte) string {
	hash := cryptoutil.Sha256Hash(manifestJSON)
	hashStr := base64.RawURLEncoding.EncodeToString(hash[:8])
	return fmt.Sprintf("%s%s.json", vormaruntime.VormaRouteManifestPrefix, hashStr)
}

func routeManifestServerLoaderFlag(nestedRouter *mux.NestedRouter, pattern string) int {
	if nestedRouter.HasTaskHandler(pattern) {
		return 1
	}
	return 0
}

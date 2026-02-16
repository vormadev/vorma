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
	marshalRouteManifestJSON                  func(any) ([]byte, error)
	writeRouteManifestJSON                    func(string, []byte, os.FileMode) error
	removeRouteManifestJSON                   func(string) error
	readStageOnePathsArtifact                 func(string) ([]byte, error)
	writeStageOnePathsArtifact                func(string, []byte, os.FileMode) error
	removeStageOnePathsArtifact               func(string) error
	writeGeneratedTypeScript                  func(*vormaruntime.LockedVorma) error
	writeStageOnePathsJSONForRuntimeState     func(*vormaruntime.Vorma, routeBuildRuntimeStateSnapshot, string) error
	writeGeneratedTypeScriptForRuntimeState   func(*vormaruntime.Vorma, routeBuildRuntimeStateSnapshot) error
	captureRouteBuildRuntimeStateWithReadLock func(*vormaruntime.Vorma) routeBuildRuntimeStateSnapshot
	captureRouteManifestStateWithRuntimeLock  func(*vormaruntime.Vorma, routeBuildRuntimeStateSnapshot) routeManifestStateSnapshot
	isRouteBuildRuntimeStateSnapshotCurrent   func(*vormaruntime.Vorma, routeBuildRuntimeStateSnapshot) bool
	commitRouteManifestFileWithRuntimeLock    func(*vormaruntime.Vorma, routeManifestCommitInput) bool
}

type routeRegistryBuildExecutor struct {
	dependencies routeRegistryBuildDependencies
}

var defaultRouteRegistryBuildExecutor = newRouteRegistryBuildExecutor(
	routeRegistryBuildDependencies{},
)

func defaultRouteRegistryBuildDependencies() routeRegistryBuildDependencies {
	return routeRegistryBuildDependencies{
		marshalRouteManifestJSON:    json.Marshal,
		writeRouteManifestJSON:      writeFileAtomically,
		removeRouteManifestJSON:     os.Remove,
		readStageOnePathsArtifact:   os.ReadFile,
		writeStageOnePathsArtifact:  writeFileAtomically,
		removeStageOnePathsArtifact: os.Remove,
		writeGeneratedTypeScript:    writeGeneratedTS,
		writeStageOnePathsJSONForRuntimeState: func(
			v *vormaruntime.Vorma,
			runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
			routeManifestFile string,
		) error {
			return writePathsToDiskStageOneFromRuntimeStateSnapshot(
				v,
				runtimeStateSnapshot,
				routeManifestFile,
			)
		},
		writeGeneratedTypeScriptForRuntimeState: writeGeneratedTSForRouteBuildRuntimeStateSnapshot,
		captureRouteBuildRuntimeStateWithReadLock: func(v *vormaruntime.Vorma) routeBuildRuntimeStateSnapshot {
			var runtimeStateSnapshot routeBuildRuntimeStateSnapshot
			v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
				runtimeStateSnapshot = captureRouteBuildRuntimeStateSnapshot(l)
			})
			return runtimeStateSnapshot
		},
		captureRouteManifestStateWithRuntimeLock: func(
			v *vormaruntime.Vorma,
			runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
		) routeManifestStateSnapshot {
			var previousRouteManifestFile string
			var routeManifest map[string]int
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				previousRouteManifestFile = l.GetRouteManifestFile()
				routeManifest = generateRouteManifestFromPaths(
					runtimeStateSnapshot.paths,
					v.LoadersRouter().NestedRouter,
				)
			})
			return routeManifestStateSnapshot{
				previousRouteManifestFile: previousRouteManifestFile,
				routeManifest:             routeManifest,
			}
		},
		isRouteBuildRuntimeStateSnapshotCurrent: routeBuildRuntimeStateSnapshotIsCurrent,
		commitRouteManifestFileWithRuntimeLock: func(
			v *vormaruntime.Vorma,
			commitInput routeManifestCommitInput,
		) bool {
			manifestCommitted := false
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				if !shouldCommitRouteManifestFileForRuntimeState(
					l.GetBuildID(),
					commitInput.expectedBuildID,
				) {
					return
				}
				commitRuntimeStateWithLock(
					l,
					runtimeStateCommitInput{
						shouldCommitRouteManifestFile: true,
						routeManifestFile:             commitInput.routeManifestFile,
					},
				)
				manifestCommitted = true
			})
			return manifestCommitted
		},
	}
}

func normalizeRouteRegistryBuildDependencies(
	dependencies routeRegistryBuildDependencies,
) routeRegistryBuildDependencies {
	defaultDependencies := defaultRouteRegistryBuildDependencies()

	if dependencies.marshalRouteManifestJSON == nil {
		dependencies.marshalRouteManifestJSON = defaultDependencies.marshalRouteManifestJSON
	}
	if dependencies.writeRouteManifestJSON == nil {
		dependencies.writeRouteManifestJSON = defaultDependencies.writeRouteManifestJSON
	}
	if dependencies.removeRouteManifestJSON == nil {
		dependencies.removeRouteManifestJSON = defaultDependencies.removeRouteManifestJSON
	}
	if dependencies.readStageOnePathsArtifact == nil {
		dependencies.readStageOnePathsArtifact = defaultDependencies.readStageOnePathsArtifact
	}
	if dependencies.writeStageOnePathsArtifact == nil {
		dependencies.writeStageOnePathsArtifact = defaultDependencies.writeStageOnePathsArtifact
	}
	if dependencies.removeStageOnePathsArtifact == nil {
		dependencies.removeStageOnePathsArtifact = defaultDependencies.removeStageOnePathsArtifact
	}
	if dependencies.writeGeneratedTypeScript == nil {
		dependencies.writeGeneratedTypeScript = defaultDependencies.writeGeneratedTypeScript
	}
	if dependencies.writeStageOnePathsJSONForRuntimeState == nil {
		dependencies.writeStageOnePathsJSONForRuntimeState = defaultDependencies.writeStageOnePathsJSONForRuntimeState
	}
	if dependencies.writeGeneratedTypeScriptForRuntimeState == nil {
		dependencies.writeGeneratedTypeScriptForRuntimeState = defaultDependencies.writeGeneratedTypeScriptForRuntimeState
	}
	if dependencies.captureRouteBuildRuntimeStateWithReadLock == nil {
		dependencies.captureRouteBuildRuntimeStateWithReadLock = defaultDependencies.captureRouteBuildRuntimeStateWithReadLock
	}
	if dependencies.captureRouteManifestStateWithRuntimeLock == nil {
		dependencies.captureRouteManifestStateWithRuntimeLock = defaultDependencies.captureRouteManifestStateWithRuntimeLock
	}
	if dependencies.isRouteBuildRuntimeStateSnapshotCurrent == nil {
		dependencies.isRouteBuildRuntimeStateSnapshotCurrent = defaultDependencies.isRouteBuildRuntimeStateSnapshotCurrent
	}
	if dependencies.commitRouteManifestFileWithRuntimeLock == nil {
		dependencies.commitRouteManifestFileWithRuntimeLock = defaultDependencies.commitRouteManifestFileWithRuntimeLock
	}

	return dependencies
}

func newRouteRegistryBuildExecutor(
	dependencies routeRegistryBuildDependencies,
) routeRegistryBuildExecutor {
	return routeRegistryBuildExecutor{
		dependencies: normalizeRouteRegistryBuildDependencies(dependencies),
	}
}

// writeRouteArtifacts writes all route-related artifacts to disk.
// Includes manifest, paths JSON, and TypeScript generation.
func writeRouteArtifacts(l *vormaruntime.LockedVorma) error {
	return defaultRouteRegistryBuildExecutor.writeRouteArtifacts(l)
}

func (executor routeRegistryBuildExecutor) writeRouteArtifacts(l *vormaruntime.LockedVorma) error {
	v := l.Vorma()
	previousRouteManifestFile := l.GetRouteManifestFile()
	stageOnePathsArtifactPath := stageOnePathsArtifactOutputPath(v)

	stageOnePathsArtifactSnapshot, err := executor.captureStageOnePathsArtifactSnapshot(
		stageOnePathsArtifactPath,
	)
	if err != nil {
		return fmt.Errorf("snapshot stage-one paths artifact: %w", err)
	}

	manifestFile, err := executor.writeRouteManifestArtifact(l)
	if err != nil {
		return fmt.Errorf("write route manifest: %w", err)
	}

	transactionErr := runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				if err := writePathsToDiskStageOneWithRouteManifest(l, manifestFile); err != nil {
					return fmt.Errorf("write paths JSON: %w", err)
				}

				if err := executor.dependencies.writeGeneratedTypeScript(l); err != nil {
					return fmt.Errorf("write generated TypeScript: %w", err)
				}
				return nil
			},
			rollbackOnFailure: func() error {
				return executor.cleanupRouteArtifactsAfterWriteFailure(
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
	return defaultRouteRegistryBuildExecutor.writeAndSetRouteManifest(l)
}

func (executor routeRegistryBuildExecutor) writeAndSetRouteManifest(
	l *vormaruntime.LockedVorma,
) error {
	manifestFile, err := executor.writeRouteManifestArtifact(l)
	if err != nil {
		return err
	}

	l.SetRouteManifestFile(manifestFile)
	return nil
}

func writeRouteManifestArtifact(l *vormaruntime.LockedVorma) (string, error) {
	return defaultRouteRegistryBuildExecutor.writeRouteManifestArtifact(l)
}

func (executor routeRegistryBuildExecutor) writeRouteManifestArtifact(
	l *vormaruntime.LockedVorma,
) (string, error) {
	v := l.Vorma()
	manifest := generateRouteManifest(l, v.LoadersRouter().NestedRouter)
	manifestFile, err := executor.writeRouteManifestToDisk(v, manifest)
	if err != nil {
		return "", err
	}
	return manifestFile, nil
}

func writeRouteManifestToDisk(v *vormaruntime.Vorma, manifest map[string]int) (string, error) {
	return defaultRouteRegistryBuildExecutor.writeRouteManifestToDisk(v, manifest)
}

func (executor routeRegistryBuildExecutor) writeRouteManifestToDisk(
	v *vormaruntime.Vorma,
	manifest map[string]int,
) (string, error) {
	manifestJSON, err := executor.dependencies.marshalRouteManifestJSON(manifest)
	if err != nil {
		return "", fmt.Errorf("marshal route manifest: %w", err)
	}

	filename := routeManifestFilename(manifestJSON)

	outPath := filepath.Join(v.Wave.GetStaticPublicOutDir(), filename)
	if err := executor.dependencies.writeRouteManifestJSON(
		outPath,
		manifestJSON,
		buildArtifactFileMode,
	); err != nil {
		return "", fmt.Errorf("write route manifest: %w", err)
	}

	return filename, nil
}

type stageOnePathsArtifactSnapshot = buildArtifactFileSnapshot

type routeManifestStateSnapshot struct {
	previousRouteManifestFile string
	routeManifest             map[string]int
}

type routeManifestCommitInput struct {
	expectedBuildID   string
	routeManifestFile string
}

type routeManifestStateReader interface {
	Vorma() *vormaruntime.Vorma
	GetPaths() map[string]*vormaruntime.Path
}

var errRouteBuildRuntimeStateSuperseded = errors.New(
	"route build runtime state superseded by newer build",
)

func writeRouteArtifactsWithoutHoldingRuntimeLock(v *vormaruntime.Vorma) error {
	return defaultRouteRegistryBuildExecutor.writeRouteArtifactsWithoutHoldingRuntimeLock(v)
}

func (executor routeRegistryBuildExecutor) writeRouteArtifactsWithoutHoldingRuntimeLock(
	v *vormaruntime.Vorma,
) error {
	runtimeStateSnapshot := executor.dependencies.captureRouteBuildRuntimeStateWithReadLock(v)
	manifestStateSnapshot := executor.dependencies.captureRouteManifestStateWithRuntimeLock(
		v,
		runtimeStateSnapshot,
	)

	stageOnePathsArtifactPath := stageOnePathsArtifactOutputPath(v)
	stageOnePathsArtifactSnapshot, err := executor.captureStageOnePathsArtifactSnapshot(
		stageOnePathsArtifactPath,
	)
	if err != nil {
		return fmt.Errorf("snapshot stage-one paths artifact: %w", err)
	}

	manifestFile, err := executor.writeRouteManifestToDisk(v, manifestStateSnapshot.routeManifest)
	if err != nil {
		return fmt.Errorf("write route manifest: %w", err)
	}

	skipRollbackForSupersededRuntimeState := false
	transactionErr := runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				if !executor.dependencies.isRouteBuildRuntimeStateSnapshotCurrent(
					v,
					runtimeStateSnapshot,
				) {
					skipRollbackForSupersededRuntimeState = true
					return errRouteBuildRuntimeStateSuperseded
				}

				if err := executor.dependencies.writeStageOnePathsJSONForRuntimeState(
					v,
					runtimeStateSnapshot,
					manifestFile,
				); err != nil {
					return fmt.Errorf("write paths JSON: %w", err)
				}
				if !executor.dependencies.isRouteBuildRuntimeStateSnapshotCurrent(
					v,
					runtimeStateSnapshot,
				) {
					skipRollbackForSupersededRuntimeState = true
					return errRouteBuildRuntimeStateSuperseded
				}

				if err := executor.dependencies.writeGeneratedTypeScriptForRuntimeState(
					v,
					runtimeStateSnapshot,
				); err != nil {
					return fmt.Errorf("write generated TypeScript: %w", err)
				}
				if !executor.dependencies.isRouteBuildRuntimeStateSnapshotCurrent(
					v,
					runtimeStateSnapshot,
				) {
					skipRollbackForSupersededRuntimeState = true
					return errRouteBuildRuntimeStateSuperseded
				}
				return nil
			},
			rollbackOnFailure: func() error {
				if skipRollbackForSupersededRuntimeState {
					return nil
				}
				return executor.cleanupRouteArtifactsAfterWriteFailure(
					v,
					manifestFile,
					manifestStateSnapshot.previousRouteManifestFile,
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
		if errors.Is(transactionErr, errRouteBuildRuntimeStateSuperseded) {
			return nil
		}
		return transactionErr
	}

	executor.dependencies.commitRouteManifestFileWithRuntimeLock(
		v,
		routeManifestCommitInput{
			expectedBuildID:   runtimeStateSnapshot.buildID,
			routeManifestFile: manifestFile,
		},
	)
	return nil
}

func routeBuildRuntimeStateSnapshotIsCurrent(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
) bool {
	var currentBuildID string
	v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
		currentBuildID = l.GetBuildID()
	})
	return currentBuildID == runtimeStateSnapshot.buildID
}

func shouldCommitRouteManifestFileForRuntimeState(
	currentBuildID string,
	expectedBuildID string,
) bool {
	return currentBuildID == expectedBuildID
}

func stageOnePathsArtifactOutputPath(v *vormaruntime.Vorma) string {
	return pathsOutputPath(v, vormaruntime.VormaPathsStageOneJSONFileName)
}

func captureStageOnePathsArtifactSnapshot(
	stageOnePathsArtifactPath string,
) (stageOnePathsArtifactSnapshot, error) {
	return defaultRouteRegistryBuildExecutor.captureStageOnePathsArtifactSnapshot(
		stageOnePathsArtifactPath,
	)
}

func (executor routeRegistryBuildExecutor) captureStageOnePathsArtifactSnapshot(
	stageOnePathsArtifactPath string,
) (stageOnePathsArtifactSnapshot, error) {
	return captureBuildArtifactFileSnapshot(
		stageOnePathsArtifactPath,
		executor.dependencies.readStageOnePathsArtifact,
	)
}

func cleanupRouteArtifactsAfterWriteFailure(
	v *vormaruntime.Vorma,
	manifestFile string,
	previousRouteManifestFile string,
	stageOnePathsArtifactPath string,
	stageOnePathsSnapshot stageOnePathsArtifactSnapshot,
) error {
	return defaultRouteRegistryBuildExecutor.cleanupRouteArtifactsAfterWriteFailure(
		v,
		manifestFile,
		previousRouteManifestFile,
		stageOnePathsArtifactPath,
		stageOnePathsSnapshot,
	)
}

func (executor routeRegistryBuildExecutor) cleanupRouteArtifactsAfterWriteFailure(
	v *vormaruntime.Vorma,
	manifestFile string,
	previousRouteManifestFile string,
	stageOnePathsArtifactPath string,
	stageOnePathsSnapshot stageOnePathsArtifactSnapshot,
) error {
	artifactCleanupErrors := make([]error, 0, 2)
	if shouldRemoveRouteManifestArtifactAfterWriteFailure(manifestFile, previousRouteManifestFile) {
		if err := executor.removeRouteManifestArtifactFile(v, manifestFile); err != nil {
			artifactCleanupErrors = append(
				artifactCleanupErrors,
				fmt.Errorf("cleanup route manifest artifact: %w", err),
			)
		}
	}
	if err := executor.restoreStageOnePathsArtifactFromSnapshot(
		stageOnePathsArtifactPath,
		stageOnePathsSnapshot,
	); err != nil {
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
	return defaultRouteRegistryBuildExecutor.removeRouteManifestArtifactFile(v, manifestFile)
}

func (executor routeRegistryBuildExecutor) removeRouteManifestArtifactFile(
	v *vormaruntime.Vorma,
	manifestFile string,
) error {
	manifestFilePath := filepath.Join(v.Wave.GetStaticPublicOutDir(), manifestFile)
	err := executor.dependencies.removeRouteManifestJSON(manifestFilePath)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func restoreStageOnePathsArtifactFromSnapshot(
	stageOnePathsArtifactPath string,
	stageOnePathsSnapshot stageOnePathsArtifactSnapshot,
) error {
	return defaultRouteRegistryBuildExecutor.restoreStageOnePathsArtifactFromSnapshot(
		stageOnePathsArtifactPath,
		stageOnePathsSnapshot,
	)
}

func (executor routeRegistryBuildExecutor) restoreStageOnePathsArtifactFromSnapshot(
	stageOnePathsArtifactPath string,
	stageOnePathsSnapshot stageOnePathsArtifactSnapshot,
) error {
	return restoreBuildArtifactFileSnapshot(
		stageOnePathsArtifactPath,
		stageOnePathsSnapshot,
		executor.dependencies.writeStageOnePathsArtifact,
		executor.dependencies.removeStageOnePathsArtifact,
	)
}

func generateRouteManifest(l routeManifestStateReader, nestedRouter *mux.NestedRouter) map[string]int {
	return generateRouteManifestFromPaths(l.GetPaths(), nestedRouter)
}

func generateRouteManifestFromPaths(
	paths map[string]*vormaruntime.Path,
	nestedRouter *mux.NestedRouter,
) map[string]int {
	manifest := make(map[string]int)
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

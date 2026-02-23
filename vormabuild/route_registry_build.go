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
	"github.com/vormadev/vorma/kit/nestedmux"
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
	readRouteManifestFileWithRuntimeLock      func(*vormaruntime.Vorma) string
	generateRouteManifestFromPaths            func(map[string]*vormaruntime.Path, *nestedmux.Router) map[string]int
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
			return writePathsToDiskStageOneFromRuntimeState(
				v,
				runtimeStateSnapshot,
				routeManifestFile,
			)
		},
		writeGeneratedTypeScriptForRuntimeState: writeGeneratedTSForRouteBuildRuntimeState,
		captureRouteBuildRuntimeStateWithReadLock: func(v *vormaruntime.Vorma) routeBuildRuntimeStateSnapshot {
			var runtimeStateSnapshot routeBuildRuntimeStateSnapshot
			v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
				runtimeStateSnapshot = captureRouteBuildRuntimeState(l)
			})
			return runtimeStateSnapshot
		},
		readRouteManifestFileWithRuntimeLock: func(v *vormaruntime.Vorma) string {
			var previousRouteManifestFile string
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				previousRouteManifestFile = l.RouteManifestFile()
			})
			return previousRouteManifestFile
		},
		generateRouteManifestFromPaths:          generateRouteManifestFromPaths,
		isRouteBuildRuntimeStateSnapshotCurrent: routeBuildRuntimeStateSnapshotIsCurrent,
		commitRouteManifestFileWithRuntimeLock: func(
			v *vormaruntime.Vorma,
			commitInput routeManifestCommitInput,
		) bool {
			manifestCommitted := false
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				if !shouldCommitRouteManifestFileForRuntimeState(
					l.BuildID(),
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
	if dependencies.readRouteManifestFileWithRuntimeLock == nil {
		dependencies.readRouteManifestFileWithRuntimeLock = defaultDependencies.readRouteManifestFileWithRuntimeLock
	}
	if dependencies.generateRouteManifestFromPaths == nil {
		dependencies.generateRouteManifestFromPaths = defaultDependencies.generateRouteManifestFromPaths
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

func (executor routeRegistryBuildExecutor) writeRouteArtifacts(
	l *vormaruntime.LockedVorma,
) error {
	v := l.Vorma()
	previousRouteManifestFile := l.RouteManifestFile()
	stageOnePathsArtifactPath := stageOnePathsArtifactOutputPath(v)

	stageOnePathsArtifactSnapshot, err := executor.captureStageOnePathsArtifact(
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
					v.Log.Error(
						"cleanup route artifacts after panic failed",
						"error",
						rollbackErr,
					)
				}
			},
		},
	)
	if transactionErr != nil {
		return transactionErr
	}

	commitRuntimeStateWithLock(
		l,
		runtimeStateCommitInput{
			shouldCommitRouteManifestFile: true,
			routeManifestFile:             manifestFile,
		},
	)
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

	commitRuntimeStateWithLock(
		l,
		runtimeStateCommitInput{
			shouldCommitRouteManifestFile: true,
			routeManifestFile:             manifestFile,
		},
	)
	return nil
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

func writeRouteManifestToDisk(
	v *vormaruntime.Vorma,
	manifest map[string]int,
) (string, error) {
	return defaultRouteRegistryBuildExecutor.writeRouteManifestToDisk(
		v,
		manifest,
	)
}

func (executor routeRegistryBuildExecutor) writeRouteManifestToDisk(
	v *vormaruntime.Vorma,
	manifest map[string]int,
) (string, error) {
	manifestJSON, err := executor.dependencies.marshalRouteManifestJSON(
		manifest,
	)
	if err != nil {
		return "", fmt.Errorf("marshal route manifest: %w", err)
	}

	filename := routeManifestFilename(manifestJSON)

	outPath := filepath.Join(v.Wave.StaticPublicOutDir(), filename)
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

type routeArtifactWritePlan struct {
	runtimeStateSnapshot      routeBuildRuntimeStateSnapshot
	manifestStateSnapshot     routeManifestStateSnapshot
	stageOnePathsArtifactPath string
	stageOnePathsSnapshot     stageOnePathsArtifactSnapshot
}

type routeManifestStateReader interface {
	Vorma() *vormaruntime.Vorma
	Paths() map[string]*vormaruntime.Path
}

var errRouteBuildRuntimeStateSuperseded = errors.New(
	"route build runtime state superseded by newer build",
)

func writeRouteArtifactsWithoutHoldingRuntimeLock(v *vormaruntime.Vorma) error {
	return defaultRouteRegistryBuildExecutor.writeRouteArtifactsWithoutHoldingRuntimeLock(
		v,
	)
}

func (executor routeRegistryBuildExecutor) writeRouteArtifactsWithoutHoldingRuntimeLock(
	v *vormaruntime.Vorma,
) error {
	plannedWrite, err := executor.planRouteArtifactWrite(v)
	if err != nil {
		return err
	}

	manifestFile, shouldCommit, err := executor.stageRouteArtifactWrite(
		v,
		plannedWrite,
	)
	if err != nil {
		return err
	}
	if !shouldCommit {
		return nil
	}

	executor.commitRouteArtifactWrite(v, plannedWrite, manifestFile)
	return nil
}

func (executor routeRegistryBuildExecutor) planRouteArtifactWrite(
	v *vormaruntime.Vorma,
) (routeArtifactWritePlan, error) {
	runtimeStateSnapshot := executor.dependencies.captureRouteBuildRuntimeStateWithReadLock(
		v,
	)
	manifestStateSnapshot := routeManifestStateSnapshot{
		previousRouteManifestFile: executor.dependencies.readRouteManifestFileWithRuntimeLock(
			v,
		),
		routeManifest: executor.dependencies.generateRouteManifestFromPaths(
			runtimeStateSnapshot.paths,
			v.LoadersRouter().NestedRouter,
		),
	}

	stageOnePathsArtifactPath := stageOnePathsArtifactOutputPath(v)
	stageOnePathsSnapshot, err := executor.captureStageOnePathsArtifact(
		stageOnePathsArtifactPath,
	)
	if err != nil {
		return routeArtifactWritePlan{}, fmt.Errorf(
			"snapshot stage-one paths artifact: %w",
			err,
		)
	}

	return routeArtifactWritePlan{
		runtimeStateSnapshot:      runtimeStateSnapshot,
		manifestStateSnapshot:     manifestStateSnapshot,
		stageOnePathsArtifactPath: stageOnePathsArtifactPath,
		stageOnePathsSnapshot:     stageOnePathsSnapshot,
	}, nil
}

func (executor routeRegistryBuildExecutor) stageRouteArtifactWrite(
	v *vormaruntime.Vorma,
	plannedWrite routeArtifactWritePlan,
) (string, bool, error) {
	manifestFile, err := executor.writeRouteManifestToDisk(
		v,
		plannedWrite.manifestStateSnapshot.routeManifest,
	)
	if err != nil {
		return "", false, fmt.Errorf("write route manifest: %w", err)
	}

	skipRollbackForSupersededRuntimeState := false
	transactionErr := runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				if !executor.dependencies.isRouteBuildRuntimeStateSnapshotCurrent(
					v,
					plannedWrite.runtimeStateSnapshot,
				) {
					skipRollbackForSupersededRuntimeState = true
					return errRouteBuildRuntimeStateSuperseded
				}

				if err := executor.dependencies.writeStageOnePathsJSONForRuntimeState(
					v,
					plannedWrite.runtimeStateSnapshot,
					manifestFile,
				); err != nil {
					return fmt.Errorf("write paths JSON: %w", err)
				}
				if !executor.dependencies.isRouteBuildRuntimeStateSnapshotCurrent(
					v,
					plannedWrite.runtimeStateSnapshot,
				) {
					skipRollbackForSupersededRuntimeState = true
					return errRouteBuildRuntimeStateSuperseded
				}

				if err := executor.dependencies.writeGeneratedTypeScriptForRuntimeState(
					v,
					plannedWrite.runtimeStateSnapshot,
				); err != nil {
					return fmt.Errorf("write generated TypeScript: %w", err)
				}
				if !executor.dependencies.isRouteBuildRuntimeStateSnapshotCurrent(
					v,
					plannedWrite.runtimeStateSnapshot,
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
					plannedWrite.manifestStateSnapshot.previousRouteManifestFile,
					plannedWrite.stageOnePathsArtifactPath,
					plannedWrite.stageOnePathsSnapshot,
				)
			},
			logRollbackFailureAfterPanic: func(rollbackErr error) {
				if v.Log != nil {
					v.Log.Error(
						"cleanup route artifacts after panic failed",
						"error",
						rollbackErr,
					)
				}
			},
		},
	)
	if transactionErr != nil {
		if errors.Is(transactionErr, errRouteBuildRuntimeStateSuperseded) {
			return "", false, nil
		}
		return "", false, transactionErr
	}

	return manifestFile, true, nil
}

func (executor routeRegistryBuildExecutor) commitRouteArtifactWrite(
	v *vormaruntime.Vorma,
	plannedWrite routeArtifactWritePlan,
	manifestFile string,
) {
	executor.dependencies.commitRouteManifestFileWithRuntimeLock(
		v,
		routeManifestCommitInput{
			expectedBuildID:   plannedWrite.runtimeStateSnapshot.buildID,
			routeManifestFile: manifestFile,
		},
	)
}

func routeBuildRuntimeStateSnapshotIsCurrent(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
) bool {
	var currentBuildID string
	v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
		currentBuildID = l.BuildID()
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

func (executor routeRegistryBuildExecutor) captureStageOnePathsArtifact(
	stageOnePathsArtifactPath string,
) (stageOnePathsArtifactSnapshot, error) {
	return captureBuildArtifactFile(
		stageOnePathsArtifactPath,
		executor.dependencies.readStageOnePathsArtifact,
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
	if shouldRemoveRouteManifestArtifactAfterWriteFailure(
		manifestFile,
		previousRouteManifestFile,
	) {
		if err := executor.removeRouteManifestArtifactFile(v, manifestFile); err != nil {
			artifactCleanupErrors = append(
				artifactCleanupErrors,
				fmt.Errorf("cleanup route manifest artifact: %w", err),
			)
		}
	}
	if err := executor.restoreStageOnePathsArtifactFromState(
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

func (executor routeRegistryBuildExecutor) removeRouteManifestArtifactFile(
	v *vormaruntime.Vorma,
	manifestFile string,
) error {
	manifestFilePath := filepath.Join(v.Wave.StaticPublicOutDir(), manifestFile)
	err := executor.dependencies.removeRouteManifestJSON(manifestFilePath)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func (executor routeRegistryBuildExecutor) restoreStageOnePathsArtifactFromState(
	stageOnePathsArtifactPath string,
	stageOnePathsSnapshot stageOnePathsArtifactSnapshot,
) error {
	return restoreBuildArtifactFile(
		stageOnePathsArtifactPath,
		stageOnePathsSnapshot,
		executor.dependencies.writeStageOnePathsArtifact,
		executor.dependencies.removeStageOnePathsArtifact,
	)
}

func generateRouteManifest(
	l routeManifestStateReader,
	nestedRouter *nestedmux.Router,
) map[string]int {
	return generateRouteManifestFromPaths(l.Paths(), nestedRouter)
}

func generateRouteManifestFromPaths(
	paths map[string]*vormaruntime.Path,
	nestedRouter *nestedmux.Router,
) map[string]int {
	manifest := make(map[string]int)
	for _, currentPath := range paths {
		manifest[currentPath.OriginalPattern] = routeManifestServerLoaderFlag(
			nestedRouter,
			currentPath.OriginalPattern,
		)
	}

	return manifest
}

func routeManifestFilename(manifestJSON []byte) string {
	hash := cryptoutil.Sha256Hash(manifestJSON)
	hashStr := base64.RawURLEncoding.EncodeToString(hash[:8])
	return fmt.Sprintf(
		"%s%s.json",
		vormaruntime.VormaRouteManifestPrefix,
		hashStr,
	)
}

func routeManifestServerLoaderFlag(
	nestedRouter *nestedmux.Router,
	pattern string,
) int {
	if nestedRouter.HasTaskHandler(pattern) {
		return 1
	}
	return 0
}

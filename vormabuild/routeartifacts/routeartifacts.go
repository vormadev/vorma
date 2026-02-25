// Package routeartifacts owns route-manifest and stage one/two route-artifact
// orchestration for vormabuild flows.
//
// This package isolates plan/stage/commit behavior for route artifact writes so
// build orchestration can compose it without re-implementing rollback and
// runtime-state commit rules.
package routeartifacts

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vormabuild/artifactio"
	"github.com/vormadev/vorma/vormabuild/buildlifecycle"
	"github.com/vormadev/vorma/vormabuild/tsartifactgen"
)

const (
	buildArtifactDirectoryMode os.FileMode = 0o755
	buildArtifactFileMode      os.FileMode = 0o644
)

func WriteRouteArtifacts(l *vormaruntime.LockedVorma) error {
	return writeRouteArtifacts(l)
}

func WriteAndSetRouteManifest(l *vormaruntime.LockedVorma) error {
	return writeAndSetRouteManifest(l)
}

func WriteRouteManifestToDisk(
	v *vormaruntime.Vorma,
	manifest map[string]int,
) (string, error) {
	return writeRouteManifestToDisk(v, manifest)
}

func WriteRouteArtifactsWithoutHoldingRuntimeLock(v *vormaruntime.Vorma) error {
	return writeRouteArtifactsWithoutHoldingRuntimeLock(v)
}

func GenerateRouteManifest(
	l RouteManifestStateReader,
	nestedRouter *nestedmux.Router,
) map[string]int {
	return generateRouteManifest(l, nestedRouter)
}

func GenerateRouteManifestFromPaths(
	paths map[string]*vormaruntime.Path,
	nestedRouter *nestedmux.Router,
) map[string]int {
	return generateRouteManifestFromPaths(paths, nestedRouter)
}

func RouteManifestFilename(manifestJSON []byte) string {
	return routeManifestFilename(manifestJSON)
}

func RouteManifestServerLoaderFlag(
	nestedRouter *nestedmux.Router,
	pattern string,
) int {
	return routeManifestServerLoaderFlag(nestedRouter, pattern)
}

func StageOnePathsArtifactOutputPath(v *vormaruntime.Vorma) string {
	return stageOnePathsArtifactOutputPath(v)
}

func ToPathsFileStageTwo(
	v *vormaruntime.Vorma,
) (*runtimepaths.PathsFile, error) {
	return toPathsFileStageTwo(v)
}

func WritePathsToDiskStageTwo(
	v *vormaruntime.Vorma,
	pathsFile *runtimepaths.PathsFile,
) error {
	return writePathsToDiskStageTwo(v, pathsFile)
}

func ShouldRemoveRouteManifestArtifactAfterWriteFailure(
	manifestFile string,
	previousRouteManifestFile string,
) bool {
	return shouldRemoveRouteManifestArtifactAfterWriteFailure(
		manifestFile,
		previousRouteManifestFile,
	)
}

type routeRegistryBuildDependencies struct {
	marshalRouteManifestJSON                  func(any) ([]byte, error)
	writeRouteManifestJSON                    func(string, []byte, os.FileMode) error
	removeRouteManifestJSON                   func(string) error
	readStageOnePathsArtifact                 func(string) ([]byte, error)
	writeStageOnePathsArtifact                func(string, []byte, os.FileMode) error
	removeStageOnePathsArtifact               func(string) error
	writeGeneratedTypeScript                  func(*vormaruntime.LockedVorma) error
	writeStageOnePathsJSONForRuntimeState     func(*vormaruntime.Vorma, buildlifecycle.RouteBuildRuntimeStateSnapshot, string) error
	writeGeneratedTypeScriptForRuntimeState   func(*vormaruntime.Vorma, buildlifecycle.RouteBuildRuntimeStateSnapshot) error
	captureRouteBuildRuntimeStateWithReadLock func(*vormaruntime.Vorma) buildlifecycle.RouteBuildRuntimeStateSnapshot
	readRouteManifestFileWithRuntimeLock      func(*vormaruntime.Vorma) string
	generateRouteManifestFromPaths            func(map[string]*vormaruntime.Path, *nestedmux.Router) map[string]int
	isRouteBuildRuntimeStateSnapshotCurrent   func(*vormaruntime.Vorma, buildlifecycle.RouteBuildRuntimeStateSnapshot) bool
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
		writeRouteManifestJSON:      artifactio.WriteFileAtomically,
		removeRouteManifestJSON:     os.Remove,
		readStageOnePathsArtifact:   os.ReadFile,
		writeStageOnePathsArtifact:  artifactio.WriteFileAtomically,
		removeStageOnePathsArtifact: os.Remove,
		writeGeneratedTypeScript: func(l *vormaruntime.LockedVorma) error {
			return tsartifactgen.WriteGeneratedTS(
				l,
				tsartifactgen.WriteDependencies{
					WriteGeneratedTSFile: artifactio.WriteFileAtomically,
				},
			)
		},
		writeStageOnePathsJSONForRuntimeState: func(
			v *vormaruntime.Vorma,
			runtimeStateSnapshot buildlifecycle.RouteBuildRuntimeStateSnapshot,
			routeManifestFile string,
		) error {
			return writePathsToDiskStageOneFromRuntimeState(
				v,
				runtimeStateSnapshot,
				routeManifestFile,
			)
		},
		writeGeneratedTypeScriptForRuntimeState: func(
			v *vormaruntime.Vorma,
			runtimeStateSnapshot buildlifecycle.RouteBuildRuntimeStateSnapshot,
		) error {
			return tsartifactgen.WriteGeneratedTSForRuntimeState(
				v,
				tsartifactgen.RuntimeStateSnapshot{
					Paths:   runtimeStateSnapshot.Paths,
					BuildID: runtimeStateSnapshot.BuildID,
				},
				tsartifactgen.WriteDependencies{
					WriteGeneratedTSFile: artifactio.WriteFileAtomically,
				},
			)
		},
		captureRouteBuildRuntimeStateWithReadLock: func(v *vormaruntime.Vorma) buildlifecycle.RouteBuildRuntimeStateSnapshot {
			var runtimeStateSnapshot buildlifecycle.RouteBuildRuntimeStateSnapshot
			v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
				runtimeStateSnapshot = buildlifecycle.CaptureRouteBuildRuntimeState(
					l,
				)
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
		isRouteBuildRuntimeStateSnapshotCurrent: buildlifecycle.RouteBuildRuntimeStateSnapshotIsCurrent,
		commitRouteManifestFileWithRuntimeLock: func(
			v *vormaruntime.Vorma,
			commitInput routeManifestCommitInput,
		) bool {
			manifestCommitted := false
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				if !buildlifecycle.ShouldCommitRouteManifestFileForRuntimeState(
					l.BuildID(),
					commitInput.expectedBuildID,
				) {
					return
				}
				buildlifecycle.CommitRuntimeStateWithLock(
					l,
					buildlifecycle.RuntimeStateCommitInput{
						ShouldCommitRouteManifestFile: true,
						RouteManifestFile:             commitInput.routeManifestFile,
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

	transactionErr := buildlifecycle.RunWithRollbackOnFailureAndPanic(
		buildlifecycle.RollbackTransactionOptions{
			Run: func() error {
				if err := writePathsToDiskStageOneWithRouteManifest(l, manifestFile); err != nil {
					return fmt.Errorf("write paths JSON: %w", err)
				}

				if err := executor.dependencies.writeGeneratedTypeScript(l); err != nil {
					return fmt.Errorf("write generated TypeScript: %w", err)
				}
				return nil
			},
			RollbackOnFailure: func() error {
				return executor.cleanupRouteArtifactsAfterWriteFailure(
					v,
					manifestFile,
					previousRouteManifestFile,
					stageOnePathsArtifactPath,
					stageOnePathsArtifactSnapshot,
				)
			},
			LogRollbackFailureAfterPanic: func(rollbackErr error) {
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

	buildlifecycle.CommitRuntimeStateWithLock(
		l,
		buildlifecycle.RuntimeStateCommitInput{
			ShouldCommitRouteManifestFile: true,
			RouteManifestFile:             manifestFile,
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

	buildlifecycle.CommitRuntimeStateWithLock(
		l,
		buildlifecycle.RuntimeStateCommitInput{
			ShouldCommitRouteManifestFile: true,
			RouteManifestFile:             manifestFile,
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

type stageOnePathsArtifactSnapshot = artifactio.BuildArtifactFileSnapshot

type routeManifestStateSnapshot struct {
	previousRouteManifestFile string
	routeManifest             map[string]int
}

type routeManifestCommitInput struct {
	expectedBuildID   string
	routeManifestFile string
}

type routeArtifactWritePlan struct {
	runtimeStateSnapshot      buildlifecycle.RouteBuildRuntimeStateSnapshot
	manifestStateSnapshot     routeManifestStateSnapshot
	stageOnePathsArtifactPath string
	stageOnePathsSnapshot     stageOnePathsArtifactSnapshot
}

type RouteManifestStateReader interface {
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
			runtimeStateSnapshot.Paths,
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
	transactionErr := buildlifecycle.RunWithRollbackOnFailureAndPanic(
		buildlifecycle.RollbackTransactionOptions{
			Run: func() error {
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
			RollbackOnFailure: func() error {
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
			LogRollbackFailureAfterPanic: func(rollbackErr error) {
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
			expectedBuildID:   plannedWrite.runtimeStateSnapshot.BuildID,
			routeManifestFile: manifestFile,
		},
	)
}

func stageOnePathsArtifactOutputPath(v *vormaruntime.Vorma) string {
	return pathsOutputPath(v, runtimepaths.VormaPathsStageOneJSONFileName)
}

func (executor routeRegistryBuildExecutor) captureStageOnePathsArtifact(
	stageOnePathsArtifactPath string,
) (stageOnePathsArtifactSnapshot, error) {
	return artifactio.CaptureBuildArtifactFile(
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
	return artifactio.RestoreBuildArtifactFile(
		stageOnePathsArtifactPath,
		stageOnePathsSnapshot,
		executor.dependencies.writeStageOnePathsArtifact,
		executor.dependencies.removeStageOnePathsArtifact,
	)
}

func generateRouteManifest(
	l RouteManifestStateReader,
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

type stageOnePathsWriterDependencies struct {
	marshalStageOnePathsFile         func(*runtimepaths.PathsFile) ([]byte, error)
	makeStageOnePathsOutputDirectory func(string, os.FileMode) error
	writeStageOnePathsJSON           func(string, []byte, os.FileMode) error
}

type stageOnePathsWriterExecutor struct {
	dependencies stageOnePathsWriterDependencies
}

var defaultStageOnePathsWriterExecutor = newStageOnePathsWriterExecutor(
	stageOnePathsWriterDependencies{},
)

func defaultStageOnePathsWriterDependencies() stageOnePathsWriterDependencies {
	return stageOnePathsWriterDependencies{
		marshalStageOnePathsFile: func(pathsFile *runtimepaths.PathsFile) ([]byte, error) {
			return json.MarshalIndent(pathsFile, "", "\t")
		},
		makeStageOnePathsOutputDirectory: os.MkdirAll,
		writeStageOnePathsJSON:           artifactio.WriteFileAtomically,
	}
}

func normalizeStageOnePathsWriterDependencies(
	dependencies stageOnePathsWriterDependencies,
) stageOnePathsWriterDependencies {
	defaultDependencies := defaultStageOnePathsWriterDependencies()
	if dependencies.marshalStageOnePathsFile == nil {
		dependencies.marshalStageOnePathsFile = defaultDependencies.marshalStageOnePathsFile
	}
	if dependencies.makeStageOnePathsOutputDirectory == nil {
		dependencies.makeStageOnePathsOutputDirectory = defaultDependencies.makeStageOnePathsOutputDirectory
	}
	if dependencies.writeStageOnePathsJSON == nil {
		dependencies.writeStageOnePathsJSON = defaultDependencies.writeStageOnePathsJSON
	}
	return dependencies
}

func newStageOnePathsWriterExecutor(
	dependencies stageOnePathsWriterDependencies,
) stageOnePathsWriterExecutor {
	return stageOnePathsWriterExecutor{
		dependencies: normalizeStageOnePathsWriterDependencies(dependencies),
	}
}

func writePathsToDiskStageOne(l *vormaruntime.LockedVorma) error {
	return defaultStageOnePathsWriterExecutor.writePathsToDiskStageOneWithRouteManifest(
		l,
		l.RouteManifestFile(),
	)
}

func writePathsToDiskStageOneWithRouteManifest(
	l *vormaruntime.LockedVorma,
	routeManifestFile string,
) error {
	return defaultStageOnePathsWriterExecutor.writePathsToDiskStageOneWithRouteManifest(
		l,
		routeManifestFile,
	)
}

func (executor stageOnePathsWriterExecutor) writePathsToDiskStageOneWithRouteManifest(
	l *vormaruntime.LockedVorma,
	routeManifestFile string,
) error {
	return executor.writeStageOnePathsFileToDisk(
		l.Vorma(),
		stageOnePathsFile(l, routeManifestFile),
	)
}

func writePathsToDiskStageOneFromRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot buildlifecycle.RouteBuildRuntimeStateSnapshot,
	routeManifestFile string,
) error {
	return defaultStageOnePathsWriterExecutor.writePathsToDiskStageOneFromRuntimeState(
		v,
		runtimeStateSnapshot,
		routeManifestFile,
	)
}

func (executor stageOnePathsWriterExecutor) writePathsToDiskStageOneFromRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot buildlifecycle.RouteBuildRuntimeStateSnapshot,
	routeManifestFile string,
) error {
	return executor.writeStageOnePathsFileToDisk(
		v,
		stageOnePathsFileFromRuntimeState(
			v,
			runtimeStateSnapshot,
			routeManifestFile,
		),
	)
}

func (executor stageOnePathsWriterExecutor) writeStageOnePathsFileToDisk(
	v *vormaruntime.Vorma,
	stageOnePathsFileData *runtimepaths.PathsFile,
) error {
	pathsJSONOut := pathsOutputPath(
		v,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)
	pathsAsJSON, err := executor.dependencies.marshalStageOnePathsFile(
		stageOnePathsFileData,
	)
	if err != nil {
		return fmt.Errorf("marshal stage-one paths file: %w", err)
	}

	if err := executor.dependencies.makeStageOnePathsOutputDirectory(
		filepath.Dir(pathsJSONOut),
		buildArtifactDirectoryMode,
	); err != nil {
		return fmt.Errorf("create stage-one paths output directory: %w", err)
	}
	if err := executor.dependencies.writeStageOnePathsJSON(
		pathsJSONOut,
		pathsAsJSON,
		buildArtifactFileMode,
	); err != nil {
		return fmt.Errorf("write stage-one paths JSON: %w", err)
	}
	return nil
}

func stageOnePathsFile(
	l *vormaruntime.LockedVorma,
	routeManifestFile string,
) *runtimepaths.PathsFile {
	v := l.Vorma()
	return &runtimepaths.PathsFile{
		Stage:             "one",
		Paths:             toRuntimePathsMap(l.Paths()),
		ClientEntrySrc:    v.Config.ClientEntry,
		BuildID:           l.BuildID(),
		RouteManifestFile: routeManifestFile,
	}
}

func stageOnePathsFileFromRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot buildlifecycle.RouteBuildRuntimeStateSnapshot,
	routeManifestFile string,
) *runtimepaths.PathsFile {
	return &runtimepaths.PathsFile{
		Stage:             "one",
		Paths:             toRuntimePathsMap(runtimeStateSnapshot.Paths),
		ClientEntrySrc:    v.Config.ClientEntry,
		BuildID:           runtimeStateSnapshot.BuildID,
		RouteManifestFile: routeManifestFile,
	}
}

type viteManifestApplicationResult struct {
	clientEntryOut    string
	clientEntryDeps   []string
	depToCSSBundleMap map[string][]string
}

func applyViteManifestToPaths(
	viteManifest viteutil.Manifest,
	paths map[string]*vormaruntime.Path,
	cleanClientEntry string,
) viteManifestApplicationResult {
	result := viteManifestApplicationResult{
		clientEntryDeps:   []string{},
		depToCSSBundleMap: make(map[string][]string),
	}
	pathsBySourcePath := indexPathsBySourcePath(paths)

	for key, chunk := range viteManifest {
		cleanChunkOutPath := filepath.Base(chunk.File)

		if len(chunk.CSS) > 0 {
			result.depToCSSBundleMap[cleanChunkOutPath] = collectCSSBundleFileNames(
				chunk.CSS,
			)
		}

		dependencies := viteutil.FindAllDependencies(viteManifest, key)

		if chunk.IsEntry && cleanClientEntry == chunk.Src {
			result.clientEntryOut = cleanChunkOutPath
			result.clientEntryDeps = removeDependency(
				dependencies,
				result.clientEntryOut,
			)
			continue
		}

		updateRoutePathsForChunk(
			pathsBySourcePath,
			chunk.Src,
			cleanChunkOutPath,
			dependencies,
		)
	}

	return result
}

func collectCSSBundleFileNames(cssFiles []string) []string {
	cssBundleFileNames := make([]string, 0, len(cssFiles))
	for _, cssFile := range cssFiles {
		cssBundleFileNames = append(
			cssBundleFileNames,
			filepath.Base(cssFile),
		)
	}
	return cssBundleFileNames
}

func removeDependency(
	dependencies []string,
	dependencyToRemove string,
) []string {
	dependenciesWithoutTarget := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		if dependency == dependencyToRemove {
			continue
		}
		dependenciesWithoutTarget = append(
			dependenciesWithoutTarget,
			dependency,
		)
	}
	return dependenciesWithoutTarget
}

func updateRoutePathsForChunk(
	pathsBySourcePath map[string][]*vormaruntime.Path,
	chunkSourcePath string,
	chunkOutPath string,
	chunkDependencies []string,
) {
	for _, currentPath := range pathsBySourcePath[chunkSourcePath] {
		currentPath.OutPath = chunkOutPath
		currentPath.Deps = chunkDependencies
	}
}

func indexPathsBySourcePath(
	paths map[string]*vormaruntime.Path,
) map[string][]*vormaruntime.Path {
	pathsBySourcePath := make(map[string][]*vormaruntime.Path)
	for _, currentPath := range paths {
		pathsBySourcePath[currentPath.SrcPath] = append(
			pathsBySourcePath[currentPath.SrcPath],
			currentPath,
		)
	}
	return pathsBySourcePath
}

type stageTwoBuildIDDependencies struct {
	readHTMLTemplate  func(string) ([]byte, error)
	marshalPathsFile  func(any) ([]byte, error)
	summarizePublicFS func(fs.FS) ([]byte, error)
}

type stageTwoBuildIDExecutor struct {
	dependencies stageTwoBuildIDDependencies
}

var defaultStageTwoBuildIDExecutor = newStageTwoBuildIDExecutor(
	stageTwoBuildIDDependencies{},
)

func defaultStageTwoBuildIDDependencies() stageTwoBuildIDDependencies {
	return stageTwoBuildIDDependencies{
		readHTMLTemplate:  os.ReadFile,
		marshalPathsFile:  json.Marshal,
		summarizePublicFS: artifactio.GetFSSummaryHash,
	}
}

func normalizeStageTwoBuildIDDependencies(
	dependencies stageTwoBuildIDDependencies,
) stageTwoBuildIDDependencies {
	defaultDependencies := defaultStageTwoBuildIDDependencies()

	if dependencies.readHTMLTemplate == nil {
		dependencies.readHTMLTemplate = defaultDependencies.readHTMLTemplate
	}
	if dependencies.marshalPathsFile == nil {
		dependencies.marshalPathsFile = defaultDependencies.marshalPathsFile
	}
	if dependencies.summarizePublicFS == nil {
		dependencies.summarizePublicFS = defaultDependencies.summarizePublicFS
	}

	return dependencies
}

func newStageTwoBuildIDExecutor(
	dependencies stageTwoBuildIDDependencies,
) stageTwoBuildIDExecutor {
	return stageTwoBuildIDExecutor{
		dependencies: normalizeStageTwoBuildIDDependencies(dependencies),
	}
}

func computeStageTwoBuildID(
	v *vormaruntime.Vorma,
	pathsFile *runtimepaths.PathsFile,
) (string, error) {
	return defaultStageTwoBuildIDExecutor.computeStageTwoBuildID(v, pathsFile)
}

func (executor stageTwoBuildIDExecutor) computeStageTwoBuildID(
	v *vormaruntime.Vorma,
	pathsFile *runtimepaths.PathsFile,
) (string, error) {
	htmlTemplateContent, err := executor.dependencies.readHTMLTemplate(
		path.Join(v.Wave.PrivateStaticDir(), v.Config.HTMLTemplateLocation),
	)
	if err != nil {
		return "", fmt.Errorf("read HTML template: %w", err)
	}
	htmlContentHash := cryptoutil.Sha256Hash(htmlTemplateContent)

	asJSON, err := executor.dependencies.marshalPathsFile(pathsFile)
	if err != nil {
		return "", fmt.Errorf("marshal paths file: %w", err)
	}
	pathsFileJSONHash := cryptoutil.Sha256Hash(asJSON)

	publicFSSummaryHash, err := executor.dependencies.summarizePublicFS(
		os.DirFS(v.Wave.StaticPublicOutDir()),
	)
	if err != nil {
		return "", fmt.Errorf("get FS summary hash: %w", err)
	}

	fullHash := sha256.New()
	fullHash.Write(htmlContentHash)
	fullHash.Write(pathsFileJSONHash)
	fullHash.Write(publicFSSummaryHash)
	return base64.RawURLEncoding.EncodeToString(fullHash.Sum(nil)[:16]), nil
}

type stageTwoPathsWriteDependencies struct {
	marshalStageTwoPathsFile         func(*runtimepaths.PathsFile) ([]byte, error)
	makeStageTwoPathsOutputDirectory func(string, os.FileMode) error
	writeStageTwoPathsJSON           func(string, []byte, os.FileMode) error
}

type stageTwoPathsWriteExecutor struct {
	dependencies stageTwoPathsWriteDependencies
}

var defaultStageTwoPathsWriteExecutor = newStageTwoPathsWriteExecutor(
	stageTwoPathsWriteDependencies{},
)

func defaultStageTwoPathsWriteDependencies() stageTwoPathsWriteDependencies {
	return stageTwoPathsWriteDependencies{
		marshalStageTwoPathsFile: func(pathsFile *runtimepaths.PathsFile) ([]byte, error) {
			return json.MarshalIndent(pathsFile, "", "\t")
		},
		makeStageTwoPathsOutputDirectory: os.MkdirAll,
		writeStageTwoPathsJSON:           artifactio.WriteFileAtomically,
	}
}

func normalizeStageTwoPathsWriteDependencies(
	dependencies stageTwoPathsWriteDependencies,
) stageTwoPathsWriteDependencies {
	defaultDependencies := defaultStageTwoPathsWriteDependencies()

	if dependencies.marshalStageTwoPathsFile == nil {
		dependencies.marshalStageTwoPathsFile = defaultDependencies.marshalStageTwoPathsFile
	}
	if dependencies.makeStageTwoPathsOutputDirectory == nil {
		dependencies.makeStageTwoPathsOutputDirectory = defaultDependencies.makeStageTwoPathsOutputDirectory
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

func toPathsFileStageTwo(
	v *vormaruntime.Vorma,
) (*runtimepaths.PathsFile, error) {
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

	pathsFile := buildStageTwoPathsFile(v, paths, manifestApplicationResult)

	buildID, err := computeStageTwoBuildID(v, pathsFile)
	if err != nil {
		return nil, err
	}

	applyBuildIDToPathsFile(pathsFile, buildID)
	return pathsFile, nil
}

func writePathsToDiskStageTwo(
	v *vormaruntime.Vorma,
	pathsFile *runtimepaths.PathsFile,
) error {
	return defaultStageTwoPathsWriteExecutor.writePathsToDiskStageTwo(
		v,
		pathsFile,
	)
}

func (executor stageTwoPathsWriteExecutor) writePathsToDiskStageTwo(
	v *vormaruntime.Vorma,
	pathsFile *runtimepaths.PathsFile,
) error {
	pathsAsJSON, err := executor.dependencies.marshalStageTwoPathsFile(
		pathsFile,
	)
	if err != nil {
		return fmt.Errorf("marshal stage-two paths file: %w", err)
	}

	pathsJSONOutputPath := pathsOutputPath(
		v,
		runtimepaths.VormaPathsStageTwoJSONFileName,
	)
	if err := executor.dependencies.makeStageTwoPathsOutputDirectory(
		filepath.Dir(pathsJSONOutputPath),
		buildArtifactDirectoryMode,
	); err != nil {
		return fmt.Errorf("create stage-two paths output directory: %w", err)
	}
	if err := executor.dependencies.writeStageTwoPathsJSON(
		pathsJSONOutputPath,
		pathsAsJSON,
		buildArtifactFileMode,
	); err != nil {
		return fmt.Errorf("write stage-two paths JSON: %w", err)
	}
	return nil
}

func applyBuildIDToPathsFile(
	pathsFile *runtimepaths.PathsFile,
	buildID string,
) {
	pathsFile.BuildID = buildID
}

func buildStageTwoPathsFile(
	v *vormaruntime.Vorma,
	paths map[string]*vormaruntime.Path,
	manifestApplicationResult viteManifestApplicationResult,
) *runtimepaths.PathsFile {
	return &runtimepaths.PathsFile{
		Stage:             "two",
		DepToCSSBundleMap: manifestApplicationResult.depToCSSBundleMap,
		Paths:             toRuntimePathsMap(paths),
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

func toRuntimePathsMap(
	paths map[string]*vormaruntime.Path,
) map[string]*runtimepaths.RoutePath {
	if len(paths) == 0 {
		return map[string]*runtimepaths.RoutePath{}
	}

	convertedPaths := make(map[string]*runtimepaths.RoutePath, len(paths))
	for key, currentPath := range paths {
		if currentPath == nil {
			convertedPaths[key] = nil
			continue
		}
		convertedPaths[key] = &runtimepaths.RoutePath{
			OriginalPattern: currentPath.OriginalPattern,
			SrcPath:         currentPath.SrcPath,
			ExportKey:       currentPath.ExportKey,
			ErrorExportKey:  currentPath.ErrorExportKey,
			OutPath:         currentPath.OutPath,
			Deps:            append([]string(nil), currentPath.Deps...),
		}
	}
	return convertedPaths
}

func pathsOutputPath(v *vormaruntime.Vorma, fileName string) string {
	return filepath.Join(
		v.Wave.StaticPrivateOutDir(),
		runtimepaths.VormaOutDirname,
		fileName,
	)
}

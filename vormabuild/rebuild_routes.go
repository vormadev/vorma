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
	cleanRouteManifestsOnly       func(*vormaruntime.Vorma) error
	writeRouteArtifacts           func(*vormaruntime.Vorma) error
	readRouteManifestArtifact     func(string) ([]byte, error)
	writeRouteManifestArtifact    func(string, []byte, os.FileMode) error
	removeRouteManifestArtifact   func(string) error
	getCurrentBuildIDWithReadLock func(*vormaruntime.Vorma) string
}

type fastRouteRebuildExecutor struct {
	dependencies     fastRouteRebuildDependencies
	artifactExecutor fastRouteRebuildArtifactExecutor
}

type fastRouteRebuildArtifactExecutor struct {
	dependencies fastRouteRebuildArtifactDependencies
}

type fastRouteRebuildBuildIDDependencies struct {
	generateFastRebuildIDSuffix func() (string, error)
}

type fastRouteRebuildIDExecutor struct {
	dependencies fastRouteRebuildBuildIDDependencies
}

func defaultFastRouteRebuildDependencies() fastRouteRebuildDependencies {
	return fastRouteRebuildDependencies{
		parseClientRoutes:             parseClientRoutes,
		newFastRebuildID:              newFastRebuildID,
		runRouteSyncExecution:         runRouteSyncExecution,
		logFastRouteRebuildCompletion: logFastRouteRebuildCompletion,
	}
}

func normalizeFastRouteRebuildDependencies(
	dependencies fastRouteRebuildDependencies,
) fastRouteRebuildDependencies {
	defaultDependencies := defaultFastRouteRebuildDependencies()
	if dependencies.parseClientRoutes == nil {
		dependencies.parseClientRoutes = defaultDependencies.parseClientRoutes
	}
	if dependencies.newFastRebuildID == nil {
		dependencies.newFastRebuildID = defaultDependencies.newFastRebuildID
	}
	if dependencies.runRouteSyncExecution == nil {
		dependencies.runRouteSyncExecution = defaultDependencies.runRouteSyncExecution
	}
	if dependencies.logFastRouteRebuildCompletion == nil {
		dependencies.logFastRouteRebuildCompletion = defaultDependencies.logFastRouteRebuildCompletion
	}
	return dependencies
}

func defaultFastRouteRebuildArtifactDependencies() fastRouteRebuildArtifactDependencies {
	return fastRouteRebuildArtifactDependencies{
		cleanRouteManifestsOnly:       cleanRouteManifestsOnly,
		writeRouteArtifacts:           writeRouteArtifactsWithoutHoldingRuntimeLock,
		readRouteManifestArtifact:     os.ReadFile,
		writeRouteManifestArtifact:    writeFileAtomically,
		removeRouteManifestArtifact:   os.Remove,
		getCurrentBuildIDWithReadLock: currentBuildIDWithReadLock,
	}
}

func normalizeFastRouteRebuildArtifactDependencies(
	dependencies fastRouteRebuildArtifactDependencies,
) fastRouteRebuildArtifactDependencies {
	defaultDependencies := defaultFastRouteRebuildArtifactDependencies()
	if dependencies.cleanRouteManifestsOnly == nil {
		dependencies.cleanRouteManifestsOnly = defaultDependencies.cleanRouteManifestsOnly
	}
	if dependencies.writeRouteArtifacts == nil {
		dependencies.writeRouteArtifacts = defaultDependencies.writeRouteArtifacts
	}
	if dependencies.readRouteManifestArtifact == nil {
		dependencies.readRouteManifestArtifact = defaultDependencies.readRouteManifestArtifact
	}
	if dependencies.writeRouteManifestArtifact == nil {
		dependencies.writeRouteManifestArtifact = defaultDependencies.writeRouteManifestArtifact
	}
	if dependencies.removeRouteManifestArtifact == nil {
		dependencies.removeRouteManifestArtifact = defaultDependencies.removeRouteManifestArtifact
	}
	if dependencies.getCurrentBuildIDWithReadLock == nil {
		dependencies.getCurrentBuildIDWithReadLock = defaultDependencies.getCurrentBuildIDWithReadLock
	}
	return dependencies
}

func newFastRouteRebuildArtifactExecutor(
	dependencies fastRouteRebuildArtifactDependencies,
) fastRouteRebuildArtifactExecutor {
	return fastRouteRebuildArtifactExecutor{
		dependencies: normalizeFastRouteRebuildArtifactDependencies(dependencies),
	}
}

func newFastRouteRebuildExecutor(
	dependencies fastRouteRebuildDependencies,
	artifactDependencies fastRouteRebuildArtifactDependencies,
) fastRouteRebuildExecutor {
	return fastRouteRebuildExecutor{
		dependencies:     normalizeFastRouteRebuildDependencies(dependencies),
		artifactExecutor: newFastRouteRebuildArtifactExecutor(artifactDependencies),
	}
}

var defaultFastRouteRebuildIDExecutor = newFastRouteRebuildIDExecutor(
	fastRouteRebuildBuildIDDependencies{},
)

var defaultFastRouteRebuildExecutor = newFastRouteRebuildExecutor(
	fastRouteRebuildDependencies{},
	fastRouteRebuildArtifactDependencies{},
)

func defaultFastRouteRebuildBuildIDDependencies() fastRouteRebuildBuildIDDependencies {
	return fastRouteRebuildBuildIDDependencies{
		generateFastRebuildIDSuffix: func() (string, error) {
			return id.New(16)
		},
	}
}

func normalizeFastRouteRebuildBuildIDDependencies(
	dependencies fastRouteRebuildBuildIDDependencies,
) fastRouteRebuildBuildIDDependencies {
	defaultDependencies := defaultFastRouteRebuildBuildIDDependencies()
	if dependencies.generateFastRebuildIDSuffix == nil {
		dependencies.generateFastRebuildIDSuffix = defaultDependencies.generateFastRebuildIDSuffix
	}
	return dependencies
}

func newFastRouteRebuildIDExecutor(
	dependencies fastRouteRebuildBuildIDDependencies,
) fastRouteRebuildIDExecutor {
	return fastRouteRebuildIDExecutor{
		dependencies: normalizeFastRouteRebuildBuildIDDependencies(dependencies),
	}
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
	return defaultFastRouteRebuildExecutor.rebuildRoutesOnly(v)
}

func rebuildRoutesOnlyWithDependencies(
	v *vormaruntime.Vorma,
	dependencies fastRouteRebuildDependencies,
	artifactDependencies fastRouteRebuildArtifactDependencies,
) error {
	return newFastRouteRebuildExecutor(dependencies, artifactDependencies).rebuildRoutesOnly(v)
}

func (executor fastRouteRebuildExecutor) rebuildRoutesOnly(v *vormaruntime.Vorma) error {
	start := time.Now()

	if !v.IsDevMode() {
		return errors.New("rebuildRoutesOnly should only be called in dev mode")
	}

	buildLifecycleStateMachine, err := newBuildLifecycleStateMachineWithOptions(
		buildLifecycleWorkflowFastRouteRebuild,
		v.Log,
		buildLifecycleStateMachineOptions{
			attemptInputs: []buildLifecycleAttemptInput{
				{
					Key:   "is_dev_mode",
					Value: "true",
				},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("configure build lifecycle state machine: %w", err)
	}
	if err := buildLifecycleStateMachine.transitionTo(buildLifecyclePhaseStarted, "fast route rebuild started"); err != nil {
		return fmt.Errorf("transition build lifecycle to started: %w", err)
	}

	v.Log.Info("START fast route rebuild")

	err = executor.dependencies.runRouteSyncExecution(
		v,
		routeSyncExecutionOptions{
			parseClientRoutes:          executor.dependencies.parseClientRoutes,
			generateBuildID:            executor.dependencies.newFastRebuildID,
			parseClientRoutesErrorText: "parse client routes",
			postSyncHook: func(v *vormaruntime.Vorma) error {
				if err := buildLifecycleStateMachine.transitionTo(
					buildLifecyclePhaseRoutesSynchronized,
					"client routes synchronized",
				); err != nil {
					return fmt.Errorf("transition build lifecycle to routes-synchronized: %w", err)
				}
				if err := executor.artifactExecutor.writeFastRebuildArtifactsAfterRouteSync(v); err != nil {
					return err
				}
				if err := buildLifecycleStateMachine.transitionTo(
					buildLifecyclePhaseRouteArtifactsWritten,
					"route artifacts written",
				); err != nil {
					return fmt.Errorf("transition build lifecycle to route-artifacts-written: %w", err)
				}
				return nil
			},
		},
	)
	if err != nil {
		if rollbackTraceErr := buildLifecycleStateMachine.recordRollback(
			buildLifecycleRollbackDecisionRequired,
			buildLifecycleRollbackOutcomeNotAttempted,
			"fast rebuild failure path requires route-manifest rollback in artifact writer",
			nil,
		); rollbackTraceErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf("record lifecycle rollback decision: %w", rollbackTraceErr),
			)
		}
		if transitionErr := buildLifecycleStateMachine.transitionToFailed("fast route rebuild failed", err); transitionErr != nil {
			return errors.Join(
				err,
				fmt.Errorf("transition build lifecycle to failed: %w", transitionErr),
			)
		}
		return err
	}
	if rollbackTraceErr := buildLifecycleStateMachine.recordRollback(
		buildLifecycleRollbackDecisionNotRequired,
		buildLifecycleRollbackOutcomeNotRequired,
		"fast route rebuild completed successfully without requiring rollback",
		nil,
	); rollbackTraceErr != nil {
		return fmt.Errorf("record lifecycle rollback decision: %w", rollbackTraceErr)
	}
	if err := buildLifecycleStateMachine.transitionTo(buildLifecyclePhaseCompleted, "fast route rebuild completed"); err != nil {
		return fmt.Errorf("transition build lifecycle to completed: %w", err)
	}

	executor.dependencies.logFastRouteRebuildCompletion(v, start)
	return nil
}

func newFastRebuildID() (string, error) {
	return defaultFastRouteRebuildIDExecutor.newFastRebuildID()
}

func (executor fastRouteRebuildIDExecutor) newFastRebuildID() (string, error) {
	return generateBuildIDWithPrefix(
		"dev_fast_",
		executor.dependencies.generateFastRebuildIDSuffix,
	)
}

func writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
	v *vormaruntime.Vorma,
	dependencies fastRouteRebuildArtifactDependencies,
) error {
	return newFastRouteRebuildArtifactExecutor(dependencies).writeFastRebuildArtifactsAfterRouteSync(v)
}

func (executor fastRouteRebuildArtifactExecutor) writeFastRebuildArtifactsAfterRouteSync(
	v *vormaruntime.Vorma,
) error {
	expectedBuildID := executor.dependencies.getCurrentBuildIDWithReadLock(v)

	var previousRouteManifestFile string
	v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
		previousRouteManifestFile = l.RouteManifestFile()
	})
	previousRouteManifestSnapshot, err := executor.captureFastRebuildRouteManifestArtifact(
		v,
		previousRouteManifestFile,
	)
	if err != nil {
		return fmt.Errorf("snapshot current route manifest artifact: %w", err)
	}

	skipRollbackForSupersededBuildID := false
	return runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				if !shouldRunFastRebuildArtifactWriteForBuildID(
					executor.dependencies.getCurrentBuildIDWithReadLock(v),
					expectedBuildID,
				) {
					skipRollbackForSupersededBuildID = true
					return nil
				}

				if err := executor.dependencies.cleanRouteManifestsOnly(v); err != nil {
					return fmt.Errorf("clean route manifests: %w", err)
				}

				if !shouldRunFastRebuildArtifactWriteForBuildID(
					executor.dependencies.getCurrentBuildIDWithReadLock(v),
					expectedBuildID,
				) {
					skipRollbackForSupersededBuildID = true
					return nil
				}

				return executor.dependencies.writeRouteArtifacts(v)
			},
			rollbackOnFailure: func() error {
				if skipRollbackForSupersededBuildID {
					return nil
				}
				if !shouldRestoreFastRebuildManifestSnapshotForBuildID(
					executor.dependencies.getCurrentBuildIDWithReadLock(v),
					expectedBuildID,
				) {
					return nil
				}
				return executor.restoreFastRebuildRouteManifestArtifact(
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

func shouldRunFastRebuildArtifactWriteForBuildID(
	currentBuildID string,
	expectedBuildID string,
) bool {
	return currentBuildID == expectedBuildID
}

func shouldRestoreFastRebuildManifestSnapshotForBuildID(
	currentBuildID string,
	expectedBuildID string,
) bool {
	return currentBuildID == expectedBuildID
}

type fastRebuildRouteManifestArtifactSnapshot = buildArtifactFileSnapshot

func captureFastRebuildRouteManifestArtifactSnapshotWithDependencies(
	v *vormaruntime.Vorma,
	manifestFile string,
	dependencies fastRouteRebuildArtifactDependencies,
) (fastRebuildRouteManifestArtifactSnapshot, error) {
	return newFastRouteRebuildArtifactExecutor(
		dependencies,
	).captureFastRebuildRouteManifestArtifact(v, manifestFile)
}

func (executor fastRouteRebuildArtifactExecutor) captureFastRebuildRouteManifestArtifact(
	v *vormaruntime.Vorma,
	manifestFile string,
) (fastRebuildRouteManifestArtifactSnapshot, error) {
	if manifestFile == "" {
		return fastRebuildRouteManifestArtifactSnapshot{}, nil
	}

	manifestFilePath := filepath.Join(v.Wave.StaticPublicOutDir(), manifestFile)
	return captureBuildArtifactFile(
		manifestFilePath,
		executor.dependencies.readRouteManifestArtifact,
	)
}

func restoreFastRebuildRouteManifestArtifactSnapshotWithDependencies(
	v *vormaruntime.Vorma,
	manifestFile string,
	snapshot fastRebuildRouteManifestArtifactSnapshot,
	dependencies fastRouteRebuildArtifactDependencies,
) error {
	return newFastRouteRebuildArtifactExecutor(
		dependencies,
	).restoreFastRebuildRouteManifestArtifact(v, manifestFile, snapshot)
}

func (executor fastRouteRebuildArtifactExecutor) restoreFastRebuildRouteManifestArtifact(
	v *vormaruntime.Vorma,
	manifestFile string,
	snapshot fastRebuildRouteManifestArtifactSnapshot,
) error {
	if manifestFile == "" {
		return nil
	}

	manifestFilePath := filepath.Join(v.Wave.StaticPublicOutDir(), manifestFile)
	return restoreBuildArtifactFile(
		manifestFilePath,
		snapshot,
		executor.dependencies.writeRouteManifestArtifact,
		executor.dependencies.removeRouteManifestArtifact,
	)
}

func logFastRouteRebuildCompletion(v *vormaruntime.Vorma, start time.Time) {
	v.Log.Info("DONE fast route rebuild",
		"buildID", v.BuildID(),
		"routes", len(v.Paths()),
		"duration", time.Since(start),
	)
}

func cleanRouteManifestsOnly(v *vormaruntime.Vorma) error {
	staticPublicOutDir := v.Wave.StaticPublicOutDir()
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

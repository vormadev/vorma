package vormabuild

import (
	"errors"
	"fmt"
	"time"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/id"
	wavebuild "github.com/vormadev/vorma/wave/tooling"
)

type buildInnerOptions struct {
	isDev bool
}

type buildInnerDependencies struct {
	captureBuildInnerRuntimeState             func(*vormaruntime.Vorma) buildInnerRuntimeStateSnapshot
	restoreBuildInnerRuntimeStateAfterFailure func(
		*vormaruntime.Vorma,
		buildInnerRuntimeStateSnapshot,
		string,
	) bool
	getCurrentBuildIDWithReadLock func(*vormaruntime.Vorma) string
	initializeBuildInnerState     func(*vormaruntime.Vorma, *buildInnerOptions) error
	parseAndSyncClientRoutes      func(*vormaruntime.Vorma) error
	cleanStaticPublicOutDir       func(*vormaruntime.Vorma) error
	writePublicFileMapTypeScript  func(*vormaruntime.Vorma) error
	writeRouteArtifacts           func(*vormaruntime.Vorma) error
	logBuildInnerCompletion       func(*vormaruntime.Vorma, time.Time)
}

type buildInnerExecutor struct {
	dependencies buildInnerDependencies
}

type buildInnerRuntimeStateSnapshot = buildRuntimeStateSnapshot

type buildInnerRouteSyncDependencies struct {
	parseClientRoutes                func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error)
	parseBackendLoaderPatterns       func(*vormaruntime.Vorma) ([]string, error)
	mergeBackendLoaderPatternsInPath func(
		map[string]*vormaruntime.Path,
		[]string,
	) map[string]*vormaruntime.Path
	runRouteSyncExecution func(*vormaruntime.Vorma, routeSyncExecutionOptions) error
}

type buildInnerRouteSyncExecutor struct {
	dependencies buildInnerRouteSyncDependencies
}

type buildInnerBuildIDDependencies struct {
	generateDevBuildIDSuffix func() (string, error)
}

type buildInnerPublicFileMapDependencies struct {
	newPublicFileMapWriter func(*vormaruntime.Vorma) buildInnerPublicFileMapWriter
}

type buildInnerBuildIDExecutor struct {
	dependencies buildInnerBuildIDDependencies
}

type buildInnerPublicFileMapExecutor struct {
	dependencies buildInnerPublicFileMapDependencies
}

type buildInnerPublicFileMapWriter interface {
	WritePublicFileMapTS(string) error
	Close() error
}

func defaultBuildInnerDependencies() buildInnerDependencies {
	return buildInnerDependencies{
		captureBuildInnerRuntimeState:             captureBuildInnerRuntimeState,
		restoreBuildInnerRuntimeStateAfterFailure: restoreBuildInnerRuntimeStateAfterFailure,
		getCurrentBuildIDWithReadLock:             currentBuildIDWithReadLock,
		initializeBuildInnerState:                 initializeBuildInnerState,
		parseAndSyncClientRoutes:                  parseAndSyncClientRoutes,
		cleanStaticPublicOutDir:                   cleanStaticPublicOutDir,
		writePublicFileMapTypeScript:              writePublicFileMapTypeScript,
		writeRouteArtifacts:                       writeRouteArtifactsWithoutHoldingRuntimeLock,
		logBuildInnerCompletion:                   logBuildInnerCompletion,
	}
}

func normalizeBuildInnerDependencies(
	dependencies buildInnerDependencies,
) buildInnerDependencies {
	defaultDependencies := defaultBuildInnerDependencies()
	if dependencies.captureBuildInnerRuntimeState == nil {
		dependencies.captureBuildInnerRuntimeState = defaultDependencies.captureBuildInnerRuntimeState
	}
	if dependencies.restoreBuildInnerRuntimeStateAfterFailure == nil {
		dependencies.restoreBuildInnerRuntimeStateAfterFailure = defaultDependencies.restoreBuildInnerRuntimeStateAfterFailure
	}
	if dependencies.getCurrentBuildIDWithReadLock == nil {
		dependencies.getCurrentBuildIDWithReadLock = defaultDependencies.getCurrentBuildIDWithReadLock
	}
	if dependencies.initializeBuildInnerState == nil {
		dependencies.initializeBuildInnerState = defaultDependencies.initializeBuildInnerState
	}
	if dependencies.parseAndSyncClientRoutes == nil {
		dependencies.parseAndSyncClientRoutes = defaultDependencies.parseAndSyncClientRoutes
	}
	if dependencies.cleanStaticPublicOutDir == nil {
		dependencies.cleanStaticPublicOutDir = defaultDependencies.cleanStaticPublicOutDir
	}
	if dependencies.writePublicFileMapTypeScript == nil {
		dependencies.writePublicFileMapTypeScript = defaultDependencies.writePublicFileMapTypeScript
	}
	if dependencies.writeRouteArtifacts == nil {
		dependencies.writeRouteArtifacts = defaultDependencies.writeRouteArtifacts
	}
	if dependencies.logBuildInnerCompletion == nil {
		dependencies.logBuildInnerCompletion = defaultDependencies.logBuildInnerCompletion
	}
	return dependencies
}

func newBuildInnerExecutor(dependencies buildInnerDependencies) buildInnerExecutor {
	return buildInnerExecutor{
		dependencies: normalizeBuildInnerDependencies(dependencies),
	}
}

func defaultBuildInnerRouteSyncDependencies() buildInnerRouteSyncDependencies {
	return buildInnerRouteSyncDependencies{
		parseClientRoutes:                parseClientRoutes,
		parseBackendLoaderPatterns:       parseBackendLoaderPatterns,
		mergeBackendLoaderPatternsInPath: mergeBackendLoaderPatternsInPath,
		runRouteSyncExecution:            runRouteSyncExecution,
	}
}

func normalizeBuildInnerRouteSyncDependencies(
	dependencies buildInnerRouteSyncDependencies,
) buildInnerRouteSyncDependencies {
	defaultDependencies := defaultBuildInnerRouteSyncDependencies()
	if dependencies.parseClientRoutes == nil {
		dependencies.parseClientRoutes = defaultDependencies.parseClientRoutes
	}
	if dependencies.parseBackendLoaderPatterns == nil {
		dependencies.parseBackendLoaderPatterns = defaultDependencies.parseBackendLoaderPatterns
	}
	if dependencies.mergeBackendLoaderPatternsInPath == nil {
		dependencies.mergeBackendLoaderPatternsInPath = defaultDependencies.mergeBackendLoaderPatternsInPath
	}
	if dependencies.runRouteSyncExecution == nil {
		dependencies.runRouteSyncExecution = defaultDependencies.runRouteSyncExecution
	}
	return dependencies
}

func newBuildInnerRouteSyncExecutor(
	dependencies buildInnerRouteSyncDependencies,
) buildInnerRouteSyncExecutor {
	return buildInnerRouteSyncExecutor{
		dependencies: normalizeBuildInnerRouteSyncDependencies(dependencies),
	}
}

func defaultBuildInnerBuildIDDependencies() buildInnerBuildIDDependencies {
	return buildInnerBuildIDDependencies{
		generateDevBuildIDSuffix: func() (string, error) {
			return id.New(16)
		},
	}
}

func normalizeBuildInnerBuildIDDependencies(
	dependencies buildInnerBuildIDDependencies,
) buildInnerBuildIDDependencies {
	defaultDependencies := defaultBuildInnerBuildIDDependencies()
	if dependencies.generateDevBuildIDSuffix == nil {
		dependencies.generateDevBuildIDSuffix = defaultDependencies.generateDevBuildIDSuffix
	}
	return dependencies
}

func newBuildInnerBuildIDExecutor(
	dependencies buildInnerBuildIDDependencies,
) buildInnerBuildIDExecutor {
	return buildInnerBuildIDExecutor{
		dependencies: normalizeBuildInnerBuildIDDependencies(dependencies),
	}
}

func defaultBuildInnerPublicFileMapDependencies() buildInnerPublicFileMapDependencies {
	return buildInnerPublicFileMapDependencies{
		newPublicFileMapWriter: func(v *vormaruntime.Vorma) buildInnerPublicFileMapWriter {
			return wavebuild.NewBuilder(
				configureBuildEnvironment(v),
				v.Wave.Logger(),
			)
		},
	}
}

func normalizeBuildInnerPublicFileMapDependencies(
	dependencies buildInnerPublicFileMapDependencies,
) buildInnerPublicFileMapDependencies {
	defaultDependencies := defaultBuildInnerPublicFileMapDependencies()
	if dependencies.newPublicFileMapWriter == nil {
		dependencies.newPublicFileMapWriter = defaultDependencies.newPublicFileMapWriter
	}
	return dependencies
}

func newBuildInnerPublicFileMapExecutor(
	dependencies buildInnerPublicFileMapDependencies,
) buildInnerPublicFileMapExecutor {
	return buildInnerPublicFileMapExecutor{
		dependencies: normalizeBuildInnerPublicFileMapDependencies(dependencies),
	}
}

var defaultBuildInnerExecutor = newBuildInnerExecutor(
	buildInnerDependencies{},
)

func buildInner(v *vormaruntime.Vorma, opts *buildInnerOptions) error {
	return defaultBuildInnerExecutor.buildInner(v, opts)
}

func buildInnerWithDependencies(
	v *vormaruntime.Vorma,
	opts *buildInnerOptions,
	dependencies buildInnerDependencies,
) error {
	return newBuildInnerExecutor(dependencies).buildInner(v, opts)
}

func (executor buildInnerExecutor) buildInner(v *vormaruntime.Vorma, opts *buildInnerOptions) error {
	normalizedOptions := normalizeBuildInnerOptions(opts)
	requestedBuildMode := "prod"
	if normalizedOptions.isDev {
		requestedBuildMode = "dev"
	}

	start := time.Now()
	buildLifecycleStateMachine, err := newBuildLifecycleStateMachineWithOptions(
		buildLifecycleWorkflowFullBuild,
		v.Log,
		buildLifecycleStateMachineOptions{
			attemptInputs: []buildLifecycleAttemptInput{
				{
					Key:   "requested_mode",
					Value: requestedBuildMode,
				},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("configure build lifecycle state machine: %w", err)
	}
	if err := buildLifecycleStateMachine.transitionTo(buildLifecyclePhaseStarted, "full build started"); err != nil {
		return fmt.Errorf("transition build lifecycle to started: %w", err)
	}

	initialRuntimeState := executor.dependencies.captureBuildInnerRuntimeState(v)
	rollbackAttempted := false
	rollbackSkippedForSupersededBuildID := false
	currentAttemptCommittedBuildID := executor.dependencies.getCurrentBuildIDWithReadLock(v)
	runBuildInnerStep := func(
		step func() error,
		stepFailureErrorContext string,
		transitionToPhase buildLifecyclePhase,
		transitionReason string,
		transitionFailureErrorContext string,
	) error {
		if err := step(); err != nil {
			if stepFailureErrorContext == "" {
				return err
			}
			return fmt.Errorf("%s: %w", stepFailureErrorContext, err)
		}
		if err := buildLifecycleStateMachine.transitionTo(transitionToPhase, transitionReason); err != nil {
			return fmt.Errorf("%s: %w", transitionFailureErrorContext, err)
		}
		return nil
	}
	buildErr := runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				if err := runBuildInnerStep(
					func() error {
						return executor.dependencies.initializeBuildInnerState(v, &normalizedOptions)
					},
					"",
					buildLifecyclePhaseRuntimeStateInitialized,
					"runtime state initialized",
					"transition build lifecycle to runtime-state-initialized",
				); err != nil {
					return err
				}
				currentAttemptCommittedBuildID = executor.dependencies.getCurrentBuildIDWithReadLock(v)
				if err := runBuildInnerStep(
					func() error {
						return executor.dependencies.parseAndSyncClientRoutes(v)
					},
					"parse client routes",
					buildLifecyclePhaseRoutesSynchronized,
					"client routes synchronized",
					"transition build lifecycle to routes-synchronized",
				); err != nil {
					return err
				}
				if err := runBuildInnerStep(
					func() error {
						return executor.dependencies.cleanStaticPublicOutDir(v)
					},
					"clean static public out dir",
					buildLifecyclePhasePublicOutputCleaned,
					"static public output cleaned",
					"transition build lifecycle to public-output-cleaned",
				); err != nil {
					return err
				}
				if err := runBuildInnerStep(
					func() error {
						return executor.dependencies.writePublicFileMapTypeScript(v)
					},
					"write public file map TS",
					buildLifecyclePhasePublicFileMapWritten,
					"public file map written",
					"transition build lifecycle to public-file-map-written",
				); err != nil {
					return err
				}
				if err := runBuildInnerStep(
					func() error {
						return executor.dependencies.writeRouteArtifacts(v)
					},
					"write route artifacts",
					buildLifecyclePhaseRouteArtifactsWritten,
					"route artifacts written",
					"transition build lifecycle to route-artifacts-written",
				); err != nil {
					return err
				}
				return nil
			},
			rollbackOnFailure: func() error {
				rollbackAttempted = executor.dependencies.restoreBuildInnerRuntimeStateAfterFailure(
					v,
					initialRuntimeState,
					currentAttemptCommittedBuildID,
				)
				rollbackSkippedForSupersededBuildID = !rollbackAttempted
				return nil
			},
		},
	)
	if buildErr != nil {
		rollbackOutcome := buildLifecycleRollbackOutcomeNotAttempted
		rollbackReason := "runtime-state rollback was not attempted after full build failure"
		if rollbackAttempted {
			rollbackOutcome = buildLifecycleRollbackOutcomeSucceeded
			rollbackReason = "restored captured runtime state after full build failure"
		} else if rollbackSkippedForSupersededBuildID {
			rollbackReason = "skipped runtime-state rollback because build ID was superseded by a newer build"
		}
		if rollbackTraceErr := buildLifecycleStateMachine.recordRollback(
			buildLifecycleRollbackDecisionRequired,
			rollbackOutcome,
			rollbackReason,
			nil,
		); rollbackTraceErr != nil {
			buildErr = errors.Join(
				buildErr,
				fmt.Errorf("record lifecycle rollback decision: %w", rollbackTraceErr),
			)
		}
		if transitionErr := buildLifecycleStateMachine.transitionToFailed("full build failed", buildErr); transitionErr != nil {
			return errors.Join(
				buildErr,
				fmt.Errorf("transition build lifecycle to failed: %w", transitionErr),
			)
		}
		return buildErr
	}
	if rollbackTraceErr := buildLifecycleStateMachine.recordRollback(
		buildLifecycleRollbackDecisionNotRequired,
		buildLifecycleRollbackOutcomeNotRequired,
		"build completed successfully without requiring rollback",
		nil,
	); rollbackTraceErr != nil {
		return fmt.Errorf("record lifecycle rollback decision: %w", rollbackTraceErr)
	}
	if err := buildLifecycleStateMachine.transitionTo(buildLifecyclePhaseCompleted, "full build completed"); err != nil {
		return fmt.Errorf("transition build lifecycle to completed: %w", err)
	}

	executor.dependencies.logBuildInnerCompletion(v, start)
	return nil
}

func normalizeBuildInnerOptions(opts *buildInnerOptions) buildInnerOptions {
	if opts == nil {
		return buildInnerOptions{}
	}
	return *opts
}

func captureBuildInnerRuntimeState(v *vormaruntime.Vorma) buildInnerRuntimeStateSnapshot {
	var runtimeStateSnapshot buildInnerRuntimeStateSnapshot
	v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
		runtimeStateSnapshot = captureBuildRuntimeStateSnapshot(l)
	})
	return runtimeStateSnapshot
}

func restoreBuildInnerRuntimeStateAfterFailure(
	v *vormaruntime.Vorma,
	state buildInnerRuntimeStateSnapshot,
	currentAttemptCommittedBuildID string,
) bool {
	restored := false
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		if !shouldRollbackBuildInnerRuntimeStateAfterFailure(
			l.GetBuildID(),
			currentAttemptCommittedBuildID,
		) {
			return
		}
		restoreBuildRuntimeStateSnapshot(l, state)
		restored = true
	})
	return restored
}

func shouldRollbackBuildInnerRuntimeStateAfterFailure(
	currentBuildID string,
	currentAttemptCommittedBuildID string,
) bool {
	return shouldRestoreRuntimeStateSnapshotForAttemptBuildID(
		currentBuildID,
		currentAttemptCommittedBuildID,
	)
}

func initializeBuildInnerState(v *vormaruntime.Vorma, opts *buildInnerOptions) error {
	return initializeBuildInnerStateWithBuildIDDependencies(
		v,
		opts,
		buildInnerBuildIDDependencies{},
	)
}

func initializeBuildInnerStateWithBuildIDDependencies(
	v *vormaruntime.Vorma,
	opts *buildInnerOptions,
	dependencies buildInnerBuildIDDependencies,
) error {
	if !opts.isDev {
		commitRuntimeState(
			v,
			runtimeStateCommitInput{
				shouldCommitIsDev: true,
				isDev:             false,
			},
		)
		v.Log.Info("START building Vorma (PROD)")
		return nil
	}

	buildID, err := newDevBuildIDWithDependencies(dependencies)
	if err != nil {
		return err
	}

	commitRuntimeState(
		v,
		runtimeStateCommitInput{
			shouldCommitIsDev:   true,
			isDev:               true,
			shouldCommitBuildID: true,
			buildID:             buildID,
		},
	)
	v.Log.Info("START building Vorma (DEV)")
	return nil
}

func newDevBuildIDWithDependencies(
	dependencies buildInnerBuildIDDependencies,
) (string, error) {
	return newBuildInnerBuildIDExecutor(dependencies).newDevBuildID()
}

func (executor buildInnerBuildIDExecutor) newDevBuildID() (string, error) {
	return generateBuildIDWithPrefix(
		"dev_",
		executor.dependencies.generateDevBuildIDSuffix,
	)
}

func parseAndSyncClientRoutes(v *vormaruntime.Vorma) error {
	return parseAndSyncClientRoutesWithDependencies(v, buildInnerRouteSyncDependencies{})
}

func parseAndSyncClientRoutesWithDependencies(
	v *vormaruntime.Vorma,
	dependencies buildInnerRouteSyncDependencies,
) error {
	return newBuildInnerRouteSyncExecutor(dependencies).parseAndSyncClientRoutes(v)
}

func (executor buildInnerRouteSyncExecutor) parseAndSyncClientRoutes(v *vormaruntime.Vorma) error {
	return executor.dependencies.runRouteSyncExecution(
		v,
		routeSyncExecutionOptions{
			parseClientRoutes: executor.parseClientAndBackendRoutesForSync,
		},
	)
}

func parseClientAndBackendRoutesForSyncWithDependencies(
	v *vormaruntime.Vorma,
	dependencies buildInnerRouteSyncDependencies,
) (map[string]*vormaruntime.Path, error) {
	return newBuildInnerRouteSyncExecutor(dependencies).parseClientAndBackendRoutesForSync(v)
}

func (executor buildInnerRouteSyncExecutor) parseClientAndBackendRoutesForSync(
	v *vormaruntime.Vorma,
) (map[string]*vormaruntime.Path, error) {
	clientPaths, err := executor.dependencies.parseClientRoutes(v)
	if err != nil {
		return nil, err
	}

	backendLoaderPatterns, err := executor.dependencies.parseBackendLoaderPatterns(v)
	if err != nil {
		return nil, err
	}

	return executor.dependencies.mergeBackendLoaderPatternsInPath(
		clientPaths,
		backendLoaderPatterns,
	), nil
}

func mergeBackendLoaderPatternsInPath(
	clientPaths map[string]*vormaruntime.Path,
	backendLoaderPatterns []string,
) map[string]*vormaruntime.Path {
	if len(backendLoaderPatterns) == 0 {
		return clientPaths
	}

	if clientPaths == nil {
		clientPaths = map[string]*vormaruntime.Path{}
	}
	for _, backendLoaderPattern := range backendLoaderPatterns {
		if _, hasClientPath := clientPaths[backendLoaderPattern]; hasClientPath {
			continue
		}
		clientPaths[backendLoaderPattern] = &vormaruntime.Path{
			OriginalPattern: backendLoaderPattern,
			SrcPath:         "",
			ExportKey:       "default",
		}
	}
	return clientPaths
}

func writePublicFileMapTypeScript(v *vormaruntime.Vorma) error {
	return writePublicFileMapTypeScriptWithDependencies(v, buildInnerPublicFileMapDependencies{})
}

func writePublicFileMapTypeScriptWithDependencies(
	v *vormaruntime.Vorma,
	dependencies buildInnerPublicFileMapDependencies,
) error {
	return newBuildInnerPublicFileMapExecutor(dependencies).writePublicFileMapTypeScript(v)
}

func (executor buildInnerPublicFileMapExecutor) writePublicFileMapTypeScript(
	v *vormaruntime.Vorma,
) error {
	return executor.runWithPublicFileMapWriter(v, func(writer buildInnerPublicFileMapWriter) error {
		return writer.WritePublicFileMapTS(v.Config.TSGenOutDir)
	})
}

func (executor buildInnerPublicFileMapExecutor) runWithPublicFileMapWriter(
	v *vormaruntime.Vorma,
	runWithWriter func(buildInnerPublicFileMapWriter) error,
) (operationErr error) {
	writer := executor.dependencies.newPublicFileMapWriter(v)
	return runWithClosableResource(
		writer,
		"close wave builder",
		runWithWriter,
	)
}

func logBuildInnerCompletion(v *vormaruntime.Vorma, start time.Time) {
	v.Log.Info("DONE building Vorma",
		"buildID", v.GetBuildID(),
		"routes found", len(v.GetPathsSnapshot()),
		"duration", time.Since(start),
	)
}

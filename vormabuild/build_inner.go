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
	captureBuildInnerRuntimeState func(*vormaruntime.Vorma) buildInnerRuntimeStateSnapshot
	restoreBuildInnerRuntimeState func(*vormaruntime.Vorma, buildInnerRuntimeStateSnapshot)
	initializeBuildInnerState     func(*vormaruntime.Vorma, *buildInnerOptions) error
	parseAndSyncClientRoutes      func(*vormaruntime.Vorma) error
	cleanStaticPublicOutDir       func(*vormaruntime.Vorma) error
	writePublicFileMapTypeScript  func(*vormaruntime.Vorma) error
	writeRouteArtifactsWithLock   func(*vormaruntime.Vorma) error
	logBuildInnerCompletion       func(*vormaruntime.Vorma, time.Time)
}

type buildInnerRuntimeStateSnapshot struct {
	isDev                  bool
	routeBuildRuntimeState routeBuildRuntimeStateSnapshot
}

type buildInnerRouteSyncDependencies struct {
	parseClientRoutes                func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error)
	parseBackendLoaderPatterns       func(*vormaruntime.Vorma) ([]string, error)
	mergeBackendLoaderPatternsInPath func(
		map[string]*vormaruntime.Path,
		[]string,
	) map[string]*vormaruntime.Path
	runRouteSyncExecution func(*vormaruntime.Vorma, routeSyncExecutionOptions) error
}

type buildInnerBuildIDDependencies struct {
	generateDevBuildIDSuffix func() (string, error)
}

type buildInnerPublicFileMapDependencies struct {
	newPublicFileMapWriter func(*vormaruntime.Vorma) buildInnerPublicFileMapWriter
}

type buildInnerPublicFileMapWriter interface {
	WritePublicFileMapTS(string) error
	Close() error
}

var buildInnerDeps = buildInnerDependencies{
	captureBuildInnerRuntimeState: captureBuildInnerRuntimeState,
	restoreBuildInnerRuntimeState: restoreBuildInnerRuntimeState,
	initializeBuildInnerState:     initializeBuildInnerState,
	parseAndSyncClientRoutes:      parseAndSyncClientRoutes,
	cleanStaticPublicOutDir:       cleanStaticPublicOutDir,
	writePublicFileMapTypeScript:  writePublicFileMapTypeScript,
	writeRouteArtifactsWithLock:   writeRouteArtifactsWithLock,
	logBuildInnerCompletion:       logBuildInnerCompletion,
}

var buildInnerRouteSyncDeps = buildInnerRouteSyncDependencies{
	parseClientRoutes:                parseClientRoutes,
	parseBackendLoaderPatterns:       parseBackendLoaderPatterns,
	mergeBackendLoaderPatternsInPath: mergeBackendLoaderPatternsInPath,
	runRouteSyncExecution:            runRouteSyncExecution,
}

var buildInnerBuildIDDeps = buildInnerBuildIDDependencies{
	generateDevBuildIDSuffix: func() (string, error) {
		return id.New(16)
	},
}

var buildInnerPublicFileMapDeps = buildInnerPublicFileMapDependencies{
	newPublicFileMapWriter: func(v *vormaruntime.Vorma) buildInnerPublicFileMapWriter {
		return wavebuild.NewBuilder(
			configureBuildEnvironment(v),
			v.Wave.Logger(),
		)
	},
}

func buildInner(v *vormaruntime.Vorma, opts *buildInnerOptions) error {
	normalizedOptions := normalizeBuildInnerOptions(opts)

	start := time.Now()
	buildLifecycleStateMachine, err := newBuildLifecycleStateMachine(
		buildLifecycleWorkflowFullBuild,
		v.Log,
		nil,
	)
	if err != nil {
		return fmt.Errorf("configure build lifecycle state machine: %w", err)
	}
	if err := buildLifecycleStateMachine.transitionTo(buildLifecyclePhaseStarted, "full build started"); err != nil {
		return fmt.Errorf("transition build lifecycle to started: %w", err)
	}

	initialRuntimeState := buildInnerDeps.captureBuildInnerRuntimeState(v)
	buildErr := runWithRollbackOnFailureAndPanic(
		rollbackTransactionOptions{
			run: func() error {
				if err := buildInnerDeps.initializeBuildInnerState(v, &normalizedOptions); err != nil {
					return err
				}
				if err := buildLifecycleStateMachine.transitionTo(
					buildLifecyclePhaseRuntimeStateInitialized,
					"runtime state initialized",
				); err != nil {
					return fmt.Errorf("transition build lifecycle to runtime-state-initialized: %w", err)
				}

				if err := buildInnerDeps.parseAndSyncClientRoutes(v); err != nil {
					return fmt.Errorf("parse client routes: %w", err)
				}
				if err := buildLifecycleStateMachine.transitionTo(
					buildLifecyclePhaseRoutesSynchronized,
					"client routes synchronized",
				); err != nil {
					return fmt.Errorf("transition build lifecycle to routes-synchronized: %w", err)
				}

				if err := buildInnerDeps.cleanStaticPublicOutDir(v); err != nil {
					return fmt.Errorf("clean static public out dir: %w", err)
				}
				if err := buildLifecycleStateMachine.transitionTo(
					buildLifecyclePhasePublicOutputCleaned,
					"static public output cleaned",
				); err != nil {
					return fmt.Errorf("transition build lifecycle to public-output-cleaned: %w", err)
				}

				if err := buildInnerDeps.writePublicFileMapTypeScript(v); err != nil {
					return fmt.Errorf("write public file map TS: %w", err)
				}
				if err := buildLifecycleStateMachine.transitionTo(
					buildLifecyclePhasePublicFileMapWritten,
					"public file map written",
				); err != nil {
					return fmt.Errorf("transition build lifecycle to public-file-map-written: %w", err)
				}

				if err := buildInnerDeps.writeRouteArtifactsWithLock(v); err != nil {
					return fmt.Errorf("write route artifacts: %w", err)
				}
				if err := buildLifecycleStateMachine.transitionTo(
					buildLifecyclePhaseRouteArtifactsWritten,
					"route artifacts written",
				); err != nil {
					return fmt.Errorf("transition build lifecycle to route-artifacts-written: %w", err)
				}
				return nil
			},
			rollbackOnFailure: func() error {
				buildInnerDeps.restoreBuildInnerRuntimeState(v, initialRuntimeState)
				return nil
			},
		},
	)
	if buildErr != nil {
		if transitionErr := buildLifecycleStateMachine.transitionToFailed("full build failed", buildErr); transitionErr != nil {
			return errors.Join(
				buildErr,
				fmt.Errorf("transition build lifecycle to failed: %w", transitionErr),
			)
		}
		return buildErr
	}
	if err := buildLifecycleStateMachine.transitionTo(buildLifecyclePhaseCompleted, "full build completed"); err != nil {
		return fmt.Errorf("transition build lifecycle to completed: %w", err)
	}

	buildInnerDeps.logBuildInnerCompletion(v, start)
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
		runtimeStateSnapshot = buildInnerRuntimeStateSnapshot{
			isDev:                  l.GetIsDev(),
			routeBuildRuntimeState: captureRouteBuildRuntimeStateSnapshot(l),
		}
	})
	return runtimeStateSnapshot
}

func restoreBuildInnerRuntimeState(
	v *vormaruntime.Vorma,
	state buildInnerRuntimeStateSnapshot,
) {
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetIsDev(state.isDev)
		restoreRouteBuildRuntimeStateSnapshot(l, state.routeBuildRuntimeState)
	})
}

func initializeBuildInnerState(v *vormaruntime.Vorma, opts *buildInnerOptions) error {
	if !opts.isDev {
		v.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetIsDev(false)
		})
		v.Log.Info("START building Vorma (PROD)")
		return nil
	}

	buildID, err := newDevBuildID()
	if err != nil {
		return err
	}

	v.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetIsDev(true)
		l.SetBuildID(buildID)
	})
	v.Log.Info("START building Vorma (DEV)")
	return nil
}

func newDevBuildID() (string, error) {
	return generateBuildIDWithPrefix(
		"dev_",
		buildInnerBuildIDDeps.generateDevBuildIDSuffix,
	)
}

func parseAndSyncClientRoutes(v *vormaruntime.Vorma) error {
	return buildInnerRouteSyncDeps.runRouteSyncExecution(
		v,
		routeSyncExecutionOptions{
			parseClientRoutes: parseClientAndBackendRoutesForSync,
		},
	)
}

func parseClientAndBackendRoutesForSync(
	v *vormaruntime.Vorma,
) (map[string]*vormaruntime.Path, error) {
	clientPaths, err := buildInnerRouteSyncDeps.parseClientRoutes(v)
	if err != nil {
		return nil, err
	}

	backendLoaderPatterns, err := buildInnerRouteSyncDeps.parseBackendLoaderPatterns(v)
	if err != nil {
		return nil, err
	}

	return buildInnerRouteSyncDeps.mergeBackendLoaderPatternsInPath(
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
	return runWithPublicFileMapWriter(v, func(writer buildInnerPublicFileMapWriter) error {
		return writer.WritePublicFileMapTS(v.Config.TSGenOutDir)
	})
}

func runWithPublicFileMapWriter(
	v *vormaruntime.Vorma,
	runWithWriter func(buildInnerPublicFileMapWriter) error,
) (operationErr error) {
	writer := buildInnerPublicFileMapDeps.newPublicFileMapWriter(v)
	return runWithClosableResource(
		writer,
		"close wave builder",
		runWithWriter,
	)
}

func writeRouteArtifactsWithLock(v *vormaruntime.Vorma) error {
	var writeRouteArtifactsErr error
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		writeRouteArtifactsErr = writeRouteArtifacts(l)
	})
	return writeRouteArtifactsErr
}

func logBuildInnerCompletion(v *vormaruntime.Vorma, start time.Time) {
	v.Log.Info("DONE building Vorma",
		"buildID", v.GetBuildID(),
		"routes found", len(v.GetPathsSnapshot()),
		"duration", time.Since(start),
	)
}

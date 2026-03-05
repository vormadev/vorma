// Package buildinner owns the hook-time build pipeline that parses routes,
// refreshes artifacts, and commits runtime state for a single build attempt.
//
// Keeping this pipeline isolated from CLI/runtime entrypoint code keeps tests
// focused on build-step semantics (state initialization, rollback policy, and
// artifact sequencing) without coupling to command parsing concerns.
package buildinner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vormadev/vorma/internal/artifactio"
	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/kit/id"
	"github.com/vormadev/vorma/vormabuild/internal/backendroutes"
	"github.com/vormadev/vorma/vormabuild/internal/buildenv"
	"github.com/vormadev/vorma/vormabuild/internal/buildlifecycle"
	"github.com/vormadev/vorma/vormabuild/internal/routeartifacts"
	"github.com/vormadev/vorma/vormabuild/internal/routeparse"
	"github.com/vormadev/vorma/wave/buildtime/builder"
	"github.com/vormadev/vorma/wave/waveartifacts"
)

// RunOptions configures how a single build-inner run should execute.
type RunOptions struct {
	IsDev bool
}

type buildInnerOptions struct {
	isDev bool
}

type buildInnerDependencies struct {
	captureBuildInnerRuntimeState             func(*vormaruntime.Vorma) buildlifecycle.BuildRuntimeStateSnapshot
	restoreBuildInnerRuntimeStateAfterFailure func(
		*vormaruntime.Vorma,
		buildlifecycle.BuildRuntimeStateSnapshot,
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

type buildInnerRouteSyncDependencies struct {
	parseClientRoutes                func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error)
	parseBackendLoaderPatterns       func(*vormaruntime.Vorma) ([]string, error)
	mergeBackendLoaderPatternsInPath func(
		map[string]*vormaruntime.Path,
		[]string,
	) map[string]*vormaruntime.Path
	runRouteSyncExecution func(*vormaruntime.Vorma, buildlifecycle.RouteSyncExecutionOptions) error
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

type canonicalPublicFileMapValue struct {
	Dist string `json:"dist"`
}

type buildInnerPublicFileMapWriter interface {
	ProcessPublicFilesOnly() error
	WritePublicFileMapTS(string) error
	Close() error
}

type waveBuilderBackedPublicFileMapWriter struct {
	waveBuilder *builder.Builder
	vorma       *vormaruntime.Vorma
}

func defaultBuildInnerDependencies() buildInnerDependencies {
	return buildInnerDependencies{
		captureBuildInnerRuntimeState:             captureBuildInnerRuntimeState,
		restoreBuildInnerRuntimeStateAfterFailure: restoreBuildInnerRuntimeStateAfterFailure,
		getCurrentBuildIDWithReadLock:             buildlifecycle.CurrentBuildIDWithReadLock,
		initializeBuildInnerState:                 initializeBuildInnerState,
		parseAndSyncClientRoutes:                  parseAndSyncClientRoutes,
		cleanStaticPublicOutDir: func(*vormaruntime.Vorma) error {
			return nil
		},
		writePublicFileMapTypeScript: writePublicFileMapTypeScript,
		writeRouteArtifacts:          routeartifacts.WriteRouteArtifactsWithoutHoldingRuntimeLock,
		logBuildInnerCompletion:      logBuildInnerCompletion,
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

func newBuildInnerExecutor(
	dependencies buildInnerDependencies,
) buildInnerExecutor {
	return buildInnerExecutor{
		dependencies: normalizeBuildInnerDependencies(dependencies),
	}
}

func defaultBuildInnerRouteSyncDependencies() buildInnerRouteSyncDependencies {
	return buildInnerRouteSyncDependencies{
		parseClientRoutes:                routeparse.ParseClientRoutes,
		parseBackendLoaderPatterns:       backendroutes.ParseBackendLoaderPatterns,
		mergeBackendLoaderPatternsInPath: mergeBackendLoaderPatternsInPath,
		runRouteSyncExecution:            buildlifecycle.RunRouteSyncExecution,
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
			return &waveBuilderBackedPublicFileMapWriter{
				waveBuilder: builder.NewBuilder(
					buildenv.Configure(v),
					v.Wave.Logger(),
				),
				vorma: v,
			}
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
		dependencies: normalizeBuildInnerPublicFileMapDependencies(
			dependencies,
		),
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

func (executor buildInnerExecutor) buildInner(
	v *vormaruntime.Vorma,
	opts *buildInnerOptions,
) error {
	normalizedOptions := normalizeBuildInnerOptions(opts)
	requestedBuildMode := "prod"
	if normalizedOptions.isDev {
		requestedBuildMode = "dev"
	}

	start := time.Now()
	buildLifecycleStateMachine, err := buildlifecycle.NewLifecycleStateMachineWithOptions(
		buildlifecycle.WorkflowFullBuild,
		v.Log,
		buildlifecycle.LifecycleStateMachineOptions{
			AttemptInputs: []buildlifecycle.LifecycleAttemptInput{
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
	if err := buildLifecycleStateMachine.TransitionTo(buildlifecycle.PhaseStarted, "full build started"); err != nil {
		return fmt.Errorf("transition build lifecycle to started: %w", err)
	}

	initialRuntimeState := executor.dependencies.captureBuildInnerRuntimeState(
		v,
	)
	rollbackAttempted := false
	rollbackSkippedForSupersededBuildID := false
	currentAttemptCommittedBuildID := executor.dependencies.getCurrentBuildIDWithReadLock(
		v,
	)
	runBuildInnerStep := func(
		step func() error,
		stepFailureErrorContext string,
		transitionToPhase buildlifecycle.LifecyclePhase,
		transitionReason string,
		transitionFailureErrorContext string,
	) error {
		if err := step(); err != nil {
			if stepFailureErrorContext == "" {
				return err
			}
			return fmt.Errorf("%s: %w", stepFailureErrorContext, err)
		}
		if err := buildLifecycleStateMachine.TransitionTo(transitionToPhase, transitionReason); err != nil {
			return fmt.Errorf("%s: %w", transitionFailureErrorContext, err)
		}
		return nil
	}
	buildErr := buildlifecycle.RunWithRollbackOnFailureAndPanic(
		buildlifecycle.RollbackTransactionOptions{
			Run: func() error {
				if err := runBuildInnerStep(
					func() error {
						return executor.dependencies.initializeBuildInnerState(v, &normalizedOptions)
					},
					"",
					buildlifecycle.PhaseRuntimeStateInitialized,
					"runtime state initialized",
					"transition build lifecycle to runtime-state-initialized",
				); err != nil {
					return err
				}
				currentAttemptCommittedBuildID = executor.dependencies.getCurrentBuildIDWithReadLock(
					v,
				)
				if err := runBuildInnerStep(
					func() error {
						return executor.dependencies.parseAndSyncClientRoutes(v)
					},
					"parse client routes",
					buildlifecycle.PhaseRoutesSynchronized,
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
					buildlifecycle.PhasePublicOutputCleaned,
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
					buildlifecycle.PhasePublicFileMapWritten,
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
					buildlifecycle.PhaseRouteArtifactsWritten,
					"route artifacts written",
					"transition build lifecycle to route-artifacts-written",
				); err != nil {
					return err
				}
				return nil
			},
			RollbackOnFailure: func() error {
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
		rollbackOutcome := buildlifecycle.OutcomeNotAttempted
		rollbackReason := "runtime-state rollback was not attempted after full build failure"
		if rollbackAttempted {
			rollbackOutcome = buildlifecycle.OutcomeSucceeded
			rollbackReason = "restored captured runtime state after full build failure"
		} else if rollbackSkippedForSupersededBuildID {
			rollbackReason = "skipped runtime-state rollback because build ID was superseded by a newer build"
		}
		if rollbackTraceErr := buildLifecycleStateMachine.RecordRollback(
			buildlifecycle.DecisionRequired,
			rollbackOutcome,
			rollbackReason,
			nil,
		); rollbackTraceErr != nil {
			buildErr = errors.Join(
				buildErr,
				fmt.Errorf(
					"record lifecycle rollback decision: %w",
					rollbackTraceErr,
				),
			)
		}
		if transitionErr := buildLifecycleStateMachine.TransitionToFailed("full build failed", buildErr); transitionErr != nil {
			return errors.Join(
				buildErr,
				fmt.Errorf(
					"transition build lifecycle to failed: %w",
					transitionErr,
				),
			)
		}
		return buildErr
	}
	if rollbackTraceErr := buildLifecycleStateMachine.RecordRollback(
		buildlifecycle.DecisionNotRequired,
		buildlifecycle.OutcomeNotRequired,
		"build completed successfully without requiring rollback",
		nil,
	); rollbackTraceErr != nil {
		return fmt.Errorf(
			"record lifecycle rollback decision: %w",
			rollbackTraceErr,
		)
	}
	if err := buildLifecycleStateMachine.TransitionTo(buildlifecycle.PhaseCompleted, "full build completed"); err != nil {
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

func captureBuildInnerRuntimeState(
	v *vormaruntime.Vorma,
) buildlifecycle.BuildRuntimeStateSnapshot {
	var runtimeStateSnapshot buildlifecycle.BuildRuntimeStateSnapshot
	v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
		runtimeStateSnapshot = buildlifecycle.CaptureBuildRuntimeState(l)
	})
	return runtimeStateSnapshot
}

func restoreBuildInnerRuntimeStateAfterFailure(
	v *vormaruntime.Vorma,
	state buildlifecycle.BuildRuntimeStateSnapshot,
	currentAttemptCommittedBuildID string,
) bool {
	restored := false
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		if !shouldRollbackBuildInnerRuntimeStateAfterFailure(
			l.BuildID(),
			currentAttemptCommittedBuildID,
		) {
			return
		}
		buildlifecycle.RestoreBuildRuntimeState(l, state)
		restored = true
	})
	return restored
}

func shouldRollbackBuildInnerRuntimeStateAfterFailure(
	currentBuildID string,
	currentAttemptCommittedBuildID string,
) bool {
	return buildlifecycle.ShouldRestoreRuntimeStateSnapshotForAttemptBuildID(
		currentBuildID,
		currentAttemptCommittedBuildID,
	)
}

func initializeBuildInnerState(
	v *vormaruntime.Vorma,
	opts *buildInnerOptions,
) error {
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
		buildlifecycle.CommitRuntimeState(
			v,
			buildlifecycle.RuntimeStateCommitInput{
				ShouldCommitIsDev: true,
				IsDev:             false,
			},
		)
		v.Log.Info("START building Vorma (PROD)")
		return nil
	}

	buildID, err := newDevBuildIDWithDependencies(dependencies)
	if err != nil {
		return err
	}

	buildlifecycle.CommitRuntimeState(
		v,
		buildlifecycle.RuntimeStateCommitInput{
			ShouldCommitIsDev:   true,
			IsDev:               true,
			ShouldCommitBuildID: true,
			BuildID:             buildID,
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
	return parseAndSyncClientRoutesWithDependencies(
		v,
		buildInnerRouteSyncDependencies{},
	)
}

func parseAndSyncClientRoutesWithDependencies(
	v *vormaruntime.Vorma,
	dependencies buildInnerRouteSyncDependencies,
) error {
	return newBuildInnerRouteSyncExecutor(
		dependencies,
	).parseAndSyncClientRoutes(v)
}

func (executor buildInnerRouteSyncExecutor) parseAndSyncClientRoutes(
	v *vormaruntime.Vorma,
) error {
	return executor.dependencies.runRouteSyncExecution(
		v,
		buildlifecycle.RouteSyncExecutionOptions{
			ParseClientRoutes: executor.parseClientAndBackendRoutesForSync,
		},
	)
}

func parseClientAndBackendRoutesForSyncWithDependencies(
	v *vormaruntime.Vorma,
	dependencies buildInnerRouteSyncDependencies,
) (map[string]*vormaruntime.Path, error) {
	return newBuildInnerRouteSyncExecutor(
		dependencies,
	).parseClientAndBackendRoutesForSync(v)
}

func (executor buildInnerRouteSyncExecutor) parseClientAndBackendRoutesForSync(
	v *vormaruntime.Vorma,
) (map[string]*vormaruntime.Path, error) {
	clientPaths, err := executor.dependencies.parseClientRoutes(v)
	if err != nil {
		return nil, err
	}

	backendLoaderPatterns, err := executor.dependencies.parseBackendLoaderPatterns(
		v,
	)
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
	return writePublicFileMapTypeScriptWithDependencies(
		v,
		buildInnerPublicFileMapDependencies{},
	)
}

func writePublicFileMapTypeScriptWithDependencies(
	v *vormaruntime.Vorma,
	dependencies buildInnerPublicFileMapDependencies,
) error {
	return newBuildInnerPublicFileMapExecutor(
		dependencies,
	).writePublicFileMapTypeScript(v)
}

func (executor buildInnerPublicFileMapExecutor) writePublicFileMapTypeScript(
	v *vormaruntime.Vorma,
) error {
	return executor.runWithPublicFileMapWriter(
		v,
		func(writer buildInnerPublicFileMapWriter) error {
			if processPublicFilesError := writer.ProcessPublicFilesOnly(); processPublicFilesError != nil {
				return processPublicFilesError
			}
			return writer.WritePublicFileMapTS(v.Config.TSGenOutDir())
		},
	)
}

func (writer *waveBuilderBackedPublicFileMapWriter) ProcessPublicFilesOnly() error {
	if writer == nil || writer.waveBuilder == nil {
		return errors.New("wave builder is unavailable")
	}
	return writer.waveBuilder.ProcessPublicFilesOnly()
}

func (writer *waveBuilderBackedPublicFileMapWriter) WritePublicFileMapTS(
	outputDirectoryPath string,
) error {
	if writer == nil || writer.vorma == nil {
		return errors.New("vorma runtime is unavailable")
	}
	return writePublicFileMapTSFromCanonicalWaveOutput(
		writer.vorma,
		outputDirectoryPath,
	)
}

func (writer *waveBuilderBackedPublicFileMapWriter) Close() error {
	if writer == nil || writer.waveBuilder == nil {
		return nil
	}
	return writer.waveBuilder.Close()
}

func writePublicFileMapTSFromCanonicalWaveOutput(
	v *vormaruntime.Vorma,
	outputDirectoryPath string,
) error {
	canonicalPublicFileMap, readError := readCanonicalWavePublicFileMap(v)
	if readError != nil {
		return readError
	}

	if mkdirError := os.MkdirAll(outputDirectoryPath, 0o755); mkdirError != nil {
		return fmt.Errorf("create public filemap output directory: %w", mkdirError)
	}

	typeScriptContent := renderPublicFileMapTypeScript(canonicalPublicFileMap)
	typeScriptPath := filepath.Join(
		outputDirectoryPath,
		runtimepaths.GeneratedTypeScriptPublicFileMapFileName,
	)
	if writeError := artifactio.WriteFileAtomically(
		typeScriptPath,
		[]byte(typeScriptContent),
		0o644,
	); writeError != nil {
		return fmt.Errorf("write public filemap TypeScript output: %w", writeError)
	}

	legacyJSONPath := filepath.Join(
		outputDirectoryPath,
		waveartifacts.PublicFileMapJSONName,
	)
	if removeError := os.Remove(legacyJSONPath); removeError != nil &&
		!errors.Is(removeError, os.ErrNotExist) {
		return fmt.Errorf("remove legacy public filemap JSON output: %w", removeError)
	}

	return nil
}

func readCanonicalWavePublicFileMap(
	v *vormaruntime.Vorma,
) (map[string]string, error) {
	if v == nil {
		return nil, errors.New("vorma runtime is nil")
	}

	parsedConfig := v.Wave.ParsedConfig()
	if parsedConfig == nil {
		return nil, errors.New("wave build config is nil")
	}

	refFileBytes, readRefError := os.ReadFile(parsedConfig.Dist().PublicFileMapRef())
	if readRefError != nil {
		return nil, fmt.Errorf("read canonical public filemap ref: %w", readRefError)
	}

	referencedFileName := strings.TrimSpace(string(refFileBytes))
	if referencedFileName == "" {
		return nil, errors.New("canonical public filemap ref is empty")
	}
	referencedFileName = filepath.ToSlash(filepath.Clean(referencedFileName))
	if referencedFileName == "." ||
		referencedFileName == ".." ||
		strings.HasPrefix(referencedFileName, "../") {
		return nil, fmt.Errorf(
			"canonical public filemap ref escapes static public root: %q",
			referencedFileName,
		)
	}

	canonicalJSONPath := filepath.Join(
		parsedConfig.Dist().StaticPublic(),
		filepath.FromSlash(referencedFileName),
	)
	relativePathFromPublicRoot, relativePathError := filepath.Rel(
		parsedConfig.Dist().StaticPublic(),
		canonicalJSONPath,
	)
	if relativePathError != nil {
		return nil, fmt.Errorf(
			"resolve canonical public filemap path: %w",
			relativePathError,
		)
	}
	normalizedRelativePathFromPublicRoot := filepath.ToSlash(
		relativePathFromPublicRoot,
	)
	if normalizedRelativePathFromPublicRoot == ".." ||
		strings.HasPrefix(normalizedRelativePathFromPublicRoot, "../") {
		return nil, fmt.Errorf(
			"canonical public filemap path escapes static public root: %q",
			referencedFileName,
		)
	}

	canonicalJSONBytes, readJSONError := os.ReadFile(canonicalJSONPath)
	if readJSONError != nil {
		return nil, fmt.Errorf(
			"read canonical public filemap JSON %q: %w",
			canonicalJSONPath,
			readJSONError,
		)
	}

	var canonicalRawMap map[string]canonicalPublicFileMapValue
	if unmarshalError := json.Unmarshal(canonicalJSONBytes, &canonicalRawMap); unmarshalError != nil {
		return nil, fmt.Errorf(
			"parse canonical public filemap JSON %q: %w",
			canonicalJSONPath,
			unmarshalError,
		)
	}

	publicFileMap := make(map[string]string, len(canonicalRawMap))
	for sourcePath, rawValue := range canonicalRawMap {
		trimmedDistName := strings.TrimSpace(rawValue.Dist)
		if trimmedDistName == "" {
			return nil, fmt.Errorf(
				"canonical public filemap entry %q is missing dist output",
				sourcePath,
			)
		}
		publicFileMap[sourcePath] = trimmedDistName
	}
	return publicFileMap, nil
}

func renderPublicFileMapTypeScript(publicFileMap map[string]string) string {
	sortedKeys := make([]string, 0, len(publicFileMap))
	for key := range publicFileMap {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	typeScriptBuilder := &strings.Builder{}
	typeScriptBuilder.WriteString("/////// Auto-generated by Vorma. Do not edit.\n\n")
	typeScriptBuilder.WriteString("export const staticPublicAssetMap = {\n")
	for _, key := range sortedKeys {
		typeScriptBuilder.WriteString(
			fmt.Sprintf("\t%q: %q,\n", key, publicFileMap[key]),
		)
	}
	typeScriptBuilder.WriteString("} as const;\n")
	return typeScriptBuilder.String()
}

func (executor buildInnerPublicFileMapExecutor) runWithPublicFileMapWriter(
	v *vormaruntime.Vorma,
	runWithWriter func(buildInnerPublicFileMapWriter) error,
) (operationErr error) {
	writer := executor.dependencies.newPublicFileMapWriter(v)
	return artifactio.RunWithClosableResource(
		writer,
		"close wave builder",
		runWithWriter,
	)
}

func logBuildInnerCompletion(v *vormaruntime.Vorma, start time.Time) {
	v.Log.Info("DONE building Vorma",
		"buildID", v.BuildID(),
		"routes found", len(v.Paths()),
		"duration", time.Since(start),
	)
}

func generateBuildIDWithPrefix(
	prefix string,
	generateBuildIDSuffix func() (string, error),
) (string, error) {
	buildIDSuffix, err := generateBuildIDSuffix()
	if err != nil {
		return "", fmt.Errorf("generate build ID: %w", err)
	}

	return prefix + buildIDSuffix, nil
}

// Run executes a single build-inner hook pipeline.
func Run(v *vormaruntime.Vorma, opts *RunOptions) error {
	return buildInner(v, runOptionsToBuildInnerOptions(opts))
}

func runOptionsToBuildInnerOptions(opts *RunOptions) *buildInnerOptions {
	if opts == nil {
		return nil
	}
	return &buildInnerOptions{isDev: opts.IsDev}
}

// Package devreload owns dev-only route/watch reload orchestration for
// vormabuild flows.
//
// It centralizes attempt-scoped rebuild behavior, route-manifest rollback, and
// deferred runtime reload metadata generation so watch callbacks and default
// watch-pattern setup remain thin orchestration glue.
package devreload

import (
	"errors"
	"fmt"
	"github.com/vormadev/vorma/wave/waveframework"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/vormadev/vorma/internal/artifactio"
	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/kit/id"
	"github.com/vormadev/vorma/vormabuild/internal/buildlifecycle"
	"github.com/vormadev/vorma/vormabuild/internal/routeartifacts"
	"github.com/vormadev/vorma/vormabuild/internal/routeparse"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/wavewatch"
)

type fastRouteRebuildDependencies struct {
	parseClientRoutes             func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error)
	newFastRebuildID              func() (string, error)
	runRouteSyncExecution         func(*vormaruntime.Vorma, buildlifecycle.RouteSyncExecutionOptions) error
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
		parseClientRoutes:             routeparse.ParseClientRoutes,
		newFastRebuildID:              newFastRebuildID,
		runRouteSyncExecution:         buildlifecycle.RunRouteSyncExecution,
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
		writeRouteArtifacts:           routeartifacts.WriteRouteArtifactsWithoutHoldingRuntimeLock,
		readRouteManifestArtifact:     os.ReadFile,
		writeRouteManifestArtifact:    artifactio.WriteFileAtomically,
		removeRouteManifestArtifact:   os.Remove,
		getCurrentBuildIDWithReadLock: buildlifecycle.CurrentBuildIDWithReadLock,
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
		dependencies: normalizeFastRouteRebuildArtifactDependencies(
			dependencies,
		),
	}
}

func newFastRouteRebuildExecutor(
	dependencies fastRouteRebuildDependencies,
	artifactDependencies fastRouteRebuildArtifactDependencies,
) fastRouteRebuildExecutor {
	return fastRouteRebuildExecutor{
		dependencies: normalizeFastRouteRebuildDependencies(dependencies),
		artifactExecutor: newFastRouteRebuildArtifactExecutor(
			artifactDependencies,
		),
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
		dependencies: normalizeFastRouteRebuildBuildIDDependencies(
			dependencies,
		),
	}
}

// RebuildRoutesOnly is the fast path for rebuilding when only vorma.routes.ts changes.
// Runs in Process A (Dev Server), which has handlers registered for type reflection.
//
// Flow:
//  1. Parse client routes with esbuild
//  2. Generate TypeScript using live reflection (Process A has handlers)
//  3. Write all artifacts to disk
//  4. Wave calls Process B's reload endpoint to sync from disk
//
// Performance: ~50ms vs ~1.5s for full rebuild
func RebuildRoutesOnly(v *vormaruntime.Vorma) error {
	return defaultFastRouteRebuildExecutor.rebuildRoutesOnly(v)
}

func rebuildRoutesOnlyWithDependencies(
	v *vormaruntime.Vorma,
	dependencies fastRouteRebuildDependencies,
	artifactDependencies fastRouteRebuildArtifactDependencies,
) error {
	return newFastRouteRebuildExecutor(
		dependencies,
		artifactDependencies,
	).rebuildRoutesOnly(v)
}

func (executor fastRouteRebuildExecutor) rebuildRoutesOnly(
	v *vormaruntime.Vorma,
) error {
	start := time.Now()

	if !v.IsDevMode() {
		return errors.New("rebuildRoutesOnly should only be called in dev mode")
	}

	buildLifecycleStateMachine, err := buildlifecycle.NewLifecycleStateMachineWithOptions(
		buildlifecycle.WorkflowFastRouteRebuild,
		v.Log,
		buildlifecycle.LifecycleStateMachineOptions{
			AttemptInputs: []buildlifecycle.LifecycleAttemptInput{
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
	if err := buildLifecycleStateMachine.TransitionTo(buildlifecycle.PhaseStarted, "fast route rebuild started"); err != nil {
		return fmt.Errorf("transition build lifecycle to started: %w", err)
	}

	v.Log.Info("START fast route rebuild")

	err = executor.dependencies.runRouteSyncExecution(
		v,
		buildlifecycle.RouteSyncExecutionOptions{
			ParseClientRoutes:          executor.dependencies.parseClientRoutes,
			GenerateBuildID:            executor.dependencies.newFastRebuildID,
			ParseClientRoutesErrorText: "parse client routes",
			PostSyncHook: func(v *vormaruntime.Vorma) error {
				if err := buildLifecycleStateMachine.TransitionTo(
					buildlifecycle.PhaseRoutesSynchronized,
					"client routes synchronized",
				); err != nil {
					return fmt.Errorf(
						"transition build lifecycle to routes-synchronized: %w",
						err,
					)
				}
				if err := executor.artifactExecutor.writeFastRebuildArtifactsAfterRouteSync(v); err != nil {
					return err
				}
				if err := buildLifecycleStateMachine.TransitionTo(
					buildlifecycle.PhaseRouteArtifactsWritten,
					"route artifacts written",
				); err != nil {
					return fmt.Errorf(
						"transition build lifecycle to route-artifacts-written: %w",
						err,
					)
				}
				return nil
			},
		},
	)
	if err != nil {
		if rollbackTraceErr := buildLifecycleStateMachine.RecordRollback(
			buildlifecycle.DecisionRequired,
			buildlifecycle.OutcomeNotAttempted,
			"fast rebuild failure path requires route-manifest rollback in artifact writer",
			nil,
		); rollbackTraceErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf(
					"record lifecycle rollback decision: %w",
					rollbackTraceErr,
				),
			)
		}
		if transitionErr := buildLifecycleStateMachine.TransitionToFailed("fast route rebuild failed", err); transitionErr != nil {
			return errors.Join(
				err,
				fmt.Errorf(
					"transition build lifecycle to failed: %w",
					transitionErr,
				),
			)
		}
		return err
	}
	if rollbackTraceErr := buildLifecycleStateMachine.RecordRollback(
		buildlifecycle.DecisionNotRequired,
		buildlifecycle.OutcomeNotRequired,
		"fast route rebuild completed successfully without requiring rollback",
		nil,
	); rollbackTraceErr != nil {
		return fmt.Errorf(
			"record lifecycle rollback decision: %w",
			rollbackTraceErr,
		)
	}
	if err := buildLifecycleStateMachine.TransitionTo(buildlifecycle.PhaseCompleted, "fast route rebuild completed"); err != nil {
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

func writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
	v *vormaruntime.Vorma,
	dependencies fastRouteRebuildArtifactDependencies,
) error {
	return newFastRouteRebuildArtifactExecutor(
		dependencies,
	).writeFastRebuildArtifactsAfterRouteSync(v)
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
	return buildlifecycle.RunWithRollbackOnFailureAndPanic(
		buildlifecycle.RollbackTransactionOptions{
			Run: func() error {
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
			RollbackOnFailure: func() error {
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
			RollbackErrorContext: "restore route manifest artifact",
			LogRollbackFailureAfterPanic: func(rollbackErr error) {
				if v.Log != nil {
					v.Log.Error(
						"restore route manifest artifact after panic failed",
						"error",
						rollbackErr,
					)
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

type fastRebuildRouteManifestArtifactSnapshot = artifactio.BuildArtifactFileSnapshot

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
	return artifactio.CaptureBuildArtifactFile(
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
	return artifactio.RestoreBuildArtifactFile(
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
	err := removeMatchingTopLevelFiles(
		staticPublicOutDir,
		isGeneratedRouteManifestFilename,
	)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func removeMatchingTopLevelFiles(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !shouldRemove(name) {
			continue
		}
		if err := os.Remove(filepath.Join(rootDir, name)); err != nil {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	return nil
}

func isGeneratedRouteManifestFilename(fileName string) bool {
	return len(fileName) > len(vormaruntime.VormaRouteManifestPrefix) &&
		strings.HasPrefix(fileName, vormaruntime.VormaRouteManifestPrefix)
}

type reloadActionDependencies struct {
	nextReloadAttemptID func() string
}

type reloadActionExecutor struct {
	dependencies reloadActionDependencies
}

var reloadAttemptSequence atomic.Uint64

var defaultReloadActionExecutor = newReloadActionExecutor(
	reloadActionDependencies{
		nextReloadAttemptID: nextReloadAttemptID,
	},
)

func nextReloadAttemptID() string {
	attemptSequence := reloadAttemptSequence.Add(1)
	return fmt.Sprintf("reload-%d", attemptSequence)
}

func normalizeReloadActionDependencies(
	dependencies reloadActionDependencies,
) reloadActionDependencies {
	if dependencies.nextReloadAttemptID == nil {
		dependencies.nextReloadAttemptID = nextReloadAttemptID
	}
	return dependencies
}

func newReloadActionExecutor(
	dependencies reloadActionDependencies,
) reloadActionExecutor {
	return reloadActionExecutor{
		dependencies: normalizeReloadActionDependencies(dependencies),
	}
}

// GetDeferredFrameworkRuntimeReloadAction resolves the refresh action for a
// watcher-triggered runtime reload request.
func GetDeferredFrameworkRuntimeReloadAction(
	v *vormaruntime.Vorma,
	endpoint string,
	warnMessage string,
	reloadTrigger string,
	hookContext *wavewatch.HookContext,
	expectedBuildID string,
) *wavewatch.RefreshAction {
	return defaultReloadActionExecutor.getDeferredFrameworkRuntimeReloadAction(
		v,
		endpoint,
		warnMessage,
		reloadTrigger,
		hookContext,
		expectedBuildID,
	)
}

func (executor reloadActionExecutor) getDeferredFrameworkRuntimeReloadAction(
	v *vormaruntime.Vorma,
	endpoint string,
	warnMessage string,
	reloadTrigger string,
	_ *wavewatch.HookContext,
	expectedBuildID string,
) *wavewatch.RefreshAction {
	trimmedEndpoint := strings.TrimSpace(endpoint)
	if trimmedEndpoint == "" {
		if v != nil && v.Log != nil {
			v.Log.Warn(
				warnMessage,
				"error",
				errors.New("reload endpoint path is required"),
				"fallback_action",
				"restart-without-recompile",
				"fallback_reason",
				"reload-endpoint-path-missing",
			)
		}
		return newRestartWithoutRecompileAction()
	}
	if !strings.HasPrefix(trimmedEndpoint, "/") {
		trimmedEndpoint = "/" + trimmedEndpoint
	}
	normalizedExpectedBuildID := strings.TrimSpace(expectedBuildID)

	reloadAction := newReloadBrowserAndWaitAction()
	reloadAction.FrameworkRuntimeReloadRequest = &wavewatch.FrameworkRuntimeReloadRequest{
		EndpointPath:    trimmedEndpoint,
		ReloadAttemptID: executor.dependencies.nextReloadAttemptID(),
		ExpectedBuildID: normalizedExpectedBuildID,
		ReloadTrigger:   strings.TrimSpace(reloadTrigger),
	}
	return reloadAction
}

func newReloadBrowserAndWaitAction() *wavewatch.RefreshAction {
	return &wavewatch.RefreshAction{
		ReloadBrowser: true,
		WaitForApp:    true,
		WaitForVite:   true,
	}
}

func newRestartWithoutRecompileAction() *wavewatch.RefreshAction {
	return &wavewatch.RefreshAction{
		TriggerRestart: true,
		RecompileGo:    false,
	}
}

const (
	watchReloadTriggerRouteDefinitions = "route-definitions-watch"
	watchReloadTriggerHTMLTemplate     = "html-template-watch"
)

type frameworkReloadActionResolver func(
	v *vormaruntime.Vorma,
	reloadEndpoint string,
	warnMessage string,
	reloadTrigger string,
	hookContext *wavewatch.HookContext,
	expectedBuildID string,
) *wavewatch.RefreshAction

func InjectDefaultWatchPatterns(v *vormaruntime.Vorma) waveconfig.ParsedConfig {
	cfg := v.Wave.ParsedConfig()
	InjectDefaultWatchPatternsInConfig(cfg, v)
	return cfg
}

func InjectDefaultWatchPatternsInConfig(
	cfg waveconfig.ParsedConfig,
	v *vormaruntime.Vorma,
) {
	if !shouldInjectDefaultWatchPatterns(v) {
		return
	}
	if cfg == nil {
		return
	}

	patterns := getDefaultWatchPatterns(v)
	appendMissingFrameworkWatchPatterns(cfg, patterns)

	if v.Config.TSGenOutDir() != "" {
		injectGeneratedOutputPathsForDefaultWatchPatterns(
			cfg,
			v.Config.TSGenOutDir(),
		)
	}
}

func appendMissingFrameworkWatchPatterns(
	cfg waveconfig.ParsedConfig,
	defaultPatterns []wavewatch.WatchedFile,
) {
	for _, defaultPattern := range defaultPatterns {
		if hasFrameworkWatchPattern(
			waveframework.StateForConfig(cfg).WatchPatterns,
			defaultPattern.Pattern,
		) {
			continue
		}
		waveframework.StateForConfig(cfg).WatchPatterns = append(
			waveframework.StateForConfig(cfg).WatchPatterns,
			defaultPattern,
		)
	}
}

func hasFrameworkWatchPattern(
	existingPatterns []wavewatch.WatchedFile,
	pattern string,
) bool {
	normalizedExpectedPattern := normalizeFrameworkWatchPatternPath(pattern)
	for _, existingPattern := range existingPatterns {
		if existingPattern.Pattern == pattern {
			return true
		}
		if normalizeFrameworkWatchPatternPath(existingPattern.Pattern) ==
			normalizedExpectedPattern {
			return true
		}
	}
	return false
}

func getDefaultWatchPatterns(v *vormaruntime.Vorma) []wavewatch.WatchedFile {
	var patterns []wavewatch.WatchedFile

	patterns = append(patterns, routeDefinitionWatchPatterns(v)...)

	htmlTemplatePattern := htmlTemplateWatchPattern(v)
	if htmlTemplatePattern != nil {
		patterns = append(patterns, *htmlTemplatePattern)
	}

	patterns = append(patterns, goFilesWatchPattern())

	return patterns
}

func routeDefinitionWatchPatterns(v *vormaruntime.Vorma) []wavewatch.WatchedFile {
	normalizedRouteDefinitionPatterns, err := routeparse.NormalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ClientRouteDefinitionPatterns(),
	)
	if err != nil {
		panic(
			fmt.Sprintf("normalize client route definition patterns: %v", err),
		)
	}
	if len(normalizedRouteDefinitionPatterns) == 0 {
		return nil
	}

	onChangeCallback := routeDefinitionsOnChangeCallback(v)
	watchPatterns := make(
		[]wavewatch.WatchedFile,
		0,
		len(normalizedRouteDefinitionPatterns),
	)
	for _, routeDefinitionPattern := range normalizedRouteDefinitionPatterns {
		normalizedWatchPattern := normalizeFrameworkWatchPatternPath(
			routeDefinitionPattern,
		)
		watchPatterns = append(
			watchPatterns,
			runOnChangeOnlyWatchPattern(
				normalizedWatchPattern,
				onChangeCallback,
				true,
			),
		)
	}
	return watchPatterns
}

func htmlTemplateWatchPattern(v *vormaruntime.Vorma) *wavewatch.WatchedFile {
	htmlTemplateLocation := v.Config.HTMLTemplateLocation()
	privateStaticDir := v.Wave.PrivateStaticDir()
	if htmlTemplateLocation == "" || privateStaticDir == "" {
		return nil
	}

	templatePath := normalizeFrameworkWatchPatternPath(
		filepath.Join(privateStaticDir, htmlTemplateLocation),
	)
	watchPattern := wavewatch.WatchedFile{
		Pattern: templatePath,
		OnChangeHooks: []wavewatch.OnChangeHook{{
			Timing:   wavewatch.OnChangeStrategyPost,
			Callback: htmlTemplateOnChangeCallback(v),
		}},
	}
	return &watchPattern
}

func normalizeFrameworkWatchPatternPath(pathPattern string) string {
	if pathPattern == "" {
		return ""
	}

	cleanedPathPattern := filepath.Clean(pathPattern)
	if filepath.IsAbs(cleanedPathPattern) {
		return cleanedPathPattern
	}

	absolutePathPattern, absolutePathError := filepath.Abs(cleanedPathPattern)
	if absolutePathError != nil {
		return cleanedPathPattern
	}

	return filepath.Clean(absolutePathPattern)
}

func goFilesWatchPattern() wavewatch.WatchedFile {
	return wavewatch.WatchedFile{
		Pattern: "**/*.go",
		OnChangeHooks: []wavewatch.OnChangeHook{{
			RunCombinedDevBuildHookCommands: true,
			Timing:                          wavewatch.OnChangeStrategyConcurrent,
		}},
	}
}

func routeDefinitionsOnChangeCallback(
	v *vormaruntime.Vorma,
) func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
	return routeDefinitionsOnChangeCallbackWithReloadActionResolver(
		v,
		GetDeferredFrameworkRuntimeReloadAction,
	)
}

func routeDefinitionsOnChangeCallbackWithReloadActionResolver(
	v *vormaruntime.Vorma,
	resolveReloadAction frameworkReloadActionResolver,
) func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
	return watchReloadCallback(
		v,
		v.DevReloadRoutesEndpointPath(),
		"route reload endpoint path is invalid, falling back to restart",
		watchReloadTriggerRouteDefinitions,
		RebuildRoutesOnly,
		resolveReloadAction,
	)
}

func htmlTemplateOnChangeCallback(
	v *vormaruntime.Vorma,
) func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
	return htmlTemplateOnChangeCallbackWithReloadActionResolver(
		v,
		GetDeferredFrameworkRuntimeReloadAction,
	)
}

func htmlTemplateOnChangeCallbackWithReloadActionResolver(
	v *vormaruntime.Vorma,
	resolveReloadAction frameworkReloadActionResolver,
) func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
	return watchReloadCallback(
		v,
		v.DevReloadTemplateEndpointPath(),
		"template reload endpoint path is invalid, falling back to restart",
		watchReloadTriggerHTMLTemplate,
		nil,
		resolveReloadAction,
	)
}

func watchReloadCallback(
	v *vormaruntime.Vorma,
	reloadEndpoint string,
	reloadEndpointFailureWarnMessage string,
	reloadTrigger string,
	preReloadAction func(*vormaruntime.Vorma) error,
	resolveReloadAction frameworkReloadActionResolver,
) func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
	if resolveReloadAction == nil {
		resolveReloadAction = GetDeferredFrameworkRuntimeReloadAction
	}

	return func(ctx *wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
		expectedBuildID := ""
		if v != nil {
			expectedBuildID = strings.TrimSpace(v.BuildID())
		}
		if preReloadAction != nil {
			if err := preReloadAction(v); err != nil {
				return nil, fmt.Errorf(
					"run pre-reload action for trigger %q: %w",
					reloadTrigger,
					err,
				)
			}
		}

		if ctx != nil && ctx.AppStoppedForBatch {
			if v.Log != nil {
				v.Log.Debug(
					"watch reload callback skipped",
					"reload_endpoint",
					reloadEndpoint,
					"reload_trigger",
					reloadTrigger,
					"skip_reason",
					"app-stopped-for-batch",
				)
			}
			return nil, nil
		}

		return resolveReloadAction(
			v,
			reloadEndpoint,
			reloadEndpointFailureWarnMessage,
			reloadTrigger,
			ctx,
			expectedBuildID,
		), nil
	}
}

func shouldInjectDefaultWatchPatterns(v *vormaruntime.Vorma) bool {
	if v.Config.IncludeDefaults() == nil {
		return true
	}
	return *v.Config.IncludeDefaults()
}

func injectGeneratedOutputPathsForDefaultWatchPatterns(
	cfg waveconfig.ParsedConfig,
	tsGenOutDir string,
) {
	appendMissingFrameworkIgnoredPatterns(
		cfg,
		filepath.Join(
			tsGenOutDir,
			runtimepaths.GeneratedTypeScriptIndexFileName,
		),
		filepath.Join(
			tsGenOutDir,
			runtimepaths.GeneratedTypeScriptPublicFileMapFileName,
		),
	)
}

func appendMissingFrameworkIgnoredPatterns(
	cfg waveconfig.ParsedConfig,
	defaultIgnoredPatterns ...string,
) {
	for _, defaultIgnoredPattern := range defaultIgnoredPatterns {
		if hasString(waveframework.StateForConfig(cfg).IgnoredPatterns, defaultIgnoredPattern) {
			continue
		}
		waveframework.StateForConfig(cfg).IgnoredPatterns = append(
			waveframework.StateForConfig(cfg).IgnoredPatterns,
			defaultIgnoredPattern,
		)
	}
}

func hasString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func runOnChangeOnlyWatchPattern(
	pattern string,
	callback func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error),
	skipRebuildingNotification bool,
) wavewatch.WatchedFile {
	return wavewatch.WatchedFile{
		Pattern:         pattern,
		RunOnChangeOnly: true,
		OnChangeHooks: []wavewatch.OnChangeHook{{
			Callback: callback,
		}},
		SkipRebuildingNotification: skipRebuildingNotification,
	}
}

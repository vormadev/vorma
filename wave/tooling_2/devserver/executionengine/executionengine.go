package executionengine

import (
	"context"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling_2/builder"
	"github.com/vormadev/vorma/wave/tooling_2/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/tooling_2/devserver/hooks"
	"github.com/vormadev/vorma/wave/tooling_2/watch"
	"golang.org/x/sync/errgroup"
	"log/slog"
)

// WatcherExecutionTraceContext carries watcher-cycle correlation ids.
type WatcherExecutionTraceContext struct {
	CycleID uint64
	BatchID uint64
}

// Dependencies defines all orchestration callbacks required by Engine.
type Dependencies struct {
	Log    *slog.Logger
	Config *wave.ParsedConfig

	GetCurrentWatcher func() *watch.Watcher
	GetCurrentBuilder func() *toolingbuilder.Builder

	CurrentRunCycleContextOrBackground func() context.Context
	ExecuteBuildPhase                  func(*eventpipeline.WorkSet) error
	ExecuteBrowserPhase                func(*eventpipeline.WorkSet)
	StartApp                           func()
	StopApp                            func() error
	TriggerRestart                     func()
	TriggerRestartNoGo                 func()
	TriggerConfigRestart               func()
	BroadcastRebuilding                func()

	BuildEventExecutionPlan func(
		events []fsnotify.Event,
		watcher *watch.Watcher,
		builder *toolingbuilder.Builder,
	) eventpipeline.EventExecutionPlanningResult

	DeriveWatcherExecutionTraceContext       func() WatcherExecutionTraceContext
	SetCurrentWatcherExecutionTraceContext   func(WatcherExecutionTraceContext)
	ClearCurrentWatcherExecutionTraceContext func()
	GetCurrentWatcherExecutionTraceContext   func() WatcherExecutionTraceContext

	RunNoWaitHookWithConcurrencyLimit               func(func())
	GetOrCreateConcurrentNoWaitHookLifecycleContext func() context.Context
}

// Engine runs watcher/event/hook orchestration independent of Server state.
type Engine struct {
	dependencies Dependencies
}

func New(dependencies Dependencies) *Engine {
	return &Engine{
		dependencies: dependencies,
	}
}

func (e *Engine) logInfo(message string, args ...any) {
	if e == nil || e.dependencies.Log == nil {
		return
	}
	e.dependencies.Log.Info(message, args...)
}

func (e *Engine) logWarn(message string, args ...any) {
	if e == nil || e.dependencies.Log == nil {
		return
	}
	e.dependencies.Log.Warn(message, args...)
}

func (e *Engine) logError(message string, args ...any) {
	if e == nil || e.dependencies.Log == nil {
		return
	}
	e.dependencies.Log.Error(message, args...)
}

func (e *Engine) shouldContinueConcurrentHookExecution(
	concurrentHookExecutionContext context.Context,
) bool {
	return hooks.ShouldContinueConcurrentHookExecution(
		concurrentHookExecutionContext,
	)
}

func (e *Engine) currentRunCycleContextOrBackground() context.Context {
	if e == nil || e.dependencies.CurrentRunCycleContextOrBackground == nil {
		return context.Background()
	}
	return e.dependencies.CurrentRunCycleContextOrBackground()
}

func (e *Engine) deriveWatcherExecutionTraceContext() WatcherExecutionTraceContext {
	if e == nil || e.dependencies.DeriveWatcherExecutionTraceContext == nil {
		return WatcherExecutionTraceContext{}
	}
	return e.dependencies.DeriveWatcherExecutionTraceContext()
}

func (e *Engine) setCurrentWatcherExecutionTraceContext(
	traceContext WatcherExecutionTraceContext,
) {
	if e == nil || e.dependencies.SetCurrentWatcherExecutionTraceContext == nil {
		return
	}
	e.dependencies.SetCurrentWatcherExecutionTraceContext(traceContext)
}

func (e *Engine) clearCurrentWatcherExecutionTraceContext() {
	if e == nil || e.dependencies.ClearCurrentWatcherExecutionTraceContext == nil {
		return
	}
	e.dependencies.ClearCurrentWatcherExecutionTraceContext()
}

func (e *Engine) getCurrentWatcherExecutionTraceContext() WatcherExecutionTraceContext {
	if e == nil || e.dependencies.GetCurrentWatcherExecutionTraceContext == nil {
		return WatcherExecutionTraceContext{}
	}
	return e.dependencies.GetCurrentWatcherExecutionTraceContext()
}

func getUserDevBuildHook(parsedConfig *wave.ParsedConfig) string {
	if parsedConfig == nil || parsedConfig.Core == nil {
		return ""
	}
	return parsedConfig.Core.DevBuildHook
}

func getFrameworkDevBuildHook(parsedConfig *wave.ParsedConfig) string {
	if parsedConfig == nil {
		return ""
	}
	return parsedConfig.FrameworkDevBuildHook
}

func (e *Engine) resolveHookCommand(hook wave.OnChangeHook) string {
	if hook.RunCombinedDevBuildHookCommands {
		if strings.TrimSpace(hook.Cmd) != "" {
			return hooks.ResolveSequentialShellCommands(
				hook.Cmd,
				getUserDevBuildHook(e.dependencies.Config),
				getFrameworkDevBuildHook(e.dependencies.Config),
			)
		}
		return hooks.ResolveSequentialShellCommands(
			getUserDevBuildHook(e.dependencies.Config),
			getFrameworkDevBuildHook(e.dependencies.Config),
		)
	}
	return hook.Cmd
}

func (e *Engine) resolveHookExecutionPlan(
	hook wave.OnChangeHook,
) hooks.HookExecutionPlan {
	if e == nil || e.dependencies.Config == nil {
		return hooks.DeriveHookExecutionPlanFromHook(hook, nil)
	}
	if !hook.RunCombinedDevBuildHookCommands ||
		e.dependencies.Config.FrameworkRunBuildHook == nil {
		return hooks.DeriveHookExecutionPlanFromHook(
			hook,
			e.resolveHookCommand,
		)
	}

	frameworkBuildHookRunner := e.dependencies.Config.FrameworkRunBuildHook
	userAndExplicitCommand := hooks.ResolveSequentialShellCommands(
		hook.Cmd,
		getUserDevBuildHook(e.dependencies.Config),
	)
	originalCallback := hook.Callback

	return hooks.HookExecutionPlan{
		Callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
			var callbackAction *wave.RefreshAction
			var callbackErr error
			if originalCallback != nil {
				callbackAction, callbackErr = originalCallback(hookContext)
				if callbackErr != nil {
					return callbackAction, callbackErr
				}
			}

			hookExecutionContext := context.Background()
			if hookContext != nil && hookContext.ExecutionContext != nil {
				hookExecutionContext = hookContext.ExecutionContext
			}

			if strings.TrimSpace(userAndExplicitCommand) != "" {
				if err := hooks.ExecuteHookCommandWithContext(
					hookExecutionContext,
					userAndExplicitCommand,
				); err != nil {
					return callbackAction, err
				}
			}

			if err := frameworkBuildHookRunner(
				hookExecutionContext,
				true,
			); err != nil {
				return callbackAction, err
			}
			return callbackAction, nil
		},
		Command:                     "",
		CommandTimeoutMilliseconds:  hook.CommandTimeoutMilliseconds,
		DisableStageCommandTimeout:  hook.DisableStageCommandTimeout,
		CallbackTimeoutMilliseconds: hook.CallbackTimeoutMilliseconds,
		DisableStageCallbackTimeout: hook.DisableStageCallbackTimeout,
	}
}

func (e *Engine) deriveHookCommandTimeoutForExecutionPlan(
	stageType hooks.HookStageType,
	executionPlan hooks.HookExecutionPlan,
) time.Duration {
	if e == nil || e.dependencies.Config == nil {
		return hooks.DeriveHookCommandTimeoutDurationForExecutionPlan(
			nil,
			stageType,
			executionPlan,
		)
	}
	return hooks.DeriveHookCommandTimeoutDurationForExecutionPlan(
		e.dependencies.Config.Watch,
		stageType,
		executionPlan,
	)
}

func (e *Engine) deriveHookCallbackTimeoutForExecutionPlan(
	stageType hooks.HookStageType,
	executionPlan hooks.HookExecutionPlan,
) time.Duration {
	if e == nil || e.dependencies.Config == nil {
		return hooks.DeriveHookCallbackTimeoutDurationForExecutionPlan(
			nil,
			stageType,
			executionPlan,
		)
	}
	return hooks.DeriveHookCallbackTimeoutDurationForExecutionPlan(
		e.dependencies.Config.Watch,
		stageType,
		executionPlan,
	)
}

func (e *Engine) runNoWaitHookWithConcurrencyLimit(runNoWaitHook func()) {
	if runNoWaitHook == nil {
		return
	}
	if e == nil || e.dependencies.RunNoWaitHookWithConcurrencyLimit == nil {
		runNoWaitHook()
		return
	}
	e.dependencies.RunNoWaitHookWithConcurrencyLimit(runNoWaitHook)
}

func (e *Engine) concurrentNoWaitHookLifecycleContext() context.Context {
	if e == nil || e.dependencies.GetOrCreateConcurrentNoWaitHookLifecycleContext == nil {
		return context.Background()
	}
	return e.dependencies.GetOrCreateConcurrentNoWaitHookLifecycleContext()
}

func (e *Engine) getCurrentWatcher() *watch.Watcher {
	if e == nil || e.dependencies.GetCurrentWatcher == nil {
		return nil
	}
	return e.dependencies.GetCurrentWatcher()
}

func (e *Engine) getCurrentBuilder() *toolingbuilder.Builder {
	if e == nil || e.dependencies.GetCurrentBuilder == nil {
		return nil
	}
	return e.dependencies.GetCurrentBuilder()
}

func (e *Engine) executeBuildPhase(work *eventpipeline.WorkSet) error {
	if e == nil || e.dependencies.ExecuteBuildPhase == nil {
		return nil
	}
	return e.dependencies.ExecuteBuildPhase(work)
}

func (e *Engine) executeBrowserPhase(work *eventpipeline.WorkSet) {
	if e == nil || e.dependencies.ExecuteBrowserPhase == nil {
		return
	}
	e.dependencies.ExecuteBrowserPhase(work)
}

func (e *Engine) startApp() {
	if e == nil || e.dependencies.StartApp == nil {
		return
	}
	e.dependencies.StartApp()
}

func (e *Engine) stopApp() error {
	if e == nil || e.dependencies.StopApp == nil {
		return nil
	}
	return e.dependencies.StopApp()
}

func (e *Engine) triggerRestart() {
	if e == nil || e.dependencies.TriggerRestart == nil {
		return
	}
	e.dependencies.TriggerRestart()
}

func (e *Engine) triggerRestartNoGo() {
	if e == nil || e.dependencies.TriggerRestartNoGo == nil {
		return
	}
	e.dependencies.TriggerRestartNoGo()
}

func (e *Engine) triggerConfigRestart() {
	if e == nil || e.dependencies.TriggerConfigRestart == nil {
		return
	}
	e.dependencies.TriggerConfigRestart()
}

func (e *Engine) broadcastRebuilding() {
	if e == nil || e.dependencies.BroadcastRebuilding == nil {
		return
	}
	e.dependencies.BroadcastRebuilding()
}

func (e *Engine) buildEventExecutionPlan(
	events []fsnotify.Event,
	watcher *watch.Watcher,
	builder *toolingbuilder.Builder,
) eventpipeline.EventExecutionPlanningResult {
	if e == nil || e.dependencies.BuildEventExecutionPlan == nil {
		return eventpipeline.EventExecutionPlanningResult{}
	}
	return e.dependencies.BuildEventExecutionPlan(events, watcher, builder)
}

func (e *Engine) runOnEventsWithWatcher(
	watcher *watch.Watcher,
	watcherExecutionContext context.Context,
) {
	debouncer := watch.NewDebouncer(30*time.Millisecond, func(events []fsnotify.Event) {
		if watcherExecutionContext != nil {
			select {
			case <-watcherExecutionContext.Done():
				return
			default:
			}
		}
		e.ProcessEvents(events)
	})
	defer debouncer.Stop()

	for {
		select {
		case <-watcherExecutionContext.Done():
			return
		case watcherEvent, ok := <-watcher.Events():
			if !ok {
				return
			}
			debouncer.Add(watcherEvent)
		case watcherError, ok := <-watcher.Errors():
			if !ok {
				return
			}
			if watcherError != nil {
				e.logError("watcher error", "error", watcherError)
			}
		}
	}
}

// RunWatcherWithContext consumes watcher events until context cancellation or
// watcher closure.
func (e *Engine) RunWatcherWithContext(
	watcherExecutionContext context.Context,
) {
	watcher := e.getCurrentWatcher()
	if watcher == nil {
		return
	}
	e.runOnEventsWithWatcher(watcher, watcherExecutionContext)
}

// ProcessEvents evaluates one debounced watcher batch and executes the
// deterministic pipeline for that batch.
func (e *Engine) ProcessEvents(events []fsnotify.Event) {
	watcher := e.getCurrentWatcher()
	builder := e.getCurrentBuilder()
	if watcher == nil || builder == nil {
		return
	}

	traceContextForWatcherExecution := e.deriveWatcherExecutionTraceContext()
	e.setCurrentWatcherExecutionTraceContext(traceContextForWatcherExecution)
	defer e.clearCurrentWatcherExecutionTraceContext()

	executionPlanningResult := e.buildEventExecutionPlan(events, watcher, builder)
	watcherEventExecutionInputForPlanningResult := eventpipeline.BuildWatcherEventExecutionInputFromPlanningResult(
		executionPlanningResult,
	)
	watcherEventFlowDecisionForPlanningResult := watcherEventExecutionInputForPlanningResult.FlowDecision
	if watcherEventFlowDecisionForPlanningResult.TriggerConfigRestart {
		e.logInfo("Config changed, restarting")
		e.triggerConfigRestart()
		return
	}

	eventsWithHooksForExecution := watcherEventExecutionInputForPlanningResult.EventsWithHooks
	if len(eventsWithHooksForExecution) == 0 {
		return
	}

	if watcherEventFlowDecisionForPlanningResult.BroadcastRebuildingOverlay {
		e.broadcastRebuilding()
	}

	work := &eventpipeline.WorkSet{}
	for _, watcherEventLogPayloadForExecutionPlan := range watcherEventExecutionInputForPlanningResult.WatcherEventLogPayloads {
		e.logInfo(
			"[watcher]",
			"op",
			watcherEventLogPayloadForExecutionPlan.Operation,
			"file",
			watcherEventLogPayloadForExecutionPlan.FilePath,
			"cycle_id",
			traceContextForWatcherExecution.CycleID,
			"batch_id",
			traceContextForWatcherExecution.BatchID,
		)
	}

	e.ExecuteEventExecutionPlan(
		eventsWithHooksForExecution,
		watcherEventFlowDecisionForPlanningResult.BehavioralDecision,
		work,
		watcher,
	)

	watcher.RemoveStale()
}

func (e *Engine) configuredHookStageFailurePolicy() string {
	if e == nil || e.dependencies.Config == nil || e.dependencies.Config.Watch == nil {
		return ""
	}
	return e.dependencies.Config.Watch.HookStageFailurePolicy
}

// ExecuteEventExecutionPlan applies stop strategy and runs deterministic
// execution for one planned event batch.
func (e *Engine) ExecuteEventExecutionPlan(
	eventsWithHooks []eventpipeline.EventWithHooks,
	behavioralDecision eventpipeline.EventExecutionPlanBehavioralDecision,
	work *eventpipeline.WorkSet,
	watcher *watch.Watcher,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	eventsWithHooksForExecution := hooks.DeriveEventsWithHooksForExecution(
		eventsWithHooks,
		behavioralDecision.AppStopStrategy,
	)
	if len(eventsWithHooksForExecution) == 0 {
		return
	}

	e.ExecuteAppStopStrategy(behavioralDecision.AppStopStrategy)
	e.ProcessEventsWithDeterministicPipeline(
		behavioralDecision,
		work,
		watcher,
		eventsWithHooksForExecution,
	)
}

// ProcessEventsWithDeterministicPipeline runs pre/build+concurrent/post phases
// in deterministic order.
func (e *Engine) ProcessEventsWithDeterministicPipeline(
	behavioralDecision eventpipeline.EventExecutionPlanBehavioralDecision,
	work *eventpipeline.WorkSet,
	watcher *watch.Watcher,
	eventsWithHooks []eventpipeline.EventWithHooks,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	e.FireNoWaitHooksForEvents(eventsWithHooks, watcher)

	preHookStageResult := hooks.RunAndApplyHookStageActionsAndErrorsToWorkSet(
		hooks.HookStageTypePre,
		func() ([]wave.RefreshAction, []error) {
			return e.RunPreHooksForEventsWithErrors(eventsWithHooks, work, watcher)
		},
		work,
	)
	if !e.ContinuePipelineAfterHookStageOrTriggerRestart(preHookStageResult) {
		return
	}

	implicitBuildDecision := hooks.DeriveImplicitBuildExecutionDecision(
		behavioralDecision.RunImplicitBuild,
		len(eventsWithHooks),
	)
	if !implicitBuildDecision.ShouldRunImplicitBuild {
		e.logInfo(implicitBuildDecision.SkipImplicitBuildLogEntry)
	} else if e != nil && e.dependencies.Config != nil {
		work.Resolve(e.dependencies.Config.UsingVite())
	}

	buildAndConcurrentHooksContext, cancelBuildAndConcurrentHooks := context.WithCancel(
		e.currentRunCycleContextOrBackground(),
	)
	defer cancelBuildAndConcurrentHooks()

	var buildAndConcurrentHooksGroup errgroup.Group
	if implicitBuildDecision.ShouldRunImplicitBuild {
		buildAndConcurrentHooksGroup.Go(func() error {
			buildPhaseError := e.executeBuildPhase(work)
			if buildPhaseError != nil {
				cancelBuildAndConcurrentHooks()
				return buildPhaseError
			}
			return nil
		})
	}

	var concurrentActions []wave.RefreshAction
	var concurrentHookExecutionErrors []error
	buildAndConcurrentHooksGroup.Go(func() error {
		concurrentActions, concurrentHookExecutionErrors = e.RunConcurrentHooksForEventsWithContextAndErrors(
			buildAndConcurrentHooksContext,
			eventsWithHooks,
			watcher,
		)
		return nil
	})
	buildAndConcurrentHooksError := buildAndConcurrentHooksGroup.Wait()
	if buildAndConcurrentHooksError != nil {
		e.logWarn(
			"Stopping pipeline after build phase failure",
			"error",
			buildAndConcurrentHooksError,
		)
		return
	}

	concurrentHookStageResult := hooks.RunAndApplyHookStageActionsAndErrorsToWorkSet(
		hooks.HookStageTypeConcurrent,
		func() ([]wave.RefreshAction, []error) {
			return concurrentActions, concurrentHookExecutionErrors
		},
		work,
	)
	if !e.ContinuePipelineAfterHookStageOrTriggerRestart(concurrentHookStageResult) {
		return
	}

	postHookStageResult := hooks.RunAndApplyHookStageActionsAndErrorsToWorkSet(
		hooks.HookStageTypePost,
		func() ([]wave.RefreshAction, []error) {
			return e.RunPostHooksForEventsWithErrors(eventsWithHooks, watcher)
		},
		work,
	)
	if !e.ContinuePipelineAfterHookStageOrTriggerRestart(postHookStageResult) {
		return
	}

	if hooks.ShouldStartAppAfterImplicitBuild(
		implicitBuildDecision.ShouldRunImplicitBuild,
		work.Restart,
	) {
		e.logInfo("Restarting app")
		e.startApp()
	}

	if hooks.ShouldExecuteBrowserPhaseAfterHookStageResults(
		preHookStageResult,
		concurrentHookStageResult,
		postHookStageResult,
	) {
		e.executeBrowserPhase(work)
	}
}

// ExecuteAppStopStrategy applies restart behavior for the current event batch.
func (e *Engine) ExecuteAppStopStrategy(
	appStopStrategyForExecution eventpipeline.AppStopStrategy,
) {
	switch appStopStrategyForExecution {
	case eventpipeline.AppStopStrategySingleEventHardReload:
		e.logInfo("Terminating running app")
		if err := e.stopApp(); err != nil {
			e.logError("Failed to terminate app", "error", err)
		}

	case eventpipeline.AppStopStrategyBatchHardReload:
		e.logInfo("Stopping app for batch rebuild")
		if err := e.stopApp(); err != nil {
			e.logError("Failed to stop app", "error", err)
		}

	case eventpipeline.AppStopStrategyNone:
	}
}

// ContinuePipelineAfterHookStageOrTriggerRestart resolves continuation against
// configured failure policy.
func (e *Engine) ContinuePipelineAfterHookStageOrTriggerRestart(
	hookStageResultForContinuation hooks.HookStageResult,
) bool {
	return e.ContinuePipelineAfterHookStageOrTriggerRestartWithFailurePolicy(
		hookStageResultForContinuation,
		hooks.DeriveHookStageFailurePolicy(
			hookStageResultForContinuation.StageType,
			e.configuredHookStageFailurePolicy(),
		),
	)
}

// ContinuePipelineAfterHookStageOrTriggerRestartWithFailurePolicy applies
// continuation policy and dispatches restart side effects when needed.
func (e *Engine) ContinuePipelineAfterHookStageOrTriggerRestartWithFailurePolicy(
	hookStageResultForContinuation hooks.HookStageResult,
	hookStageFailurePolicyForContinuation hooks.HookStageFailurePolicy,
) bool {
	continuationDecision := hooks.DeriveHookStageContinuationDecisionWithFailurePolicy(
		hookStageResultForContinuation,
		hookStageFailurePolicyForContinuation,
	)
	if continuationDecision.ShouldContinue {
		return true
	}

	if continuationDecision.StopReason == hooks.HookStageContinuationStopReasonRestartRequested {
		e.TriggerRestartFromRefreshActions(continuationDecision.RestartActionResult)
	}
	if continuationDecision.StopReason == hooks.HookStageContinuationStopReasonStageFailure {
		traceContextForContinuation := e.getCurrentWatcherExecutionTraceContext()
		e.logWarn(
			"Stopping pipeline after hook stage errors",
			"stage",
			hooks.DeriveHookStageLabel(hookStageResultForContinuation.StageType),
			"error_count",
			len(hookStageResultForContinuation.ExecutionErrors),
			"cycle_id",
			traceContextForContinuation.CycleID,
			"batch_id",
			traceContextForContinuation.BatchID,
		)
	}
	return false
}

// TriggerRestartFromRefreshActions applies restart mode based on action result.
func (e *Engine) TriggerRestartFromRefreshActions(
	actionResult eventpipeline.RefreshActionApplicationResult,
) {
	if actionResult.RecompileGo {
		e.triggerRestart()
		return
	}
	e.triggerRestartNoGo()
}

// RunSequentialHookStageForEligibleEventsWithErrors executes one hook stage in
// event order and collects stage actions/errors.
func (e *Engine) RunSequentialHookStageForEligibleEventsWithErrors(
	eventsWithHooks []eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
	runHooksForEvent func(eventpipeline.EventWithHooks, *watch.Watcher) ([]wave.RefreshAction, error),
	hookExecutionFailureLogMessage string,
) ([]wave.RefreshAction, []error) {
	if runHooksForEvent == nil {
		return nil, nil
	}

	descriptors := hooks.DeriveHookStageExecutionDescriptors(eventsWithHooks)
	allStageActions := make([]wave.RefreshAction, 0)
	stageExecutionErrors := make([]error, 0)
	traceContextForHookStage := e.getCurrentWatcherExecutionTraceContext()
	for _, descriptor := range descriptors {
		stageActions, err := runHooksForEvent(descriptor.EventWithHooks, watcher)
		if err != nil {
			e.logError(
				hookExecutionFailureLogMessage,
				"error",
				err,
				"cycle_id",
				traceContextForHookStage.CycleID,
				"batch_id",
				traceContextForHookStage.BatchID,
			)
			stageExecutionErrors = append(stageExecutionErrors, err)
		}
		allStageActions = append(allStageActions, stageActions...)
	}
	return allStageActions, stageExecutionErrors
}

// FireNoWaitHooksForEvents starts concurrent-no-wait hooks for all eligible
// events in descriptor order.
func (e *Engine) FireNoWaitHooksForEvents(
	eventsWithHooks []eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) {
	descriptors := hooks.DeriveHookStageExecutionDescriptors(eventsWithHooks)
	for _, descriptor := range descriptors {
		e.FireNoWaitHooks(descriptor.EventWithHooks, watcher)
	}
}

func (e *Engine) RunPreHooksForEventsWithErrors(
	eventsWithHooks []eventpipeline.EventWithHooks,
	work *eventpipeline.WorkSet,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, []error) {
	if work != nil {
		for _, eventWithHooksForPre := range eventsWithHooks {
			work.AddImplicitWork(eventWithHooksForPre.Classified)
		}
	}
	return e.RunSequentialHookStageForEligibleEventsWithErrors(
		eventsWithHooks,
		watcher,
		e.RunPreHooks,
		"Pre-hook execution failed",
	)
}

func (e *Engine) RunConcurrentHooksForEventsWithContextAndErrors(
	concurrentHookExecutionContext context.Context,
	eventsWithHooks []eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, []error) {
	descriptors := hooks.DeriveHookStageExecutionDescriptors(eventsWithHooks)
	traceContextForHookStage := e.getCurrentWatcherExecutionTraceContext()
	actionsByDescriptorIndex := make([][]wave.RefreshAction, len(descriptors))
	executionErrorsByDescriptorIndex := make([]error, len(descriptors))
	var concurrentHooksGroup errgroup.Group

	for descriptorIndex := range descriptors {
		if !e.shouldContinueConcurrentHookExecution(concurrentHookExecutionContext) {
			break
		}

		descriptorForExecution := descriptors[descriptorIndex]
		descriptorIndexForResult := descriptorIndex
		concurrentHooksGroup.Go(func() error {
			if !e.shouldContinueConcurrentHookExecution(concurrentHookExecutionContext) {
				return nil
			}

			concurrentActions, err := e.RunConcurrentHooksWithContext(
				concurrentHookExecutionContext,
				descriptorForExecution.EventWithHooks,
				watcher,
			)
			if err != nil {
				if e.shouldContinueConcurrentHookExecution(
					concurrentHookExecutionContext,
				) {
					e.logError(
						"Concurrent hook execution failed",
						"error",
						err,
						"cycle_id",
						traceContextForHookStage.CycleID,
						"batch_id",
						traceContextForHookStage.BatchID,
					)
					executionErrorsByDescriptorIndex[descriptorIndexForResult] = err
				}
			}
			actionsByDescriptorIndex[descriptorIndexForResult] = concurrentActions
			return nil
		})
	}

	_ = concurrentHooksGroup.Wait()

	allConcurrentActions := make([]wave.RefreshAction, 0)
	for _, concurrentActionsForEvent := range actionsByDescriptorIndex {
		allConcurrentActions = append(
			allConcurrentActions,
			concurrentActionsForEvent...,
		)
	}
	concurrentHookExecutionErrors := make([]error, 0)
	for _, concurrentHookExecutionError := range executionErrorsByDescriptorIndex {
		if concurrentHookExecutionError == nil {
			continue
		}
		concurrentHookExecutionErrors = append(
			concurrentHookExecutionErrors,
			concurrentHookExecutionError,
		)
	}

	return allConcurrentActions, concurrentHookExecutionErrors
}

func (e *Engine) RunPostHooksForEventsWithErrors(
	eventsWithHooks []eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, []error) {
	return e.RunSequentialHookStageForEligibleEventsWithErrors(
		eventsWithHooks,
		watcher,
		e.RunPostHooks,
		"Post-hook execution failed",
	)
}

// FireNoWaitHooks executes one event's concurrent-no-wait hook plans.
func (e *Engine) FireNoWaitHooks(
	ewh eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) {
	concurrentNoWaitHookLifecycleContext := e.concurrentNoWaitHookLifecycleContext()
	traceContextForHookExecution := e.getCurrentWatcherExecutionTraceContext()

	plans := hooks.DeriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		hooks.HookStageTypeConcurrentNoWait,
		e.resolveHookExecutionPlan,
	)
	for _, plan := range plans {
		if plan.Callback != nil {
			callbackForExecution := plan.Callback
			changedFilePathForExecution := ewh.Classified.Event.Name
			hookCallbackTimeoutForExecution := e.deriveHookCallbackTimeoutForExecutionPlan(
				hooks.HookStageTypeConcurrentNoWait,
				plan,
			)
			e.runNoWaitHookWithConcurrencyLimit(func() {
				hookCallbackExecutionContext, cancelHookCallbackExecutionContext := hooks.DeriveExecutionContextWithOptionalTimeout(
					concurrentNoWaitHookLifecycleContext,
					hookCallbackTimeoutForExecution,
				)
				if cancelHookCallbackExecutionContext != nil {
					defer cancelHookCallbackExecutionContext()
				}

				hookContextForExecution := hooks.CloneHookContextForExecution(
					ewh.HookCtx,
					hooks.DeriveHookExecutionContext(hookCallbackExecutionContext),
				)

				if _, err := hooks.ExecuteHookCallbackSafely(
					callbackForExecution,
					hookContextForExecution,
				); err != nil {
					e.logWarn(
						"concurrent-no-wait callback failed",
						"stage",
						hooks.DeriveHookStageLabel(hooks.HookStageTypeConcurrentNoWait),
						"path",
						changedFilePathForExecution,
						"error",
						err,
						"cycle_id",
						traceContextForHookExecution.CycleID,
						"batch_id",
						traceContextForHookExecution.BatchID,
					)
				}
			})
		}
		if strings.TrimSpace(plan.Command) != "" {
			commandForExecution := plan.Command
			changedFilePathForExecution := ewh.Classified.Event.Name
			hookCommandTimeoutForExecution := e.deriveHookCommandTimeoutForExecutionPlan(
				hooks.HookStageTypeConcurrentNoWait,
				plan,
			)
			e.runNoWaitHookWithConcurrencyLimit(func() {
				hookCommandExecutionContext, cancelHookCommandExecutionContext := hooks.DeriveExecutionContextWithOptionalTimeout(
					concurrentNoWaitHookLifecycleContext,
					hookCommandTimeoutForExecution,
				)
				if cancelHookCommandExecutionContext != nil {
					defer cancelHookCommandExecutionContext()
				}

				if err := hooks.ExecuteHookCommandWithContext(
					hookCommandExecutionContext,
					commandForExecution,
				); err != nil {
					e.logWarn(
						"concurrent-no-wait hook failed",
						"stage",
						hooks.DeriveHookStageLabel(hooks.HookStageTypeConcurrentNoWait),
						"path",
						changedFilePathForExecution,
						"cmd",
						commandForExecution,
						"error",
						err,
						"cycle_id",
						traceContextForHookExecution.CycleID,
						"batch_id",
						traceContextForHookExecution.BatchID,
					)
				}
			})
		}
	}
}

func (e *Engine) RunPreHooks(
	ewh eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction

	plans := hooks.DeriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		hooks.HookStageTypePre,
		e.resolveHookExecutionPlan,
	)
	for _, plan := range plans {
		action, err := e.ExecuteHookExecutionPlanWithContext(
			context.Background(),
			hooks.HookStageTypePre,
			plan,
			ewh.HookCtx,
		)
		if action != nil {
			actions = append(actions, *action)
		}
		if err != nil {
			return actions, hooks.WrapHookExecutionErrorWithStageAndPath(
				hooks.HookStageTypePre,
				ewh.Classified.Event.Name,
				err,
			)
		}
	}

	return actions, nil
}

func (e *Engine) RunConcurrentHooksWithContext(
	concurrentHookExecutionContext context.Context,
	ewh eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, error) {
	plans := hooks.DeriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		hooks.HookStageTypeConcurrent,
		e.resolveHookExecutionPlan,
	)
	if len(plans) == 0 {
		return nil, nil
	}

	actionsByHookIndex := make([]*wave.RefreshAction, len(plans))
	hookExecutionErrorsByHookIndex := make([]error, len(plans))
	var executionGroup errgroup.Group

	for hookIndex := range plans {
		if !e.shouldContinueConcurrentHookExecution(
			concurrentHookExecutionContext,
		) {
			break
		}

		planForExecution := plans[hookIndex]
		hookIndexForResult := hookIndex
		executionGroup.Go(func() error {
			if !e.shouldContinueConcurrentHookExecution(
				concurrentHookExecutionContext,
			) {
				return nil
			}

			action, err := e.ExecuteHookExecutionPlanWithContext(
				concurrentHookExecutionContext,
				hooks.HookStageTypeConcurrent,
				planForExecution,
				ewh.HookCtx,
			)
			actionsByHookIndex[hookIndexForResult] = action
			if err != nil {
				hookExecutionErrorsByHookIndex[hookIndexForResult] = hooks.WrapHookExecutionErrorWithStageAndPath(
					hooks.HookStageTypeConcurrent,
					ewh.Classified.Event.Name,
					err,
				)
			}
			return nil
		})
	}

	_ = executionGroup.Wait()
	actions := make([]wave.RefreshAction, 0, len(actionsByHookIndex))
	for _, action := range actionsByHookIndex {
		if action != nil {
			actions = append(actions, *action)
		}
	}

	return actions, hooks.JoinHookExecutionErrorsInOrder(
		hookExecutionErrorsByHookIndex,
	)
}

func (e *Engine) RunPostHooks(
	ewh eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction

	plans := hooks.DeriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		hooks.HookStageTypePost,
		e.resolveHookExecutionPlan,
	)
	for _, plan := range plans {
		action, err := e.ExecuteHookExecutionPlanWithContext(
			context.Background(),
			hooks.HookStageTypePost,
			plan,
			ewh.HookCtx,
		)
		if action != nil {
			actions = append(actions, *action)
		}
		if err != nil {
			return actions, hooks.WrapHookExecutionErrorWithStageAndPath(
				hooks.HookStageTypePost,
				ewh.Classified.Event.Name,
				err,
			)
		}
	}

	return actions, nil
}

func (e *Engine) ExecuteHookExecutionPlanWithContext(
	parentHookExecutionContext context.Context,
	stageType hooks.HookStageType,
	plan hooks.HookExecutionPlan,
	hookContext *wave.HookContext,
) (*wave.RefreshAction, error) {
	var action *wave.RefreshAction
	if plan.Callback != nil {
		hookCallbackExecutionContext, cancelHookCallbackExecutionContext := hooks.DeriveExecutionContextWithOptionalTimeout(
			parentHookExecutionContext,
			e.deriveHookCallbackTimeoutForExecutionPlan(stageType, plan),
		)
		if cancelHookCallbackExecutionContext != nil {
			defer cancelHookCallbackExecutionContext()
		}

		hookExecutionContext := hooks.DeriveHookExecutionContext(
			hookCallbackExecutionContext,
		)
		hookContextForExecution := hooks.CloneHookContextForExecution(
			hookContext,
			hookExecutionContext,
		)

		callbackAction, err := hooks.ExecuteHookCallbackSafely(
			plan.Callback,
			hookContextForExecution,
		)
		if err != nil {
			return nil, err
		}
		action = callbackAction
	}

	if strings.TrimSpace(plan.Command) != "" {
		hookCommandExecutionContext, cancelHookCommandExecutionContext := hooks.DeriveExecutionContextWithOptionalTimeout(
			parentHookExecutionContext,
			e.deriveHookCommandTimeoutForExecutionPlan(stageType, plan),
		)
		if cancelHookCommandExecutionContext != nil {
			defer cancelHookCommandExecutionContext()
		}

		if !e.shouldContinueConcurrentHookExecution(hookCommandExecutionContext) {
			return action, hooks.DeriveHookExecutionContextError(
				hookCommandExecutionContext,
			)
		}

		if err := hooks.ExecuteHookCommandWithContext(
			hookCommandExecutionContext,
			plan.Command,
		); err != nil {
			return action, err
		}
	}

	return action, nil
}

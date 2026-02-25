// Package runloop executes the dev-server watcher event loop.
//
// It owns the deterministic ordering of hooks, builds, restarts, and browser
// notifications so caller code does not orchestrate these steps ad hoc.
package runloop

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavebuild/builder"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/hooks"
	"github.com/vormadev/vorma/wave/wavedev/internal/watch"
	"golang.org/x/sync/errgroup"
)

const defaultWatcherBatchDurationWarningThreshold = 1500 * time.Millisecond

// WatcherExecutionTraceContext carries watcher-cycle and batch identifiers.
type WatcherExecutionTraceContext struct {
	CycleID uint64
	BatchID uint64
}

// Dependencies defines orchestration callbacks required by Engine.
type Dependencies struct {
	Log    *slog.Logger
	Config *wave.ParsedConfig

	GetCurrentWatcher func() *watch.Watcher
	GetCurrentBuilder func() *builder.Builder

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
		builder *builder.Builder,
	) eventpipeline.EventExecutionPlanningResult

	DeriveWatcherExecutionTraceContext       func() WatcherExecutionTraceContext
	SetCurrentWatcherExecutionTraceContext   func(WatcherExecutionTraceContext)
	ClearCurrentWatcherExecutionTraceContext func()
	GetCurrentWatcherExecutionTraceContext   func() WatcherExecutionTraceContext

	RunNoWaitHookWithConcurrencyLimit               func(func())
	GetOrCreateConcurrentNoWaitHookLifecycleContext func() context.Context

	ResolveHookExecutionPlan func(wave.OnChangeHook) hooks.HookExecutionPlan
	IsWaitingForBuildRetry   func() bool

	WatcherBatchDurationWarningThreshold time.Duration
}

// Engine owns deterministic watcher batch processing and hook/build orchestration.
type Engine struct {
	dependencies Dependencies
}

// New creates a runloop engine.
func New(dependencies Dependencies) *Engine {
	return &Engine{dependencies: dependencies}
}

// RunWatcherWithContext consumes watcher events until cancellation or watcher closure.
func (engine *Engine) RunWatcherWithContext(
	watcherExecutionContext context.Context,
) {
	if watcherExecutionContext == nil {
		watcherExecutionContext = context.Background()
	}

	watcher := engine.currentWatcher()
	if watcher == nil {
		return
	}

	debouncer := watch.NewDebouncer(
		30*time.Millisecond,
		func(events []fsnotify.Event) {
			select {
			case <-watcherExecutionContext.Done():
				return
			default:
			}
			engine.ProcessEvents(events)
		},
	)
	defer debouncer.Stop()

	for {
		select {
		case <-watcherExecutionContext.Done():
			return
		case watcherEvent, open := <-watcher.Events():
			if !open {
				return
			}
			watcher.TrackEvent(watcherEvent.Name)
			watcher.EnsureDirectoryWatchForEventPath(watcherEvent.Name)
			debouncer.Add(watcherEvent)
		case watcherError, open := <-watcher.Errors():
			if !open {
				return
			}
			if watcherError != nil {
				engine.logError("watcher error", "error", watcherError)
			}
		}
	}
}

// ProcessEvents evaluates one debounced watcher batch and executes deterministic pipeline.
func (engine *Engine) ProcessEvents(events []fsnotify.Event) {
	watcher := engine.currentWatcher()
	builder := engine.currentBuilder()
	if watcher == nil || builder == nil {
		return
	}
	for _, watcherEvent := range events {
		watcher.TrackEvent(watcherEvent.Name)
		watcher.EnsureDirectoryWatchForEventPath(watcherEvent.Name)
	}

	traceContext := engine.deriveWatcherExecutionTraceContext()
	engine.setCurrentWatcherExecutionTraceContext(traceContext)
	defer engine.clearCurrentWatcherExecutionTraceContext()
	batchStartedAt := time.Now()
	defer engine.warnIfWatcherBatchProcessingExceededThreshold(
		batchStartedAt,
		traceContext,
		len(events),
	)

	executionPlanningResult := engine.buildEventExecutionPlan(
		events,
		watcher,
		builder,
	)
	executionInput := eventpipeline.BuildWatcherEventExecutionInputFromPlanningResult(
		executionPlanningResult,
	)
	flowDecision := executionInput.FlowDecision

	if flowDecision.TriggerConfigRestart {
		engine.broadcastRebuilding()
		engine.logInfo("configuration changed; scheduling config restart")
		engine.triggerConfigRestart()
		return
	}

	if len(executionInput.EventsWithHooks) == 0 {
		return
	}

	if flowDecision.BroadcastRebuildingOverlay {
		engine.broadcastRebuilding()
	}

	for _, logPayload := range executionInput.WatcherEventLogPayloads {
		engine.logInfo(
			"watch event",
			"operation",
			logPayload.Operation,
			"file",
			logPayload.FilePath,
			"cycle_id",
			traceContext.CycleID,
			"batch_id",
			traceContext.BatchID,
		)
	}
	if engine.isWaitingForBuildRetry() {
		engine.logInfo(
			"waiting for build retry; queuing restart from watcher batch",
			"cycle_id",
			traceContext.CycleID,
			"batch_id",
			traceContext.BatchID,
		)
		engine.triggerRestart()
		return
	}

	work := &eventpipeline.WorkSet{}
	engine.ExecuteEventExecutionPlan(
		executionInput.EventsWithHooks,
		flowDecision.BehavioralDecision,
		work,
		watcher,
	)

	watcher.RemoveStale()
}

// ExecuteEventExecutionPlan applies stop strategy and deterministic pipeline for a batch.
func (engine *Engine) ExecuteEventExecutionPlan(
	eventsWithHooks []eventpipeline.EventWithHooks,
	behavioralDecision eventpipeline.EventExecutionPlanBehavioralDecision,
	work *eventpipeline.WorkSet,
	watcher *watch.Watcher,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	eventsForExecution := hooks.DeriveEventsWithHooksForExecution(
		eventsWithHooks,
		behavioralDecision.AppStopStrategy,
	)
	if len(eventsForExecution) == 0 {
		return
	}

	engine.ExecuteAppStopStrategy(behavioralDecision.AppStopStrategy)
	engine.ProcessEventsWithDeterministicPipeline(
		behavioralDecision,
		work,
		watcher,
		eventsForExecution,
	)
}

// ProcessEventsWithDeterministicPipeline runs hooks/build/browser in fixed order.
func (engine *Engine) ProcessEventsWithDeterministicPipeline(
	behavioralDecision eventpipeline.EventExecutionPlanBehavioralDecision,
	work *eventpipeline.WorkSet,
	watcher *watch.Watcher,
	eventsWithHooks []eventpipeline.EventWithHooks,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	engine.FireNoWaitHooksForEvents(eventsWithHooks, watcher)

	preHookStageResult := hooks.RunAndApplyHookStageActionsAndErrorsToWorkSet(
		hooks.HookStageTypePre,
		func() ([]wave.RefreshAction, []error) {
			return engine.RunPreHooksForEventsWithErrors(
				eventsWithHooks,
				work,
				watcher,
			)
		},
		work,
	)
	if !engine.ContinuePipelineAfterHookStageOrTriggerRestart(
		preHookStageResult,
	) {
		return
	}

	implicitBuildDecision := hooks.DeriveImplicitBuildExecutionDecision(
		behavioralDecision.RunImplicitBuild,
		len(eventsWithHooks),
	)
	if !implicitBuildDecision.ShouldRunImplicitBuild {
		engine.logInfo(implicitBuildDecision.SkipImplicitBuildLogEntry)
	} else if engine.dependencies.Config != nil {
		work.Resolve(engine.dependencies.Config.UsingVite())
	}

	buildAndConcurrentContext, cancelBuildAndConcurrent := context.WithCancel(
		engine.currentRunCycleContextOrBackground(),
	)
	defer cancelBuildAndConcurrent()

	var buildAndConcurrentGroup errgroup.Group
	if implicitBuildDecision.ShouldRunImplicitBuild {
		buildAndConcurrentGroup.Go(func() error {
			buildPhaseError := engine.executeBuildPhase(work)
			if buildPhaseError != nil {
				cancelBuildAndConcurrent()
				return buildPhaseError
			}
			return nil
		})
	}

	var concurrentActions []wave.RefreshAction
	var concurrentHookExecutionErrors []error
	buildAndConcurrentGroup.Go(func() error {
		concurrentActions, concurrentHookExecutionErrors = engine.RunConcurrentHooksForEventsWithContextAndErrors(
			buildAndConcurrentContext,
			eventsWithHooks,
			watcher,
		)
		return nil
	})

	if buildAndConcurrentError := buildAndConcurrentGroup.Wait(); buildAndConcurrentError != nil {
		engine.logWarn(
			"stopping pipeline after build failure",
			"error",
			buildAndConcurrentError,
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
	if !engine.ContinuePipelineAfterHookStageOrTriggerRestart(
		concurrentHookStageResult,
	) {
		return
	}

	postHookStageResult := hooks.RunAndApplyHookStageActionsAndErrorsToWorkSet(
		hooks.HookStageTypePost,
		func() ([]wave.RefreshAction, []error) {
			return engine.RunPostHooksForEventsWithErrors(
				eventsWithHooks,
				watcher,
			)
		},
		work,
	)
	if !engine.ContinuePipelineAfterHookStageOrTriggerRestart(
		postHookStageResult,
	) {
		return
	}

	if hooks.ShouldStartAppAfterImplicitBuild(
		implicitBuildDecision.ShouldRunImplicitBuild,
		work.Restart,
	) {
		engine.logInfo("restarting app process")
		engine.startApp()
	}

	if hooks.ShouldExecuteBrowserPhaseAfterHookStageResults(
		preHookStageResult,
		concurrentHookStageResult,
		postHookStageResult,
	) {
		engine.executeBrowserPhase(work)
	}
}

// ExecuteAppStopStrategy applies stop strategy for current batch.
func (engine *Engine) ExecuteAppStopStrategy(
	appStopStrategy eventpipeline.AppStopStrategy,
) {
	switch appStopStrategy {
	case eventpipeline.AppStopStrategySingleEventHardReload:
		engine.logInfo("stopping app for hard reload")
		if stopError := engine.stopApp(); stopError != nil {
			engine.logError("failed to stop app", "error", stopError)
		}
	case eventpipeline.AppStopStrategyBatchHardReload:
		engine.logInfo("stopping app for batch rebuild")
		if stopError := engine.stopApp(); stopError != nil {
			engine.logError("failed to stop app", "error", stopError)
		}
	case eventpipeline.AppStopStrategyNone:
	}
}

// ContinuePipelineAfterHookStageOrTriggerRestart resolves continuation with configured policy.
func (engine *Engine) ContinuePipelineAfterHookStageOrTriggerRestart(
	hookStageResult hooks.HookStageResult,
) bool {
	return engine.ContinuePipelineAfterHookStageOrTriggerRestartWithFailurePolicy(
		hookStageResult,
		hooks.DeriveHookStageFailurePolicy(
			engine.configuredHookStageFailurePolicy(),
		),
	)
}

// ContinuePipelineAfterHookStageOrTriggerRestartWithFailurePolicy resolves continuation decision.
func (engine *Engine) ContinuePipelineAfterHookStageOrTriggerRestartWithFailurePolicy(
	hookStageResult hooks.HookStageResult,
	hookStageFailurePolicy hooks.HookStageFailurePolicy,
) bool {
	continuationDecision := hooks.DeriveHookStageContinuationDecisionWithFailurePolicy(
		hookStageResult,
		hookStageFailurePolicy,
	)
	if continuationDecision.ShouldContinue {
		return true
	}

	if continuationDecision.StopReason == hooks.HookStageContinuationStopReasonRestartRequested {
		engine.TriggerRestartFromRefreshActions(
			continuationDecision.RestartActionResult,
		)
	}
	if continuationDecision.StopReason == hooks.HookStageContinuationStopReasonStageFailure {
		traceContext := engine.currentWatcherExecutionTraceContext()
		engine.logWarn(
			"stopping pipeline after hook stage errors",
			"stage",
			hooks.DeriveHookStageLabel(hookStageResult.StageType),
			"error_count",
			len(hookStageResult.ExecutionErrors),
			"cycle_id",
			traceContext.CycleID,
			"batch_id",
			traceContext.BatchID,
		)
	}

	return false
}

// TriggerRestartFromRefreshActions triggers restart side effects from action result.
func (engine *Engine) TriggerRestartFromRefreshActions(
	actionResult eventpipeline.RefreshActionApplicationResult,
) {
	if actionResult.RecompileGo {
		engine.triggerRestart()
		return
	}
	engine.triggerRestartNoGo()
}

// RunSequentialHookStageForEligibleEventsWithErrors executes one stage in event order.
func (engine *Engine) RunSequentialHookStageForEligibleEventsWithErrors(
	eventsWithHooks []eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
	runHooksForEvent func(eventpipeline.EventWithHooks, *watch.Watcher) ([]wave.RefreshAction, error),
	hookExecutionFailureLogMessage string,
) ([]wave.RefreshAction, []error) {
	if runHooksForEvent == nil {
		return nil, nil
	}

	descriptors := hooks.DeriveHookStageExecutionDescriptors(eventsWithHooks)
	stageActions := make([]wave.RefreshAction, 0)
	stageExecutionErrors := make([]error, 0)
	traceContext := engine.currentWatcherExecutionTraceContext()

	for _, descriptor := range descriptors {
		actions, hookError := runHooksForEvent(
			descriptor.EventWithHooks,
			watcher,
		)
		if hookError != nil {
			engine.logError(
				hookExecutionFailureLogMessage,
				"error",
				hookError,
				"cycle_id",
				traceContext.CycleID,
				"batch_id",
				traceContext.BatchID,
			)
			stageExecutionErrors = append(stageExecutionErrors, hookError)
		}
		stageActions = append(stageActions, actions...)
	}

	return stageActions, stageExecutionErrors
}

// FireNoWaitHooksForEvents starts concurrent-no-wait hooks for all eligible events.
func (engine *Engine) FireNoWaitHooksForEvents(
	eventsWithHooks []eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) {
	descriptors := hooks.DeriveHookStageExecutionDescriptors(eventsWithHooks)
	for _, descriptor := range descriptors {
		engine.FireNoWaitHooks(descriptor.EventWithHooks, watcher)
	}
}

// RunPreHooksForEventsWithErrors executes pre hooks for all events.
func (engine *Engine) RunPreHooksForEventsWithErrors(
	eventsWithHooks []eventpipeline.EventWithHooks,
	work *eventpipeline.WorkSet,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, []error) {
	if work != nil {
		for _, eventWithHooks := range eventsWithHooks {
			if eventWithHooks.RunOnChangeOnly {
				continue
			}
			work.AddImplicitWork(eventWithHooks.Classified)
		}
	}
	return engine.RunSequentialHookStageForEligibleEventsWithErrors(
		eventsWithHooks,
		watcher,
		engine.RunPreHooks,
		"pre-hook execution failed",
	)
}

// RunConcurrentHooksForEventsWithContextAndErrors executes concurrent hooks and preserves event order.
func (engine *Engine) RunConcurrentHooksForEventsWithContextAndErrors(
	concurrentHookExecutionContext context.Context,
	eventsWithHooks []eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, []error) {
	descriptors := hooks.DeriveHookStageExecutionDescriptors(eventsWithHooks)
	traceContext := engine.currentWatcherExecutionTraceContext()
	actionsByDescriptorIndex := make([][]wave.RefreshAction, len(descriptors))
	executionErrorsByDescriptorIndex := make([]error, len(descriptors))
	var concurrentHooksGroup errgroup.Group

	for descriptorIndex := range descriptors {
		if !hooks.ShouldContinueConcurrentHookExecution(
			concurrentHookExecutionContext,
		) {
			break
		}

		descriptorForExecution := descriptors[descriptorIndex]
		descriptorIndexForResult := descriptorIndex
		concurrentHooksGroup.Go(func() error {
			if !hooks.ShouldContinueConcurrentHookExecution(
				concurrentHookExecutionContext,
			) {
				return nil
			}

			concurrentActions, hookError := engine.RunConcurrentHooksWithContext(
				concurrentHookExecutionContext,
				descriptorForExecution.EventWithHooks,
				watcher,
			)
			actionsByDescriptorIndex[descriptorIndexForResult] = concurrentActions
			if hookError != nil {
				executionErrorsByDescriptorIndex[descriptorIndexForResult] = hookError
				engine.logError(
					"concurrent hook execution failed",
					"error",
					hookError,
					"cycle_id",
					traceContext.CycleID,
					"batch_id",
					traceContext.BatchID,
				)
			}
			return nil
		})
	}

	_ = concurrentHooksGroup.Wait()

	allConcurrentActions := make([]wave.RefreshAction, 0)
	for _, actionsForDescriptor := range actionsByDescriptorIndex {
		allConcurrentActions = append(
			allConcurrentActions,
			actionsForDescriptor...)
	}

	concurrentHookExecutionErrors := make([]error, 0)
	for _, executionError := range executionErrorsByDescriptorIndex {
		if executionError != nil {
			concurrentHookExecutionErrors = append(
				concurrentHookExecutionErrors,
				executionError,
			)
		}
	}

	return allConcurrentActions, concurrentHookExecutionErrors
}

// RunPostHooksForEventsWithErrors executes post hooks for all events.
func (engine *Engine) RunPostHooksForEventsWithErrors(
	eventsWithHooks []eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, []error) {
	return engine.RunSequentialHookStageForEligibleEventsWithErrors(
		eventsWithHooks,
		watcher,
		engine.RunPostHooks,
		"post-hook execution failed",
	)
}

// FireNoWaitHooks executes detached concurrent-no-wait hook plans for one event.
func (engine *Engine) FireNoWaitHooks(
	eventWithHooks eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) {
	traceContext := engine.currentWatcherExecutionTraceContext()
	concurrentNoWaitLifecycleContext := engine.concurrentNoWaitHookLifecycleContext()

	executionPlans := hooks.DeriveHookExecutionPlansForEventStage(
		watcher,
		eventWithHooks,
		hooks.HookStageTypeConcurrentNoWait,
		engine.resolveHookExecutionPlan,
	)

	for _, executionPlan := range executionPlans {
		if executionPlan.Callback != nil {
			callbackForExecution := executionPlan.Callback
			pathForExecution := eventWithHooks.Classified.Event.Name
			callbackTimeout := engine.deriveHookCallbackTimeoutForExecutionPlan(
				hooks.HookStageTypeConcurrentNoWait,
				executionPlan,
			)
			engine.runNoWaitHookWithConcurrencyLimit(func() {
				hookCallbackContext, cancelHookCallbackContext := hooks.DeriveExecutionContextWithOptionalTimeout(
					concurrentNoWaitLifecycleContext,
					callbackTimeout,
				)
				if cancelHookCallbackContext != nil {
					defer cancelHookCallbackContext()
				}

				hookContextForExecution := hooks.CloneHookContextForExecution(
					eventWithHooks.HookCtx,
					hookCallbackContext,
				)
				if _, callbackError := hooks.ExecuteHookCallbackSafely(callbackForExecution, hookContextForExecution); callbackError != nil {
					engine.logWarn(
						"concurrent-no-wait callback failed",
						"path",
						pathForExecution,
						"error",
						callbackError,
						"cycle_id",
						traceContext.CycleID,
						"batch_id",
						traceContext.BatchID,
					)
				}
			})
		}

		if strings.TrimSpace(executionPlan.Command) != "" {
			commandForExecution := executionPlan.Command
			pathForExecution := eventWithHooks.Classified.Event.Name
			commandTimeout := engine.deriveHookCommandTimeoutForExecutionPlan(
				hooks.HookStageTypeConcurrentNoWait,
				executionPlan,
			)
			engine.runNoWaitHookWithConcurrencyLimit(func() {
				hookCommandContext, cancelHookCommandContext := hooks.DeriveExecutionContextWithOptionalTimeout(
					concurrentNoWaitLifecycleContext,
					commandTimeout,
				)
				if cancelHookCommandContext != nil {
					defer cancelHookCommandContext()
				}

				if commandError := hooks.ExecuteHookCommandWithContext(hookCommandContext, commandForExecution); commandError != nil {
					engine.logWarn(
						"concurrent-no-wait hook command failed",
						"path",
						pathForExecution,
						"command",
						commandForExecution,
						"error",
						commandError,
						"cycle_id",
						traceContext.CycleID,
						"batch_id",
						traceContext.BatchID,
					)
				}
			})
		}
	}
}

// RunPreHooks executes pre hook plans for one event.
func (engine *Engine) RunPreHooks(
	eventWithHooks eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction
	executionPlans := hooks.DeriveHookExecutionPlansForEventStage(
		watcher,
		eventWithHooks,
		hooks.HookStageTypePre,
		engine.resolveHookExecutionPlan,
	)

	for _, executionPlan := range executionPlans {
		action, executionError := engine.ExecuteHookExecutionPlanWithContext(
			engine.currentRunCycleContextOrBackground(),
			hooks.HookStageTypePre,
			executionPlan,
			eventWithHooks.HookCtx,
		)
		if action != nil {
			actions = append(actions, *action)
		}
		if executionError != nil {
			return actions, hooks.WrapHookExecutionErrorWithStageAndPath(
				hooks.HookStageTypePre,
				eventWithHooks.Classified.Event.Name,
				executionError,
			)
		}
	}
	return actions, nil
}

// RunConcurrentHooksWithContext executes concurrent hooks for one event.
func (engine *Engine) RunConcurrentHooksWithContext(
	concurrentHookExecutionContext context.Context,
	eventWithHooks eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, error) {
	executionPlans := hooks.DeriveHookExecutionPlansForEventStage(
		watcher,
		eventWithHooks,
		hooks.HookStageTypeConcurrent,
		engine.resolveHookExecutionPlan,
	)
	if len(executionPlans) == 0 {
		return nil, nil
	}

	actionsByHookIndex := make([]*wave.RefreshAction, len(executionPlans))
	executionErrorsByHookIndex := make([]error, len(executionPlans))
	var executionGroup errgroup.Group

	for hookIndex := range executionPlans {
		if !hooks.ShouldContinueConcurrentHookExecution(
			concurrentHookExecutionContext,
		) {
			break
		}

		executionPlan := executionPlans[hookIndex]
		hookIndexForResult := hookIndex
		executionGroup.Go(func() error {
			if !hooks.ShouldContinueConcurrentHookExecution(
				concurrentHookExecutionContext,
			) {
				return nil
			}

			action, executionError := engine.ExecuteHookExecutionPlanWithContext(
				concurrentHookExecutionContext,
				hooks.HookStageTypeConcurrent,
				executionPlan,
				eventWithHooks.HookCtx,
			)
			actionsByHookIndex[hookIndexForResult] = action
			if executionError != nil {
				executionErrorsByHookIndex[hookIndexForResult] = hooks.WrapHookExecutionErrorWithStageAndPath(
					hooks.HookStageTypeConcurrent,
					eventWithHooks.Classified.Event.Name,
					executionError,
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
		executionErrorsByHookIndex,
	)
}

// RunPostHooks executes post hook plans for one event.
func (engine *Engine) RunPostHooks(
	eventWithHooks eventpipeline.EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction
	executionPlans := hooks.DeriveHookExecutionPlansForEventStage(
		watcher,
		eventWithHooks,
		hooks.HookStageTypePost,
		engine.resolveHookExecutionPlan,
	)

	for _, executionPlan := range executionPlans {
		action, executionError := engine.ExecuteHookExecutionPlanWithContext(
			engine.currentRunCycleContextOrBackground(),
			hooks.HookStageTypePost,
			executionPlan,
			eventWithHooks.HookCtx,
		)
		if action != nil {
			actions = append(actions, *action)
		}
		if executionError != nil {
			return actions, hooks.WrapHookExecutionErrorWithStageAndPath(
				hooks.HookStageTypePost,
				eventWithHooks.Classified.Event.Name,
				executionError,
			)
		}
	}
	return actions, nil
}

// ExecuteHookExecutionPlanWithContext executes callback and command portions of one plan.
func (engine *Engine) ExecuteHookExecutionPlanWithContext(
	parentHookExecutionContext context.Context,
	stageType hooks.HookStageType,
	executionPlan hooks.HookExecutionPlan,
	hookContext *wave.HookContext,
) (*wave.RefreshAction, error) {
	var action *wave.RefreshAction
	if executionPlan.Callback != nil {
		hookCallbackContext, cancelHookCallbackContext := hooks.DeriveExecutionContextWithOptionalTimeout(
			parentHookExecutionContext,
			engine.deriveHookCallbackTimeoutForExecutionPlan(
				stageType,
				executionPlan,
			),
		)
		if cancelHookCallbackContext != nil {
			defer cancelHookCallbackContext()
		}

		hookContextForExecution := hooks.CloneHookContextForExecution(
			hookContext,
			hooks.DeriveHookExecutionContext(hookCallbackContext),
		)

		callbackAction, callbackError := hooks.ExecuteHookCallbackSafely(
			executionPlan.Callback,
			hookContextForExecution,
		)
		if callbackError != nil {
			return nil, callbackError
		}
		action = callbackAction
	}

	if strings.TrimSpace(executionPlan.Command) != "" {
		hookCommandContext, cancelHookCommandContext := hooks.DeriveExecutionContextWithOptionalTimeout(
			parentHookExecutionContext,
			engine.deriveHookCommandTimeoutForExecutionPlan(
				stageType,
				executionPlan,
			),
		)
		if cancelHookCommandContext != nil {
			defer cancelHookCommandContext()
		}

		if !hooks.ShouldContinueConcurrentHookExecution(hookCommandContext) {
			return action, hooks.DeriveHookExecutionContextError(
				hookCommandContext,
			)
		}

		if commandError := hooks.ExecuteHookCommandWithContext(hookCommandContext, executionPlan.Command); commandError != nil {
			return action, commandError
		}
	}

	return action, nil
}

// configuredHookStageFailurePolicy reads configured stage failure policy.
func (engine *Engine) configuredHookStageFailurePolicy() string {
	if engine == nil || engine.dependencies.Config == nil ||
		engine.dependencies.Config.Watch == nil {
		return ""
	}
	return engine.dependencies.Config.Watch.HookStageFailurePolicy
}

// resolveHookExecutionPlan resolves hook plan through dependency callback or default planner.
func (engine *Engine) resolveHookExecutionPlan(
	hook wave.OnChangeHook,
) hooks.HookExecutionPlan {
	if engine == nil || engine.dependencies.ResolveHookExecutionPlan == nil {
		return hooks.DeriveHookExecutionPlanFromHook(hook, nil)
	}
	return engine.dependencies.ResolveHookExecutionPlan(hook)
}

// deriveHookCommandTimeoutForExecutionPlan resolves command timeout from config + plan policy.
func (engine *Engine) deriveHookCommandTimeoutForExecutionPlan(
	stageType hooks.HookStageType,
	executionPlan hooks.HookExecutionPlan,
) time.Duration {
	if engine == nil || engine.dependencies.Config == nil {
		return hooks.DeriveHookCommandTimeoutDurationForExecutionPlan(
			nil,
			stageType,
			executionPlan,
		)
	}
	return hooks.DeriveHookCommandTimeoutDurationForExecutionPlan(
		engine.dependencies.Config.Watch,
		stageType,
		executionPlan,
	)
}

// deriveHookCallbackTimeoutForExecutionPlan resolves callback timeout from config + plan policy.
func (engine *Engine) deriveHookCallbackTimeoutForExecutionPlan(
	stageType hooks.HookStageType,
	executionPlan hooks.HookExecutionPlan,
) time.Duration {
	if engine == nil || engine.dependencies.Config == nil {
		return hooks.DeriveHookCallbackTimeoutDurationForExecutionPlan(
			nil,
			stageType,
			executionPlan,
		)
	}
	return hooks.DeriveHookCallbackTimeoutDurationForExecutionPlan(
		engine.dependencies.Config.Watch,
		stageType,
		executionPlan,
	)
}

// currentWatcher returns active watcher from dependency callback.
func (engine *Engine) currentWatcher() *watch.Watcher {
	if engine == nil || engine.dependencies.GetCurrentWatcher == nil {
		return nil
	}
	return engine.dependencies.GetCurrentWatcher()
}

// currentBuilder returns active builder from dependency callback.
func (engine *Engine) currentBuilder() *builder.Builder {
	if engine == nil || engine.dependencies.GetCurrentBuilder == nil {
		return nil
	}
	return engine.dependencies.GetCurrentBuilder()
}

// currentRunCycleContextOrBackground resolves cycle context fallback.
func (engine *Engine) currentRunCycleContextOrBackground() context.Context {
	if engine == nil ||
		engine.dependencies.CurrentRunCycleContextOrBackground == nil {
		return context.Background()
	}
	return engine.dependencies.CurrentRunCycleContextOrBackground()
}

// executeBuildPhase delegates build phase execution.
func (engine *Engine) executeBuildPhase(work *eventpipeline.WorkSet) error {
	if engine == nil || engine.dependencies.ExecuteBuildPhase == nil {
		return nil
	}
	return engine.dependencies.ExecuteBuildPhase(work)
}

// executeBrowserPhase delegates browser phase execution.
func (engine *Engine) executeBrowserPhase(work *eventpipeline.WorkSet) {
	if engine == nil || engine.dependencies.ExecuteBrowserPhase == nil {
		return
	}
	engine.dependencies.ExecuteBrowserPhase(work)
}

// startApp delegates app start callback.
func (engine *Engine) startApp() {
	if engine == nil || engine.dependencies.StartApp == nil {
		return
	}
	engine.dependencies.StartApp()
}

// stopApp delegates app stop callback.
func (engine *Engine) stopApp() error {
	if engine == nil || engine.dependencies.StopApp == nil {
		return nil
	}
	return engine.dependencies.StopApp()
}

// triggerRestart delegates go rebuild restart callback.
func (engine *Engine) triggerRestart() {
	if engine == nil || engine.dependencies.TriggerRestart == nil {
		return
	}
	engine.dependencies.TriggerRestart()
}

// triggerRestartNoGo delegates no-go restart callback.
func (engine *Engine) triggerRestartNoGo() {
	if engine == nil || engine.dependencies.TriggerRestartNoGo == nil {
		return
	}
	engine.dependencies.TriggerRestartNoGo()
}

// triggerConfigRestart delegates config restart callback.
func (engine *Engine) triggerConfigRestart() {
	if engine == nil || engine.dependencies.TriggerConfigRestart == nil {
		return
	}
	engine.dependencies.TriggerConfigRestart()
}

// broadcastRebuilding delegates rebuilding overlay callback.
func (engine *Engine) broadcastRebuilding() {
	if engine == nil || engine.dependencies.BroadcastRebuilding == nil {
		return
	}
	engine.dependencies.BroadcastRebuilding()
}

// buildEventExecutionPlan delegates planning callback.
func (engine *Engine) buildEventExecutionPlan(
	events []fsnotify.Event,
	watcher *watch.Watcher,
	builder *builder.Builder,
) eventpipeline.EventExecutionPlanningResult {
	if engine == nil || engine.dependencies.BuildEventExecutionPlan == nil {
		return eventpipeline.EventExecutionPlanningResult{}
	}
	return engine.dependencies.BuildEventExecutionPlan(events, watcher, builder)
}

// deriveWatcherExecutionTraceContext resolves trace context callback.
func (engine *Engine) deriveWatcherExecutionTraceContext() WatcherExecutionTraceContext {
	if engine == nil ||
		engine.dependencies.DeriveWatcherExecutionTraceContext == nil {
		return WatcherExecutionTraceContext{}
	}
	return engine.dependencies.DeriveWatcherExecutionTraceContext()
}

// isWaitingForBuildRetry reports whether lifecycle is currently in retry-wait state.
func (engine *Engine) isWaitingForBuildRetry() bool {
	if engine == nil || engine.dependencies.IsWaitingForBuildRetry == nil {
		return false
	}
	return engine.dependencies.IsWaitingForBuildRetry()
}

func (engine *Engine) watcherBatchDurationWarningThreshold() time.Duration {
	if engine == nil {
		return defaultWatcherBatchDurationWarningThreshold
	}
	if engine.dependencies.WatcherBatchDurationWarningThreshold > 0 {
		return engine.dependencies.WatcherBatchDurationWarningThreshold
	}
	return defaultWatcherBatchDurationWarningThreshold
}

func (engine *Engine) warnIfWatcherBatchProcessingExceededThreshold(
	startedAt time.Time,
	traceContext WatcherExecutionTraceContext,
	eventCount int,
) {
	threshold := engine.watcherBatchDurationWarningThreshold()
	if threshold <= 0 || eventCount <= 0 {
		return
	}

	elapsed := time.Since(startedAt)
	if elapsed < threshold {
		return
	}

	engine.logWarn(
		"watcher batch processing exceeded duration threshold",
		"duration",
		elapsed,
		"threshold",
		threshold,
		"event_count",
		eventCount,
		"cycle_id",
		traceContext.CycleID,
		"batch_id",
		traceContext.BatchID,
		"waiting_for_build_retry",
		engine.isWaitingForBuildRetry(),
	)
}

// setCurrentWatcherExecutionTraceContext delegates trace context set callback.
func (engine *Engine) setCurrentWatcherExecutionTraceContext(
	traceContext WatcherExecutionTraceContext,
) {
	if engine == nil ||
		engine.dependencies.SetCurrentWatcherExecutionTraceContext == nil {
		return
	}
	engine.dependencies.SetCurrentWatcherExecutionTraceContext(traceContext)
}

// clearCurrentWatcherExecutionTraceContext delegates trace context clear callback.
func (engine *Engine) clearCurrentWatcherExecutionTraceContext() {
	if engine == nil ||
		engine.dependencies.ClearCurrentWatcherExecutionTraceContext == nil {
		return
	}
	engine.dependencies.ClearCurrentWatcherExecutionTraceContext()
}

// currentWatcherExecutionTraceContext resolves current trace context callback.
func (engine *Engine) currentWatcherExecutionTraceContext() WatcherExecutionTraceContext {
	if engine == nil ||
		engine.dependencies.GetCurrentWatcherExecutionTraceContext == nil {
		return WatcherExecutionTraceContext{}
	}
	return engine.dependencies.GetCurrentWatcherExecutionTraceContext()
}

// runNoWaitHookWithConcurrencyLimit delegates concurrency-limited no-wait hook execution.
func (engine *Engine) runNoWaitHookWithConcurrencyLimit(runNoWaitHook func()) {
	if runNoWaitHook == nil {
		return
	}
	if engine == nil ||
		engine.dependencies.RunNoWaitHookWithConcurrencyLimit == nil {
		runNoWaitHook()
		return
	}
	engine.dependencies.RunNoWaitHookWithConcurrencyLimit(runNoWaitHook)
}

// concurrentNoWaitHookLifecycleContext resolves lifecycle context for no-wait hooks.
func (engine *Engine) concurrentNoWaitHookLifecycleContext() context.Context {
	if engine == nil ||
		engine.dependencies.GetOrCreateConcurrentNoWaitHookLifecycleContext == nil {
		return context.Background()
	}
	return engine.dependencies.GetOrCreateConcurrentNoWaitHookLifecycleContext()
}

// logInfo emits info log when logger is available.
func (engine *Engine) logInfo(message string, arguments ...any) {
	if engine == nil || engine.dependencies.Log == nil {
		return
	}
	engine.dependencies.Log.Info(message, arguments...)
}

// logWarn emits warn log when logger is available.
func (engine *Engine) logWarn(message string, arguments ...any) {
	if engine == nil || engine.dependencies.Log == nil {
		return
	}
	engine.dependencies.Log.Warn(message, arguments...)
}

// logError emits error log when logger is available.
func (engine *Engine) logError(message string, arguments ...any) {
	if engine == nil || engine.dependencies.Log == nil {
		return
	}
	engine.dependencies.Log.Error(message, arguments...)
}

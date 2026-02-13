package tooling

import (
	"github.com/vormadev/vorma/wave"
	"golang.org/x/sync/errgroup"
)

type implicitBuildExecutionDecision struct {
	shouldRunImplicitBuild    bool
	skipImplicitBuildLogEntry string
}

type hookStageResult struct {
	actions             []wave.RefreshAction
	refreshActionResult refreshActionApplicationResult
}

type hookStageContinuationDecision struct {
	shouldContinue      bool
	restartActionResult refreshActionApplicationResult
}

func deriveImplicitBuildExecutionDecision(
	shouldRunImplicitBuild bool,
	eventCount int,
) implicitBuildExecutionDecision {
	if shouldRunImplicitBuild {
		return implicitBuildExecutionDecision{
			shouldRunImplicitBuild: true,
		}
	}

	if eventCount == 1 {
		return implicitBuildExecutionDecision{
			skipImplicitBuildLogEntry: "RunOnChangeOnly: skipping implicit build phase",
		}
	}

	return implicitBuildExecutionDecision{
		skipImplicitBuildLogEntry: "All events are RunOnChangeOnly, skipping implicit build phase",
	}
}

func shouldShortCircuitPipelineForHookStageResult(
	hookStageResultForCheck hookStageResult,
) bool {
	return hookStageResultForCheck.refreshActionResult.restartRequested
}

func deriveHookStageContinuationDecision(
	hookStageResultForContinuation hookStageResult,
) hookStageContinuationDecision {
	if shouldShortCircuitPipelineForHookStageResult(
		hookStageResultForContinuation,
	) {
		return hookStageContinuationDecision{
			restartActionResult: hookStageResultForContinuation.refreshActionResult,
		}
	}
	return hookStageContinuationDecision{
		shouldContinue: true,
	}
}

func (s *server) continuePipelineAfterHookStageOrTriggerRestart(
	hookStageResultForContinuation hookStageResult,
) bool {
	continuationDecision := deriveHookStageContinuationDecision(
		hookStageResultForContinuation,
	)
	if continuationDecision.shouldContinue {
		return true
	}

	s.triggerRestartFromRefreshActions(continuationDecision.restartActionResult)
	return false
}

func applyHookStageActionsToWorkSet(
	hookStageActions []wave.RefreshAction,
	work *workSet,
) hookStageResult {
	hookStageResultForWork := hookStageResult{
		actions: append([]wave.RefreshAction(nil), hookStageActions...),
	}
	if work == nil {
		return hookStageResultForWork
	}

	hookStageResultForWork.refreshActionResult = work.applyRefreshActions(hookStageActions)
	return hookStageResultForWork
}

func runAndApplyHookStageActionsToWorkSet(
	runHookStageActions func() []wave.RefreshAction,
	work *workSet,
) hookStageResult {
	if runHookStageActions == nil {
		return applyHookStageActionsToWorkSet(nil, work)
	}
	return applyHookStageActionsToWorkSet(runHookStageActions(), work)
}

func shouldStartAppAfterImplicitBuild(
	shouldRunImplicitBuild bool,
	restart restartPhaseDecision,
) bool {
	return shouldRunImplicitBuild && restart.restartApp
}

func shouldExecuteBrowserPhaseAfterHookStageResults(
	hookStageResults ...hookStageResult,
) bool {
	for _, hookStageResultForCheck := range hookStageResults {
		if shouldShortCircuitPipelineForHookStageResult(hookStageResultForCheck) {
			return false
		}
	}
	return true
}

func deriveEventsWithHooksForExecution(
	eventsWithHooks []eventWithHooks,
	appStopStrategyForExecution appStopStrategy,
) []eventWithHooks {
	if len(eventsWithHooks) == 0 {
		return nil
	}
	if appStopStrategyForExecution != appStopStrategyBatchHardReload {
		return eventsWithHooks
	}

	executionEventsWithHooks := make([]eventWithHooks, len(eventsWithHooks))
	copy(executionEventsWithHooks, eventsWithHooks)
	for eventIndex := range executionEventsWithHooks {
		executionEventWithHooks := executionEventsWithHooks[eventIndex]
		if executionEventWithHooks.hookCtx == nil {
			continue
		}

		executionHookContext := *executionEventWithHooks.hookCtx
		executionHookContext.AppStoppedForBatch = true
		executionEventWithHooks.hookCtx = &executionHookContext
		executionEventsWithHooks[eventIndex] = executionEventWithHooks
	}

	return executionEventsWithHooks
}

func (s *server) executeEventExecutionPlan(
	eventsWithHooks []eventWithHooks,
	behavioralDecision eventExecutionPlanBehavioralDecision,
	work *workSet,
	watcher *Watcher,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	eventsWithHooksForExecution := deriveEventsWithHooksForExecution(
		eventsWithHooks,
		behavioralDecision.appStopStrategy,
	)

	switch behavioralDecision.appStopStrategy {
	case appStopStrategySingleEventHardReload:
		s.log.Info("Terminating running app")
		if err := s.stopApp(); err != nil {
			s.log.Error("Failed to terminate app", "error", err)
		}

	case appStopStrategyBatchHardReload:
		s.log.Info("Stopping app for batch rebuild")
		if err := s.stopApp(); err != nil {
			s.log.Error("Failed to stop app", "error", err)
		}

	case appStopStrategyNone:
	}

	s.processEventsWithDeterministicPipeline(
		behavioralDecision,
		work,
		watcher,
		eventsWithHooksForExecution,
	)
}

func (s *server) processEventsWithDeterministicPipeline(
	behavioralDecision eventExecutionPlanBehavioralDecision,
	work *workSet,
	watcher *Watcher,
	eventsWithHooks []eventWithHooks,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	s.fireNoWaitHooksForEvents(eventsWithHooks, watcher)

	preHookStageResult := runAndApplyHookStageActionsToWorkSet(
		func() []wave.RefreshAction {
			return s.runPreHooksForEvents(eventsWithHooks, work, watcher)
		},
		work,
	)
	if !s.continuePipelineAfterHookStageOrTriggerRestart(
		preHookStageResult,
	) {
		return
	}

	implicitBuildDecision := deriveImplicitBuildExecutionDecision(
		behavioralDecision.runImplicitBuild,
		len(eventsWithHooks),
	)
	if !implicitBuildDecision.shouldRunImplicitBuild {
		s.log.Info(implicitBuildDecision.skipImplicitBuildLogEntry)
	} else {
		work.resolve(s.cfg.UsingVite())
	}

	var buildAndConcurrentHooksGroup errgroup.Group
	if implicitBuildDecision.shouldRunImplicitBuild {
		buildAndConcurrentHooksGroup.Go(func() error {
			s.executeBuildPhase(work)
			return nil
		})
	}

	var concurrentActions []wave.RefreshAction
	buildAndConcurrentHooksGroup.Go(func() error {
		concurrentActions = s.runConcurrentHooksForEvents(eventsWithHooks, watcher)
		return nil
	})
	_ = buildAndConcurrentHooksGroup.Wait()

	concurrentHookStageResult := applyHookStageActionsToWorkSet(
		concurrentActions,
		work,
	)
	if !s.continuePipelineAfterHookStageOrTriggerRestart(
		concurrentHookStageResult,
	) {
		return
	}

	postHookStageResult := runAndApplyHookStageActionsToWorkSet(
		func() []wave.RefreshAction {
			return s.runPostHooksForEvents(eventsWithHooks, watcher)
		},
		work,
	)
	if !s.continuePipelineAfterHookStageOrTriggerRestart(
		postHookStageResult,
	) {
		return
	}

	if shouldStartAppAfterImplicitBuild(
		implicitBuildDecision.shouldRunImplicitBuild,
		work.restart,
	) {
		s.log.Info("Restarting app")
		s.startApp()
	}

	if shouldExecuteBrowserPhaseAfterHookStageResults(
		preHookStageResult,
		concurrentHookStageResult,
		postHookStageResult,
	) {
		s.executeBrowserPhase(work)
	}
}

func (s *server) triggerRestartFromRefreshActions(
	actionResult refreshActionApplicationResult,
) {
	if actionResult.recompileGo {
		s.triggerRestart()
		return
	}
	s.triggerRestartNoGo()
}

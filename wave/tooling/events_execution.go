package tooling

import (
	"context"

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
	stageType           hookStageType
	executionErrors     []error
}

type hookStageContinuationDecision struct {
	shouldContinue      bool
	stopReason          hookStageContinuationStopReason
	restartActionResult refreshActionApplicationResult
}

func (s *server) executeEventExecutionPlan(
	eventsWithHooks []eventWithHooks,
	behavioralDecision eventExecutionPlanBehavioralDecision,
	work *workSet,
	watcher *watcher,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	eventsWithHooksForExecution := deriveEventsWithHooksForExecution(
		eventsWithHooks,
		behavioralDecision.appStopStrategy,
	)
	if len(eventsWithHooksForExecution) == 0 {
		return
	}

	s.executeAppStopStrategy(behavioralDecision.appStopStrategy)
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
	watcher *watcher,
	eventsWithHooks []eventWithHooks,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	s.fireNoWaitHooksForEvents(eventsWithHooks, watcher)

	preHookStageResult := runAndApplyHookStageActionsAndErrorsToWorkSet(
		hookStageTypePre,
		func() ([]wave.RefreshAction, []error) {
			return s.runPreHooksForEventsWithErrors(
				eventsWithHooks,
				work,
				watcher,
			)
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

	buildAndConcurrentHooksContext, cancelBuildAndConcurrentHooks := context.WithCancel(
		s.currentRunCycleContextOrBackground(),
	)
	defer cancelBuildAndConcurrentHooks()

	var buildAndConcurrentHooksGroup errgroup.Group
	if implicitBuildDecision.shouldRunImplicitBuild {
		buildAndConcurrentHooksGroup.Go(func() error {
			buildPhaseError := s.executeBuildPhase(work)
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
		concurrentActions, concurrentHookExecutionErrors = s.runConcurrentHooksForEventsWithContextAndErrors(
			buildAndConcurrentHooksContext,
			eventsWithHooks,
			watcher,
		)
		return nil
	})
	buildAndConcurrentHooksError := buildAndConcurrentHooksGroup.Wait()
	if buildAndConcurrentHooksError != nil {
		s.log.Warn(
			"Stopping pipeline after build phase failure",
			"error",
			buildAndConcurrentHooksError,
		)
		return
	}

	concurrentHookStageResult := applyHookStageActionsAndErrorsToWorkSet(
		hookStageTypeConcurrent,
		concurrentActions,
		concurrentHookExecutionErrors,
		work,
	)
	if !s.continuePipelineAfterHookStageOrTriggerRestart(
		concurrentHookStageResult,
	) {
		return
	}

	postHookStageResult := runAndApplyHookStageActionsAndErrorsToWorkSet(
		hookStageTypePost,
		func() ([]wave.RefreshAction, []error) {
			return s.runPostHooksForEventsWithErrors(eventsWithHooks, watcher)
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

func (s *server) executeAppStopStrategy(
	appStopStrategyForExecution appStopStrategy,
) {
	switch appStopStrategyForExecution {
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
}

func (s *server) continuePipelineAfterHookStageOrTriggerRestart(
	hookStageResultForContinuation hookStageResult,
) bool {
	configuredHookStageFailurePolicy := ""
	if s != nil && s.cfg != nil && s.cfg.Watch != nil {
		configuredHookStageFailurePolicy = s.cfg.Watch.HookStageFailurePolicy
	}

	return s.continuePipelineAfterHookStageOrTriggerRestartWithFailurePolicy(
		hookStageResultForContinuation,
		deriveHookStageFailurePolicy(
			hookStageResultForContinuation.stageType,
			configuredHookStageFailurePolicy,
		),
	)
}

func (s *server) continuePipelineAfterHookStageOrTriggerRestartWithFailurePolicy(
	hookStageResultForContinuation hookStageResult,
	hookStageFailurePolicyForContinuation hookStageFailurePolicy,
) bool {
	continuationDecision := deriveHookStageContinuationDecisionWithFailurePolicy(
		hookStageResultForContinuation,
		hookStageFailurePolicyForContinuation,
	)
	if continuationDecision.shouldContinue {
		return true
	}

	if continuationDecision.stopReason == hookStageContinuationStopReasonRestartRequested {
		s.triggerRestartFromRefreshActions(continuationDecision.restartActionResult)
	}
	if continuationDecision.stopReason == hookStageContinuationStopReasonStageFailure {
		traceContextForContinuation := s.getCurrentWatcherExecutionTraceContext()
		s.log.Warn(
			"Stopping pipeline after hook stage errors",
			"stage",
			deriveHookStageLabel(hookStageResultForContinuation.stageType),
			"error_count",
			len(hookStageResultForContinuation.executionErrors),
			"cycle_id",
			traceContextForContinuation.cycleID,
			"batch_id",
			traceContextForContinuation.batchID,
		)
	}
	return false
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

func applyHookStageActionsAndErrorsToWorkSet(
	stageType hookStageType,
	hookStageActions []wave.RefreshAction,
	hookStageExecutionErrors []error,
	work *workSet,
) hookStageResult {
	hookStageResultForWork := applyHookStageActionsToWorkSet(
		hookStageActions,
		work,
	)
	hookStageResultForWork.stageType = stageType
	hookStageResultForWork.executionErrors = append(
		[]error(nil),
		hookStageExecutionErrors...,
	)
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

func runAndApplyHookStageActionsAndErrorsToWorkSet(
	stageType hookStageType,
	runHookStageActions func() ([]wave.RefreshAction, []error),
	work *workSet,
) hookStageResult {
	if runHookStageActions == nil {
		return applyHookStageActionsAndErrorsToWorkSet(
			stageType,
			nil,
			nil,
			work,
		)
	}
	hookStageActions, hookStageExecutionErrors := runHookStageActions()
	return applyHookStageActionsAndErrorsToWorkSet(
		stageType,
		hookStageActions,
		hookStageExecutionErrors,
		work,
	)
}

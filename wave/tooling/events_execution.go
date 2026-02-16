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
			return s.runPreHooksForEventsWithErrors(eventsWithHooks, work, watcher)
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

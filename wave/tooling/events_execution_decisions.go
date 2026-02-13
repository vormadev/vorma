package tooling

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

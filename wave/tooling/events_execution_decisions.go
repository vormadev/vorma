package tooling

import "strings"

type hookStageFailurePolicy int

const (
	hookStageFailurePolicyFailOpen hookStageFailurePolicy = iota
	hookStageFailurePolicyFailClosed
)

const (
	configuredHookStageFailurePolicyFailOpen   = "fail-open"
	configuredHookStageFailurePolicyFailClosed = "fail-closed"
)

type hookStageContinuationStopReason int

const (
	hookStageContinuationStopReasonNone hookStageContinuationStopReason = iota
	hookStageContinuationStopReasonRestartRequested
	hookStageContinuationStopReasonStageFailure
)

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
	return !deriveHookStageContinuationDecisionWithFailurePolicy(
		hookStageResultForCheck,
		hookStageFailurePolicyFailOpen,
	).shouldContinue
}

func deriveHookStageContinuationDecision(
	hookStageResultForContinuation hookStageResult,
) hookStageContinuationDecision {
	return deriveHookStageContinuationDecisionWithFailurePolicy(
		hookStageResultForContinuation,
		hookStageFailurePolicyFailOpen,
	)
}

func deriveHookStageContinuationDecisionWithFailurePolicy(
	hookStageResultForContinuation hookStageResult,
	hookStageFailurePolicyForStage hookStageFailurePolicy,
) hookStageContinuationDecision {
	if hookStageResultForContinuation.refreshActionResult.restartRequested {
		return hookStageContinuationDecision{
			stopReason:          hookStageContinuationStopReasonRestartRequested,
			restartActionResult: hookStageResultForContinuation.refreshActionResult,
		}
	}

	hasHookStageExecutionErrors := len(hookStageResultForContinuation.executionErrors) > 0
	if hasHookStageExecutionErrors &&
		hookStageFailurePolicyForStage == hookStageFailurePolicyFailClosed {
		return hookStageContinuationDecision{
			stopReason: hookStageContinuationStopReasonStageFailure,
		}
	}

	return hookStageContinuationDecision{
		shouldContinue: true,
		stopReason:     hookStageContinuationStopReasonNone,
	}
}

func deriveHookStageFailurePolicy(
	stageType hookStageType,
	configuredHookStageFailurePolicy string,
) hookStageFailurePolicy {
	resolvedHookStageFailurePolicy := deriveHookStageFailurePolicyFromConfiguredValue(
		configuredHookStageFailurePolicy,
	)

	switch stageType {
	case hookStageTypePre, hookStageTypeConcurrent, hookStageTypePost:
		return resolvedHookStageFailurePolicy
	default:
		return resolvedHookStageFailurePolicy
	}
}

func deriveHookStageFailurePolicyFromConfiguredValue(
	configuredHookStageFailurePolicy string,
) hookStageFailurePolicy {
	switch normalizeConfiguredHookStageFailurePolicy(configuredHookStageFailurePolicy) {
	case configuredHookStageFailurePolicyFailClosed:
		return hookStageFailurePolicyFailClosed
	case configuredHookStageFailurePolicyFailOpen, "":
		return hookStageFailurePolicyFailOpen
	default:
		return hookStageFailurePolicyFailOpen
	}
}

func normalizeConfiguredHookStageFailurePolicy(
	configuredHookStageFailurePolicy string,
) string {
	return strings.TrimSpace(strings.ToLower(configuredHookStageFailurePolicy))
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

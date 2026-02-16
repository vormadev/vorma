package tooling

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

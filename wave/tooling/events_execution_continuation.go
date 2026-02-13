package tooling

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

func (s *server) triggerRestartFromRefreshActions(
	actionResult refreshActionApplicationResult,
) {
	if actionResult.recompileGo {
		s.triggerRestart()
		return
	}
	s.triggerRestartNoGo()
}

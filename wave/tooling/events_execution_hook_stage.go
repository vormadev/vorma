package tooling

import "github.com/vormadev/vorma/wave"

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

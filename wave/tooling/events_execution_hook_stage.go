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

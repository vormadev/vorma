package tooling

import "github.com/vormadev/vorma/wave"

// addFromRefreshAction merges a RefreshAction from a callback into the work set.
func (work *workSet) addFromRefreshAction(action wave.RefreshAction) {
	workMutationDecision := deriveRefreshActionWorkMutationDecision(action)
	work.applyRefreshActionWorkMutationDecision(workMutationDecision)
}

func deriveRefreshActionWorkMutationDecision(
	action wave.RefreshAction,
) refreshActionWorkMutationDecision {
	workMutationDecision := refreshActionWorkMutationDecision{}
	if action.TriggerRestart {
		workMutationDecision.restartApp = true
		workMutationDecision.compileGo = action.RecompileGo
	}
	if action.ReloadBrowser {
		workMutationDecision.requestBrowserAction = true
		workMutationDecision.browserAction = browserPhaseActionHardReload
	}
	workMutationDecision.waitForApp = action.WaitForApp
	workMutationDecision.waitForVite = action.WaitForVite
	return workMutationDecision
}

func (work *workSet) applyRefreshActionWorkMutationDecision(
	workMutationDecision refreshActionWorkMutationDecision,
) {
	if workMutationDecision.restartApp {
		work.restart.restartApp = true
	}
	if workMutationDecision.compileGo {
		work.build.compileGo = true
	}
	if workMutationDecision.requestBrowserAction {
		work.requestBrowserAction(workMutationDecision.browserAction)
	}
	if workMutationDecision.waitForApp {
		work.browser.waitForApp = true
	}
	if workMutationDecision.waitForVite {
		work.browser.waitForVite = true
	}
}

func (work *workSet) applyRefreshActions(
	actions []wave.RefreshAction,
) refreshActionApplicationResult {
	reductionDecision := reduceRefreshActionsInStableOrder(actions)
	for _, action := range reductionDecision.actionsBeforeRestart {
		work.addFromRefreshAction(action)
	}

	return reductionDecision.applicationResult
}

func reduceRefreshActionsInStableOrder(
	actions []wave.RefreshAction,
) refreshActionReductionDecision {
	reductionDecision := refreshActionReductionDecision{
		actionsBeforeRestart: make([]wave.RefreshAction, 0, len(actions)),
		restartActionIndex:   -1,
	}

	for actionIndex, action := range actions {
		if action.TriggerRestart {
			reductionDecision.restartActionEncountered = true
			reductionDecision.restartActionIndex = actionIndex
			reductionDecision.applicationResult = refreshActionApplicationResult{
				restartRequested: true,
				recompileGo:      action.RecompileGo,
			}
			return reductionDecision
		}
		reductionDecision.actionsBeforeRestart = append(reductionDecision.actionsBeforeRestart, action)
	}

	return reductionDecision
}

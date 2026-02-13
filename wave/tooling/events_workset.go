package tooling

import (
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

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

func normalizeChangedSourceFilePathForWorkSet(
	filePath string,
) string {
	return pathnorm.Absolute(filePath)
}

func appendNormalizedFilePathIfMissing(
	existingFilePaths []string,
	existingFilePathSet map[string]struct{},
	filePath string,
) ([]string, map[string]struct{}) {
	normalizedFilePath := normalizeChangedSourceFilePathForWorkSet(filePath)
	if normalizedFilePath == "" {
		return existingFilePaths, existingFilePathSet
	}

	if existingFilePathSet == nil {
		existingFilePathSet = make(map[string]struct{}, len(existingFilePaths)+1)
		for _, existingFilePath := range existingFilePaths {
			existingFilePathSet[existingFilePath] = struct{}{}
		}
	}
	if _, alreadyExists := existingFilePathSet[normalizedFilePath]; alreadyExists {
		return existingFilePaths, existingFilePathSet
	}

	existingFilePaths = append(existingFilePaths, normalizedFilePath)
	existingFilePathSet[normalizedFilePath] = struct{}{}
	return existingFilePaths, existingFilePathSet
}

func (buildDecision *buildPhaseDecision) addPublicStaticChangedFilePath(
	filePath string,
) {
	buildDecision.publicStaticChangedFilePaths, buildDecision.publicStaticChangedFilePathSet = appendNormalizedFilePathIfMissing(
		buildDecision.publicStaticChangedFilePaths,
		buildDecision.publicStaticChangedFilePathSet,
		filePath,
	)
}

func (buildDecision *buildPhaseDecision) addPrivateStaticChangedFilePath(
	filePath string,
) {
	buildDecision.privateStaticChangedFilePaths, buildDecision.privateStaticChangedFilePathSet = appendNormalizedFilePathIfMissing(
		buildDecision.privateStaticChangedFilePaths,
		buildDecision.privateStaticChangedFilePathSet,
		filePath,
	)
}

// addImplicitWork adds build work implied by a file type.
func (work *workSet) addImplicitWork(classifiedEventForWork classifiedEvent) {
	implicitWorkDecisionForClassifiedEvent := deriveImplicitWorkDecisionForClassifiedEvent(classifiedEventForWork)
	work.applyImplicitWorkDecision(implicitWorkDecisionForClassifiedEvent)
}

func deriveImplicitWorkDecisionForClassifiedEvent(
	classifiedEventForWork classifiedEvent,
) implicitWorkDecision {
	watchedFileForWork := classifiedEventForWork.watchedFile
	if watchedFileForWork != nil && watchedFileForWork.RunOnChangeOnly {
		return implicitWorkDecision{}
	}

	decision := implicitWorkDecision{}
	if watchedFileForWork != nil && watchedFileForWork.OnlyRunClientDefinedRevalidateFunc {
		decision.preferRevalidate = true
	}

	switch classifiedEventForWork.fileType {
	case fileTypeGo:
		decision.compileGo = true
		decision.restartApp = true

	case fileTypeCriticalCSS:
		decision.buildCriticalCSS = true
		decision.restartApp = watchedFileForWork != nil && needsHardReload(watchedFileForWork)

	case fileTypeNormalCSS:
		decision.buildNormalCSS = true
		decision.restartApp = watchedFileForWork != nil && needsHardReload(watchedFileForWork)

	case fileTypeCriticalAndNormalCSS:
		decision.buildCriticalCSS = true
		decision.buildNormalCSS = true
		decision.restartApp = watchedFileForWork != nil && needsHardReload(watchedFileForWork)

	case fileTypePublicStatic:
		decision.processPublicFiles = true
		decision.publicStaticChangedFilePath = classifiedEventForWork.event.Name

	case fileTypePrivateStatic:
		decision.processPrivateFiles = true
		decision.privateStaticChangedFilePath = classifiedEventForWork.event.Name

	case fileTypeOther:
		if watchedFileForWork != nil {
			decision.compileGo = watchedFileForWork.RecompileGoBinary
			decision.restartApp = watchedFileForWork.RestartApp || watchedFileForWork.RecompileGoBinary
		}
	}

	return decision
}

func (work *workSet) applyImplicitWorkDecision(
	decision implicitWorkDecision,
) {
	if decision.preferRevalidate {
		work.preferRevalidate = true
	}
	if decision.compileGo {
		work.build.compileGo = true
	}
	if decision.buildCriticalCSS {
		work.build.buildCriticalCSS = true
	}
	if decision.buildNormalCSS {
		work.build.buildNormalCSS = true
	}
	if decision.processPublicFiles {
		work.build.processPublicFiles = true
		work.build.addPublicStaticChangedFilePath(decision.publicStaticChangedFilePath)
	}
	if decision.processPrivateFiles {
		work.build.processPrivateFiles = true
		work.build.addPrivateStaticChangedFilePath(decision.privateStaticChangedFilePath)
	}
	if decision.restartApp {
		work.restart.restartApp = true
	}
}

// resolve determines browser behavior based on build work and user preferences.
func (work *workSet) resolve(usingVite bool) {
	if work.build.compileGo {
		work.restart.restartApp = true
	}
	work.determineBrowserBehavior(usingVite)
}

func (work *workSet) determineBrowserBehavior(usingVite bool) {
	browserPhaseResolutionForWork := deriveBrowserPhaseResolutionForWorkSet(
		work.build,
		work.restart,
		work.preferRevalidate,
		usingVite,
	)
	work.requestBrowserAction(browserPhaseResolutionForWork.action)
	if browserPhaseResolutionForWork.applyWaitFlags {
		work.browser.waitForApp = browserPhaseResolutionForWork.waitForApp
		work.browser.waitForVite = browserPhaseResolutionForWork.waitForVite
	}
}

func (work *workSet) requestBrowserAction(action browserPhaseAction) {
	if action > work.browser.action {
		work.browser.action = action
	}
}

func deriveBrowserPhaseResolutionForWorkSet(
	buildDecision buildPhaseDecision,
	restartDecision restartPhaseDecision,
	preferRevalidate bool,
	usingVite bool,
) browserPhaseResolution {
	if shouldUseRestartBrowserResolution(restartDecision) {
		return browserPhaseResolution{
			action:         browserPhaseActionHardReload,
			applyWaitFlags: true,
			waitForApp:     true,
			waitForVite:    usingVite,
		}
	}

	if shouldUseRevalidateBrowserResolution(preferRevalidate) {
		return browserPhaseResolution{
			action:         browserPhaseActionRevalidate,
			applyWaitFlags: true,
			waitForApp:     true,
			waitForVite:    usingVite,
		}
	}

	if isCSSOnlyBuildWorkForBrowserPhase(buildDecision) {
		return browserPhaseResolution{
			action: browserPhaseActionHotReloadCSS,
		}
	}

	if buildDecision.processPublicFiles {
		return browserPhaseResolution{
			action: browserPhaseActionInvalidateVite,
		}
	}

	if buildDecision.processPrivateFiles || buildDecision.buildCriticalCSS || buildDecision.buildNormalCSS {
		return browserPhaseResolution{
			action:         browserPhaseActionHardReload,
			applyWaitFlags: true,
			waitForApp:     true,
			waitForVite:    usingVite,
		}
	}

	return browserPhaseResolution{
		action: browserPhaseActionNone,
	}
}

func shouldUseRestartBrowserResolution(
	restartDecision restartPhaseDecision,
) bool {
	return restartDecision.restartApp
}

func shouldUseRevalidateBrowserResolution(
	preferRevalidate bool,
) bool {
	// User preference takes precedence over automatic optimizations.
	return preferRevalidate
}

func isCSSOnlyBuildWorkForBrowserPhase(
	buildDecision buildPhaseDecision,
) bool {
	cssWork := buildDecision.buildCriticalCSS || buildDecision.buildNormalCSS
	return cssWork &&
		!buildDecision.processPublicFiles &&
		!buildDecision.processPrivateFiles
}

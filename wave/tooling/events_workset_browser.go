package tooling

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

package wavebuild

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// phase3BatchInput is the backend-settling input produced by phase 2.
type phase3BatchInput struct {
	batch                  phaseBatchInput
	phase2RequestedEffects phase2RequestedEffects
}

// phase3RequestedEffects are frontend-settling effects requested by phase 3.
type phase3RequestedEffects struct {
	terminalBrowserAction frontendTerminalBrowserAction
}

/////////////////////////////////////////////////////////////////////
/////// Phase Reductions
/////////////////////////////////////////////////////////////////////

func (phase2RequestedEffects phase2RequestedEffects) derivePhase3RequestedEffects() phase3RequestedEffects {
	if phase2RequestedEffects.queueRetryWaitRestart {
		return phase3RequestedEffects{
			terminalBrowserAction: frontendTerminalBrowserActionNone,
		}
	}

	terminalBrowserAction := phase2RequestedEffects.requestedTerminalBrowserAction
	if phase2RequestedEffects.restartViteProcess &&
		terminalBrowserAction == frontendTerminalBrowserActionNotifyVitePublicFileMapChanged {
		terminalBrowserAction = frontendTerminalBrowserActionNone
	}

	// Restarting Vite currently requires terminal hard reload so browser clients
	// reconnect against the active Vite endpoint.
	// Potential policy refinement: require this only when restart changes the
	// effective browser-facing Vite endpoint (for example, port change).
	if phase2RequestedEffects.restartDevServerCycle ||
		phase2RequestedEffects.restartAppProcess ||
		phase2RequestedEffects.restartViteProcess ||
		phase2RequestedEffects.refreshFrameworkRoute ||
		phase2RequestedEffects.refreshFrameworkTemplate ||
		phase2RequestedEffects.refreshFrameworkPublicFileMap {
		terminalBrowserAction = terminalBrowserAction.dominantWith(
			frontendTerminalBrowserActionHardReload,
		)
	}
	return phase3RequestedEffects{
		terminalBrowserAction: terminalBrowserAction,
	}
}

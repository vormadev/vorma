package wavebuild

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// p3_BatchInput is the backend-settling input produced by phase 2.
type p3_BatchInput struct {
	batch               phaseBatchInput
	p2_RequestedEffects p2_RequestedEffects
}

// p3_RequestedEffects are frontend-settling effects requested by phase 3.
type p3_RequestedEffects struct {
	terminalBrowserAction frontendTerminalBrowserAction
}

/////////////////////////////////////////////////////////////////////
/////// Phase Reductions
/////////////////////////////////////////////////////////////////////

func (p2_RequestedEffects p2_RequestedEffects) deriveP3_RequestedEffects() p3_RequestedEffects {
	if p2_RequestedEffects.queueRetryWaitRestart {
		return p3_RequestedEffects{
			terminalBrowserAction: frontendTerminalBrowserActionNone,
		}
	}

	terminalBrowserAction := p2_RequestedEffects.requestedTerminalBrowserAction
	if p2_RequestedEffects.restartViteProcess &&
		terminalBrowserAction == frontendTerminalBrowserActionNotifyVitePublicFileMapChanged {
		terminalBrowserAction = frontendTerminalBrowserActionNone
	}

	// Restarting Vite currently requires terminal hard reload so browser clients
	// reconnect against the active Vite endpoint.
	// Potential policy refinement: require this only when restart changes the
	// effective browser-facing Vite endpoint (for example, port change).
	if p2_RequestedEffects.restartDevServerCycle ||
		p2_RequestedEffects.restartAppProcess ||
		p2_RequestedEffects.restartViteProcess ||
		p2_RequestedEffects.refreshFrameworkRoute ||
		p2_RequestedEffects.refreshFrameworkTemplate ||
		p2_RequestedEffects.refreshFrameworkPublicFileMap {
		terminalBrowserAction = terminalBrowserAction.dominantWith(
			frontendTerminalBrowserActionHardReload,
		)
	}
	return p3_RequestedEffects{
		terminalBrowserAction: terminalBrowserAction,
	}
}

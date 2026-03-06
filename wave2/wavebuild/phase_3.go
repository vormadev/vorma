package wavebuild

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// p3_BatchInput is the backend-mutation input produced by phase 2.
type p3_BatchInput struct {
	batch               phaseBatchInput
	p2_RequestedEffects p2_RequestedEffects
}

// p4_RequestedEffects are backend-convergence effects requested by phase 3.
type p4_RequestedEffects struct {
	awaitBackendReadiness bool
	p5_RequestedEffects   p5_RequestedEffects
}

// p3_Output is the full phase-3 planner output consumed by phase 4.
type p3_Output struct {
	p4_RequestedEffects p4_RequestedEffects
}

/////////////////////////////////////////////////////////////////////
/////// Phase Reductions
/////////////////////////////////////////////////////////////////////

func (p2_RequestedEffects p2_RequestedEffects) deriveP4_RequestedEffects() p4_RequestedEffects {
	if p2_RequestedEffects.queueRetryWaitRestart {
		return p4_RequestedEffects{
			p5_RequestedEffects: p5_RequestedEffects{
				terminalBrowserAction: frontendTerminalBrowserActionNone,
			},
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
	return p4_RequestedEffects{
		awaitBackendReadiness: p2_RequestedEffects.awaitBackendReadiness,
		p5_RequestedEffects: p5_RequestedEffects{
			terminalBrowserAction: terminalBrowserAction,
		},
	}
}

package wavebuild

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// p2_BatchInput is the build-phase input produced by phase 1.
type p2_BatchInput struct {
	batch               phaseBatchInput
	p1_RequestedEffects p1_RequestedEffects
}

// p2_BuildOutcomeFacts are observable build outcome facts produced by phase 2.
//
// These facts are phase-2 outputs consumed by later phase planners; they are
// not direct watcher-event classifications.
type p2_BuildOutcomeFacts struct {
	publicFileMapArtifactsChanged  bool
	publicFileMapArtifactsRepaired bool
}

// p2_RequestedEffects are backend-mutation effects requested by phase 2.
type p2_RequestedEffects struct {
	restartDevServerCycle          bool
	restartAppProcess              bool
	restartViteProcess             bool
	refreshFrameworkRoute          bool
	refreshFrameworkTemplate       bool
	refreshFrameworkPublicFileMap  bool
	awaitBackendReadiness          bool
	queueRetryWaitRestart          bool
	requestedTerminalBrowserAction frontendTerminalBrowserAction
}

// p2_Output is the full phase-2 planner output consumed by phase 3.
type p2_Output struct {
	p2_RequestedEffects p2_RequestedEffects
	buildOutcomeFacts   p2_BuildOutcomeFacts
}

/////////////////////////////////////////////////////////////////////
/////// Phase Reductions
/////////////////////////////////////////////////////////////////////

func (leftFacts p2_BuildOutcomeFacts) merge(
	rightFacts p2_BuildOutcomeFacts,
) p2_BuildOutcomeFacts {
	return p2_BuildOutcomeFacts{
		publicFileMapArtifactsChanged: leftFacts.publicFileMapArtifactsChanged ||
			rightFacts.publicFileMapArtifactsChanged,
		publicFileMapArtifactsRepaired: leftFacts.publicFileMapArtifactsRepaired ||
			rightFacts.publicFileMapArtifactsRepaired,
	}
}

func (p1_RequestedEffects p1_RequestedEffects) deriveP2_RequestedEffects(
	buildOutcomeFacts p2_BuildOutcomeFacts,
) p2_RequestedEffects {
	if p1_RequestedEffects.queueRetryWaitRestart {
		return p2_RequestedEffects{
			queueRetryWaitRestart: true,
		}
	}

	requestedTerminalBrowserAction := p1_RequestedEffects.requestedTerminalBrowserAction
	if buildOutcomeFacts.publicFileMapArtifactsChanged ||
		buildOutcomeFacts.publicFileMapArtifactsRepaired {
		requestedTerminalBrowserAction = requestedTerminalBrowserAction.dominantWith(
			frontendTerminalBrowserActionNotifyVitePublicFileMapChanged,
		)
	}

	requestedEffects := p2_RequestedEffects{
		restartDevServerCycle: p1_RequestedEffects.restartDevServerCycle,
		restartAppProcess: p1_RequestedEffects.requestBackendRestart ||
			p1_RequestedEffects.compileGoBinary,
		restartViteProcess:       p1_RequestedEffects.requestViteRestart,
		refreshFrameworkRoute:    p1_RequestedEffects.requestFrameworkRouteRefresh,
		refreshFrameworkTemplate: p1_RequestedEffects.requestFrameworkTemplateRefresh,
		refreshFrameworkPublicFileMap: p1_RequestedEffects.requestFrameworkPublicFileMapRefresh ||
			buildOutcomeFacts.publicFileMapArtifactsChanged ||
			buildOutcomeFacts.publicFileMapArtifactsRepaired,
		requestedTerminalBrowserAction: requestedTerminalBrowserAction,
	}
	requestedEffects.awaitBackendReadiness =
		requestedEffects.restartDevServerCycle ||
			requestedEffects.restartAppProcess ||
			requestedEffects.restartViteProcess ||
			requestedEffects.refreshFrameworkRoute ||
			requestedEffects.refreshFrameworkTemplate ||
			requestedEffects.refreshFrameworkPublicFileMap
	return requestedEffects
}

func (leftRequestedEffects p2_RequestedEffects) merge(
	rightRequestedEffects p2_RequestedEffects,
) p2_RequestedEffects {
	return p2_RequestedEffects{
		restartDevServerCycle: leftRequestedEffects.restartDevServerCycle ||
			rightRequestedEffects.restartDevServerCycle,
		restartAppProcess: leftRequestedEffects.restartAppProcess ||
			rightRequestedEffects.restartAppProcess,
		restartViteProcess: leftRequestedEffects.restartViteProcess ||
			rightRequestedEffects.restartViteProcess,
		refreshFrameworkRoute: leftRequestedEffects.refreshFrameworkRoute ||
			rightRequestedEffects.refreshFrameworkRoute,
		refreshFrameworkTemplate: leftRequestedEffects.refreshFrameworkTemplate ||
			rightRequestedEffects.refreshFrameworkTemplate,
		refreshFrameworkPublicFileMap: leftRequestedEffects.refreshFrameworkPublicFileMap ||
			rightRequestedEffects.refreshFrameworkPublicFileMap,
		awaitBackendReadiness: leftRequestedEffects.awaitBackendReadiness ||
			rightRequestedEffects.awaitBackendReadiness,
		queueRetryWaitRestart: leftRequestedEffects.queueRetryWaitRestart ||
			rightRequestedEffects.queueRetryWaitRestart,
		requestedTerminalBrowserAction: leftRequestedEffects.requestedTerminalBrowserAction.dominantWith(
			rightRequestedEffects.requestedTerminalBrowserAction,
		),
	}
}

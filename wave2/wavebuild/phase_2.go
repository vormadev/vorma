package wavebuild

import (
	"github.com/vormadev/vorma/kit/tasks"
)

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// phase2BatchInput is the build-phase input produced by phase 1.
type phase2BatchInput struct {
	batch                  phaseBatchInput
	phase1RequestedEffects phase1RequestedEffects
}

// phase2BuildOutcomeFacts are observable build outcome facts produced by phase 2.
//
// These facts are phase-2 outputs consumed by later phase planners; they are
// not direct watcher-event classifications.
type phase2BuildOutcomeFacts struct {
	publicFileMapArtifactsChanged  bool
	publicFileMapArtifactsRepaired bool
}

// phase2RequestedEffects are backend-settling effects requested by phase 2.
type phase2RequestedEffects struct {
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

// phase2Output is the full phase-2 planner output consumed by phase 3.
type phase2Output struct {
	phase2RequestedEffects phase2RequestedEffects
	buildOutcomeFacts      phase2BuildOutcomeFacts
}

/////////////////////////////////////////////////////////////////////
/////// Shared Phase Tasks
/////////////////////////////////////////////////////////////////////

var noopPhase2EffectTask = tasks.NewTask(
	func(taskContext *tasks.Ctx, input phase2BatchInput,
	) (struct{}, error) {
		return struct{}{}, nil
	},
)

/////////////////////////////////////////////////////////////////////
/////// Phase Reductions
/////////////////////////////////////////////////////////////////////

func (leftFacts phase2BuildOutcomeFacts) merge(
	rightFacts phase2BuildOutcomeFacts,
) phase2BuildOutcomeFacts {
	return phase2BuildOutcomeFacts{
		publicFileMapArtifactsChanged: leftFacts.publicFileMapArtifactsChanged ||
			rightFacts.publicFileMapArtifactsChanged,
		publicFileMapArtifactsRepaired: leftFacts.publicFileMapArtifactsRepaired ||
			rightFacts.publicFileMapArtifactsRepaired,
	}
}

func (phase1RequestedEffects phase1RequestedEffects) derivePhase2RequestedEffects(
	buildOutcomeFacts phase2BuildOutcomeFacts,
) phase2RequestedEffects {
	if phase1RequestedEffects.queueRetryWaitRestart {
		return phase2RequestedEffects{
			queueRetryWaitRestart: true,
		}
	}

	requestedTerminalBrowserAction := phase1RequestedEffects.requestedTerminalBrowserAction
	if buildOutcomeFacts.publicFileMapArtifactsChanged ||
		buildOutcomeFacts.publicFileMapArtifactsRepaired {
		requestedTerminalBrowserAction = requestedTerminalBrowserAction.dominantWith(
			frontendTerminalBrowserActionNotifyVitePublicFileMapChanged,
		)
	}

	requestedEffects := phase2RequestedEffects{
		restartDevServerCycle: phase1RequestedEffects.restartDevServerCycle,
		restartAppProcess: phase1RequestedEffects.requestBackendRestart ||
			phase1RequestedEffects.compileGoBinary,
		restartViteProcess:       phase1RequestedEffects.requestViteRestart,
		refreshFrameworkRoute:    phase1RequestedEffects.requestFrameworkRouteRefresh,
		refreshFrameworkTemplate: phase1RequestedEffects.requestFrameworkTemplateRefresh,
		refreshFrameworkPublicFileMap: phase1RequestedEffects.requestFrameworkPublicFileMapRefresh ||
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

func (leftRequestedEffects phase2RequestedEffects) merge(
	rightRequestedEffects phase2RequestedEffects,
) phase2RequestedEffects {
	return phase2RequestedEffects{
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

package wavebuild

import (
	"slices"
)

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// phaseBatchInput is the shared per-batch phase envelope passed after phase 1.
type phaseBatchInput struct {
	mode                     mode
	generationID             string
	fwExecutionRegistrations *fwExecutionRegistrations
}

// p1_BatchInput is the phase-1 input contract for one reduced batch.
type p1_BatchInput struct {
	p1 *p1_Input
}

// p1_RequestedEffects are phase-2 build effects plus carry-forward settle intents.
type p1_RequestedEffects struct {
	restartDevServerCycle           bool
	compileGoBinary                 bool
	buildCriticalCSS                bool
	buildNormalCSS                  bool
	processPublicStaticAssets       bool
	cleanupStalePublicStaticOutputs bool
	processPrivateStaticAssets      bool
	generatePublicFileMap           bool
	runRequestedBuildEffects        bool
	queueRetryWaitRestart           bool

	requestBackendRestart                       bool
	requestViteRestart                          bool
	requestedTerminalBrowserAction              frontendTerminalBrowserAction
	wavePublicFileMapNotificationDestinationKey fwNotificationDestinationKey
	fwExecutionRegistrations                    *fwExecutionRegistrations
	fwRequestedEffects                          *fwRequestedEffects
}

/////////////////////////////////////////////////////////////////////
/////// Phase Reductions
/////////////////////////////////////////////////////////////////////

func (facts p1_Facts) hasEventType(
	eventType eventType,
) bool {
	return slices.Contains(facts.eventTypes, eventType)
}

func (facts p1_Facts) deriveImplicitBrowserActionIntent() frontendTerminalBrowserAction {
	actionIntent := frontendTerminalBrowserActionNone
	if facts.hasEventType(
		eventTypeCriticalCSSSourceChanged,
	) ||
		facts.hasEventType(
			eventTypeNormalCSSSourceChanged,
		) {
		actionIntent = actionIntent.dominantWith(
			frontendTerminalBrowserActionCSSHotReload,
		)
	}
	if facts.hasEventType(
		eventTypePublicStaticAssetChanged,
	) {
		actionIntent = actionIntent.dominantWith(
			frontendTerminalBrowserActionNotifyVitePublicFileMapChanged,
		)
	}
	if facts.hasEventType(
		eventTypeGoSourceChanged,
	) ||
		facts.hasEventType(
			eventTypePrivateStaticAssetChanged,
		) {
		actionIntent = actionIntent.dominantWith(
			frontendTerminalBrowserActionHardReload,
		)
	}
	return actionIntent
}

func (facts p1_Facts) deriveP1_RequestedEffects() p1_RequestedEffects {
	if facts.mode == modeDev &&
		facts.waitingForBuildRetry {
		return p1_RequestedEffects{
			queueRetryWaitRestart: true,
		}
	}
	configChanged := facts.hasEventType(
		eventTypeConfigFileChanged,
	)
	goSourceChanged := facts.hasEventType(
		eventTypeGoSourceChanged,
	)
	criticalCSSChanged := facts.hasEventType(
		eventTypeCriticalCSSSourceChanged,
	)
	normalCSSChanged := facts.hasEventType(
		eventTypeNormalCSSSourceChanged,
	)
	publicStaticChanged := facts.hasEventType(
		eventTypePublicStaticAssetChanged,
	)
	privateStaticChanged := facts.hasEventType(
		eventTypePrivateStaticAssetChanged,
	)
	appDefinedWatchActionOnlyChanged := facts.hasEventType(
		eventTypeAppDefinedWatchActionOnlyChanged,
	)
	appDefinedWatchWithRebuildChanged := facts.hasEventType(
		eventTypeAppDefinedWatchWithRebuildChanged,
	)
	appDefinedWatchChanged := appDefinedWatchActionOnlyChanged ||
		appDefinedWatchWithRebuildChanged

	requestedFWEffects := facts.fwRequestedEffects
	appRequestedOutcomes := appRequestedOutcomes{}
	if appDefinedWatchChanged || requestedFWEffects.hasAny() {
		appRequestedOutcomes = facts.appRequestedOutcomes
	}
	requestedFWEffects = requestedFWEffects.merge(
		appRequestedOutcomes.fwRequestedEffects,
	)
	mergedBrowserActionIntent := facts.deriveImplicitBrowserActionIntent().
		dominantWith(
			appRequestedOutcomes.requestedTerminalBrowserAction,
		)

	requestBackendRestart := goSourceChanged ||
		appRequestedOutcomes.requestRestart
	requestViteRestart := configChanged

	if facts.mode == modeProd {
		requestBackendRestart = false
		requestViteRestart = false
		mergedBrowserActionIntent = frontendTerminalBrowserActionNone
		requestedFWEffects = fwRequestedEffects{}
	}

	requestedEffects := p1_RequestedEffects{
		restartDevServerCycle: configChanged,
		compileGoBinary: goSourceChanged ||
			appRequestedOutcomes.requestGoCompile,
		buildCriticalCSS:                criticalCSSChanged,
		buildNormalCSS:                  normalCSSChanged,
		processPublicStaticAssets:       publicStaticChanged,
		cleanupStalePublicStaticOutputs: publicStaticChanged,
		processPrivateStaticAssets:      privateStaticChanged,
		generatePublicFileMap:           publicStaticChanged,

		requestBackendRestart:                       requestBackendRestart,
		requestViteRestart:                          requestViteRestart,
		requestedTerminalBrowserAction:              mergedBrowserActionIntent,
		wavePublicFileMapNotificationDestinationKey: facts.wavePublicFileMapNotificationDestinationKey,
		fwExecutionRegistrations:                    facts.fwExecutionRegistrations,
		fwRequestedEffects: newFWRequestedEffectsPointerIfAny(
			requestedFWEffects,
		),
	}
	if facts.mode == modeProd {
		requestedEffects.restartDevServerCycle = false
	}

	requestedEffects.runRequestedBuildEffects =
		requestedEffects.compileGoBinary ||
			requestedEffects.buildCriticalCSS ||
			requestedEffects.buildNormalCSS ||
			requestedEffects.processPublicStaticAssets ||
			requestedEffects.cleanupStalePublicStaticOutputs ||
			requestedEffects.processPrivateStaticAssets ||
			requestedEffects.generatePublicFileMap

	return requestedEffects
}

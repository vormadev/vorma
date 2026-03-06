package wavebuild

import (
	"slices"
)

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// phaseBatchInput is the shared per-batch phase envelope passed after phase 1.
type phaseBatchInput struct {
	mode         mode
	generationID string
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
	validateBuildOutputs            bool
	queueRetryWaitRestart           bool

	requestBackendRestart                bool
	requestViteRestart                   bool
	requestFrameworkRouteRefresh         bool
	requestFrameworkTemplateRefresh      bool
	requestFrameworkPublicFileMapRefresh bool
	requestedTerminalBrowserAction       frontendTerminalBrowserAction
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
		) ||
		facts.hasEventType(
			eventTypeFrameworkRouteDefinitionChanged,
		) ||
		facts.hasEventType(
			eventTypeFrameworkTemplateChanged,
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
	frameworkRouteDefinitionChanged := facts.hasEventType(
		eventTypeFrameworkRouteDefinitionChanged,
	)
	frameworkTemplateChanged := facts.hasEventType(
		eventTypeFrameworkTemplateChanged,
	)
	appDefinedWatchActionOnlyChanged := facts.hasEventType(
		eventTypeAppDefinedWatchActionOnlyChanged,
	)
	appDefinedWatchWithRebuildChanged := facts.hasEventType(
		eventTypeAppDefinedWatchWithRebuildChanged,
	)
	appDefinedWatchChanged := appDefinedWatchActionOnlyChanged ||
		appDefinedWatchWithRebuildChanged
	appRequestedOutcomes := appRequestedOutcomes{}
	if appDefinedWatchChanged {
		appRequestedOutcomes = facts.appRequestedOutcomes
	}
	mergedBrowserActionIntent := facts.deriveImplicitBrowserActionIntent().
		dominantWith(
			appRequestedOutcomes.requestedTerminalBrowserAction,
		)

	requestBackendRestart := goSourceChanged ||
		appRequestedOutcomes.requestRestart
	requestViteRestart := configChanged
	requestFrameworkRouteRefresh :=
		frameworkRouteDefinitionChanged ||
			appRequestedOutcomes.requestFrameworkRefresh
	requestFrameworkTemplateRefresh := frameworkTemplateChanged
	requestFrameworkPublicFileMapRefresh := publicStaticChanged

	if facts.mode == modeProd {
		requestBackendRestart = false
		requestViteRestart = false
		requestFrameworkRouteRefresh = false
		requestFrameworkTemplateRefresh = false
		requestFrameworkPublicFileMapRefresh = false
		mergedBrowserActionIntent = frontendTerminalBrowserActionNone
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

		requestBackendRestart:                requestBackendRestart,
		requestViteRestart:                   requestViteRestart,
		requestFrameworkRouteRefresh:         requestFrameworkRouteRefresh,
		requestFrameworkTemplateRefresh:      requestFrameworkTemplateRefresh,
		requestFrameworkPublicFileMapRefresh: requestFrameworkPublicFileMapRefresh,
		requestedTerminalBrowserAction:       mergedBrowserActionIntent,
	}
	if facts.mode == modeProd {
		requestedEffects.restartDevServerCycle = false
	}

	requestedEffects.validateBuildOutputs =
		requestedEffects.compileGoBinary ||
			requestedEffects.buildCriticalCSS ||
			requestedEffects.buildNormalCSS ||
			requestedEffects.processPublicStaticAssets ||
			requestedEffects.cleanupStalePublicStaticOutputs ||
			requestedEffects.processPrivateStaticAssets ||
			requestedEffects.generatePublicFileMap

	return requestedEffects
}

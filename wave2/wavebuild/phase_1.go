package wavebuild

import (
	"errors"
	"slices"

	"github.com/vormadev/vorma/kit/tasks"
)

// phaseBatchInput is the shared per-batch phase envelope passed after phase 1.
type phaseBatchInput struct {
	mode         mode
	generationID string
	execution    *phaseExecutionScope
}

// phase1BatchInput is the phase-1 input contract for one reduced batch.
type phase1BatchInput struct {
	phase1    *phase1Input
	execution *phaseExecutionScope
}

// phase1BuildGoals are phase-2 build goals plus carry-forward settle intents.
type phase1BuildGoals struct {
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

func (facts phase1Facts) hasEventType(
	eventType eventType,
) bool {
	return slices.Contains(facts.eventTypes, eventType)
}

func (facts phase1Facts) deriveImplicitBrowserActionIntent() frontendTerminalBrowserAction {
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

func (facts phase1Facts) derivePhase1BuildGoals() phase1BuildGoals {
	if facts.mode == modeDev &&
		facts.waitingForBuildRetry {
		return phase1BuildGoals{
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

	phase1BuildGoals := phase1BuildGoals{
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
		phase1BuildGoals.restartDevServerCycle = false
	}

	phase1BuildGoals.validateBuildOutputs =
		phase1BuildGoals.compileGoBinary ||
			phase1BuildGoals.buildCriticalCSS ||
			phase1BuildGoals.buildNormalCSS ||
			phase1BuildGoals.processPublicStaticAssets ||
			phase1BuildGoals.cleanupStalePublicStaticOutputs ||
			phase1BuildGoals.processPrivateStaticAssets ||
			phase1BuildGoals.generatePublicFileMap

	return phase1BuildGoals
}

// phase1PlanBuildGoalsTask maps event facts to build-phase goals.
var phase1PlanBuildGoalsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase1BatchInput,
	) (phase1BuildGoals, error) {
		if input.phase1 == nil {
			return phase1BuildGoals{}, errors.New(
				"wavebuild: phase-1 input is required",
			)
		}
		phase1Facts, phase1FactsError := input.phase1.buildFacts()
		if phase1FactsError != nil {
			return phase1BuildGoals{}, phase1FactsError
		}
		return phase1Facts.derivePhase1BuildGoals(), nil
	},
)

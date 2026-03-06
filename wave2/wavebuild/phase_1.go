package wavebuild

import (
	"errors"

	"github.com/vormadev/vorma/kit/tasks"
)

// PhaseBatchInput is the shared per-batch phase envelope passed after phase 1.
type PhaseBatchInput struct {
	Mode         Mode
	GenerationID string
	Execution    *PhaseExecutionScope
}

// EventsPhaseBatchInput is the phase-1 input contract for one reduced batch.
type EventsPhaseBatchInput struct {
	Events    *EventsPhaseInput
	Execution *PhaseExecutionScope
}

// Phase1BuildGoals are phase-2 build goals plus carry-forward settle intents.
type Phase1BuildGoals struct {
	RestartDevServerCycle           bool
	CompileGoBinary                 bool
	BuildCriticalCSS                bool
	BuildNormalCSS                  bool
	ProcessPublicStaticAssets       bool
	CleanupStalePublicStaticOutputs bool
	ProcessPrivateStaticAssets      bool
	GeneratePublicFileMap           bool
	ValidateBuildOutputs            bool
	QueueRetryWaitRestart           bool

	RequestBackendRestart                bool
	RequestViteRestart                   bool
	RequestFrameworkRouteRefresh         bool
	RequestFrameworkTemplateRefresh      bool
	RequestFrameworkPublicFileMapRefresh bool

	RequestBrowserCSSHotReload                   bool
	RequestBrowserNotifyVitePublicFileMapChanged bool
	RequestBrowserRevalidate                     bool
	RequestBrowserHardReload                     bool
}

func eventsPhaseFactsContainsType(
	facts EventsPhaseFacts,
	eventType EventType,
) bool {
	for _, candidateType := range facts.EventTypes {
		if candidateType == eventType {
			return true
		}
	}
	return false
}

func deriveImplicitBrowserActionIntent(
	eventsPhaseFacts EventsPhaseFacts,
) FrontendTerminalBrowserAction {
	actionIntent := FrontendTerminalBrowserActionNone
	if eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypeCriticalCSSSourceChanged,
	) ||
		eventsPhaseFactsContainsType(
			eventsPhaseFacts,
			EventTypeNormalCSSSourceChanged,
		) {
		actionIntent = dominantFrontendTerminalBrowserAction(
			actionIntent,
			FrontendTerminalBrowserActionCSSHotReload,
		)
	}
	if eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypePublicStaticAssetChanged,
	) {
		actionIntent = dominantFrontendTerminalBrowserAction(
			actionIntent,
			FrontendTerminalBrowserActionNotifyVitePublicFileMapChanged,
		)
	}
	if eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypeGoSourceChanged,
	) ||
		eventsPhaseFactsContainsType(
			eventsPhaseFacts,
			EventTypePrivateStaticAssetChanged,
		) ||
		eventsPhaseFactsContainsType(
			eventsPhaseFacts,
			EventTypeFrameworkRouteDefinitionChanged,
		) ||
		eventsPhaseFactsContainsType(
			eventsPhaseFacts,
			EventTypeFrameworkTemplateChanged,
		) {
		actionIntent = dominantFrontendTerminalBrowserAction(
			actionIntent,
			FrontendTerminalBrowserActionHardReload,
		)
	}
	return actionIntent
}

func deriveAppRequestedBrowserActionIntent(
	eventsPhaseFacts EventsPhaseFacts,
) FrontendTerminalBrowserAction {
	appRequestedOutcomes := eventsPhaseFacts.AppRequestedOutcomes
	actionIntent := FrontendTerminalBrowserActionNone
	if appRequestedOutcomes.RequestBrowserRevalidate {
		actionIntent = dominantFrontendTerminalBrowserAction(
			actionIntent,
			FrontendTerminalBrowserActionRevalidate,
		)
	}
	if appRequestedOutcomes.RequestNotifyVitePublicFileMapChanged {
		actionIntent = dominantFrontendTerminalBrowserAction(
			actionIntent,
			FrontendTerminalBrowserActionNotifyVitePublicFileMapChanged,
		)
	}
	if appRequestedOutcomes.RequestBrowserHardReload {
		actionIntent = dominantFrontendTerminalBrowserAction(
			actionIntent,
			FrontendTerminalBrowserActionHardReload,
		)
	}
	return actionIntent
}

func reducePhase1BuildGoals(
	eventsPhaseFacts EventsPhaseFacts,
) Phase1BuildGoals {
	if eventsPhaseFacts.Mode == ModeDev &&
		eventsPhaseFacts.WaitingForBuildRetry {
		return Phase1BuildGoals{
			QueueRetryWaitRestart: true,
		}
	}
	configChanged := eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypeConfigFileChanged,
	)
	goSourceChanged := eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypeGoSourceChanged,
	)
	criticalCSSChanged := eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypeCriticalCSSSourceChanged,
	)
	normalCSSChanged := eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypeNormalCSSSourceChanged,
	)
	publicStaticChanged := eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypePublicStaticAssetChanged,
	)
	privateStaticChanged := eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypePrivateStaticAssetChanged,
	)
	frameworkRouteDefinitionChanged := eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypeFrameworkRouteDefinitionChanged,
	)
	frameworkTemplateChanged := eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypeFrameworkTemplateChanged,
	)
	appRequestedOutcomes := eventsPhaseFacts.AppRequestedOutcomes
	mergedBrowserActionIntent := dominantFrontendTerminalBrowserAction(
		deriveImplicitBrowserActionIntent(eventsPhaseFacts),
		deriveAppRequestedBrowserActionIntent(eventsPhaseFacts),
	)

	requestBackendRestart := goSourceChanged ||
		appRequestedOutcomes.RequestRestart
	requestViteRestart := configChanged
	requestFrameworkRouteRefresh :=
		frameworkRouteDefinitionChanged ||
			appRequestedOutcomes.RequestFrameworkRefresh
	requestFrameworkTemplateRefresh := frameworkTemplateChanged
	requestFrameworkPublicFileMapRefresh := publicStaticChanged
	requestBrowserCSSHotReload :=
		mergedBrowserActionIntent == FrontendTerminalBrowserActionCSSHotReload
	requestBrowserNotifyVitePublicFileMapChanged :=
		mergedBrowserActionIntent == FrontendTerminalBrowserActionNotifyVitePublicFileMapChanged
	requestBrowserRevalidate :=
		mergedBrowserActionIntent == FrontendTerminalBrowserActionRevalidate
	requestBrowserHardReload :=
		mergedBrowserActionIntent == FrontendTerminalBrowserActionHardReload

	if eventsPhaseFacts.Mode == ModeProd {
		requestBackendRestart = false
		requestViteRestart = false
		requestFrameworkRouteRefresh = false
		requestFrameworkTemplateRefresh = false
		requestFrameworkPublicFileMapRefresh = false
		requestBrowserCSSHotReload = false
		requestBrowserNotifyVitePublicFileMapChanged = false
		requestBrowserRevalidate = false
		requestBrowserHardReload = false
	}

	phase1BuildGoals := Phase1BuildGoals{
		RestartDevServerCycle: configChanged,
		CompileGoBinary: goSourceChanged ||
			appRequestedOutcomes.RequestGoCompile,
		BuildCriticalCSS:                criticalCSSChanged,
		BuildNormalCSS:                  normalCSSChanged,
		ProcessPublicStaticAssets:       publicStaticChanged,
		CleanupStalePublicStaticOutputs: publicStaticChanged,
		ProcessPrivateStaticAssets:      privateStaticChanged,
		GeneratePublicFileMap:           publicStaticChanged,

		RequestBackendRestart:                requestBackendRestart,
		RequestViteRestart:                   requestViteRestart,
		RequestFrameworkRouteRefresh:         requestFrameworkRouteRefresh,
		RequestFrameworkTemplateRefresh:      requestFrameworkTemplateRefresh,
		RequestFrameworkPublicFileMapRefresh: requestFrameworkPublicFileMapRefresh,

		RequestBrowserCSSHotReload:                   requestBrowserCSSHotReload,
		RequestBrowserNotifyVitePublicFileMapChanged: requestBrowserNotifyVitePublicFileMapChanged,
		RequestBrowserRevalidate:                     requestBrowserRevalidate,
		RequestBrowserHardReload:                     requestBrowserHardReload,
	}
	if eventsPhaseFacts.Mode == ModeProd {
		phase1BuildGoals.RestartDevServerCycle = false
	}

	phase1BuildGoals.ValidateBuildOutputs =
		phase1BuildGoals.CompileGoBinary ||
			phase1BuildGoals.BuildCriticalCSS ||
			phase1BuildGoals.BuildNormalCSS ||
			phase1BuildGoals.ProcessPublicStaticAssets ||
			phase1BuildGoals.CleanupStalePublicStaticOutputs ||
			phase1BuildGoals.ProcessPrivateStaticAssets ||
			phase1BuildGoals.GeneratePublicFileMap

	return phase1BuildGoals
}

// Phase1BuildEventsPhaseFactsTask reduces events batch input into phase-1 facts.
var Phase1BuildEventsPhaseFactsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input EventsPhaseBatchInput,
	) (EventsPhaseFacts, error) {
		if input.Events == nil {
			return EventsPhaseFacts{}, errors.New(
				"wavebuild: events phase input is required",
			)
		}
		return BuildEventsPhaseFacts(*input.Events)
	},
)

// Phase1PlanBuildGoalsTask maps event facts to build-phase goals.
var Phase1PlanBuildGoalsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input EventsPhaseBatchInput,
	) (Phase1BuildGoals, error) {
		eventsPhaseFacts, eventsPhaseFactsError := Phase1BuildEventsPhaseFactsTask.Run(
			taskContext,
			input,
		)
		if eventsPhaseFactsError != nil {
			return Phase1BuildGoals{}, eventsPhaseFactsError
		}
		return reducePhase1BuildGoals(eventsPhaseFacts), nil
	},
)

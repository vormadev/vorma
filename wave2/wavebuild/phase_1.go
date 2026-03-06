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

	RequestBrowserCSSHotReload           bool
	RequestBrowserInvalidatePublicAssets bool
	RequestBrowserRevalidate             bool
	RequestBrowserHardReload             bool
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

type phase1BrowserActionIntent string

const (
	phase1BrowserActionIntentNone         phase1BrowserActionIntent = "none"
	phase1BrowserActionIntentCSSHotReload phase1BrowserActionIntent = "css_hot_reload"
	phase1BrowserActionIntentRevalidate   phase1BrowserActionIntent = "revalidate"
	phase1BrowserActionIntentInvalidate   phase1BrowserActionIntent = "invalidate"
	phase1BrowserActionIntentHardReload   phase1BrowserActionIntent = "hard_reload"
)

func browserActionIntentPriority(
	actionIntent phase1BrowserActionIntent,
) int {
	switch actionIntent {
	case phase1BrowserActionIntentHardReload:
		return 4
	case phase1BrowserActionIntentInvalidate:
		return 3
	case phase1BrowserActionIntentRevalidate:
		return 2
	case phase1BrowserActionIntentCSSHotReload:
		return 1
	default:
		return 0
	}
}

func dominantBrowserActionIntent(
	leftActionIntent phase1BrowserActionIntent,
	rightActionIntent phase1BrowserActionIntent,
) phase1BrowserActionIntent {
	if browserActionIntentPriority(rightActionIntent) >
		browserActionIntentPriority(leftActionIntent) {
		return rightActionIntent
	}
	return leftActionIntent
}

func deriveImplicitBrowserActionIntent(
	eventsPhaseFacts EventsPhaseFacts,
) phase1BrowserActionIntent {
	actionIntent := phase1BrowserActionIntentNone
	if eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypeCriticalCSSSourceChanged,
	) ||
		eventsPhaseFactsContainsType(
			eventsPhaseFacts,
			EventTypeNormalCSSSourceChanged,
		) {
		actionIntent = dominantBrowserActionIntent(
			actionIntent,
			phase1BrowserActionIntentCSSHotReload,
		)
	}
	if eventsPhaseFactsContainsType(
		eventsPhaseFacts,
		EventTypePublicStaticAssetChanged,
	) {
		actionIntent = dominantBrowserActionIntent(
			actionIntent,
			phase1BrowserActionIntentInvalidate,
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
		actionIntent = dominantBrowserActionIntent(
			actionIntent,
			phase1BrowserActionIntentHardReload,
		)
	}
	return actionIntent
}

func deriveAppRequestedBrowserActionIntent(
	eventsPhaseFacts EventsPhaseFacts,
) phase1BrowserActionIntent {
	appRequestedOutcomes := eventsPhaseFacts.AppRequestedOutcomes
	actionIntent := phase1BrowserActionIntentNone
	if appRequestedOutcomes.RequestBrowserRevalidate {
		actionIntent = dominantBrowserActionIntent(
			actionIntent,
			phase1BrowserActionIntentRevalidate,
		)
	}
	if appRequestedOutcomes.RequestBrowserInvalidate {
		actionIntent = dominantBrowserActionIntent(
			actionIntent,
			phase1BrowserActionIntentInvalidate,
		)
	}
	if appRequestedOutcomes.RequestBrowserHardReload {
		actionIntent = dominantBrowserActionIntent(
			actionIntent,
			phase1BrowserActionIntentHardReload,
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
	mergedBrowserActionIntent := dominantBrowserActionIntent(
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
		mergedBrowserActionIntent == phase1BrowserActionIntentCSSHotReload
	requestBrowserInvalidatePublicAssets :=
		mergedBrowserActionIntent == phase1BrowserActionIntentInvalidate
	requestBrowserRevalidate :=
		mergedBrowserActionIntent == phase1BrowserActionIntentRevalidate
	requestBrowserHardReload :=
		mergedBrowserActionIntent == phase1BrowserActionIntentHardReload

	if eventsPhaseFacts.Mode == ModeProd {
		requestBackendRestart = false
		requestViteRestart = false
		requestFrameworkRouteRefresh = false
		requestFrameworkTemplateRefresh = false
		requestFrameworkPublicFileMapRefresh = false
		requestBrowserCSSHotReload = false
		requestBrowserInvalidatePublicAssets = false
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

		RequestBrowserCSSHotReload:           requestBrowserCSSHotReload,
		RequestBrowserInvalidatePublicAssets: requestBrowserInvalidatePublicAssets,
		RequestBrowserRevalidate:             requestBrowserRevalidate,
		RequestBrowserHardReload:             requestBrowserHardReload,
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

// RunPhase1TaskGraph executes phase-1 task roots and returns build goals.
func RunPhase1TaskGraph(
	taskContext *tasks.Ctx,
	input EventsPhaseBatchInput,
) (Phase1BuildGoals, error) {
	return Phase1PlanBuildGoalsTask.Run(taskContext, input)
}

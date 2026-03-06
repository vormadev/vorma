package wavebuild

import (
	"errors"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/tasks"
)

// PhaseTaskTraceRecorder records deterministic task execution for one batch run.
type PhaseTaskTraceRecorder struct {
	mu               sync.Mutex
	orderedTaskNames []string
}

// RecordTaskExecution appends one task name to the trace.
func (traceRecorder *PhaseTaskTraceRecorder) RecordTaskExecution(
	taskName string,
) {
	if traceRecorder == nil {
		return
	}
	traceRecorder.mu.Lock()
	defer traceRecorder.mu.Unlock()
	traceRecorder.orderedTaskNames = append(
		traceRecorder.orderedTaskNames,
		taskName,
	)
}

// SnapshotOrderedTaskNames returns an isolated copy of the trace entries.
func (traceRecorder *PhaseTaskTraceRecorder) SnapshotOrderedTaskNames() []string {
	if traceRecorder == nil {
		return nil
	}
	traceRecorder.mu.Lock()
	defer traceRecorder.mu.Unlock()
	if len(traceRecorder.orderedTaskNames) == 0 {
		return nil
	}
	cloned := make([]string, len(traceRecorder.orderedTaskNames))
	copy(cloned, traceRecorder.orderedTaskNames)
	return cloned
}

// PhaseBatchInput is the shared per-batch phase envelope passed after phase 1.
type PhaseBatchInput struct {
	Mode         Mode
	GenerationID string
	Trace        *PhaseTaskTraceRecorder
}

// EventsPhaseBatchInput is the phase-1 input contract for one reduced batch.
type EventsPhaseBatchInput struct {
	Events *EventsPhaseInput
	Trace  *PhaseTaskTraceRecorder
}

// Phase1EventFacts are event-phase derived facts used by later phases.
type Phase1EventFacts struct {
	ConfigChanged                   bool
	GoSourceChanged                 bool
	CriticalCSSChanged              bool
	NormalCSSChanged                bool
	PublicStaticChanged             bool
	PrivateStaticChanged            bool
	FrameworkRouteDefinitionChanged bool
	FrameworkTemplateChanged        bool
	AppRequestedFrameworkRefresh    bool
	AppRequestedBrowserInvalidate   bool
	AppRequestedBrowserRevalidate   bool
	AppRequestedBrowserHardReload   bool
	AppRequestedRestart             bool
	AppRequestedGoCompile           bool
	WaitingForBuildRetry            bool
	HasMeaningfulWork               bool
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

func recordPhaseTaskExecution(
	traceRecorder *PhaseTaskTraceRecorder,
	taskName string,
) {
	if traceRecorder == nil {
		return
	}
	traceRecorder.RecordTaskExecution(taskName)
}

func recordPhase1TaskExecution(
	input EventsPhaseBatchInput,
	taskName string,
) {
	recordPhaseTaskExecution(input.Trace, taskName)
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
	eventFacts Phase1EventFacts,
) phase1BrowserActionIntent {
	actionIntent := phase1BrowserActionIntentNone
	if eventFacts.CriticalCSSChanged || eventFacts.NormalCSSChanged {
		actionIntent = dominantBrowserActionIntent(
			actionIntent,
			phase1BrowserActionIntentCSSHotReload,
		)
	}
	if eventFacts.PublicStaticChanged {
		actionIntent = dominantBrowserActionIntent(
			actionIntent,
			phase1BrowserActionIntentInvalidate,
		)
	}
	if eventFacts.GoSourceChanged ||
		eventFacts.PrivateStaticChanged ||
		eventFacts.FrameworkRouteDefinitionChanged ||
		eventFacts.FrameworkTemplateChanged {
		actionIntent = dominantBrowserActionIntent(
			actionIntent,
			phase1BrowserActionIntentHardReload,
		)
	}
	return actionIntent
}

func deriveAppRequestedBrowserActionIntent(
	eventFacts Phase1EventFacts,
) phase1BrowserActionIntent {
	actionIntent := phase1BrowserActionIntentNone
	if eventFacts.AppRequestedBrowserRevalidate {
		actionIntent = dominantBrowserActionIntent(
			actionIntent,
			phase1BrowserActionIntentRevalidate,
		)
	}
	if eventFacts.AppRequestedBrowserInvalidate {
		actionIntent = dominantBrowserActionIntent(
			actionIntent,
			phase1BrowserActionIntentInvalidate,
		)
	}
	if eventFacts.AppRequestedBrowserHardReload {
		actionIntent = dominantBrowserActionIntent(
			actionIntent,
			phase1BrowserActionIntentHardReload,
		)
	}
	return actionIntent
}

func reducePhase1BuildGoals(eventFacts Phase1EventFacts) Phase1BuildGoals {
	if eventFacts.WaitingForBuildRetry {
		return Phase1BuildGoals{
			QueueRetryWaitRestart: true,
		}
	}
	mergedBrowserActionIntent := dominantBrowserActionIntent(
		deriveImplicitBrowserActionIntent(eventFacts),
		deriveAppRequestedBrowserActionIntent(eventFacts),
	)

	phase1BuildGoals := Phase1BuildGoals{
		RestartDevServerCycle:           eventFacts.ConfigChanged,
		CompileGoBinary:                 eventFacts.GoSourceChanged || eventFacts.AppRequestedGoCompile,
		BuildCriticalCSS:                eventFacts.CriticalCSSChanged,
		BuildNormalCSS:                  eventFacts.NormalCSSChanged,
		ProcessPublicStaticAssets:       eventFacts.PublicStaticChanged,
		CleanupStalePublicStaticOutputs: eventFacts.PublicStaticChanged,
		ProcessPrivateStaticAssets:      eventFacts.PrivateStaticChanged,
		GeneratePublicFileMap:           eventFacts.PublicStaticChanged,

		RequestBackendRestart:                eventFacts.GoSourceChanged || eventFacts.AppRequestedRestart,
		RequestViteRestart:                   eventFacts.ConfigChanged,
		RequestFrameworkRouteRefresh:         eventFacts.FrameworkRouteDefinitionChanged || eventFacts.AppRequestedFrameworkRefresh,
		RequestFrameworkTemplateRefresh:      eventFacts.FrameworkTemplateChanged,
		RequestFrameworkPublicFileMapRefresh: eventFacts.PublicStaticChanged,

		RequestBrowserCSSHotReload:           mergedBrowserActionIntent == phase1BrowserActionIntentCSSHotReload,
		RequestBrowserInvalidatePublicAssets: mergedBrowserActionIntent == phase1BrowserActionIntentInvalidate,
		RequestBrowserRevalidate:             mergedBrowserActionIntent == phase1BrowserActionIntentRevalidate,
		RequestBrowserHardReload:             mergedBrowserActionIntent == phase1BrowserActionIntentHardReload,
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

// Phase1CollectBatchEnvelopeTask captures stable batch identity input.
var Phase1CollectBatchEnvelopeTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input EventsPhaseBatchInput,
	) (PhaseBatchInput, error) {
		recordPhase1TaskExecution(input, "phase_1.collect_batch_envelope")
		if input.Events == nil {
			return PhaseBatchInput{}, errors.New("wavebuild: events phase input is required")
		}
		return PhaseBatchInput{
			Mode:         input.Events.Mode,
			GenerationID: strings.TrimSpace(input.Events.GenerationID),
			Trace:        input.Trace,
		}, nil
	},
)

// Phase1NormalizeWatcherEventsTask normalizes watcher event shape for planning.
var Phase1NormalizeWatcherEventsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input EventsPhaseBatchInput,
	) (EventsPhaseInput, error) {
		recordPhase1TaskExecution(input, "phase_1.normalize_watcher_events")
		batchEnvelope, normalizeError := Phase1CollectBatchEnvelopeTask.Run(
			taskContext,
			input,
		)
		if normalizeError != nil {
			return EventsPhaseInput{}, normalizeError
		}
		return EventsPhaseInput{
			Mode:         batchEnvelope.Mode,
			GenerationID: batchEnvelope.GenerationID,
			Events: append(
				[]ObservedBatchEvent(nil),
				input.Events.Events...,
			),
			AppRequestedOutcomes: input.Events.AppRequestedOutcomes,
			WaitingForBuildRetry: input.Events.WaitingForBuildRetry,
		}, nil
	},
)

// Phase1BuildEventsPhaseInputTask maps batch input into canonical events-phase input.
var Phase1BuildEventsPhaseInputTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input EventsPhaseBatchInput,
	) (EventsPhaseInput, error) {
		recordPhase1TaskExecution(input, "phase_1.build_events_phase_input")
		eventsPhaseInput, normalizeError := Phase1NormalizeWatcherEventsTask.Run(
			taskContext,
			input,
		)
		if normalizeError != nil {
			return EventsPhaseInput{}, normalizeError
		}
		return eventsPhaseInput, nil
	},
)

// Phase1BuildEventsPhaseFactsTask reduces canonical events input into phase-1 facts.
var Phase1BuildEventsPhaseFactsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input EventsPhaseBatchInput,
	) (EventsPhaseFacts, error) {
		recordPhase1TaskExecution(input, "phase_1.build_events_phase_facts")
		eventsPhaseInput, inputError := Phase1BuildEventsPhaseInputTask.Run(
			taskContext,
			input,
		)
		if inputError != nil {
			return EventsPhaseFacts{}, inputError
		}
		return BuildEventsPhaseFacts(eventsPhaseInput)
	},
)

// Phase1DeriveEventFactsTask derives canonical event facts for phase handoff.
var Phase1DeriveEventFactsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input EventsPhaseBatchInput,
	) (Phase1EventFacts, error) {
		recordPhase1TaskExecution(input, "phase_1.derive_event_facts")
		eventsPhaseFacts, eventsPhaseFactsError := Phase1BuildEventsPhaseFactsTask.Run(
			taskContext,
			input,
		)
		if eventsPhaseFactsError != nil {
			return Phase1EventFacts{}, eventsPhaseFactsError
		}

		eventFacts := Phase1EventFacts{
			ConfigChanged: eventsPhaseFactsContainsType(
				eventsPhaseFacts,
				EventTypeConfigFileChanged,
			),
			GoSourceChanged: eventsPhaseFactsContainsType(
				eventsPhaseFacts,
				EventTypeGoSourceChanged,
			),
			CriticalCSSChanged: eventsPhaseFactsContainsType(
				eventsPhaseFacts,
				EventTypeCriticalCSSSourceChanged,
			),
			NormalCSSChanged: eventsPhaseFactsContainsType(
				eventsPhaseFacts,
				EventTypeNormalCSSSourceChanged,
			),
			PublicStaticChanged: eventsPhaseFactsContainsType(
				eventsPhaseFacts,
				EventTypePublicStaticAssetChanged,
			),
			PrivateStaticChanged: eventsPhaseFactsContainsType(
				eventsPhaseFacts,
				EventTypePrivateStaticAssetChanged,
			),
			FrameworkRouteDefinitionChanged: eventsPhaseFactsContainsType(
				eventsPhaseFacts,
				EventTypeFrameworkRouteDefinitionChanged,
			),
			FrameworkTemplateChanged: eventsPhaseFactsContainsType(
				eventsPhaseFacts,
				EventTypeFrameworkTemplateChanged,
			),
			AppRequestedFrameworkRefresh:  eventsPhaseFacts.AppRequestedOutcomes.RequestFrameworkRefresh,
			AppRequestedBrowserInvalidate: eventsPhaseFacts.AppRequestedOutcomes.RequestBrowserInvalidate,
			AppRequestedBrowserRevalidate: eventsPhaseFacts.AppRequestedOutcomes.RequestBrowserRevalidate,
			AppRequestedBrowserHardReload: eventsPhaseFacts.AppRequestedOutcomes.RequestBrowserHardReload,
			AppRequestedRestart:           eventsPhaseFacts.AppRequestedOutcomes.RequestRestart,
			AppRequestedGoCompile:         eventsPhaseFacts.AppRequestedOutcomes.RequestGoCompile,
			WaitingForBuildRetry:          eventsPhaseFacts.WaitingForBuildRetry,
		}
		eventFacts.HasMeaningfulWork =
			!eventsPhaseFacts.HasNoActionableEvents ||
				eventFacts.ConfigChanged ||
				eventFacts.GoSourceChanged ||
				eventFacts.CriticalCSSChanged ||
				eventFacts.NormalCSSChanged ||
				eventFacts.PublicStaticChanged ||
				eventFacts.PrivateStaticChanged ||
				eventFacts.FrameworkRouteDefinitionChanged ||
				eventFacts.FrameworkTemplateChanged ||
				eventFacts.AppRequestedFrameworkRefresh ||
				eventFacts.AppRequestedBrowserInvalidate ||
				eventFacts.AppRequestedBrowserRevalidate ||
				eventFacts.AppRequestedBrowserHardReload ||
				eventFacts.AppRequestedRestart ||
				eventFacts.AppRequestedGoCompile ||
				eventFacts.WaitingForBuildRetry

		return eventFacts, nil
	},
)

// Phase1PlanBuildGoalsTask maps event facts to build-phase goals.
var Phase1PlanBuildGoalsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input EventsPhaseBatchInput,
	) (Phase1BuildGoals, error) {
		recordPhase1TaskExecution(input, "phase_1.plan_build_goals")
		eventFacts, eventFactsError := Phase1DeriveEventFactsTask.Run(
			taskContext,
			input,
		)
		if eventFactsError != nil {
			return Phase1BuildGoals{}, eventFactsError
		}
		return reducePhase1BuildGoals(eventFacts), nil
	},
)

// RunPhase1TaskGraph executes phase-1 task roots and returns build goals.
func RunPhase1TaskGraph(
	taskContext *tasks.Ctx,
	input EventsPhaseBatchInput,
) (Phase1BuildGoals, error) {
	return Phase1PlanBuildGoalsTask.Run(taskContext, input)
}

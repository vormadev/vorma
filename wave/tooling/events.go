package tooling

import (
	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/internal/watchereventdedup"
)

type fileType int

const (
	fileTypeOther fileType = iota
	fileTypeGo
	fileTypeCriticalCSS
	fileTypeNormalCSS
	fileTypeCriticalAndNormalCSS
	fileTypePublicStatic
	fileTypePrivateStatic
)

type classifiedEvent struct {
	event       fsnotify.Event
	fileType    fileType
	watchedFile *wave.WatchedFile
	ignored     bool
	chmodOnly   bool
}

// eventWithHooks pairs a classified event with its sorted hooks.
type eventWithHooks struct {
	classified         classifiedEvent
	hooks              *wave.SortedHooks
	hookCtx            *wave.HookContext
	runOnChangeOnly    bool
	needsHardReload    bool
	skipDuplicateHooks bool
}

type buildPhaseDecision struct {
	compileGo                       bool
	buildCriticalCSS                bool
	buildNormalCSS                  bool
	processPublicFiles              bool
	processPrivateFiles             bool
	publicStaticChangedFilePaths    []string
	privateStaticChangedFilePaths   []string
	publicStaticChangedFilePathSet  map[string]struct{}
	privateStaticChangedFilePathSet map[string]struct{}
}

type restartPhaseDecision struct {
	restartApp bool
}

type browserPhaseAction int

const (
	browserPhaseActionNone browserPhaseAction = iota
	browserPhaseActionHotReloadCSS
	browserPhaseActionRevalidate
	browserPhaseActionHardReload
	browserPhaseActionInvalidateVite
)

type browserPhaseDecision struct {
	action      browserPhaseAction
	waitForApp  bool
	waitForVite bool
	cycleVite   bool
}

type browserPhaseResolution struct {
	action         browserPhaseAction
	applyWaitFlags bool
	waitForApp     bool
	waitForVite    bool
}

// workSet collects per-phase execution decisions for a watcher cycle.
type workSet struct {
	build   buildPhaseDecision
	restart restartPhaseDecision
	browser browserPhaseDecision

	// User preference collected from watched files and applied during resolve.
	preferRevalidate bool
}

type appStopStrategy int

const (
	appStopStrategyNone appStopStrategy = iota
	appStopStrategySingleEventHardReload
	appStopStrategyBatchHardReload
)

type eventExecutionPlanningResult struct {
	eventsWithHooks []eventWithHooks
	configChanged   bool
}

type watcherEventFlowDecision struct {
	triggerConfigRestart       bool
	broadcastRebuildingOverlay bool
	behavioralDecision         eventExecutionPlanBehavioralDecision
}

type watcherEventExecutionInput struct {
	flowDecision            watcherEventFlowDecision
	eventsWithHooks         []eventWithHooks
	watcherEventLogPayloads []watcherEventLogPayload
}

type eventExecutionPlanBehavioralDecision struct {
	showRebuildingOverlay bool
	appStopStrategy       appStopStrategy
	runImplicitBuild      bool
}

type watcherEventLogPayload struct {
	operation string
	filePath  string
}

type refreshActionApplicationResult struct {
	restartRequested bool
	recompileGo      bool
}

type refreshActionWorkMutationDecision struct {
	restartApp           bool
	compileGo            bool
	requestBrowserAction bool
	browserAction        browserPhaseAction
	waitForApp           bool
	waitForVite          bool
}

type refreshActionReductionDecision struct {
	actionsBeforeRestart     []wave.RefreshAction
	restartActionEncountered bool
	restartActionIndex       int
	applicationResult        refreshActionApplicationResult
}

type implicitWorkDecision struct {
	compileGo                    bool
	buildCriticalCSS             bool
	buildNormalCSS               bool
	processPublicFiles           bool
	processPrivateFiles          bool
	restartApp                   bool
	preferRevalidate             bool
	publicStaticChangedFilePath  string
	privateStaticChangedFilePath string
}

func (s *server) buildEventExecutionPlan(
	events []fsnotify.Event,
	watcher *watcher,
	builder *Builder,
) eventExecutionPlanningResult {
	deduplicatedEvents := watchereventdedup.DeduplicateWatcherEventsByPath(events)
	classifiedEvents, configChanged := s.classifyWatcherEventsForProcessing(
		deduplicatedEvents,
		watcher,
		builder,
	)
	if configChanged {
		return eventExecutionPlanningResult{
			configChanged: true,
		}
	}

	if len(classifiedEvents) == 0 {
		return eventExecutionPlanningResult{}
	}

	eventsWithHooks := buildEventExecutionPlanFromClassifiedEvents(classifiedEvents)
	if len(eventsWithHooks) == 0 {
		return eventExecutionPlanningResult{}
	}

	return eventExecutionPlanningResult{
		eventsWithHooks: eventsWithHooks,
	}
}

func buildEventExecutionPlanFromClassifiedEvents(
	classifiedEvents []classifiedEvent,
) []eventWithHooks {
	if len(classifiedEvents) == 0 {
		return nil
	}

	eventsWithHooks := buildEventHooksForProcessing(classifiedEvents)
	if len(eventsWithHooks) == 0 {
		return nil
	}

	return eventsWithHooks
}

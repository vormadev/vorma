package tooling

import (
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
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

func (s *server) runWatcher() {
	s.mu.Lock()
	watcher := s.watcher
	s.mu.Unlock()

	if watcher == nil {
		return
	}

	debouncer := NewDebouncer(30*time.Millisecond, func(events []fsnotify.Event) {
		s.processEvents(events)
	})
	defer debouncer.Stop()

	for {
		select {
		case evt, ok := <-watcher.Events():
			if !ok {
				return
			}
			debouncer.Add(evt)
		case err := <-watcher.Errors():
			s.log.Error("Watcher error", "error", err)
		}
	}
}

func (s *server) processEvents(events []fsnotify.Event) {
	s.mu.Lock()
	watcher := s.watcher
	builder := s.builder
	s.mu.Unlock()

	if watcher == nil || builder == nil {
		return
	}

	executionPlanningResult := s.buildEventExecutionPlan(events, watcher, builder)
	watcherEventExecutionInputForPlanningResult := buildWatcherEventExecutionInputFromPlanningResult(
		executionPlanningResult,
	)
	watcherEventFlowDecisionForPlanningResult := watcherEventExecutionInputForPlanningResult.flowDecision
	if watcherEventFlowDecisionForPlanningResult.triggerConfigRestart {
		s.log.Info("Config changed, restarting")
		s.triggerConfigRestart()
		return
	}

	eventsWithHooksForExecution := watcherEventExecutionInputForPlanningResult.eventsWithHooks
	if len(eventsWithHooksForExecution) == 0 {
		return
	}

	if watcherEventFlowDecisionForPlanningResult.broadcastRebuildingOverlay {
		s.broadcastRebuilding()
	}

	work := &workSet{}
	for _, watcherEventLogPayloadForExecutionPlan := range watcherEventExecutionInputForPlanningResult.watcherEventLogPayloads {
		s.log.Info(
			"[watcher]",
			"op",
			watcherEventLogPayloadForExecutionPlan.operation,
			"file",
			watcherEventLogPayloadForExecutionPlan.filePath,
		)
	}

	s.executeEventExecutionPlan(
		eventsWithHooksForExecution,
		watcherEventFlowDecisionForPlanningResult.behavioralDecision,
		work,
		watcher,
	)

	watcher.RemoveStale()
}

func buildWatcherEventExecutionInputFromPlanningResult(
	executionPlanningResult eventExecutionPlanningResult,
) watcherEventExecutionInput {
	flowDecision := deriveWatcherEventFlowDecisionFromPlanningResult(
		executionPlanningResult,
	)
	executionInput := watcherEventExecutionInput{
		flowDecision: flowDecision,
	}
	if flowDecision.triggerConfigRestart || len(executionPlanningResult.eventsWithHooks) == 0 {
		return executionInput
	}
	executionInput.eventsWithHooks = executionPlanningResult.eventsWithHooks
	executionInput.watcherEventLogPayloads = buildWatcherEventLogPayloadsForEventsWithHooks(
		executionInput.eventsWithHooks,
	)
	return executionInput
}

func deriveWatcherEventFlowDecisionFromPlanningResult(
	executionPlanningResult eventExecutionPlanningResult,
) watcherEventFlowDecision {
	if executionPlanningResult.configChanged {
		return watcherEventFlowDecision{
			triggerConfigRestart: true,
		}
	}
	if len(executionPlanningResult.eventsWithHooks) == 0 {
		return watcherEventFlowDecision{}
	}
	behavioralDecisionForExecutionPlan := deriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		executionPlanningResult.eventsWithHooks,
	)

	return watcherEventFlowDecision{
		broadcastRebuildingOverlay: behavioralDecisionForExecutionPlan.showRebuildingOverlay,
		behavioralDecision:         behavioralDecisionForExecutionPlan,
	}
}

func (s *server) buildEventExecutionPlan(
	events []fsnotify.Event,
	watcher *Watcher,
	builder *Builder,
) eventExecutionPlanningResult {
	deduplicatedEvents := deduplicateWatcherEventsByPath(events)
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

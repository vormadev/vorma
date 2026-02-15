package tooling

import (
	"time"

	"github.com/fsnotify/fsnotify"
)

func (s *server) runWatcher() {
	s.mu.Lock()
	watcher := s.watcher
	s.mu.Unlock()

	if watcher == nil {
		return
	}

	debouncer := newDebouncer(30*time.Millisecond, func(events []fsnotify.Event) {
		s.processEvents(events)
	})
	defer debouncer.Stop()

	for {
		select {
		case watcherEvent, ok := <-watcher.Events():
			if !ok {
				return
			}
			debouncer.Add(watcherEvent)
		case watcherError, ok := <-watcher.Errors():
			if !ok {
				return
			}
			if watcherError != nil {
				s.log.Error("watcher error", "error", watcherError)
			}
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

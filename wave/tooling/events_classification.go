package tooling

import (
	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
	"github.com/vormadev/vorma/wave/tooling/internal/watchereventclassification"
)

// classifyWatcherEventsForProcessing applies pre-classification side effects,
// maps events into file categories, then drops ignored/chmod-only entries.
func (s *server) classifyWatcherEventsForProcessing(
	events []fsnotify.Event,
	watcher *watcher,
	builder *Builder,
) ([]classifiedEvent, bool) {
	if len(events) == 0 {
		return nil, false
	}

	eventClassificationProber := watchereventclassification.NewEventClassificationProber(
		s.isConfigFile,
	)
	preClassificationPlan := watchereventclassification.BuildPreClassificationPlanFromEvents(
		events,
		eventClassificationProber.ProbeIsConfigFile,
		eventClassificationProber.ProbeEventDirectoryStatus,
	)
	if preClassificationPlan.ConfigChanged {
		return nil, true
	}

	s.applyWatcherEventPreClassificationSideEffects(watcher, preClassificationPlan)
	classifiedEvents := s.classifyWatcherEventsFromPreClassificationPlan(
		preClassificationPlan,
		watcher,
		builder,
	)

	return filterClassifiedEventsForProcessingByPostClassificationDecision(
		classifiedEvents,
	), false
}

// applyWatcherEventPreClassificationSideEffects runs watcher mutations derived
// from pre-classification. All logging suppression decisions live in
// shouldLogWatcherAddDirectoryError so this loop stays strictly orchestration.
func (s *server) applyWatcherEventPreClassificationSideEffects(
	watcher *watcher,
	preClassificationPlan watchereventclassification.PreClassificationPlan,
) {
	for _, directoryPathToWatch := range preClassificationPlan.AddDirectoryWatchPaths {
		addDirectoryWatchError := watcher.AddDir(directoryPathToWatch)
		if watchereventclassification.ShouldLogAddDirectoryWatchError(addDirectoryWatchError) {
			s.log.Warn(
				"failed to add directory watch",
				"path",
				directoryPathToWatch,
				"error",
				addDirectoryWatchError,
			)
		}
	}
}

// classifyWatcherEventsFromPreClassificationPlan maps each event that survived
// pre-classification into a typed classifiedEvent.
func (s *server) classifyWatcherEventsFromPreClassificationPlan(
	preClassificationPlan watchereventclassification.PreClassificationPlan,
	watcher *watcher,
	builder *Builder,
) []classifiedEvent {
	if len(preClassificationPlan.EventsToClassify) == 0 {
		return nil
	}

	classifiedEvents := make([]classifiedEvent, 0, len(preClassificationPlan.EventsToClassify))
	for _, eventToClassify := range preClassificationPlan.EventsToClassify {
		classifiedEvents = append(
			classifiedEvents,
			s.classifyEventWithWatcherAndBuilder(eventToClassify, watcher, builder),
		)
	}
	return classifiedEvents
}

// isConfigFile checks whether a watcher path points at the active config file.
// It intentionally tolerates nil/empty state so early startup or teardown
// phases can classify events without panicking.
func (s *server) isConfigFile(path string) bool {
	if s == nil || s.cfg == nil || s.cfg.Core == nil {
		return false
	}

	configPath := s.cfg.Core.ConfigLocation
	if configPath == "" {
		return false
	}

	return pathnorm.PathsReferToSameLocation(path, configPath)
}

// filterClassifiedEventsForProcessingByPostClassificationDecision removes
// ignored and chmod-only classified events before planning/execution.
func filterClassifiedEventsForProcessingByPostClassificationDecision(
	classifiedEvents []classifiedEvent,
) []classifiedEvent {
	if len(classifiedEvents) == 0 {
		return nil
	}

	filteredClassifiedEvents := make([]classifiedEvent, 0, len(classifiedEvents))
	for _, classifiedEventForProcessing := range classifiedEvents {
		postClassificationDecision := watchereventclassification.DerivePostClassificationDecision(
			classifiedEventForProcessing.ignored,
			classifiedEventForProcessing.chmodOnly,
		)
		if !postClassificationDecision.IncludeClassifiedEvent {
			continue
		}
		filteredClassifiedEvents = append(
			filteredClassifiedEvents,
			classifiedEventForProcessing,
		)
	}
	return filteredClassifiedEvents
}

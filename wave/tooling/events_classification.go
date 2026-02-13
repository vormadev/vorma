package tooling

import (
	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

func (s *server) classifyWatcherEventsForProcessing(
	events []fsnotify.Event,
	watcher *Watcher,
	builder *Builder,
) ([]classifiedEvent, bool) {
	if len(events) == 0 {
		return nil, false
	}

	eventClassificationProber := newWatcherEventClassificationProber(s.isConfigFile)
	preClassificationPlan := buildWatcherEventPreClassificationPlanFromEvents(
		events,
		eventClassificationProber,
	)
	if preClassificationPlan.configChanged {
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

func (s *server) applyWatcherEventPreClassificationSideEffects(
	watcher *Watcher,
	preClassificationPlan watcherEventPreClassificationPlan,
) {
	for _, directoryPathToWatch := range preClassificationPlan.addDirectoryWatchPaths {
		addDirectoryWatchError := watcher.AddDir(directoryPathToWatch)
		if shouldLogWatcherAddDirectoryError(addDirectoryWatchError) {
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

func (s *server) classifyWatcherEventsFromPreClassificationPlan(
	preClassificationPlan watcherEventPreClassificationPlan,
	watcher *Watcher,
	builder *Builder,
) []classifiedEvent {
	if len(preClassificationPlan.eventsToClassify) == 0 {
		return nil
	}

	classifiedEvents := make([]classifiedEvent, 0, len(preClassificationPlan.eventsToClassify))
	for _, eventToClassify := range preClassificationPlan.eventsToClassify {
		classifiedEvents = append(
			classifiedEvents,
			s.classifyEventWithWatcherAndBuilder(eventToClassify, watcher, builder),
		)
	}
	return classifiedEvents
}

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

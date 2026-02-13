package tooling

import "github.com/fsnotify/fsnotify"

type watcherEventPreClassificationDecision struct {
	configChanged     bool
	addDirectoryWatch bool
	classifyEvent     bool
}

type watcherEventPreClassificationPlan struct {
	configChanged          bool
	addDirectoryWatchPaths []string
	eventsToClassify       []fsnotify.Event
}

type watcherEventPreClassificationStepResult struct {
	configChanged         bool
	addDirectoryWatchPath string
	classifyEvent         bool
}

func appendDirectoryWatchPathIfMissing(
	addDirectoryWatchPaths []string,
	addDirectoryWatchPathSet map[string]struct{},
	addDirectoryWatchPath string,
) ([]string, map[string]struct{}) {
	if addDirectoryWatchPath == "" {
		return addDirectoryWatchPaths, addDirectoryWatchPathSet
	}

	if addDirectoryWatchPathSet == nil {
		addDirectoryWatchPathSet = make(map[string]struct{}, len(addDirectoryWatchPaths)+1)
		for _, existingDirectoryWatchPath := range addDirectoryWatchPaths {
			addDirectoryWatchPathSet[existingDirectoryWatchPath] = struct{}{}
		}
	}
	if _, alreadyTracked := addDirectoryWatchPathSet[addDirectoryWatchPath]; alreadyTracked {
		return addDirectoryWatchPaths, addDirectoryWatchPathSet
	}

	addDirectoryWatchPaths = append(addDirectoryWatchPaths, addDirectoryWatchPath)
	addDirectoryWatchPathSet[addDirectoryWatchPath] = struct{}{}
	return addDirectoryWatchPaths, addDirectoryWatchPathSet
}

func buildWatcherEventPreClassificationPlanFromEvents(
	events []fsnotify.Event,
	eventClassificationProber *watcherEventClassificationProber,
) watcherEventPreClassificationPlan {
	if len(events) == 0 {
		return watcherEventPreClassificationPlan{}
	}

	addDirectoryWatchPaths := make([]string, 0, len(events))
	var addDirectoryWatchPathSet map[string]struct{}
	eventsToClassify := make([]fsnotify.Event, 0, len(events))
	for _, event := range events {
		preClassificationStepResult := deriveWatcherEventPreClassificationStepResult(
			event,
			eventClassificationProber,
		)
		if preClassificationStepResult.configChanged {
			return watcherEventPreClassificationPlan{
				configChanged: true,
			}
		}
		addDirectoryWatchPaths, addDirectoryWatchPathSet = appendDirectoryWatchPathIfMissing(
			addDirectoryWatchPaths,
			addDirectoryWatchPathSet,
			preClassificationStepResult.addDirectoryWatchPath,
		)
		if preClassificationStepResult.classifyEvent {
			eventsToClassify = append(eventsToClassify, event)
		}
	}

	return watcherEventPreClassificationPlan{
		addDirectoryWatchPaths: addDirectoryWatchPaths,
		eventsToClassify:       eventsToClassify,
	}
}

func deriveWatcherEventPreClassificationStepResult(
	event fsnotify.Event,
	eventClassificationProber *watcherEventClassificationProber,
) watcherEventPreClassificationStepResult {
	isConfigFile := eventClassificationProber.probeIsConfigFile(event.Name)
	if isConfigMutationEvent(event, isConfigFile) {
		return watcherEventPreClassificationStepResult{
			configChanged: true,
		}
	}

	directoryProbeResult := eventClassificationProber.probeEventDirectoryStatus(event.Name)
	preClassificationDecision := deriveWatcherEventPreClassificationDecisionForNonConfigEvent(
		event,
		directoryProbeResult.isDirectory,
		directoryProbeResult.statProbeSucceeded,
	)

	stepResult := watcherEventPreClassificationStepResult{
		classifyEvent: preClassificationDecision.classifyEvent,
	}
	if preClassificationDecision.addDirectoryWatch {
		stepResult.addDirectoryWatchPath = event.Name
	}
	return stepResult
}

func deriveWatcherEventPreClassificationDecision(
	event fsnotify.Event,
	isConfigFile bool,
	eventIsDirectory bool,
	eventPathStatProbeSucceeded bool,
) watcherEventPreClassificationDecision {
	if isConfigMutationEvent(event, isConfigFile) {
		return watcherEventPreClassificationDecision{
			configChanged: true,
		}
	}

	return deriveWatcherEventPreClassificationDecisionForNonConfigEvent(
		event,
		eventIsDirectory,
		eventPathStatProbeSucceeded,
	)
}

func deriveWatcherEventPreClassificationDecisionForNonConfigEvent(
	event fsnotify.Event,
	eventIsDirectory bool,
	eventPathStatProbeSucceeded bool,
) watcherEventPreClassificationDecision {
	if !eventPathStatProbeSucceeded && (event.Has(fsnotify.Create) || event.Has(fsnotify.Rename)) {
		return watcherEventPreClassificationDecision{
			addDirectoryWatch: true,
			classifyEvent:     true,
		}
	}

	if eventIsDirectory {
		return watcherEventPreClassificationDecision{
			addDirectoryWatch: event.Has(fsnotify.Create) || event.Has(fsnotify.Rename),
		}
	}

	return watcherEventPreClassificationDecision{
		classifyEvent: true,
	}
}

func isConfigMutationEvent(
	event fsnotify.Event,
	isConfigFile bool,
) bool {
	return isConfigFile &&
		(event.Has(fsnotify.Write) ||
			event.Has(fsnotify.Create) ||
			event.Has(fsnotify.Remove) ||
			event.Has(fsnotify.Rename))
}

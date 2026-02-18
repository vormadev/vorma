// Package watchereventclassification contains watcher-event classification
// decisions used by tooling event orchestration.
//
// Boundary:
// - This package owns pure decision logic for pre/post classification.
// - This package does not own watcher side effects, builder state, or logging.
package watchereventclassification

import "github.com/fsnotify/fsnotify"

// DirectoryProbeResult reports whether probing path metadata succeeded and
// whether the probed path is a directory.
type DirectoryProbeResult struct {
	StatProbeSucceeded bool
	IsDirectory        bool
}

// PreClassificationDecision is the immediate classification decision for one
// watcher event before event-type mapping.
type PreClassificationDecision struct {
	ConfigChanged     bool
	AddDirectoryWatch bool
	ClassifyEvent     bool
}

// PreClassificationPlan accumulates pre-classification actions across a batch
// of watcher events.
type PreClassificationPlan struct {
	ConfigChanged          bool
	AddDirectoryWatchPaths []string
	EventsToClassify       []fsnotify.Event
}

// PreClassificationStepResult captures per-event side effects and inclusion.
type PreClassificationStepResult struct {
	ConfigChanged         bool
	AddDirectoryWatchPath string
	ClassifyEvent         bool
}

// ProbeIsConfigFileFunc resolves whether a path is the active config file.
type ProbeIsConfigFileFunc func(string) bool

// ProbeEventDirectoryStatusFunc probes directory status for an event path.
type ProbeEventDirectoryStatusFunc func(string) DirectoryProbeResult

// BuildPreClassificationPlanFromEvents folds a watcher-event batch into a plan
// that can be applied before event-type mapping.
func BuildPreClassificationPlanFromEvents(
	events []fsnotify.Event,
	probeIsConfigFile ProbeIsConfigFileFunc,
	probeEventDirectoryStatus ProbeEventDirectoryStatusFunc,
) PreClassificationPlan {
	if len(events) == 0 {
		return PreClassificationPlan{}
	}

	addDirectoryWatchPaths := make([]string, 0, len(events))
	var addDirectoryWatchPathSet map[string]struct{}
	eventsToClassify := make([]fsnotify.Event, 0, len(events))
	for _, event := range events {
		preClassificationStepResult := DerivePreClassificationStepResult(
			event,
			probeIsConfigFile,
			probeEventDirectoryStatus,
		)
		if preClassificationStepResult.ConfigChanged {
			return PreClassificationPlan{
				ConfigChanged: true,
			}
		}

		addDirectoryWatchPaths, addDirectoryWatchPathSet = appendDirectoryWatchPathIfMissing(
			addDirectoryWatchPaths,
			addDirectoryWatchPathSet,
			preClassificationStepResult.AddDirectoryWatchPath,
		)
		if preClassificationStepResult.ClassifyEvent {
			eventsToClassify = append(eventsToClassify, event)
		}
	}

	return PreClassificationPlan{
		AddDirectoryWatchPaths: addDirectoryWatchPaths,
		EventsToClassify:       eventsToClassify,
	}
}

// DerivePreClassificationStepResult builds the side-effect and inclusion
// outcome for one watcher event.
func DerivePreClassificationStepResult(
	event fsnotify.Event,
	probeIsConfigFile ProbeIsConfigFileFunc,
	probeEventDirectoryStatus ProbeEventDirectoryStatusFunc,
) PreClassificationStepResult {
	isConfigFile := false
	if probeIsConfigFile != nil {
		isConfigFile = probeIsConfigFile(event.Name)
	}
	if IsConfigMutationEvent(event, isConfigFile) {
		return PreClassificationStepResult{
			ConfigChanged: true,
		}
	}

	directoryProbeResult := DirectoryProbeResult{}
	if probeEventDirectoryStatus != nil {
		directoryProbeResult = probeEventDirectoryStatus(event.Name)
	}

	preClassificationDecision := DerivePreClassificationDecisionForNonConfigEvent(
		event,
		directoryProbeResult.IsDirectory,
		directoryProbeResult.StatProbeSucceeded,
	)
	stepResult := PreClassificationStepResult{
		ClassifyEvent: preClassificationDecision.ClassifyEvent,
	}
	if preClassificationDecision.AddDirectoryWatch {
		stepResult.AddDirectoryWatchPath = event.Name
	}
	return stepResult
}

// DerivePreClassificationDecision resolves per-event pre-classification intent.
func DerivePreClassificationDecision(
	event fsnotify.Event,
	isConfigFile bool,
	eventIsDirectory bool,
	eventPathStatProbeSucceeded bool,
) PreClassificationDecision {
	if IsConfigMutationEvent(event, isConfigFile) {
		return PreClassificationDecision{
			ConfigChanged: true,
		}
	}

	return DerivePreClassificationDecisionForNonConfigEvent(
		event,
		eventIsDirectory,
		eventPathStatProbeSucceeded,
	)
}

// DerivePreClassificationDecisionForNonConfigEvent resolves per-event
// pre-classification intent for non-config events.
func DerivePreClassificationDecisionForNonConfigEvent(
	event fsnotify.Event,
	eventIsDirectory bool,
	eventPathStatProbeSucceeded bool,
) PreClassificationDecision {
	if !eventPathStatProbeSucceeded && (event.Has(fsnotify.Create) || event.Has(fsnotify.Rename)) {
		return PreClassificationDecision{
			AddDirectoryWatch: true,
			ClassifyEvent:     true,
		}
	}

	if eventIsDirectory {
		return PreClassificationDecision{
			AddDirectoryWatch: event.Has(fsnotify.Create) || event.Has(fsnotify.Rename),
		}
	}

	return PreClassificationDecision{
		ClassifyEvent: true,
	}
}

// IsConfigMutationEvent returns true for config create/write/remove/rename
// mutations that should trigger config restart flow.
func IsConfigMutationEvent(
	event fsnotify.Event,
	isConfigFile bool,
) bool {
	return isConfigFile &&
		(event.Has(fsnotify.Write) ||
			event.Has(fsnotify.Create) ||
			event.Has(fsnotify.Remove) ||
			event.Has(fsnotify.Rename))
}

// appendDirectoryWatchPathIfMissing preserves stable first-seen order while
// avoiding duplicate watch requests in one watcher batch.
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

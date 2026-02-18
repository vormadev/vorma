// Package watchereventclassification contains watcher-event classification
// decisions used by tooling event orchestration.
//
// Why this package exists:
// watcher intake has to answer the same questions consistently on every cycle:
// config restart vs normal processing, directory watch mutations, and event
// inclusion. Without a dedicated decision boundary, this logic gets duplicated
// across orchestration code, making behavior drift and regressions likely.
//
// Boundary:
// - This package owns pure decision logic for pre/post classification.
// - This package does not own watcher side effects, builder state, or logging.
package watchereventclassification

import (
	"errors"
	"io/fs"
	"os"
	"syscall"

	"github.com/fsnotify/fsnotify"
)

// DirectoryProbeResult reports whether probing path metadata succeeded and
// whether the probed path is a directory.
type DirectoryProbeResult struct {
	// StatProbeSucceeded reports whether path metadata was read successfully.
	StatProbeSucceeded bool
	// IsDirectory reports whether the probed path is a directory.
	IsDirectory bool
}

// PreClassificationDecision is the immediate classification decision for one
// watcher event before event-type mapping.
type PreClassificationDecision struct {
	// ConfigChanged requests config-reload flow.
	ConfigChanged bool
	// AddDirectoryWatch requests watcher AddDir side effects.
	AddDirectoryWatch bool
	// ClassifyEvent controls whether the event continues to type mapping.
	ClassifyEvent bool
}

// PreClassificationPlan accumulates pre-classification actions across a batch
// of watcher events.
type PreClassificationPlan struct {
	// ConfigChanged short-circuits normal processing for config reload flow.
	ConfigChanged bool
	// AddDirectoryWatchPaths lists unique directory paths to pass to AddDir.
	AddDirectoryWatchPaths []string
	// EventsToClassify includes events that should continue into type mapping.
	EventsToClassify []fsnotify.Event
}

// PreClassificationStepResult captures per-event side effects and inclusion.
type PreClassificationStepResult struct {
	// ConfigChanged short-circuits normal processing for config reload flow.
	ConfigChanged bool
	// AddDirectoryWatchPath requests one AddDir side effect when non-empty.
	AddDirectoryWatchPath string
	// ClassifyEvent controls whether this event continues to type mapping.
	ClassifyEvent bool
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

// PostClassificationDecision resolves whether a classified event should move
// forward into pipeline planning.
type PostClassificationDecision struct {
	// IncludeClassifiedEvent controls whether one classified event proceeds.
	IncludeClassifiedEvent bool
}

// DerivePostClassificationDecision resolves event inclusion after mapping.
func DerivePostClassificationDecision(
	classifiedEventIgnored bool,
	classifiedEventIsChmodOnly bool,
) PostClassificationDecision {
	if classifiedEventIgnored || classifiedEventIsChmodOnly {
		return PostClassificationDecision{}
	}

	return PostClassificationDecision{
		IncludeClassifiedEvent: true,
	}
}

// ShouldLogAddDirectoryWatchError suppresses expected watcher add-dir probe
// failures and preserves logging for actionable errors.
func ShouldLogAddDirectoryWatchError(
	addDirectoryWatchError error,
) bool {
	if addDirectoryWatchError == nil {
		return false
	}

	if os.IsNotExist(addDirectoryWatchError) || errors.Is(addDirectoryWatchError, fs.ErrNotExist) {
		return false
	}

	if errors.Is(addDirectoryWatchError, syscall.ENOTDIR) {
		return false
	}

	return true
}

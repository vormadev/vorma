package classification

import (
	"errors"
	"io/fs"
	"os"
	"syscall"

	"github.com/fsnotify/fsnotify"
)

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

// PathProbeSnapshot memoizes all probes for a single path within one batch.
type PathProbeSnapshot struct {
	// HasConfigFileProbe reports whether config-match probe has run.
	HasConfigFileProbe bool
	// IsConfigFile stores the cached config-match probe result.
	IsConfigFile bool
	// HasDirectoryProbe reports whether directory probe has run.
	HasDirectoryProbe bool
	// DirectoryProbeState stores the cached directory probe result.
	DirectoryProbeState DirectoryProbeResult
}

// EventClassificationProber caches config and directory probes so one watcher
// batch does not repeatedly probe the same path.
type EventClassificationProber struct {
	// PathProbeSnapshotByPath memoizes path probe state for one batch.
	PathProbeSnapshotByPath map[string]*PathProbeSnapshot
	// IsConfigFileFn probes whether one path is the active config file.
	IsConfigFileFn func(string) bool
	// StatPathFn probes filesystem metadata for one path.
	StatPathFn func(string) (os.FileInfo, error)
}

// NewEventClassificationProber builds a batch-scoped probe cache.
func NewEventClassificationProber(
	isConfigFileFn func(string) bool,
) *EventClassificationProber {
	return &EventClassificationProber{
		PathProbeSnapshotByPath: make(
			map[string]*PathProbeSnapshot,
		),
		IsConfigFileFn: isConfigFileFn,
		StatPathFn:     os.Stat,
	}
}

// resolvePathProbe returns the shared per-path snapshot used by config and
// directory probes.
func (prober *EventClassificationProber) resolvePathProbe(
	path string,
) *PathProbeSnapshot {
	if prober == nil {
		return nil
	}
	if prober.PathProbeSnapshotByPath == nil {
		prober.PathProbeSnapshotByPath = make(
			map[string]*PathProbeSnapshot,
		)
	}

	pathProbeSnapshot, hasCachedSnapshot := prober.PathProbeSnapshotByPath[path]
	if !hasCachedSnapshot || pathProbeSnapshot == nil {
		pathProbeSnapshot = &PathProbeSnapshot{}
		prober.PathProbeSnapshotByPath[path] = pathProbeSnapshot
	}
	return pathProbeSnapshot
}

// ProbeIsConfigFile memoizes config-path matching for one path.
func (prober *EventClassificationProber) ProbeIsConfigFile(
	path string,
) bool {
	if prober == nil {
		return false
	}
	pathProbeSnapshot := prober.resolvePathProbe(path)
	if pathProbeSnapshot == nil {
		return false
	}
	if pathProbeSnapshot.HasConfigFileProbe {
		return pathProbeSnapshot.IsConfigFile
	}

	resolvedIsConfigFile := false
	if prober.IsConfigFileFn != nil {
		resolvedIsConfigFile = prober.IsConfigFileFn(path)
	}

	pathProbeSnapshot.HasConfigFileProbe = true
	pathProbeSnapshot.IsConfigFile = resolvedIsConfigFile
	return resolvedIsConfigFile
}

// ProbeEventDirectoryStatus memoizes os.Stat-derived directory status for one
// path.
func (prober *EventClassificationProber) ProbeEventDirectoryStatus(
	path string,
) DirectoryProbeResult {
	if prober == nil {
		return DirectoryProbeResult{}
	}
	pathProbeSnapshot := prober.resolvePathProbe(path)
	if pathProbeSnapshot == nil {
		return DirectoryProbeResult{}
	}
	if pathProbeSnapshot.HasDirectoryProbe {
		return pathProbeSnapshot.DirectoryProbeState
	}

	isDirectory := false
	statProbeSucceeded := false
	if prober.StatPathFn != nil {
		pathInfo, pathStatError := prober.StatPathFn(path)
		statProbeSucceeded = pathStatError == nil && pathInfo != nil
		isDirectory = statProbeSucceeded && pathInfo.IsDir()
	}

	directoryProbeResult := DirectoryProbeResult{
		StatProbeSucceeded: statProbeSucceeded,
		IsDirectory:        isDirectory,
	}
	pathProbeSnapshot.HasDirectoryProbe = true
	pathProbeSnapshot.DirectoryProbeState = directoryProbeResult
	return directoryProbeResult
}

package tooling

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

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

	classifiedEvents := make([]classifiedEvent, 0, len(preClassificationPlan.eventsToClassify))
	for _, eventToClassify := range preClassificationPlan.eventsToClassify {
		classifiedEventForProcessing := s.classifyEventWithWatcherAndBuilder(
			eventToClassify,
			watcher,
			builder,
		)
		classifiedEvents = append(classifiedEvents, classifiedEventForProcessing)
	}

	return filterClassifiedEventsForProcessingByPostClassificationDecision(
		classifiedEvents,
	), false
}

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

type watcherEventDirectoryProbeResult struct {
	statProbeSucceeded bool
	isDirectory        bool
}

type watcherEventPathProbeSnapshot struct {
	hasConfigFileProbe  bool
	isConfigFile        bool
	hasDirectoryProbe   bool
	directoryProbeState watcherEventDirectoryProbeResult
}

type watcherEventClassificationProber struct {
	pathProbeSnapshotByPath map[string]*watcherEventPathProbeSnapshot
	isConfigFileFn          func(string) bool
	statPathFn              func(string) (os.FileInfo, error)
}

func newWatcherEventClassificationProber(
	isConfigFileFn func(string) bool,
) *watcherEventClassificationProber {
	return &watcherEventClassificationProber{
		pathProbeSnapshotByPath: make(map[string]*watcherEventPathProbeSnapshot),
		isConfigFileFn:          isConfigFileFn,
		statPathFn:              os.Stat,
	}
}

func (prober *watcherEventClassificationProber) resolvePathProbeSnapshot(
	path string,
) *watcherEventPathProbeSnapshot {
	if prober == nil {
		return nil
	}
	if prober.pathProbeSnapshotByPath == nil {
		prober.pathProbeSnapshotByPath = make(map[string]*watcherEventPathProbeSnapshot)
	}

	pathProbeSnapshot, hasCachedSnapshot := prober.pathProbeSnapshotByPath[path]
	if !hasCachedSnapshot || pathProbeSnapshot == nil {
		pathProbeSnapshot = &watcherEventPathProbeSnapshot{}
		prober.pathProbeSnapshotByPath[path] = pathProbeSnapshot
	}
	return pathProbeSnapshot
}

func (prober *watcherEventClassificationProber) probeIsConfigFile(
	path string,
) bool {
	if prober == nil {
		return false
	}
	pathProbeSnapshot := prober.resolvePathProbeSnapshot(path)
	if pathProbeSnapshot == nil {
		return false
	}
	if pathProbeSnapshot.hasConfigFileProbe {
		return pathProbeSnapshot.isConfigFile
	}

	resolvedIsConfigFile := false
	if prober.isConfigFileFn != nil {
		resolvedIsConfigFile = prober.isConfigFileFn(path)
	}

	pathProbeSnapshot.hasConfigFileProbe = true
	pathProbeSnapshot.isConfigFile = resolvedIsConfigFile
	return resolvedIsConfigFile
}

func (prober *watcherEventClassificationProber) probeEventDirectoryStatus(
	path string,
) watcherEventDirectoryProbeResult {
	if prober == nil {
		return watcherEventDirectoryProbeResult{}
	}
	pathProbeSnapshot := prober.resolvePathProbeSnapshot(path)
	if pathProbeSnapshot == nil {
		return watcherEventDirectoryProbeResult{}
	}
	if pathProbeSnapshot.hasDirectoryProbe {
		return pathProbeSnapshot.directoryProbeState
	}

	directoryProbeResult := watcherEventDirectoryProbeResult{}
	isDirectory := false
	statProbeSucceeded := false
	if prober.statPathFn != nil {
		pathInfo, pathStatError := prober.statPathFn(path)
		statProbeSucceeded = pathStatError == nil && pathInfo != nil
		isDirectory = statProbeSucceeded && pathInfo.IsDir()
	}

	directoryProbeResult = watcherEventDirectoryProbeResult{
		statProbeSucceeded: statProbeSucceeded,
		isDirectory:        isDirectory,
	}
	pathProbeSnapshot.hasDirectoryProbe = true
	pathProbeSnapshot.directoryProbeState = directoryProbeResult
	return directoryProbeResult
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

type watcherEventPostClassificationDecision struct {
	includeClassifiedEvent bool
}

func shouldLogWatcherAddDirectoryError(
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

func deriveWatcherEventPostClassificationDecision(
	classifiedEventForProcessing classifiedEvent,
) watcherEventPostClassificationDecision {
	if classifiedEventForProcessing.ignored || classifiedEventForProcessing.chmodOnly {
		return watcherEventPostClassificationDecision{}
	}

	return watcherEventPostClassificationDecision{
		includeClassifiedEvent: true,
	}
}

func filterClassifiedEventsForProcessingByPostClassificationDecision(
	classifiedEvents []classifiedEvent,
) []classifiedEvent {
	if len(classifiedEvents) == 0 {
		return nil
	}

	filteredClassifiedEvents := make([]classifiedEvent, 0, len(classifiedEvents))
	for _, classifiedEventForProcessing := range classifiedEvents {
		postClassificationDecision := deriveWatcherEventPostClassificationDecision(
			classifiedEventForProcessing,
		)
		if !postClassificationDecision.includeClassifiedEvent {
			continue
		}
		filteredClassifiedEvents = append(
			filteredClassifiedEvents,
			classifiedEventForProcessing,
		)
	}
	return filteredClassifiedEvents
}

func (s *server) classifyEventWithWatcherAndBuilder(
	watcherEvent fsnotify.Event,
	watcher *Watcher,
	builder *Builder,
) classifiedEvent {
	classifiedEventForProcessing := classifiedEvent{event: watcherEvent}

	if watcherEvent.Name == "" {
		classifiedEventForProcessing.ignored = true
		return classifiedEventForProcessing
	}

	classifiedEventForProcessing.ignored = watcher.IsIgnoredFile(watcherEvent.Name)

	isCriticalCSSFile := builder.IsCriticalCSSFile(watcherEvent.Name)
	isNormalCSSFile := builder.IsNormalCSSFile(watcherEvent.Name)

	if isCriticalCSSFile && isNormalCSSFile {
		classifiedEventForProcessing.fileType = fileTypeCriticalAndNormalCSS
	} else if isCriticalCSSFile {
		classifiedEventForProcessing.fileType = fileTypeCriticalCSS
	} else if isNormalCSSFile {
		classifiedEventForProcessing.fileType = fileTypeNormalCSS
	} else if filepath.Ext(watcherEvent.Name) == ".go" {
		classifiedEventForProcessing.fileType = fileTypeGo
	} else if watcher.IsPublicStaticFile(watcherEvent.Name) {
		classifiedEventForProcessing.fileType = fileTypePublicStatic
	} else if watcher.IsPrivateStaticFile(watcherEvent.Name) {
		classifiedEventForProcessing.fileType = fileTypePrivateStatic
	} else {
		classifiedEventForProcessing.fileType = fileTypeOther
	}

	classifiedEventForProcessing.watchedFile = watcher.FindWatchedFile(watcherEvent.Name)

	if classifiedEventForProcessing.fileType == fileTypeGo &&
		classifiedEventForProcessing.watchedFile != nil &&
		classifiedEventForProcessing.watchedFile.TreatAsNonGo {
		classifiedEventForProcessing.fileType = fileTypeOther
	}

	if classifiedEventForProcessing.fileType == fileTypeOther &&
		classifiedEventForProcessing.watchedFile == nil {
		classifiedEventForProcessing.ignored = true
	}

	classifiedEventForProcessing.chmodOnly = isNonEmptyChmodOnly(watcherEvent)

	return classifiedEventForProcessing
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

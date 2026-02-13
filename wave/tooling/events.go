package tooling

import (
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
	"golang.org/x/sync/errgroup"
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

func (d buildPhaseDecision) hasFileProcessingWork() bool {
	return d.processPublicFiles ||
		d.processPrivateFiles ||
		d.buildCriticalCSS ||
		d.buildNormalCSS
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

// workSet collects per-phase execution decisions for a watcher cycle.
type workSet struct {
	build   buildPhaseDecision
	restart restartPhaseDecision
	browser browserPhaseDecision

	// User preference collected from watched files and applied during resolve.
	preferRevalidate bool
}

type eventExecutionPlan struct {
	eventsWithHooks       []eventWithHooks
	showRebuildingOverlay bool
	appStopStrategy       appStopStrategy
	runImplicitBuild      bool
}

type appStopStrategy int

const (
	appStopStrategyNone appStopStrategy = iota
	appStopStrategySingleEventHardReload
	appStopStrategyBatchHardReload
)

type eventExecutionPlanningResult struct {
	plan          *eventExecutionPlan
	configChanged bool
}

type refreshActionApplicationResult struct {
	restartRequested bool
	recompileGo      bool
}

// addFromRefreshAction merges a RefreshAction from a callback into the work set.
func (w *workSet) addFromRefreshAction(action wave.RefreshAction) {
	if action.TriggerRestart {
		w.restart.restartApp = true
		if action.RecompileGo {
			w.build.compileGo = true
		}
	}
	if action.ReloadBrowser {
		w.requestBrowserAction(browserPhaseActionHardReload)
	}
	if action.WaitForApp {
		w.browser.waitForApp = true
	}
	if action.WaitForVite {
		w.browser.waitForVite = true
	}
}

func (w *workSet) applyRefreshActions(
	actions []wave.RefreshAction,
) refreshActionApplicationResult {
	actionsToApply, actionResult := reduceRefreshActionsInStableOrder(actions)
	for _, action := range actionsToApply {
		w.addFromRefreshAction(action)
	}

	return actionResult
}

func reduceRefreshActionsInStableOrder(
	actions []wave.RefreshAction,
) ([]wave.RefreshAction, refreshActionApplicationResult) {
	appliedActions := make([]wave.RefreshAction, 0, len(actions))

	for _, action := range actions {
		if action.TriggerRestart {
			return appliedActions, refreshActionApplicationResult{
				restartRequested: true,
				recompileGo:      action.RecompileGo,
			}
		}
		appliedActions = append(appliedActions, action)
	}

	return appliedActions, refreshActionApplicationResult{}
}

func normalizeChangedSourceFilePathForWorkSet(
	filePath string,
) string {
	return pathnorm.Absolute(filePath)
}

func appendNormalizedFilePathIfMissing(
	existingFilePaths []string,
	existingFilePathSet map[string]struct{},
	filePath string,
) ([]string, map[string]struct{}) {
	normalizedFilePath := normalizeChangedSourceFilePathForWorkSet(filePath)
	if normalizedFilePath == "" {
		return existingFilePaths, existingFilePathSet
	}

	if existingFilePathSet == nil {
		existingFilePathSet = make(map[string]struct{}, len(existingFilePaths)+1)
		for _, existingFilePath := range existingFilePaths {
			existingFilePathSet[existingFilePath] = struct{}{}
		}
	}
	if _, alreadyExists := existingFilePathSet[normalizedFilePath]; alreadyExists {
		return existingFilePaths, existingFilePathSet
	}

	existingFilePaths = append(existingFilePaths, normalizedFilePath)
	existingFilePathSet[normalizedFilePath] = struct{}{}
	return existingFilePaths, existingFilePathSet
}

func (d *buildPhaseDecision) addPublicStaticChangedFilePath(
	filePath string,
) {
	d.publicStaticChangedFilePaths, d.publicStaticChangedFilePathSet = appendNormalizedFilePathIfMissing(
		d.publicStaticChangedFilePaths,
		d.publicStaticChangedFilePathSet,
		filePath,
	)
}

func (d *buildPhaseDecision) addPrivateStaticChangedFilePath(
	filePath string,
) {
	d.privateStaticChangedFilePaths, d.privateStaticChangedFilePathSet = appendNormalizedFilePathIfMissing(
		d.privateStaticChangedFilePaths,
		d.privateStaticChangedFilePathSet,
		filePath,
	)
}

// addImplicitWork adds build work implied by a file type.
func (w *workSet) addImplicitWork(c classifiedEvent) {
	wf := c.watchedFile
	if wf != nil && wf.RunOnChangeOnly {
		return
	}

	if wf != nil && wf.OnlyRunClientDefinedRevalidateFunc {
		w.preferRevalidate = true
	}

	switch c.fileType {
	case fileTypeGo:
		w.build.compileGo = true
		w.restart.restartApp = true

	case fileTypeCriticalCSS:
		w.build.buildCriticalCSS = true
		if wf != nil && needsHardReload(wf) {
			w.restart.restartApp = true
		}

	case fileTypeNormalCSS:
		w.build.buildNormalCSS = true
		if wf != nil && needsHardReload(wf) {
			w.restart.restartApp = true
		}

	case fileTypeCriticalAndNormalCSS:
		w.build.buildCriticalCSS = true
		w.build.buildNormalCSS = true
		if wf != nil && needsHardReload(wf) {
			w.restart.restartApp = true
		}

	case fileTypePublicStatic:
		w.build.processPublicFiles = true
		w.build.addPublicStaticChangedFilePath(
			c.event.Name,
		)

	case fileTypePrivateStatic:
		w.build.processPrivateFiles = true
		w.build.addPrivateStaticChangedFilePath(
			c.event.Name,
		)

	case fileTypeOther:
		if wf != nil {
			if wf.RecompileGoBinary {
				w.build.compileGo = true
			}
			if wf.RestartApp || wf.RecompileGoBinary {
				w.restart.restartApp = true
			}
		}
	}
}

// resolve determines browser behavior based on build work and user preferences.
func (w *workSet) resolve(usingVite bool) {
	if w.build.compileGo {
		w.restart.restartApp = true
	}
	w.determineBrowserBehavior(usingVite)
}

func (w *workSet) determineBrowserBehavior(usingVite bool) {
	if w.restart.restartApp {
		w.requestBrowserAction(browserPhaseActionHardReload)
		w.browser.waitForApp = true
		w.browser.waitForVite = usingVite
		return
	}

	// User preference takes precedence over automatic optimizations
	if w.preferRevalidate {
		w.requestBrowserAction(browserPhaseActionRevalidate)
		w.browser.waitForApp = true
		w.browser.waitForVite = usingVite
		return
	}

	cssWork := w.build.buildCriticalCSS || w.build.buildNormalCSS
	cssOnly := cssWork &&
		!w.build.processPublicFiles &&
		!w.build.processPrivateFiles

	if cssOnly {
		w.requestBrowserAction(browserPhaseActionHotReloadCSS)
		return
	}

	if w.build.processPublicFiles {
		w.requestBrowserAction(browserPhaseActionInvalidateVite)
		return
	}

	if w.build.processPrivateFiles || cssWork {
		w.requestBrowserAction(browserPhaseActionHardReload)
		w.browser.waitForApp = true
		w.browser.waitForVite = usingVite
		return
	}
}

func (w *workSet) requestBrowserAction(action browserPhaseAction) {
	if action > w.browser.action {
		w.browser.action = action
	}
}

func planBrowserReloadForAction(
	action browserPhaseAction,
	browserDecision browserPhaseDecision,
) (reloadOpts, bool) {
	switch action {
	case browserPhaseActionHardReload:
		return reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeOther},
			waitApp:   browserDecision.waitForApp,
			waitVite:  browserDecision.waitForVite,
			cycleVite: browserDecision.cycleVite,
		}, true
	case browserPhaseActionRevalidate:
		return reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeRevalidate},
			waitApp:   browserDecision.waitForApp,
			waitVite:  browserDecision.waitForVite,
			cycleVite: false,
		}, true
	default:
		return reloadOpts{}, false
	}
}

func planInvalidateViteFallbackBrowserDecision(
	usingVite bool,
) browserPhaseDecision {
	return browserPhaseDecision{
		action:      browserPhaseActionHardReload,
		waitForApp:  true,
		waitForVite: usingVite,
	}
}

func planHotReloadCSSPayloads(
	includeCriticalCSS bool,
	criticalCSS string,
	criticalCSSAvailable bool,
	includeNormalCSS bool,
	normalCSSURL string,
	normalCSSURLAvailable bool,
) []refreshPayload {
	payloads := make([]refreshPayload, 0, 2)
	if includeCriticalCSS && criticalCSSAvailable {
		payloads = append(payloads, refreshPayload{
			ChangeType:  changeTypeCriticalCSS,
			CriticalCSS: base64.StdEncoding.EncodeToString([]byte(criticalCSS)),
		})
	}
	if includeNormalCSS && normalCSSURLAvailable {
		payloads = append(payloads, refreshPayload{
			ChangeType:   changeTypeNormalCSS,
			NormalCSSURL: normalCSSURL,
		})
	}
	return payloads
}

// eventWithHooks pairs a classified event with its sorted hooks
type eventWithHooks struct {
	classified         classifiedEvent
	hooks              *wave.SortedHooks
	hookCtx            *wave.HookContext
	runOnChangeOnly    bool
	needsHardReload    bool
	skipDuplicateHooks bool
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
	if executionPlanningResult.configChanged {
		s.log.Info("Config changed, restarting")
		s.triggerConfigRestart()
		return
	}

	executionPlan := executionPlanningResult.plan
	if executionPlan == nil {
		return
	}

	if executionPlan.showRebuildingOverlay {
		s.broadcastRebuilding()
	}

	work := &workSet{}
	for _, eventWithHooksForLogging := range executionPlan.eventsWithHooks {
		s.log.Info(
			"[watcher]",
			"op",
			eventWithHooksForLogging.classified.event.Op.String(),
			"file",
			eventWithHooksForLogging.classified.event.Name,
		)
	}

	s.executeEventExecutionPlan(executionPlan, work, watcher)

	watcher.RemoveStale()
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

	return eventExecutionPlanningResult{
		plan: buildEventExecutionPlanFromClassifiedEvents(classifiedEvents),
	}
}

func buildEventExecutionPlanFromClassifiedEvents(
	classifiedEvents []classifiedEvent,
) *eventExecutionPlan {
	if len(classifiedEvents) == 0 {
		return nil
	}

	eventsWithHooks := buildEventHooksForProcessing(classifiedEvents)
	if len(eventsWithHooks) == 0 {
		return nil
	}

	return &eventExecutionPlan{
		eventsWithHooks:       eventsWithHooks,
		showRebuildingOverlay: shouldShowRebuildingOverlay(classifiedEvents),
		appStopStrategy:       resolveAppStopStrategy(eventsWithHooks),
		runImplicitBuild:      shouldRunImplicitBuildForEvents(eventsWithHooks),
	}
}

func deduplicateWatcherEventsByPath(
	events []fsnotify.Event,
) []fsnotify.Event {
	if len(events) == 0 {
		return nil
	}

	mergedEventOpsByPath := make(map[string]fsnotify.Op, len(events))
	deduplicationIndex := newWatcherEventDeduplicationIndex()
	for _, event := range events {
		eventPathKey := normalizeWatcherEventPathForDeduplication(event.Name)
		if existingPathKey := deduplicationIndex.resolveExistingPathKey(
			mergedEventOpsByPath,
			eventPathKey,
		); existingPathKey != "" {
			eventPathKey = existingPathKey
		} else {
			deduplicationIndex.recordPathKey(eventPathKey)
		}
		mergedEventOpsByPath[eventPathKey] |= event.Op
	}

	deduplicatedPaths := make([]string, 0, len(mergedEventOpsByPath))
	for eventPath := range mergedEventOpsByPath {
		deduplicatedPaths = append(deduplicatedPaths, eventPath)
	}
	sort.Strings(deduplicatedPaths)

	deduplicatedEvents := make([]fsnotify.Event, 0, len(deduplicatedPaths))
	for _, deduplicatedPath := range deduplicatedPaths {
		deduplicatedEvents = append(deduplicatedEvents, fsnotify.Event{
			Name: deduplicatedPath,
			Op:   mergedEventOpsByPath[deduplicatedPath],
		})
	}

	return deduplicatedEvents
}

type watcherEventDeduplicationIndex struct {
	canonicalPathToPathKey        map[string]string
	missingFileAliasKeyToPathKey  map[string]string
	canonicalPathByAbsolutePath   map[string]string
	missingAliasKeyByAbsolutePath map[string]string
}

func newWatcherEventDeduplicationIndex() *watcherEventDeduplicationIndex {
	return &watcherEventDeduplicationIndex{
		canonicalPathToPathKey:        make(map[string]string),
		missingFileAliasKeyToPathKey:  make(map[string]string),
		canonicalPathByAbsolutePath:   make(map[string]string),
		missingAliasKeyByAbsolutePath: make(map[string]string),
	}
}

func (index *watcherEventDeduplicationIndex) resolveExistingPathKey(
	mergedEventOpsByPath map[string]fsnotify.Op,
	normalizedEventPath string,
) string {
	if normalizedEventPath == "" {
		if _, alreadyExists := mergedEventOpsByPath[""]; alreadyExists {
			return ""
		}
		return ""
	}

	if _, alreadyExists := mergedEventOpsByPath[normalizedEventPath]; alreadyExists {
		return normalizedEventPath
	}

	if !filepath.IsAbs(normalizedEventPath) {
		return ""
	}

	canonicalPath := index.resolveCanonicalPathForAbsolutePath(normalizedEventPath)
	if canonicalPath != "" {
		if existingPathKey, exists := index.canonicalPathToPathKey[canonicalPath]; exists {
			return existingPathKey
		}
	}

	missingFileAliasKey := index.resolveMissingAliasKeyForAbsolutePath(normalizedEventPath)
	if missingFileAliasKey != "" {
		if existingPathKey, exists := index.missingFileAliasKeyToPathKey[missingFileAliasKey]; exists {
			return existingPathKey
		}
	}

	return ""
}

func (index *watcherEventDeduplicationIndex) recordPathKey(pathKey string) {
	if pathKey == "" || !filepath.IsAbs(pathKey) {
		return
	}

	canonicalPath := index.resolveCanonicalPathForAbsolutePath(pathKey)
	if canonicalPath != "" {
		if _, exists := index.canonicalPathToPathKey[canonicalPath]; !exists {
			index.canonicalPathToPathKey[canonicalPath] = pathKey
		}
	}

	missingFileAliasKey := index.resolveMissingAliasKeyForAbsolutePath(pathKey)
	if missingFileAliasKey != "" {
		if _, exists := index.missingFileAliasKeyToPathKey[missingFileAliasKey]; !exists {
			index.missingFileAliasKeyToPathKey[missingFileAliasKey] = pathKey
		}
	}
}

const missingAliasKeyResolutionEmptySentinel = "\x00"

func (index *watcherEventDeduplicationIndex) resolveCanonicalPathForAbsolutePath(
	absolutePath string,
) string {
	if absolutePath == "" || !filepath.IsAbs(absolutePath) {
		return ""
	}

	if canonicalPath, exists := index.canonicalPathByAbsolutePath[absolutePath]; exists {
		return canonicalPath
	}

	canonicalPath := canonicalizeAbsolutePathForWatcherEventDeduplication(absolutePath)
	if canonicalPath == "" {
		return ""
	}
	index.canonicalPathByAbsolutePath[absolutePath] = canonicalPath
	return canonicalPath
}

func (index *watcherEventDeduplicationIndex) resolveMissingAliasKeyForAbsolutePath(
	absolutePath string,
) string {
	if absolutePath == "" || !filepath.IsAbs(absolutePath) {
		return ""
	}

	if cachedAliasKey, exists := index.missingAliasKeyByAbsolutePath[absolutePath]; exists {
		if cachedAliasKey == missingAliasKeyResolutionEmptySentinel {
			return ""
		}
		return cachedAliasKey
	}

	missingAliasKey := missingFileAliasKeyForWatcherEventDeduplication(absolutePath)
	if missingAliasKey == "" {
		index.missingAliasKeyByAbsolutePath[absolutePath] = missingAliasKeyResolutionEmptySentinel
		return ""
	}

	index.missingAliasKeyByAbsolutePath[absolutePath] = missingAliasKey
	return missingAliasKey
}

func normalizeWatcherEventPathForDeduplication(
	eventPath string,
) string {
	trimmedEventPath := strings.TrimSpace(eventPath)
	if trimmedEventPath == "" {
		return ""
	}

	cleanedEventPath := filepath.Clean(trimmedEventPath)
	if filepath.IsAbs(cleanedEventPath) {
		return pathnorm.Absolute(cleanedEventPath)
	}

	return cleanedEventPath
}

func canonicalizeAbsolutePathForWatcherEventDeduplication(
	absolutePath string,
) string {
	return pathnorm.CanonicalizePathForLocationComparison(absolutePath)
}

func missingFileAliasKeyForWatcherEventDeduplication(
	absolutePath string,
) string {
	if absolutePath == "" || !filepath.IsAbs(absolutePath) {
		return ""
	}

	baseName := filepath.Base(absolutePath)
	if baseName == "" || baseName == "." || baseName == string(filepath.Separator) {
		return ""
	}

	parentDirectoryPath := pathnorm.AbsoluteDirectory(absolutePath)
	if parentDirectoryPath == "" {
		return ""
	}
	canonicalParentDirectoryPath := canonicalizeAbsolutePathForWatcherEventDeduplication(
		parentDirectoryPath,
	)
	if canonicalParentDirectoryPath == "" {
		return ""
	}

	return canonicalParentDirectoryPath + "\x00" + baseName
}

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

	classifiedEvents := make([]classifiedEvent, 0, len(preClassificationPlan.plannedEvents))
	for _, plannedEvent := range preClassificationPlan.plannedEvents {
		if plannedEvent.preClassificationDecision.addDirectoryWatch {
			addDirectoryWatchError := watcher.AddDir(plannedEvent.event.Name)
			if shouldLogWatcherAddDirectoryError(addDirectoryWatchError) {
				s.log.Warn(
					"failed to add directory watch",
					"path",
					plannedEvent.event.Name,
					"error",
					addDirectoryWatchError,
				)
			}
		}
		if !plannedEvent.preClassificationDecision.classifyEvent {
			continue
		}

		classifiedEventForProcessing := s.classifyEventWithWatcherAndBuilder(
			plannedEvent.event,
			watcher,
			builder,
		)
		postClassificationDecision := deriveWatcherEventPostClassificationDecision(
			classifiedEventForProcessing,
		)
		if !postClassificationDecision.includeClassifiedEvent {
			continue
		}

		classifiedEvents = append(classifiedEvents, classifiedEventForProcessing)
	}

	return classifiedEvents, false
}

type watcherEventPreClassificationDecision struct {
	configChanged     bool
	addDirectoryWatch bool
	classifyEvent     bool
}

type watcherEventPreClassificationInput struct {
	event                       fsnotify.Event
	isConfigFile                bool
	eventIsDirectory            bool
	eventPathStatProbeSucceeded bool
}

type watcherEventPreClassificationPlannedEvent struct {
	event                     fsnotify.Event
	preClassificationDecision watcherEventPreClassificationDecision
}

type watcherEventPreClassificationPlan struct {
	configChanged bool
	plannedEvents []watcherEventPreClassificationPlannedEvent
}

func buildWatcherEventPreClassificationPlanFromEvents(
	events []fsnotify.Event,
	eventClassificationProber *watcherEventClassificationProber,
) watcherEventPreClassificationPlan {
	if len(events) == 0 {
		return watcherEventPreClassificationPlan{}
	}

	plannedEvents := make([]watcherEventPreClassificationPlannedEvent, 0, len(events))
	for _, event := range events {
		isConfigFile := eventClassificationProber.probeIsConfigFile(event.Name)
		if isConfigMutationEvent(event, isConfigFile) {
			return watcherEventPreClassificationPlan{
				configChanged: true,
			}
		}

		directoryProbeResult := eventClassificationProber.probeEventDirectoryStatus(event.Name)
		preClassificationDecision := deriveWatcherEventPreClassificationDecision(
			event,
			isConfigFile,
			directoryProbeResult.isDirectory,
			directoryProbeResult.statProbeSucceeded,
		)

		if !preClassificationDecision.addDirectoryWatch &&
			!preClassificationDecision.classifyEvent {
			continue
		}

		plannedEvents = append(
			plannedEvents,
			watcherEventPreClassificationPlannedEvent{
				event:                     event,
				preClassificationDecision: preClassificationDecision,
			},
		)
	}

	return watcherEventPreClassificationPlan{
		plannedEvents: plannedEvents,
	}
}

type watcherEventDirectoryProbeResult struct {
	statProbeSucceeded bool
	isDirectory        bool
}

type watcherEventClassificationProber struct {
	isConfigFileByPath map[string]bool
	isDirectoryByPath  map[string]watcherEventDirectoryProbeResult
	isConfigFileFn     func(string) bool
	statPathFn         func(string) (os.FileInfo, error)
}

func newWatcherEventClassificationProber(
	isConfigFileFn func(string) bool,
) *watcherEventClassificationProber {
	return &watcherEventClassificationProber{
		isConfigFileByPath: make(map[string]bool),
		isDirectoryByPath:  make(map[string]watcherEventDirectoryProbeResult),
		isConfigFileFn:     isConfigFileFn,
		statPathFn:         os.Stat,
	}
}

func (prober *watcherEventClassificationProber) probeIsConfigFile(path string) bool {
	if prober == nil {
		return false
	}
	if resolvedIsConfigFile, hasCachedResult := prober.isConfigFileByPath[path]; hasCachedResult {
		return resolvedIsConfigFile
	}

	resolvedIsConfigFile := false
	if prober.isConfigFileFn != nil {
		resolvedIsConfigFile = prober.isConfigFileFn(path)
	}

	prober.isConfigFileByPath[path] = resolvedIsConfigFile
	return resolvedIsConfigFile
}

func (prober *watcherEventClassificationProber) probeEventDirectoryStatus(
	path string,
) watcherEventDirectoryProbeResult {
	if prober == nil {
		return watcherEventDirectoryProbeResult{}
	}
	if cachedDirectoryProbeResult, hasCachedResult := prober.isDirectoryByPath[path]; hasCachedResult {
		return cachedDirectoryProbeResult
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
	prober.isDirectoryByPath[path] = directoryProbeResult
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

func buildWatcherEventPreClassificationPlan(
	preClassificationInputs []watcherEventPreClassificationInput,
) watcherEventPreClassificationPlan {
	if len(preClassificationInputs) == 0 {
		return watcherEventPreClassificationPlan{}
	}

	plannedEvents := make([]watcherEventPreClassificationPlannedEvent, 0, len(preClassificationInputs))
	for _, preClassificationInput := range preClassificationInputs {
		preClassificationDecision := deriveWatcherEventPreClassificationDecision(
			preClassificationInput.event,
			preClassificationInput.isConfigFile,
			preClassificationInput.eventIsDirectory,
			preClassificationInput.eventPathStatProbeSucceeded,
		)
		if preClassificationDecision.configChanged {
			return watcherEventPreClassificationPlan{
				configChanged: true,
			}
		}
		if !preClassificationDecision.addDirectoryWatch &&
			!preClassificationDecision.classifyEvent {
			continue
		}

		plannedEvents = append(
			plannedEvents,
			watcherEventPreClassificationPlannedEvent{
				event:                     preClassificationInput.event,
				preClassificationDecision: preClassificationDecision,
			},
		)
	}

	return watcherEventPreClassificationPlan{
		plannedEvents: plannedEvents,
	}
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

func buildEventHooksForProcessing(
	classifiedEvents []classifiedEvent,
) []eventWithHooks {
	if len(classifiedEvents) == 0 {
		return nil
	}

	eventsWithHooks := make([]eventWithHooks, 0, len(classifiedEvents))
	handledWatchedPatterns := make(map[string]struct{})
	changedFilePathsByWatchedPattern := make(map[string][]string)

	for _, classifiedEventForProcessing := range classifiedEvents {
		if classifiedEventForProcessing.watchedFile != nil {
			watchedPattern := classifiedEventForProcessing.watchedFile.Pattern
			changedFilePathsByWatchedPattern[watchedPattern] = append(
				changedFilePathsByWatchedPattern[watchedPattern],
				classifiedEventForProcessing.event.Name,
			)
		}

		watchedFile := classifiedEventForProcessing.watchedFile
		if watchedFile == nil {
			watchedFile = &wave.WatchedFile{}
		}
		if watchedFile.SortedHooks == nil {
			watchedFile.Sort()
		}
		if watchedFile.SortedHooks == nil {
			watchedFile.SortedHooks = &wave.SortedHooks{}
		}

		eventNeedsHardReload := classifiedEventForProcessing.fileType == fileTypeGo ||
			needsHardReload(watchedFile)
		skipDuplicateHooks := false
		if classifiedEventForProcessing.watchedFile != nil {
			watchedPattern := classifiedEventForProcessing.watchedFile.Pattern
			if _, alreadyHandled := handledWatchedPatterns[watchedPattern]; alreadyHandled {
				skipDuplicateHooks = true
			} else {
				handledWatchedPatterns[watchedPattern] = struct{}{}
			}
		}

		eventsWithHooks = append(eventsWithHooks, eventWithHooks{
			classified:         classifiedEventForProcessing,
			hooks:              watchedFile.SortedHooks,
			runOnChangeOnly:    watchedFile.RunOnChangeOnly,
			needsHardReload:    eventNeedsHardReload,
			skipDuplicateHooks: skipDuplicateHooks,
			hookCtx: &wave.HookContext{
				FilePath:           classifiedEventForProcessing.event.Name,
				ChangedFilePaths:   []string{classifiedEventForProcessing.event.Name},
				AppStoppedForBatch: false,
			},
		})
	}

	for i := range eventsWithHooks {
		classifiedEventForProcessing := eventsWithHooks[i].classified
		if classifiedEventForProcessing.watchedFile == nil {
			continue
		}
		watchedPattern := classifiedEventForProcessing.watchedFile.Pattern
		eventsWithHooks[i].hookCtx.ChangedFilePaths = append(
			[]string(nil),
			changedFilePathsByWatchedPattern[watchedPattern]...,
		)
	}

	return eventsWithHooks
}

func shouldShowRebuildingOverlay(
	classifiedEvents []classifiedEvent,
) bool {
	for _, classifiedEventForOverlay := range classifiedEvents {
		if classifiedEventForOverlay.fileType == fileTypeCriticalCSS ||
			classifiedEventForOverlay.fileType == fileTypeNormalCSS ||
			classifiedEventForOverlay.fileType == fileTypeCriticalAndNormalCSS {
			continue
		}

		if classifiedEventForOverlay.watchedFile == nil ||
			!classifiedEventForOverlay.watchedFile.SkipRebuildingNotification {
			return true
		}
	}

	return false
}

func anyEventNeedsHardReload(eventsWithHooks []eventWithHooks) bool {
	for _, eventWithHooksForCheck := range eventsWithHooks {
		if eventWithHooksForCheck.needsHardReload {
			return true
		}
	}
	return false
}

func resolveAppStopStrategy(eventsWithHooks []eventWithHooks) appStopStrategy {
	if len(eventsWithHooks) == 0 {
		return appStopStrategyNone
	}

	if len(eventsWithHooks) == 1 {
		if eventsWithHooks[0].needsHardReload {
			return appStopStrategySingleEventHardReload
		}
		return appStopStrategyNone
	}

	if anyEventNeedsHardReload(eventsWithHooks) {
		return appStopStrategyBatchHardReload
	}

	return appStopStrategyNone
}

func shouldRunImplicitBuildForEvents(eventsWithHooks []eventWithHooks) bool {
	for _, eventWithHooksForCheck := range eventsWithHooks {
		if !eventWithHooksForCheck.runOnChangeOnly {
			return true
		}
	}
	return false
}

func (s *server) executeEventExecutionPlan(
	plan *eventExecutionPlan,
	work *workSet,
	watcher *Watcher,
) {
	if plan == nil || len(plan.eventsWithHooks) == 0 {
		return
	}

	switch plan.appStopStrategy {
	case appStopStrategySingleEventHardReload:
		s.log.Info("Terminating running app")
		if err := s.stopApp(); err != nil {
			s.log.Error("Failed to terminate app", "error", err)
		}

	case appStopStrategyBatchHardReload:
		s.log.Info("Stopping app for batch rebuild")
		if err := s.stopApp(); err != nil {
			s.log.Error("Failed to stop app", "error", err)
		}
		for i := range plan.eventsWithHooks {
			if plan.eventsWithHooks[i].hookCtx != nil {
				plan.eventsWithHooks[i].hookCtx.AppStoppedForBatch = true
			}
		}

	case appStopStrategyNone:
	}

	s.processEventsWithDeterministicPipeline(plan, work, watcher)
}

func (s *server) processEventsWithDeterministicPipeline(
	plan *eventExecutionPlan,
	work *workSet,
	watcher *Watcher,
) {
	if plan == nil || len(plan.eventsWithHooks) == 0 {
		return
	}

	eventsWithHooks := plan.eventsWithHooks
	s.fireNoWaitHooksForEvents(eventsWithHooks, watcher)

	preActions := s.runPreHooksForEvents(eventsWithHooks, work, watcher)
	preActionResult := work.applyRefreshActions(preActions)
	if preActionResult.restartRequested {
		s.triggerRestartFromRefreshActions(preActionResult)
		return
	}

	if !plan.runImplicitBuild {
		if len(eventsWithHooks) == 1 {
			s.log.Info("RunOnChangeOnly: skipping implicit build phase")
		} else {
			s.log.Info("All events are RunOnChangeOnly, skipping implicit build phase")
		}
	} else {
		work.resolve(s.cfg.UsingVite())
	}

	var buildAndConcurrentHooksGroup errgroup.Group
	if plan.runImplicitBuild {
		buildAndConcurrentHooksGroup.Go(func() error {
			s.executeBuildPhase(work)
			return nil
		})
	}

	var concurrentActions []wave.RefreshAction
	buildAndConcurrentHooksGroup.Go(func() error {
		concurrentActions = s.runConcurrentHooksForEvents(eventsWithHooks, watcher)
		return nil
	})
	_ = buildAndConcurrentHooksGroup.Wait()

	concurrentActionResult := work.applyRefreshActions(concurrentActions)
	if concurrentActionResult.restartRequested {
		s.triggerRestartFromRefreshActions(concurrentActionResult)
		return
	}

	postActions := s.runPostHooksForEvents(eventsWithHooks, watcher)
	postActionResult := work.applyRefreshActions(postActions)
	if postActionResult.restartRequested {
		s.triggerRestartFromRefreshActions(postActionResult)
		return
	}

	if plan.runImplicitBuild && work.restart.restartApp {
		s.log.Info("Restarting app")
		s.startApp()
	}

	s.executeBrowserPhase(work)
}

func (s *server) fireNoWaitHooksForEvents(
	eventsWithHooks []eventWithHooks,
	watcher *Watcher,
) {
	for _, eventWithHooksForFire := range eventsWithHooks {
		if eventWithHooksForFire.skipDuplicateHooks {
			continue
		}
		s.fireNoWaitHooks(eventWithHooksForFire, watcher)
	}
}

func (s *server) runPreHooksForEvents(
	eventsWithHooks []eventWithHooks,
	work *workSet,
	watcher *Watcher,
) []wave.RefreshAction {
	allPreActions := make([]wave.RefreshAction, 0)

	for _, eventWithHooksForPre := range eventsWithHooks {
		work.addImplicitWork(eventWithHooksForPre.classified)

		if eventWithHooksForPre.skipDuplicateHooks {
			continue
		}

		preActions, err := s.runPreHooks(eventWithHooksForPre, watcher)
		if err != nil {
			s.log.Error("Pre-hook execution failed", "error", err)
		}
		allPreActions = append(allPreActions, preActions...)
	}

	return allPreActions
}

func (s *server) runConcurrentHooksForEvents(
	eventsWithHooks []eventWithHooks,
	watcher *Watcher,
) []wave.RefreshAction {
	actionsByEventIndex := make([][]wave.RefreshAction, len(eventsWithHooks))
	var concurrentHooksGroup errgroup.Group

	for eventIndex := range eventsWithHooks {
		eventWithHooksForConcurrent := eventsWithHooks[eventIndex]
		if eventWithHooksForConcurrent.skipDuplicateHooks {
			continue
		}

		eventIndexForResult := eventIndex
		eventWithHooksForConcurrentCopy := eventWithHooksForConcurrent
		concurrentHooksGroup.Go(func() error {
			concurrentActions, err := s.runConcurrentHooks(
				eventWithHooksForConcurrentCopy,
				watcher,
			)
			if err != nil {
				s.log.Error("Concurrent hook execution failed", "error", err)
			}
			actionsByEventIndex[eventIndexForResult] = concurrentActions
			return nil
		})
	}

	_ = concurrentHooksGroup.Wait()

	allConcurrentActions := make([]wave.RefreshAction, 0)
	for _, concurrentActionsForEvent := range actionsByEventIndex {
		allConcurrentActions = append(allConcurrentActions, concurrentActionsForEvent...)
	}

	return allConcurrentActions
}

func (s *server) runPostHooksForEvents(
	eventsWithHooks []eventWithHooks,
	watcher *Watcher,
) []wave.RefreshAction {
	allPostActions := make([]wave.RefreshAction, 0)

	for _, eventWithHooksForPost := range eventsWithHooks {
		if eventWithHooksForPost.skipDuplicateHooks {
			continue
		}

		postActions, err := s.runPostHooks(eventWithHooksForPost, watcher)
		if err != nil {
			s.log.Error("Post-hook execution failed", "error", err)
		}
		allPostActions = append(allPostActions, postActions...)
	}

	return allPostActions
}

func (s *server) fireNoWaitHooks(ewh eventWithHooks, watcher *Watcher) {
	for _, hook := range ewh.hooks.ConcurrentNoWait {
		if watcher.IsIgnored(ewh.classified.event.Name, hook.Exclude) {
			continue
		}
		if hook.Callback != nil {
			go func(cb func(*wave.HookContext) (*wave.RefreshAction, error), ctx *wave.HookContext) {
				if _, err := cb(ctx); err != nil {
					s.log.Warn("concurrent-no-wait callback failed", "error", err)
				}
			}(hook.Callback, ewh.hookCtx)
		}
		resolvedCommand := s.resolveHookCommand(hook)
		if resolvedCommand != "" {
			go func(command string) {
				if err := executil.RunShell(command); err != nil {
					s.log.Warn("concurrent-no-wait hook failed", "cmd", command, "error", err)
				}
			}(resolvedCommand)
		}
	}
}

func (s *server) runPreHooks(ewh eventWithHooks, watcher *Watcher) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction

	for _, preHook := range ewh.hooks.Pre {
		if watcher.IsIgnored(ewh.classified.event.Name, preHook.Exclude) {
			continue
		}
		action, err := s.executeHook(preHook, ewh.hookCtx)
		if action != nil {
			actions = append(actions, *action)
		}
		if err != nil {
			return actions, err
		}
	}

	return actions, nil
}

func (s *server) runConcurrentHooks(ewh eventWithHooks, watcher *Watcher) ([]wave.RefreshAction, error) {
	if len(ewh.hooks.Concurrent) == 0 {
		return nil, nil
	}

	concurrentHooksToRun := make([]wave.OnChangeHook, 0, len(ewh.hooks.Concurrent))
	for _, concurrentHook := range ewh.hooks.Concurrent {
		if watcher.IsIgnored(ewh.classified.event.Name, concurrentHook.Exclude) {
			continue
		}
		hook, shouldRunHook := prepareHookForExecutionWithRunOnChangeOnlyRules(
			ewh.runOnChangeOnly,
			concurrentHook,
		)
		if !shouldRunHook {
			continue
		}

		concurrentHooksToRun = append(concurrentHooksToRun, hook)
	}

	actionsByHookIndex := make([]*wave.RefreshAction, len(concurrentHooksToRun))
	var eg errgroup.Group

	for hookIndex := range concurrentHooksToRun {
		hookForExecution := concurrentHooksToRun[hookIndex]
		hookIndexForResult := hookIndex
		eg.Go(func() error {
			action, err := s.executeHook(hookForExecution, ewh.hookCtx)
			actionsByHookIndex[hookIndexForResult] = action
			return err
		})
	}

	err := eg.Wait()
	actions := make([]wave.RefreshAction, 0, len(actionsByHookIndex))
	for _, action := range actionsByHookIndex {
		if action != nil {
			actions = append(actions, *action)
		}
	}

	return actions, err
}

func (s *server) runPostHooks(ewh eventWithHooks, watcher *Watcher) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction

	for _, postHook := range ewh.hooks.Post {
		if watcher.IsIgnored(ewh.classified.event.Name, postHook.Exclude) {
			continue
		}
		hook, shouldRunHook := prepareHookForExecutionWithRunOnChangeOnlyRules(
			ewh.runOnChangeOnly,
			postHook,
		)
		if !shouldRunHook {
			continue
		}

		action, err := s.executeHook(hook, ewh.hookCtx)
		if action != nil {
			actions = append(actions, *action)
		}
		if err != nil {
			return actions, err
		}
	}

	return actions, nil
}

func prepareHookForExecutionWithRunOnChangeOnlyRules(
	isRunOnChangeOnly bool,
	hook wave.OnChangeHook,
) (wave.OnChangeHook, bool) {
	if !isRunOnChangeOnly || !hookHasCommandAction(hook) {
		return hook, true
	}

	if hook.Callback == nil {
		return wave.OnChangeHook{}, false
	}

	hook.Cmd = ""
	hook.RunCombinedDevBuildHookCommands = false
	return hook, true
}

func hookHasCommandAction(hook wave.OnChangeHook) bool {
	return strings.TrimSpace(hook.Cmd) != "" || hook.RunCombinedDevBuildHookCommands
}

func (s *server) executeHook(
	hook wave.OnChangeHook,
	hookContext *wave.HookContext,
) (*wave.RefreshAction, error) {
	var action *wave.RefreshAction
	if hook.Callback != nil {
		callbackAction, err := hook.Callback(hookContext)
		if err != nil {
			return nil, err
		}
		action = callbackAction
	}

	resolvedCommand := s.resolveHookCommand(hook)
	if resolvedCommand != "" {
		if err := executil.RunShell(resolvedCommand); err != nil {
			return action, err
		}
	}

	return action, nil
}

func (s *server) triggerRestartFromRefreshActions(
	actionResult refreshActionApplicationResult,
) {
	if actionResult.recompileGo {
		s.triggerRestart()
		return
	}
	s.triggerRestartNoGo()
}

func (s *server) executeBuildPhase(work *workSet) {
	builder := s.getBuilder()
	if builder == nil {
		s.log.Error("Builder is nil during build phase")
		return
	}

	var g errgroup.Group

	if work.build.compileGo {
		g.Go(func() error {
			if err := builder.CompileGoOnly(true); err != nil {
				s.log.Error("Go compilation failed", "error", err)
				return err
			}
			return nil
		})
	}

	needsFileProcessing := work.build.hasFileProcessingWork()

	if needsFileProcessing {
		g.Go(func() error {
			if work.build.processPublicFiles {
				var publicProcessingError error
				if len(work.build.publicStaticChangedFilePaths) > 0 {
					publicProcessingError = builder.processPublicFilesOnlyForChangedPaths(
						work.build.publicStaticChangedFilePaths,
					)
				} else {
					publicProcessingError = builder.ProcessPublicFilesOnly()
				}
				if publicProcessingError != nil {
					s.log.Error("Public files processing failed", "error", publicProcessingError)
					return publicProcessingError
				}
				if s.cfg.FrameworkPublicFileMapOutDir != "" {
					if err := builder.WritePublicFileMapTS(s.cfg.FrameworkPublicFileMapOutDir); err != nil {
						s.log.Error("Write public file map TS failed", "error", err)
						return err
					}
				}
			}

			var innerG errgroup.Group

			if work.build.processPrivateFiles {
				innerG.Go(func() error {
					var privateProcessingError error
					if len(work.build.privateStaticChangedFilePaths) > 0 {
						privateProcessingError = builder.processPrivateFilesOnlyForChangedPaths(
							work.build.privateStaticChangedFilePaths,
						)
					} else {
						privateProcessingError = builder.ProcessPrivateFilesOnly()
					}
					if privateProcessingError != nil {
						s.log.Error("Private files processing failed", "error", privateProcessingError)
						return privateProcessingError
					}
					return nil
				})
			}

			if work.build.buildCriticalCSS {
				innerG.Go(func() error {
					if err := builder.BuildCriticalCSS(true); err != nil {
						s.log.Error("Critical CSS build failed", "error", err)
						return err
					}
					return nil
				})
			}

			if work.build.buildNormalCSS {
				innerG.Go(func() error {
					if err := builder.BuildNormalCSS(true); err != nil {
						s.log.Error("Normal CSS build failed", "error", err)
						return err
					}
					return nil
				})
			}

			return innerG.Wait()
		})
	}

	if err := g.Wait(); err != nil {
		s.log.Error("Build phase had errors", "error", err)
	}
}

func (s *server) executeBrowserPhase(work *workSet) {
	if !s.cfg.UsingBrowser() {
		return
	}

	builder := s.getBuilder()

	switch work.browser.action {
	case browserPhaseActionInvalidateVite:
		if s.cfg.UsingVite() {
			if err := s.callViteFilemapInvalidate(); err != nil {
				s.log.Warn("Vite filemap invalidate failed, falling back to reload", "error", err)
			} else {
				return
			}
		}
		fallbackDecision := planInvalidateViteFallbackBrowserDecision(s.cfg.UsingVite())
		work.browser.action = fallbackDecision.action
		work.browser.waitForApp = fallbackDecision.waitForApp
		work.browser.waitForVite = fallbackDecision.waitForVite
		fallthrough

	case browserPhaseActionHardReload, browserPhaseActionRevalidate:
		if reloadPlan, hasReloadPlan := planBrowserReloadForAction(work.browser.action, work.browser); hasReloadPlan {
			if work.browser.action == browserPhaseActionHardReload {
				s.log.Info("Hard reloading browser")
			} else {
				s.log.Info("Running client-defined revalidate function")
			}
			s.broadcastReload(reloadPlan)
			return
		}

	case browserPhaseActionHotReloadCSS:
		if builder == nil {
			return
		}

		s.log.Info("Hot reloading CSS")

		criticalCSS := ""
		criticalCSSAvailable := false
		if work.build.buildCriticalCSS {
			var readCriticalCSSError error
			criticalCSS, readCriticalCSSError = builder.ReadCriticalCSSForHotReload(true)
			if readCriticalCSSError != nil {
				s.log.Warn(
					"Skipping critical CSS hot reload payload due to missing fresh build output",
					"error",
					readCriticalCSSError,
				)
			} else {
				criticalCSSAvailable = true
			}
		}

		normalCSSURL := ""
		normalCSSURLAvailable := false
		if work.build.buildNormalCSS {
			var readNormalCSSURLError error
			normalCSSURL, readNormalCSSURLError = builder.ReadNormalCSSURLForHotReload(true)
			if readNormalCSSURLError != nil {
				s.log.Warn(
					"Skipping normal CSS hot reload payload due to missing fresh build output",
					"error",
					readNormalCSSURLError,
				)
			} else {
				normalCSSURLAvailable = true
			}
		}

		payloads := planHotReloadCSSPayloads(
			work.build.buildCriticalCSS,
			criticalCSS,
			criticalCSSAvailable,
			work.build.buildNormalCSS,
			normalCSSURL,
			normalCSSURLAvailable,
		)
		for _, payload := range payloads {
			s.broadcastReload(reloadOpts{
				payload: payload,
			})
		}
		return

	case browserPhaseActionNone:
		return
	}
}

func (s *server) classifyEventWithWatcherAndBuilder(evt fsnotify.Event, watcher *Watcher, builder *Builder) classifiedEvent {
	result := classifiedEvent{event: evt}

	if evt.Name == "" {
		result.ignored = true
		return result
	}

	result.ignored = watcher.IsIgnoredFile(evt.Name)

	isCriticalCSSFile := builder.IsCriticalCSSFile(evt.Name)
	isNormalCSSFile := builder.IsNormalCSSFile(evt.Name)

	if isCriticalCSSFile && isNormalCSSFile {
		result.fileType = fileTypeCriticalAndNormalCSS
	} else if isCriticalCSSFile {
		result.fileType = fileTypeCriticalCSS
	} else if isNormalCSSFile {
		result.fileType = fileTypeNormalCSS
	} else if filepath.Ext(evt.Name) == ".go" {
		result.fileType = fileTypeGo
	} else if watcher.IsPublicStaticFile(evt.Name) {
		result.fileType = fileTypePublicStatic
	} else if watcher.IsPrivateStaticFile(evt.Name) {
		result.fileType = fileTypePrivateStatic
	} else {
		result.fileType = fileTypeOther
	}

	result.watchedFile = watcher.FindWatchedFile(evt.Name)

	if result.fileType == fileTypeGo && result.watchedFile != nil && result.watchedFile.TreatAsNonGo {
		result.fileType = fileTypeOther
	}

	if result.fileType == fileTypeOther && result.watchedFile == nil {
		result.ignored = true
	}

	result.chmodOnly = isNonEmptyChmodOnly(evt)

	return result
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

func needsHardReload(wf *wave.WatchedFile) bool {
	if wf == nil {
		return false
	}
	return wf.RecompileGoBinary || wf.RestartApp
}

func (s *server) resolveHookCommand(hook wave.OnChangeHook) string {
	if hook.RunCombinedDevBuildHookCommands {
		if strings.TrimSpace(hook.Cmd) != "" {
			return resolveSequentialShellCommands(
				hook.Cmd,
				getUserDevBuildHook(s.cfg),
				getFrameworkDevBuildHook(s.cfg),
			)
		}
		return resolveSequentialShellCommands(
			getUserDevBuildHook(s.cfg),
			getFrameworkDevBuildHook(s.cfg),
		)
	}
	return hook.Cmd
}

func getUserDevBuildHook(cfg *wave.ParsedConfig) string {
	if cfg == nil || cfg.Core == nil {
		return ""
	}
	return cfg.Core.DevBuildHook
}

func getFrameworkDevBuildHook(cfg *wave.ParsedConfig) string {
	if cfg == nil {
		return ""
	}
	return cfg.FrameworkDevBuildHook
}

// resolveSequentialShellCommands combines non-empty shell commands in order.
// The resulting command preserves "fail fast" behavior by chaining with &&.
func resolveSequentialShellCommands(commands ...string) string {
	nonEmptyCommands := make([]string, 0, len(commands))
	for _, command := range commands {
		trimmed := strings.TrimSpace(command)
		if trimmed != "" {
			nonEmptyCommands = append(nonEmptyCommands, trimmed)
		}
	}
	return strings.Join(nonEmptyCommands, " && ")
}

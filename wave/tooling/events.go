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

// addFromRefreshAction merges a RefreshAction from a callback into the work set.
func (w *workSet) addFromRefreshAction(action wave.RefreshAction) {
	workMutationDecision := deriveRefreshActionWorkMutationDecision(action)
	w.applyRefreshActionWorkMutationDecision(workMutationDecision)
}

func deriveRefreshActionWorkMutationDecision(
	action wave.RefreshAction,
) refreshActionWorkMutationDecision {
	workMutationDecision := refreshActionWorkMutationDecision{}
	if action.TriggerRestart {
		workMutationDecision.restartApp = true
		workMutationDecision.compileGo = action.RecompileGo
	}
	if action.ReloadBrowser {
		workMutationDecision.requestBrowserAction = true
		workMutationDecision.browserAction = browserPhaseActionHardReload
	}
	workMutationDecision.waitForApp = action.WaitForApp
	workMutationDecision.waitForVite = action.WaitForVite
	return workMutationDecision
}

func (w *workSet) applyRefreshActionWorkMutationDecision(
	workMutationDecision refreshActionWorkMutationDecision,
) {
	if workMutationDecision.restartApp {
		w.restart.restartApp = true
	}
	if workMutationDecision.compileGo {
		w.build.compileGo = true
	}
	if workMutationDecision.requestBrowserAction {
		w.requestBrowserAction(workMutationDecision.browserAction)
	}
	if workMutationDecision.waitForApp {
		w.browser.waitForApp = true
	}
	if workMutationDecision.waitForVite {
		w.browser.waitForVite = true
	}
}

func (w *workSet) applyRefreshActions(
	actions []wave.RefreshAction,
) refreshActionApplicationResult {
	reductionDecision := reduceRefreshActionsInStableOrder(actions)
	for _, action := range reductionDecision.actionsBeforeRestart {
		w.addFromRefreshAction(action)
	}

	return reductionDecision.applicationResult
}

func reduceRefreshActionsInStableOrder(
	actions []wave.RefreshAction,
) refreshActionReductionDecision {
	reductionDecision := refreshActionReductionDecision{
		actionsBeforeRestart: make([]wave.RefreshAction, 0, len(actions)),
		restartActionIndex:   -1,
	}

	for actionIndex, action := range actions {
		if action.TriggerRestart {
			reductionDecision.restartActionEncountered = true
			reductionDecision.restartActionIndex = actionIndex
			reductionDecision.applicationResult = refreshActionApplicationResult{
				restartRequested: true,
				recompileGo:      action.RecompileGo,
			}
			return reductionDecision
		}
		reductionDecision.actionsBeforeRestart = append(reductionDecision.actionsBeforeRestart, action)
	}

	return reductionDecision
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
	implicitWorkDecisionForClassifiedEvent := deriveImplicitWorkDecisionForClassifiedEvent(c)
	w.applyImplicitWorkDecision(implicitWorkDecisionForClassifiedEvent)
}

func deriveImplicitWorkDecisionForClassifiedEvent(
	classifiedEventForWork classifiedEvent,
) implicitWorkDecision {
	watchedFileForWork := classifiedEventForWork.watchedFile
	if watchedFileForWork != nil && watchedFileForWork.RunOnChangeOnly {
		return implicitWorkDecision{}
	}

	decision := implicitWorkDecision{}
	if watchedFileForWork != nil && watchedFileForWork.OnlyRunClientDefinedRevalidateFunc {
		decision.preferRevalidate = true
	}

	switch classifiedEventForWork.fileType {
	case fileTypeGo:
		decision.compileGo = true
		decision.restartApp = true

	case fileTypeCriticalCSS:
		decision.buildCriticalCSS = true
		decision.restartApp = watchedFileForWork != nil && needsHardReload(watchedFileForWork)

	case fileTypeNormalCSS:
		decision.buildNormalCSS = true
		decision.restartApp = watchedFileForWork != nil && needsHardReload(watchedFileForWork)

	case fileTypeCriticalAndNormalCSS:
		decision.buildCriticalCSS = true
		decision.buildNormalCSS = true
		decision.restartApp = watchedFileForWork != nil && needsHardReload(watchedFileForWork)

	case fileTypePublicStatic:
		decision.processPublicFiles = true
		decision.publicStaticChangedFilePath = classifiedEventForWork.event.Name

	case fileTypePrivateStatic:
		decision.processPrivateFiles = true
		decision.privateStaticChangedFilePath = classifiedEventForWork.event.Name

	case fileTypeOther:
		if watchedFileForWork != nil {
			decision.compileGo = watchedFileForWork.RecompileGoBinary
			decision.restartApp = watchedFileForWork.RestartApp || watchedFileForWork.RecompileGoBinary
		}
	}

	return decision
}

func (w *workSet) applyImplicitWorkDecision(
	decision implicitWorkDecision,
) {
	if decision.preferRevalidate {
		w.preferRevalidate = true
	}
	if decision.compileGo {
		w.build.compileGo = true
	}
	if decision.buildCriticalCSS {
		w.build.buildCriticalCSS = true
	}
	if decision.buildNormalCSS {
		w.build.buildNormalCSS = true
	}
	if decision.processPublicFiles {
		w.build.processPublicFiles = true
		w.build.addPublicStaticChangedFilePath(decision.publicStaticChangedFilePath)
	}
	if decision.processPrivateFiles {
		w.build.processPrivateFiles = true
		w.build.addPrivateStaticChangedFilePath(decision.privateStaticChangedFilePath)
	}
	if decision.restartApp {
		w.restart.restartApp = true
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
	browserPhaseResolutionForWork := deriveBrowserPhaseResolutionForWorkSet(
		w.build,
		w.restart,
		w.preferRevalidate,
		usingVite,
	)
	w.requestBrowserAction(browserPhaseResolutionForWork.action)
	if browserPhaseResolutionForWork.applyWaitFlags {
		w.browser.waitForApp = browserPhaseResolutionForWork.waitForApp
		w.browser.waitForVite = browserPhaseResolutionForWork.waitForVite
	}
}

func (w *workSet) requestBrowserAction(action browserPhaseAction) {
	if action > w.browser.action {
		w.browser.action = action
	}
}

func deriveBrowserPhaseResolutionForWorkSet(
	buildDecision buildPhaseDecision,
	restartDecision restartPhaseDecision,
	preferRevalidate bool,
	usingVite bool,
) browserPhaseResolution {
	if shouldUseRestartBrowserResolution(restartDecision) {
		return browserPhaseResolution{
			action:         browserPhaseActionHardReload,
			applyWaitFlags: true,
			waitForApp:     true,
			waitForVite:    usingVite,
		}
	}

	if shouldUseRevalidateBrowserResolution(preferRevalidate) {
		return browserPhaseResolution{
			action:         browserPhaseActionRevalidate,
			applyWaitFlags: true,
			waitForApp:     true,
			waitForVite:    usingVite,
		}
	}

	if isCSSOnlyBuildWorkForBrowserPhase(buildDecision) {
		return browserPhaseResolution{
			action: browserPhaseActionHotReloadCSS,
		}
	}

	if buildDecision.processPublicFiles {
		return browserPhaseResolution{
			action: browserPhaseActionInvalidateVite,
		}
	}

	if buildDecision.processPrivateFiles || buildDecision.buildCriticalCSS || buildDecision.buildNormalCSS {
		return browserPhaseResolution{
			action:         browserPhaseActionHardReload,
			applyWaitFlags: true,
			waitForApp:     true,
			waitForVite:    usingVite,
		}
	}

	return browserPhaseResolution{
		action: browserPhaseActionNone,
	}
}

func shouldUseRestartBrowserResolution(
	restartDecision restartPhaseDecision,
) bool {
	return restartDecision.restartApp
}

func shouldUseRevalidateBrowserResolution(
	preferRevalidate bool,
) bool {
	// User preference takes precedence over automatic optimizations.
	return preferRevalidate
}

func isCSSOnlyBuildWorkForBrowserPhase(
	buildDecision buildPhaseDecision,
) bool {
	cssWork := buildDecision.buildCriticalCSS || buildDecision.buildNormalCSS
	return cssWork &&
		!buildDecision.processPublicFiles &&
		!buildDecision.processPrivateFiles
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

type browserPhaseExecutionCategory int

const (
	browserPhaseExecutionCategoryNone browserPhaseExecutionCategory = iota
	browserPhaseExecutionCategoryReload
	browserPhaseExecutionCategoryHotReloadCSS
)

func shouldAttemptViteInvalidateForBrowserDecision(
	browserDecision browserPhaseDecision,
	usingVite bool,
) bool {
	return browserDecision.action == browserPhaseActionInvalidateVite && usingVite
}

func resolveBrowserDecisionAfterInvalidateViteFallback(
	browserDecision browserPhaseDecision,
	usingVite bool,
) browserPhaseDecision {
	if browserDecision.action != browserPhaseActionInvalidateVite {
		return browserDecision
	}

	fallbackDecision := planInvalidateViteFallbackBrowserDecision(usingVite)
	browserDecision.action = fallbackDecision.action
	browserDecision.waitForApp = fallbackDecision.waitForApp
	browserDecision.waitForVite = fallbackDecision.waitForVite
	return browserDecision
}

func deriveBrowserPhaseExecutionCategory(
	action browserPhaseAction,
) browserPhaseExecutionCategory {
	switch action {
	case browserPhaseActionHardReload, browserPhaseActionRevalidate:
		return browserPhaseExecutionCategoryReload
	case browserPhaseActionHotReloadCSS:
		return browserPhaseExecutionCategoryHotReloadCSS
	default:
		return browserPhaseExecutionCategoryNone
	}
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

func deriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
	eventsWithHooks []eventWithHooks,
) eventExecutionPlanBehavioralDecision {
	return eventExecutionPlanBehavioralDecision{
		showRebuildingOverlay: shouldShowRebuildingOverlayForEventsWithHooks(eventsWithHooks),
		appStopStrategy:       resolveAppStopStrategy(eventsWithHooks),
		runImplicitBuild:      shouldRunImplicitBuildForEvents(eventsWithHooks),
	}
}

func shouldShowRebuildingOverlayForEventsWithHooks(
	eventsWithHooks []eventWithHooks,
) bool {
	if len(eventsWithHooks) == 0 {
		return false
	}

	classifiedEvents := make([]classifiedEvent, 0, len(eventsWithHooks))
	for _, eventWithHooksForOverlay := range eventsWithHooks {
		classifiedEvents = append(classifiedEvents, eventWithHooksForOverlay.classified)
	}
	return shouldShowRebuildingOverlay(classifiedEvents)
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
		return ""
	}

	if _, alreadyExists := mergedEventOpsByPath[normalizedEventPath]; alreadyExists {
		return normalizedEventPath
	}

	if !isAbsolutePathForWatcherEventDeduplication(normalizedEventPath) {
		return ""
	}

	return index.resolveExistingPathKeyFromCanonicalAndMissingAliasProbes(
		normalizedEventPath,
	)
}

func (index *watcherEventDeduplicationIndex) recordPathKey(pathKey string) {
	if !isAbsolutePathForWatcherEventDeduplication(pathKey) {
		return
	}

	index.recordPathKeyForCanonicalAndMissingAliasProbes(pathKey)
}

const missingAliasKeyResolutionEmptySentinel = "\x00"

func (index *watcherEventDeduplicationIndex) resolveCanonicalPathForAbsolutePath(
	absolutePath string,
) string {
	if !isAbsolutePathForWatcherEventDeduplication(absolutePath) {
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
	if !isAbsolutePathForWatcherEventDeduplication(absolutePath) {
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

func (index *watcherEventDeduplicationIndex) resolveExistingPathKeyFromCanonicalAndMissingAliasProbes(
	absolutePath string,
) string {
	if !isAbsolutePathForWatcherEventDeduplication(absolutePath) {
		return ""
	}

	if existingPathKey := index.resolveExistingPathKeyFromCanonicalPathProbe(absolutePath); existingPathKey != "" {
		return existingPathKey
	}

	return index.resolveExistingPathKeyFromMissingAliasProbe(absolutePath)
}

func (index *watcherEventDeduplicationIndex) resolveExistingPathKeyFromCanonicalPathProbe(
	absolutePath string,
) string {
	canonicalPath := index.resolveCanonicalPathForAbsolutePath(absolutePath)
	return resolveExistingPathKeyFromProbeLookup(
		index.canonicalPathToPathKey,
		canonicalPath,
	)
}

func (index *watcherEventDeduplicationIndex) resolveExistingPathKeyFromMissingAliasProbe(
	absolutePath string,
) string {
	missingAliasKey := index.resolveMissingAliasKeyForAbsolutePath(absolutePath)
	return resolveExistingPathKeyFromProbeLookup(
		index.missingFileAliasKeyToPathKey,
		missingAliasKey,
	)
}

func (index *watcherEventDeduplicationIndex) recordPathKeyForCanonicalAndMissingAliasProbes(
	pathKey string,
) {
	if !isAbsolutePathForWatcherEventDeduplication(pathKey) {
		return
	}

	canonicalPath := index.resolveCanonicalPathForAbsolutePath(pathKey)
	recordPathKeyForProbeLookupIfAbsent(
		index.canonicalPathToPathKey,
		canonicalPath,
		pathKey,
	)

	missingAliasKey := index.resolveMissingAliasKeyForAbsolutePath(pathKey)
	recordPathKeyForProbeLookupIfAbsent(
		index.missingFileAliasKeyToPathKey,
		missingAliasKey,
		pathKey,
	)
}

func resolveExistingPathKeyFromProbeLookup(
	pathKeyByProbe map[string]string,
	probeKey string,
) string {
	if pathKeyByProbe == nil || probeKey == "" {
		return ""
	}
	return pathKeyByProbe[probeKey]
}

func recordPathKeyForProbeLookupIfAbsent(
	pathKeyByProbe map[string]string,
	probeKey string,
	pathKey string,
) {
	if pathKeyByProbe == nil || probeKey == "" || pathKey == "" {
		return
	}
	if _, exists := pathKeyByProbe[probeKey]; exists {
		return
	}
	pathKeyByProbe[probeKey] = pathKey
}

func normalizeWatcherEventPathForDeduplication(
	eventPath string,
) string {
	cleanedEventPath := pathnorm.TrimAndCleanPath(eventPath)
	if cleanedEventPath == "" {
		return ""
	}

	if isAbsolutePathForWatcherEventDeduplication(cleanedEventPath) {
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
	if !isAbsolutePathForWatcherEventDeduplication(absolutePath) {
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

func isAbsolutePathForWatcherEventDeduplication(path string) bool {
	return path != "" && filepath.IsAbs(path)
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

func buildWatcherEventLogPayloadsForEventsWithHooks(
	eventsWithHooks []eventWithHooks,
) []watcherEventLogPayload {
	if len(eventsWithHooks) == 0 {
		return nil
	}

	watcherEventLogPayloads := make(
		[]watcherEventLogPayload,
		0,
		len(eventsWithHooks),
	)
	for _, eventWithHooksForLogging := range eventsWithHooks {
		watcherEventLogPayloads = append(
			watcherEventLogPayloads,
			watcherEventLogPayload{
				operation: eventWithHooksForLogging.classified.event.Op.String(),
				filePath:  eventWithHooksForLogging.classified.event.Name,
			},
		)
	}
	return watcherEventLogPayloads
}

func buildEventHooksForProcessing(
	classifiedEvents []classifiedEvent,
) []eventWithHooks {
	if len(classifiedEvents) == 0 {
		return nil
	}

	normalizedChangedFilePathsByWatchedPattern := buildNormalizedChangedFilePathsByWatchedPatternForHookContexts(
		classifiedEvents,
	)
	watchedPatternOccurrenceCount := buildWatchedPatternOccurrenceCountForHookContexts(
		classifiedEvents,
	)
	skipDuplicateHooksByClassifiedEventIndex := buildSkipDuplicateHooksByClassifiedEventIndex(
		classifiedEvents,
	)

	eventsWithHooks := make([]eventWithHooks, 0, len(classifiedEvents))
	for eventIndex, classifiedEventForProcessing := range classifiedEvents {
		normalizedEventPathForHookContext := normalizeHookContextPathShape(
			classifiedEventForProcessing.event.Name,
		)
		changedFilePathsForHookContext := deriveChangedFilePathsForHookContext(
			classifiedEventForProcessing,
			normalizedChangedFilePathsByWatchedPattern,
			watchedPatternOccurrenceCount,
			normalizedEventPathForHookContext,
		)
		eventWithHooksForProcessing := buildEventWithHooksForClassifiedEvent(
			classifiedEventForProcessing,
			skipDuplicateHooksByClassifiedEventIndex[eventIndex],
			changedFilePathsForHookContext,
		)
		eventsWithHooks = append(eventsWithHooks, eventWithHooksForProcessing)
	}

	return eventsWithHooks
}

func buildNormalizedChangedFilePathsByWatchedPatternForHookContexts(
	classifiedEvents []classifiedEvent,
) map[string][]string {
	normalizedChangedFilePathsByWatchedPattern := make(map[string][]string)
	seenNormalizedChangedFilePathsByWatchedPattern := make(map[string]map[string]struct{})

	for _, classifiedEventForProcessing := range classifiedEvents {
		watchedFileForProcessing := classifiedEventForProcessing.watchedFile
		if watchedFileForProcessing == nil {
			continue
		}
		normalizedEventPathForHookContext := normalizeHookContextPathShape(
			classifiedEventForProcessing.event.Name,
		)
		recordChangedPathForWatchedPatternIfNew(
			watchedFileForProcessing.Pattern,
			normalizedEventPathForHookContext,
			normalizedChangedFilePathsByWatchedPattern,
			seenNormalizedChangedFilePathsByWatchedPattern,
		)
	}

	return normalizedChangedFilePathsByWatchedPattern
}

func buildSkipDuplicateHooksByClassifiedEventIndex(
	classifiedEvents []classifiedEvent,
) []bool {
	skipDuplicateHooksByClassifiedEventIndex := make([]bool, len(classifiedEvents))
	handledWatchedPatterns := make(map[string]struct{})

	for eventIndex, classifiedEventForProcessing := range classifiedEvents {
		watchedFileForProcessing := classifiedEventForProcessing.watchedFile
		if watchedFileForProcessing == nil {
			continue
		}

		watchedPattern := watchedFileForProcessing.Pattern
		if _, alreadyHandled := handledWatchedPatterns[watchedPattern]; alreadyHandled {
			skipDuplicateHooksByClassifiedEventIndex[eventIndex] = true
			continue
		}
		handledWatchedPatterns[watchedPattern] = struct{}{}
	}

	return skipDuplicateHooksByClassifiedEventIndex
}

func buildWatchedPatternOccurrenceCountForHookContexts(
	classifiedEvents []classifiedEvent,
) map[string]int {
	watchedPatternOccurrenceCount := make(map[string]int)
	for _, classifiedEventForProcessing := range classifiedEvents {
		watchedFileForProcessing := classifiedEventForProcessing.watchedFile
		if watchedFileForProcessing == nil {
			continue
		}
		watchedPatternOccurrenceCount[watchedFileForProcessing.Pattern]++
	}
	return watchedPatternOccurrenceCount
}

func deriveChangedFilePathsForHookContext(
	classifiedEventForProcessing classifiedEvent,
	normalizedChangedFilePathsByWatchedPattern map[string][]string,
	watchedPatternOccurrenceCount map[string]int,
	normalizedEventPathForHookContext string,
) []string {
	watchedFileForProcessing := classifiedEventForProcessing.watchedFile
	if watchedFileForProcessing == nil {
		return []string{normalizedEventPathForHookContext}
	}

	watchedPattern := watchedFileForProcessing.Pattern
	normalizedChangedFilePathsForWatchedPattern := normalizedChangedFilePathsByWatchedPattern[watchedPattern]
	if len(normalizedChangedFilePathsForWatchedPattern) == 0 {
		return []string{normalizedEventPathForHookContext}
	}

	if watchedPatternOccurrenceCount[watchedPattern] <= 1 {
		return normalizedChangedFilePathsForWatchedPattern
	}

	return append(
		[]string(nil),
		normalizedChangedFilePathsForWatchedPattern...,
	)
}

func buildEventWithHooksForClassifiedEvent(
	classifiedEventForProcessing classifiedEvent,
	skipDuplicateHooks bool,
	changedFilePathsForHookContext []string,
) eventWithHooks {
	watchedFileForProcessing := classifiedEventForProcessing.watchedFile
	if watchedFileForProcessing == nil {
		watchedFileForProcessing = &wave.WatchedFile{}
	}
	if watchedFileForProcessing.SortedHooks == nil {
		watchedFileForProcessing.Sort()
	}
	if watchedFileForProcessing.SortedHooks == nil {
		watchedFileForProcessing.SortedHooks = &wave.SortedHooks{}
	}

	normalizedEventPathForHookContext := normalizeHookContextPathShape(
		classifiedEventForProcessing.event.Name,
	)
	if len(changedFilePathsForHookContext) == 0 {
		changedFilePathsForHookContext = []string{normalizedEventPathForHookContext}
	}
	eventNeedsHardReload := classifiedEventForProcessing.fileType == fileTypeGo ||
		needsHardReload(watchedFileForProcessing)

	return eventWithHooks{
		classified:         classifiedEventForProcessing,
		hooks:              watchedFileForProcessing.SortedHooks,
		runOnChangeOnly:    watchedFileForProcessing.RunOnChangeOnly,
		needsHardReload:    eventNeedsHardReload,
		skipDuplicateHooks: skipDuplicateHooks,
		hookCtx: &wave.HookContext{
			FilePath:           normalizedEventPathForHookContext,
			ChangedFilePaths:   changedFilePathsForHookContext,
			AppStoppedForBatch: false,
		},
	}
}

func normalizeHookContextPathShape(path string) string {
	return pathnorm.TrimAndCleanPath(path)
}

func recordChangedPathForWatchedPatternIfNew(
	watchedPattern string,
	normalizedChangedPath string,
	changedFilePathsByWatchedPattern map[string][]string,
	seenChangedFilePathsByWatchedPattern map[string]map[string]struct{},
) {
	if changedFilePathsByWatchedPattern == nil || seenChangedFilePathsByWatchedPattern == nil {
		return
	}

	seenChangedPathsForWatchedPattern, hasSeenSet := seenChangedFilePathsByWatchedPattern[watchedPattern]
	if !hasSeenSet || seenChangedPathsForWatchedPattern == nil {
		seenChangedPathsForWatchedPattern = make(map[string]struct{})
		seenChangedFilePathsByWatchedPattern[watchedPattern] = seenChangedPathsForWatchedPattern
	}

	if _, alreadyTracked := seenChangedPathsForWatchedPattern[normalizedChangedPath]; alreadyTracked {
		return
	}
	seenChangedPathsForWatchedPattern[normalizedChangedPath] = struct{}{}
	changedFilePathsByWatchedPattern[watchedPattern] = append(
		changedFilePathsByWatchedPattern[watchedPattern],
		normalizedChangedPath,
	)
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

type implicitBuildExecutionDecision struct {
	shouldRunImplicitBuild    bool
	skipImplicitBuildLogEntry string
}

type staticFileProcessingExecutionMode int

const (
	staticFileProcessingExecutionModeNone staticFileProcessingExecutionMode = iota
	staticFileProcessingExecutionModeFullScan
	staticFileProcessingExecutionModeChangedPaths
)

type staticFileProcessingExecutionDecision struct {
	mode             staticFileProcessingExecutionMode
	changedFilePaths []string
}

func (d staticFileProcessingExecutionDecision) shouldProcess() bool {
	return d.mode != staticFileProcessingExecutionModeNone
}

type buildPhaseExecutionDecision struct {
	compileGo                   bool
	publicStaticProcessing      staticFileProcessingExecutionDecision
	privateStaticProcessing     staticFileProcessingExecutionDecision
	buildCriticalCSS            bool
	buildNormalCSS              bool
	writeFrameworkPublicFileMap bool
}

func deriveStaticFileProcessingExecutionDecision(
	shouldProcess bool,
	changedFilePaths []string,
) staticFileProcessingExecutionDecision {
	if !shouldProcess {
		return staticFileProcessingExecutionDecision{}
	}
	if len(changedFilePaths) == 0 {
		return staticFileProcessingExecutionDecision{
			mode: staticFileProcessingExecutionModeFullScan,
		}
	}
	return staticFileProcessingExecutionDecision{
		mode:             staticFileProcessingExecutionModeChangedPaths,
		changedFilePaths: append([]string(nil), changedFilePaths...),
	}
}

func shouldWriteFrameworkPublicFileMapTSForBuildDecision(
	shouldProcessPublicStaticFiles bool,
	frameworkPublicFileMapOutDir string,
) bool {
	return shouldProcessPublicStaticFiles && frameworkPublicFileMapOutDir != ""
}

func deriveBuildPhaseExecutionDecision(
	buildDecision buildPhaseDecision,
	frameworkPublicFileMapOutDir string,
) buildPhaseExecutionDecision {
	return buildPhaseExecutionDecision{
		compileGo: buildDecision.compileGo,
		publicStaticProcessing: deriveStaticFileProcessingExecutionDecision(
			buildDecision.processPublicFiles,
			buildDecision.publicStaticChangedFilePaths,
		),
		privateStaticProcessing: deriveStaticFileProcessingExecutionDecision(
			buildDecision.processPrivateFiles,
			buildDecision.privateStaticChangedFilePaths,
		),
		buildCriticalCSS: buildDecision.buildCriticalCSS,
		buildNormalCSS:   buildDecision.buildNormalCSS,
		writeFrameworkPublicFileMap: shouldWriteFrameworkPublicFileMapTSForBuildDecision(
			buildDecision.processPublicFiles,
			frameworkPublicFileMapOutDir,
		),
	}
}

func shouldExecuteAnyFileProcessingForBuildDecision(
	executionDecision buildPhaseExecutionDecision,
) bool {
	return executionDecision.publicStaticProcessing.shouldProcess() ||
		executionDecision.privateStaticProcessing.shouldProcess() ||
		executionDecision.buildCriticalCSS ||
		executionDecision.buildNormalCSS
}

type hookStageResult struct {
	actions             []wave.RefreshAction
	refreshActionResult refreshActionApplicationResult
}

type hookStageContinuationDecision struct {
	shouldContinue      bool
	restartActionResult refreshActionApplicationResult
}

func deriveImplicitBuildExecutionDecision(
	shouldRunImplicitBuild bool,
	eventCount int,
) implicitBuildExecutionDecision {
	if shouldRunImplicitBuild {
		return implicitBuildExecutionDecision{
			shouldRunImplicitBuild: true,
		}
	}

	if eventCount == 1 {
		return implicitBuildExecutionDecision{
			skipImplicitBuildLogEntry: "RunOnChangeOnly: skipping implicit build phase",
		}
	}

	return implicitBuildExecutionDecision{
		skipImplicitBuildLogEntry: "All events are RunOnChangeOnly, skipping implicit build phase",
	}
}

func shouldShortCircuitPipelineForHookStageResult(
	hookStageResultForCheck hookStageResult,
) bool {
	return hookStageResultForCheck.refreshActionResult.restartRequested
}

func deriveHookStageContinuationDecision(
	hookStageResultForContinuation hookStageResult,
) hookStageContinuationDecision {
	if shouldShortCircuitPipelineForHookStageResult(
		hookStageResultForContinuation,
	) {
		return hookStageContinuationDecision{
			restartActionResult: hookStageResultForContinuation.refreshActionResult,
		}
	}
	return hookStageContinuationDecision{
		shouldContinue: true,
	}
}

func (s *server) continuePipelineAfterHookStageOrTriggerRestart(
	hookStageResultForContinuation hookStageResult,
) bool {
	continuationDecision := deriveHookStageContinuationDecision(
		hookStageResultForContinuation,
	)
	if continuationDecision.shouldContinue {
		return true
	}

	s.triggerRestartFromRefreshActions(continuationDecision.restartActionResult)
	return false
}

func applyHookStageActionsToWorkSet(
	hookStageActions []wave.RefreshAction,
	work *workSet,
) hookStageResult {
	hookStageResultForWork := hookStageResult{
		actions: append([]wave.RefreshAction(nil), hookStageActions...),
	}
	if work == nil {
		return hookStageResultForWork
	}

	hookStageResultForWork.refreshActionResult = work.applyRefreshActions(hookStageActions)
	return hookStageResultForWork
}

func runAndApplyHookStageActionsToWorkSet(
	runHookStageActions func() []wave.RefreshAction,
	work *workSet,
) hookStageResult {
	if runHookStageActions == nil {
		return applyHookStageActionsToWorkSet(nil, work)
	}
	return applyHookStageActionsToWorkSet(runHookStageActions(), work)
}

func shouldStartAppAfterImplicitBuild(
	shouldRunImplicitBuild bool,
	restart restartPhaseDecision,
) bool {
	return shouldRunImplicitBuild && restart.restartApp
}

func shouldExecuteBrowserPhaseAfterHookStageResults(
	hookStageResults ...hookStageResult,
) bool {
	for _, hookStageResultForCheck := range hookStageResults {
		if shouldShortCircuitPipelineForHookStageResult(hookStageResultForCheck) {
			return false
		}
	}
	return true
}

func deriveEventsWithHooksForExecution(
	eventsWithHooks []eventWithHooks,
	appStopStrategyForExecution appStopStrategy,
) []eventWithHooks {
	if len(eventsWithHooks) == 0 {
		return nil
	}
	if appStopStrategyForExecution != appStopStrategyBatchHardReload {
		return eventsWithHooks
	}

	executionEventsWithHooks := make([]eventWithHooks, len(eventsWithHooks))
	copy(executionEventsWithHooks, eventsWithHooks)
	for eventIndex := range executionEventsWithHooks {
		executionEventWithHooks := executionEventsWithHooks[eventIndex]
		if executionEventWithHooks.hookCtx == nil {
			continue
		}

		executionHookContext := *executionEventWithHooks.hookCtx
		executionHookContext.AppStoppedForBatch = true
		executionEventWithHooks.hookCtx = &executionHookContext
		executionEventsWithHooks[eventIndex] = executionEventWithHooks
	}

	return executionEventsWithHooks
}

func (s *server) executeEventExecutionPlan(
	eventsWithHooks []eventWithHooks,
	behavioralDecision eventExecutionPlanBehavioralDecision,
	work *workSet,
	watcher *Watcher,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	eventsWithHooksForExecution := deriveEventsWithHooksForExecution(
		eventsWithHooks,
		behavioralDecision.appStopStrategy,
	)

	switch behavioralDecision.appStopStrategy {
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

	case appStopStrategyNone:
	}

	s.processEventsWithDeterministicPipeline(
		behavioralDecision,
		work,
		watcher,
		eventsWithHooksForExecution,
	)
}

func (s *server) processEventsWithDeterministicPipeline(
	behavioralDecision eventExecutionPlanBehavioralDecision,
	work *workSet,
	watcher *Watcher,
	eventsWithHooks []eventWithHooks,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	s.fireNoWaitHooksForEvents(eventsWithHooks, watcher)

	preHookStageResult := runAndApplyHookStageActionsToWorkSet(
		func() []wave.RefreshAction {
			return s.runPreHooksForEvents(eventsWithHooks, work, watcher)
		},
		work,
	)
	if !s.continuePipelineAfterHookStageOrTriggerRestart(
		preHookStageResult,
	) {
		return
	}

	implicitBuildDecision := deriveImplicitBuildExecutionDecision(
		behavioralDecision.runImplicitBuild,
		len(eventsWithHooks),
	)
	if !implicitBuildDecision.shouldRunImplicitBuild {
		s.log.Info(implicitBuildDecision.skipImplicitBuildLogEntry)
	} else {
		work.resolve(s.cfg.UsingVite())
	}

	var buildAndConcurrentHooksGroup errgroup.Group
	if implicitBuildDecision.shouldRunImplicitBuild {
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

	concurrentHookStageResult := applyHookStageActionsToWorkSet(
		concurrentActions,
		work,
	)
	if !s.continuePipelineAfterHookStageOrTriggerRestart(
		concurrentHookStageResult,
	) {
		return
	}

	postHookStageResult := runAndApplyHookStageActionsToWorkSet(
		func() []wave.RefreshAction {
			return s.runPostHooksForEvents(eventsWithHooks, watcher)
		},
		work,
	)
	if !s.continuePipelineAfterHookStageOrTriggerRestart(
		postHookStageResult,
	) {
		return
	}

	if shouldStartAppAfterImplicitBuild(
		implicitBuildDecision.shouldRunImplicitBuild,
		work.restart,
	) {
		s.log.Info("Restarting app")
		s.startApp()
	}

	if shouldExecuteBrowserPhaseAfterHookStageResults(
		preHookStageResult,
		concurrentHookStageResult,
		postHookStageResult,
	) {
		s.executeBrowserPhase(work)
	}
}

func forEachEventWithHooksEligibleForHookExecution(
	eventsWithHooks []eventWithHooks,
	onEligibleEvent func(eventIndex int, eligibleEvent eventWithHooks),
) {
	if onEligibleEvent == nil {
		return
	}
	for eventIndex, eventWithHooksForExecution := range eventsWithHooks {
		if eventWithHooksForExecution.skipDuplicateHooks {
			continue
		}
		onEligibleEvent(eventIndex, eventWithHooksForExecution)
	}
}

func (s *server) runSequentialHookStageForEligibleEvents(
	eventsWithHooks []eventWithHooks,
	watcher *Watcher,
	runHooksForEvent func(eventWithHooks, *Watcher) ([]wave.RefreshAction, error),
	hookExecutionFailureLogMessage string,
) []wave.RefreshAction {
	if runHooksForEvent == nil {
		return nil
	}

	allStageActions := make([]wave.RefreshAction, 0)
	forEachEventWithHooksEligibleForHookExecution(
		eventsWithHooks,
		func(_ int, eventWithHooksForStage eventWithHooks) {
			stageActions, err := runHooksForEvent(eventWithHooksForStage, watcher)
			if err != nil {
				s.log.Error(hookExecutionFailureLogMessage, "error", err)
			}
			allStageActions = append(allStageActions, stageActions...)
		},
	)
	return allStageActions
}

func (s *server) fireNoWaitHooksForEvents(
	eventsWithHooks []eventWithHooks,
	watcher *Watcher,
) {
	forEachEventWithHooksEligibleForHookExecution(
		eventsWithHooks,
		func(_ int, eventWithHooksForFire eventWithHooks) {
			s.fireNoWaitHooks(eventWithHooksForFire, watcher)
		},
	)
}

func (s *server) runPreHooksForEvents(
	eventsWithHooks []eventWithHooks,
	work *workSet,
	watcher *Watcher,
) []wave.RefreshAction {
	for _, eventWithHooksForPre := range eventsWithHooks {
		work.addImplicitWork(eventWithHooksForPre.classified)
	}
	return s.runSequentialHookStageForEligibleEvents(
		eventsWithHooks,
		watcher,
		s.runPreHooks,
		"Pre-hook execution failed",
	)
}

func (s *server) runConcurrentHooksForEvents(
	eventsWithHooks []eventWithHooks,
	watcher *Watcher,
) []wave.RefreshAction {
	actionsByEventIndex := make([][]wave.RefreshAction, len(eventsWithHooks))
	var concurrentHooksGroup errgroup.Group

	forEachEventWithHooksEligibleForHookExecution(
		eventsWithHooks,
		func(eventIndex int, eventWithHooksForConcurrent eventWithHooks) {
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
		},
	)

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
	return s.runSequentialHookStageForEligibleEvents(
		eventsWithHooks,
		watcher,
		s.runPostHooks,
		"Post-hook execution failed",
	)
}

func (s *server) fireNoWaitHooks(ewh eventWithHooks, watcher *Watcher) {
	hooksForExecution := deriveExecutableHooksForStage(
		watcher,
		ewh.classified.event.Name,
		ewh.runOnChangeOnly,
		false,
		ewh.hooks.ConcurrentNoWait,
	)
	for _, hookForExecution := range hooksForExecution {
		if hookForExecution.Callback != nil {
			go func(cb func(*wave.HookContext) (*wave.RefreshAction, error), ctx *wave.HookContext) {
				if _, err := cb(ctx); err != nil {
					s.log.Warn("concurrent-no-wait callback failed", "error", err)
				}
			}(hookForExecution.Callback, ewh.hookCtx)
		}
		resolvedCommand := s.resolveHookCommand(hookForExecution)
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

	preHooksForExecution := deriveExecutableHooksForStage(
		watcher,
		ewh.classified.event.Name,
		ewh.runOnChangeOnly,
		false,
		ewh.hooks.Pre,
	)
	for _, preHookForExecution := range preHooksForExecution {
		action, err := s.executeHook(preHookForExecution, ewh.hookCtx)
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
	concurrentHooksToRun := deriveExecutableHooksForStage(
		watcher,
		ewh.classified.event.Name,
		ewh.runOnChangeOnly,
		true,
		ewh.hooks.Concurrent,
	)
	if len(concurrentHooksToRun) == 0 {
		return nil, nil
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

	postHooksForExecution := deriveExecutableHooksForStage(
		watcher,
		ewh.classified.event.Name,
		ewh.runOnChangeOnly,
		true,
		ewh.hooks.Post,
	)
	for _, postHookForExecution := range postHooksForExecution {
		action, err := s.executeHook(postHookForExecution, ewh.hookCtx)
		if action != nil {
			actions = append(actions, *action)
		}
		if err != nil {
			return actions, err
		}
	}

	return actions, nil
}

func deriveExecutableHooksForStage(
	watcher *Watcher,
	eventPath string,
	isRunOnChangeOnly bool,
	shouldApplyRunOnChangeOnlyRules bool,
	stageHooks []wave.OnChangeHook,
) []wave.OnChangeHook {
	if len(stageHooks) == 0 {
		return nil
	}

	hooksForExecution := make([]wave.OnChangeHook, 0, len(stageHooks))
	for _, stageHook := range stageHooks {
		hookForExecution, shouldRunHook := resolveHookForStageExecution(
			watcher,
			eventPath,
			isRunOnChangeOnly,
			shouldApplyRunOnChangeOnlyRules,
			stageHook,
		)
		if !shouldRunHook {
			continue
		}
		hooksForExecution = append(hooksForExecution, hookForExecution)
	}

	return hooksForExecution
}

func resolveHookForStageExecution(
	watcher *Watcher,
	eventPath string,
	isRunOnChangeOnly bool,
	shouldApplyRunOnChangeOnlyRules bool,
	hook wave.OnChangeHook,
) (wave.OnChangeHook, bool) {
	if watcher.IsIgnored(eventPath, hook.Exclude) {
		return wave.OnChangeHook{}, false
	}
	if !shouldApplyRunOnChangeOnlyRules {
		return hook, true
	}
	return prepareHookForExecutionWithRunOnChangeOnlyRules(
		isRunOnChangeOnly,
		hook,
	)
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

	buildExecutionDecision := deriveBuildPhaseExecutionDecision(
		work.build,
		s.cfg.FrameworkPublicFileMapOutDir,
	)

	var g errgroup.Group

	if buildExecutionDecision.compileGo {
		g.Go(func() error {
			if err := builder.CompileGoOnly(true); err != nil {
				s.log.Error("Go compilation failed", "error", err)
				return err
			}
			return nil
		})
	}

	if shouldExecuteAnyFileProcessingForBuildDecision(buildExecutionDecision) {
		g.Go(func() error {
			if err := s.executePublicStaticProcessingForBuildPhase(
				builder,
				buildExecutionDecision,
			); err != nil {
				return err
			}

			var innerG errgroup.Group

			if buildExecutionDecision.privateStaticProcessing.shouldProcess() {
				innerG.Go(func() error {
					return s.executePrivateStaticProcessingForBuildPhase(
						builder,
						buildExecutionDecision.privateStaticProcessing,
					)
				})
			}

			if buildExecutionDecision.buildCriticalCSS {
				innerG.Go(func() error {
					if err := builder.BuildCriticalCSS(true); err != nil {
						s.log.Error("Critical CSS build failed", "error", err)
						return err
					}
					return nil
				})
			}

			if buildExecutionDecision.buildNormalCSS {
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

func (s *server) executePublicStaticProcessingForBuildPhase(
	builder *Builder,
	buildExecutionDecision buildPhaseExecutionDecision,
) error {
	if !buildExecutionDecision.publicStaticProcessing.shouldProcess() {
		return nil
	}

	publicStaticProcessingError := executeStaticFileProcessingForBuildPhase(
		builder.ProcessPublicFilesOnly,
		builder.processPublicFilesOnlyForChangedPaths,
		buildExecutionDecision.publicStaticProcessing,
	)
	if publicStaticProcessingError != nil {
		s.log.Error("Public files processing failed", "error", publicStaticProcessingError)
		return publicStaticProcessingError
	}

	if buildExecutionDecision.writeFrameworkPublicFileMap {
		if err := builder.WritePublicFileMapTS(s.cfg.FrameworkPublicFileMapOutDir); err != nil {
			s.log.Error("Write public file map TS failed", "error", err)
			return err
		}
	}

	return nil
}

func (s *server) executePrivateStaticProcessingForBuildPhase(
	builder *Builder,
	privateStaticProcessingDecision staticFileProcessingExecutionDecision,
) error {
	privateStaticProcessingError := executeStaticFileProcessingForBuildPhase(
		builder.ProcessPrivateFilesOnly,
		builder.processPrivateFilesOnlyForChangedPaths,
		privateStaticProcessingDecision,
	)
	if privateStaticProcessingError != nil {
		s.log.Error("Private files processing failed", "error", privateStaticProcessingError)
		return privateStaticProcessingError
	}
	return nil
}

func executeStaticFileProcessingForBuildPhase(
	executeFullStaticProcessing func() error,
	executeChangedPathsStaticProcessing func([]string) error,
	staticProcessingDecision staticFileProcessingExecutionDecision,
) error {
	switch staticProcessingDecision.mode {
	case staticFileProcessingExecutionModeNone:
		return nil

	case staticFileProcessingExecutionModeChangedPaths:
		if executeChangedPathsStaticProcessing == nil {
			return nil
		}
		return executeChangedPathsStaticProcessing(staticProcessingDecision.changedFilePaths)

	case staticFileProcessingExecutionModeFullScan:
		if executeFullStaticProcessing == nil {
			return nil
		}
		return executeFullStaticProcessing()

	default:
		return nil
	}
}

func (s *server) executeBrowserPhase(work *workSet) {
	if !s.cfg.UsingBrowser() {
		return
	}

	builder := s.getBuilder()
	browserDecisionForExecution := work.browser
	if browserDecisionForExecution.action == browserPhaseActionInvalidateVite {
		if shouldAttemptViteInvalidateForBrowserDecision(
			browserDecisionForExecution,
			s.cfg.UsingVite(),
		) {
			if err := s.callViteFilemapInvalidate(); err != nil {
				s.log.Warn("Vite filemap invalidate failed, falling back to reload", "error", err)
			} else {
				return
			}
		}
		browserDecisionForExecution = resolveBrowserDecisionAfterInvalidateViteFallback(
			browserDecisionForExecution,
			s.cfg.UsingVite(),
		)
		work.browser = browserDecisionForExecution
	}

	switch deriveBrowserPhaseExecutionCategory(browserDecisionForExecution.action) {
	case browserPhaseExecutionCategoryReload:
		reloadPlan, hasReloadPlan := planBrowserReloadForAction(
			browserDecisionForExecution.action,
			browserDecisionForExecution,
		)
		if !hasReloadPlan {
			return
		}
		if browserDecisionForExecution.action == browserPhaseActionHardReload {
			s.log.Info("Hard reloading browser")
		} else {
			s.log.Info("Running client-defined revalidate function")
		}
		s.broadcastReload(reloadPlan)
		return

	case browserPhaseExecutionCategoryHotReloadCSS:
		if builder == nil {
			return
		}
		s.executeHotReloadCSSBrowserPhase(builder, work.build)
		return

	case browserPhaseExecutionCategoryNone:
		return
	}
}

func (s *server) executeHotReloadCSSBrowserPhase(
	builder *Builder,
	buildDecision buildPhaseDecision,
) {
	s.log.Info("Hot reloading CSS")

	criticalCSS := ""
	criticalCSSAvailable := false
	if buildDecision.buildCriticalCSS {
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
	if buildDecision.buildNormalCSS {
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
		buildDecision.buildCriticalCSS,
		criticalCSS,
		criticalCSSAvailable,
		buildDecision.buildNormalCSS,
		normalCSSURL,
		normalCSSURLAvailable,
	)
	for _, payload := range payloads {
		s.broadcastReload(reloadOpts{
			payload: payload,
		})
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

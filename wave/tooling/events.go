package tooling

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// workSet collects all work to be done in response to file changes.
type workSet struct {
	// Build phase (determined by file type, union semantics)
	compileGo           bool
	buildCriticalCSS    bool
	buildNormalCSS      bool
	processPublicFiles  bool
	processPrivateFiles bool
	restartApp          bool

	// Browser behavior (determined in resolve based on work + preferences)
	reloadBrowser  bool
	hotReloadCSS   bool
	invalidateVite bool
	revalidate     bool
	waitForApp     bool
	waitForVite    bool
	cycleVite      bool

	// User preferences (collected from watchedFiles)
	preferRevalidate bool
}

type eventExecutionPlan struct {
	classifiedEvents      []classifiedEvent
	eventsWithHooks       []eventWithHooks
	isBatch               bool
	batchNeedsAppStop     bool
	showRebuildingOverlay bool
}

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
		w.restartApp = true
		if action.RecompileGo {
			w.compileGo = true
		}
	}
	if action.ReloadBrowser {
		w.reloadBrowser = true
	}
	if action.WaitForApp {
		w.waitForApp = true
	}
	if action.WaitForVite {
		w.waitForVite = true
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
		w.compileGo = true
		w.restartApp = true

	case fileTypeCriticalCSS:
		w.buildCriticalCSS = true
		if wf != nil && needsHardReload(wf) {
			w.restartApp = true
		}

	case fileTypeNormalCSS:
		w.buildNormalCSS = true
		if wf != nil && needsHardReload(wf) {
			w.restartApp = true
		}

	case fileTypePublicStatic:
		w.processPublicFiles = true

	case fileTypePrivateStatic:
		w.processPrivateFiles = true

	case fileTypeOther:
		if wf != nil {
			if wf.RecompileGoBinary {
				w.compileGo = true
			}
			if wf.RestartApp || wf.RecompileGoBinary {
				w.restartApp = true
			}
		}
	}
}

// resolve determines browser behavior based on build work and user preferences.
func (w *workSet) resolve(usingVite bool) {
	if w.compileGo {
		w.restartApp = true
	}
	w.determineBrowserBehavior(usingVite)
}

func (w *workSet) determineBrowserBehavior(usingVite bool) {
	if w.restartApp {
		w.reloadBrowser = true
		w.waitForApp = true
		w.waitForVite = usingVite
		return
	}

	// User preference takes precedence over automatic optimizations
	if w.preferRevalidate {
		w.revalidate = true
		w.waitForApp = true
		w.waitForVite = usingVite
		return
	}

	cssWork := w.buildCriticalCSS || w.buildNormalCSS
	cssOnly := cssWork && !w.processPublicFiles && !w.processPrivateFiles

	if cssOnly {
		w.hotReloadCSS = true
		return
	}

	if w.processPublicFiles {
		w.invalidateVite = true
		return
	}

	if w.processPrivateFiles || cssWork {
		w.reloadBrowser = true
		w.waitForApp = true
		w.waitForVite = usingVite
		return
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

	if executionPlan.isBatch {
		s.processBatchedEvents(executionPlan.eventsWithHooks, work, watcher)
	} else {
		s.processSingleEvent(executionPlan.eventsWithHooks[0], work, watcher)
	}

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

	eventsWithHooks, batchNeedsAppStop := buildEventHooksForProcessing(classifiedEvents)

	return eventExecutionPlanningResult{
		plan: &eventExecutionPlan{
			classifiedEvents:      classifiedEvents,
			eventsWithHooks:       eventsWithHooks,
			isBatch:               len(classifiedEvents) > 1,
			batchNeedsAppStop:     batchNeedsAppStop,
			showRebuildingOverlay: shouldShowRebuildingOverlay(classifiedEvents),
		},
	}
}

func deduplicateWatcherEventsByPath(
	events []fsnotify.Event,
) []fsnotify.Event {
	if len(events) == 0 {
		return nil
	}

	mergedEventOpsByPath := make(map[string]fsnotify.Op, len(events))
	for _, event := range events {
		mergedEventOpsByPath[event.Name] |= event.Op
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

func (s *server) classifyWatcherEventsForProcessing(
	events []fsnotify.Event,
	watcher *Watcher,
	builder *Builder,
) ([]classifiedEvent, bool) {
	if len(events) == 0 {
		return nil, false
	}

	classifiedEvents := make([]classifiedEvent, 0, len(events))

	for _, event := range events {
		if s.isConfigFile(event.Name) && (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) {
			return nil, true
		}

		info, _ := os.Stat(event.Name)
		if info != nil && info.IsDir() {
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				_ = watcher.AddDir(event.Name)
			}
			continue
		}

		classifiedEventForProcessing := s.classifyEventWithWatcherAndBuilder(
			event,
			watcher,
			builder,
		)
		if classifiedEventForProcessing.ignored || classifiedEventForProcessing.chmodOnly {
			continue
		}

		classifiedEvents = append(classifiedEvents, classifiedEventForProcessing)
	}

	return classifiedEvents, false
}

func buildEventHooksForProcessing(
	classifiedEvents []classifiedEvent,
) ([]eventWithHooks, bool) {
	if len(classifiedEvents) == 0 {
		return nil, false
	}

	eventsWithHooks := make([]eventWithHooks, 0, len(classifiedEvents))
	handledWatchedPatterns := make(map[string]struct{})
	changedFilePathsByWatchedPattern := make(map[string][]string)
	batchNeedsAppStop := false

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
		if eventNeedsHardReload {
			batchNeedsAppStop = true
		}

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

	return eventsWithHooks, batchNeedsAppStop
}

func shouldShowRebuildingOverlay(
	classifiedEvents []classifiedEvent,
) bool {
	for _, classifiedEventForOverlay := range classifiedEvents {
		if classifiedEventForOverlay.fileType == fileTypeCriticalCSS ||
			classifiedEventForOverlay.fileType == fileTypeNormalCSS {
			continue
		}

		if classifiedEventForOverlay.watchedFile == nil ||
			!classifiedEventForOverlay.watchedFile.SkipRebuildingNotification {
			return true
		}
	}

	return false
}

func (s *server) processSingleEvent(
	ewh eventWithHooks,
	work *workSet,
	watcher *Watcher,
) {
	if ewh.needsHardReload {
		s.log.Info("Terminating running app")
		if err := s.stopApp(); err != nil {
			s.log.Error("Failed to terminate app", "error", err)
		}
	}

	s.processEventsWithDeterministicPipeline([]eventWithHooks{ewh}, work, watcher)
}

// processBatchedEvents handles multiple file changes.
func (s *server) processBatchedEvents(
	eventsWithHooks []eventWithHooks,
	work *workSet,
	watcher *Watcher,
) {
	if batchNeedsHardReloadStop(eventsWithHooks) {
		s.log.Info("Stopping app for batch rebuild")
		if err := s.stopApp(); err != nil {
			s.log.Error("Failed to stop app", "error", err)
		}
		for i := range eventsWithHooks {
			if eventsWithHooks[i].hookCtx != nil {
				eventsWithHooks[i].hookCtx.AppStoppedForBatch = true
			}
		}
	}

	s.processEventsWithDeterministicPipeline(eventsWithHooks, work, watcher)
}

func batchNeedsHardReloadStop(eventsWithHooks []eventWithHooks) bool {
	if !anyEventNeedsHardReload(eventsWithHooks) {
		return false
	}
	return !allBatchHookContextsMarkedAppStopped(eventsWithHooks)
}

func anyEventNeedsHardReload(eventsWithHooks []eventWithHooks) bool {
	for _, eventWithHooksForCheck := range eventsWithHooks {
		if eventWithHooksForCheck.needsHardReload {
			return true
		}
	}
	return false
}

func allBatchHookContextsMarkedAppStopped(eventsWithHooks []eventWithHooks) bool {
	if len(eventsWithHooks) == 0 {
		return false
	}

	for _, eventWithHooksForCheck := range eventsWithHooks {
		if eventWithHooksForCheck.hookCtx == nil ||
			!eventWithHooksForCheck.hookCtx.AppStoppedForBatch {
			return false
		}
	}

	return true
}

func (s *server) processEventsWithDeterministicPipeline(
	eventsWithHooks []eventWithHooks,
	work *workSet,
	watcher *Watcher,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	s.fireNoWaitHooksForEvents(eventsWithHooks, watcher)

	preActions := s.runPreHooksForEvents(eventsWithHooks, work, watcher)
	preActionResult := work.applyRefreshActions(preActions)
	if preActionResult.restartRequested {
		s.triggerRestartFromRefreshActions(preActionResult)
		return
	}

	shouldRunImplicitBuild := s.shouldRunImplicitBuild(eventsWithHooks)
	if !shouldRunImplicitBuild {
		if len(eventsWithHooks) == 1 {
			s.log.Info("RunOnChangeOnly: skipping implicit build phase")
		} else {
			s.log.Info("All events are RunOnChangeOnly, skipping implicit build phase")
		}
	} else {
		work.resolve(s.cfg.UsingVite())
	}

	var buildAndConcurrentHooksGroup errgroup.Group
	if shouldRunImplicitBuild {
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

	if shouldRunImplicitBuild && work.restartApp {
		s.log.Info("Restarting app")
		s.startApp()
	}

	s.executeBrowserPhase(work)
}

func (s *server) shouldRunImplicitBuild(eventsWithHooks []eventWithHooks) bool {
	for _, eventWithHooksForCheck := range eventsWithHooks {
		if !eventWithHooksForCheck.runOnChangeOnly {
			return true
		}
	}
	return false
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

	if work.compileGo {
		g.Go(func() error {
			if err := builder.CompileGoOnly(true); err != nil {
				s.log.Error("Go compilation failed", "error", err)
				return err
			}
			return nil
		})
	}

	needsFileProcessing := work.processPublicFiles || work.processPrivateFiles ||
		work.buildCriticalCSS || work.buildNormalCSS

	if needsFileProcessing {
		g.Go(func() error {
			if work.processPublicFiles {
				if err := builder.ProcessPublicFilesOnly(); err != nil {
					s.log.Error("Public files processing failed", "error", err)
					return err
				}
				if s.cfg.FrameworkPublicFileMapOutDir != "" {
					if err := builder.WritePublicFileMapTS(s.cfg.FrameworkPublicFileMapOutDir); err != nil {
						s.log.Error("Write public file map TS failed", "error", err)
						return err
					}
				}
			}

			var innerG errgroup.Group

			if work.processPrivateFiles {
				innerG.Go(func() error {
					if err := builder.ProcessPrivateFilesOnly(); err != nil {
						s.log.Error("Private files processing failed", "error", err)
						return err
					}
					return nil
				})
			}

			if work.buildCriticalCSS {
				innerG.Go(func() error {
					if err := builder.BuildCriticalCSS(true); err != nil {
						s.log.Error("Critical CSS build failed", "error", err)
						return err
					}
					return nil
				})
			}

			if work.buildNormalCSS {
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

	if work.invalidateVite {
		if s.cfg.UsingVite() {
			if err := s.callViteFilemapInvalidate(); err != nil {
				s.log.Warn("Vite filemap invalidate failed, falling back to reload", "error", err)
				work.reloadBrowser = true
				work.waitForApp = true
				work.waitForVite = true
			} else {
				return
			}
		} else {
			work.reloadBrowser = true
			work.waitForApp = true
		}
	}

	if work.reloadBrowser {
		s.log.Info("Hard reloading browser")
		s.broadcastReload(reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeOther},
			waitApp:   work.waitForApp,
			waitVite:  work.waitForVite,
			cycleVite: work.cycleVite,
		})
		return
	}

	if work.revalidate {
		s.log.Info("Running client-defined revalidate function")
		s.broadcastReload(reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeRevalidate},
			waitApp:   work.waitForApp,
			waitVite:  work.waitForVite,
			cycleVite: false,
		})
		return
	}

	if work.hotReloadCSS && builder != nil {
		s.log.Info("Hot reloading CSS")
		criticalCSS, _ := builder.ReadCriticalCSS()
		normalURL, _ := builder.ReadNormalCSSURL()

		if work.buildCriticalCSS && work.buildNormalCSS {
			s.broadcastReload(reloadOpts{
				payload: refreshPayload{
					ChangeType:  changeTypeCriticalCSS,
					CriticalCSS: base64.StdEncoding.EncodeToString([]byte(criticalCSS)),
				},
			})
			s.broadcastReload(reloadOpts{
				payload: refreshPayload{
					ChangeType:   changeTypeNormalCSS,
					NormalCSSURL: normalURL,
				},
			})
		} else if work.buildCriticalCSS {
			s.broadcastReload(reloadOpts{
				payload: refreshPayload{
					ChangeType:  changeTypeCriticalCSS,
					CriticalCSS: base64.StdEncoding.EncodeToString([]byte(criticalCSS)),
				},
			})
		} else if work.buildNormalCSS {
			s.broadcastReload(reloadOpts{
				payload: refreshPayload{
					ChangeType:   changeTypeNormalCSS,
					NormalCSSURL: normalURL,
				},
			})
		}
	}
}

func (s *server) classifyEventWithWatcherAndBuilder(evt fsnotify.Event, watcher *Watcher, builder *Builder) classifiedEvent {
	result := classifiedEvent{event: evt}

	if evt.Name == "" {
		result.ignored = true
		return result
	}

	result.ignored = watcher.IsIgnoredFile(evt.Name)

	if builder.IsCriticalCSSFile(evt.Name) {
		result.fileType = fileTypeCriticalCSS
	} else if builder.IsNormalCSSFile(evt.Name) {
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

	normalizedPath := pathnorm.Absolute(path)
	normalizedConfigPath := pathnorm.Absolute(configPath)
	return normalizedPath != "" &&
		normalizedConfigPath != "" &&
		normalizedPath == normalizedConfigPath
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

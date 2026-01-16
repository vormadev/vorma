package tooling

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/wave"
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
	classified      classifiedEvent
	hooks           *wave.SortedHooks
	hookCtx         *wave.HookContext
	runOnChangeOnly bool
	needsHardReload bool
}

func (s *server) runWatcher() {
	debouncer := NewDebouncer(30*time.Millisecond, func(events []fsnotify.Event) {
		s.processEvents(events)
	})
	defer debouncer.Stop()

	for {
		select {
		case evt, ok := <-s.watcher.Events():
			if !ok {
				return
			}
			debouncer.Add(evt)
		case err := <-s.watcher.Errors():
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

	// Deduplicate events by path
	eventMap := make(map[string]fsnotify.Event)
	for _, evt := range events {
		eventMap[evt.Name] = evt
	}

	// 1. Classify events and check for config change
	var classified []classifiedEvent
	handledPatterns := make(map[string]bool)

	for _, evt := range eventMap {
		if s.isConfigFile(evt.Name) && (evt.Has(fsnotify.Write) || evt.Has(fsnotify.Create)) {
			s.log.Info("Config changed, restarting")
			s.triggerConfigRestart()
			return
		}

		info, _ := os.Stat(evt.Name)
		if info != nil && info.IsDir() {
			if evt.Has(fsnotify.Create) || evt.Has(fsnotify.Rename) {
				watcher.AddDir(evt.Name)
			}
			continue
		}

		c := s.classifyEventWithWatcherAndBuilder(evt, watcher, builder)
		if c.ignored || c.chmodOnly {
			continue
		}

		patternKey := ""
		if c.watchedFile != nil {
			patternKey = c.watchedFile.Pattern
		}
		if handledPatterns[patternKey] {
			continue
		}
		handledPatterns[patternKey] = true

		classified = append(classified, c)
	}

	if len(classified) == 0 {
		return
	}

	// 2. Determine if we should show rebuilding overlay - check IMMEDIATELY
	showRebuilding := false
	for _, c := range classified {
		if c.fileType != fileTypeCriticalCSS && c.fileType != fileTypeNormalCSS {
			if c.watchedFile == nil || !c.watchedFile.SkipRebuildingNotification {
				showRebuilding = true
				break
			}
		}
	}
	if showRebuilding {
		s.broadcastRebuilding()
	}

	// 3. Prepare events with hooks
	work := &workSet{}
	eventsWithHooks := make([]eventWithHooks, 0, len(classified))
	isBatch := len(classified) > 1
	batchNeedsAppStop := false

	for _, c := range classified {
		s.log.Info("[watcher]", "op", c.event.Op.String(), "file", c.event.Name)

		wf := c.watchedFile
		if wf == nil {
			wf = &wave.WatchedFile{}
		}
		if wf.SortedHooks == nil {
			wf.Sort()
		}
		if wf.SortedHooks == nil {
			wf.SortedHooks = &wave.SortedHooks{}
		}

		eventNeedsHardReload := c.fileType == fileTypeGo || needsHardReload(wf)
		if eventNeedsHardReload {
			batchNeedsAppStop = true
		}

		eventsWithHooks = append(eventsWithHooks, eventWithHooks{
			classified:      c,
			hooks:           wf.SortedHooks,
			runOnChangeOnly: wf.RunOnChangeOnly,
			needsHardReload: eventNeedsHardReload,
			hookCtx: &wave.HookContext{
				FilePath:           c.event.Name,
				AppStoppedForBatch: false,
			},
		})
	}

	// 4. For batches with hard reload, stop app upfront (safer for batch processing)
	if isBatch && batchNeedsAppStop {
		s.log.Info("Stopping app for batch rebuild")
		if err := s.stopApp(); err != nil {
			s.log.Error("Failed to stop app", "error", err)
		}
		for i := range eventsWithHooks {
			eventsWithHooks[i].hookCtx.AppStoppedForBatch = true
		}
	}

	// 5. Process events - for batches, process all then reload once
	//    For single events, use the original parallel kill + build approach
	if isBatch {
		s.processBatchedEvents(eventsWithHooks, work, watcher)
	} else {
		s.processSingleEvent(eventsWithHooks[0], work, watcher)
	}

	watcher.RemoveStale()
}

// processSingleEvent handles a single file change with maximum parallelism:
// app termination runs in parallel with hooks and build (matching old behavior)
func (s *server) processSingleEvent(ewh eventWithHooks, work *workSet, watcher *Watcher) {
	// Fire no-wait hooks immediately
	s.fireNoWaitHooks(ewh, watcher)

	// Start app termination in parallel if needed
	var killEg errgroup.Group
	if ewh.needsHardReload {
		killEg.Go(func() error {
			s.log.Info("Terminating running app")
			return s.stopApp()
		})
	}

	// Run pre hooks
	preActions, err := s.runPreHooks(ewh, watcher)
	if err != nil {
		s.log.Error("Pre-hook execution failed", "error", err)
	}

	// Check for restart request from pre hooks
	for _, action := range preActions {
		if action.TriggerRestart {
			killEg.Wait()
			if action.RecompileGo {
				s.triggerRestart()
			} else {
				s.triggerRestartNoGo()
			}
			return
		}
		work.addFromRefreshAction(action)
	}

	// Add implicit work
	work.addImplicitWork(ewh.classified)

	// Check for RunOnChangeOnly - if so, we're done
	if ewh.runOnChangeOnly {
		s.log.Info("RunOnChangeOnly: skipping build phase")
		killEg.Wait()
		return
	}

	// Resolve browser behavior
	work.resolve(s.cfg.UsingVite())

	// Run build AND concurrent hooks in parallel (both also parallel with app kill)
	var buildAndConcurrentEg errgroup.Group
	var concurrentActions []wave.RefreshAction
	var concurrentActionsMu sync.Mutex

	buildAndConcurrentEg.Go(func() error {
		s.executeBuildPhase(work)
		return nil
	})

	buildAndConcurrentEg.Go(func() error {
		actions, err := s.runConcurrentHooks(ewh, watcher)
		if err != nil {
			s.log.Error("Concurrent hook execution failed", "error", err)
		}
		if len(actions) > 0 {
			concurrentActionsMu.Lock()
			concurrentActions = append(concurrentActions, actions...)
			concurrentActionsMu.Unlock()
		}
		return nil
	})

	// Wait for build and concurrent hooks
	buildAndConcurrentEg.Wait()

	// Process concurrent hook actions
	for _, action := range concurrentActions {
		if action.TriggerRestart {
			killEg.Wait()
			if action.RecompileGo {
				s.triggerRestart()
			} else {
				s.triggerRestartNoGo()
			}
			return
		}
		work.addFromRefreshAction(action)
	}

	// Wait for app termination to complete before post hooks
	if err := killEg.Wait(); err != nil {
		s.log.Error("Failed to terminate app", "error", err)
	}

	// Run post hooks (after build, after app stopped)
	postActions, err := s.runPostHooks(ewh, watcher)
	if err != nil {
		s.log.Error("Post-hook execution failed", "error", err)
	}

	for _, action := range postActions {
		if action.TriggerRestart {
			if action.RecompileGo {
				s.triggerRestart()
			} else {
				s.triggerRestartNoGo()
			}
			return
		}
		work.addFromRefreshAction(action)
	}

	// Restart app if needed
	if work.restartApp {
		s.log.Info("Restarting app")
		s.startApp()
	}

	// Browser refresh
	s.executeBrowserPhase(work)
}

// processBatchedEvents handles multiple file changes - app already stopped upfront
func (s *server) processBatchedEvents(eventsWithHooks []eventWithHooks, work *workSet, watcher *Watcher) {
	// Fire all no-wait hooks
	for _, ewh := range eventsWithHooks {
		s.fireNoWaitHooks(ewh, watcher)
	}

	// Run all pre hooks
	var allPreActions []wave.RefreshAction
	for _, ewh := range eventsWithHooks {
		actions, err := s.runPreHooks(ewh, watcher)
		if err != nil {
			s.log.Error("Pre-hook execution failed", "error", err)
		}
		allPreActions = append(allPreActions, actions...)
		work.addImplicitWork(ewh.classified)
	}

	// Check for restart request from pre hooks
	for _, action := range allPreActions {
		if action.TriggerRestart {
			if action.RecompileGo {
				s.triggerRestart()
			} else {
				s.triggerRestartNoGo()
			}
			return
		}
		work.addFromRefreshAction(action)
	}

	// Check if ALL events are RunOnChangeOnly
	allRunOnChangeOnly := true
	for _, ewh := range eventsWithHooks {
		if !ewh.runOnChangeOnly {
			allRunOnChangeOnly = false
			break
		}
	}
	if allRunOnChangeOnly {
		s.log.Info("All events are RunOnChangeOnly, skipping build phase")
		return
	}

	// Resolve browser behavior
	work.resolve(s.cfg.UsingVite())

	// Run build AND all concurrent hooks in parallel
	var buildAndConcurrentEg errgroup.Group
	var concurrentActions []wave.RefreshAction
	var concurrentActionsMu sync.Mutex

	buildAndConcurrentEg.Go(func() error {
		s.executeBuildPhase(work)
		return nil
	})

	for _, ewh := range eventsWithHooks {
		if ewh.runOnChangeOnly {
			continue
		}
		ewh := ewh
		buildAndConcurrentEg.Go(func() error {
			actions, err := s.runConcurrentHooks(ewh, watcher)
			if err != nil {
				s.log.Error("Concurrent hook execution failed", "error", err)
			}
			if len(actions) > 0 {
				concurrentActionsMu.Lock()
				concurrentActions = append(concurrentActions, actions...)
				concurrentActionsMu.Unlock()
			}
			return nil
		})
	}

	buildAndConcurrentEg.Wait()

	// Process concurrent hook actions
	for _, action := range concurrentActions {
		if action.TriggerRestart {
			if action.RecompileGo {
				s.triggerRestart()
			} else {
				s.triggerRestartNoGo()
			}
			return
		}
		work.addFromRefreshAction(action)
	}

	// Run all post hooks
	var allPostActions []wave.RefreshAction
	for _, ewh := range eventsWithHooks {
		if ewh.runOnChangeOnly {
			continue
		}
		actions, err := s.runPostHooks(ewh, watcher)
		if err != nil {
			s.log.Error("Post-hook execution failed", "error", err)
		}
		allPostActions = append(allPostActions, actions...)
	}

	for _, action := range allPostActions {
		if action.TriggerRestart {
			if action.RecompileGo {
				s.triggerRestart()
			} else {
				s.triggerRestartNoGo()
			}
			return
		}
		work.addFromRefreshAction(action)
	}

	// Restart app if needed
	if work.restartApp {
		s.log.Info("Restarting app")
		s.startApp()
	}

	// Single browser refresh for entire batch
	s.executeBrowserPhase(work)
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
		cmd := s.resolveCmd(hook.Cmd)
		if cmd != "" {
			go func(c string) {
				if err := executil.RunShell(c); err != nil {
					s.log.Warn("concurrent-no-wait hook failed", "cmd", c, "error", err)
				}
			}(cmd)
		}
	}
}

func (s *server) runPreHooks(ewh eventWithHooks, watcher *Watcher) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction

	for _, hook := range ewh.hooks.Pre {
		if watcher.IsIgnored(ewh.classified.event.Name, hook.Exclude) {
			continue
		}
		if hook.Callback != nil {
			action, err := hook.Callback(ewh.hookCtx)
			if err != nil {
				return actions, err
			}
			if action != nil {
				actions = append(actions, *action)
			}
		}
		cmd := s.resolveCmd(hook.Cmd)
		if cmd != "" {
			if err := executil.RunShell(cmd); err != nil {
				return actions, err
			}
		}
	}

	return actions, nil
}

func (s *server) runConcurrentHooks(ewh eventWithHooks, watcher *Watcher) ([]wave.RefreshAction, error) {
	if len(ewh.hooks.Concurrent) == 0 {
		return nil, nil
	}

	var actions []wave.RefreshAction
	var actionsMu sync.Mutex
	var eg errgroup.Group

	for _, hook := range ewh.hooks.Concurrent {
		if watcher.IsIgnored(ewh.classified.event.Name, hook.Exclude) {
			continue
		}
		h := hook
		eg.Go(func() error {
			if h.Callback != nil {
				action, err := h.Callback(ewh.hookCtx)
				if err != nil {
					return err
				}
				if action != nil {
					actionsMu.Lock()
					actions = append(actions, *action)
					actionsMu.Unlock()
				}
			}
			cmd := s.resolveCmd(h.Cmd)
			if cmd != "" {
				return executil.RunShell(cmd)
			}
			return nil
		})
	}

	err := eg.Wait()
	return actions, err
}

func (s *server) runPostHooks(ewh eventWithHooks, watcher *Watcher) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction

	for _, hook := range ewh.hooks.Post {
		if watcher.IsIgnored(ewh.classified.event.Name, hook.Exclude) {
			continue
		}
		if hook.Callback != nil {
			action, err := hook.Callback(ewh.hookCtx)
			if err != nil {
				return actions, err
			}
			if action != nil {
				actions = append(actions, *action)
			}
		}
		cmd := s.resolveCmd(hook.Cmd)
		if cmd != "" {
			if err := executil.RunShell(cmd); err != nil {
				return actions, err
			}
		}
	}

	return actions, nil
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
	configPath := s.cfg.Core.ConfigLocation
	if configPath == "" {
		return false
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		absConfigPath = configPath
	}

	return absPath == absConfigPath
}

func needsHardReload(wf *wave.WatchedFile) bool {
	if wf == nil {
		return false
	}
	return wf.RecompileGoBinary || wf.RestartApp
}

func (s *server) resolveCmd(cmd string) string {
	if cmd == "DevBuildHook" {
		return s.cfg.Core.DevBuildHook
	}
	return cmd
}

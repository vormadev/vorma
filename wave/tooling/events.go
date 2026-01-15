package tooling

import (
	"encoding/base64"
	"fmt"
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
)

type classifiedEvent struct {
	event       fsnotify.Event
	fileType    fileType
	watchedFile *wave.WatchedFile
	ignored     bool
	chmodOnly   bool
}

// workSet collects all work to be done in response to file changes.
// This separates "what to do" from "how to coordinate it."
type workSet struct {
	// Build phase
	compileGo          bool
	buildCriticalCSS   bool
	buildNormalCSS     bool
	processPublicFiles bool

	// App phase
	restartApp bool

	// Browser phase
	reloadBrowser  bool
	hotReloadCSS   bool
	invalidateVite bool
	revalidate     bool
	waitForApp     bool
	waitForVite    bool
	cycleVite      bool

	// Notification
	showRebuilding bool
}

// addFromRefreshAction merges a RefreshAction into the work set.
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

// addImplicitWork adds work implied by a file type and its watched config.
func (w *workSet) addImplicitWork(c classifiedEvent) {
	wf := c.watchedFile
	if wf != nil && wf.RunOnChangeOnly {
		return // Callbacks handle everything
	}

	switch c.fileType {
	case fileTypeGo:
		w.compileGo = true
		w.restartApp = true
		w.reloadBrowser = true
		w.waitForApp = true
		w.waitForVite = true

	case fileTypeCriticalCSS:
		w.buildCriticalCSS = true
		if wf == nil || !needsHardReload(wf) {
			w.hotReloadCSS = true
		} else {
			w.restartApp = true
			w.reloadBrowser = true
		}

	case fileTypeNormalCSS:
		w.buildNormalCSS = true
		if wf == nil || !needsHardReload(wf) {
			w.hotReloadCSS = true
		} else {
			w.restartApp = true
			w.reloadBrowser = true
		}

	case fileTypePublicStatic:
		w.processPublicFiles = true
		w.invalidateVite = true
		// Fallback to reloadBrowser handled in executeBrowserPhase if not using Vite

	case fileTypeOther:
		if wf != nil {
			if wf.RecompileGoBinary {
				w.compileGo = true
			}
			if wf.RestartApp || wf.RecompileGoBinary {
				w.restartApp = true
				w.reloadBrowser = true
				w.waitForApp = true
				w.waitForVite = true
			}
			if wf.OnlyRunClientDefinedRevalidateFunc {
				w.revalidate = true
				w.waitForApp = true
				w.waitForVite = true
			}
		}
	}

	// Notification logic
	if wf == nil || !wf.SkipRebuildingNotification {
		if c.fileType != fileTypeCriticalCSS && c.fileType != fileTypeNormalCSS {
			w.showRebuilding = true
		}
	}
}

// resolve handles mutual exclusivity and subsumption.
func (w *workSet) resolve() {
	// Full browser reload subsumes CSS hot reload and revalidate
	if w.reloadBrowser {
		w.hotReloadCSS = false
		w.revalidate = false
		w.invalidateVite = false
	}

	// Restart implies wait for app and full reload
	if w.restartApp {
		w.waitForApp = true
		w.reloadBrowser = true
		w.hotReloadCSS = false
		w.revalidate = false
	}

	// Go recompile implies restart
	if w.compileGo {
		w.restartApp = true
	}
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
				s.watcher.AddDir(evt.Name)
			}
			continue
		}

		c := s.classifyEvent(evt)
		if c.ignored || c.chmodOnly {
			continue
		}

		// Dedupe by pattern
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

	// 2. Run hooks and collect work
	work := &workSet{}
	var allActions []wave.RefreshAction

	// Determine if we need to stop app before hooks (for hard reload batches)
	needsAppStop := false
	for _, c := range classified {
		if c.fileType == fileTypeGo || needsHardReload(c.watchedFile) {
			needsAppStop = true
			break
		}
	}

	// Show rebuilding overlay if any event requires it
	for _, c := range classified {
		if c.fileType == fileTypeCriticalCSS || c.fileType == fileTypeNormalCSS {
			continue
		}
		if c.watchedFile == nil || !c.watchedFile.SkipRebuildingNotification {
			work.showRebuilding = true
			break
		}
	}

	if work.showRebuilding {
		s.broadcastRebuilding()
	}

	// Stop app before hooks if needed
	appStoppedForBatch := false
	if needsAppStop && len(classified) > 1 {
		s.log.Info("Stopping app for batch rebuild")
		if err := s.stopApp(); err != nil {
			panic(fmt.Sprintf("failed to stop app: %v", err))
		}
		appStoppedForBatch = true
	}

	// Run hooks for each event
	for _, c := range classified {
		s.log.Info("File changed", "op", c.event.Op.String(), "file", c.event.Name)
		actions, err := s.runHooksForEvent(c, appStoppedForBatch)
		if err != nil {
			s.log.Error("Hook execution failed", "error", err)
		}
		allActions = append(allActions, actions...)

		// Add implicit work from file type
		work.addImplicitWork(c)
	}

	// Check if any callback requested a full restart cycle (bail out)
	for _, action := range allActions {
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

	// 3. Resolve mutual exclusivity
	work.resolve()

	// 4. Stop app if needed before build
	needsStop := work.compileGo || work.restartApp
	if needsStop && !appStoppedForBatch {
		s.log.Info("Stopping app")
		if err := s.stopApp(); err != nil {
			panic(fmt.Sprintf("failed to stop app: %v", err))
		}
	}

	// 5. Execute build phase
	s.executeBuildPhase(work)

	// 6. Start app if needed
	if work.restartApp {
		s.startApp()
	}

	// 7. Execute browser phase
	s.executeBrowserPhase(work)

	s.watcher.RemoveStale()
}

func (s *server) runHooksForEvent(c classifiedEvent, appStoppedForBatch bool) ([]wave.RefreshAction, error) {
	wf := c.watchedFile
	if wf == nil {
		wf = &wave.WatchedFile{}
	}
	if wf.SortedHooks == nil {
		wf.SortedHooks = &wave.SortedHooks{}
	}

	sorted := wf.SortedHooks
	hookCtx := &wave.HookContext{
		FilePath:           c.event.Name,
		AppStoppedForBatch: appStoppedForBatch,
	}

	var actions []wave.RefreshAction

	// Fire-and-forget hooks
	for _, hook := range sorted.ConcurrentNoWait {
		if s.watcher.IsIgnored(c.event.Name, hook.Exclude) {
			continue
		}
		if hook.Callback != nil {
			go func(cb func(*wave.HookContext) (*wave.RefreshAction, error), ctx *wave.HookContext) {
				if _, err := cb(ctx); err != nil {
					s.log.Warn("concurrent-no-wait callback failed", "error", err)
				}
			}(hook.Callback, hookCtx)
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

	// Pre hooks
	for _, hook := range sorted.Pre {
		if s.watcher.IsIgnored(c.event.Name, hook.Exclude) {
			continue
		}
		if hook.Callback != nil {
			action, err := hook.Callback(hookCtx)
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

	// Concurrent hooks
	var eg errgroup.Group
	var actionsMu sync.Mutex

	for _, hook := range sorted.Concurrent {
		if s.watcher.IsIgnored(c.event.Name, hook.Exclude) {
			continue
		}
		h := hook
		eg.Go(func() error {
			if h.Callback != nil {
				action, err := h.Callback(hookCtx)
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

	if err := eg.Wait(); err != nil {
		return actions, err
	}

	// Post hooks
	for _, hook := range sorted.Post {
		if s.watcher.IsIgnored(c.event.Name, hook.Exclude) {
			continue
		}
		if hook.Callback != nil {
			action, err := hook.Callback(hookCtx)
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
	// Compile Go (once for batch)
	if work.compileGo {
		s.log.Info("Compiling Go binary")
		if err := s.builder.CompileGoOnly(true); err != nil {
			s.log.Error("Go compilation failed", "error", err)
		}
	}

	// Build CSS
	if work.buildCriticalCSS {
		if err := s.builder.BuildCriticalCSS(true); err != nil {
			s.log.Error("Critical CSS build failed", "error", err)
		}
	}
	if work.buildNormalCSS {
		if err := s.builder.BuildNormalCSS(true); err != nil {
			s.log.Error("Normal CSS build failed", "error", err)
		}
	}

	// Process public files
	if work.processPublicFiles {
		if err := s.builder.ProcessPublicFilesOnly(); err != nil {
			s.log.Error("Public files processing failed", "error", err)
		}
		if s.cfg.FrameworkPublicFileMapOutDir != "" {
			if err := s.builder.WritePublicFileMapTS(s.cfg.FrameworkPublicFileMapOutDir); err != nil {
				s.log.Error("Write public file map TS failed", "error", err)
			}
		}
	}
}

func (s *server) executeBrowserPhase(work *workSet) {
	if !s.cfg.UsingBrowser() {
		return
	}

	// Vite invalidation (for public static changes)
	// Attempt if requested. Falls back to standard reload if Vite fails or isn't used.
	if work.invalidateVite {
		if s.cfg.UsingVite() {
			if err := s.callViteFilemapInvalidate(); err != nil {
				s.log.Warn("Vite filemap invalidate failed, falling back to reload", "error", err)
				work.reloadBrowser = true
			} else {
				// Success: Vite handles browser reload via its own websocket
				return
			}
		} else {
			// Not using Vite: fall back to standard reload
			work.reloadBrowser = true
		}
	}

	// Full browser reload
	if work.reloadBrowser {
		s.broadcastReload(reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeOther},
			waitApp:   work.waitForApp,
			waitVite:  work.waitForVite,
			cycleVite: work.cycleVite,
		})
		return
	}

	// Revalidate (client-side refresh)
	if work.revalidate {
		s.broadcastReload(reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeRevalidate},
			waitApp:   work.waitForApp,
			waitVite:  work.waitForVite,
			cycleVite: false,
		})
		return
	}

	// CSS hot reload
	if work.hotReloadCSS {
		criticalCSS, _ := s.builder.ReadCriticalCSS()
		normalURL, _ := s.builder.ReadNormalCSSURL()

		// Send both if both changed, otherwise send individually
		if work.buildCriticalCSS && work.buildNormalCSS {
			// Send critical first, then normal
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

func (s *server) classifyEvent(evt fsnotify.Event) classifiedEvent {
	result := classifiedEvent{event: evt}

	if evt.Name == "" {
		result.ignored = true
		return result
	}

	result.ignored = s.watcher.IsIgnoredFile(evt.Name)

	if s.builder.IsCriticalCSSFile(evt.Name) {
		result.fileType = fileTypeCriticalCSS
	} else if s.builder.IsNormalCSSFile(evt.Name) {
		result.fileType = fileTypeNormalCSS
	} else if filepath.Ext(evt.Name) == ".go" {
		result.fileType = fileTypeGo
	} else if s.watcher.IsPublicStaticFile(evt.Name) {
		result.fileType = fileTypePublicStatic
	} else {
		result.fileType = fileTypeOther
	}

	result.watchedFile = s.watcher.FindWatchedFile(evt.Name)

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

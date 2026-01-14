package tooling

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	strategy    *wave.OnChangeStrategy
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
	eventMap := make(map[string]fsnotify.Event)
	for _, evt := range events {
		eventMap[evt.Name] = evt
	}

	var relevantEvents []classifiedEvent
	needsHardReload := false
	needsGoCompile := false
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

		classified := s.classifyEvent(evt)
		if classified.ignored {
			continue
		}

		patternKey := ""
		if classified.watchedFile != nil {
			patternKey = classified.watchedFile.Pattern
		}
		if handledPatterns[patternKey] {
			continue
		}
		handledPatterns[patternKey] = true

		// Strategy events flow through handleFileChange so Callbacks run
		if classified.chmodOnly {
			continue
		}

		if classified.fileType == fileTypeGo {
			needsGoCompile = true
		}

		// Only force hard reload if there is NO strategy.
		// Strategies handle their own reload logic.
		if classified.strategy == nil && !needsHardReload {
			needsHardReload = classified.fileType == fileTypeGo || s.needsHardReload(classified.watchedFile)
		}

		relevantEvents = append(relevantEvents, classified)
	}

	if len(relevantEvents) == 0 {
		return
	}

	isBatch := len(relevantEvents) > 1

	// Determine if we should show the rebuilding overlay.
	// We show it if any involved file does NOT explicitly skip notification.
	shouldBroadcast := false
	for _, e := range relevantEvents {
		// If no watched file config (e.g. standard go file outside patterns), default is show.
		if e.watchedFile == nil {
			shouldBroadcast = true
			break
		}
		if !e.watchedFile.SkipRebuildingNotification {
			shouldBroadcast = true
			break
		}
	}

	if shouldBroadcast {
		s.broadcastRebuilding()
	}

	// For standard (non-strategy) batch + hard reload, stop app before compiling
	// We check if *any* event is standard (no strategy) to trigger standard stop logic
	anyStandardEvents := false
	for _, e := range relevantEvents {
		if e.strategy == nil {
			anyStandardEvents = true
			break
		}
	}

	if anyStandardEvents && isBatch && needsHardReload {
		s.log.Info("Shutting down running app (batch)")
		if err := s.stopApp(); err != nil {
			panic(fmt.Sprintf("failed to stop app: %v", err))
		}
	}

	// For standard Go batches: compile Go once, not per-file
	if anyStandardEvents && isBatch && needsGoCompile {
		s.log.Info("Batch Go change detected, compiling once")
		if err := s.builder.CompileGoOnly(true); err != nil {
			s.log.Error("Go compilation failed", "error", err)
		}
	}

	for _, classified := range relevantEvents {
		s.log.Info("File changed", "op", classified.event.Op.String(), "file", classified.event.Name)
		skipGoCompile := isBatch && needsGoCompile && classified.fileType == fileTypeGo
		if err := s.handleFileChange(classified, isBatch, skipGoCompile); err != nil {
			s.log.Error("Handle change failed", "error", err)
		}
	}

	if anyStandardEvents && isBatch && needsHardReload {
		s.startApp()
		s.broadcastReload(reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeOther},
			waitApp:   true,
			waitVite:  true,
			cycleVite: false,
		})
	}

	s.watcher.RemoveStale()
}

func (s *server) executeStrategy(strategy *wave.OnChangeStrategy) error {
	if strategy == nil {
		return nil
	}

	if strategy.HttpEndpoint != "" {
		s.log.Info("Executing strategy HTTP endpoint", "endpoint", strategy.HttpEndpoint)
		if !s.waitForApp() {
			return s.handleStrategyFallback(strategy, fmt.Errorf("app not ready"))
		}
		if err := s.callStrategyEndpoint(strategy.HttpEndpoint); err != nil {
			return s.handleStrategyFallback(strategy, err)
		}
	}

	if strategy.ReloadBrowser {
		s.broadcastReload(reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeOther},
			waitApp:   strategy.WaitForApp,
			waitVite:  strategy.WaitForVite,
			cycleVite: false,
		})
	}

	return nil
}

func (s *server) callStrategyEndpoint(endpoint string) error {
	url := fmt.Sprintf("http://localhost:%d%s", wave.MustGetPort(), endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	s.log.Info("Strategy endpoint succeeded", "endpoint", endpoint)
	return nil
}

func (s *server) handleStrategyFallback(strategy *wave.OnChangeStrategy, originalErr error) error {
	s.log.Warn("Strategy failed, executing fallback", "error", originalErr, "fallback", strategy.FallbackAction)

	switch strategy.FallbackAction {
	case wave.FallbackRestart:
		s.triggerRestart()
		return nil
	case wave.FallbackRestartNoGo:
		s.triggerRestartNoGo()
		return nil
	case wave.FallbackNone, "":
		return originalErr
	default:
		s.log.Warn("Unknown fallback action", "action", strategy.FallbackAction)
		return originalErr
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

	result.strategy = s.watcher.GetFirstStrategy(result.watchedFile)
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

func (s *server) needsHardReload(wf *wave.WatchedFile) bool {
	if wf == nil {
		return false
	}
	return wf.RecompileGoBinary || wf.RestartApp
}

func (s *server) handleFileChange(classified classifiedEvent, isBatch bool, skipGoCompile bool) error {
	wf := classified.watchedFile
	if wf == nil {
		wf = &wave.WatchedFile{}
	}
	if wf.SortedHooks == nil {
		wf.SortedHooks = &wave.SortedHooks{}
	}

	sorted := wf.SortedHooks

	// 1. EXECUTE HOOKS (Callbacks & Cmds)
	// We run Callbacks regardless of whether a Strategy exists.
	// We skip Cmds if a Strategy exists.

	// Fire-and-forget hooks
	for _, hook := range sorted.ConcurrentNoWait {
		if s.watcher.IsIgnored(classified.event.Name, hook.Exclude) {
			continue
		}
		// Always run Callback (internal framework logic)
		if hook.Callback != nil {
			go func(cb func(string) error, name string) {
				if err := cb(name); err != nil {
					s.log.Warn("concurrent-no-wait callback failed", "error", err)
				}
			}(hook.Callback, classified.event.Name)
		}
		// Skip Cmd if Strategy overrides it
		if hook.HasStrategy() {
			continue
		}
		cmd := s.resolveCmd(hook.Cmd)
		if cmd == "" {
			continue
		}
		go func(c string) {
			if err := executil.RunShell(c); err != nil {
				s.log.Warn("concurrent-no-wait hook failed", "cmd", c, "error", err)
			}
		}(cmd)
	}

	// Pre hooks
	for _, hook := range sorted.Pre {
		if s.watcher.IsIgnored(classified.event.Name, hook.Exclude) {
			continue
		}
		// Always run Callback
		if hook.Callback != nil {
			if err := hook.Callback(classified.event.Name); err != nil {
				return err
			}
		}
		// Skip Cmd if Strategy overrides it
		if hook.HasStrategy() {
			continue
		}
		cmd := s.resolveCmd(hook.Cmd)
		if cmd == "" {
			continue
		}
		if err := executil.RunShell(cmd); err != nil {
			return err
		}
	}

	var callbackEg errgroup.Group

	// Concurrent hooks
	for _, hook := range sorted.Concurrent {
		if s.watcher.IsIgnored(classified.event.Name, hook.Exclude) {
			continue
		}
		h := hook
		callbackEg.Go(func() error {
			if h.Callback != nil {
				if err := h.Callback(classified.event.Name); err != nil {
					return err
				}
			}
			if h.HasStrategy() {
				return nil
			}
			cmd := s.resolveCmd(h.Cmd)
			if cmd == "" {
				return nil
			}
			return executil.RunShell(cmd)
		})
	}

	if err := callbackEg.Wait(); err != nil {
		return err
	}

	// Post hooks
	for _, hook := range sorted.Post {
		if s.watcher.IsIgnored(classified.event.Name, hook.Exclude) {
			continue
		}
		if hook.Callback != nil {
			if err := hook.Callback(classified.event.Name); err != nil {
				return err
			}
		}
		if hook.HasStrategy() {
			continue
		}
		cmd := s.resolveCmd(hook.Cmd)
		if cmd == "" {
			continue
		}
		if err := executil.RunShell(cmd); err != nil {
			return err
		}
	}

	// 2. CHECK FOR STRATEGY
	// If a strategy exists, it overrides the default build/restart behavior below.
	if classified.strategy != nil {
		if err := s.executeStrategy(classified.strategy); err != nil {
			return s.handleStrategyFallback(classified.strategy, err)
		}
		return nil
	}

	// 3. STANDARD BUILD / RESTART LOGIC
	// Only runs if no strategy handled the event.
	isGo := classified.fileType == fileTypeGo

	needsRestart := (isGo || s.needsHardReload(wf)) && !isBatch

	var stopEg errgroup.Group
	if needsRestart {
		stopEg.Go(func() error {
			s.log.Info("Terminating running app")
			return s.stopApp()
		})
	}

	if wf.RunOnChangeOnly {
		s.log.Info("ran applicable onChange callbacks (RunOnChangeOnly)")
		if needsRestart {
			if err := stopEg.Wait(); err != nil {
				panic(fmt.Sprintf("failed to stop app: %v", err))
			}
			s.startApp()
		}
		return nil
	}

	// Run main build callback
	if err := s.fileChangeCallback(wf, classified, skipGoCompile); err != nil {
		return err
	}

	if needsRestart {
		if err := stopEg.Wait(); err != nil {
			panic(fmt.Sprintf("failed to stop app: %v", err))
		}
		s.startApp()
	}

	if isBatch {
		return nil
	}

	return s.handleBrowserReload(classified, wf)
}

func (s *server) fileChangeCallback(wf *wave.WatchedFile, classified classifiedEvent, skipGoCompile bool) error {
	isCSS := classified.fileType == fileTypeCriticalCSS || classified.fileType == fileTypeNormalCSS

	if classified.fileType == fileTypeGo {
		if skipGoCompile {
			return nil
		}
		return s.builder.CompileGoOnly(true)
	}

	if classified.fileType == fileTypePublicStatic {
		if err := s.builder.ProcessPublicFilesOnly(); err != nil {
			return err
		}
		if s.cfg.FrameworkPublicFileMapOutDir != "" {
			return s.builder.WritePublicFileMapTS(s.cfg.FrameworkPublicFileMapOutDir)
		}
		return nil
	}

	if isCSS {
		if classified.fileType == fileTypeCriticalCSS {
			if err := s.builder.BuildCriticalCSS(true); err != nil {
				return err
			}
		} else {
			if err := s.builder.BuildNormalCSS(true); err != nil {
				return err
			}
		}
		if !s.needsHardReload(wf) {
			return nil
		}
	}

	return s.builder.Build(BuildOpts{
		IsDev:        true,
		CompileGo:    wf.RecompileGoBinary,
		IsRebuild:    true,
		FileOnlyMode: isCSS || wf.OnlyRunClientDefinedRevalidateFunc,
	})
}

func (s *server) handleBrowserReload(classified classifiedEvent, wf *wave.WatchedFile) error {
	if !s.cfg.UsingBrowser() {
		return nil
	}

	if wf.OnlyRunClientDefinedRevalidateFunc {
		s.broadcastReload(reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeRevalidate},
			waitApp:   true,
			waitVite:  true,
			cycleVite: false,
		})
		return nil
	}

	isCSS := classified.fileType == fileTypeCriticalCSS || classified.fileType == fileTypeNormalCSS
	isPublicStatic := classified.fileType == fileTypePublicStatic

	if isPublicStatic {
		s.broadcastReload(reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeOther},
			waitApp:   false,
			waitVite:  true,
			cycleVite: true,
		})
		return nil
	}

	if !isCSS || s.needsHardReload(wf) {
		s.broadcastReload(reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeOther},
			waitApp:   true,
			waitVite:  true,
			cycleVite: false,
		})
		return nil
	}

	changeType := changeTypeNormalCSS
	if classified.fileType == fileTypeCriticalCSS {
		changeType = changeTypeCriticalCSS
	}

	criticalCSS, _ := s.builder.ReadCriticalCSS()
	normalURL, _ := s.builder.ReadNormalCSSURL()

	s.broadcastReload(reloadOpts{
		payload: refreshPayload{
			ChangeType:   changeType,
			CriticalCSS:  base64.StdEncoding.EncodeToString([]byte(criticalCSS)),
			NormalCSSURL: normalURL,
		},
		waitApp:   false,
		waitVite:  false,
		cycleVite: false,
	})

	return nil
}

func (s *server) resolveCmd(cmd string) string {
	if cmd == "DevBuildHook" {
		return s.cfg.Core.DevBuildHook
	}
	return cmd
}

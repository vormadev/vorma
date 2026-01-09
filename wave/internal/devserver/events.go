package devserver

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
	"github.com/vormadev/vorma/wave/internal/builder"
	"github.com/vormadev/vorma/wave/internal/config"
	"golang.org/x/sync/errgroup"
)

// fileType categorizes the type of file that changed
type fileType int

const (
	fileTypeOther fileType = iota
	fileTypeGo
	fileTypeCriticalCSS
	fileTypeNormalCSS
	fileTypePublicStatic
)

// classifiedEvent contains all analysis of a file change event
type classifiedEvent struct {
	event       fsnotify.Event
	fileType    fileType
	watchedFile *config.WatchedFile
	ignored     bool
	chmodOnly   bool
	strategy    *config.OnChangeStrategy
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
	// Dedupe by path
	eventMap := make(map[string]fsnotify.Event)
	for _, evt := range events {
		eventMap[evt.Name] = evt
	}

	var relevantEvents []classifiedEvent
	needsHardReload := false
	needsGoCompile := false
	handledPatterns := make(map[string]bool)

	var strategyEvents []classifiedEvent

	for _, evt := range eventMap {
		// Config file changed -> full restart with Go recompile
		if s.isConfigFile(evt.Name) && (evt.Has(fsnotify.Write) || evt.Has(fsnotify.Create)) {
			s.log.Info("Config changed, restarting")
			s.triggerConfigRestart()
			return
		}

		// Handle new directories
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

		// Dedupe by pattern
		patternKey := ""
		if classified.watchedFile != nil {
			patternKey = classified.watchedFile.Pattern
		}
		if handledPatterns[patternKey] {
			continue
		}
		handledPatterns[patternKey] = true

		if classified.strategy != nil {
			if classified.chmodOnly {
				continue
			}
			strategyEvents = append(strategyEvents, classified)
			continue
		}

		if classified.fileType == fileTypeGo {
			needsGoCompile = true
		}

		if !needsHardReload {
			needsHardReload = classified.fileType == fileTypeGo || s.needsHardReload(classified.watchedFile)
		}

		relevantEvents = append(relevantEvents, classified)
	}

	for _, classified := range strategyEvents {
		s.log.Info("File changed (strategy)", "op", classified.event.Op.String(), "file", classified.event.Name)
		if err := s.executeStrategy(classified.strategy); err != nil {
			s.log.Warn("Strategy execution failed", "error", err)
		}
	}

	if len(relevantEvents) == 0 {
		return
	}

	// Skip if all events are chmod-only on non-empty files
	allChmod := true
	for _, e := range relevantEvents {
		if !e.chmodOnly {
			allChmod = false
			break
		}
	}
	if allChmod {
		return
	}

	isBatch := len(relevantEvents) > 1

	if isBatch || needsHardReload {
		s.broadcastRebuilding()
	}

	// For batch + hard reload, stop app before compiling
	if isBatch && needsHardReload {
		s.log.Info("Shutting down running app (batch)")
		if err := s.stopApp(); err != nil {
			panic(fmt.Sprintf("failed to stop app: %v", err))
		}
	}

	// For batches with Go files: compile Go once, not per-file
	if isBatch && needsGoCompile {
		s.log.Info("Batch Go change detected, compiling once")
		if err := s.builder.CompileGoOnly(true); err != nil {
			s.log.Error("Go compilation failed", "error", err)
		}
	}

	for _, classified := range relevantEvents {
		s.log.Info("File changed", "op", classified.event.Op.String(), "file", classified.event.Name)

		// Skip Go compilation for individual files if we already did batch compile
		skipGoCompile := isBatch && needsGoCompile && classified.fileType == fileTypeGo

		if err := s.handleFileChange(classified, isBatch, skipGoCompile); err != nil {
			s.log.Error("Handle change failed", "error", err)
		}
	}

	if isBatch && needsHardReload {
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

func (s *server) executeStrategy(strategy *config.OnChangeStrategy) error {
	if strategy == nil {
		return nil
	}

	if strategy.HttpEndpoint != "" {
		s.log.Info("Executing strategy HTTP endpoint", "endpoint", strategy.HttpEndpoint)

		// Ensure app is ready before calling endpoint
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
	url := fmt.Sprintf("http://localhost:%d%s", config.MustGetAppPort(), endpoint)

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

func (s *server) handleStrategyFallback(strategy *config.OnChangeStrategy, originalErr error) error {
	s.log.Warn("Strategy failed, executing fallback",
		"error", originalErr,
		"fallback", strategy.FallbackAction,
	)

	switch strategy.FallbackAction {
	case config.FallbackRestart:
		s.triggerRestart()
		return nil
	case config.FallbackRestartNoGo:
		s.triggerRestartNoGo()
		return nil
	case config.FallbackNone, "":
		return originalErr
	default:
		s.log.Warn("Unknown fallback action", "action", strategy.FallbackAction)
		return originalErr
	}
}

func (s *server) classifyEvent(evt fsnotify.Event) classifiedEvent {
	result := classifiedEvent{
		event: evt,
	}

	if evt.Name == "" {
		result.ignored = true
		return result
	}

	// Check if ignored
	result.ignored = s.watcher.IsIgnoredFile(evt.Name)

	// Determine file type using builder's CSS tracking
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

	// Find matching watched file config
	result.watchedFile = s.watcher.FindWatchedFile(evt.Name)

	// Handle TreatAsNonGo override
	if result.fileType == fileTypeGo && result.watchedFile != nil && result.watchedFile.TreatAsNonGo {
		result.fileType = fileTypeOther
	}

	// Ignore if not Go, not CSS, not public static, and no watched file
	if result.fileType == fileTypeOther && result.watchedFile == nil {
		result.ignored = true
	}

	result.strategy = s.watcher.GetFirstStrategy(result.watchedFile)

	// Check if chmod-only on non-empty file (skip these)
	result.chmodOnly = isNonEmptyChmodOnly(evt)

	return result
}

func (s *server) isConfigFile(path string) bool {
	configPath := s.cfg.Core.ConfigLocation
	if configPath == "" {
		return false
	}

	// Use absolute paths for reliable comparison.
	// The path from fsnotify may be absolute or relative depending on how
	// the directory was added to the watcher.
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

func (s *server) needsHardReload(wf *config.WatchedFile) bool {
	if wf == nil {
		return false
	}
	return wf.RecompileGoBinary || wf.RestartApp
}

func (s *server) handleFileChange(classified classifiedEvent, isBatch bool, skipGoCompile bool) error {
	wf := classified.watchedFile
	if wf == nil {
		wf = &config.WatchedFile{}
	}
	if wf.SortedHooks == nil {
		wf.SortedHooks = &config.SortedHooks{}
	}

	sorted := wf.SortedHooks

	isCSS := classified.fileType == fileTypeCriticalCSS || classified.fileType == fileTypeNormalCSS
	isGo := classified.fileType == fileTypeGo
	isPublicStatic := classified.fileType == fileTypePublicStatic

	if !isBatch && !wf.SkipRebuildingNotification && !isCSS && !isPublicStatic {
		s.broadcastRebuilding()
	}

	// Check both watched file flags AND whether this is a Go file
	needsRestart := (isGo || s.needsHardReload(wf)) && !isBatch

	// Start killing app in parallel with pre-hooks
	var stopEg errgroup.Group
	if needsRestart {
		stopEg.Go(func() error {
			s.log.Info("Terminating running app")
			return s.stopApp()
		})
	}

	// Fire-and-forget hooks (concurrent-no-wait)
	for _, hook := range sorted.ConcurrentNoWait {
		if hook.HasStrategy() {
			continue
		}
		if s.watcher.IsIgnored(classified.event.Name, hook.Exclude) {
			continue
		}
		cmd := s.resolveCmd(hook.Cmd)
		if cmd == "" {
			continue
		}
		go func(c string) {
			if err := builder.RunShellCommand(context.Background(), c); err != nil {
				s.log.Warn("concurrent-no-wait hook failed", "cmd", c, "error", err)
			}
		}(cmd)
	}

	// Pre hooks (sequential, but run while app is stopping)
	for _, hook := range sorted.Pre {
		if hook.HasStrategy() {
			continue
		}
		if s.watcher.IsIgnored(classified.event.Name, hook.Exclude) {
			continue
		}
		cmd := s.resolveCmd(hook.Cmd)
		if cmd == "" {
			continue
		}
		if err := builder.RunShellCommand(context.Background(), cmd); err != nil {
			return err
		}
	}

	// If RunOnChangeOnly, return early after pre hooks
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

	// Run callback in parallel with concurrent hooks
	var callbackEg errgroup.Group

	// Main callback runs in parallel
	callbackEg.Go(func() error {
		return s.fileChangeCallback(wf, classified, skipGoCompile)
	})

	// Concurrent hooks run in parallel with callback
	for _, hook := range sorted.Concurrent {
		if hook.HasStrategy() {
			continue
		}
		if s.watcher.IsIgnored(classified.event.Name, hook.Exclude) {
			continue
		}
		h := hook
		callbackEg.Go(func() error {
			cmd := s.resolveCmd(h.Cmd)
			if cmd == "" {
				return nil
			}
			return builder.RunShellCommand(context.Background(), cmd)
		})
	}

	// Wait for callback + concurrent hooks
	if err := callbackEg.Wait(); err != nil {
		return err
	}

	// Post hooks (sequential)
	for _, hook := range sorted.Post {
		if hook.HasStrategy() {
			continue
		}
		if s.watcher.IsIgnored(classified.event.Name, hook.Exclude) {
			continue
		}
		cmd := s.resolveCmd(hook.Cmd)
		if cmd == "" {
			continue
		}
		if err := builder.RunShellCommand(context.Background(), cmd); err != nil {
			return err
		}
	}

	// Wait for stop and panic on failure
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

func (s *server) fileChangeCallback(wf *config.WatchedFile, classified classifiedEvent, skipGoCompile bool) error {
	isCSS := classified.fileType == fileTypeCriticalCSS || classified.fileType == fileTypeNormalCSS

	if classified.fileType == fileTypeGo {
		if skipGoCompile {
			return nil // Already compiled in batch
		}
		return s.builder.CompileGoOnly(true)
	}

	if classified.fileType == fileTypePublicStatic {
		if err := s.builder.ProcessPublicFilesOnly(); err != nil {
			return err
		}
		// Write TS filemap for Vite HMR using generic config
		if s.cfg.FrameworkPublicFileMapOutDir != "" {
			return s.builder.WritePublicFileMapTS(s.cfg.FrameworkPublicFileMapOutDir)
		}
		return nil
	}

	if isCSS {
		// Rebuild CSS only
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

	return s.builder.Build(builder.Opts{
		IsDev:        true,
		CompileGo:    wf.RecompileGoBinary,
		IsRebuild:    true,
		FileOnlyMode: isCSS || wf.OnlyRunClientDefinedRevalidateFunc,
	})
}

func (s *server) handleBrowserReload(classified classifiedEvent, wf *config.WatchedFile) error {
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

	// Public static files need Vite cycle because filemap.ts changed
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

	// CSS hot reload
	changeType := changeTypeNormalCSS
	if classified.fileType == fileTypeCriticalCSS {
		changeType = changeTypeCriticalCSS
	}

	// Read CSS content from builder
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

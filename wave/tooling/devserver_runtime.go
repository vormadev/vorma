package tooling

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

func (s *server) run() error {
	firstRun := true
	recompileGo := true // First run always compiles
	isConfigRestart := false

	// Initialize refresh server once (crucial -- persists across rebuilds)
	s.mustGetPort()

	if s.cfg.UsingBrowser() {
		s.refreshMgrCtx, s.refreshMgrCancel = context.WithCancel(context.Background())
		s.refreshMgr = newClientManager()
		go s.refreshMgr.start(s.refreshMgrCtx)
		if _, err := s.startRefreshServer(defaultRefreshPort); err != nil {
			return fmt.Errorf("start refresh server: %w", err)
		}
	}

	// Ensure refresh server is cleaned up on exit
	defer s.cleanupRefreshServer()

	for {
		if !firstRun {
			if err := s.reloadConfig(); err != nil {
				s.log.Error("config reload failed", "error", err)
			}
		}

		// Create/recreate builder with current config
		s.setBuilder(NewBuilder(s.cfg, s.log))

		if err := s.initWatcher(); err != nil {
			return fmt.Errorf("init watcher: %w", err)
		}

		isRebuild := !firstRun
		sequentialGo := s.cfg.Core.SequentialGoBuild

		// Run builds - either in parallel or sequentially based on config
		var buildEg errgroup.Group

		buildEg.Go(func() error {
			b := s.getBuilder()
			if b == nil {
				return fmt.Errorf("builder is nil")
			}
			return b.Build(BuildOpts{
				IsDev:     true,
				CompileGo: false,
				IsRebuild: isRebuild,
			})
		})

		// Compile Go concurrently only if recompileGo is true AND sequential mode is disabled
		if recompileGo && !sequentialGo {
			buildEg.Go(func() error {
				b := s.getBuilder()
				if b == nil {
					return fmt.Errorf("builder is nil")
				}
				return b.CompileGoOnly(true)
			})
		}

		if err := buildEg.Wait(); err != nil {
			s.log.Error("build failed", "error", err)
			s.log.Info("Waiting for file changes to retry build...")
			s.waitForBuildRetry()
			firstRun = false
			continue
		}

		// If sequential mode is enabled, compile Go after build hooks have completed
		if recompileGo && sequentialGo {
			b := s.getBuilder()
			if b == nil {
				s.log.Error("builder is nil for sequential Go compile")
				s.log.Info("Waiting for file changes to retry build...")
				s.waitForBuildRetry()
				firstRun = false
				continue
			}
			if err := b.CompileGoOnly(true); err != nil {
				s.log.Error("go compilation failed", "error", err)
				s.log.Info("Waiting for file changes to retry build...")
				s.waitForBuildRetry()
				firstRun = false
				continue
			}
		}

		// Start Vite AFTER build completes (TypeScript files now exist)
		if s.viteCtx == nil && s.cfg.UsingVite() {
			if err := s.startVite(); err != nil {
				s.log.Error("vite start failed", "error", err)
			}
		}

		// Start the app
		s.startApp()

		// Initialize watcher start channel for this iteration
		s.watcherStartCh = make(chan struct{})

		// Start watching in a goroutine that waits for signal
		go func() {
			<-s.watcherStartCh
			s.runWatcher()
		}()

		// If this was a config restart, broadcast reload after app is ready,
		// THEN signal watcher to start (prevents watcher from triggering reload first)
		if isConfigRestart {
			s.broadcastReload(reloadOpts{
				payload:   refreshPayload{ChangeType: changeTypeOther},
				waitApp:   true,
				waitVite:  true,
				cycleVite: true,
			})
			isConfigRestart = false
		}

		// Now signal watcher to start processing events
		close(s.watcherStartCh)

		firstRun = false

		// Wait for restart request
		restartRequestForRun := <-s.restartCh
		recompileGo = restartRequestForRun.recompileGo
		isConfigRestart = restartRequestForRun.isConfigRestart
		s.log.Info("Restarting dev server...", "recompile_go", recompileGo, "config_restart", isConfigRestart)

		// Send rebuilding signal while refresh server is still alive
		s.broadcastRebuilding()

		// Clean up everything except refresh server and Vite
		s.cleanupForRebuild()
	}
}

// waitForBuildRetry waits for a file change that might fix the build error.
// It starts the watcher and waits for any restart request.
func (s *server) waitForBuildRetry() {
	// Initialize watcher start channel
	s.watcherStartCh = make(chan struct{})

	// Start watcher immediately since we're waiting for fixes
	go func() {
		<-s.watcherStartCh
		s.runWatcher()
	}()
	close(s.watcherStartCh)

	// Wait for any file change to trigger a restart
	<-s.restartCh

	// Clean up for the retry
	s.cleanupForRebuild()
}

// cleanupForRebuild cleans up resources but keeps refresh server and Vite alive
func (s *server) cleanupForRebuild() {
	if err := s.stopApp(); err != nil {
		s.log.Error("stop app failed", "error", err)
	}

	// Don't stop Vite here - it's cycled in broadcastReload when needed

	// Close watcher and set to nil under lock to prevent race with processEvents
	s.mu.Lock()
	watcher := s.watcher
	s.watcher = nil
	s.mu.Unlock()

	if watcher != nil {
		if err := watcher.Close(); err != nil {
			s.log.Error("close watcher failed", "error", err)
		}
	}

	// Close builder under lock
	s.mu.Lock()
	builder := s.builder
	s.builder = nil
	s.mu.Unlock()

	if builder != nil {
		if err := builder.Close(); err != nil {
			s.log.Error("close builder failed", "error", err)
		}
	}
}

// cleanupRefreshServer cleans up the refresh server (called on full shutdown)
func (s *server) cleanupRefreshServer() {
	if err := s.stopRefreshServer(); err != nil {
		s.log.Error("stop refresh server failed", "error", err)
	}

	if s.refreshMgrCancel != nil {
		s.refreshMgrCancel()
		if s.refreshMgr != nil {
			s.refreshMgr.wait()
		}
		s.refreshMgrCancel = nil
	}
}

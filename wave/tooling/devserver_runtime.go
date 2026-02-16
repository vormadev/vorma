package tooling

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

type runIntent struct {
	recompileGo     bool
	isConfigRestart bool
}

type runLifecycleCommand string

const (
	runLifecycleCommandPrepareCycle        runLifecycleCommand = "prepare_cycle"
	runLifecycleCommandBuildCycle          runLifecycleCommand = "build_cycle"
	runLifecycleCommandAwaitBuildRetry     runLifecycleCommand = "await_build_retry"
	runLifecycleCommandStartRuntime        runLifecycleCommand = "start_runtime"
	runLifecycleCommandAwaitRestartRequest runLifecycleCommand = "await_restart_request"
	runLifecycleCommandCleanupForNextCycle runLifecycleCommand = "cleanup_for_next_cycle"
)

type runLifecycleCommandInput struct {
	firstRun         bool
	currentRunIntent runIntent
}

type runLifecycleCommandResult struct {
	runLifecycleEvent               runLifecycleEvent
	updatedRunIntent                *runIntent
	shouldMarkFirstRunAsNotFirstRun bool
}

func deriveRunIntentFromRestartRequest(
	restartRequestForIntent restartRequest,
) runIntent {
	normalizedRestartRequest := normalizeRestartRequest(restartRequestForIntent)
	return runIntent{
		recompileGo:     normalizedRestartRequest.recompileGo,
		isConfigRestart: normalizedRestartRequest.isConfigRestart,
	}
}

func (s *server) run() error {
	firstRun := true
	currentRunIntent := runIntent{
		recompileGo: true, // First run always compiles
	}
	currentRunLifecycleState := runLifecycleStatePreparingCycle
	currentLifecycleCycleID := uint64(1)

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
		runLifecycleCommandForState, err := deriveRunLifecycleCommandForState(
			currentRunLifecycleState,
		)
		if err != nil {
			return err
		}

		runLifecycleCommandResultForState, err := s.executeRunLifecycleCommand(
			runLifecycleCommandForState,
			runLifecycleCommandInput{
				firstRun:         firstRun,
				currentRunIntent: currentRunIntent,
			},
		)
		if err != nil {
			return err
		}

		if runLifecycleCommandResultForState.updatedRunIntent != nil {
			currentRunIntent = *runLifecycleCommandResultForState.updatedRunIntent
		}
		if runLifecycleCommandResultForState.shouldMarkFirstRunAsNotFirstRun {
			firstRun = false
		}

		nextRunLifecycleState, err := s.transitionRunLifecycleState(
			currentRunLifecycleState,
			runLifecycleCommandResultForState.runLifecycleEvent,
			currentLifecycleCycleID,
		)
		if err != nil {
			return err
		}
		currentRunLifecycleState = nextRunLifecycleState
		if currentRunLifecycleState == runLifecycleStatePreparingCycle &&
			runLifecycleCommandForState != runLifecycleCommandPrepareCycle {
			currentLifecycleCycleID++
		}
	}
}

func deriveRunLifecycleCommandForState(
	currentRunLifecycleState runLifecycleState,
) (runLifecycleCommand, error) {
	switch currentRunLifecycleState {
	case runLifecycleStatePreparingCycle:
		return runLifecycleCommandPrepareCycle, nil
	case runLifecycleStateBuildingCycle:
		return runLifecycleCommandBuildCycle, nil
	case runLifecycleStateAwaitingBuildRetry:
		return runLifecycleCommandAwaitBuildRetry, nil
	case runLifecycleStateStartingRuntime:
		return runLifecycleCommandStartRuntime, nil
	case runLifecycleStateAwaitingRestart:
		return runLifecycleCommandAwaitRestartRequest, nil
	case runLifecycleStateCleaningUpForNextCycle:
		return runLifecycleCommandCleanupForNextCycle, nil
	default:
		return "", fmt.Errorf("unknown run lifecycle state: %q", currentRunLifecycleState)
	}
}

func (s *server) executeRunLifecycleCommand(
	runLifecycleCommandForState runLifecycleCommand,
	runLifecycleCommandInputForState runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	switch runLifecycleCommandForState {
	case runLifecycleCommandPrepareCycle:
		return s.executeRunLifecycleCommandPrepareCycle(
			runLifecycleCommandInputForState,
		)
	case runLifecycleCommandBuildCycle:
		return s.executeRunLifecycleCommandBuildCycle(
			runLifecycleCommandInputForState,
		)
	case runLifecycleCommandAwaitBuildRetry:
		return s.executeRunLifecycleCommandAwaitBuildRetry(
			runLifecycleCommandInputForState,
		)
	case runLifecycleCommandStartRuntime:
		return s.executeRunLifecycleCommandStartRuntime(
			runLifecycleCommandInputForState,
		)
	case runLifecycleCommandAwaitRestartRequest:
		return s.executeRunLifecycleCommandAwaitRestartRequest(
			runLifecycleCommandInputForState,
		)
	case runLifecycleCommandCleanupForNextCycle:
		return s.executeRunLifecycleCommandCleanupForNextCycle(
			runLifecycleCommandInputForState,
		)
	default:
		return runLifecycleCommandResult{}, fmt.Errorf(
			"unknown run lifecycle command: %q",
			runLifecycleCommandForState,
		)
	}
}

func (s *server) prepareRunCycle(firstRun bool) error {
	if !firstRun {
		if err := s.reloadConfig(); err != nil {
			s.log.Error("config reload failed", "error", err)
		}
	}

	// Create/recreate builder with current config.
	s.setBuilder(NewBuilder(s.cfg, s.log))

	if err := s.initWatcher(); err != nil {
		return fmt.Errorf("init watcher: %w", err)
	}

	return nil
}

func (s *server) executeRunBuildForIntent(
	isRebuild bool,
	currentRunIntent runIntent,
) error {
	buildExecutionOrderingDecision := deriveRunBuildExecutionOrderingDecision(
		currentRunIntent.recompileGo,
		s.cfg.Core.SequentialGoBuild,
	)
	s.log.Debug(
		"resolved build execution ordering decision",
		"go_compilation_ordering_policy",
		buildExecutionOrderingDecision.goCompilationOrderingPolicy,
		"compile_in_parallel",
		buildExecutionOrderingDecision.runCompileInParallel,
		"compile_after_build_hooks",
		buildExecutionOrderingDecision.runCompileAfterBuildHooks,
	)

	// Run builds - either in parallel or sequentially based on config.
	var buildGroup errgroup.Group

	buildGroup.Go(func() error {
		builderForBuild := s.getBuilder()
		if builderForBuild == nil {
			return fmt.Errorf("builder is nil")
		}
		return builderForBuild.Build(BuildOpts{
			IsDev:     true,
			CompileGo: false,
			IsRebuild: isRebuild,
		})
	})

	if buildExecutionOrderingDecision.runCompileInParallel {
		buildGroup.Go(func() error {
			builderForCompile := s.getBuilder()
			if builderForCompile == nil {
				return fmt.Errorf("builder is nil")
			}
			return builderForCompile.CompileGoOnly(true)
		})
	}

	if err := buildGroup.Wait(); err != nil {
		return err
	}

	if buildExecutionOrderingDecision.runCompileAfterBuildHooks {
		builderForSequentialCompile := s.getBuilder()
		if builderForSequentialCompile == nil {
			return fmt.Errorf("builder is nil for sequential Go compile")
		}
		if err := builderForSequentialCompile.CompileGoOnly(true); err != nil {
			return fmt.Errorf("go compilation failed: %w", err)
		}
	}

	return nil
}

func (s *server) startRunCycleRuntime(currentRunIntent *runIntent) {
	// Start Vite after build completes (TypeScript files now exist).
	if s.viteCtx == nil && s.cfg.UsingVite() {
		if err := s.startVite(); err != nil {
			s.log.Error("vite start failed", "error", err)
		}
	}

	// Start the app.
	s.startApp()

	currentRunCycleScope := s.startRunCycleScope()

	// Initialize watcher start channel for this iteration.
	s.watcherStartCh = make(chan struct{})

	// Start watching in a cycle-scoped goroutine that waits for signal.
	if currentRunCycleScope != nil {
		currentRunCycleScope.launchAsyncWork(func(
			currentRunCycleContext context.Context,
		) {
			select {
			case <-s.watcherStartCh:
			case <-currentRunCycleContext.Done():
				return
			}
			s.runWatcherWithContext(currentRunCycleContext)
		})
	}

	// If this was a config restart, broadcast reload after app is ready,
	// then signal watcher to start (prevents watcher from triggering reload first).
	if currentRunIntent != nil && currentRunIntent.isConfigRestart {
		s.broadcastReload(reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeOther},
			waitApp:   true,
			waitVite:  true,
			cycleVite: true,
		})
		currentRunIntent.isConfigRestart = false
	}

	// Now signal watcher to start processing events.
	close(s.watcherStartCh)
}

// waitForBuildRetry waits for a file change that might fix the build error.
// It starts the watcher and waits for any restart request.
func (s *server) waitForBuildRetry() restartRequest {
	s.setWaitingForBuildRetry(true)
	defer s.setWaitingForBuildRetry(false)

	if pendingRestartRequest, hasPendingRestartRequest := s.consumePendingRestartRequest(); hasPendingRestartRequest {
		normalizedPendingRestartRequest := normalizeRestartRequest(pendingRestartRequest)
		s.cleanupForRebuild()
		return normalizedPendingRestartRequest
	}

	// Initialize watcher start channel
	s.watcherStartCh = make(chan struct{})
	buildRetryRunCycleScope := s.startRunCycleScope()

	// Start watcher immediately since we're waiting for fixes
	if buildRetryRunCycleScope != nil {
		buildRetryRunCycleScope.launchAsyncWork(func(
			buildRetryRunCycleContext context.Context,
		) {
			select {
			case <-s.watcherStartCh:
			case <-buildRetryRunCycleContext.Done():
				return
			}
			s.runWatcherWithContext(buildRetryRunCycleContext)
		})
	}
	close(s.watcherStartCh)

	// Wait for any file change to trigger a restart
	restartRequestForRetry := s.consumeRestartRequestBlocking()

	// Clean up for the retry
	s.cleanupForRebuild()
	return restartRequestForRetry
}

// cleanupForRebuild cleans up resources but keeps refresh server and Vite alive
func (s *server) cleanupForRebuild() {
	s.cancelAndJoinCurrentRunCycleScope()
	s.cancelConcurrentNoWaitHookLifecycleContext()

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
	s.cancelAndJoinCurrentRunCycleScope()
	s.cancelConcurrentNoWaitHookLifecycleContext()

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

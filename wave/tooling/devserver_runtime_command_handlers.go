package tooling

func (s *server) executeRunLifecycleCommandPrepareCycle(
	runLifecycleCommandInputForState runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	if err := s.prepareRunCycle(runLifecycleCommandInputForState.firstRun); err != nil {
		return runLifecycleCommandResult{}, err
	}
	return runLifecycleCommandResult{
		runLifecycleEvent: runLifecycleEventCyclePrepared,
	}, nil
}

func (s *server) executeRunLifecycleCommandBuildCycle(
	runLifecycleCommandInputForState runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	if err := s.executeRunBuildForIntent(
		!runLifecycleCommandInputForState.firstRun,
		runLifecycleCommandInputForState.currentRunIntent,
	); err != nil {
		s.log.Error("build failed", "error", err)
		return runLifecycleCommandResult{
			runLifecycleEvent: runLifecycleEventBuildFailed,
		}, nil
	}
	return runLifecycleCommandResult{
		runLifecycleEvent: runLifecycleEventBuildSucceeded,
	}, nil
}

func (s *server) executeRunLifecycleCommandAwaitBuildRetry(
	_ runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	s.log.Info("Waiting for file changes to retry build...")
	restartRequestForRetry := s.waitForBuildRetry()
	currentRunIntentForRetry := deriveRunIntentFromRestartRequest(
		restartRequestForRetry,
	)
	return runLifecycleCommandResult{
		runLifecycleEvent:               runLifecycleEventBuildRetryRestartReceived,
		updatedRunIntent:                &currentRunIntentForRetry,
		shouldMarkFirstRunAsNotFirstRun: true,
	}, nil
}

func (s *server) executeRunLifecycleCommandStartRuntime(
	runLifecycleCommandInputForState runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	currentRunIntentForRuntime := runLifecycleCommandInputForState.currentRunIntent
	s.startRunCycleRuntime(&currentRunIntentForRuntime)
	return runLifecycleCommandResult{
		runLifecycleEvent:               runLifecycleEventRuntimeStarted,
		updatedRunIntent:                &currentRunIntentForRuntime,
		shouldMarkFirstRunAsNotFirstRun: true,
	}, nil
}

func (s *server) executeRunLifecycleCommandAwaitRestartRequest(
	_ runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	restartRequestForRun := s.consumeRestartRequestBlocking()
	currentRunIntentForRestart := deriveRunIntentFromRestartRequest(
		restartRequestForRun,
	)
	s.log.Info(
		"Restarting dev server...",
		"recompile_go",
		currentRunIntentForRestart.recompileGo,
		"config_restart",
		currentRunIntentForRestart.isConfigRestart,
	)
	return runLifecycleCommandResult{
		runLifecycleEvent: runLifecycleEventRestartRequestReceived,
		updatedRunIntent:  &currentRunIntentForRestart,
	}, nil
}

func (s *server) executeRunLifecycleCommandCleanupForNextCycle(
	_ runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	// Send rebuilding signal while refresh server is still alive.
	s.broadcastRebuilding()

	// Clean up everything except refresh server and Vite.
	s.cleanupForRebuild()

	return runLifecycleCommandResult{
		runLifecycleEvent: runLifecycleEventCleanupCompleted,
	}, nil
}


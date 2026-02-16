package tooling

import "fmt"

type runLifecycleState string

const (
	runLifecycleStatePreparingCycle         runLifecycleState = "preparing_cycle"
	runLifecycleStateBuildingCycle          runLifecycleState = "building_cycle"
	runLifecycleStateAwaitingBuildRetry     runLifecycleState = "awaiting_build_retry"
	runLifecycleStateStartingRuntime        runLifecycleState = "starting_runtime"
	runLifecycleStateAwaitingRestart        runLifecycleState = "awaiting_restart"
	runLifecycleStateCleaningUpForNextCycle runLifecycleState = "cleaning_up_for_next_cycle"
)

type runLifecycleEvent string

const (
	runLifecycleEventCyclePrepared             runLifecycleEvent = "cycle_prepared"
	runLifecycleEventBuildSucceeded            runLifecycleEvent = "build_succeeded"
	runLifecycleEventBuildFailed               runLifecycleEvent = "build_failed"
	runLifecycleEventBuildRetryRestartReceived runLifecycleEvent = "build_retry_restart_received"
	runLifecycleEventRuntimeStarted            runLifecycleEvent = "runtime_started"
	runLifecycleEventRestartRequestReceived    runLifecycleEvent = "restart_request_received"
	runLifecycleEventCleanupCompleted          runLifecycleEvent = "cleanup_completed"
)

func deriveRunLifecycleStateAfterEvent(
	currentRunLifecycleState runLifecycleState,
	runLifecycleEventForTransition runLifecycleEvent,
) (runLifecycleState, error) {
	switch currentRunLifecycleState {
	case runLifecycleStatePreparingCycle:
		if runLifecycleEventForTransition == runLifecycleEventCyclePrepared {
			return runLifecycleStateBuildingCycle, nil
		}

	case runLifecycleStateBuildingCycle:
		if runLifecycleEventForTransition == runLifecycleEventBuildSucceeded {
			return runLifecycleStateStartingRuntime, nil
		}
		if runLifecycleEventForTransition == runLifecycleEventBuildFailed {
			return runLifecycleStateAwaitingBuildRetry, nil
		}

	case runLifecycleStateAwaitingBuildRetry:
		if runLifecycleEventForTransition == runLifecycleEventBuildRetryRestartReceived {
			return runLifecycleStatePreparingCycle, nil
		}

	case runLifecycleStateStartingRuntime:
		if runLifecycleEventForTransition == runLifecycleEventRuntimeStarted {
			return runLifecycleStateAwaitingRestart, nil
		}

	case runLifecycleStateAwaitingRestart:
		if runLifecycleEventForTransition == runLifecycleEventRestartRequestReceived {
			return runLifecycleStateCleaningUpForNextCycle, nil
		}

	case runLifecycleStateCleaningUpForNextCycle:
		if runLifecycleEventForTransition == runLifecycleEventCleanupCompleted {
			return runLifecycleStatePreparingCycle, nil
		}
	}

	return "", fmt.Errorf(
		"invalid run lifecycle transition: state=%q event=%q",
		currentRunLifecycleState,
		runLifecycleEventForTransition,
	)
}

func (s *server) transitionRunLifecycleState(
	currentRunLifecycleState runLifecycleState,
	runLifecycleEventForTransition runLifecycleEvent,
	currentLifecycleCycleID uint64,
) (runLifecycleState, error) {
	nextRunLifecycleState, err := deriveRunLifecycleStateAfterEvent(
		currentRunLifecycleState,
		runLifecycleEventForTransition,
	)
	if err != nil {
		return "", err
	}

	if s != nil && s.log != nil {
		s.log.Info(
			"run lifecycle transition",
			"cycle_id",
			currentLifecycleCycleID,
			"from",
			currentRunLifecycleState,
			"event",
			runLifecycleEventForTransition,
			"to",
			nextRunLifecycleState,
		)
	}

	return nextRunLifecycleState, nil
}

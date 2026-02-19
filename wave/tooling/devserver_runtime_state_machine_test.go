package tooling

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/tooling/devserver/devserverengine"
)

func TestDeriveRunLifecycleStateAfterEvent_ValidTransitions(t *testing.T) {
	testCases := []struct {
		Name                     string
		CurrentRunLifecycleState devserverengine.RunLifecycleState
		RunLifecycleEvent        devserverengine.RunLifecycleEvent
		ExpectedNextState        devserverengine.RunLifecycleState
	}{
		{
			Name:                     "prepare to build",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStatePreparingCycle,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventCyclePrepared,
			ExpectedNextState:        devserverengine.RunLifecycleStateBuildingCycle,
		},
		{
			Name:                     "build success to runtime start",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateBuildingCycle,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventBuildSucceeded,
			ExpectedNextState:        devserverengine.RunLifecycleStateStartingRuntime,
		},
		{
			Name:                     "build failure to retry wait",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateBuildingCycle,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventBuildFailed,
			ExpectedNextState:        devserverengine.RunLifecycleStateAwaitingBuildRetry,
		},
		{
			Name:                     "retry restart to prepare",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateAwaitingBuildRetry,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventBuildRetryRestartReceived,
			ExpectedNextState:        devserverengine.RunLifecycleStatePreparingCycle,
		},
		{
			Name:                     "runtime start to restart wait",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateStartingRuntime,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventRuntimeStarted,
			ExpectedNextState:        devserverengine.RunLifecycleStateAwaitingRestart,
		},
		{
			Name:                     "restart request to cleanup",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateAwaitingRestart,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventRestartRequestReceived,
			ExpectedNextState:        devserverengine.RunLifecycleStateCleaningUpForNextCycle,
		},
		{
			Name:                     "cleanup to prepare",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateCleaningUpForNextCycle,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventCleanupCompleted,
			ExpectedNextState:        devserverengine.RunLifecycleStatePreparingCycle,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			nextRunLifecycleState, err := devserverengine.DeriveRunLifecycleStateAfterEvent(
				testCase.CurrentRunLifecycleState,
				testCase.RunLifecycleEvent,
			)
			if err != nil {
				t.Fatalf(
					"deriveRunLifecycleStateAfterEvent(%q, %q) returned error: %v",
					testCase.CurrentRunLifecycleState,
					testCase.RunLifecycleEvent,
					err,
				)
			}
			if nextRunLifecycleState != testCase.ExpectedNextState {
				t.Fatalf(
					"deriveRunLifecycleStateAfterEvent(%q, %q) = %q, want %q",
					testCase.CurrentRunLifecycleState,
					testCase.RunLifecycleEvent,
					nextRunLifecycleState,
					testCase.ExpectedNextState,
				)
			}
		})
	}
}

func TestDeriveRunLifecycleStateAfterEvent_InvalidTransitionsReturnError(
	t *testing.T,
) {
	testCases := []struct {
		Name                     string
		CurrentRunLifecycleState devserverengine.RunLifecycleState
		RunLifecycleEvent        devserverengine.RunLifecycleEvent
	}{
		{
			Name:                     "prepare rejects build failed event",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStatePreparingCycle,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventBuildFailed,
		},
		{
			Name:                     "build rejects cleanup event",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateBuildingCycle,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventCleanupCompleted,
		},
		{
			Name:                     "retry wait rejects runtime started event",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateAwaitingBuildRetry,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventRuntimeStarted,
		},
		{
			Name:                     "runtime start rejects restart received event",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateStartingRuntime,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventRestartRequestReceived,
		},
		{
			Name:                     "restart wait rejects cycle prepared event",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateAwaitingRestart,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventCyclePrepared,
		},
		{
			Name:                     "cleanup rejects build success event",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateCleaningUpForNextCycle,
			RunLifecycleEvent:        devserverengine.RunLifecycleEventBuildSucceeded,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			nextRunLifecycleState, err := devserverengine.DeriveRunLifecycleStateAfterEvent(
				testCase.CurrentRunLifecycleState,
				testCase.RunLifecycleEvent,
			)
			if err == nil {
				t.Fatalf(
					"deriveRunLifecycleStateAfterEvent(%q, %q) expected error, got next state %q",
					testCase.CurrentRunLifecycleState,
					testCase.RunLifecycleEvent,
					nextRunLifecycleState,
				)
			}
		})
	}
}

func TestDeriveRunLifecycleCommandForState(t *testing.T) {
	testCases := []struct {
		Name                     string
		CurrentRunLifecycleState devserverengine.RunLifecycleState
		ExpectedCommand          devserverengine.RunLifecycleCommand
	}{
		{
			Name:                     "prepare state maps to prepare command",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStatePreparingCycle,
			ExpectedCommand:          devserverengine.RunLifecycleCommandPrepareCycle,
		},
		{
			Name:                     "build state maps to build command",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateBuildingCycle,
			ExpectedCommand:          devserverengine.RunLifecycleCommandBuildCycle,
		},
		{
			Name:                     "retry state maps to retry command",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateAwaitingBuildRetry,
			ExpectedCommand:          devserverengine.RunLifecycleCommandAwaitBuildRetry,
		},
		{
			Name:                     "runtime state maps to start-runtime command",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateStartingRuntime,
			ExpectedCommand:          devserverengine.RunLifecycleCommandStartRuntime,
		},
		{
			Name:                     "await-restart state maps to await-restart command",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateAwaitingRestart,
			ExpectedCommand:          devserverengine.RunLifecycleCommandAwaitRestartRequest,
		},
		{
			Name:                     "cleanup state maps to cleanup command",
			CurrentRunLifecycleState: devserverengine.RunLifecycleStateCleaningUpForNextCycle,
			ExpectedCommand:          devserverengine.RunLifecycleCommandCleanupForNextCycle,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			runLifecycleCommandForState, err := devserverengine.DeriveRunLifecycleCommandForState(
				testCase.CurrentRunLifecycleState,
			)
			if err != nil {
				t.Fatalf(
					"devserverengine.DeriveRunLifecycleCommandForState(%q) returned error: %v",
					testCase.CurrentRunLifecycleState,
					err,
				)
			}
			if runLifecycleCommandForState != testCase.ExpectedCommand {
				t.Fatalf(
					"devserverengine.DeriveRunLifecycleCommandForState(%q) = %q, want %q",
					testCase.CurrentRunLifecycleState,
					runLifecycleCommandForState,
					testCase.ExpectedCommand,
				)
			}
		})
	}
}

func TestDeriveRunLifecycleCommandForState_UnknownStateReturnsError(
	t *testing.T,
) {
	runLifecycleCommandForState, err := devserverengine.DeriveRunLifecycleCommandForState(
		devserverengine.RunLifecycleState("unknown_state"),
	)
	if err == nil {
		t.Fatalf(
			"devserverengine.DeriveRunLifecycleCommandForState(unknown_state) expected error, got command %q",
			runLifecycleCommandForState,
		)
	}
}

func TestTransitionRunLifecycleState_LogsCycleAndTransitionFields(t *testing.T) {
	var transitionLogBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&transitionLogBuffer, nil))

	nextRunLifecycleState, err := devserverengine.TransitionRunLifecycleState(
		logger,
		devserverengine.RunLifecycleStatePreparingCycle,
		devserverengine.RunLifecycleEventCyclePrepared,
		42,
	)
	if err != nil {
		t.Fatalf("transitionRunLifecycleState returned error: %v", err)
	}
	if nextRunLifecycleState != devserverengine.RunLifecycleStateBuildingCycle {
		t.Fatalf(
			"nextRunLifecycleState=%q, want %q",
			nextRunLifecycleState,
			devserverengine.RunLifecycleStateBuildingCycle,
		)
	}

	transitionLogOutput := transitionLogBuffer.String()
	if !strings.Contains(transitionLogOutput, "cycle_id=42") {
		t.Fatalf(
			"expected transition log to include cycle_id=42, got %q",
			transitionLogOutput,
		)
	}
	if !strings.Contains(transitionLogOutput, "from=preparing_cycle") {
		t.Fatalf(
			"expected transition log to include from=preparing_cycle, got %q",
			transitionLogOutput,
		)
	}
	if !strings.Contains(transitionLogOutput, "event=cycle_prepared") {
		t.Fatalf(
			"expected transition log to include event=cycle_prepared, got %q",
			transitionLogOutput,
		)
	}
	if !strings.Contains(transitionLogOutput, "to=building_cycle") {
		t.Fatalf(
			"expected transition log to include to=building_cycle, got %q",
			transitionLogOutput,
		)
	}
}

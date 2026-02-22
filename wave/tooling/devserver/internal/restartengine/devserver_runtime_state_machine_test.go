package restartengine_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/tooling/devserver/internal/restartengine"
)

func TestDeriveRunLifecycleStateAfterEvent_ValidTransitions(t *testing.T) {
	testCases := []struct {
		Name                     string
		CurrentRunLifecycleState restartengine.RunLifecycleState
		RunLifecycleEvent        restartengine.RunLifecycleEvent
		ExpectedNextState        restartengine.RunLifecycleState
	}{
		{
			Name:                     "prepare to build",
			CurrentRunLifecycleState: restartengine.RunLifecycleStatePreparingCycle,
			RunLifecycleEvent:        restartengine.RunLifecycleEventCyclePrepared,
			ExpectedNextState:        restartengine.RunLifecycleStateBuildingCycle,
		},
		{
			Name:                     "build success to runtime start",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateBuildingCycle,
			RunLifecycleEvent:        restartengine.RunLifecycleEventBuildSucceeded,
			ExpectedNextState:        restartengine.RunLifecycleStateStartingRuntime,
		},
		{
			Name:                     "build failure to retry wait",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateBuildingCycle,
			RunLifecycleEvent:        restartengine.RunLifecycleEventBuildFailed,
			ExpectedNextState:        restartengine.RunLifecycleStateAwaitingBuildRetry,
		},
		{
			Name:                     "retry restart to cleanup",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateAwaitingBuildRetry,
			RunLifecycleEvent:        restartengine.RunLifecycleEventBuildRetryRestartReceived,
			ExpectedNextState:        restartengine.RunLifecycleStateCleaningUpForNextCycle,
		},
		{
			Name:                     "runtime start to restart wait",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateStartingRuntime,
			RunLifecycleEvent:        restartengine.RunLifecycleEventRuntimeStarted,
			ExpectedNextState:        restartengine.RunLifecycleStateAwaitingRestart,
		},
		{
			Name:                     "restart request to cleanup",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateAwaitingRestart,
			RunLifecycleEvent:        restartengine.RunLifecycleEventRestartRequestReceived,
			ExpectedNextState:        restartengine.RunLifecycleStateCleaningUpForNextCycle,
		},
		{
			Name:                     "cleanup to prepare",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateCleaningUpForNextCycle,
			RunLifecycleEvent:        restartengine.RunLifecycleEventCleanupCompleted,
			ExpectedNextState:        restartengine.RunLifecycleStatePreparingCycle,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			nextRunLifecycleState, deriveError := restartengine.DeriveRunLifecycleStateAfterEvent(
				testCase.CurrentRunLifecycleState,
				testCase.RunLifecycleEvent,
			)
			if deriveError != nil {
				t.Fatalf(
					"DeriveRunLifecycleStateAfterEvent(%v, %v) returned error: %v",
					testCase.CurrentRunLifecycleState,
					testCase.RunLifecycleEvent,
					deriveError,
				)
			}
			if nextRunLifecycleState != testCase.ExpectedNextState {
				t.Fatalf(
					"DeriveRunLifecycleStateAfterEvent(%v, %v) = %v, want %v",
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
		CurrentRunLifecycleState restartengine.RunLifecycleState
		RunLifecycleEvent        restartengine.RunLifecycleEvent
	}{
		{
			Name:                     "prepare rejects build failed event",
			CurrentRunLifecycleState: restartengine.RunLifecycleStatePreparingCycle,
			RunLifecycleEvent:        restartengine.RunLifecycleEventBuildFailed,
		},
		{
			Name:                     "build rejects cleanup event",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateBuildingCycle,
			RunLifecycleEvent:        restartengine.RunLifecycleEventCleanupCompleted,
		},
		{
			Name:                     "retry wait rejects runtime started event",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateAwaitingBuildRetry,
			RunLifecycleEvent:        restartengine.RunLifecycleEventRuntimeStarted,
		},
		{
			Name:                     "runtime start rejects restart received event",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateStartingRuntime,
			RunLifecycleEvent:        restartengine.RunLifecycleEventRestartRequestReceived,
		},
		{
			Name:                     "restart wait rejects cycle prepared event",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateAwaitingRestart,
			RunLifecycleEvent:        restartengine.RunLifecycleEventCyclePrepared,
		},
		{
			Name:                     "cleanup rejects build success event",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateCleaningUpForNextCycle,
			RunLifecycleEvent:        restartengine.RunLifecycleEventBuildSucceeded,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			nextRunLifecycleState, deriveError := restartengine.DeriveRunLifecycleStateAfterEvent(
				testCase.CurrentRunLifecycleState,
				testCase.RunLifecycleEvent,
			)
			if deriveError == nil {
				t.Fatalf(
					"DeriveRunLifecycleStateAfterEvent(%v, %v) expected error, got next state %v",
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
		CurrentRunLifecycleState restartengine.RunLifecycleState
		ExpectedCommand          restartengine.RunLifecycleCommand
	}{
		{
			Name:                     "prepare state maps to prepare command",
			CurrentRunLifecycleState: restartengine.RunLifecycleStatePreparingCycle,
			ExpectedCommand:          restartengine.RunLifecycleCommandPrepareCycle,
		},
		{
			Name:                     "build state maps to build command",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateBuildingCycle,
			ExpectedCommand:          restartengine.RunLifecycleCommandBuildCycle,
		},
		{
			Name:                     "retry state maps to retry command",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateAwaitingBuildRetry,
			ExpectedCommand:          restartengine.RunLifecycleCommandAwaitBuildRetry,
		},
		{
			Name:                     "runtime state maps to start-runtime command",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateStartingRuntime,
			ExpectedCommand:          restartengine.RunLifecycleCommandStartRuntime,
		},
		{
			Name:                     "await-restart state maps to await-restart command",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateAwaitingRestart,
			ExpectedCommand:          restartengine.RunLifecycleCommandAwaitRestartRequest,
		},
		{
			Name:                     "cleanup state maps to cleanup command",
			CurrentRunLifecycleState: restartengine.RunLifecycleStateCleaningUpForNextCycle,
			ExpectedCommand:          restartengine.RunLifecycleCommandCleanupForNextCycle,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			commandForState, deriveError := restartengine.DeriveRunLifecycleCommandForState(
				testCase.CurrentRunLifecycleState,
			)
			if deriveError != nil {
				t.Fatalf(
					"DeriveRunLifecycleCommandForState(%v) returned error: %v",
					testCase.CurrentRunLifecycleState,
					deriveError,
				)
			}
			if commandForState != testCase.ExpectedCommand {
				t.Fatalf(
					"DeriveRunLifecycleCommandForState(%v) = %v, want %v",
					testCase.CurrentRunLifecycleState,
					commandForState,
					testCase.ExpectedCommand,
				)
			}
		})
	}
}

func TestDeriveRunLifecycleCommandForState_UnknownStateReturnsError(
	t *testing.T,
) {
	commandForState, deriveError := restartengine.DeriveRunLifecycleCommandForState(
		restartengine.RunLifecycleState(999),
	)
	if deriveError == nil {
		t.Fatalf(
			"DeriveRunLifecycleCommandForState(unknown_state) expected error, got command %v",
			commandForState,
		)
	}
}

func TestTransitionRunLifecycleState_LogsCycleAndTransitionFields(t *testing.T) {
	var transitionLogBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&transitionLogBuffer, nil))

	nextRunLifecycleState, transitionError := restartengine.TransitionRunLifecycleState(
		logger,
		restartengine.RunLifecycleStatePreparingCycle,
		restartengine.RunLifecycleEventCyclePrepared,
		42,
	)
	if transitionError != nil {
		t.Fatalf("TransitionRunLifecycleState returned error: %v", transitionError)
	}
	if nextRunLifecycleState != restartengine.RunLifecycleStateBuildingCycle {
		t.Fatalf(
			"nextRunLifecycleState=%v, want %v",
			nextRunLifecycleState,
			restartengine.RunLifecycleStateBuildingCycle,
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

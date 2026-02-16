package tooling

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestDeriveRunLifecycleStateAfterEvent_ValidTransitions(t *testing.T) {
	testCases := []struct {
		name                     string
		currentRunLifecycleState runLifecycleState
		runLifecycleEvent        runLifecycleEvent
		expectedNextState        runLifecycleState
	}{
		{
			name:                     "prepare to build",
			currentRunLifecycleState: runLifecycleStatePreparingCycle,
			runLifecycleEvent:        runLifecycleEventCyclePrepared,
			expectedNextState:        runLifecycleStateBuildingCycle,
		},
		{
			name:                     "build success to runtime start",
			currentRunLifecycleState: runLifecycleStateBuildingCycle,
			runLifecycleEvent:        runLifecycleEventBuildSucceeded,
			expectedNextState:        runLifecycleStateStartingRuntime,
		},
		{
			name:                     "build failure to retry wait",
			currentRunLifecycleState: runLifecycleStateBuildingCycle,
			runLifecycleEvent:        runLifecycleEventBuildFailed,
			expectedNextState:        runLifecycleStateAwaitingBuildRetry,
		},
		{
			name:                     "retry restart to prepare",
			currentRunLifecycleState: runLifecycleStateAwaitingBuildRetry,
			runLifecycleEvent:        runLifecycleEventBuildRetryRestartReceived,
			expectedNextState:        runLifecycleStatePreparingCycle,
		},
		{
			name:                     "runtime start to restart wait",
			currentRunLifecycleState: runLifecycleStateStartingRuntime,
			runLifecycleEvent:        runLifecycleEventRuntimeStarted,
			expectedNextState:        runLifecycleStateAwaitingRestart,
		},
		{
			name:                     "restart request to cleanup",
			currentRunLifecycleState: runLifecycleStateAwaitingRestart,
			runLifecycleEvent:        runLifecycleEventRestartRequestReceived,
			expectedNextState:        runLifecycleStateCleaningUpForNextCycle,
		},
		{
			name:                     "cleanup to prepare",
			currentRunLifecycleState: runLifecycleStateCleaningUpForNextCycle,
			runLifecycleEvent:        runLifecycleEventCleanupCompleted,
			expectedNextState:        runLifecycleStatePreparingCycle,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			nextRunLifecycleState, err := deriveRunLifecycleStateAfterEvent(
				testCase.currentRunLifecycleState,
				testCase.runLifecycleEvent,
			)
			if err != nil {
				t.Fatalf(
					"deriveRunLifecycleStateAfterEvent(%q, %q) returned error: %v",
					testCase.currentRunLifecycleState,
					testCase.runLifecycleEvent,
					err,
				)
			}
			if nextRunLifecycleState != testCase.expectedNextState {
				t.Fatalf(
					"deriveRunLifecycleStateAfterEvent(%q, %q) = %q, want %q",
					testCase.currentRunLifecycleState,
					testCase.runLifecycleEvent,
					nextRunLifecycleState,
					testCase.expectedNextState,
				)
			}
		})
	}
}

func TestDeriveRunLifecycleStateAfterEvent_InvalidTransitionsReturnError(t *testing.T) {
	testCases := []struct {
		name                     string
		currentRunLifecycleState runLifecycleState
		runLifecycleEvent        runLifecycleEvent
	}{
		{
			name:                     "prepare rejects build failed event",
			currentRunLifecycleState: runLifecycleStatePreparingCycle,
			runLifecycleEvent:        runLifecycleEventBuildFailed,
		},
		{
			name:                     "build rejects cleanup event",
			currentRunLifecycleState: runLifecycleStateBuildingCycle,
			runLifecycleEvent:        runLifecycleEventCleanupCompleted,
		},
		{
			name:                     "retry wait rejects runtime started event",
			currentRunLifecycleState: runLifecycleStateAwaitingBuildRetry,
			runLifecycleEvent:        runLifecycleEventRuntimeStarted,
		},
		{
			name:                     "runtime start rejects restart received event",
			currentRunLifecycleState: runLifecycleStateStartingRuntime,
			runLifecycleEvent:        runLifecycleEventRestartRequestReceived,
		},
		{
			name:                     "restart wait rejects cycle prepared event",
			currentRunLifecycleState: runLifecycleStateAwaitingRestart,
			runLifecycleEvent:        runLifecycleEventCyclePrepared,
		},
		{
			name:                     "cleanup rejects build success event",
			currentRunLifecycleState: runLifecycleStateCleaningUpForNextCycle,
			runLifecycleEvent:        runLifecycleEventBuildSucceeded,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			nextRunLifecycleState, err := deriveRunLifecycleStateAfterEvent(
				testCase.currentRunLifecycleState,
				testCase.runLifecycleEvent,
			)
			if err == nil {
				t.Fatalf(
					"deriveRunLifecycleStateAfterEvent(%q, %q) expected error, got next state %q",
					testCase.currentRunLifecycleState,
					testCase.runLifecycleEvent,
					nextRunLifecycleState,
				)
			}
		})
	}
}

func TestDeriveRunLifecycleCommandForState(t *testing.T) {
	testCases := []struct {
		name                     string
		currentRunLifecycleState runLifecycleState
		expectedCommand          runLifecycleCommand
	}{
		{
			name:                     "prepare state maps to prepare command",
			currentRunLifecycleState: runLifecycleStatePreparingCycle,
			expectedCommand:          runLifecycleCommandPrepareCycle,
		},
		{
			name:                     "build state maps to build command",
			currentRunLifecycleState: runLifecycleStateBuildingCycle,
			expectedCommand:          runLifecycleCommandBuildCycle,
		},
		{
			name:                     "retry state maps to retry command",
			currentRunLifecycleState: runLifecycleStateAwaitingBuildRetry,
			expectedCommand:          runLifecycleCommandAwaitBuildRetry,
		},
		{
			name:                     "runtime state maps to start-runtime command",
			currentRunLifecycleState: runLifecycleStateStartingRuntime,
			expectedCommand:          runLifecycleCommandStartRuntime,
		},
		{
			name:                     "await-restart state maps to await-restart command",
			currentRunLifecycleState: runLifecycleStateAwaitingRestart,
			expectedCommand:          runLifecycleCommandAwaitRestartRequest,
		},
		{
			name:                     "cleanup state maps to cleanup command",
			currentRunLifecycleState: runLifecycleStateCleaningUpForNextCycle,
			expectedCommand:          runLifecycleCommandCleanupForNextCycle,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			runLifecycleCommandForState, err := deriveRunLifecycleCommandForState(
				testCase.currentRunLifecycleState,
			)
			if err != nil {
				t.Fatalf(
					"deriveRunLifecycleCommandForState(%q) returned error: %v",
					testCase.currentRunLifecycleState,
					err,
				)
			}
			if runLifecycleCommandForState != testCase.expectedCommand {
				t.Fatalf(
					"deriveRunLifecycleCommandForState(%q) = %q, want %q",
					testCase.currentRunLifecycleState,
					runLifecycleCommandForState,
					testCase.expectedCommand,
				)
			}
		})
	}
}

func TestDeriveRunLifecycleCommandForState_UnknownStateReturnsError(t *testing.T) {
	runLifecycleCommandForState, err := deriveRunLifecycleCommandForState(
		runLifecycleState("unknown_state"),
	)
	if err == nil {
		t.Fatalf(
			"deriveRunLifecycleCommandForState(unknown_state) expected error, got command %q",
			runLifecycleCommandForState,
		)
	}
}

func TestTransitionRunLifecycleState_LogsCycleAndTransitionFields(t *testing.T) {
	var transitionLogBuffer bytes.Buffer
	s := &server{
		log: slog.New(slog.NewTextHandler(&transitionLogBuffer, nil)),
	}

	nextRunLifecycleState, err := s.transitionRunLifecycleState(
		runLifecycleStatePreparingCycle,
		runLifecycleEventCyclePrepared,
		42,
	)
	if err != nil {
		t.Fatalf("transitionRunLifecycleState returned error: %v", err)
	}
	if nextRunLifecycleState != runLifecycleStateBuildingCycle {
		t.Fatalf("nextRunLifecycleState=%q, want %q", nextRunLifecycleState, runLifecycleStateBuildingCycle)
	}

	transitionLogOutput := transitionLogBuffer.String()
	if !strings.Contains(transitionLogOutput, "cycle_id=42") {
		t.Fatalf("expected transition log to include cycle_id=42, got %q", transitionLogOutput)
	}
	if !strings.Contains(transitionLogOutput, "from=preparing_cycle") {
		t.Fatalf("expected transition log to include from=preparing_cycle, got %q", transitionLogOutput)
	}
	if !strings.Contains(transitionLogOutput, "event=cycle_prepared") {
		t.Fatalf("expected transition log to include event=cycle_prepared, got %q", transitionLogOutput)
	}
	if !strings.Contains(transitionLogOutput, "to=building_cycle") {
		t.Fatalf("expected transition log to include to=building_cycle, got %q", transitionLogOutput)
	}
}

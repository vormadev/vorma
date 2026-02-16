package vormabuild

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestBuildLifecycleStateMachine_FullBuildHappyPath(t *testing.T) {
	observedTransitions := make([]buildLifecycleTransitionRecord, 0, 7)
	buildLifecycleMachine, err := newBuildLifecycleStateMachine(
		buildLifecycleWorkflowFullBuild,
		nil,
		func(transition buildLifecycleTransitionRecord) {
			observedTransitions = append(observedTransitions, transition)
		},
	)
	if err != nil {
		t.Fatalf("newBuildLifecycleStateMachine returned error: %v", err)
	}

	transitionSequence := []buildLifecyclePhase{
		buildLifecyclePhaseStarted,
		buildLifecyclePhaseRuntimeStateInitialized,
		buildLifecyclePhaseRoutesSynchronized,
		buildLifecyclePhasePublicOutputCleaned,
		buildLifecyclePhasePublicFileMapWritten,
		buildLifecyclePhaseRouteArtifactsWritten,
		buildLifecyclePhaseCompleted,
	}
	for _, nextPhase := range transitionSequence {
		if err := buildLifecycleMachine.transitionTo(nextPhase, "unit-test transition"); err != nil {
			t.Fatalf("transitionTo(%q) returned error: %v", nextPhase, err)
		}
	}

	if currentPhase := buildLifecycleMachine.currentPhaseSnapshot(); currentPhase != buildLifecyclePhaseCompleted {
		t.Fatalf("current phase = %q, want %q", currentPhase, buildLifecyclePhaseCompleted)
	}

	observedDestinationPhases := make([]buildLifecyclePhase, 0, len(observedTransitions))
	for _, transition := range observedTransitions {
		observedDestinationPhases = append(observedDestinationPhases, transition.To)
	}
	if !slices.Equal(observedDestinationPhases, transitionSequence) {
		t.Fatalf("observed destination phases = %#v, want %#v", observedDestinationPhases, transitionSequence)
	}
}

func TestBuildLifecycleStateMachine_FastRouteRebuildHappyPath(t *testing.T) {
	buildLifecycleMachine, err := newBuildLifecycleStateMachine(
		buildLifecycleWorkflowFastRouteRebuild,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("newBuildLifecycleStateMachine returned error: %v", err)
	}

	transitionSequence := []buildLifecyclePhase{
		buildLifecyclePhaseStarted,
		buildLifecyclePhaseRoutesSynchronized,
		buildLifecyclePhaseRouteArtifactsWritten,
		buildLifecyclePhaseCompleted,
	}
	for _, nextPhase := range transitionSequence {
		if err := buildLifecycleMachine.transitionTo(nextPhase, "unit-test transition"); err != nil {
			t.Fatalf("transitionTo(%q) returned error: %v", nextPhase, err)
		}
	}
}

func TestBuildLifecycleStateMachine_RejectsInvalidTransition(t *testing.T) {
	buildLifecycleMachine, err := newBuildLifecycleStateMachine(
		buildLifecycleWorkflowFullBuild,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("newBuildLifecycleStateMachine returned error: %v", err)
	}

	if err := buildLifecycleMachine.transitionTo(buildLifecyclePhaseStarted, "start"); err != nil {
		t.Fatalf("transitionTo(started) returned error: %v", err)
	}

	err = buildLifecycleMachine.transitionTo(buildLifecyclePhaseRoutesSynchronized, "skip phase")
	if err == nil {
		t.Fatal("expected invalid transition error when skipping runtime-state-initialized")
	}
	if !strings.Contains(err.Error(), "invalid workflow") {
		t.Fatalf("error = %q, expected invalid transition message", err)
	}
}

func TestBuildLifecycleStateMachine_AllowsFailedTransitionFromIntermediatePhase(t *testing.T) {
	buildLifecycleMachine, err := newBuildLifecycleStateMachine(
		buildLifecycleWorkflowFullBuild,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("newBuildLifecycleStateMachine returned error: %v", err)
	}

	if err := buildLifecycleMachine.transitionTo(buildLifecyclePhaseStarted, "start"); err != nil {
		t.Fatalf("transitionTo(started) returned error: %v", err)
	}
	if err := buildLifecycleMachine.transitionTo(
		buildLifecyclePhaseRuntimeStateInitialized,
		"runtime initialized",
	); err != nil {
		t.Fatalf("transitionTo(runtime-state-initialized) returned error: %v", err)
	}

	buildErr := errors.New("build step failed")
	if err := buildLifecycleMachine.transitionToFailed("mark failure", buildErr); err != nil {
		t.Fatalf("transitionToFailed returned error: %v", err)
	}

	if currentPhase := buildLifecycleMachine.currentPhaseSnapshot(); currentPhase != buildLifecyclePhaseFailed {
		t.Fatalf("current phase = %q, want %q", currentPhase, buildLifecyclePhaseFailed)
	}
}

func TestBuildLifecycleStateMachine_RejectsFailedTransitionWithoutError(t *testing.T) {
	buildLifecycleMachine, err := newBuildLifecycleStateMachine(
		buildLifecycleWorkflowFastRouteRebuild,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("newBuildLifecycleStateMachine returned error: %v", err)
	}

	if err := buildLifecycleMachine.transitionTo(buildLifecyclePhaseStarted, "start"); err != nil {
		t.Fatalf("transitionTo(started) returned error: %v", err)
	}

	err = buildLifecycleMachine.transitionToFailed("mark failure", nil)
	if err == nil {
		t.Fatal("expected transitionToFailed to require a non-nil error")
	}
	if !strings.Contains(err.Error(), "build error is required") {
		t.Fatalf("error = %q, expected missing-build-error message", err)
	}
}

func TestBuildLifecycleStateMachine_UnknownWorkflowReturnsError(t *testing.T) {
	_, err := newBuildLifecycleStateMachine("unknown", nil, nil)
	if err == nil {
		t.Fatal("expected unknown workflow to return error")
	}
	if !strings.Contains(err.Error(), "unknown build lifecycle workflow") {
		t.Fatalf("error = %q, expected unknown-workflow message", err)
	}
}

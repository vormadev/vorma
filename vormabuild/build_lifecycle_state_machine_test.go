package vormabuild

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
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

	if currentPhase := buildLifecycleMachine.currentPhaseValue(); currentPhase != buildLifecyclePhaseCompleted {
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

func TestBuildLifecycleStateMachine_AttemptMetadataPropagatesToTransitions(t *testing.T) {
	observedTransitions := make([]buildLifecycleTransitionRecord, 0, 4)
	buildLifecycleMachine, err := newBuildLifecycleStateMachineWithOptions(
		buildLifecycleWorkflowFastRouteRebuild,
		nil,
		buildLifecycleStateMachineOptions{
			transitionObserver: func(transition buildLifecycleTransitionRecord) {
				observedTransitions = append(observedTransitions, transition)
			},
			attemptInputs: []buildLifecycleAttemptInput{
				{
					Key:   " requested_mode ",
					Value: " dev ",
				},
				{
					Key:   "",
					Value: "discarded",
				},
				{
					Key:   "is_dev_mode",
					Value: "true",
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("newBuildLifecycleStateMachineWithOptions returned error: %v", err)
	}

	if attemptID := buildLifecycleMachine.attemptIDValue(); attemptID == "" {
		t.Fatal("expected non-empty lifecycle attempt ID")
	}

	if err := buildLifecycleMachine.transitionTo(buildLifecyclePhaseStarted, "start"); err != nil {
		t.Fatalf("transitionTo(started) returned error: %v", err)
	}
	if err := buildLifecycleMachine.transitionTo(buildLifecyclePhaseRoutesSynchronized, "sync routes"); err != nil {
		t.Fatalf("transitionTo(routes-synchronized) returned error: %v", err)
	}
	if err := buildLifecycleMachine.transitionTo(buildLifecyclePhaseRouteArtifactsWritten, "write artifacts"); err != nil {
		t.Fatalf("transitionTo(route-artifacts-written) returned error: %v", err)
	}
	if err := buildLifecycleMachine.transitionTo(buildLifecyclePhaseCompleted, "complete"); err != nil {
		t.Fatalf("transitionTo(completed) returned error: %v", err)
	}

	attemptInputs := buildLifecycleMachine.attemptInputValues()
	if len(attemptInputs) != 2 {
		t.Fatalf("attempt input count = %d, want 2 (%#v)", len(attemptInputs), attemptInputs)
	}
	if attemptInputs[0].Key != "is_dev_mode" || attemptInputs[0].Value != "true" {
		t.Fatalf("attemptInputs[0] = %#v, want is_dev_mode=true", attemptInputs[0])
	}
	if attemptInputs[1].Key != "requested_mode" || attemptInputs[1].Value != "dev" {
		t.Fatalf("attemptInputs[1] = %#v, want requested_mode=dev", attemptInputs[1])
	}

	if len(observedTransitions) != 4 {
		t.Fatalf("observed transitions = %d, want 4", len(observedTransitions))
	}
	for _, observedTransition := range observedTransitions {
		if observedTransition.AttemptID != buildLifecycleMachine.attemptIDValue() {
			t.Fatalf(
				"transition attempt ID = %q, want %q",
				observedTransition.AttemptID,
				buildLifecycleMachine.attemptIDValue(),
			)
		}
		if observedTransition.AtUTC == "" {
			t.Fatalf("expected non-empty transition timestamp for %+v", observedTransition)
		}
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

	if currentPhase := buildLifecycleMachine.currentPhaseValue(); currentPhase != buildLifecyclePhaseFailed {
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

func TestBuildLifecycleStateMachine_RecordRollback(t *testing.T) {
	buildLifecycleMachine, err := newBuildLifecycleStateMachine(
		buildLifecycleWorkflowFastRouteRebuild,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("newBuildLifecycleStateMachine returned error: %v", err)
	}

	if err := buildLifecycleMachine.recordRollback(
		buildLifecycleRollbackDecisionRequired,
		buildLifecycleRollbackOutcomeSucceeded,
		"rollback completed after failure",
		nil,
	); err != nil {
		t.Fatalf("recordRollback returned error: %v", err)
	}

	rollbackHistory := buildLifecycleMachine.rollbackHistoryEntries()
	if len(rollbackHistory) != 1 {
		t.Fatalf("rollback history count = %d, want 1", len(rollbackHistory))
	}
	if rollbackHistory[0].Decision != buildLifecycleRollbackDecisionRequired {
		t.Fatalf(
			"rollback decision = %q, want %q",
			rollbackHistory[0].Decision,
			buildLifecycleRollbackDecisionRequired,
		)
	}
	if rollbackHistory[0].Outcome != buildLifecycleRollbackOutcomeSucceeded {
		t.Fatalf(
			"rollback outcome = %q, want %q",
			rollbackHistory[0].Outcome,
			buildLifecycleRollbackOutcomeSucceeded,
		)
	}
	if rollbackHistory[0].Reason != "rollback completed after failure" {
		t.Fatalf(
			"rollback reason = %q, want %q",
			rollbackHistory[0].Reason,
			"rollback completed after failure",
		)
	}
	if rollbackHistory[0].AttemptID == "" {
		t.Fatal("expected rollback history to include attempt ID")
	}
	if rollbackHistory[0].AtUTC == "" {
		t.Fatal("expected rollback history to include timestamp")
	}

	err = buildLifecycleMachine.recordRollback(
		buildLifecycleRollbackDecisionRequired,
		buildLifecycleRollbackOutcomeFailed,
		"",
		errors.New("rollback failed"),
	)
	if err == nil {
		t.Fatal("expected recordRollback to require a reason")
	}
	if !strings.Contains(err.Error(), "rollback reason is required") {
		t.Fatalf("error = %q, expected missing rollback reason message", err)
	}
}

func TestBuildLifecycleStateMachine_UsesProvidedDependencyOverrides(t *testing.T) {
	fixedNow := time.Date(2026, time.February, 16, 10, 11, 12, 130000000, time.UTC)
	buildLifecycleMachine, err := newBuildLifecycleStateMachineWithOptions(
		buildLifecycleWorkflowFastRouteRebuild,
		nil,
		buildLifecycleStateMachineOptions{
			dependencies: buildLifecycleStateMachineDependencies{
				nowUTC: func() time.Time {
					return fixedNow
				},
				nextAttemptID: func(buildLifecycleWorkflow) string {
					return "fixed-attempt-id"
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("newBuildLifecycleStateMachineWithOptions returned error: %v", err)
	}

	if buildLifecycleMachine.attemptIDValue() != "fixed-attempt-id" {
		t.Fatalf(
			"attemptID = %q, want %q",
			buildLifecycleMachine.attemptIDValue(),
			"fixed-attempt-id",
		)
	}

	if err := buildLifecycleMachine.transitionTo(buildLifecyclePhaseStarted, "start"); err != nil {
		t.Fatalf("transitionTo(started) returned error: %v", err)
	}
	if err := buildLifecycleMachine.recordRollback(
		buildLifecycleRollbackDecisionNotRequired,
		buildLifecycleRollbackOutcomeNotRequired,
		"no rollback needed",
		nil,
	); err != nil {
		t.Fatalf("recordRollback returned error: %v", err)
	}

	transitionHistory := buildLifecycleMachine.transitionHistoryEntries()
	if len(transitionHistory) != 1 {
		t.Fatalf("transition history count = %d, want 1", len(transitionHistory))
	}
	if transitionHistory[0].AtUTC != fixedNow.Format(time.RFC3339Nano) {
		t.Fatalf(
			"transition timestamp = %q, want %q",
			transitionHistory[0].AtUTC,
			fixedNow.Format(time.RFC3339Nano),
		)
	}

	rollbackHistory := buildLifecycleMachine.rollbackHistoryEntries()
	if len(rollbackHistory) != 1 {
		t.Fatalf("rollback history count = %d, want 1", len(rollbackHistory))
	}
	if rollbackHistory[0].AtUTC != fixedNow.Format(time.RFC3339Nano) {
		t.Fatalf(
			"rollback timestamp = %q, want %q",
			rollbackHistory[0].AtUTC,
			fixedNow.Format(time.RFC3339Nano),
		)
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

package phaselane

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type phase_step_recorder struct {
	mu    sync.Mutex
	steps []string
}

func (recorder *phase_step_recorder) record(
	step string,
) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.steps = append(recorder.steps, step)
}

func (recorder *phase_step_recorder) snapshot() []string {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	snapshot := make([]string, len(recorder.steps))
	copy(snapshot, recorder.steps)
	return snapshot
}

func TestRun_loop_back_and_stop(t *testing.T) {
	recorder := &phase_step_recorder{}

	plan := Plan{
		Phases: []Phase{
			{
				ID: "phase_1",
				Cells: []PhaseCell{
					{
						LaneID:   "main",
						WaitMode: CellWaitModeAwaitCompletion,
						Run:      phase_recording_cell_runner("phase_1", recorder),
					},
				},
				TransitionPolicyID: "policy_phase_1",
			},
			{
				ID: "phase_2",
				Cells: []PhaseCell{
					{
						LaneID:   "main",
						WaitMode: CellWaitModeAwaitCompletion,
						Run:      phase_recording_cell_runner("phase_2", recorder),
					},
				},
				TransitionPolicyID: "policy_phase_2",
			},
			{
				ID: "phase_3",
				Cells: []PhaseCell{
					{
						LaneID:   "main",
						WaitMode: CellWaitModeAwaitCompletion,
						Run:      phase_recording_cell_runner("phase_3", recorder),
					},
				},
				TransitionPolicyID: "policy_phase_3",
			},
		},
		TransitionPlanner: TransitionPlanner{
			Policies: map[string]TransitionPolicy{
				"policy_phase_1": {
					Rules: []TransitionRule{
						{
							RuleID: "always-advance",
							Matches: func(
								input TransitionPlanInput,
							) bool {
								return input.Plan != nil
							},
							Decision: TransitionDecision{
								Kind: DecisionKindAdvance,
							},
						},
					},
				},
				"policy_phase_2": {
					Rules: []TransitionRule{
						{
							RuleID: "first-pass-loop-back",
							Matches: func(
								input TransitionPlanInput,
							) bool {
								return input.CurrentStepIndex == 1
							},
							Decision: TransitionDecision{
								Kind:          DecisionKindLoopBack,
								TargetPhaseID: "phase_1",
							},
						},
						{
							RuleID: "otherwise-advance",
							Matches: func(
								input TransitionPlanInput,
							) bool {
								return input.CurrentStepIndex >= 0
							},
							Decision: TransitionDecision{
								Kind: DecisionKindAdvance,
							},
						},
					},
				},
				"policy_phase_3": {
					Rules: []TransitionRule{
						{
							RuleID: "stop",
							Matches: func(
								input TransitionPlanInput,
							) bool {
								return input.CurrentStepIndex >= 0
							},
							Decision: TransitionDecision{
								Kind: DecisionKindStop,
							},
						},
					},
				},
			},
		},
	}

	result, err := Run(
		ExecutionInput{
			Context:  context.Background(),
			Plan:     plan,
			MaxSteps: 10,
		},
	)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	got_phase_order := make(
		[]PhaseID,
		0,
		len(result.Steps),
	)
	for _, step := range result.Steps {
		got_phase_order = append(
			got_phase_order,
			step.PhaseRunSummary.PhaseID,
		)
	}

	want_phase_order := []PhaseID{
		"phase_1",
		"phase_2",
		"phase_1",
		"phase_2",
		"phase_3",
	}
	if len(got_phase_order) != len(want_phase_order) {
		t.Fatalf(
			"phase count mismatch: got=%d want=%d",
			len(got_phase_order),
			len(want_phase_order),
		)
	}
	for index := range want_phase_order {
		if got_phase_order[index] != want_phase_order[index] {
			t.Fatalf(
				"phase sequence mismatch at index=%d: got=%q want=%q",
				index,
				got_phase_order[index],
				want_phase_order[index],
			)
		}
	}

	got_recorded_steps := recorder.snapshot()
	want_recorded_steps := []string{
		"phase_1:0",
		"phase_2:1",
		"phase_1:2",
		"phase_2:3",
		"phase_3:4",
	}
	if len(got_recorded_steps) != len(want_recorded_steps) {
		t.Fatalf(
			"recorded step count mismatch: got=%d want=%d",
			len(got_recorded_steps),
			len(want_recorded_steps),
		)
	}
	for index := range want_recorded_steps {
		if got_recorded_steps[index] != want_recorded_steps[index] {
			t.Fatalf(
				"recorded step mismatch at index=%d: got=%q want=%q",
				index,
				got_recorded_steps[index],
				want_recorded_steps[index],
			)
		}
	}

	if result.FinalState.Values == nil {
		t.Fatal("final state values must not be nil")
	}
	got_last_phase, ok := result.FinalState.Values["last_phase"]
	if !ok {
		t.Fatal("expected last_phase value in final state")
	}
	if got_last_phase != "phase_3" {
		t.Fatalf("unexpected last_phase: got=%v want=%q", got_last_phase, "phase_3")
	}
}

func TestRun_detached_lane_returns_ticket(t *testing.T) {
	release := make(chan struct{})
	notified := make(chan struct{}, 1)

	plan := Plan{
		Phases: []Phase{
			{
				ID: "phase_detached",
				Cells: []PhaseCell{
					{
						LaneID:   "awaited",
						WaitMode: CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							input CellRunInput,
						) (CellRunResult, error) {
							_ = run_context
							_ = input
							return CellRunResult{
								LaneValue: "awaited_done",
							}, nil
						},
					},
					{
						LaneID:   "detached",
						WaitMode: CellWaitModeDetached,
						Run: func(
							run_context context.Context,
							input CellRunInput,
						) (CellRunResult, error) {
							_ = input
							select {
							case <-release:
							case <-run_context.Done():
								return CellRunResult{}, run_context.Err()
							}
							select {
							case notified <- struct{}{}:
							default:
							}
							return CellRunResult{}, nil
						},
					},
				},
				TransitionPolicyID: "policy_detached",
			},
		},
		TransitionPlanner: TransitionPlanner{
			Policies: map[string]TransitionPolicy{
				"policy_detached": {
					Rules: []TransitionRule{
						{
							RuleID: "stop",
							Matches: func(
								input TransitionPlanInput,
							) bool {
								return input.CurrentStepIndex >= 0
							},
							Decision: TransitionDecision{
								Kind: DecisionKindStop,
							},
						},
					},
				},
			},
		},
	}

	result, err := Run(
		ExecutionInput{
			Context:  context.Background(),
			Plan:     plan,
			MaxSteps: 2,
		},
	)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(result.DetachedLaneTickets) != 1 {
		t.Fatalf(
			"detached ticket count mismatch: got=%d want=1",
			len(result.DetachedLaneTickets),
		)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected one step, got=%d", len(result.Steps))
	}
	if got_pending := result.Steps[0].PhaseRunSummary.PendingDetachedByLane["detached"]; got_pending != 1 {
		t.Fatalf(
			"pending detached count mismatch: got=%d want=1",
			got_pending,
		)
	}
	ticket := result.DetachedLaneTickets[0]

	select {
	case <-ticket.Done:
		t.Fatal("detached lane should not complete before release")
	default:
	}

	close(release)

	select {
	case err := <-ticket.Done:
		if err != nil {
			t.Fatalf("detached lane returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for detached lane completion")
	}

	select {
	case <-notified:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for detached lane notification")
	}
}

func TestRun_error_when_no_transition_rule_matches(t *testing.T) {
	plan := Plan{
		Phases: []Phase{
			{
				ID:                 "phase_unmatched",
				TransitionPolicyID: "policy_unmatched",
			},
		},
		TransitionPlanner: TransitionPlanner{
			Policies: map[string]TransitionPolicy{
				"policy_unmatched": {
					Rules: []TransitionRule{
						{
							RuleID: "never-match",
							Matches: func(
								input TransitionPlanInput,
							) bool {
								return false
							},
							Decision: TransitionDecision{
								Kind: DecisionKindStop,
							},
						},
					},
				},
			},
		},
	}

	_, err := Run(
		ExecutionInput{
			Context:  context.Background(),
			Plan:     plan,
			MaxSteps: 2,
		},
	)
	if err == nil {
		t.Fatal("expected error when no transition rule matches")
	}
	if !strings.Contains(err.Error(), "no matching rule") {
		t.Fatalf("expected no-matching-rule error, got: %v", err)
	}
}

func TestRun_detached_lane_state_patch_error_surfaces_in_ticket(t *testing.T) {
	plan := Plan{
		Phases: []Phase{
			{
				ID: "phase_detached",
				Cells: []PhaseCell{
					{
						LaneID:   "detached",
						WaitMode: CellWaitModeDetached,
						Run: func(
							run_context context.Context,
							input CellRunInput,
						) (CellRunResult, error) {
							_ = run_context
							_ = input
							return CellRunResult{
								StatePatch: map[string]any{
									"invalid": true,
								},
							}, nil
						},
					},
				},
				TransitionPolicyID: "policy_stop",
			},
		},
		TransitionPlanner: TransitionPlanner{
			Policies: map[string]TransitionPolicy{
				"policy_stop": {
					Rules: []TransitionRule{
						{
							RuleID: "stop",
							Matches: func(
								input TransitionPlanInput,
							) bool {
								return input.CurrentStepIndex >= 0
							},
							Decision: TransitionDecision{
								Kind: DecisionKindStop,
							},
						},
					},
				},
			},
		},
	}

	result, err := Run(
		ExecutionInput{
			Context:  context.Background(),
			Plan:     plan,
			MaxSteps: 2,
		},
	)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(result.DetachedLaneTickets) != 1 {
		t.Fatalf(
			"detached ticket count mismatch: got=%d want=1",
			len(result.DetachedLaneTickets),
		)
	}

	select {
	case detached_error := <-result.DetachedLaneTickets[0].Done:
		if detached_error == nil {
			t.Fatal("expected detached state patch error")
		}
		if !strings.Contains(detached_error.Error(), "cannot return state patch") {
			t.Fatalf("unexpected detached error: %v", detached_error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for detached lane error")
	}
}

func TestRun_row_data_flows_to_next_row(t *testing.T) {
	plan := Plan{
		Phases: []Phase{
			{
				ID: "phase_1",
				Cells: []PhaseCell{
					{
						LaneID:   "main",
						WaitMode: CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							input CellRunInput,
						) (CellRunResult, error) {
							_ = run_context
							_ = input
							return CellRunResult{
								LaneValue: "facts_v1",
								StatePatch: map[string]any{
									"phase_1_done": true,
								},
							}, nil
						},
					},
				},
				TransitionPolicyID: "policy_advance",
			},
			{
				ID: "phase_2",
				Cells: []PhaseCell{
					{
						LaneID:   "main",
						WaitMode: CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							input CellRunInput,
						) (CellRunResult, error) {
							_ = run_context
							if got_prev := input.PreviousRowLaneValues["main"]; got_prev != "facts_v1" {
								return CellRunResult{}, fmt.Errorf(
									"unexpected previous row value: got=%v want=%q",
									got_prev,
									"facts_v1",
								)
							}
							phase_1_done_value, has_phase_1_done_value := input.StateSnapshot["phase_1_done"]
							if !has_phase_1_done_value || phase_1_done_value != true {
								return CellRunResult{}, fmt.Errorf(
									"unexpected state snapshot phase_1_done=%v",
									phase_1_done_value,
								)
							}
							return CellRunResult{
								StatePatch: map[string]any{
									"phase_2_seen_prev": true,
								},
							}, nil
						},
					},
				},
				TransitionPolicyID: "policy_stop",
			},
		},
		TransitionPlanner: TransitionPlanner{
			Policies: map[string]TransitionPolicy{
				"policy_advance": {
					Rules: []TransitionRule{
						{
							RuleID: "advance",
							Matches: func(
								input TransitionPlanInput,
							) bool {
								return input.CurrentStepIndex >= 0
							},
							Decision: TransitionDecision{
								Kind: DecisionKindAdvance,
							},
						},
					},
				},
				"policy_stop": {
					Rules: []TransitionRule{
						{
							RuleID: "stop",
							Matches: func(
								input TransitionPlanInput,
							) bool {
								return input.CurrentStepIndex >= 0
							},
							Decision: TransitionDecision{
								Kind: DecisionKindStop,
							},
						},
					},
				},
			},
		},
	}

	result, err := Run(
		ExecutionInput{
			Context:  context.Background(),
			Plan:     plan,
			MaxSteps: 4,
		},
	)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got=%d", len(result.Steps))
	}
	if first_step_lane_value := result.Steps[0].PhaseRunSummary.AwaitedLaneValues["main"]; first_step_lane_value != "facts_v1" {
		t.Fatalf(
			"unexpected first step lane value: got=%v want=%q",
			first_step_lane_value,
			"facts_v1",
		)
	}
	if final_state_phase_2_seen_prev := result.FinalState.Values["phase_2_seen_prev"]; final_state_phase_2_seen_prev != true {
		t.Fatalf(
			"unexpected final state phase_2_seen_prev: got=%v want=true",
			final_state_phase_2_seen_prev,
		)
	}
}

func phase_recording_cell_runner(
	phase_name string,
	recorder *phase_step_recorder,
) CellRunner {
	return func(
		run_context context.Context,
		input CellRunInput,
	) (CellRunResult, error) {
		_ = run_context
		recorder.record(
			fmt.Sprintf(
				"%s:%d",
				phase_name,
				input.StepIndex,
			),
		)
		return CellRunResult{
			StatePatch: map[string]any{
				"last_phase": phase_name,
			},
		}, nil
	}
}

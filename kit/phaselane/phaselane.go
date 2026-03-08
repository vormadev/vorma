package phaselane

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

/////////////////////////////////////////////////////////////////////
/////// Public APIs
/////////////////////////////////////////////////////////////////////

// PhaseID identifies one phase row in the phase/lane matrix.
type PhaseID string

// LaneID identifies one lane column in the phase/lane matrix.
type LaneID string

// CellWaitMode controls whether lane work gates phase completion.
type CellWaitMode string

const (
	// CellWaitModeAwaitCompletion means lane work must finish before transition
	// planning executes.
	CellWaitModeAwaitCompletion CellWaitMode = "await-completion"
	// CellWaitModeDetached means lane work is launched and not awaited by the
	// critical path.
	CellWaitModeDetached CellWaitMode = "detached"
)

// State carries mutable batch state used by cell callbacks and transition
// policies.
type State struct {
	Values map[string]any
}

// CellRunInput is the full per-cell callback input.
type CellRunInput struct {
	CurrentPhaseID        PhaseID
	CurrentPhaseIndex     int
	StepIndex             int
	StateSnapshot         map[string]any
	PreviousRowLaneValues map[LaneID]any
	PendingDetachedByLane map[LaneID]int
}

// CellRunResult is one cell callback output payload.
type CellRunResult struct {
	LaneValue  any
	StatePatch map[string]any
}

// CellRunner executes one phase/lane cell.
type CellRunner func(
	run_context context.Context,
	input CellRunInput,
) (CellRunResult, error)

// PhaseCell defines one lane cell inside one phase.
type PhaseCell struct {
	LaneID   LaneID
	WaitMode CellWaitMode
	Run      CellRunner
}

// Phase defines one phase row and its transition policy.
type Phase struct {
	ID                 PhaseID
	Cells              []PhaseCell
	TransitionPolicyID string
}

// DecisionKind selects the next control-flow move after a phase finishes.
type DecisionKind string

const (
	// DecisionKindAdvance moves to the next phase in Plan.Phases.
	DecisionKindAdvance DecisionKind = "advance"
	// DecisionKindJump moves to TransitionDecision.TargetPhaseID.
	DecisionKindJump DecisionKind = "jump"
	// DecisionKindLoopBack moves to an earlier TransitionDecision.TargetPhaseID.
	DecisionKindLoopBack DecisionKind = "loop-back"
	// DecisionKindRepeat reruns the current phase.
	DecisionKindRepeat DecisionKind = "repeat"
	// DecisionKindStop ends orchestration.
	DecisionKindStop DecisionKind = "stop"
)

// TransitionDecision is one policy-selected next-step decision.
type TransitionDecision struct {
	Kind          DecisionKind
	TargetPhaseID PhaseID
}

// LaneRunSummary describes one lane launch in one phase step.
type LaneRunSummary struct {
	LaneID                LaneID
	WaitMode              CellWaitMode
	DetachedTicketPresent bool
	ProducedLaneValue     bool
}

// PhaseRunSummary describes one phase step execution.
type PhaseRunSummary struct {
	PhaseID               PhaseID
	LaneRuns              []LaneRunSummary
	AwaitedLaneValues     map[LaneID]any
	PendingDetachedByLane map[LaneID]int
}

// TransitionPlanInput is the input contract for transition rule matching.
type TransitionPlanInput struct {
	PolicyID         string
	Plan             *Plan
	State            *State
	CurrentPhaseID   PhaseID
	CurrentPhaseStep PhaseRunSummary
	CurrentStepIndex int
}

// TransitionRule is one ordered branch in a transition policy.
type TransitionRule struct {
	RuleID   string
	Matches  func(input TransitionPlanInput) bool
	Decision TransitionDecision
}

// TransitionPolicy is one reusable transition rule set.
type TransitionPolicy struct {
	Rules []TransitionRule
}

// TransitionPlanner resolves next-step transitions using named policies.
type TransitionPlanner struct {
	Policies map[string]TransitionPolicy
}

// Plan is the dynamic phase/lane matrix and transition planning bundle.
type Plan struct {
	Phases            []Phase
	TransitionPlanner TransitionPlanner
}

// DetachedLaneTicket is one detached lane execution handle.
type DetachedLaneTicket struct {
	LaneID LaneID
	Done   <-chan error
}

// ExecutionInput carries everything required for one orchestration run.
type ExecutionInput struct {
	Context      context.Context
	Plan         Plan
	InitialState State
	StartPhaseID PhaseID
	MaxSteps     int
}

// ExecutionStep captures one executed phase and its transition decision.
type ExecutionStep struct {
	PhaseRunSummary    PhaseRunSummary
	TransitionDecision TransitionDecision
}

// ExecutionResult is the full orchestration run output.
type ExecutionResult struct {
	FinalState          State
	Steps               []ExecutionStep
	DetachedLaneTickets []DetachedLaneTicket
}

// Run executes a dynamic phase/lane matrix with first-class transition
// planning.
func Run(
	input ExecutionInput,
) (ExecutionResult, error) {
	if input.Context == nil {
		input.Context = context.Background()
	}

	phase_order, phase_defs_by_id, err := normalize_phase_plan(input.Plan.Phases)
	if err != nil {
		return ExecutionResult{}, err
	}
	if len(phase_order) == 0 {
		return ExecutionResult{}, errors.New("phaselane: orchestrator phases are required")
	}
	if input.MaxSteps < 1 {
		return ExecutionResult{}, errors.New("phaselane: orchestrator max_steps must be >= 1")
	}

	state := input.InitialState
	if state.Values == nil {
		state.Values = map[string]any{}
	}

	current_phase_id := input.StartPhaseID
	if current_phase_id == "" {
		current_phase_id = phase_order[0]
	}
	current_phase_index, err := find_phase_index(phase_order, current_phase_id)
	if err != nil {
		return ExecutionResult{}, err
	}

	run_steps := make([]ExecutionStep, 0, input.MaxSteps)
	all_detached_lane_tickets := make([]DetachedLaneTicket, 0, len(phase_order))
	active_detached_lane_tickets := make([]active_detached_lane_ticket, 0, len(phase_order))
	pending_detached_by_lane := map[LaneID]int{}
	previous_row_lane_values := map[LaneID]any{}

	for step_index := 0; step_index < input.MaxSteps; step_index++ {
		active_detached_lane_tickets = drain_completed_detached_tickets(
			active_detached_lane_tickets,
			pending_detached_by_lane,
		)

		current_phase_def, has_phase_def := phase_defs_by_id[current_phase_id]
		if !has_phase_def {
			return ExecutionResult{}, fmt.Errorf(
				"phaselane: missing phase def for phase %q",
				current_phase_id,
			)
		}

		phase_run_summary, launched_tickets, launched_active_tickets, awaited_lane_values, err := run_phase(
			run_phase_input{
				native_context:           input.Context,
				state:                    &state,
				phase_id:                 current_phase_id,
				phase_index:              current_phase_index,
				step_index:               step_index,
				phase:                    current_phase_def,
				previous_row_lane_values: previous_row_lane_values,
				pending_detached_by_lane: pending_detached_by_lane,
			},
		)
		if err != nil {
			return ExecutionResult{}, err
		}

		all_detached_lane_tickets = append(all_detached_lane_tickets, launched_tickets...)
		active_detached_lane_tickets = append(active_detached_lane_tickets, launched_active_tickets...)
		for _, launched_active_ticket := range launched_active_tickets {
			pending_detached_by_lane[launched_active_ticket.lane_id]++
		}
		phase_run_summary.PendingDetachedByLane = pending_detached_by_lane

		transition_decision, err := input.Plan.TransitionPlanner.plan_next_transition(
			transition_plan_input{
				policy_id:          current_phase_def.TransitionPolicyID,
				plan:               &input.Plan,
				state:              &state,
				current_phase_id:   current_phase_id,
				current_phase_step: phase_run_summary,
				current_step_index: step_index,
			},
		)
		if err != nil {
			return ExecutionResult{}, err
		}
		run_steps = append(
			run_steps,
			ExecutionStep{
				PhaseRunSummary:    phase_run_summary,
				TransitionDecision: transition_decision,
			},
		)

		next_phase_id, next_phase_index, should_stop, err := apply_transition_decision(
			apply_transition_decision_input{
				phase_order:         phase_order,
				current_phase_id:    current_phase_id,
				current_phase_index: current_phase_index,
				decision:            transition_decision,
			},
		)
		if err != nil {
			return ExecutionResult{}, err
		}
		if should_stop {
			return ExecutionResult{
				FinalState:          state,
				Steps:               run_steps,
				DetachedLaneTickets: all_detached_lane_tickets,
			}, nil
		}

		current_phase_id = next_phase_id
		current_phase_index = next_phase_index
		previous_row_lane_values = awaited_lane_values
	}

	return ExecutionResult{}, fmt.Errorf(
		"phaselane: orchestrator exceeded max_steps=%d without stop",
		input.MaxSteps,
	)
}

/////////////////////////////////////////////////////////////////////
/////// Internal APIs
/////////////////////////////////////////////////////////////////////

type run_phase_input struct {
	native_context           context.Context
	state                    *State
	phase_id                 PhaseID
	phase_index              int
	step_index               int
	phase                    Phase
	previous_row_lane_values map[LaneID]any
	pending_detached_by_lane map[LaneID]int
}

type awaited_lane struct {
	lane_id LaneID
	run     CellRunner
	input   CellRunInput
}

type awaited_lane_outcome struct {
	result CellRunResult
	err    error
}

type active_detached_lane_ticket struct {
	lane_id LaneID
	done    <-chan error
}

func run_phase(
	input run_phase_input,
) (
	PhaseRunSummary,
	[]DetachedLaneTicket,
	[]active_detached_lane_ticket,
	map[LaneID]any,
	error,
) {
	lane_runs := make([]LaneRunSummary, 0, len(input.phase.Cells))
	lane_run_index_by_lane_id := make(map[LaneID]int, len(input.phase.Cells))
	awaited_lanes := make([]awaited_lane, 0, len(input.phase.Cells))
	launched_tickets := make([]DetachedLaneTicket, 0, len(input.phase.Cells))
	launched_active_tickets := make([]active_detached_lane_ticket, 0, len(input.phase.Cells))

	for _, cell := range input.phase.Cells {
		if cell.Run == nil {
			return PhaseRunSummary{}, nil, nil, nil, fmt.Errorf(
				"phaselane: phase %q lane %q has nil runner",
				input.phase_id,
				cell.LaneID,
			)
		}

		lane_run_index_by_lane_id[cell.LaneID] = len(lane_runs)
		lane_runs = append(
			lane_runs,
			LaneRunSummary{
				LaneID:   cell.LaneID,
				WaitMode: cell.WaitMode,
			},
		)

		cell_run_input := CellRunInput{
			CurrentPhaseID:        input.phase_id,
			CurrentPhaseIndex:     input.phase_index,
			StepIndex:             input.step_index,
			StateSnapshot:         input.state.Values,
			PreviousRowLaneValues: input.previous_row_lane_values,
			PendingDetachedByLane: input.pending_detached_by_lane,
		}

		switch cell.WaitMode {
		case CellWaitModeAwaitCompletion:
			awaited_lanes = append(
				awaited_lanes,
				awaited_lane{
					lane_id: cell.LaneID,
					run:     cell.Run,
					input:   cell_run_input,
				},
			)
		case CellWaitModeDetached:
			done := make(chan error, 1)
			go func(
				phase_id PhaseID,
				lane_id LaneID,
				run CellRunner,
				cell_run_input CellRunInput,
			) {
				defer close(done)
				result, err := run(input.native_context, cell_run_input)
				if err != nil {
					done <- fmt.Errorf(
						"phaselane: detached lane %q in phase %q callback failed: %w",
						lane_id,
						phase_id,
						err,
					)
					return
				}
				if len(result.StatePatch) > 0 {
					done <- fmt.Errorf(
						"phaselane: detached lane %q in phase %q cannot return state patch",
						lane_id,
						phase_id,
					)
					return
				}
				done <- nil
			}(input.phase_id, cell.LaneID, cell.Run, cell_run_input)

			launched_tickets = append(
				launched_tickets,
				DetachedLaneTicket{
					LaneID: cell.LaneID,
					Done:   done,
				},
			)
			launched_active_tickets = append(
				launched_active_tickets,
				active_detached_lane_ticket{
					lane_id: cell.LaneID,
					done:    done,
				},
			)
			lane_run_index := lane_run_index_by_lane_id[cell.LaneID]
			lane_runs[lane_run_index].DetachedTicketPresent = true
		default:
			return PhaseRunSummary{}, nil, nil, nil, fmt.Errorf(
				"phaselane: unsupported wait_mode %q for phase %q lane %q",
				cell.WaitMode,
				input.phase_id,
				cell.LaneID,
			)
		}
	}

	outcomes := make([]awaited_lane_outcome, len(awaited_lanes))
	if len(awaited_lanes) > 0 {
		awaited_context, cancel_awaited_context := context.WithCancel(input.native_context)
		defer cancel_awaited_context()

		var awaited_group sync.WaitGroup
		for awaited_lane_index := range awaited_lanes {
			current_index := awaited_lane_index
			current_lane := awaited_lanes[current_index]
			awaited_group.Add(1)
			go func() {
				defer awaited_group.Done()
				result, err := current_lane.run(awaited_context, current_lane.input)
				outcomes[current_index] = awaited_lane_outcome{
					result: result,
					err:    err,
				}
				if err != nil {
					cancel_awaited_context()
				}
			}()
		}
		awaited_group.Wait()
	}

	if input.state.Values == nil {
		input.state.Values = map[string]any{}
	}
	awaited_lane_values := map[LaneID]any{}
	for awaited_lane_index, awaited_lane := range awaited_lanes {
		outcome := outcomes[awaited_lane_index]
		if outcome.err != nil {
			return PhaseRunSummary{}, nil, nil, nil, fmt.Errorf(
				"phaselane: phase %q lane %q callback failed: %w",
				input.phase_id,
				awaited_lane.lane_id,
				outcome.err,
			)
		}
		for state_key, state_value := range outcome.result.StatePatch {
			input.state.Values[state_key] = state_value
		}
		awaited_lane_values[awaited_lane.lane_id] = outcome.result.LaneValue
		if outcome.result.LaneValue != nil {
			lane_run_index := lane_run_index_by_lane_id[awaited_lane.lane_id]
			lane_runs[lane_run_index].ProducedLaneValue = true
		}
	}

	return PhaseRunSummary{
			PhaseID:           input.phase_id,
			LaneRuns:          lane_runs,
			AwaitedLaneValues: awaited_lane_values,
		},
		launched_tickets,
		launched_active_tickets,
		awaited_lane_values,
		nil
}

func drain_completed_detached_tickets(
	active_tickets []active_detached_lane_ticket,
	pending_detached_by_lane map[LaneID]int,
) []active_detached_lane_ticket {
	if len(active_tickets) == 0 {
		return active_tickets
	}
	remaining_tickets := make([]active_detached_lane_ticket, 0, len(active_tickets))
	for _, active_ticket := range active_tickets {
		select {
		case <-active_ticket.done:
			if pending_count := pending_detached_by_lane[active_ticket.lane_id]; pending_count > 1 {
				pending_detached_by_lane[active_ticket.lane_id] = pending_count - 1
			} else {
				delete(pending_detached_by_lane, active_ticket.lane_id)
			}
		default:
			remaining_tickets = append(remaining_tickets, active_ticket)
		}
	}
	return remaining_tickets
}

type transition_plan_input struct {
	policy_id          string
	plan               *Plan
	state              *State
	current_phase_id   PhaseID
	current_phase_step PhaseRunSummary
	current_step_index int
}

func (planner TransitionPlanner) plan_next_transition(
	input transition_plan_input,
) (TransitionDecision, error) {
	if input.policy_id == "" {
		return TransitionDecision{}, errors.New(
			"phaselane: transition policy id is required",
		)
	}
	policy, has_policy := planner.Policies[input.policy_id]
	if !has_policy {
		return TransitionDecision{}, fmt.Errorf(
			"phaselane: transition policy %q is not registered",
			input.policy_id,
		)
	}
	for rule_index, rule := range policy.Rules {
		if rule.Matches == nil {
			return TransitionDecision{}, fmt.Errorf(
				"phaselane: transition policy %q rule index %d (%q) has nil matcher",
				input.policy_id,
				rule_index,
				rule.RuleID,
			)
		}
		if !rule.Matches(
			TransitionPlanInput{
				PolicyID:         input.policy_id,
				Plan:             input.plan,
				State:            input.state,
				CurrentPhaseID:   input.current_phase_id,
				CurrentPhaseStep: input.current_phase_step,
				CurrentStepIndex: input.current_step_index,
			},
		) {
			continue
		}
		return rule.Decision, nil
	}
	return TransitionDecision{}, fmt.Errorf(
		"phaselane: transition policy %q had no matching rule for phase %q",
		input.policy_id,
		input.current_phase_id,
	)
}

type apply_transition_decision_input struct {
	phase_order         []PhaseID
	current_phase_id    PhaseID
	current_phase_index int
	decision            TransitionDecision
}

func apply_transition_decision(
	input apply_transition_decision_input,
) (PhaseID, int, bool, error) {
	switch input.decision.Kind {
	case DecisionKindStop:
		return "", -1, true, nil
	case DecisionKindRepeat:
		return input.current_phase_id, input.current_phase_index, false, nil
	case DecisionKindAdvance:
		next_phase_index := input.current_phase_index + 1
		if next_phase_index >= len(input.phase_order) {
			return "", -1, true, nil
		}
		return input.phase_order[next_phase_index], next_phase_index, false, nil
	case DecisionKindJump:
		if input.decision.TargetPhaseID == "" {
			return "", -1, false, errors.New(
				"phaselane: jump transition requires target_phase_id",
			)
		}
		target_phase_index, err := find_phase_index(
			input.phase_order,
			input.decision.TargetPhaseID,
		)
		if err != nil {
			return "", -1, false, err
		}
		return input.phase_order[target_phase_index], target_phase_index, false, nil
	case DecisionKindLoopBack:
		if input.decision.TargetPhaseID == "" {
			return "", -1, false, errors.New(
				"phaselane: loop-back transition requires target_phase_id",
			)
		}
		target_phase_index, err := find_phase_index(
			input.phase_order,
			input.decision.TargetPhaseID,
		)
		if err != nil {
			return "", -1, false, err
		}
		if target_phase_index >= input.current_phase_index {
			return "", -1, false, fmt.Errorf(
				"phaselane: loop-back target %q must be earlier than current phase %q",
				input.decision.TargetPhaseID,
				input.current_phase_id,
			)
		}
		return input.phase_order[target_phase_index], target_phase_index, false, nil
	default:
		return "", -1, false, fmt.Errorf(
			"phaselane: unsupported transition kind %q",
			input.decision.Kind,
		)
	}
}

func find_phase_index(
	phase_order []PhaseID,
	target_phase_id PhaseID,
) (int, error) {
	for phase_index, phase_id := range phase_order {
		if phase_id == target_phase_id {
			return phase_index, nil
		}
	}
	return -1, fmt.Errorf(
		"phaselane: phase %q is not present in phase_order",
		target_phase_id,
	)
}

func normalize_phase_plan(
	phases []Phase,
) ([]PhaseID, map[PhaseID]Phase, error) {
	phase_order := make([]PhaseID, 0, len(phases))
	phase_defs_by_id := make(map[PhaseID]Phase, len(phases))
	for phase_index, phase := range phases {
		if phase.ID == "" {
			return nil, nil, fmt.Errorf(
				"phaselane: phase at index %d has empty id",
				phase_index,
			)
		}
		if _, exists := phase_defs_by_id[phase.ID]; exists {
			return nil, nil, fmt.Errorf(
				"phaselane: duplicate phase id %q",
				phase.ID,
			)
		}
		lanes_by_id := map[LaneID]struct{}{}
		for cell_index, cell := range phase.Cells {
			if cell.LaneID == "" {
				return nil, nil, fmt.Errorf(
					"phaselane: phase %q cell index %d has empty lane id",
					phase.ID,
					cell_index,
				)
			}
			if _, lane_exists := lanes_by_id[cell.LaneID]; lane_exists {
				return nil, nil, fmt.Errorf(
					"phaselane: phase %q has duplicate lane id %q",
					phase.ID,
					cell.LaneID,
				)
			}
			lanes_by_id[cell.LaneID] = struct{}{}
		}
		phase_order = append(phase_order, phase.ID)
		phase_defs_by_id[phase.ID] = phase
	}
	return phase_order, phase_defs_by_id, nil
}

# kit/phaselane

`github.com/vormadev/vorma/kit/phaselane`

Lightweight phase/lane orchestration with callback cells and explicit transition
planning.

Use this package when you want:

- dynamic phase count
- dynamic lane count per phase
- explicit phase barriers
- detached (no-wait) lane work
- centralized transition policies (`advance`, `jump`, `loop-back`, `repeat`,
  `stop`)
- row-to-row awaited lane values

## Import

```go
import "github.com/vormadev/vorma/kit/phaselane"
```

## Core Model

- `Plan.Phases` is the ordered phase list.
- each `PhaseCell` provides one `Run` callback.
- awaited cells finish before transition planning.
- detached cells return completion tickets and do not patch state.
- each row exposes awaited lane outputs to the next row.

## Quick Start

```go
plan := phaselane.Plan{
	Phases: []phaselane.Phase{
		{
			ID: "facts",
			Cells: []phaselane.PhaseCell{
				{
					LaneID:   "main",
					WaitMode: phaselane.CellWaitModeAwaitCompletion,
					Run: func(
						run_context context.Context,
						input phaselane.CellRunInput,
					) (phaselane.CellRunResult, error) {
						_ = run_context
						_ = input
						return phaselane.CellRunResult{
							LaneValue: "facts_ready",
							StatePatch: map[string]any{
								"facts_done": true,
							},
						}, nil
					},
				},
			},
			TransitionPolicyID: "facts_to_done",
		},
		{
			ID: "done",
			Cells: []phaselane.PhaseCell{
				{
					LaneID:   "main",
					WaitMode: phaselane.CellWaitModeAwaitCompletion,
					Run: func(
						run_context context.Context,
						input phaselane.CellRunInput,
					) (phaselane.CellRunResult, error) {
						_ = run_context
						_ = input
						return phaselane.CellRunResult{}, nil
					},
				},
			},
			TransitionPolicyID: "stop",
		},
	},
	TransitionPlanner: phaselane.TransitionPlanner{
		Policies: map[string]phaselane.TransitionPolicy{
			"facts_to_done": {
				Rules: []phaselane.TransitionRule{
					{
						RuleID: "advance",
						Matches: func(input phaselane.TransitionPlanInput) bool {
							return true
						},
						Decision: phaselane.TransitionDecision{
							Kind: phaselane.DecisionKindAdvance,
						},
					},
				},
			},
			"stop": {
				Rules: []phaselane.TransitionRule{
					{
						RuleID: "stop",
						Matches: func(input phaselane.TransitionPlanInput) bool {
							return true
						},
						Decision: phaselane.TransitionDecision{
							Kind: phaselane.DecisionKindStop,
						},
					},
				},
			},
		},
	},
}

result, err := phaselane.Run(phaselane.ExecutionInput{
	Context: context.Background(),
	Plan:    plan,
	MaxSteps: 64,
})
if err != nil {
	return err
}

_ = result
```

## Detached Lanes

Use `CellWaitModeDetached` for no-wait callbacks.

- detached callbacks run asynchronously
- each detached launch returns one `DetachedLaneTicket`
- detached callbacks may not return `StatePatch`
- pending detached lane counts are visible in
  `CellRunInput.PendingDetachedByLane`

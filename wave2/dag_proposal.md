# Wave2 DAG Proposal

## Goal

Define a task-native orchestration model that:

- preserves Wave1 observable behavior
- keeps control-flow explicit at phase boundaries
- pushes all prerequisite/dependency wiring into `kit/tasks`
- keeps transition planning first-class and centralized

## Non-Negotiable Rules

1. Phase orchestration is control-flow, not a second DAG scheduler.
2. Task prerequisites, dedupe, and parallelism are owned only by `kit/tasks`.
3. The phase plan must be dynamic: no baked-in phase count, no baked-in lane
   count.
4. Transition planning is centralized policy, not duplicated `if/else` blocks
   spread across phase code.
5. Concurrent-no-wait hooks are not a dedicated lane domain; they are flighted
   from the hook timing phase as non-awaited work.

## Core Model

Think in a phase/lane matrix:

- Rows: ordered phases.
- Columns: lanes chosen per phase (dynamic).
- Cell: planner picks terminal roots for that phase+lane intersection.

Each cell only selects terminal roots. Each terminal root owns its full
prerequisite graph via `task.Run(...)` / `RunParallel(...)`.

Transition decisions are planned after each phase step from one shared policy
table:

- `advance`
- `repeat`
- `jump`
- `loop_back`
- `stop`

## Default 9-Phase Profile (Initial)

This is the initial profile we should memorialize now. It is not a permanent
hard-coded contract.

1. `phase_1_batch_facts`
2. `phase_2_pre_hook_adjustment`
3. `phase_3_materialization_and_concurrent_hooks`
4. `phase_4_post_hook_adjustment`
5. `phase_5_backend_mutation`
6. `phase_6_backend_convergence`
7. `phase_7_frontend_settling`
8. `phase_8_healing_loopback_gate`
9. `phase_9_batch_complete`

## Phase Intent + Terminal Roots

### Phase 1: `phase_1_batch_facts`

Purpose:

- reduce watcher/config/runtime inputs to semantic batch facts

Terminal root:

- `derive_batch_facts`

### Phase 2: `phase_2_pre_hook_adjustment`

Purpose:

- run pre-hook stage
- reduce refresh actions into updated requested outcomes
- flight concurrent-no-wait hooks at the correct timing as non-awaited work

Terminal root:

- `apply_pre_hook_adjustments`

### Phase 3: `phase_3_materialization_and_concurrent_hooks`

Purpose:

- run additive build/materialization surfaces
- run concurrent hook stage
- await both before proceeding

Terminal roots:

- `materialize_wave_metadata_state`
- `materialize_go_binary_state`
- `materialize_critical_css_state`
- `materialize_normal_css_state`
- `materialize_public_asset_state`
- `materialize_private_asset_state`
- `materialize_frontend_bundle_state`
- `execute_concurrent_hooks`

### Phase 4: `phase_4_post_hook_adjustment`

Purpose:

- run post-hook stage after phase-3 convergence
- reduce post actions into final backend/browser requests

Terminal root:

- `apply_post_hook_adjustments`

### Phase 5: `phase_5_backend_mutation`

Purpose:

- apply backend mutation switch domain

Terminal root:

- `apply_backend_mutation_plan`

Branches owned inside this root:

- `queue_retry_wait_restart`
- `restart_dev_server_cycle`
- `apply_normal_backend_mutations`

### Phase 6: `phase_6_backend_convergence`

Purpose:

- wait for backend readiness gates
- execute framework convergence notifications/reloads
- produce healing request facts when convergence policy requires repair

Terminal root:

- `converge_backend_state`

### Phase 7: `phase_7_frontend_settling`

Purpose:

- execute one terminal browser action

Terminal root:

- `apply_frontend_terminal_action`

Switch outcomes owned inside this root:

- no action
- css hot reload
- revalidate
- notify vite public-filemap changed
- hard reload

### Phase 8: `phase_8_healing_loopback_gate`

Purpose:

- central loopback decision boundary
- convert healing facts into explicit transition request

Terminal root:

- `plan_healing_loopback_transition`

### Phase 9: `phase_9_batch_complete`

Purpose:

- finalize batch and terminate orchestration

Terminal root:

- `complete_batch`

## Transition Planning Is First-Class

All phase transition logic is centralized in one transition planner table.

Each phase references policy by id. Policies inspect state facts and return one
of:

- `advance`
- `repeat`
- `jump`
- `loop_back`
- `stop`

This prevents duplicated transition condition code across phase implementations.

## Why Not One Flat Giant DAG

A single giant flat graph for every phase and leaf prereq obscures control-flow
domains that are not ordinary prerequisites:

- short-circuit / stop semantics
- restart-healing loopback
- selected-next-phase behavior based on newly produced facts

The right split is:

- phase orchestrator controls cross-phase control-flow and transition policy
- `kit/tasks` controls intra-phase prerequisite DAG

## No-Wait Hook Positioning

Concurrent-no-wait hooks are not a separate lifecycle lane domain.

They are flighted from the hook timing phase (`phase_2_pre_hook_adjustment`) as
non-awaited work. They do not gate critical-path phase completion and do not own
transition decisions.

## Implementation Direction (Current)

1. Keep the orchestrator matrix dynamic (`[]Phase`, `[]Cell` per phase).
2. Keep transition planning centralized and policy-driven.
3. Stub terminal roots and prerequisite tasks with real names first.
4. Wire dependencies in tasks immediately; leave side-effect internals empty
   until task map is complete.

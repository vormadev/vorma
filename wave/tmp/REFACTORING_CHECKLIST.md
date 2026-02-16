# Wave Refactoring Checklist

## Purpose

This checklist tracks refactors for `wave/*` with a focus on deterministic
lifecycle behavior, explicit execution seams, and debuggable state transitions.

Goals:

- Make lifecycle behavior explicit and deterministic.
- Remove hidden cross-cycle side effects.
- Improve operational debuggability.
- Keep changes handoff-safe across agents.

Out of scope:

- Feature expansion unrelated to lifecycle/state robustness.
- Back-compat adapter layers for old internal behavior.

## Working Rules

- Keep this file up to date as tasks move.
- Mark completed items with `[x]`.
- Add brief notes for non-obvious decisions in the handoff section.
- Prefer small, reviewable slices with tests per slice.
- Do not merge behavior-changing work without focused tests.

## Desired End State

- Devserver is driven by an explicit state machine and typed events.
- Restart intent is accumulated and consumed deterministically.
- All async work is cycle-scoped and cancelable.
- Hook pipeline continuation/stop rules are explicit and testable.
- Watcher planning is immutable and separate from parsed config mutation.
- Browser reload orchestration has explicit outcome/fallback behavior.
- App process lifecycle errors are surfaced and observable.
- Logs include transition-level trace context (`cycle_id`, state, event).

## Refactor Tracks

### Track A: Lifecycle State Machine

- [x] Introduce explicit lifecycle states in
      `wave/tooling/devserver_runtime.go`.
- [x] Introduce typed lifecycle events and transition reducer.
- [x] Move side effects into command execution layer (not in transition logic).
- [x] Replace implicit loop flow with state-machine-driven loop.
- [x] Add transition tests for happy path, build failure, retry, config restart,
      shutdown.

### Track B: Restart Intent Accumulator

- [x] Replace ad hoc restart channel semantics with deterministic intent
      accumulator in `wave/tooling/devserver_restart.go`.
- [x] Ensure retry paths cannot drop restart/config intent.
- [x] Define merge rules once and test all combinations.
- [x] Ensure consumer always reads and clears intent atomically.

### Track C: Cycle-Scoped Context and Async Group

- [x] Introduce cycle context/group object for each run cycle.
- [x] Attach watcher loop, no-wait hooks, and async hook commands to cycle
      scope.
- [x] Cancel and join cycle work during rebuild/restart cleanup.
- [x] Replace blocking no-wait limiter behavior with queue/worker behavior that
      does not stall main pipeline.

### Track D: Deterministic Hook Pipeline Contract

- [x] Introduce typed phase result object (`actions`, `errors`, `restart_delta`,
      `continue`).
- [x] Centralize continuation policy (explicit fail-open/fail-closed rules).
- [x] Make stage error handling intentional and testable.
- [x] Keep action reduction order deterministic and documented in tests.

### Track E: Watcher Plan Immutability

- [x] Build immutable watcher plan from parsed config reload.
- [x] Remove in-place mutation of watched patterns/hook excludes in runtime
      config.
- [x] Normalize paths once at plan construction boundary.
- [x] Ensure `HookContext.FilePath` absolute-path contract is enforced.

### Track F: Browser Reload Orchestration Outcomes

- [x] Make reload orchestration return explicit outcome object in
      `wave/tooling/broadcast_server_actions.go`.
- [x] Define fallback rules for Vite cycle failure and readiness failures.
- [x] Ensure no path can silently suppress both cycle and payload reload.
- [x] Add tests for cycle success, cycle failure, no active Vite context, and
      fallback reload.

### Track G: App Process Manager Seam

- [x] Extract app process lifecycle into dedicated seam in
      `wave/tooling/devserver_app_process.go`.
- [x] Support graceful stop + kill fallback.
- [x] Surface stop/wait errors to lifecycle flow.
- [x] Add tests for stop failures and restart behavior after failures.

### Track H: Traceability and Debuggability

- [x] Add structured lifecycle trace fields (`cycle_id`, `batch_id`, state/event
      transitions).
- [x] Emit one transition log per state change.
- [x] Ensure hook-stage logs include cycle/batch correlation.
- [x] Add tests/snapshots for key transition log presence where practical.

### Track I: Build/Hook Ordering Contract

- [x] Make compile/build-hook ordering policy explicit.
- [x] Decide and enforce default mode for safe hook-generated artifact handling.
- [x] Add tests for both configured modes and generated-artifact races.

## Current Known Targets (Keep In Scope)

- [x] Build retry must not drop restart/config intent.
- [x] No-wait hooks must not outlive cycle boundaries.
- [x] No-wait execution must not block the main event pipeline when saturated.
- [x] `HookContext.FilePath` must always match its documented path contract.
- [x] Reload flow must never silently suppress all reload outcomes.
- [x] Hook error handling policy must be explicit and deterministic.
- [x] App stop/wait errors must be surfaced instead of swallowed.
- [x] Compile/build-hook ordering behavior must be explicit and tested.
- [x] Watcher planning/config handling must avoid hidden runtime mutation.

## Test Checklist

- [x] Unit tests for each new reducer/decision function.
- [x] End-to-end devserver orchestration tests for multi-cycle behavior.
- [x] Concurrency tests around restart storms and hook saturation.
- [x] `go test ./wave/... -count=1`.
- [x] `go test -race ./wave/tooling -count=1`.

## Handoff Notes

Use this section for short, decision-focused notes only.

- Date:
- Agent:
- In-progress track:
- What changed:
- Open questions:
- Next concrete step:

### 2026-02-15

- Agent: Codex
- In-progress track: Regression test scaffolding before implementation
- What changed:
    - Added regression test for build-retry restart intent propagation in
      `wave/tooling/devserver_run_test.go`.
    - Added regression test for no-wait hook cancellation on rebuild cleanup in
      `wave/tooling/events_hook_execution_test.go`.
    - Added regression test for non-blocking no-wait scheduling under saturation
      in `wave/tooling/events_hook_no_wait_limit_test.go`.
    - Added regression test for absolute `HookContext.FilePath` contract in
      `wave/tooling/events_plan_test.go`.
    - Added regression test for cycle-vite failure fallback reload behavior in
      `wave/tooling/broadcast_behavior_test.go`.
    - Added regression test for surfacing app stop termination errors in
      `wave/tooling/devserver_app_process_test.go`.
- Open questions:
    - Whether to keep both old and new no-wait saturation semantics tests in the
      same phase, or update old semantics tests once Track C starts.
- Next concrete step:
    - Start Track B + Track C implementation slice and make the six new
      regression tests pass.

### 2026-02-15 (Implementation Pass)

- Agent: Codex
- In-progress track: Track B, Track C, Track E, Track F, Track G
- What changed:
    - Propagated retry restart intent through `waitForBuildRetry` so retry
      restarts preserve `recompileGo`/config intent into the next run cycle.
    - Introduced explicit `runIntent` derivation in
      `wave/tooling/devserver_runtime.go` so restart intent handling is a typed
      state transition instead of ad hoc boolean mutation.
    - Added deterministic retry-intent unit coverage in
      `wave/tooling/devserver_run_test.go` and removed a flaky full-run variant
      that could be polluted by unrelated restart upgrades.
    - Introduced cancelable no-wait hook lifecycle context on `server` and
      canceled it from rebuild/full cleanup, so in-flight no-wait hooks observe
      cycle teardown.
    - Switched no-wait limiter scheduling to non-blocking caller semantics while
      preserving execution cap.
    - Enforced absolute-path normalization for hook context file paths.
    - Made Vite cycle application return success/failure and used that explicit
      outcome to drive payload fallback broadcasting.
    - Surfaced process termination/wait errors from `stopApp` while ignoring
      expected forced-termination wait statuses.
    - Updated conflicting saturation/path-shape tests to match the redesigned
      contracts.
- Open questions:
    - Whether Track C should also fold watcher goroutine lifecycle into the same
      cycle-scoped cancellation object (currently no-wait hooks are covered).
    - Whether Track G should move from force-kill to graceful interrupt with
      timeout + kill fallback.
- Next concrete step:
    - Implement Track A typed lifecycle reducer so state transitions and side
      effects are fully explicit and traceable.

### 2026-02-16

- Agent: Codex
- In-progress track: Track A lifecycle state machine
- What changed:
    - Added explicit run lifecycle states/events and reducer in
      `wave/tooling/devserver_runtime_state_machine.go`.
    - Refactored `server.run()` into an explicit state-machine loop in
      `wave/tooling/devserver_runtime.go`.
    - Split run-loop side effects into dedicated execution seams:
      `prepareRunCycle`, `executeRunBuildForIntent`, and `startRunCycleRuntime`.
    - Added transition unit tests in
      `wave/tooling/devserver_runtime_state_machine_test.go` covering valid and
      invalid transitions.
- Open questions:
    - Whether to extract current in-loop side effects into command objects
      (`[]lifecycleCommand`) for full reducer purity under Track A.
- Next concrete step:
    - Move remaining run-loop side effects behind explicit command execution so
      transition logic is fully side-effect free.

### 2026-02-16 (Command Execution Layer)

- Agent: Codex
- In-progress track: Track A lifecycle state machine
- What changed:
    - Added explicit run lifecycle command types and command input/result
      contracts in `wave/tooling/devserver_runtime.go`.
    - Refactored `server.run()` to execute a deterministic
      `state -> command -> event -> transition` loop.
    - Added `deriveRunLifecycleCommandForState` and `executeRunLifecycleCommand`
      to isolate lifecycle side effects from transition logic.
    - Extended state-machine unit tests in
      `wave/tooling/devserver_runtime_state_machine_test.go` to cover command
      mapping and unknown-state handling.
- Open questions:
    - Whether to split `executeRunLifecycleCommand` into dedicated files per
      command for maintainability as new states are added.
- Next concrete step:
    - Start Track D by introducing typed hook-stage result policies so
      continuation/restart semantics are reducer-driven as well.

### 2026-02-16 (Completion Pass)

- Agent: Codex
- In-progress track: Track B through Track I completion
- What changed:
    - Added `restartIntentAccumulator` and routed lifecycle restart enqueue and
      consume paths through deterministic accumulator methods in
      `wave/tooling/devserver_restart.go` and
      `wave/tooling/devserver_runtime.go`.
    - Added cycle-scoped async lifecycle management (`runCycleScope`) and wired
      watcher goroutines, no-wait hook execution, and concurrent hook pipeline
      contexts to cycle cancellation/join boundaries in
      `wave/tooling/devserver_cycle_scope.go`,
      `wave/tooling/devserver_runtime.go`, and
      `wave/tooling/events_hook_executor.go`.
    - Replaced watcher setup config mutation with immutable watcher-plan
      construction (normalized watched-file copies, normalized excludes, sorted
      hooks) in `wave/tooling/watcher_setup.go`, and switched matching to use
      watcher-owned immutable copies in `wave/tooling/watcher_matching.go`.
    - Removed hook-plan-time mutation of `WatchedFile.SortedHooks` by deriving
      sorted hooks without mutating runtime config in
      `wave/tooling/events_plan_hooks.go`.
    - Introduced explicit reload orchestration outcome/result contracts in
      `wave/tooling/broadcast_server_actions.go` and extended coverage in
      `wave/tooling/broadcast_behavior_test.go` and
      `wave/tooling/devserver_orchestration_test.go`.
    - Extracted app process lifecycle into `appProcessManager` seam with
      graceful interrupt and kill fallback in
      `wave/tooling/devserver_app_process.go`, plus fallback coverage in
      `wave/tooling/devserver_app_process_test.go`.
    - Added trace correlation helpers and logs (`cycle_id`, `batch_id`) for
      watcher and hook-stage execution paths in
      `wave/tooling/devserver_trace_context.go`,
      `wave/tooling/events_watcher_intake.go`,
      `wave/tooling/events_hook_executor.go`, and
      `wave/tooling/events_execution_continuation.go`.
    - Added transition log assertions for lifecycle state transitions in
      `wave/tooling/devserver_runtime_state_machine_test.go` and hook-stage
      trace assertions in `wave/tooling/events_hook_trace_test.go`.
    - Added explicit build/hook ordering policy decision contracts in
      `wave/tooling/devserver_build_ordering.go` and tests in
      `wave/tooling/devserver_build_ordering_test.go`.
    - Verified `go test ./wave/... -count=1` and
      `go test -race ./wave/tooling -count=1`.
- Open questions:
    - None.
- Next concrete step:
    - Keep this checklist current during any follow-on cleanup/refinement work.

### 2026-02-16 (Additional Follow-Ups)

- Agent: Codex
- In-progress track: Post-completion cleanup follow-ups
- What changed:
    - Removed `restartCh` compatibility surface from `wave/tooling` runtime and
      tests so restart intent flow is accumulator-first, and updated test
      helpers to queue/consume restart intents through server methods.
    - Split run lifecycle command handlers into dedicated command-handler
      methods in `wave/tooling/devserver_runtime_command_handlers.go` so
      `executeRunLifecycleCommand` is dispatch-only.
    - Added explicit configurable hook-stage failure policy via
      `Watch.HookStageFailurePolicy` (`fail-open` / `fail-closed`), including
      schema + validation and continuation-policy coverage.
    - Extracted immutable watcher-plan construction into dedicated `watcherPlan`
      types/helpers in `wave/tooling/watcher_plan.go`, keeping
      `wave/tooling/watcher_setup.go` as thin orchestration.
    - Re-verified `go test ./wave/tooling -count=1`,
      `go test -race ./wave/tooling -count=1`, `go test ./wave/... -count=1`,
      and `go test ./... -count=1`.
- Open questions:
    - None.
- Next concrete step:
    - Call this redesign phase complete unless new behavior requirements emerge.

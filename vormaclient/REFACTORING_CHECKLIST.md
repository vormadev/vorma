# Vormaclient Refactoring Checklist

This checklist tracks the deterministic lifecycle/state-machine refactor for
`vormaclient/client`.

## Mission

- Make navigation, revalidation, prefetch, submission, and POP history
  lifecycles deterministic.
- Improve debuggability with explicit state transitions and traceable outcomes.
- Reduce hidden side effects and duplicated logic.
- Keep behavior stable from the outside while hardening internals.

## Non-Negotiable Outcomes

- No silent failure swallowing in internal runtime flows.
- No stale operation can commit side effects.
- Skip-path behavior is observationally equivalent to server-path behavior.
- History POP processing is ordered and race-safe.
- Revalidation conflation is deterministic and not dependent on tiny wall-clock
  windows.

## Workstreams

### 1. Transactional Navigation State Machine

- [x] Define explicit navigation events and terminal states.
- [x] Centralize transition logic into one reducer/transition seam.
- [x] Remove scattered lifecycle branching in begin/outcome/render paths.
- [x] Ensure every transition has a reason/cause string for diagnostics.

### 2. Operation IDs and Commit Fences

- [x] Add monotonic operation IDs for navigation and submission entries.
- [x] Gate side effects (history, global state, head, render, CSS/module
      preload) on current ownership.
- [x] Ensure stale completions are no-ops by construction, not by best-effort
      checks.

### 3. Explicit Concurrency Lanes and Arbitration Rules

- [x] Model lanes explicitly: `active`, `revalidation`, `prefetch`,
      `submission`.
- [x] Document and implement supersede/reuse/abort rules in one arbitration
      matrix.
- [x] Apply the same arbitration semantics across `userNavigation`,
      `browserHistory`, and `redirect`.

### 4. Deterministic Revalidation Conflation

- [x] Replace `Date.now()` micro-window coalescing with state-based conflation.
- [x] Implement one-in-flight plus optional trailing revalidation
      (`needsRevalidateAgain` style).
- [x] Keep policy-level throttling/debounce at trigger layer (for example focus
      revalidate), not core state machine.

### 5. Lifecycle Debug Journal

- [x] Add an internal ring buffer journal of lifecycle transitions.
- [x] Record operation ID, lane, from/to state, reason, and causal links (for
      example superseded-by).
- [x] Expose a safe debug access seam for tests/dev inspection.

### 6. Unified Route Metadata Seam (Parity Contract)

- [x] Extract shared route metadata application into one helper.
- [x] Use that helper for init path, server-success path, and skip-success path.
- [x] Enforce parity for title/head/error-boundary/module-map behavior between
      skip and server outcomes.

### 7. Serialized POP History Pipeline

- [x] Serialize POP handling (queue/mutex) so updates are processed in-order.
- [x] Prevent overlapping POP flows from reordering `lastKnownLocation` updates.
- [x] Keep cross-document fallback behavior intact while guaranteeing ordering.

### 8. Rich Internal Navigate Result

- [x] Replace internal boolean-ish navigation result with explicit outcome union
      (committed/cancelled/failed with reason).
- [x] Preserve ergonomic public API (`vormaNavigate`) while improving internal
      observability.
- [x] Ensure failures are surfaced in debug paths instead of being silently
      swallowed.

## Test and Verification Checklist

- [x] Update contract tests for state-machine/lifecycle invariants.
- [x] Add targeted tests for stale side-effect fencing across all lanes.
- [x] Add tests for deterministic revalidation conflation under rapid repeated
      requests.
- [x] Add tests for POP queue ordering under overlapping async POP updates.
- [x] Add tests proving skip/server metadata parity for
      head/title/error-boundary/module-map.
- [x] Run full relevant test suites and record command + result in handoff log.

## Incremental Delivery Plan

- [x] Phase A: State machine core + operation IDs + arbitration matrix.
- [x] Phase B: Revalidation conflation + POP serialization.
- [x] Phase C: Metadata seam unification + skip/server parity.
- [x] Phase D: Debug journal + rich internal navigate result.
- [x] Phase E: Test hardening and cleanup.

## Follow-Up Simplification Wave

Post-checklist audit follow-ups to reduce abstraction layering and file
complexity while preserving deterministic behavior.

### 1. Successful Navigation Decision/Execution Simplification

- [x] Collapse overlapping successful-navigation decision layers
      (`stage`/`checkpoint`/`post-asset lifecycle`) into one checkpoint plan.
- [x] Replace thin per-stage command mappers with one checkpoint-to-command
      builder seam.
- [x] Convert successful-navigation checkpoint execution to table-driven
      decide/build/execute flow with fewer per-case branches.

### 2. Submit Runtime Simplification and Ownership Hardening

- [x] Replace duplicated submit staleness checkpoint branching with a generated
      map/template-based reducer.
- [x] Switch submit ownership/staleness gating from object identity to explicit
      submission operation-ID ownership.
- [x] Flatten submit post-request/finalize/error action layering into one
      linearized pipeline with explicit checkpoint guards.

### 3. Runtime Surface Area Reduction

- [x] Split `render_runtime.ts` into focused modules
      (`component_runtime`/`client_loader_runtime`/`render_commit_runtime`) with
      a thin compatibility facade.
- [x] Split `links.ts` into click lifecycle and prefetch lifecycle modules with
      shared target classification utilities.
- [x] Remove one outcome passthrough layer by merging redundant
      planner/command-wrapper indirection in navigation outcome execution.

## Handoff Log

Update this section each working session.

- [x] `2026-02-15` Agent: `Codex (GPT-5)`
- [x] Completed:
- [x] Regression tests added for POP overlap ordering, deterministic trailing
      revalidation, skip/server metadata parity, stale redirect-follow-up
      fencing, and stale preload/build-id side-effect fencing.
- [x] Revalidation runtime moved from clock-window coalescing to deterministic
      lane arbitration (one in-flight + optional trailing pass).
- [x] Skip-path now preserves per-route `errorExportKeys`, keeping module-map
      parity with server outcomes.
- [x] Fetch-phase preload side effects now respect abort state, preventing stale
      aborted completions from preloading module/CSS assets.
- [x] History POP listener now uses sequence fencing so older async POP
      completions cannot overwrite newer `lastKnownLocation`.
- [x] Navigation entries now carry monotonic `operationID` values.
- [x] In progress:
- [x] Phase A/B convergence: operation IDs are in place for navigation entries,
      but full cross-lane commit-fence wiring and explicit reducer/state-machine
      centralization are still pending.
- [x] Blockers:
- [x] None.
- [x] Next step:
- [x] Implement explicit arbitration matrix + transition reducer seam, then
      complete submission-lane operation IDs and shared route-metadata helper
      extraction.
- [x] Tests run:
- [x] `pnpm vitest vormaclient/client/src/tests/unit/history_listener_prelude.test.ts vormaclient/client/src/tests/contracts/client.state_and_revalidation.contract.test.ts vormaclient/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_modes.contract.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `6 files, 162 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `29 files, 482 tests passed`.
- [x] `2026-02-16` Agent: `Codex (GPT-5)`
- [x] Completed:
- [x] Added lifecycle debug journal ring buffer in navigation runtime with
      explicit `lane`, `operationID`, `fromState`, `toState`, `reason`, and
      `causedByOperationID`.
- [x] Added debug seam methods (`getDebugJournal`, `clearDebugJournal`) on
      runtime, and exported client accessors for test/dev inspection.
- [x] Extended operation IDs to submission entries and recorded submission
      lifecycle transitions (including dedupe causal links).
- [x] Added unit tests verifying debug journal coverage for navigation
      transitions and submission dedupe causality.
- [x] Extracted transition-event reduction and journal sink logic into
      `runtime_state_machine.ts` so `runtime.ts` dispatches explicit transition
      events instead of embedding all transition-to-journal logic inline.
- [x] Extracted deterministic revalidation lane arbitration into
      `runtime_revalidation_lane.ts` so `runtime.ts` no longer contains the
      in-flight/trailing pass state machine implementation details.
- [x] Added explicit begin-navigation arbitration reducer seam in
      `begin_navigation_state_machine.ts`, including deterministic lane
      snapshot-to-plan reduction (`abort`, `reuse`, `promote`, `create`) in one
      place.
- [x] Unified begin-navigation arbitration semantics for
      `userNavigation`/`browserHistory`/`redirect`/`action` so they all apply
      the same active-lane supersede/reuse rules.
- [x] Removed `begin_navigation_flow.ts`; begin-path branching now routes
      through the new execution-plan reducer seam.
- [x] Added unit tests covering browser-history/redirect parity for same-target
      reuse-and-promotion and stale-lane abort behavior.
- [x] Extracted top-level outcome routing and successful-navigation stage
      branching into explicit reducer seams in
      `runtime_navigation_outcome_state_machine.ts`, so
      `runtime_navigation_outcome.ts` now executes plans instead of embedding
      all decision branching inline.
- [x] Added an internal navigate-result union (`committed` vs `cancelled` with
      reason) for outcome execution, while preserving the public
      `{ didNavigate }` API surface.
- [x] Added focused unit tests for outcome reducer decisions and stage plans in
      `navigation_outcome_state_machine_internal.test.ts`.
- [x] Added lifecycle transition executor seam in
      `runtime_lifecycle_transitions.ts` so phase/removal mutations return typed
      transition events (`navigation_phase_transitioned`, `navigation_removed`)
      from one reducerized path.
- [x] Rewired `runtime.ts` to dispatch journal events from lifecycle transition
      executor outputs for begin arbitration, phase transitions, removals,
      submission state transitions, and clear-all snapshots.
- [x] Added focused unit tests for lifecycle transition executor behavior in
      `navigation_lifecycle_transitions_internal.test.ts`.
- [x] Added `runtime_lifecycle_runtime.ts` seam that encapsulates lifecycle
      mutation + transition dispatch orchestration, reducing `runtime.ts`
      orchestration coupling and centralizing debug-journal ownership.
- [x] Added focused unit tests for lifecycle runtime seam behavior in
      `navigation_lifecycle_runtime_internal.test.ts`.
- [x] Hardened build-ID commit fences for successful navigation processing by
      applying explicit sync-timing policy: idle prefetch syncs before asset
      wait, while navigational entries sync only after post-wait
      ownership/staleness checks.
- [x] Added regression coverage proving stale same-target late completions do
      not update build ID when ownership was lost during wait.
- [x] Completed successful-outcome lifecycle command seam wiring in
      `runtime_navigation_outcome.ts` by removing legacy action helpers and
      executing pre-waiting, post-waiting, and post-asset reducer plans through
      command builders in `runtime_navigation_successful_commands.ts`.
- [x] Aligned successful-navigation phase transition context to explicit
      object-shaped arguments (`targetUrl`, `phase`, `reason`) from
      `runtime.ts`, keeping lifecycle reason propagation deterministic.
- [x] Added focused unit coverage for successful lifecycle command mapping in
      `navigation_successful_commands_internal.test.ts`.
- [x] Extracted shared route module-metadata helpers in `route_metadata.ts` and
      rewired both init bootstrap
      (`initializeClientModuleMapFromInitialRouteState`) and successful outcome
      processing (`applyResponseArtifactsWhenBuildMatches`) to use the same
      merge/build implementation.
- [x] Added focused unit coverage for route metadata helpers in
      `route_metadata_internal.test.ts`.
- [x] Hardened successful-outcome side-effect commit fences by deferring route
      module/CSS artifact application until after post-asset ownership checks,
      while preserving build-match semantics against the pre-processing build
      snapshot.
- [x] Extended stale same-target late-completion regression coverage to assert
      stale outcomes cannot mutate `clientModuleMap`.
- [x] Added render commit fencing in `__reRenderApp` via `shouldCommit` so stale
      ownership loss during async module resolution cannot commit route data,
      history, title/head updates, or route-change events.
- [x] Refactored render-runtime component/error-boundary mutation into explicit
      module-map application helpers and switched rerender flow to
      prepare-then-commit sequencing.
- [x] Added regression coverage proving stale ownership loss during async render
      preparation does not apply render side effects.
- [x] Serialized `customHistoryListener` POP processing through a queue tail so
      overlapping POP updates execute strictly in enqueue order.
- [x] Updated POP overlap unit coverage to assert in-order queue execution
      semantics while preserving cross-document fallback and navigation-mode
      behavior.
- [x] Added explicit internal navigate `failed` result
      (`navigate_promise_rejected`) and rewired runtime navigate execution to
      map committed/cancelled/failed internal outcomes through
      `toPublicNavigateResult` without changing public API shape.
- [x] Added explicit `navigation_failed` transition events for navigate-promise
      rejections so failure reasons are visible in debug journal traces even
      when ownership is stale.
- [x] Removed implicit phase-transition fallback reasons from lifecycle runtime
      seams so navigation phase transitions are reason-required by type.
- [x] Refactored runtime lane storage to one explicit lane model in
      `runtime_slots.ts` (`active`, `revalidation`, `prefetch`, `submissions`)
      and switched `runtime.ts` to operate on this shared lane object instead of
      split slot/submission state.
- [x] Rewired begin/control/lifecycle seams to consume explicit lane naming
      (`revalidation` lane instead of `pendingRevalidation`) so arbitration and
      transition reducers use a single lane vocabulary end-to-end.
- [x] Updated lane/lifecycle unit tests to assert lane-model semantics via the
      new `lanes` object contracts and helper signatures.
- [x] Made lifecycle deletion transitions reason-required (removed implicit
      delete fallback reasons) and rewired active/prefetch/revalidation
      fetch-error cleanup through `deleteNavigation` transition seams so these
      removals are journaled and reason-tagged.
- [x] Added explicit unit coverage for fetch-rejection deletion reasons across
      active/prefetch/revalidation lanes.
- [x] Added contract coverage that asserts journal reasons are non-empty and
      every settled operation ends in a terminal state (`removed`/`complete`/
      `failed`/`aborted`) after mixed navigation + submit flows.
- [x] Consolidated successful-navigation stage branching behind one explicit
      stage-plan seam in `runtime_navigation_outcome_state_machine.ts`
      (`decideSuccessfulNavigationLifecycleStageExecutionPlan`) and one unified
      stage-command seam in `runtime_navigation_successful_commands.ts`
      (`buildSuccessfulNavigationLifecycleStageCommands`).
- [x] Rewired `processSuccessfulNavigationRuntime` to execute stage plans
      generically (`pre_waiting`/`post_waiting`/`post_asset`) through one
      executor path, reducing outcome/render-path branching and keeping stage
      transitions reason-driven.
- [x] Added focused unit coverage for the unified stage-plan seam and stage
      command seam.
- [x] Added explicit post-asset side-effect reducer seam in
      `runtime_navigation_outcome_state_machine.ts`
      (`decideSuccessfulNavigationPostAssetSideEffectPlan`) so
      client-loader-state commit, build-ID sync-after-wait, and
      response-artifact apply decisions are made in one typed place.
- [x] Rewired successful-navigation processing to use post-asset side-effect
      plans and a generic stage loop (`pre_waiting`/`post_waiting`) so
      stage-control branching in `runtime_navigation_outcome.ts` is reduced.
- [x] Hardened stale commit fences by deferring `setClientLoadersState` until
      after post-asset ownership/staleness checks (stopped post-asset plans no
      longer commit stale loader state).
- [x] Added regression coverage proving stale ownership loss during asset wait
      cannot commit client-loader state.
- [x] Added explicit successful-navigation command builders for build-ID sync
      timing (`before_asset_wait`) and post-asset side-effect sequencing in
      `runtime_navigation_successful_commands.ts`.
- [x] Rewired `processSuccessfulNavigationRuntime` to execute pre-asset sync and
      post-asset side effects through typed command lists, reducing inline
      branch logic in `runtime_navigation_outcome.ts`.
- [x] Added unit coverage asserting deterministic post-asset command ordering
      (commit client loaders, sync build ID, apply artifacts, then render/stop).
- [x] Added explicit top-level navigation outcome command seam in
      `runtime_navigation_outcome_commands.ts`
      (`buildNavigationOutcomeExecutionCommands`) to reducerize
      `stop`/`deleteAndStop`/`redirect`/`success` effectuation.
- [x] Rewired `runtime_navigation_outcome.ts` to execute top-level outcome
      command plans generically (delete, redirect build-ID sync, redirect
      effectuation, intent resolution, success execution, and terminal result
      commands) instead of direct per-outcome branching.
- [x] Added focused unit coverage for top-level outcome command mapping in
      `navigation_outcome_commands_internal.test.ts`.
- [x] Added explicit successful-navigation cleanup reducer seam in
      `runtime_navigation_outcome_state_machine.ts`
      (`decideSuccessfulNavigationCleanupExecutionPlan`) for deterministic
      ownership/idle-prefetch cleanup decisions.
- [x] Added cleanup command builder in
      `runtime_navigation_successful_commands.ts`
      (`buildSuccessfulNavigationCleanupCommands`) and rewired
      `runtime_navigation_outcome.ts` finally-block cleanup to execute through
      typed command plans.
- [x] Added focused unit coverage for cleanup decision/command seams in
      `navigation_outcome_state_machine_internal.test.ts` and
      `navigation_successful_commands_internal.test.ts`.
- [x] Added explicit pre-asset wait decision seam in
      `runtime_navigation_outcome_state_machine.ts`
      (`decideSuccessfulNavigationPreAssetWaitExecutionPlan`) so build-ID
      sync-before-wait policy is represented as a typed execution plan.
- [x] Added explicit combined post-asset lifecycle decision seam in
      `runtime_navigation_outcome_state_machine.ts`
      (`decideSuccessfulNavigationPostAssetLifecycleExecutionPlan`) so
      post-asset stage + side-effect decisions are composed in one reducerized
      plan object.
- [x] Rewired successful-navigation command builders to consume composed
      execution plans (`preAssetWaitExecutionPlan`,
      `postAssetLifecycleExecutionPlan`) instead of raw policy and split plan
      inputs, reducing cross-layer orchestration coupling in
      `runtime_navigation_outcome.ts`.
- [x] Added focused unit coverage for the new pre-asset and combined post-asset
      decision seams in `navigation_outcome_state_machine_internal.test.ts`.
- [x] Added explicit successful-navigation lifecycle checkpoint seam in
      `runtime_navigation_outcome_state_machine.ts`
      (`decideSuccessfulNavigationLifecycleCheckpointExecutionPlan`) that
      unifies decision routing for `pre_waiting`, `post_waiting`,
      `pre_asset_wait`, `post_asset`, and `cleanup`.
- [x] Added unified checkpoint command seam in
      `runtime_navigation_successful_commands.ts`
      (`buildSuccessfulNavigationLifecycleCheckpointCommands`) so checkpoint
      plans map to commands in one place.
- [x] Rewired `processSuccessfulNavigationRuntime` checkpoint orchestration in
      `runtime_navigation_outcome.ts` to decide-and-execute via one checkpoint
      executor helper, reducing direct stage/policy branching in runtime code.
- [x] Added focused unit coverage for lifecycle checkpoint decision + command
      seams in `navigation_outcome_state_machine_internal.test.ts` and
      `navigation_successful_commands_internal.test.ts`.
- [x] In progress:
- [x] Begin-path and outcome-stage branching are now reducerized, but full
      transition unification into one navigation lifecycle reducer, contract
      invariant hardening, and remaining commit-fence hardening across all
      side-effect surfaces are still pending.
- [x] Blockers:
- [x] None.
- [x] Next step:
- [x] Continue extracting outcome/render branching into explicit reducer seams
      that consume/emit journaled transition events, then complete remaining
      commit-fence and contract-invariant workstreams.
- [x] Tests run:
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/redirects_internal.test.ts`
- [x] Result: `2 files, 94 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `29 files, 484 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 95 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `29 files, 484 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.state_and_revalidation.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `3 files, 112 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `29 files, 484 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `3 files, 117 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.prefetch.contract.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `2 files, 115 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `29 files, 487 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `4 files, 128 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `30 files, 498 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_lifecycle_transitions_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts`
- [x] Result: `4 files, 124 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `31 files, 504 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_lifecycle_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_lifecycle_transitions_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts`
- [x] Result: `4 files, 107 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `32 files, 506 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts`
- [x] Result: `3 files, 115 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `32 files, 507 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts`
- [x] Result: `4 files, 134 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `32 files, 507 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_successful_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts`
- [x] Result: `3 files, 104 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `33 files, 511 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/route_metadata_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts vormaclient/client/src/tests/contracts/client.history_and_init.contract.test.ts`
- [x] Result: `4 files, 149 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 514 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/route_metadata_internal.test.ts vormaclient/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts`
- [x] Result: `4 files, 118 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 514 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/render_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts`
- [x] Result: `4 files, 141 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 515 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 515 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/history_listener_prelude.test.ts vormaclient/client/src/tests/contracts/client.history_and_init.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_modes.contract.test.ts`
- [x] Result: `3 files, 71 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 515 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/history_listener_prelude.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.history_and_init.contract.test.ts`
- [x] Result: `5 files, 173 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 515 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_lifecycle_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts`
- [x] Result: `3 files, 103 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 515 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_lifecycle_runtime_internal.test.ts`
- [x] Result: `2 files, 91 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 515 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_lifecycle_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_lifecycle_transitions_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts`
- [x] Result: `4 files, 109 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_modes.contract.test.ts vormaclient/client/src/tests/contracts/client.history_and_init.contract.test.ts vormaclient/client/src/tests/contracts/client.state_and_revalidation.contract.test.ts`
- [x] Result: `5 files, 107 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 515 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts`
- [x] Result: `3 files, 120 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 517 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 517 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_successful_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `3 files, 108 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 519 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_successful_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `3 files, 110 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 521 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_successful_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `2 files, 98 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `34 files, 523 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `3 files, 111 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `35 files, 529 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_successful_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `3 files, 114 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `35 files, 531 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_successful_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `3 files, 116 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `35 files, 533 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_successful_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `3 files, 118 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `35 files, 535 tests passed`.
- [x] Added explicit begin-navigation command seam in
      `begin_navigation_commands.ts` (`buildBeginNavigationExecutionCommands`)
      so begin execution plans map to typed commands in one reducerized path.
- [x] Rewired `begin_navigation.ts` to execute begin-navigation behavior through
      command interpretation (`abort`/`reuse`/`immediate-abort`/`create`) rather
      than direct per-plan branching.
- [x] Added focused unit coverage for begin command mapping in
      `begin_navigation_commands_internal.test.ts`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/begin_navigation_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_successful_commands_internal.test.ts`
- [x] Result: `5 files, 130 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `36 files, 541 tests passed`.
- [x] Moved navigation-intent resolution behind successful commit fences by
      introducing `onSuccessfulNavigationCommitted` in
      `processSuccessfulNavigationRuntime` context and wiring
      `onNavigationIntentResolved` through that post-commit callback in
      `runtime.ts`.
- [x] Removed top-level outcome-layer intent-resolution branching
      (`resolve_navigation_intent` command and `shouldResolveIntent` metadata),
      so intent side effects are no longer emitted before post-wait/post-asset
      ownership checks.
- [x] Added focused runtime regression coverage proving intent-resolution fires
      on committed success and is suppressed for stale successful completions.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/begin_navigation_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.loading_and_focus.contract.test.ts`
- [x] Result: `5 files, 155 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `36 files, 543 tests passed`.
- [x] Added explicit focus-trigger policy state-machine seam in
      `revalidation_focus_trigger_policy_state_machine.ts`
      (`decideFocusRevalidationTriggerExecutionPlan`) so focus revalidation
      trigger decisions (`navigating`/`submitting`/`revalidating`/ stale-window
      gating) are reducerized with explicit reason strings.
- [x] Rewired `revalidateOnWindowFocus` in `core/extras.ts` to execute
      revalidation via the focus-trigger execution plan instead of inline
      branching.
- [x] Added focused unit coverage for focus-trigger policy decisions in
      `revalidation_focus_trigger_policy_state_machine_internal.test.ts`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/revalidation_focus_trigger_policy_state_machine_internal.test.ts vormaclient/client/src/tests/unit/extras_internal.test.ts vormaclient/client/src/tests/contracts/client.loading_and_focus.contract.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `4 files, 133 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `37 files, 548 tests passed`.
- [x] Replaced mutable trigger timestamp singleton in `client.ts` with explicit
      timestamp-state runtime (`createRevalidationTriggerTimestampRuntime`)
      backed by reducer seam (`reduceRevalidationTriggerTimestampState`), so
      navigation/revalidation-intent committed events drive timestamp state
      transitions explicitly.
- [x] Added focused unit coverage for trigger timestamp reducer/runtime in
      `revalidation_trigger_timestamp_state_machine_internal.test.ts`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/revalidation_trigger_timestamp_state_machine_internal.test.ts vormaclient/client/src/tests/unit/revalidation_focus_trigger_policy_state_machine_internal.test.ts vormaclient/client/src/tests/contracts/client.loading_and_focus.contract.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `4 files, 134 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `38 files, 552 tests passed`.
- [x] Added explicit render commit checkpoint state-machine seam in
      `render_runtime_commit_state_machine.ts`
      (`decideRenderCommitCheckpointExecutionPlan`) so pre/post module-load
      commit-fence checks run through typed execution plans.
- [x] Added explicit render commit command seam in
      `render_runtime_commit_commands.ts` (`buildRenderCommitCommands`) so
      render side-effect sequencing (route/global state apply, history/scroll,
      title, CSS, route-change event, head updates, finish) executes via typed
      commands rather than inline branching.
- [x] Rewired `__reRenderAppInner` in `render_runtime.ts` to execute render
      commit checkpoints + command plans generically.
- [x] Added focused unit coverage for render commit checkpoint and command seams
      in `render_runtime_commit_state_machine_internal.test.ts` and
      `render_runtime_commit_commands_internal.test.ts`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/render_runtime_commit_state_machine_internal.test.ts vormaclient/client/src/tests/unit/render_runtime_commit_commands_internal.test.ts vormaclient/client/src/tests/unit/render_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `5 files, 139 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `40 files, 558 tests passed`.
- [x] Added explicit redirect effectuation state-machine seam in
      `redirect_effectuation_state_machine.ts`
      (`decideRedirectEffectuationExecutionPlan`) to reducerize
      `not-should`/`hard`/`soft`/`unknown` strategy routing.
- [x] Added explicit redirect effectuation command seam in
      `redirect_effectuation_commands.ts` (`buildRedirectEffectuationCommands`)
      so cleanup and strategy execution are command-driven.
- [x] Rewired `effectuateRedirectDataResult` in `redirects.ts` to execute
      redirect cleanup/strategy through execution plan + command interpreter
      instead of inline branching.
- [x] Added focused unit coverage for redirect effectuation decision/command
      seams in `redirect_effectuation_state_machine_internal.test.ts` and
      `redirect_effectuation_commands_internal.test.ts`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/redirect_effectuation_state_machine_internal.test.ts vormaclient/client/src/tests/unit/redirect_effectuation_commands_internal.test.ts vormaclient/client/src/tests/unit/redirects_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.submit_and_redirect.contract.test.ts`
- [x] Result: `5 files, 139 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `42 files, 566 tests passed`.
- [x] Added explicit submission lifecycle command seam in
      `runtime_submit_lifecycle_commands.ts`
      (`buildSubmissionLifecycleBeginCommands`,
      `buildSubmissionLifecycleFinishCommands`) so submission
      dedupe/start/finish transitions are command-driven.
- [x] Rewired `createSubmissionLifecycle` in `runtime_submit.ts` to execute
      submission lifecycle begin/finish behavior through command interpretation,
      removing inline mutation/transition branching.
- [x] Added focused unit coverage for submission lifecycle command mapping in
      `submission_lifecycle_commands_internal.test.ts`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/submission_lifecycle_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.submit_and_redirect.contract.test.ts vormaclient/client/src/tests/contracts/client.loading_and_focus.contract.test.ts`
- [x] Result: `4 files, 158 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `43 files, 570 tests passed`.
- [x] Added unified lifecycle transition reducer seam in
      `runtime_lifecycle_transitions.ts` (`reduceNavigationLifecycleTransition`)
      that centralizes transition reduction for navigation removal, phase
      transitions, begin arbitration, submission transitions, navigation
      failures, and clear-all snapshots.
- [x] Rewired `runtime_lifecycle_runtime.ts` to dispatch journal events via the
      unified transition reducer seam instead of calling multiple transition
      builders directly.
- [x] Extended focused lifecycle transition unit coverage to assert reducer-seam
      behavior in `navigation_lifecycle_transitions_internal.test.ts`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_lifecycle_transitions_internal.test.ts vormaclient/client/src/tests/unit/navigation_lifecycle_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `4 files, 114 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `43 files, 572 tests passed`.
- [x] Added explicit submit staleness-checkpoint reducer seam in
      `runtime_submit_staleness_state_machine.ts`
      (`decideSubmitStalenessCheckpointExecutionPlan`) covering post-request,
      pre-finalize, post-classification, post-redirect-effectuation,
      pre-success-return, and post-auto-revalidate ownership gates.
- [x] Rewired submit runtime stale ownership checks in `runtime_submit.ts` to
      execute through the staleness checkpoint seam rather than direct repeated
      inline `isCurrent` branching.
- [x] Added focused unit coverage for submit staleness checkpoint decisions in
      `submission_staleness_state_machine_internal.test.ts`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/submission_staleness_state_machine_internal.test.ts vormaclient/client/src/tests/unit/submission_lifecycle_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.submit_and_redirect.contract.test.ts vormaclient/client/src/tests/contracts/client.loading_and_focus.contract.test.ts`
- [x] Result: `5 files, 160 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `44 files, 574 tests passed`.
- [x] Added explicit server-success preload decision seam in
      `fetch_route_data_preload_state_machine.ts`
      (`decideServerSuccessPreloadExecutionPlan`) that reducerizes
      signal-aborted skip behavior and dev/prod module-preload source selection.
- [x] Added explicit server-success preload command seam in
      `fetch_route_data_preload_commands.ts`
      (`buildServerSuccessPreloadCommands`) so module and CSS preload side
      effects are emitted as typed commands from one mapping path.
- [x] Rewired `buildServerSuccessOutcome` in `fetch_route_data_server.ts` to
      execute preload behavior through the new execution-plan + command seams
      with per-command abort fencing.
- [x] Added focused unit coverage for preload decision/command seams in
      `fetch_route_data_preload_state_machine_internal.test.ts` and
      `fetch_route_data_preload_commands_internal.test.ts`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/fetch_route_data_preload_state_machine_internal.test.ts vormaclient/client/src/tests/unit/fetch_route_data_preload_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts`
- [x] Result: `5 files, 132 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 579 tests passed`.
- [x] Moved server-success preload effectuation behind successful-navigation
      lifecycle checkpoints by changing success outcomes to carry
      `preloadCommands` instead of pre-executed `cssBundlePromises`.
- [x] Rewired `buildServerSuccessOutcome` in `fetch_route_data_server.ts` and
      client-only skip success in `fetch_route_data_skip.ts` to emit typed
      preload commands without executing module/CSS preload side effects during
      fetch outcome construction.
- [x] Rewired successful-navigation asset waiting in
      `runtime_navigation_successful_runtime.ts` to execute preload commands
      only after pre-waiting/post-waiting/pre-asset checkpoint gating, then
      await CSS preload promises with existing error logging behavior.
- [x] Hardened preload command mapping in `fetch_route_data_preload_commands.ts`
      by filtering empty/non-string dependency and CSS entries at the seam
      boundary.
- [x] Updated runtime and outcome fixtures/tests for the new success outcome
      shape and preload timing semantics, including malformed preload-entry
      filtering coverage.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/fetch_route_data_preload_state_machine_internal.test.ts vormaclient/client/src/tests/unit/fetch_route_data_preload_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_commands_internal.test.ts vormaclient/client/src/tests/unit/links_internal.test.ts vormaclient/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts`
- [x] Result: `8 files, 167 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 580 tests passed`.
- [x] Added explicit operation-ID ownership on navigation controls
      (`NavigationControl.operationID`) and propagated it through
      `navigation_controls.ts` so runtime outcome handling can fence stale work
      by operation identity.
- [x] Rewired runtime outcome execution path (`runtime.ts` ->
      `handleNavigationOutcomeWithInternalResult`) to pass `expectedOperationID`
      instead of relying on promise-reference ownership in the main navigation
      flow.
- [x] Updated `decideNavigationOutcomeExecutionPlan` ownership checks to honor
      expected operation-ID ownership first, with legacy promise-ownership
      fallback for compatibility callsites.
- [x] Rewired navigate rejection cleanup in `runtime.ts` to use operation-ID
      ownership fencing before deleting entries and dispatching failure journal
      transitions.
- [x] Added focused unit coverage proving explicit operation-ID ownership allows
      correct outcome processing even when promise ownership is stale in
      `navigation_outcome_state_machine_internal.test.ts`.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/links_internal.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts`
- [x] Result: `6 files, 158 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 581 tests passed`.
- [x] Rewired link-click navigation ownership fences in `links.ts` to use
      explicit control operation IDs when available, with promise-ownership
      fallback only for compatibility paths.
- [x] Added `doesNavigationEntryBelongToControl` helper in `links.ts` so link
      outcome processing (`aborted`/`redirect`/`success`) and failed-click
      cleanup consistently share one ownership seam.
- [x] Hardened stale-suppression semantics for link flows: mismatched
      operation-ID ownership now suppresses navigation mutation side effects
      even if promise references match.
- [x] Added focused `links_internal.test.ts` coverage for operation-ID-owned
      stale-promise acceptance and operation-ID mismatch rejection semantics.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/links_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/contracts/client.link_click.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts`
- [x] Result: `6 files, 166 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 583 tests passed`.
- [x] Cleanup pass: removed redundant redirect-plan fields from
      `runtime_navigation_outcome_state_machine.ts` (`entry`,
      `shouldSyncBuildIDBeforeRedirect`) and simplified
      `runtime_navigation_outcome.ts` redirect execution to unconditionally sync
      redirect build IDs.
- [x] Cleanup pass: removed unused runtime context payload (`context`) from
      successful-navigation checkpoint execution context in
      `runtime_navigation_successful_runtime.ts`.
- [x] Updated focused navigation outcome state-machine coverage for the trimmed
      redirect plan shape.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `3 files, 117 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 581 tests passed`.
- [x] Converted successful-navigation checkpoint runtime orchestration in
      `runtime_navigation_successful_runtime.ts` to a table-driven checkpoint
      definition map (decision input + command input builders) and removed the
      long per-checkpoint switch scaffolding.
- [x] Replaced duplicated submit staleness checkpoint branching in
      `runtime_submit_staleness_state_machine.ts` with template-derived reasons
      and a single reducer path.
- [x] Added explicit submission operation-ID ownership helper in `types.ts` and
      rewired submit currentness checks in `runtime_submit.ts` to use
      operation-ID ownership instead of object identity.
- [x] Flattened submit runtime post-request/finalize/error layering in
      `runtime_submit.ts` into one linearized pipeline with explicit staleness
      checkpoint guards.
- [x] Added regression coverage in `navigation_runtime_internal.test.ts` proving
      submit ownership remains valid when the stored submission entry instance
      changes but keeps the same operation ID.
- [x] Updated stale auto-revalidate submit fixture to use monotonic submission
      operation IDs so dedupe replacement semantics match runtime invariants.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_successful_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/submission_staleness_state_machine_internal.test.ts`
- [x] Result: `4 files, 122 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 582 tests passed`.
- [x] Removed the navigation outcome command-wrapper passthrough in
      `runtime_navigation_outcome.ts` by executing
      `NavigationOutcomeExecutionPlan` directly, eliminating one plan->command
      indirection layer from runtime outcome handling.
- [x] Added focused runtime-outcome execution coverage in
      `navigation_outcome_runtime_internal.test.ts` and removed the obsolete
      command-builder internal test coverage.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `3 files, 117 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 581 tests passed`.
- [x] Split `core/links.ts` into focused lifecycle modules:
      `links_click_lifecycle.ts` and `links_prefetch_lifecycle.ts`, with shared
      target classification + shared callback option types extracted into
      `links_target_classification.ts` and `links_lifecycle_types.ts`.
- [x] Reduced `core/links.ts` to a thin facade that re-exports click and
      prefetch handlers and preserves existing internal testing seams
      (`__makeLinkOnClickFn`, `__getPrefetchHandlers`).
- [x] `pnpm vitest vormaclient/client/src/tests/unit/links_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.link_click.contract.test.ts vormaclient/client/src/tests/contracts/client.prefetch.contract.test.ts`
- [x] Result: `4 files, 145 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 581 tests passed`.
- [x] Split monolithic `core/render_runtime.ts` into focused runtime modules:
      `render_asset_runtime.ts`, `render_component_runtime.ts`,
      `render_client_loader_runtime.ts`, and `render_commit_runtime.ts`.
- [x] Reduced `core/render_runtime.ts` to a thin compatibility facade that
      re-exports the existing public/internal runtime APIs.
- [x] Preserved deterministic render commit and client-loader behavior by
      keeping existing reducer/command seams intact while relocating runtime
      ownership to focused files.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/render_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts vormaclient/client/src/tests/contracts/client.history_and_init.contract.test.ts`
- [x] Result: `4 files, 177 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 581 tests passed`.
- [x] Follow-up simplification wave started: collapsed successful-navigation
      plan layering by removing intermediate stage wrapper plans and moving to
      direct checkpoint payloads (`preWaitingExecutionPlan`,
      `postWaitingExecutionPlan`, `postAssetExecutionPlan`).
- [x] Simplified post-asset lifecycle composition to consume direct
      `postAssetExecutionPlan` instead of nested stage plan wrappers in
      `runtime_navigation_outcome_state_machine.ts`.
- [x] Removed redundant stage command seam in
      `runtime_navigation_successful_commands.ts` and rewired checkpoint command
      building to map directly from checkpoint plan payloads.
- [x] Updated focused state-machine and successful-command unit coverage to
      assert the flattened checkpoint plan shapes.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_successful_commands_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts`
- [x] Result: `3 files, 119 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 581 tests passed`.
- [x] Rewired fetch-rejection lane ownership checks in `navigation_controls.ts`
      (`active`, `prefetch`, `revalidation`) from object-identity guards to
      shared operation-ID ownership helper (`hasNavigationOperationOwnership`).
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts`
- [x] Result: `3 files, 123 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 583 tests passed`.
- [x] Completed ownership-seam DRY cleanup by switching
      `runtime_navigation_successful_runtime.ts` current-entry checks from
      object-identity comparison to shared operation-ID ownership helper
      (`hasNavigationOperationOwnership`).
- [x] `pnpm vitest vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts`
- [x] Result: `4 files, 142 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 583 tests passed`.
- [x] Extracted shared operation-ID ownership helper in `types.ts`
      (`hasNavigationOperationOwnership`) and rewired runtime outcome
      state-machine, runtime catch cleanup, and link-click ownership checks to
      use one seam.
- [x] Removed duplicated inline operation-ID ownership predicates in
      `runtime_navigation_outcome_state_machine.ts`, `runtime.ts`, and
      `links.ts`, reducing ownership drift risk across execution paths.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/links_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.link_click.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts`
- [x] Result: `6 files, 166 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 583 tests passed`.
- [x] Removed promise-reference ownership fallback from runtime navigation
      outcome state machine by requiring explicit `expectedOperationID` matching
      in `decideNavigationOutcomeExecutionPlan`.
- [x] Rewired
      `handleNavigationOutcome`/`handleNavigationOutcomeWithInternalResult` to
      require explicit `expectedOperationID` and removed legacy `controlPromise`
      ownership plumbing from runtime execution paths.
- [x] Removed promise-fallback ownership checks from link-click lifecycle in
      `links.ts`; link side effects now require operation-ID ownership via
      `doesNavigationEntryBelongToControl`.
- [x] Removed dead promise-ownership helper from `types.ts` now that runtime and
      link outcome paths are operation-ID-fenced.
- [x] Updated runtime/link/state-machine focused tests to assert strict
      operation-ID stale suppression semantics.
- [x] `pnpm vitest vormaclient/client/src/tests/unit/links_internal.test.ts vormaclient/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts vormaclient/client/src/tests/unit/navigation_runtime_internal.test.ts vormaclient/client/src/tests/contracts/client.link_click.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts`
- [x] Result: `6 files, 166 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts vormaclient/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
- [x] Result: `2 files, 30 tests passed`.
- [x] `pnpm vitest vormaclient/client/src/tests`
- [x] Result: `46 files, 583 tests passed`.

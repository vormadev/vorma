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
- [ ] Centralize transition logic into one reducer/transition seam.
- [ ] Remove scattered lifecycle branching in begin/outcome/render paths.
- [x] Ensure every transition has a reason/cause string for diagnostics.

### 2. Operation IDs and Commit Fences

- [x] Add monotonic operation IDs for navigation and submission entries.
- [ ] Gate side effects (history, global state, head, render, CSS/module
      preload) on current ownership.
- [ ] Ensure stale completions are no-ops by construction, not by best-effort
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
- [ ] Keep policy-level throttling/debounce at trigger layer (for example focus
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

- [ ] Phase A: State machine core + operation IDs + arbitration matrix.
- [ ] Phase B: Revalidation conflation + POP serialization.
- [ ] Phase C: Metadata seam unification + skip/server parity.
- [ ] Phase D: Debug journal + rich internal navigate result.
- [ ] Phase E: Test hardening and cleanup.

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

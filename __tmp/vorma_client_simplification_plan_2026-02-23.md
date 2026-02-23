# Vorma Client Simplification Plan (2026-02-23)

## Scope

Improve whole-runtime simplicity, dedupe, and aggregate client bundle efficiency
for `typescript/vorma/client/src` without changing framework behavior.

## Checklist

### 1) Remove dead redirect plumbing

- [x] Remove unused `isPrefetch` parameter threading from redirect fetch path.
- [x] Remove unused `newURL` value from redirect target parsing.
- [x] Keep redirect behavior parity for navigation/prefetch promotion paths.
- [x] Add/adjust unit tests for redirect path signatures and behavior.

### 2) Consolidate request-body serialization logic

- [x] Introduce one shared request-body normalization/serialization helper.
- [x] Reuse it from `app/helpers.ts` (`resolveVormaRequestBody`).
- [x] Reuse it from `core/redirects.ts` (`buildRedirectRequestInit`).
- [x] Preserve content-type and method/body semantics.
- [x] Add/adjust tests to verify parity across both callsites.

### 3) Eliminate duplicated CSS application responsibilities

- [x] Choose one authoritative stage for stylesheet application.
- [x] Keep prefetch behavior intentional (decide whether prefetch should apply
      CSS or only preload).
- [x] Remove duplicate `AssetManager.applyCSS` invocation path.
- [x] Codify prefetch commit boundary: pure prefetch may warm internal framework
      state (for example build ID and internal cache state), but must not mutate
      committed page state (stylesheet/title/head/history/route-change commit).
- [x] Add contract tests for navigation + prefetch CSS behavior after refactor.
- [x] Add contract tests that assert internal warm-up is allowed while
      page-committed mutations are blocked during pure prefetch.

### 4) Make debug journal production-optional

- [x] Gate debug journal runtime machinery behind a compile-time/dev guard.
- [x] Keep public debug APIs stable but no-op in production builds.
- [x] Ensure navigation lifecycle still works identically when disabled.
- [x] Add tests for dev-enabled and prod-disabled behavior.
- [x] Add artifact-level production-bundle test that asserts debug transition
      machinery is excluded when `import.meta.env.DEV` is `false`.

### 5) Flatten navigation/render orchestration layers

- [x] Collapse repetitive `execution plan -> command array -> executor` loops
      where they do not provide unique value.
- [x] Keep pure decision functions where they protect tricky invariants.
- [x] Reduce intermediate object creation in hot paths.
- [x] Preserve existing state-machine contract tests while reducing internal
      indirection.

### 6) Reduce reason-string runtime payload

- [x] Replace submit staleness runtime-generated template reasons with static
      checkpoint reason tables.
- [x] Consolidate submit lifecycle reason literals into shared constants.
- [x] Replace broad runtime string-reason unions with compact constants or
      dev-only mappings.
- [x] Keep debugging detail available in tests/dev builds.
- [x] Ensure runtime behavior does not depend on string literal text.

### 7) Simplify history integration surface

- [x] Extract minimal internal history adapter contract used by runtime.
- [x] Decouple runtime from direct third-party `history` API shape.
- [x] Evaluate replacing dependency usage with a smaller internal adapter where
      feasible.
- [x] Keep POP sequencing and hard-reload fallback behavior fully covered.
- [x] Record capability-parity guardrail: do not remove direct
      `getHistoryInstance()` access (or `npm:history` backing) unless all
      consumer-relevant history capabilities are exposed and covered by tests.

### 8) Shift matcher/route registration work toward build output

- [x] Define precompiled route/matcher payload shape.
- [x] Initialize runtime from precompiled data rather than dynamic progressive
      pattern registration where possible.
- [x] Preserve lazy/progressive enhancement behavior when manifest is absent.
- [x] Add tests for both precompiled and fallback runtime paths.

### 9) Simplify runtime lane bookkeeping hot paths

- [x] Remove lane-operation action-object indirection in `runtime_slots.ts`
      where direct lane mutation is sufficient.
- [x] Remove avoidable per-call allocations in navigation status computation.
- [x] Keep status-signal semantics unchanged under debounced dispatch.
- [x] Add focused unit tests for `computeNavigationStatus` and
      `createStatusSignaler`.

### 10) Flatten submit response orchestration

- [x] Remove submit response `decide action -> execute action` indirection in
      `runtime_submit.ts`.
- [x] Keep staleness checkpoint semantics unchanged across error, redirect, and
      success branches.
- [x] Preserve auto-revalidation and redirect effectuation behavior.
- [x] Validate with submit-focused unit/contract tests and full TS gate.

### 11) Harden deterministic revalidation lane invariants

- [x] Add focused unit tests for in-flight reuse before trailing eligibility.
- [x] Add focused unit tests for single trailing-pass coalescing behavior.
- [x] Add focused unit tests for target-mismatch replacement pass behavior.
- [x] Add focused unit tests for explicit trailing-queue cancellation behavior.
- [x] Keep existing contract-level revalidation tests passing.

### 12) Trim redundant runtime forwarding lambdas

- [x] Remove no-op forwarding closures in `runtime.ts` where function signatures
      already match.
- [x] Keep navigation lifecycle boundaries unchanged.
- [x] Validate with runtime-focused unit tests and full TS gate.

### 13) Harden server-route resolution branch invariants

- [x] Add focused unit tests for `buildRouteDataRequestURL` query parameter
      behavior.
- [x] Add focused unit tests for `resolveServerRouteDataResult` aborted,
      redirect, error, and success branches.
- [x] Keep preload command/state-machine unit suites green.
- [x] Validate with full TS gate.

### 14) Remove begin-navigation command translation indirection

- [x] Execute begin-navigation state-machine plans directly in
      `begin_navigation.ts` without a transient command layer.
- [x] Remove obsolete command-mapping-only unit tests.
- [x] Add focused unit tests for `decideBeginNavigationExecutionPlan` invariants
      across active/prefetch/revalidation flows.
- [x] Keep full runtime navigation suites green.
- [x] Validate with full TS gate.

### 15) Remove lifecycle action-reducer indirection

- [x] Remove `runtime_lifecycle_transitions.ts` generic action union and reducer
      switch layer used only as pass-through orchestration.
- [x] Call concrete lifecycle transition helpers directly from
      `runtime_lifecycle_runtime.ts`.
- [x] Add explicit `buildNavigationFailureLifecycleTransitionEvent` helper for
      symmetry and clearer direct dispatch wiring.
- [x] Update focused lifecycle transition tests to validate direct helper
      surface (including default `causedByOperationID` normalization).
- [x] Keep runtime lifecycle and contract suites green.
- [x] Validate with full TS gate.

### 16) Simplify history public type surface

- [x] Remove internal lowercase history type aliases that mirrored third-party
      types without adding semantics.
- [x] Use `BrowserHistory` directly for public `getHistoryInstance()` typing.
- [x] Keep history listener POP sequencing and init contract tests green.
- [x] Validate with full TS gate.

### 17) Simplify successful-navigation checkpoint execution wiring

- [x] Remove redundant `decide-and-execute` wrapper naming/typing layer in
      `runtime_navigation_successful_runtime.ts`.
- [x] Keep explicit checkpoint-plan decision and checkpoint-plan execution as
      the only two runtime seams.
- [x] Replace generic checkpoint input assertion helper with direct post-asset
      expected-build-id guard at the point of use.
- [x] Keep successful-navigation runtime and lifecycle contract suites green.
- [x] Validate with full TS gate.

### 18) Remove navigation-runtime proxy forwarding helper

- [x] Remove `withNavigationStateManager` closure indirection in `client.ts`.
- [x] Dispatch each `navigationStateManager` method directly to
      `getNavigationStateManager()` to reduce wrapper noise/allocation.
- [x] Keep lazy runtime initialization semantics unchanged.
- [x] Keep client initialization/history/lifecycle contract suites green.
- [x] Validate with full TS gate.

### 19) Remove lifecycle status-update getter indirection

- [x] Replace `createNavigationLifecycleRuntime`'s `getScheduleStatusUpdate`
      dependency with direct `scheduleStatusUpdate` function injection.
- [x] Initialize status signaling before lifecycle runtime creation in
      `runtime.ts` so the injected callback is stable and explicit.
- [x] Update lifecycle runtime unit tests for the direct dependency shape.
- [x] Keep navigation runtime + lifecycle contract suites green.
- [x] Validate with full TS gate.

### 20) Remove submission lifecycle command-array indirection

- [x] Remove submit lifecycle command types/builders/executor indirection in
      `runtime_submit.ts`.
- [x] Introduce direct `beginSubmissionLifecycle` and
      `finishSubmissionLifecycle` helpers that mutate state and emit transitions
      in place.
- [x] Keep dedupe abort semantics, submission state-transition emissions, and
      status-update scheduling behavior unchanged.
- [x] Replace command-shape-only unit tests with focused lifecycle execution
      side-effect tests.
- [x] Keep submit contract/runtime suites green.
- [x] Validate with full TS gate.

### 21) Collapse preload plan->command layering and trim payload

- [x] Remove `decideServerSuccessPreloadExecutionPlan` indirection and build
      preload commands directly from runtime inputs.
- [x] Remove constant preload `reason` payload fields from command objects
      (unused at runtime) to reduce allocations/string payload.
- [x] Keep dev deduping (`importURLs`) and production deps (`deps`) semantics
      unchanged.
- [x] Keep command ordering and empty/non-string filtering invariants unchanged.
- [x] Update preload-focused unit tests and runtime expectations accordingly.
- [x] Validate with full TS gate.

### 22) Remove bootstrap snapshot/adapter wrappers

- [x] Remove one-off global snapshot wrappers in `fetch_route_data_server.ts`
      for route-request and loader-map reads.
- [x] Read required globals directly at callsites while preserving defaults and
      semantics.
- [x] Remove one-field precompiled route-manifest payload wrapper in
      `app/init.ts` and use direct manifest read/init flow.
- [x] Keep init, fetch, and lifecycle contract suites green.
- [x] Validate with full TS gate.

### 23) Remove one-shot module-map builder wrapper

- [x] Remove `buildClientModuleMapFromRouteModuleMetadata` wrapper that only
      forwarded to `mergeClientModuleMapWithRouteModuleMetadata`.
- [x] Update `app/init.ts` and route-metadata unit tests to use the single merge
      helper directly with explicit `currentClientModuleMap` input.
- [x] Keep route metadata and init/runtime suites green.
- [x] Validate with full TS gate.

### 24) Share server-data resolution across parallel client loaders

- [x] Refactor `startParallelClientLoaders` to resolve server-result-to-loader
      server data once per navigation, then fan out per-pattern promises from
      shared resolved data.
- [x] Preserve unavailable-server-data behavior on missing pattern data and
      upstream server-promise failure.
- [x] Add focused unit tests for start-parallel loader happy-path data delivery
      and rejection-path abort-error mapping.
- [x] Keep fetch/runtime/lifecycle contract suites green.
- [x] Validate with full TS gate.

### 25) Remove no-state scroll-manager factory wrapper

- [x] Remove `createScrollStateManager` factory in `platform/scroll.ts` and
      expose `scrollStateManager` as a direct object over existing pure helper
      functions.
- [x] Preserve page-refresh restoration wiring and hash/coordinate apply
      behavior.
- [x] Keep scroll/history/nav lifecycle suites green.
- [x] Validate with full TS gate.

### 26) Simplify platform event dispatch helper surface

- [x] Remove split detail/no-detail dispatch helper naming in
      `platform/events.ts` and dispatch through one shared helper.
- [x] Keep all public event APIs and event-key contracts unchanged.
- [x] Add focused unit tests proving detail-payload and no-detail event dispatch
      behavior when a window event target is available.
- [x] Validate with full TS gate.

### 27) Collapse redirect effectuation command-array indirection

- [x] Remove redirect effectuation command types/builders/executor in
      `core/redirects.ts` where they only translated execution-plan outcomes.
- [x] Execute redirect effectuation plans directly while preserving
      stop-vs-cleanup boundaries and redirect strategy behavior.
- [x] Remove command-shape-only redirect unit coverage and keep behavior-focused
      redirect tests as the source of truth.
- [x] Add focused redirect behavior test proving cleanup happens before soft
      redirect navigation execution.
- [x] Validate with full TS gate.

### 28) Collapse render-commit command-array indirection

- [x] Remove render-commit command types/builders/executor in
      `core/render_commit_runtime.ts` where command translation was purely
      mechanical.
- [x] Execute the render commit pipeline directly in one explicit sequence while
      preserving history, scroll dispatch, CSS, head, and finish ordering.
- [x] Remove command-shape-only render commit unit coverage and replace with
      behavior assertions around CSS application/no-application paths.
- [x] Keep render runtime + loading/focus + head contract suites green.
- [x] Validate with full TS gate.

### 29) Remove render checkpoint reason/state-machine wrapper

- [x] Remove `RenderCommitCheckpointExecutionPlan` and
      `decideRenderCommitCheckpointExecutionPlan` from
      `core/render_commit_runtime.ts` because pre/post checkpoints shared
      identical boolean gating behavior.
- [x] Gate pre-module and post-module commit flow directly with
      `canCommitRender` while preserving double-check semantics.
- [x] Replace checkpoint-shape-only tests with behavior tests that prove
      pre-module and post-module `shouldCommit` rejection paths.
- [x] Keep render runtime + loading/focus + head contract suites green.
- [x] Validate with full TS gate.

### 30) Replace focus-revalidation plan objects with direct predicate

- [x] Remove `FocusRevalidationTriggerExecutionPlan` object union and
      `decideFocusRevalidationTriggerExecutionPlan` object-returning helper from
      `core/extras.ts`.
- [x] Add direct boolean `shouldTriggerFocusRevalidation` predicate and use it
      at `revalidateOnWindowFocus` callsite.
- [x] Preserve navigation/submission/revalidation/stale-window blocking
      semantics.
- [x] Update focus-trigger unit tests to assert behavior outcomes instead of
      plan-shape reason payloads.
- [x] Keep extras + loading/focus contract suites green.
- [x] Validate with full TS gate.

### 31) Replace submit-staleness plan objects with direct predicate

- [x] Remove `SubmitStalenessCheckpointExecutionPlan` and per-checkpoint reason
      tables from `core/navigation/runtime_submit.ts`.
- [x] Introduce direct boolean `shouldContinueSubmitStalenessCheckpoint` and use
      it in stale-submit checkpoints.
- [x] Preserve stale-submit abort semantics across request/finalize/classify/
      redirect/success-return/auto-revalidate checkpoints.
- [x] Update submission staleness unit coverage to assert behavior outcomes
      rather than plan-shape reason payloads.
- [x] Keep submit runtime + submit/redirect contract suites green.
- [x] Validate with full TS gate.

### 32) Remove dead submit checkpoint-label threading

- [x] Remove `SubmitStalenessCheckpoint` label type and checkpoint-literal
      threading from `core/navigation/runtime_submit.ts` now that stale checks
      are purely boolean current/not-current gates.
- [x] Collapse stale-check helper to direct `getStaleSubmitResultIfNotCurrent`
      predicate use at callsites.
- [x] Remove checkpoint-label-shape unit test that no longer validates runtime
      behavior.
- [x] Keep submit runtime + submit/redirect contract suites green.
- [x] Validate with full TS gate.

### 33) Collapse successful-navigation checkpoint executor wrapper

- [x] Remove the extra wrapper split between checkpoint-plan execution and
      checkpoint-plan selection in
      `core/navigation/runtime_navigation_successful_runtime.ts`.
- [x] Keep `decideSuccessfulNavigationLifecycleCheckpointExecutionPlan` as the
      decision seam, but execute each checkpoint in one function without
      forwarding indirection.
- [x] Preserve all pre/post waiting, asset sync, render/complete, and cleanup
      semantics.
- [x] Keep navigation runtime + lifecycle + history/init suites green.
- [x] Validate with full TS gate.

### 34) Replace preload command objects with compact preload plan

- [x] Replace `ServerSuccessPreloadCommand[]` with a compact
      `ServerSuccessPreloadPlan` shape (`moduleDependencies` + `cssBundles`) in
      `core/navigation/fetch_route_data_server.ts` and
      `core/navigation/types.ts`.
- [x] Update server-success outcome construction and client-only skip outcomes
      to carry preload plan data instead of tagged command objects.
- [x] Update successful-navigation asset warm-up runtime to iterate direct
      preload arrays rather than switch-dispatching command unions.
- [x] Update preload-focused and navigation/runtime unit fixtures/assertions to
      the new preload plan shape.
- [x] Keep preload/navigation/prefetch suites green.
- [x] Validate with full TS gate.

### 35) Remove successful-navigation checkpoint selector wrapper type layer

- [x] Remove `SuccessfulNavigationLifecycleCheckpoint*` selector wrapper types
      and `decideSuccessfulNavigationLifecycleCheckpointExecutionPlan` from
      `core/navigation/runtime_navigation_outcome_state_machine.ts`.
- [x] Execute successful-navigation checkpoints in
      `core/navigation/runtime_navigation_successful_runtime.ts` using direct
      specialized reducers (`pre_waiting`, `post_waiting`, `pre_asset_wait`,
      `post_asset`, `cleanup`) instead of one optional-input selector.
- [x] Tighten checkpoint executor input typing so `post_asset` requires
      `buildIDSyncTiming`, `expectedBuildID`, and `clientLoadersResult` at
      compile time.
- [x] Remove selector-wrapper-shape-only unit coverage and keep decision
      behavior tests around the direct reducers.
- [x] Keep navigation runtime + lifecycle + history/init suites green.
- [x] Validate with full TS gate.

### 36) Remove post-asset lifecycle composition wrapper

- [x] Remove `SuccessfulNavigationPostAssetLifecycleExecutionPlan` and
      `decideSuccessfulNavigationPostAssetLifecycleExecutionPlan` from
      `core/navigation/runtime_navigation_outcome_state_machine.ts`.
- [x] Compose post-asset execution and side-effect reducers directly in
      `core/navigation/runtime_navigation_successful_runtime.ts` to eliminate
      one intermediate allocation layer.
- [x] Remove combined-wrapper-shape-only unit coverage in
      `navigation_outcome_state_machine_internal.test.ts` and keep direct
      reducer coverage as the source of truth.
- [x] Keep navigation runtime + lifecycle + history/init suites green.
- [x] Validate with full TS gate.

### 37) Flatten redirect effectuation execution-plan indirection

- [x] Remove `RedirectEffectuationExecutionPlan`,
      `decideRedirectEffectuationExecutionPlan`, and
      `executeRedirectEffectuationExecutionPlan` from `core/redirects.ts`.
- [x] Execute redirect effectuation directly in `effectuateRedirectDataResult`
      while preserving: `status !== should` stop behavior, redirect-lane cleanup
      before effectuation, hard vs soft strategy behavior, and unknown-strategy
      cleanup+stop behavior.
- [x] Remove plan-shape-only unit coverage in
      `tests/unit/redirect_effectuation_state_machine_internal.test.ts` and keep
      behavior-focused redirect tests in `redirects_internal.test.ts` as source
      of truth.
- [x] Keep redirect + submit/redirect contract suites green.
- [x] Validate with full TS gate.

### 38) Inline navigation-outcome execution switch in runtime orchestrator

- [x] Remove `executeNavigationOutcomeExecutionPlan` helper from
      `core/navigation/runtime.ts` and execute
      `decideNavigationOutcomeExecutionPlan` results directly in
      `handleNavigationOutcomeWithInternalResult`.
- [x] Preserve stop/delete/redirect/success behavior, including redirect build
      ID sync, redirect-lane cleanup, redirect source props shaping, and
      committed/cancelled internal result mapping.
- [x] Remove now-unused `NavigationOutcomeExecutionPlan` runtime import in
      `core/navigation/runtime.ts`.
- [x] Keep navigation runtime + navigation state machine contract suites green.
- [x] Validate with full TS gate.

### 39) Remove redundant begin-navigation plan target payload

- [x] Remove unused `targetUrl` field from `BeginNavigationExecutionPlan` in
      `core/navigation/begin_navigation_state_machine.ts`.
- [x] Remove redundant `targetUrl` assignments from all begin-navigation plan
      branches while keeping promotion/create target shaping unchanged.
- [x] Update begin-navigation state-machine unit assertions to validate
      behavior/instructions without asserting redundant plan payload.
- [x] Keep begin-navigation + runtime navigation suites green.
- [x] Validate with full TS gate.

### 40) Inline begin-navigation create-instruction execution wrapper

- [x] Remove `executeBeginNavigationCreateInstruction` from
      `core/navigation/begin_navigation.ts`.
- [x] Execute create-instruction branches directly inside
      `executeBeginNavigationExecutionPlan` while preserving: status-update
      scheduling for prefetch creates and active/revalidation create semantics.
- [x] Remove now-unused `BeginNavigationCreateInstruction` type import in
      `begin_navigation.ts`.
- [x] Keep begin-navigation + runtime navigation suites green.
- [x] Validate with full TS gate.

### 41) Make begin-navigation plans unambiguous via discriminated union

- [x] Refactor `BeginNavigationExecutionPlan` in
      `core/navigation/begin_navigation_state_machine.ts` to discriminated
      variants (`reuse`, `create`, `immediateAbort`) so invalid nullable/flag
      combinations are unrepresentable.
- [x] Update begin-navigation state-machine plan builders to emit only the
      corresponding variant payload fields.
- [x] Simplify `executeBeginNavigationExecutionPlan` in
      `core/navigation/begin_navigation.ts` to switch on the discriminant and
      remove implicit nullable-state branching.
- [x] Update begin-navigation state-machine unit assertions to the new
      discriminated plan shape.
- [x] Keep begin-navigation + navigation state machine contract suites green.
- [x] Validate with full TS gate.

### 42) Remove constant active-create intent payload from begin plan

- [x] Remove `intent` from the `"active"` variant of
      `BeginNavigationCreateInstruction` in
      `core/navigation/begin_navigation_state_machine.ts` because it is always
      `"navigate"` for active-lane creates.
- [x] Update active-lane create plan construction to emit only the slot marker.
- [x] Update begin-navigation runtime execution to pass the literal `"navigate"`
      intent directly when executing active create instructions.
- [x] Keep begin-navigation + navigation state machine contract suites green.
- [x] Validate with full TS gate.

## Execution Order

1. Dead redirect plumbing + request-body dedupe.
2. CSS responsibility unification.
3. Debug journal gating.
4. Navigation/render layer flattening and reason-string compression.
5. History adapter simplification.
6. Matcher build-output shift.
7. Runtime lane + submit response flattening.
8. Deterministic revalidation lane hardening.
9. Runtime forwarding-lambda trim.
10. Server-route resolution branch hardening.
11. Begin-navigation command translation removal.
12. Lifecycle action-reducer indirection removal.
13. History public type-surface simplification.
14. Successful-navigation checkpoint wiring simplification.
15. Navigation-runtime proxy forwarding-helper removal.
16. Lifecycle status-update getter-indirection removal.
17. Submission lifecycle command-array indirection removal.
18. Preload plan/command layering collapse and payload trim.
19. Bootstrap snapshot/adapter wrapper removal.
20. One-shot module-map builder-wrapper removal.
21. Shared server-data resolution for parallel client loaders.
22. Scroll-manager factory-wrapper removal.
23. Platform event-dispatch helper-surface simplification.
24. Redirect effectuation command-array indirection collapse.
25. Render-commit command-array indirection collapse.
26. Render checkpoint reason/state-machine wrapper removal.
27. Focus-revalidation plan-object removal.
28. Submit-staleness plan-object removal.
29. Dead submit checkpoint-label threading removal.
30. Successful-navigation checkpoint executor wrapper collapse.
31. Preload command-object replacement with compact preload plan.
32. Successful-navigation checkpoint selector wrapper type-layer removal.
33. Post-asset lifecycle composition-wrapper removal.
34. Redirect effectuation execution-plan indirection flattening.
35. Navigation-outcome execution-switch inlining in runtime orchestrator.
36. Begin-navigation plan target-payload removal.
37. Begin-navigation create-instruction wrapper inlining.
38. Begin-navigation execution-plan discriminated-union hardening.
39. Begin-navigation active-create constant intent payload removal.

## Validation Gate (each step)

- [x] `pnpm prettier --write <edited files>`
- [x] `pnpm tsc -p typescript/vorma/client/tsconfig.json --noEmit`
- [x] Targeted vitest suites for changed modules.
- [x] `make tstest-source`

## Progress Log

- [x] Plan created.
- [x] Step 1 complete.
- [x] Step 2 complete.
- [x] Step 3 complete.
- [x] Step 4 complete.
- [x] Step 5 complete.
- [x] Step 6 complete.
- [x] Step 7 complete.
- [x] Step 8 complete.
- [x] Step 9 complete.
- [x] Step 10 complete.
- [x] Step 11 complete.
- [x] Step 12 complete.
- [x] Step 13 complete.
- [x] Step 14 complete.
- [x] Step 15 complete.
- [x] Step 16 complete.
- [x] Step 17 complete.
- [x] Step 18 complete.
- [x] Step 19 complete.
- [x] Step 20 complete.
- [x] Step 21 complete.
- [x] Step 22 complete.
- [x] Step 23 complete.
- [x] Step 24 complete.
- [x] Step 25 complete.
- [x] Step 26 complete.
- [x] Step 27 complete.
- [x] Step 28 complete.
- [x] Step 29 complete.
- [x] Step 30 complete.
- [x] Step 31 complete.
- [x] Step 32 complete.
- [x] Step 33 complete.
- [x] Step 34 complete.
- [x] Step 35 complete.
- [x] Step 36 complete.
- [x] Step 37 complete.
- [x] Step 38 complete.
- [x] Step 39 complete.
- [x] Step 40 complete.
- [x] Step 41 complete.
- [x] Step 42 complete.

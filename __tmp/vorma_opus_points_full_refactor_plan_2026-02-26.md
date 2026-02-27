# Vorma Opus-Points Full Refactor Plan (2026-02-26)

Purpose: one new, standalone checklist to fully close all seven Opus review
points with end-state architecture changes (not partial patches).

Scope: `typescript/vorma/*` runtime and shared type surface only (no kit feature
additions).

## Hard Constraints

- [x] Keep reducer/state-machine-first architecture.
- [x] Keep one authoritative reducer/orchestrator seam only; remove duplicate
      lifecycle authorities.
- [x] Keep route transition correctness as top priority (no stale commits).
- [x] Keep client-loader parallel start/reuse behavior.
- [x] Keep adapter parity (React/Preact/Solid same semantics).
- [x] Avoid bundle growth; require neutral-or-better runtime footprint after
      deletions.
- [x] No compatibility layers or dual old/new paths left behind.
- [x] Keep lane arbitration as a pure, synchronous decision function over lane
      snapshot + new intent.
- [x] Prefer straight-line async execution inside owned navigation/submission
      tasks once arbitration ownership is established.

## Execution Order

- [x] Phase 0: Baseline and safety rails.
- [x] Phase 1: Remove dual state-machine ownership.
- [x] Phase 2: Replace global mutable singletons with runtime context.
- [x] Phase 3: Simplify command execution infrastructure.
- [x] Phase 4: Simplify snapshot canonicalization pipeline.
- [x] Phase 5: Move dev-only paths out of production parse path.
- [x] Phase 6: Narrow reducer/plan type hierarchy.
- [x] Phase 7: Unify ownership guard pattern.
- [ ] Phase 8: Final deletion pass, footprint/perf checks, final gates.

## Phase 0: Baseline And Safety Rails

- [x] Record current runtime artifact sizes:
      `npm_dist/typescript/vorma/client/chunk-*.js`,
      `npm_dist/typescript/vorma/client/internal.js`, adapter entry JS files.
- [x] Record targeted correctness baselines: `make tscheck`,
      `make tstest-source`, `make tstest-dist`.
- [x] Freeze a single canonical checklist pointer for handoff: this file only
      for Opus-point closure tracking.

## Phase 1: Eliminate Dual Navigation State Machines

Goal: one authoritative navigation/submission state model.

- [x] Make `NavigationRuntimeEngineState` the sole authority for lane phase and
      ownership.
- [x] Reduce `RuntimeLanes` to execution handles only (abort controllers,
      in-flight promises, transport handles), not duplicated phase/ownership.
- [x] Remove any duplicated lane state mutation paths that bypass reducer
      transitions.
- [x] Route every navigation/submission/revalidation lifecycle mutation through
      reducer events + command execution only.
- [x] Delete legacy runtime-lane state seams made redundant by authoritative
      engine state.
- [x] Add invariant checks in runtime composition layer that forbid divergent
      lane state writes.
- [x] Stop re-deriving ownership in multiple checkpoint layers; resolve
      ownership once at orchestration boundary and pass owned handle through.
- [x] Delete legacy navigation lifecycle journal/runtime transition-event seam
      from runtime composition.

### Phase 1 tests

- [x] Add/expand unit tests asserting no direct lane-phase mutation outside the
      reducer path.
- [ ] Add transition tests where every event tick checks both public status and
      internal lane ownership coherence.
- [ ] Add tests that intentionally trigger superseded/stale events and prove
      side effects do not commit.

## Phase 2: Replace Global Mutable Singletons With Runtime Context

Goal: remove hidden process-global mutation and make runtime instance explicit.

- [x] Introduce `VormaRuntimeContext` containing: navigation runtime state
      access, history adapter, global snapshot store, pattern registry, event
      dispatchers, runtime options.
- [x] Add `createVormaRuntimeContext(...)` as the canonical constructor.
- [x] Convert module-scoped singletons (`historyState`, `navigationStateAccess`,
      `navigationStateManagerSingleton`, `__vormaClientGlobal`) into
      context-owned state.
- [x] Keep public API ergonomics by exposing a default singleton context wrapper
      that delegates to explicit context APIs.
- [ ] Make tests instantiate isolated contexts instead of mutating shared
      globals.
- [ ] Remove test-only global reset hacks that become unnecessary with context
      isolation.

### Phase 2 tests

- [ ] Add tests proving two contexts in one process do not share mutable state.
- [ ] Add tests proving context teardown/recreate does not leak prior route
      snapshot or lane ownership state.

## Phase 3: Simplify Command Execution Infrastructure

Goal: keep typed commands, remove over-generic executor indirection.

- [x] Replace `executeSynchronousRuntimeCommandPlanWithTerminalResult` and async
      equivalent with simpler command-list executors.
- [x] Keep command unions typed, but remove unnecessary terminal-result generic
      machinery that requires defensive runtime throws.
- [x] Flatten each callsite to local straightforward command execution loops
      where that improves clarity and debug-ability.
- [x] Delete dead helper layers after migration.
- [x] Remove plan objects that are built and consumed in the same frame when no
      external consumer observes/intercepts that plan.
- [x] Keep explicit structured events only where they are externally consumed
      (debug journal, instrumentation hooks, tests).

### Phase 3 tests

- [ ] Update/expand command-order tests for navigation outcome, successful
      navigation checkpoints, submission checkpoints, and revalidation flow.
- [ ] Add tests asserting no command path can complete without producing its
      expected terminal behavior.

## Phase 4: Simplify Snapshot Canonicalization Pipeline

Goal: preserve correctness guarantees with less machinery.

- [x] Audit current pipeline (`normalize -> canonicalize -> freeze -> weak-set`)
      and define minimal required invariants.
- [x] Keep immutability guarantees where they enforce correctness.
- [x] Remove redundant canonicalization layers that duplicate downstream
      identity checks.
- [x] Remove weak-set tracking if not required for correctness.
- [x] Consolidate snapshot identity stabilization in one place only.
- [x] Ensure route-outlet/store sync uses canonical snapshot semantics directly,
      with no duplicate stabilization passes.

### Phase 4 tests

- [ ] Add tests proving unchanged snapshot segments preserve identity where
      required.
- [ ] Add tests proving changed segments update atomically without mixed old/new
      reads.
- [ ] Add tests proving adapters still avoid unnecessary remounts when route
      identity is unchanged.

## Phase 5: Gate Dev-Only Code Behind Dynamic Load Boundary

Goal: reduce production parse/runtime overhead.

- [x] Move debug journal implementation out of core production runtime path.
- [x] Move HMR-only wiring out of production runtime path.
- [x] Introduce explicit dev-only module boundary loaded only in dev mode.
- [x] Remove scattered `import.meta.env.DEV` branches where module-level
      separation is cleaner.
- [x] Confirm production bundle excludes dev-only implementation chunks from the
      hot path.

### Phase 5 tests

- [ ] Dist tests prove production build behavior unchanged.
- [ ] Source tests prove dev-only instrumentation still works in dev mode.

## Phase 6: Narrow Reducer Event/Plan Type Hierarchy

Goal: reduce type-layer bloat and cognitive overhead.

- [x] Inventory single-use intermediate reducer/plan/event/transition types.
- [x] Delete single-call-site type aliases that do not cross domain boundaries.
- [x] Keep only boundary-crossing shared types.
- [x] Inline locally-scoped types at use sites when clearer.
- [x] Collapse duplicate discriminated unions with identical semantics but
      different names.
- [x] Delete single-use terminal-result unions that only protect internal
      immediate execution paths.

### Phase 6 tests

- [ ] Ensure `make tscheck` remains clean after type-surface reduction.
- [ ] Ensure unit tests for reducers/command planning still provide branch
      coverage for all remaining unions.

## Phase 7: Unify Ownership Guard Pattern

Goal: one canonical check-then-act ownership abstraction.

- [x] Introduce shared ownership guard API (for navigation and submission) that
      owns: expected-vs-actual operation resolution, stale/current
      classification, guarded execution callback.
- [x] Replace scattered ad-hoc ownership checks with this shared guard path.
- [x] Remove duplicated helper variants (`resolveOwned*`, `isStale*`, per-domain
      hand-written checks) where superseded.
- [x] Enforce command execution boundary always passes through ownership guard
      before mutating side effects.

### Phase 7 tests

- [ ] Add stale-ownership tests across navigation, revalidation, and submission
      flows asserting no side effects commit after ownership loss.
- [ ] Add tests that all command executors share the same ownership guard
      semantics.

## Phase 8: Final Deletion, Footprint, And Verification

- [x] Delete all dead exports/helpers left after Phases 1-7.
- [ ] Delete tests that only encoded removed legacy seams; replace with
      first-principles behavior tests where needed.
- [ ] Re-run build and record post-refactor artifact sizes.
- [ ] Require neutral-or-smaller runtime JS footprint versus Phase 0 baseline,
      or explicitly document exception before merge.
- [x] Re-run targeted gates: `make tscheck`, `make tstest-source`,
      `make tstest-dist`.
- [ ] Run final full gate only after all checklist items above are complete.

## Primary Files Expected To Change

- [x] `typescript/vorma/client/src/runtime.ts`
- [x] `typescript/vorma/client/types.ts`
- [x] `typescript/vorma/client/internal.ts`
- [x] `typescript/vorma/client/index.ts`
- [x] `typescript/vorma/ui-adapters/react/index.tsx`
- [x] `typescript/vorma/ui-adapters/preact/index.tsx`
- [x] `typescript/vorma/ui-adapters/solid/index.tsx`
- [x] `typescript/vorma/client/src/tests/unit/**`
- [x] `typescript/vorma/client/src/tests/dist/**`
- [x] `__tmp/vorma_opus_points_full_refactor_plan_2026-02-26.md` (this file)

## Done Definition

- [ ] All checklist items in this file are complete.
- [x] No dual-state tracking remains for lane phase/ownership.
- [ ] Runtime context isolation is explicit and test-proven.
- [ ] Command execution seams are simpler and fully covered by tests.
- [x] Snapshot pipeline complexity is reduced without weakening correctness.
- [x] Dev-only code is outside production hot parse path.
- [x] Type hierarchy is materially narrower with no loss of behavior guarantees.
- [x] Ownership guard behavior is centralized and uniform.
- [ ] Targeted TS gates pass; final full gate passes at the end.

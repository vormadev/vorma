# Vormabuild Refactoring Checklist

Last updated: 2026-02-16

## Purpose

This checklist tracks the `vormabuild` refactor toward deterministic lifecycle
behavior, stronger failure handling, and easier debugging.

Keep this file current so work can be handed off without losing context.

## Core Goals

- Make build/rebuild lifecycle transitions explicit and deterministic.
- Reduce lock scope and prevent mixed runtime states.
- Make artifact generation fail fast on invalid/incomplete inputs.
- Improve observability of build decisions and failure paths.
- Keep behavior verifiable with focused tests.

## Non-Negotiable Constraints

- No silent partial-success paths for artifact generation.
- Runtime state commits must be atomic from reader perspective.
- Side-effect-heavy phases must not run under long-held global runtime locks.
- New seams must be instance-scoped, not package-global mutable state.
- Every completed item includes tests that validate intended semantics.

## Workstreams

## 1) Explicit Lifecycle State Machine

- [x] Define a shared lifecycle model for full build, fast rebuild, and watch
      callbacks.
- [x] Implement explicit transition points (start, plan, stage, commit, reload,
      complete/fail).
- [x] Ensure failures always report terminal state with rollback outcome.
- [x] Add lifecycle unit tests that assert transitions, not incidental call
      order.
- Note: watch callback lifecycle transitions still need to be wired to this
  state machine.

## 2) Plan/Stage/Commit Pipeline

- [ ] Split flows into pure planning, artifact staging, and runtime commit
      phases.
- [ ] Move filesystem/codegen-heavy steps out of lock-held sections.
- [ ] Keep lock-held commit phase short and bounded.
- [ ] Add tests proving no long-running IO executes during commit locks.
- Note: route-sync post-sync hooks now execute outside the initial route-sync
  write lock, and this is covered by regression tests.

## 3) Atomic Runtime State Commit API

- [ ] Add one runtime commit path that updates `isDev`, `buildID`, `paths`, and
      route manifest together.
- [ ] Remove/replace multi-step runtime mutation sequences in build/rebuild
      paths.
- [ ] Add version/snapshot guarding or CAS-style protection for concurrent dev
      events.
- [ ] Add race-focused tests for concurrent watch/build events.
- Note: dev/prod build-inner initialization and runtime-state restore now apply
  `isDev` under lock with related state updates.

## 4) Artifact Correctness and Strict Validation

- [x] Validate stage-two manifest reconciliation: client entry must resolve,
      route outputs must be complete.
- [x] Fail build when expected chunks/routes are missing instead of producing
      partial artifacts.
- [x] Add configurable unresolved-route policy with strict production default.
- [x] Add negative tests for missing manifest entries and unresolved route
      modules.

## 5) Deterministic Build ID Inputs

- [x] Replace path+size FS summary hashing with content-aware deterministic
      hashing.
- [x] Keep hashing order deterministic across filesystems.
- [x] Confirm build IDs change on content-only changes with same file size.
- [x] Update and expand tests around stage-two build ID stability and
      sensitivity.

## 6) Reload Protocol Semantics

- [x] Switch dev reload endpoint calls to mutation-safe HTTP semantics.
- [ ] Include structured request/response payload fields for debugging
      (attempt/build identifiers).
- [ ] Keep fallback behavior deterministic when reload endpoint fails.
- [ ] Add integration tests for success/fallback paths.

## 7) Atomic File Write Durability

- [x] Add parent-directory sync after atomic rename where supported.
- [x] Keep error paths explicit when durability steps fail.
- [x] Add tests that cover the new durability step behavior and failures.

## 8) Dependency Seams and Executor Structure

- [ ] Refactor package-global mutable dependency vars into instance-scoped
      executors.
- [ ] Make production wiring explicit and test wiring local to each test.
- [ ] Remove cross-test/global seam coupling risks.
- [ ] Add targeted tests for executor wiring and behavior parity.

## 9) Overlay Cache Lifecycle

- [ ] Replace pointer-keyed global overlay cache with lifecycle-owned bounded
      cache.
- [ ] Define cache invalidation/eviction rules.
- [x] Ensure cache behavior is deterministic across repeated app instances.
- [ ] Add tests for cache hit/miss/invalidation cases.

## 10) Debuggability and Diagnostics

- [ ] Add structured per-attempt lifecycle traces (phases, inputs, decisions,
      rollbacks).
- [ ] Emit clear reasons for skip/fallback/error outcomes.
- [ ] Provide one command/entrypoint to print discovery/overlay/build
      diagnostics.
- [ ] Add tests that verify diagnostic output contains required fields.

## 11) Cleanup and Documentation

- [ ] Remove dead transitional helpers after each workstream lands.
- [ ] Keep this checklist updated as items move.
- [ ] Add/update package docs for new lifecycle model and invariants.
- [ ] Final verification pass: run full `vormabuild` tests and confirm no
      behavior regressions.

## Handoff Notes

- Update checkboxes in this file in the same change where work is done.
- When pausing with partial work, add a short note in the relevant workstream.
- Do not mark an item complete if tests for that behavior are missing.

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

## Active Tracks

- Track A: continue extending plan/stage/commit lock minimization beyond route
  artifacts into remaining build phases.
- Track B: continue rolling out the shared runtime-state commit API to replace
  ad hoc multi-step runtime mutation sequences.

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
- Note: route artifact writes now use a captured route-build runtime snapshot
  and perform stage-one JSON + generated TypeScript writes outside runtime write
  locks; only manifest-state capture and manifest-file commit are lock-scoped,
  and regression tests assert both lock-boundary behavior and snapshot
  consistency across artifact steps.
- Note: non-lock route artifact writes now include staleness guards (build-ID
  snapshot checks) before/after heavy artifact steps, skip stale rollback
  cleanup, and gate manifest-file commit on expected/current build-ID match.

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
- Note: `buildRuntimeStateSnapshot` now captures/restores `isDev` + route-build
  state through one helper path, and route-sync rollback now uses a build-ID
  guard to avoid stale rollback overwriting a newer concurrent commit; stale
  rollback behavior is covered by regression tests.
- Note: a shared `runtimeStateCommitInput` commit path now backs snapshot
  restore and core mutation callsites (build init mode/ID commit, route-sync
  route+build-ID commit, dev-runtime mode commit, manifest-file commit), with
  focused regression tests covering full-field commits and dev-reload route-sync
  semantics.
- Note: stage-two build-ID application now also routes through the shared
  runtime-state commit path.

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
- [x] Include structured request/response payload fields for debugging
      (attempt/build identifiers).
- [x] Keep fallback behavior deterministic when reload endpoint fails.
- [x] Add integration tests for success/fallback paths.

## 7) Atomic File Write Durability

- [x] Add parent-directory sync after atomic rename where supported.
- [x] Keep error paths explicit when durability steps fail.
- [x] Add tests that cover the new durability step behavior and failures.

## 8) Dependency Seams and Executor Structure

- [x] Refactor package-global mutable dependency vars into instance-scoped
      executors.
- [x] Make production wiring explicit and test wiring local to each test.
- [x] Remove cross-test/global seam coupling risks.
- [x] Add targeted tests for executor wiring and behavior parity.
- Note: reload endpoint/action paths and build diagnostics now run through
  instance executors with local dependency wiring in tests, and framework build
  hook wiring now uses an executor injected at config wiring time; lifecycle
  state-machine attempt/timestamp dependencies are now instance-scoped as well;
  route parsing pipeline/code/module/file seams now run through a route parsing
  executor with local test wiring; static-public cleanup and stage-one/stage-two
  paths writers now run through instance executors (with post-vite write seam
  wiring explicit); stage-two build ID hashing and route-registry artifact
  writing/rollback seams now also run through instance executors with local test
  wiring; fast-rebuild build-ID generation is now instance-scoped; backend route
  discovery and discovered-registrar overlay generation now also run through
  instance executors with local test wiring; runtime build tooling/core
  orchestration and CLI entrypoint dispatch now also use instance-scoped
  executors with local dependency wiring in tests; generated TS
  assembly/write/write-file seams are now also executor-scoped; atomic file
  write operations now also use an executor-scoped dependency seam with local
  test wiring; build-inner build-ID generation and public-file-map writer seams
  now also use instance-scoped executors; build-inner route-sync parsing/merge
  execution seams are now also executor-scoped; fast-rebuild route-rebuild and
  route-artifact seams now also use executor-scoped dependency wiring with local
  test injection; build-inner orchestration now also uses a scoped executor with
  local dependency injection in tests, and there are no remaining package-global
  mutable `*Deps` seams in `vormabuild`.

## 9) Overlay Cache Lifecycle

- [x] Replace pointer-keyed global overlay cache with lifecycle-owned bounded
      cache.
- [x] Define cache invalidation/eviction rules.
- [x] Ensure cache behavior is deterministic across repeated app instances.
- [x] Add tests for cache hit/miss/invalidation cases.
- Note: framework build hook and go-build overlay preparation now capture a
  lifecycle-owned bounded cache instance per configured runtime.

## 10) Debuggability and Diagnostics

- [x] Add structured per-attempt lifecycle traces (phases, inputs, decisions,
      rollbacks).
- [x] Emit clear reasons for skip/fallback/error outcomes.
- [x] Provide one command/entrypoint to print discovery/overlay/build
      diagnostics.
- [x] Add tests that verify diagnostic output contains required fields.
- Note: lifecycle traces now include attempt IDs, normalized inputs, transition
  history, and recorded rollback decisions; watch reload callbacks now emit
  explicit skip/pre-reload-failure reasons.

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

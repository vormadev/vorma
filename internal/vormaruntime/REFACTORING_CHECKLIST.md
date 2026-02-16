# vormaruntime Refactoring Checklist

This checklist tracks the runtime design refactor for lifecycle determinism,
debuggability, and safer seams.

## Goals

- Ensure each request is served from one coherent runtime generation.
- Make init/reload lifecycle transitions explicit and atomic.
- Centralize runtime invariants and validation.
- Improve seam quality so core behavior is testable without HTTP plumbing.
- Keep this file current so work can be safely handed off.

## Scope

- In scope: `internal/vormaruntime/*.go` and related tests.
- Out of scope: unrelated package cleanup.

## Workstreams

### 1) Immutable Runtime Snapshot

- [x] Introduce a `RuntimeSnapshot` model containing all request-serving fields
      (build/mode, route artifacts, template, manifest, deps/css map, head
      rules).
- [x] Capture one snapshot per request and thread it through JSON/HTML/SSR
      paths.
- [x] Remove request-path reads of mutable runtime fields after stage-1 route
      resolution.
- [x] Add/adjust tests proving no mixed-generation artifacts in one response.

### 2) Explicit Lifecycle State Machine

- [x] Define explicit lifecycle states for init/reload (for example:
      uninitialized, ready, reloading, reload-failed-with-previous-active).
- [x] Implement two-phase reload: parse/validate candidate first, then commit
      atomically.
- [x] Ensure failed reload does not partially mutate active runtime state.
- [x] Route both init and reload through the same commit path where possible.

### 3) Canonical Artifacts + Validation

- [x] Create one internal artifacts model used by both stage-one and stage-two
      inputs.
- [x] Consolidate semantic validation into one place (not only structural JSON
      checks).
- [x] Use this same parser/validator for both `Init` and dev reload paths.

### 4) Route-Data Cache Redesign

- [x] Replace global cache behavior with runtime-owned or generation-scoped
      cache ownership.
- [x] Key cache entries with explicit generation/snapshot identity.
- [x] Replace global scan invalidation with generation swap/drop semantics.
- [x] Preserve cross-app isolation guarantees.

### 5) Route Execution Planner Seam

- [x] Extract a pure route execution planner that returns typed decisions (not
      found, stale-build-reload-hint, terminal proxy, render-json, render-html).
- [x] Keep HTTP side effects in a thin adapter layer.
- [x] Add focused planner tests that do not require full handler wiring.

### 6) Mutation API Consolidation

- [x] Consolidate mutable runtime updates behind commit/apply APIs rather than
      scattered field setters.
- [x] Guard or remove setter paths that can violate runtime invariants.
- [x] Align all runtime field access patterns with lock/snapshot rules.

### 7) Observability and Debuggability

- [x] Add structured lifecycle transition logs with generation identifiers.
- [x] Include generation/build context in relevant debug paths.
- [x] Keep production-safe defaults; expose extra debug detail only where
      appropriate.

## Recommended Implementation Order

- [x] Land `RuntimeSnapshot` type and request read-path adoption.
- [x] Land lifecycle state machine + two-phase commit.
- [x] Land canonical artifact parser/validator wiring.
- [x] Land cache redesign.
- [x] Land route planner extraction.
- [x] Land setter cleanup and final dead-code removal.

## Validation Gates (Run After Meaningful Changes)

- [x] `go test ./internal/vormaruntime -count=1`
- [x] `go test -race ./internal/vormaruntime -count=1`
- [x] Confirm concurrency/reload tests still enforce coherent artifact sets.
- [x] Confirm stale-build JSON handshake behavior remains correct.

## Handoff Notes

### Current Focus

- [ ] No remaining in-scope workstream items; run broader integration coverage
      outside `internal/vormaruntime` to confirm no cross-package regressions.

### Open Questions

- [ ] Keep unresolved design decisions listed here with short owner/action
      notes.

### Completed Milestones

- [x] 2026-02-16: Added regression tests for stale-build cache headers, semantic
      artifact validation failures, and mid-request HTML/prod script generation
      coherence during reload.
- [x] 2026-02-16: Implemented request-scoped render snapshots for build/manifest
      and client-entry script coherence in HTML/SSR generation.
- [x] 2026-02-16: Added shared semantic artifact validation and wired it through
      both init and dev-reload stage-file loading.
- [x] 2026-02-16: Added lifecycle state transitions, two-phase dev reload commit
      flow, and shared route-artifact commit path for init + reload.
- [x] 2026-02-16: Replaced global route-data cache invalidation scans with
      per-app generation-scoped cache map swaps and preserved isolation tests.
- [x] 2026-02-16: Introduced a first-class `RuntimeSnapshot` model and wired
      route-data execution to consume request-scoped snapshots.
- [x] 2026-02-16: Extracted a pure stage-one route planning function from task
      results (`planRouteResultFromTaskResults`) and narrowed HTTP side effects
      to adapter layers.
- [x] 2026-02-16: Added snapshot-version-aware route-data cache keys and
      completed focused planner unit tests around terminal-proxy short-circuit,
      dependency truncation, and successful stage-one planning.
- [x] 2026-02-16: Added a canonical `runtimeRouteArtifacts` model and wired both
      init and dev-reload artifact commits through it before applying runtime
      state.
- [x] 2026-02-16: Added generation-aware lifecycle transition logging and
      generation/build debug context in stale-build and no-match route planning
      paths.
- [x] 2026-02-16: Removed invariants-breaking lock-scoped metadata setters,
      routed mode changes through cache-invalidating mutation helpers, and
      routed root-template writes through a shared commit helper.

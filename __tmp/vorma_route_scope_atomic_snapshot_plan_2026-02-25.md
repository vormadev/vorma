# Vorma Global Correctness Refactor Plan (2026-02-25)

Purpose: one handoff-safe master checklist for all agreed deep refactors,
implemented to final end-state with no compatibility layering.

Scope:

1. RouteScope + atomic snapshot model
2. Unified pure navigation/submission engine + command executor
3. Strict wire-boundary decoder/invariant firewall
4. Unified adapter host runtime (React/Preact/Solid shared core)

## Non-Negotiables

- [ ] Keep `matchedPatterns` as canonical route identity on wire/runtime.
- [ ] Keep canonical-pattern uniqueness as strict invariant.
- [ ] No compatibility layers; ship direct end-state behavior.
- [ ] Fail loud on invariant violations; no fallback behavior.
- [ ] Keep progressive manifest optional perf-only.
- [ ] Preserve client-loader parallel start and reuse traits.
- [ ] Keep shared-layer logic authoritative (adapter thin wrappers only).

## Dependency Order

- [ ] Phase A (wire decoder firewall) first.
- [ ] Phase B (RouteScope + atomic snapshot) second.
- [ ] Phase C (pure navigation/submission reducer + commands) third.
- [ ] Phase D (shared adapter host extraction) fourth.
- [ ] Phase E (deletion/cleanup/perf gate/final verification) fifth.

## Phase A: Wire-Boundary Decoder / Invariant Firewall

Goal: decode and validate route JSON once at ingress, then run only on canonical
internal payloads.

### A1: Canonical decoded payload model

- [ ] Introduce canonical decoded payload type in client runtime boundary.
- [ ] Include all fields used by commit/render/client-loader paths.
- [ ] Mark optional/required fields explicitly in one place.
- [ ] Ensure decoded payload is immutable after decode.

### A2: Decode function and hard invariants

- [ ] Add one decode function at route-data fetch boundary.
- [ ] Validate array-length alignment with `matchedPatterns`: `loadersData`,
      `clientLoadersData` (if present), `importURLs`, `exportKeys`,
      `errorExportKeys`.
- [ ] Validate pattern entries are non-empty strings.
- [ ] Validate `outermost*ErrorIdx` are in-range when present.
- [ ] Validate root-data/loader-index assumptions (`hasRootData` invariants).
- [ ] Throw hard errors with deterministic messages on violation.

### A3: Replace downstream shape assumptions

- [ ] Replace all direct raw JSON reads after fetch with decoded payload reads.
- [ ] Remove duplicated downstream shape guards made redundant by decoder.
- [ ] Ensure commit/render code never receives unvalidated payload.

### A4: Files to update (primary)

- [ ] `typescript/vorma/client/src/core/navigation/fetch_route_data_server.ts`
- [ ] `typescript/vorma/client/src/core/navigation/types.ts`
- [ ] `typescript/vorma/client/src/core/render_commit_runtime.ts`
- [ ] `typescript/vorma/client/src/core/render_client_loader_runtime.ts`

### A5: Phase-A tests

- [ ] Add decode-pass tests for valid payloads.
- [ ] Add decode-fail tests for every invariant violation class.
- [ ] Add tests ensuring invalid payload never reaches commit/render.

## Phase B: RouteScope + Atomic Snapshot Ownership Model

Goal: ownership-safe selectors by construction, no token lifecycle.

### B1: Immutable snapshot contract

- [ ] Define immutable `RenderSnapshot` as sole selector source.
- [ ] Keep all route/render identity and data fields in snapshot.
- [ ] Guarantee one atomic snapshot replacement per successful commit.
- [ ] Enforce commit ordering: snapshot first, events second.
- [ ] Add structural sharing for unchanged segments.

### B2: RouteScope contract

- [ ] Define immutable `RouteScope` per matched route instance.
- [ ] Include route-owned loader/client-loader slices and route-bound metadata.
- [ ] Bind route scope at component mount site.
- [ ] Ensure scope identity remount behavior is deterministic by route key.

### B3: Selector rewiring

- [ ] Route-props selectors read only from bound scope.
- [ ] Global selectors read only from current snapshot.
- [ ] Remove any prior/current-window resolution logic from selector paths.

### B4: Delete token machinery (mandatory)

- [ ] Delete token create/activate/dispose/sync helpers.
- [ ] Delete token maps/sets and token-bearing route-props helpers.
- [ ] Delete token contract branches and token-specific errors.
- [ ] Delete removed-token internal exports.

### B5: Files to update (primary)

- [ ] `typescript/vorma/client/src/app/context.ts`
- [ ] `typescript/vorma/client/src/ui/typed_adapter_helpers_runtime.ts`
- [ ] `typescript/vorma/client/src/ui/route_outlet_runtime.ts`
- [ ] `typescript/vorma/ui-adapters/react/src/react.tsx`
- [ ] `typescript/vorma/ui-adapters/preact/src/preact.tsx`
- [ ] `typescript/vorma/ui-adapters/solid/src/solid.tsx`
- [ ] `typescript/vorma/ui-adapters/*/src/helpers.ts`

### B6: Phase-B tests

- [ ] Per-tick route-props transition tests (React/Preact/Solid).
- [ ] Per-tick global selector coherence tests.
- [ ] Aborted-render tests proving no ownership leak states.
- [ ] Tests proving stale snapshot reads are unrepresentable after commit.

## Phase C: Unified Pure Navigation/Submission Engine + Command Executor

Goal: one reducer decides behavior for all lanes; side effects run only as
commands.

### C1: Unified state and transition model

- [ ] Define one reducer state for lanes: `navigate`, `prefetch`, `revalidate`,
      `submit`.
- [ ] Define explicit phase/ownership fields in reducer state.
- [ ] Define strict transition table for stale/superseded outcomes.

### C2: Unified event input model

- [ ] Encode all external stimuli as reducer events: start, fetch-resolve,
      wait-resolve, redirect, abort, render-finish, timeout,
      external-location-change.
- [ ] Remove direct mutation side paths outside reducer.

### C3: Command output model

- [ ] Reducer outputs typed command list (no side effects inside reducer).
- [ ] Command types include: fetch, start-client-loaders, preload-assets,
      commit-snapshot, history-write, redirect, sync-build-id, emit-events,
      cleanup-entry, log-error.
- [ ] Define command ordering rules explicitly.

### C4: Command executor

- [ ] Implement thin executor that runs commands in reducer order.
- [ ] Guarantee idempotent ownership guard at command execution boundary.
- [ ] Ensure command executor cannot perform side effects not present in plan.

### C5: Preserve parallelism traits

- [ ] Keep speculative client-loader starts before final commit.
- [ ] Keep running-loader map reuse across phases.
- [ ] Keep `Promise.allSettled` fanout semantics for client loaders.
- [ ] Ensure reducer/executor split does not serialize loader paths.

### C6: De-duplicate existing navigation seams

- [ ] Remove scattered stale/ownership checks duplicated across files.
- [ ] Collapse submit race handling into reducer transitions.
- [ ] Collapse build-ID/redirect stale guard logic into reducer transitions.

### C7: Files to update (primary)

- [ ] `typescript/vorma/client/src/core/navigation/runtime_navigation_outcome_state_machine.ts`
- [ ] `typescript/vorma/client/src/core/navigation/runtime_navigation_successful_runtime.ts`
- [ ] `typescript/vorma/client/src/core/navigation/runtime_navigation_outcome_runtime.ts`
- [ ] `typescript/vorma/client/src/core/navigation/runtime_navigation_pass_runtime.ts`
- [ ] `typescript/vorma/client/src/core/navigation/runtime_navigation_runtime.ts`
- [ ] `typescript/vorma/client/src/core/navigation/types.ts`

### C8: Phase-C tests

- [ ] Reducer transition tests for all lanes and race classes.
- [ ] Command-plan tests verifying exact side-effect sequence.
- [ ] Tests proving no side effect occurs without command.
- [ ] Tests proving stale completions cannot commit side effects.
- [ ] Tests proving parallel client-loader behavior unchanged.

## Phase D: Unified Adapter Host Runtime (Shared Core)

Goal: shared host owns behavior; framework packages are thin adapters.

### D1: Shared host API

- [ ] Define framework-agnostic host contract for: snapshot subscription,
      route-scope binding, remount keying, outlet branch resolution, error
      boundary selection.
- [ ] Define adapter bridge hooks for framework lifecycle entry points only.

### D2: Host extraction

- [ ] Move shared outlet/store synchronization logic into one host module.
- [ ] Move shared remount/branch/scope binding logic into host module.
- [ ] Remove duplicated logic from React/Preact/Solid implementations.

### D3: Adapter thin wrappers

- [ ] React wrapper: host->React lifecycle bindings only.
- [ ] Preact wrapper: host->Preact lifecycle bindings only.
- [ ] Solid wrapper: host->Solid lifecycle bindings only.
- [ ] Ensure wrappers do not own state-machine logic.

### D4: Files to update (primary)

- [ ] `typescript/vorma/client/src/ui/*` shared host files
- [ ] `typescript/vorma/ui-adapters/react/src/*`
- [ ] `typescript/vorma/ui-adapters/preact/src/*`
- [ ] `typescript/vorma/ui-adapters/solid/src/*`

### D5: Phase-D tests

- [ ] Cross-adapter parity tests for identical transition traces.
- [ ] Adapter conformance tests for route-props/global selector behavior.
- [ ] Tests proving remount semantics are identical across adapters.

## Phase E: Cleanup + Dead Path Deletion

- [ ] Delete all legacy seams superseded by A-D.
- [ ] Delete temporary migration toggles and dual-path branches.
- [ ] Delete tests that only encode removed seams.
- [ ] Ensure internal exports surface only end-state APIs.
- [ ] Preserve render-performance characteristics during cleanup (no accidental
      extra renders/remount churn).
- [ ] Preserve component stability guarantees (unchanged route scope/component
      pairs must not remount spuriously).

## Cross-Phase Invariant Set

- [ ] No mixed old/new snapshot reads in one render tick.
- [ ] No route-props out-of-scope data read is representable.
- [ ] No stale ownership side effects can commit after ownership loss.
- [ ] No malformed wire payload reaches commit/render logic.
- [ ] No adapter-specific behavior divergence for shared semantics.

## Global Verification Gates

### Gate 1 (after A+B)

- [ ] Run targeted source tests for decoder + RouteScope ownership.
- [ ] Run targeted dist adapter transition-tick suites.

### Gate 2 (after C)

- [ ] Run reducer/command seam suites and race coverage.
- [ ] Run targeted parallel-loader behavior tests.

### Gate 3 (after D)

- [ ] Run cross-adapter parity suites.
- [ ] Run render-performance guard suites.
- [ ] Run component-stability/remount guard suites.

### Final gate

- [ ] Run `make tstest-source`.
- [ ] Run `GOCACHE=/tmp/go-build go run ./internal/cmd/buildts`.
- [ ] Run `make tstest-dist`.
- [ ] Run targeted Go matcher/routeparse tests for canonical uniqueness.

## Concrete Tradeoffs

- [ ] Tradeoff: very large coordinated refactor surface. Impact: high sequencing
      risk without strict phase gates.
- [ ] Tradeoff: immutable models increase allocation pressure. Impact:
      structural sharing is mandatory.
- [ ] Tradeoff: short-lived temporary dual paths may increase code size. Impact:
      delete immediately per phase; do not carry forward.
- [ ] Tradeoff: strict decoder/invariant firewall increases hard failures.
      Impact: expected and desired for contract violations.
- [ ] Benefit: broad elimination of race/stale/ownership bug classes. Impact:
      simpler reasoning and fewer distributed guard branches.

## Footprint / Performance Gate

- [ ] Record baseline shipped bundle outputs and key runtime timing metrics.
- [ ] Record post-refactor outputs after full legacy deletion.
- [ ] Require non-inferior shipped footprint or explicit exception.
- [ ] Require no regression in client-loader parallel start/reuse traits.
- [ ] Require no regression in render throughput/tick latency under transition
      stress tests.
- [ ] Require no regression in component stability/remount guarantees for
      unchanged route scopes.

## Success Criteria

- [ ] RouteScope + atomic snapshot fully replaces token ownership machinery.
- [ ] Unified reducer+commands controls all navigation/submission side effects.
- [ ] Wire decoder is sole ingress and invariant firewall.
- [ ] Shared adapter host yields parity across React/Preact/Solid.
- [ ] Render-performance and component-stability guarantees are preserved.
- [ ] Transition-window stale-value bugs are eliminated by model, not patches.

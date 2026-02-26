# Vorma Deep Fix Plan: Route-Instance State Machine (Token + Snapshot)

Date: 2026-02-25
Scope: `typescript/vorma/*` (excluding e2e)
Intent: safe handoff artifact for any new agent

## Problem Summary

Current route-scoped hook behavior can observe global navigation state during transition windows. That allows temporary mismatch between a rendered route component and data selection source. Local sticky-data hacks mask symptoms but do not solve ownership.

We need a design that makes wrong-owner data reads structurally impossible for valid route props.

## Non-Negotiable Invariants

1. Route-scoped reads are ownership-safe.
A hook called with route props must only read data owned by that exact route instance.

2. Transition windows are explicit.
No implicit fallback to "whatever global state currently says".

3. Deterministic route-props behavior.
If route props came from Vorma route rendering, lookup is deterministic across transition windows.

4. Forged or stale route props fail loudly.
No graceful degradation.

5. Consistent semantics across adapters.
React, Preact, and Solid must behave the same.

6. Strict cross-hook coverage.
All route-scoped data hooks (loader + client-loader now, plus any future route-scoped hooks) must share one ownership mechanism.

## Chosen Direction (Hybrid of Options 1 + 2)

Use an opaque route-instance token as capability (runtime-owned) and route-scoped snapshots bound to that token.

- Token gives identity and ownership.
- Snapshot gives stable route-scoped data view through transition windows.
- Hooks resolve by token/snapshot path, not by global arrays/idx.

## Architecture

### 1. Route instance identity

- Introduce `RouteInstanceToken` (opaque object type, not forgeable by normal typed users).
- Create one token per mounted route instance at each outlet depth.
- Token lifecycle states:
  - `active`
  - `exiting`
  - `disposed`

### 2. Runtime route-instance store

Add runtime store keyed by token:

- `idx`
- `routeKey` (derived from importURL/exportKey/index)
- `matchedPattern`
- `loaderDataSnapshot`
- `clientLoaderDataSnapshot`
- `status` (`active|exiting|disposed`)
- `version` (optional monotonic counter)

Store transitions:

- On commit/update: refresh active token snapshots from committed navigation state.
- On branch replacement: mark prior token as `exiting` and retain snapshots.
- On unmount/dispose: mark disposed and remove or tombstone entry.

### 3. Route props internal capability binding

When adapter renders a route component, inject internal non-public props:

- route instance token
- route index (existing)
- existing route props fields

Public type should not expose internals, but runtime resolver must require them for route-scoped hook calls.

### 4. Hook resolution rules

#### `useLoaderData(routeProps)`

- Resolve token from route props.
- Read snapshot from token store.
- If token state is `active|exiting`, return snapshot.
- If missing/disposed token, throw contract violation.

No lookup by current global `matchedPatterns`/idx for route-props path.

#### `useClientLoaderData(routeProps)`

- Same token resolution.
- Verify registered pattern matches token’s bound pattern for that route.
- Return token-bound client-loader snapshot (can be `undefined` legitimately).
- Throw on missing/disposed token or mismatched pattern.

#### Pattern-only APIs (no routeProps)

- May continue using current global matched-pattern selection.
- Must be clearly documented as global-current-route selectors, not route-instance selectors.

### 5. State machine ownership boundaries

- Route outlet runtime owns token creation/update/disposal.
- Typed adapter helper runtime owns hook-level contract checks + snapshot reads.
- Adapters only pass through route props and call shared resolver.

## API Surface Evaluation (preliminary)

Current APIs are mixed:

- `usePatternLoaderData(pattern)` and `useClientLoaderData()` (no route props): generally fine as global selectors.
- `useLoaderData(routeProps)` and `useClientLoaderData(routeProps)`: conceptually valid, but current idx/global-coupled implementation is fragile.

Potential breaking API improvements (recommended to consider):

1. Make route-props contract explicit and stricter.
2. Consider route-context variants (no routeProps argument) for route components.
3. Keep pattern selectors separate and explicitly global.

## Implementation Checklist

### Phase A: Runtime state machine primitives

- [ ] Define `RouteInstanceToken` and internal store types.
- [ ] Add store operations: create/update/mark-exiting/dispose/get-or-throw.
- [ ] Integrate with route outlet branch lifecycle in shared runtime internals.
- [ ] Ensure transitions are deterministic across route commits.

### Phase B: Adapter route props wiring

- [ ] React adapter: inject token into route component props.
- [ ] Preact adapter: inject token into route component props.
- [ ] Solid adapter: inject token into route component props.
- [ ] Ensure token lifecycle hooks tie to mount/unmount semantics per adapter.

### Phase C: Typed hook resolver rewrite

- [ ] Replace idx/global route-props path with token-store resolution in shared helper runtime.
- [ ] Keep pattern-only selectors intact.
- [ ] Enforce loud throw behavior for stale/forged/invalid route props.

### Phase D: Tests (strict and exhaustive)

- [ ] Unit tests for token lifecycle and ownership invariants.
- [ ] Unit tests for hook resolver contract violations.
- [ ] Dist tests React: route-props loader/client-loader across transition window.
- [ ] Dist tests Preact: same coverage.
- [ ] Dist tests Solid: same coverage.
- [ ] Dist tests for mismatched pattern, disposed token, forged props, changed idx.
- [ ] Keep source/dist split requirements:
  - source: `make tstest-source`
  - dist: `make tstest-dist`

### Phase E: Cleanup and docs/comments

- [ ] Remove sticky/fallback hacks.
- [ ] Ensure internal comments explain ownership model and invariants.
- [ ] Keep naming explicit and self-documenting.

## Proposed Error Contract

Use explicit errors (examples):

- `useLoaderData(routeProps) contract violated: route instance token is missing.`
- `useLoaderData(routeProps) contract violated: route instance is disposed.`
- `useClientLoaderData(routeProps) contract violated for pattern "/x": token is bound to "/y".`

No silent fallback.

## Files Likely Impacted

- `typescript/vorma/client/src/ui/route_outlet_runtime.ts`
- `typescript/vorma/client/src/ui/typed_adapter_helpers_runtime.ts`
- `typescript/vorma/client/internal.ts`
- `typescript/vorma/ui-adapters/react/src/react.tsx`
- `typescript/vorma/ui-adapters/react/src/helpers.ts`
- `typescript/vorma/ui-adapters/preact/src/preact.tsx`
- `typescript/vorma/ui-adapters/preact/src/helpers.ts`
- `typescript/vorma/ui-adapters/solid/src/solid.tsx`
- `typescript/vorma/ui-adapters/solid/src/helpers.ts`
- `typescript/vorma/client/src/tests/unit/typed_adapter_helpers_runtime_internal.test.ts`
- `typescript/vorma/client/src/tests/unit/route_outlet_runtime_internal.test.ts`
- `typescript/vorma/client/src/tests/dist/npm_dist_adapters_root_outlet_runtime_state.test.ts`

## Handoff Notes

There are in-progress experimental edits from a prior partial attempt. Treat current workspace state as WIP and re-verify invariants before relying on any existing implementation.

Use this document as source of truth for direction until final implementation lands.

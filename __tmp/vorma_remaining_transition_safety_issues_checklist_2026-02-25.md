# Vorma Remaining Transition-Safety Issues Checklist

Date: 2026-02-25 Scope: `typescript/vorma/*` (exclude e2e) Purpose: track
remaining correctness gaps after route-props token-store fix

## Comprehensive End-State Plan (100% Non-Lazy)

Goal: one coherent runtime state machine where transition correctness is
guaranteed by construction, not by best-effort call ordering.

### Phase A: Single Atomic Runtime Snapshot

- [x] Introduce one canonical runtime snapshot object for all route/render state
      (`matchedPatterns`, `loadersData`, `clientLoadersData`, errors,
      components, params/splats, imports/exports, root-data/build fields).
- [x] Make snapshot reads the only source of truth for shared selectors
      (`getRouterData`, `getClientRuntimeRenderState`, adapter-global
      selectors).
- [x] Add explicit snapshot commit helpers (`setSnapshot`, `updateSnapshot`) and
      ban piecemeal route/render writes in transition paths.
- [x] Keep legacy top-level fields mirrored from snapshot only as compatibility
      surface until follow-up cleanup removes them.

### Phase B: Transition Commit State Machine

- [x] Refactor successful-navigation commit flow so server route data, client
      loader outputs, derived error state, active components, and boundary all
      land in one atomic snapshot commit.
- [x] Ensure commit ownership checks happen before snapshot swap and are not
      interleaved with partial global writes.
- [x] Keep event dispatch ordering strict: snapshot swap first, route/location
      event second.

### Phase C: Route-Props and Global Selector Semantics

- [x] Preserve strict route-props ownership through the route-instance store
      (already implemented) and validate it still reads from synchronized
      snapshot data only.
- [x] Keep pattern/global selectors explicitly dynamic-global, but guarantee
      each tick observes one coherent committed snapshot.
- [x] Add/extend per-tick tests that prove no mixed old/new snapshot reads in
      global selectors across transitions.

### Phase D: Internal API Hardening

- [x] Remove/de-export resolver APIs that can bypass route-instance ownership
      contracts.
- [x] Replace any remaining internal helper paths that accept routeProps +
      global arrays directly with snapshot/route-instance safe resolvers.

### Phase E: Matcher Invariants

- [x] Enforce normalized-pattern uniqueness as fail-fast in TS matcher
      registration.
- [x] Enforce normalized-pattern uniqueness as fail-fast in Go matcher
      registration/runtime path.
- [x] Extend build parsing/validation to reject normalized collisions, not just
      exact raw duplicate strings.

### Phase F: Verification Gate

- [x] Add unit coverage for atomic snapshot commit semantics.
- [x] Add unit coverage for forbidden internal bypass API paths.
- [x] Add dist coverage (React/Preact/Solid) for global selector coherence
      across transition ticks.
- [x] Add TS matcher tests for normalized collision rejection.
- [x] Add Go tests for normalized collision rejection.
- [x] Run `make tstest-source`.
- [x] Rebuild with `GOCACHE=/tmp/go-build go run ./internal/cmd/buildts`.
- [x] Run `make tstest-dist`.
- [x] Run targeted Go tests covering matcher/route parse invariants.

## Open Issues

- [x] Enforce normalized-pattern uniqueness (fail fast) across registration and
      build.
    - Problem: matcher state is keyed by normalized pattern; collisions can
      silently overwrite prior registrations.
    - Clarification: duplicate pattern strings in one matched chain are not
      representable with current matcher maps; the real risk is silent
      registration shadowing before matching.
    - Risk: wrong route/task can win unexpectedly when two different raw route
      definitions normalize to the same matcher key.
    - Primary files:
        - `kit/internal/matchercore/matchercore.go`
        - `typescript/kit/matcher/register.ts`
        - `vormabuild/routeparse/routeparse.go`
    - Required direction:
        - fail loudly on normalized-pattern collision (not just warn/overwrite).
        - add tests that prove collisions are rejected in both TS and Go paths.

- [x] Decide and implement transition semantics for global selector APIs.
    - Problem: `useRouterData`, `usePatternLoaderData(pattern)`, and
      `useClientLoaderData()` (no routeProps) are global-current-route reads and
      can legitimately observe transition windows.
    - Risk: consumers can still observe changing/ambiguous values across
      transitions even though route-props paths are ownership-safe.
    - Primary files:
        - `typescript/vorma/ui-adapters/react/src/helpers.ts`
        - `typescript/vorma/ui-adapters/preact/src/helpers.ts`
        - `typescript/vorma/ui-adapters/solid/src/helpers.ts`
    - Required decision:
        - keep as global semantics and document as such in code comments/tests,
          or
        - add route-instance-scoped alternatives and steer users to those for
          strict ownership.

- [x] Remove or lock down internal resolver escape hatch that bypasses
      token-store ownership.
    - Problem: `resolveTypedAdapterIndexedDataForPatternOrRouteProps` is still
      exported from internals and can be used outside the new explicit store
      model.
    - Risk: future adapter/internal usage can reintroduce old idx/global-coupled
      behavior.
    - Primary files:
        - `typescript/vorma/client/src/ui/typed_adapter_helpers_runtime.ts`
        - `typescript/vorma/client/internal.ts`
    - Required direction:
        - de-export or hard-fail for routeProps path, keep only safe resolvers
          used by adapters.

- [x] Evaluate global runtime commit atomicity for non-adapter readers.
    - Problem: route commit writes multiple global fields sequentially, not as
      one immutable snapshot replacement.
    - Risk: direct global readers can observe partial commit state.
    - Primary files:
        - `typescript/vorma/client/src/core/render_commit_runtime.ts`
        - `typescript/vorma/client/src/app/context.ts`
    - Required direction:
        - either make commits atomic via single snapshot object, or
          enforce/store all consumers behind adapter store sync boundary.

## Verification Checklist (for when fixes are implemented)

- [x] Add tests proving normalized-pattern collisions are rejected (TS matcher,
      Go matcher, and build route parsing).
- [x] Add tests that prevent use of bypassed internal resolver paths.
- [x] Add tests confirming selected global-selector semantics (explicitly global
      or stricter).
- [x] Run `make tstest-source`.
- [x] Rebuild with `GOCACHE=/tmp/go-build go run ./internal/cmd/buildts`.
- [x] Run `make tstest-dist`.

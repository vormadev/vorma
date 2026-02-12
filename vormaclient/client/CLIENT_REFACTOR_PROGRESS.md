# vormaclient/client Refactor Progress

## Snapshot

- Date: 2026-02-12
- Phase: aggressive internal cleanup with strict first-principles tests
- Status: all gates currently passing

## Current Source Of Truth

- Public behavior: `src/tests/contracts/`
- Internal branch and race hardening: `src/tests/unit/`
- Legacy tests: removed
- Rule: tests assert objectively correct behavior, never implementation quirks

## Runtime Structure

- Production modules:
    - `src/app/`
    - `src/core/`
    - `src/platform/`
    - `src/ui/`
- Test modules:
    - `src/tests/contracts/`
    - `src/tests/unit/`

## Major Completed Cleanup

- Navigation bookkeeping, matching, and phase transitions centralized around
  shared slot primitives in `src/core/navigation/runtime.ts`.
- Begin-navigation flow deduplicated in
  `src/core/navigation/begin_navigation.ts` (shared prefetch matching and shared
  promotion path).
- Link click/prefetch flow deduplicated in `src/core/links.ts`:
    - single eligible-anchor gate
    - explicit `aborted/redirect/success` handling
    - impossible nullable `NavigationControl.promise` branch removed
    - duplicate idle-prefetch guard removed
- URL normalization deduplicated with shared `resolveAbsoluteHref` in
  `src/platform/url.ts`, now used by navigation runtime, begin-navigation,
  links, history, and redirect parsing.
- Redirect logic deduplicated in `src/core/redirects.ts` via shared parser and
  strategy executor primitives.
- Render/runtime/context safety hardening completed:
    - explicit route JSON vs runtime-only state separation
    - optional-safe initialization branches for partially initialized globals
    - cross-realm global-instance detection in `src/platform/safety.ts`
- Build-time API cut applied: `route` no longer exported from runtime client
  entrypoint.
- Render-runtime loader race coverage expanded:
    - `src/tests/unit/render_runtime_internal.test.ts` now pins downstream
      loader abort propagation when an earlier client loader fails with a
      non-abort error.
- History POP fallback coverage expanded:
    - `src/tests/unit/history_listener_prelude.test.ts` now pins both
      cross-document POP fallback behavior (reload path) and successful POP
      state synchronization behavior.
    - `src/platform/history.ts` hard-reload fallback now skips reload invocation
      in JSDOM environments to keep non-browser test runtimes deterministic.
- Submit-race stale checkpoint coverage expanded:
    - `src/tests/unit/navigation_runtime_internal.test.ts` now pins stale
      deduped submit outcomes at all reachable async checkpoints:
        - after response receipt/before finalize (build-id listener replacement)
        - after JSON parsing (replacement during parse)
        - before auto-revalidation (replacement during method access)
    - removed one unreachable stale checkpoint branch in
      `src/core/navigation/runtime.ts` redirect path (no async boundary existed
      between stale checks).
- Runtime slot-matching cleanup expanded:
    - removed impossible nullable prefetch-entry branch in
      `src/core/navigation/runtime.ts` slot matching.
    - added explicit no-op contract for `removeNavigation` when target key is
      absent in `src/tests/unit/navigation_runtime_internal.test.ts`.
- Runtime defensive submit handling hardened:
    - added explicit guard + contract for impossible missing submit response
      objects from redirect handling.
- Begin-navigation and fetch-route-data defensive coverage expanded:
    - `src/tests/unit/navigation_runtime_internal.test.ts` now pins:
        - same-target prefetch dedupe reuse from active navigation
        - same-target prefetch dedupe reuse from pending revalidation
        - build-id fallback (`"1"`) in client-loader server-data handoff when
          response build-id header is missing
        - sparse partial-match invariants now fail fast with explicit errors
          before starting parallel client loaders
        - production dep preloading that ignores falsy dep entries
        - uncached client-loader reuse guard (missing current snapshot cache)
          runs correctly without incorrectly seeding stale loader results
        - skip-check invariants now fail fast for malformed matcher results
          (sparse or empty route-pattern entries)
- Runtime/buildtime API boundary contracts added:
    - `src/tests/contracts/client.utilities.contract.test.ts` now pins:
        - runtime client entrypoint does not export buildtime-only `route`
        - buildtime entrypoint exports callable `route` registration API
- Consumer package build gate re-validated:
    - `make npmbuild` surfaced strict typing regressions in `vormaclient/solid`
      and `vormaclient/preact` adapter wrappers.
    - Fixed adapter typing at first principles (explicit typed memo/accessor
      outputs, event typing alignment, and component typing for renderer
      adapters) without adding compatibility shims.
    - `make npmbuild` now passes end-to-end again.
- Render-runtime branch hardening expanded:
    - `src/tests/unit/render_runtime_internal.test.ts` now pins:
        - explicit `scrollToTop: false` user-navigation behavior
        - empty-title fallback when title HTML payload is missing
        - null head-array normalization to empty arrays
        - malformed client-loader result handling when data arrays are missing
- Runtime response-artifact fallback hardening expanded:
    - `src/tests/unit/navigation_runtime_internal.test.ts` now pins behavior
      when response artifact arrays and module map are missing.
- Render-runtime internal cleanup expanded:
    - normalized client-loader inputs once per execution in
      `src/core/render_runtime.ts` instead of repeatedly applying ad hoc
      fallback checks inside loader loops.
    - `setClientLoadersState` now uses normalized loader data consistently for
      both state assignment and error-index derivation, avoiding malformed-data
      crashes and invalid negative indexes.

## Current Test Inventory

- `27` test files
- `408` tests

## Latest Verified Gate (2026-02-12)

- `pnpm oxlint vormaclient/client/src`
- `pnpm tsc --noEmit --project vormaclient/client`
- `pnpm tsgo --noEmit --project vormaclient/client`
- `pnpm vitest --run vormaclient/client/src`
- `pnpm vitest --run vormaclient/client/src --coverage --coverage.reporter=text-summary`

Result:

- tests: pass (`27` files, `408` tests)
- coverage summary:
    - statements: `92.03%`
    - branches: `86.27%`
    - functions: `95.09%`
    - lines: `92.42%`
- notable files:
    - `src/core/links.ts`: `100%` statements/branches/functions/lines
    - `src/core/redirects.ts`: `100%` statements/branches/functions/lines
    - `src/core/navigation/runtime.ts`: `100%` statements, `100%` branches,
      `98.36%` functions, `100%` lines
    - `src/core/navigation/begin_navigation.ts`: `100%` statements, `100%`
      branches, `100%` functions, `100%` lines
    - `src/core/navigation/fetch_route_data.ts`: `100%` statements, `98.63%`
      branches, `100%` functions, `100%` lines
    - `src/core/render_runtime.ts`: `99.56%` statements, `100%` branches,
      `97.5%` functions, `100%` lines
    - `src/platform/history.ts`: `95.31%` statements, `95.12%` branches, `100%`
      functions, `95.31%` lines
    - `src/platform/safety.ts`: `100%` statements, `93.33%` branches, `100%`
      functions, `100%` lines

## Remaining Work

- Continue shrinking internal complexity in runtime/navigation/render code while
  keeping behavior pinned by strict contracts.
- Add targeted tests only where uncovered defensive branches correspond to real
  runtime risk (especially async ordering/race paths).
- Keep docs lean: update this file and `HIGH_LEVEL_PLAN.md` only with current
  state, not historical churn.

## Takeover Checklist

1. Read `HIGH_LEVEL_PLAN.md`.
2. Read this file.
3. Re-run the full validation gate above.

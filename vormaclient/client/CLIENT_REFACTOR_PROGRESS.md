# vormaclient/client Refactor Progress

## Snapshot

- Date: 2026-02-11
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

## Current Test Inventory

- `27` test files
- `380` tests

## Latest Verified Gate (2026-02-11)

- `pnpm oxlint vormaclient/client/src`
- `pnpm tsc --noEmit --project vormaclient/client`
- `pnpm tsgo --noEmit --project vormaclient/client`
- `pnpm vitest --run vormaclient/client/src`
- `pnpm vitest --run vormaclient/client/src --coverage --coverage.reporter=text-summary`

Result:

- tests: pass (`27` files, `380` tests)
- coverage summary:
    - statements: `91.53%`
    - branches: `83.47%`
    - functions: `95.08%`
    - lines: `92.32%`
- notable files:
    - `src/core/links.ts`: `100%` statements/branches/functions/lines
    - `src/core/redirects.ts`: `100%` statements/branches/functions/lines
    - `src/core/navigation/runtime.ts`: `98.67%` statements, `91.42%` branches,
      `98.36%` functions, `100%` lines
    - `src/core/render_runtime.ts`: `99.55%` statements, `90.32%` branches,
      `97.5%` functions, `100%` lines
    - `src/platform/history.ts`: `95.31%` statements, `92.68%` branches, `100%`
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

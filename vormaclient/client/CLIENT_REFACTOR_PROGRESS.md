# vormaclient/client Refactor Progress

## Snapshot

- Date: 2026-02-11
- Phase: aggressive internal cleanup with strict contract safety net
- Status: gates passing on current `vormaclient/client` tree

## Current Source of Truth

- Runtime behavior is pinned by:
    - `src/tests/contracts/` (public-behavior contracts)
    - `src/tests/unit/` (targeted internals)
- Legacy suites are removed.
- First-principles correctness is mandatory; do not weaken assertions to match
  suspicious implementation quirks.

## Recent Material Changes

- Closed remaining gap bundle (helpers/HMR/init-manifest coverage):
    - `src/tests/unit/app_helpers.test.ts`
    - `src/tests/contracts/client.utilities.contract.test.ts`
    - `src/tests/contracts/client.history_and_init.contract.test.ts`
- Hard API cut for route registration:
    - `route` removed from `vorma/client`
    - `route` now only from `vorma/buildtime`
    - dedicated buildtime entrypoint: `buildtime.ts`
- Build-time/static parsing aligned with hard cut:
    - route parser in `vormabuild` now recognizes `vorma/buildtime`
- Current contract suite count in `vormaclient/client`:
    - `24` test files
    - `295` tests
- Upgraded Vitest + coverage provider to aligned `4.0.18` and fixed upgrade
  fallout in two contract suites by isolating `scrollIntoView` spies per
  element:
    - `src/tests/contracts/client.utilities.contract.test.ts`
    - `src/tests/contracts/client.history_and_init.contract.test.ts`
    - runtime behavior unchanged; only test isolation/stability changed.

## Current Structure

- Runtime modules:
    - `src/app/`
    - `src/core/`
    - `src/platform/`
    - `src/ui/`
- Tests:
    - `src/tests/contracts/`
    - `src/tests/unit/`

## Active Policies

- Fix production code when strict tests expose objectively incorrect behavior.
- Ask the user only when behavior is truly ambiguous.
- No compatibility shims unless explicitly requested.
- Keep harness/docs lean; prune historical or dead scaffolding.

## Known Gaps

- `route(...)` is intentionally build-time-only and not a runtime client
  behavior surface.
- HMR coverage validates hook installation and callability, but not a full
  synthetic `vite:afterUpdate` event bus simulation.
- Coverage command is now operational under aligned Vitest 4 tooling.

## Latest Verified Gate

- `pnpm oxlint vormaclient/client/src`
- `pnpm tsc --noEmit --project ./vormaclient/client`
- `pnpm tsgo --noEmit --project ./vormaclient/client`
- `pnpm vitest --run vormaclient/client/src`
- `pnpm vitest --run --coverage vormaclient/client/src`
- Result: passing (`24` files, `295` tests)
- Coverage snapshot:
    - all files: `84.55%` statements, `72.17%` branches, `92.15%` functions,
      `85.8%` lines

## You-Are-Here

- `HIGH_LEVEL_PLAN.md`: step 4 in progress (cleanup/refactor).
- Highest leverage next: continue reducing internal complexity while preserving
  contract strictness.

## Takeover Checklist

1. Read `HIGH_LEVEL_PLAN.md`.
2. Read this file.
3. Run the full gate above.
4. Keep strict first-principles assertions; do not reintroduce legacy mirrors.

# vormaclient/client Refactor Progress

## Snapshot

- Date: 2026-02-11
- Current phase: safety-net hardening before deeper refactor
- Status: clean gates on current tree

## Latest Changes (2026-02-11)

- Reduced root-level documentation/tooling cruft:
    - removed historical tracker bundles and archive folder
    - removed obsolete generated parity map artifact
    - removed obsolete parity-map generator script
    - retained minimal active docs:
        - `HIGH_LEVEL_PLAN.md`
        - `CLIENT_REFACTOR_PROGRESS.md`
- Added navigation model/state-machine contract suite:
    - `src/tests/contracts/client.navigation_state_machine.contract.test.ts`
    - validates last-intent-wins behavior under out-of-order completion
    - validates mixed `prefetch -> navigate -> submit(dedupe) -> revalidate`
      invariants with stale completion ordering
- Extended contract harness fetch recorder for sequence modeling:
    - `src/tests/contracts/contract_test_harness.ts`
    - `createAbortAwareFetchRecorder()` now records request `input` and `init`
      for URL/method assertions
- Fixed runtime bug found by new strict test:
    - `src/core/navigation/runtime.ts`
    - deduped submit aborts now consistently return `"Aborted"` when the
      underlying rejection reason is not an `AbortError` object but the submit
      signal is aborted

## Current Structure

- Production modules are organized under:
    - `src/app/`
    - `src/core/` (navigation runtime under `src/core/navigation/`)
    - `src/platform/`
    - `src/ui/`
- Tests are organized under:
    - `src/tests/contracts/` (strict first-principles acceptance)
    - `src/tests/legacy/` (parity safety net; each file starts with
      `READY_TO_DELETE_AFTER_SIGNOFF`)
    - `src/tests/unit/` (targeted unit tests for extracted internals)

## Safety Policy

- Contract tests must assert first-principles correctness, not implementation
  quirks.
- If a strict test fails and behavior is objectively wrong, fix production code.
- Ask the user immediately only for truly ambiguous product behavior.
- Legacy tests remain until explicit signoff allows physical deletion.

## Latest Verified Gate

- `pnpm oxlint vormaclient/client/src`
- `pnpm tsc --noEmit --project ./vormaclient/client`
- `pnpm tsgo --noEmit --project ./vormaclient/client`
- `pnpm vitest --run vormaclient/client/src`
- Result: passing (`49` test files, `435` tests)

## You-Are-Here

- High-level plan lives in `HIGH_LEVEL_PLAN.md`.
- Current execution point: step 3, expand race-focused regressions.

## Takeover Checklist

1. Read `HIGH_LEVEL_PLAN.md`.
2. Read this file.
3. Run the full gate listed above.
4. Keep strict contract assertions; do not weaken to match suspicious behavior.
5. Keep legacy files with `READY_TO_DELETE_AFTER_SIGNOFF` until signoff.

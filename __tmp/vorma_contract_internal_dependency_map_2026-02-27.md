# Vorma Contract-Test Internal Dependency Map (2026-02-27)

This map identifies contract-suite areas that currently depend on internal
symbols or runtime internals instead of pure black-box observables.

Update:

- `contract_test_harness.ts` has been rewritten to avoid internal-runtime
  imports and `__*` helper wiring; remaining internal-coupled usage is in legacy
  contract test files.

## Summary

- Contract files with internal (`__*`) dependencies: 14
- Highest concentration files:
    - `client.history_and_init.contract.test.ts`
    - `client.prefetch.contract.test.ts`
    - `client.utilities.contract.test.ts`
    - `client.link_click.contract.test.ts`
    - `contract_test_harness.ts`

## Per-File Internal Dependency Counts

- `typescript/vorma/client/src/tests/contracts/client.history_and_init.contract.test.ts`
    - total internal-token hits: 77
    - top symbols: `__vormaClientGlobal`, `__runClientLoadersAfterHMRUpdate`,
      `__loadRouteManifestProgressively`
- `typescript/vorma/client/src/tests/contracts/client.prefetch.contract.test.ts`
    - total internal-token hits: 41
    - top symbols: `__getPrefetchHandlers`, `__vormaClientGlobal`
- `typescript/vorma/client/src/tests/contracts/contract_test_harness.ts`
    - total internal-token hits: 33
    - top symbols: `__vormaClientGlobal`, `__runClientLoadersAfterHMRUpdate`,
      `__registerClientLoaderPattern`, `__getPrefetchHandlers`
- `typescript/vorma/client/src/tests/contracts/client.utilities.contract.test.ts`
    - total internal-token hits: 28
    - top symbols: `__makeFinalLinkProps`, `__applyScrollState`,
      `__vormaClientGlobal`
- `typescript/vorma/client/src/tests/contracts/client.link_click.contract.test.ts`
    - total internal-token hits: 27
    - top symbols: `__makeLinkOnClickFn`, `__makeFinalLinkProps`
- `typescript/vorma/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts`
    - total internal-token hits: 11
    - top symbols: `__vormaClientGlobal`, `__registerClientLoaderPattern`
- `typescript/vorma/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts`
    - total internal-token hits: 11
    - top symbols: `__getPrefetchHandlers`, `__vormaClientGlobal`
- `typescript/vorma/client/src/tests/contracts/client.error_and_edge.contract.test.ts`
    - total internal-token hits: 8
    - top symbols: `__getPrefetchHandlers`, `__vormaClientGlobal`,
      `__registerClientLoaderPattern`
- `typescript/vorma/client/src/tests/contracts/vorma_ctx.contract.test.ts`
    - total internal-token hits: 7
    - top symbols: `__getVormaClientGlobal`
- `typescript/vorma/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts`
    - total internal-token hits: 5
    - top symbols: `__getPrefetchHandlers`, `__clearNavigationDebugJournal`,
      `__getNavigationDebugJournal`
- `typescript/vorma/client/src/tests/contracts/client.loading_and_focus.contract.test.ts`
    - total internal-token hits: 4
    - top symbols: `__getPrefetchHandlers`
- `typescript/vorma/client/src/tests/contracts/client.state_and_revalidation.contract.test.ts`
    - total internal-token hits: 3
    - top symbols: `__getPrefetchHandlers`, `__vormaClientGlobal`
- `typescript/vorma/client/src/tests/contracts/client.navigation_modes.contract.test.ts`
    - total internal-token hits: 2
    - top symbols: `__getPrefetchHandlers`
- `typescript/vorma/client/src/tests/contracts/client.submit_and_redirect.contract.test.ts`
    - total internal-token hits: 1
    - top symbols: `__vormaClientGlobal`

## Rewrite Direction

- Replace internal-symbol assertions with public-observable checks:
    - DOM/title/head effects
    - location/history behavior
    - event payloads and ordering
    - network request count/URL/method/body
    - public API return values and status snapshots
- Keep dedicated internal tests (unit) for engine logic only where they remain
  useful during migration, but do not let contract-suite semantics depend on
  internals.

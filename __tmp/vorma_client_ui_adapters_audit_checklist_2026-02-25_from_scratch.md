# Vorma Client + UI Adapters Fresh Audit Checklist (2026-02-25)

Scope:

- `typescript/vorma/client/*`
- `typescript/vorma/ui-adapters/*`

## Architecture / Design

- [x] Implemented: route-manifest application is now atomic in init flows.
      Manifest registration is applied to a cloned registry and only committed
      to global runtime state after full success (for both precompiled and
      progressive paths), preventing partial `routeManifest` / pattern-registry
      commits on registration throws.
      (`typescript/vorma/client/src/app/init.ts:166`,
      `typescript/vorma/client/src/app/init.ts:204`)

- [x] Decision made: manifest contract violations must fail fast. Backend-owned
      manifest payload shape/value errors (for progressive or precompiled paths)
      should throw and halt init instead of being swallowed as best-effort
      progressive enhancement; only transport/availability failures remain
      non-fatal. (`typescript/vorma/client/src/app/init.ts:199`)

- [x] Decision made: keep raw history escape hatch, but explicitly mark it
      unsafe and document intended/non-intended usage. Public API renamed from
      `getHistoryInstance()` to `getUnsafeHistoryInstance()` and JSDoc/README
      now state when to use it and when not to.
      (`typescript/vorma/client/src/client.ts:305`,
      `typescript/vorma/client/README.md:19`)

- [x] Decision made: keep `vorma_json` and `dpl` fixed and reserved. No
      configurability is needed for this protocol surface in this pass. `dpl`
      remains Vercel-specific internal behavior; `vorma_json` remains an
      explicit reserved query key documented for applications.
      (`typescript/vorma/client/src/core/navigation/fetch_route_data_server.ts:124`,
      `typescript/vorma/client/src/core/navigation/fetch_route_data_server.ts:129`,
      `typescript/vorma/client/README.md:39`)

- [x] Decision made: missing managed head section markers are a hard invariant
      failure. `updateHeadEls(...)` should throw immediately instead of silently
      no-oping when markers are absent.
      (`typescript/vorma/client/src/ui/head.ts:254`)

- [x] Decision made: keep `Symbol.for("__vorma_internal__")` fixed and
      non-configurable. Intentional collisions in application code are out of
      scope for runtime guardrails in this pass.
      (`typescript/vorma/client/src/app/context.ts:65`)

- [x] Confirmed no remaining `clientModuleMap` runtime state references in
      `client/*` or `ui-adapters/*`.

## Coupling / DRY / Bundle

- [x] Implemented: renamed modality state from `isTouchDevice` to
      `isTouchInputModalityActive` to reflect actual semantics (current input
      modality, not hardware capability).
      (`typescript/vorma/client/src/app/context.ts:104`,
      `typescript/vorma/client/src/app/init.ts:39`,
      `typescript/vorma/client/src/ui/helpers.ts:142`)

- [x] Implemented: avoided redundant modality writes on high-frequency pointer
      events by no-oping when modality state is unchanged before writing.
      (`typescript/vorma/client/src/app/init.ts:39`,
      `typescript/vorma/client/src/app/init.ts:47`)

- [x] Decision made: retaining one head tag per unique dependency URL is
      acceptable in this pass (modulepreload/preload/stylesheet links stay in
      DOM and are deduped by URL).
      (`typescript/vorma/client/src/core/render_commit_runtime.ts:23`,
      `typescript/vorma/client/src/core/render_commit_runtime.ts:39`,
      `typescript/vorma/client/src/core/render_commit_runtime.ts:63`)

- [x] Implemented: fixed CSS preload in-flight semantics without changing
      retention strategy. `preloadCSS(...)` now tracks and shares one pending
      promise per href until load/error.
      (`typescript/vorma/client/src/core/render_commit_runtime.ts:23`,
      `typescript/vorma/client/src/core/render_commit_runtime.ts:41`,
      `typescript/vorma/client/src/core/navigation/runtime_navigation_successful_runtime.ts:130`)

## Tests / Contracts

- [x] Implemented: manifest contract tests now assert fail-fast behavior for
      backend-owned manifest payload violations (invalid shape/flags/pattern
      normalization) in both progressive and precompiled paths, while preserving
      non-fatal behavior for transport/availability failures.
      (`typescript/vorma/client/src/tests/contracts/client.history_and_init.contract.test.ts:1303`,
      `typescript/vorma/client/src/tests/contracts/client.history_and_init.contract.test.ts:1413`,
      `typescript/vorma/client/src/tests/contracts/client.history_and_init.contract.test.ts:1534`)

- [x] Decision made: no change for `makeTypedUseLoaderData(...)` in this pass.
      Contract assumes blessed route-component context with correctly-typed
      route props; misuse outside that context is out of scope.

- [x] Decision made: no change for `useClientLoaderData(routeProps)` overload in
      this pass. Non-abort client-loader failures drive error-boundary rendering
      at the failed index, so the failed route component does not continue
      rendering normal route-content hooks.
      (`typescript/vorma/client/src/core/render_client_loader_runtime.ts:198`,
      `typescript/vorma/client/src/core/render_client_loader_runtime.ts:285`,
      `typescript/vorma/client/src/ui/route_outlet_runtime.ts:374`)

- [x] Implemented: head marker contract tests now assert fail-fast behavior for
      missing/out-of-order markers (explicit throw expectations).
      (`typescript/vorma/client/src/tests/contracts/head_elements.contract.test.ts:63`,
      `typescript/vorma/client/src/tests/contracts/head_elements.contract.test.ts:548`,
      `typescript/vorma/client/src/ui/head.ts:254`)

- [x] Implemented: added regression tests for CSS preload in-flight dedupe
      semantics so repeated/concurrent same-href preloads await a shared pending
      promise and do not resolve early.
      (`typescript/vorma/client/src/tests/unit/render_runtime_internal.test.ts:735`,
      `typescript/vorma/client/src/tests/contracts/client.prefetch.contract.test.ts:505`,
      `typescript/vorma/client/src/core/render_commit_runtime.ts:41`)

- [x] Decision made: no additional collision/override tests for reserved query
      keys in this pass. Contract is explicit reservation, not app-param
      coexistence.

## Verification Snapshot

- [x] `make tstest-source` passed (`67` files, `1228` tests).
- [x] `make tstest-dist` passed (`6` files, `75` tests).

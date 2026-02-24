# Vorma Client + UI Adapters Audit Checklist (2026-02-24)

## Runtime and Adapter Issues

- [x] Strip all navigation-internal props from adapter anchor DOM output, not
      just `prefetch`/`scrollToTop`/`replace`/`state`; ensure `beforeBegin`,
      `beforeRender`, `afterRender`, and `prefetchDelayMs` never leak to
      rendered `<a>` elements in
      `typescript/vorma/ui-adapters/react/src/link.tsx`.
- [x] Strip all navigation-internal props from adapter anchor DOM output, not
      just `prefetch`/`scrollToTop`/`replace`/`state`; ensure `beforeBegin`,
      `beforeRender`, `afterRender`, and `prefetchDelayMs` never leak to
      rendered `<a>` elements in
      `typescript/vorma/ui-adapters/preact/src/link.tsx`.
- [x] Strip all navigation-internal props from adapter anchor DOM output, not
      just `prefetch`/`scrollToTop`/`replace`/`state`; ensure `beforeBegin`,
      `beforeRender`, `afterRender`, and `prefetchDelayMs` never leak to
      rendered `<a>` elements in
      `typescript/vorma/ui-adapters/solid/src/link.tsx`.
- [x] Update dist adapter link tests to assert that all navigation-only props
      are stripped from DOM anchors, not only the current subset in
      `typescript/vorma/client/src/tests/dist/npm_dist_adapters_helpers_link_mocked.test.ts`.
- [x] Fix client-loader skip behavior so server-error skip decisions come from
      the current navigation payload, not stale global state from previous
      routes in
      `typescript/vorma/client/src/core/render_client_loader_runtime.ts`.
- [x] Extend client-loader runtime types and flow to carry fresh
      `outermostServerErrorIdx` through completion, including navigation fetch
      path integration in
      `typescript/vorma/client/src/core/navigation/fetch_route_data_server.ts`.
- [x] Add focused tests proving client-loader skip behavior is correct when
      prior global error state differs from the new navigation response.
- [x] Make `resolveVormaPath` fail fast when required dynamic params or splat
      values are unresolved, instead of silently returning unresolved tokens
      like `:id` or `*`, in `typescript/vorma/client/src/app/helpers.ts`.
- [x] Add tests for unresolved path token failure cases in
      `typescript/vorma/client/src/tests/unit/app_helpers.test.ts`.

## Contracts and Assumptions to Clarify

- [x] Clarify and memorialize usage contract for `makeTypedAddClientLoader`
      registration timing and duplicate registration expectations in adapter
      helper modules.
- [x] Memorialize the speculative client-loader execution contract in public
      runtime typing docs: client loaders may execute before final ownership
      checks, stale results may be discarded, side effects should be idempotent,
      and `AbortSignal` should be honored for cancellation-aware cleanup.
- [x] Add contract coverage proving speculative client-loader execution can run
      for a navigation that is later superseded while stale commits are still
      blocked.

## Refactor and Bundle Hygiene

- [x] Extract duplicated typed-link and helper-registration logic shared across
      React/Preact/Solid adapters to reduce drift and maintenance surface.
- [x] Verify no additional accidental public/internal leakage exists beyond
      `client/__internal`, especially around globals and adapter-facing
      contracts.

## Validation Follow-Up

- [x] Run source and dist TypeScript test splits after fixes
      (`make tstest-source` and `make tstest-dist`) and confirm no behavior
      regressions.

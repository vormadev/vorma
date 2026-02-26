# Fresh Audit Checklist (2026-02-25)

Scope:

- `typescript/vorma/client/*`
- `typescript/vorma/ui-adapters/*`

## Architecture and contracts

- [ ] Make progressive manifest payload contract failures deterministic during
      normal `initClient(...)` flow.
    - Why: `initClient(...)` currently calls
      `void loadRouteManifestProgressively()` when precompiled manifest is
      absent, so payload parse/validation failures reject an unobserved promise.
    - References:
        - `typescript/vorma/client/src/app/init.ts`
          (`void loadRouteManifestProgressively()`)
        - `typescript/vorma/client/src/app/init.ts`
          (`parseRouteManifestPayloadOrThrow`)
    - Follow-up tests:
        - Contract test where `initClient(...)` triggers progressive load with
          malformed payload, asserting the chosen fail-hard behavior (not only
          direct `__loadRouteManifestProgressively()` invocation).

## Behavior and runtime safety

- [ ] Eliminate repeated module-loading passes during initial bootstrap.
    - Why: bootstrap path currently calls component loading multiple times
      (`handleComponents`, `setupClientLoaders` -> `loadComponents`, and
      `handleErrorBoundaryComponent` -> `loadComponents`).
    - References:
        - `typescript/vorma/client/src/app/init.ts`
          (`bootstrapInitialClientRuntime`)
        - `typescript/vorma/client/src/core/render_client_loader_runtime.ts`
          (`executeClientLoaders`)
        - `typescript/vorma/client/src/core/render_component_runtime.ts`
          (`loadComponents`)
    - Follow-up tests:
        - Unit test validating single module-loading pass across init bootstrap
          steps.

## Resource policy clarification

- [ ] Codify the CSS preload-link retention policy and test it.
    - Why: preload links are deduped but retained indefinitely; this should be
      explicit and tested either as an intentional once-per-href cache marker,
      or changed to cleanup-after-load with equivalent safety.
    - Reference:
        - `typescript/vorma/client/src/core/render_commit_runtime.ts`
          (`preloadCSS`)
    - Follow-up tests:
        - Contract test that repeated preloads do not add duplicate preload
          tags.
        - If cleanup policy is chosen: contract test that loaded tags are
          removed and future preloads still behave correctly.

## Audit execution notes

- Source tests run: `make tstest-source` (pass).
- Dist tests run: `make tstest-dist` (pass).

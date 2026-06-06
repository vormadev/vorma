# Frontend Audit Friendliness

Purpose: make the TypeScript/frontend framework easy and pleasant to audit without
changing behavior, changing public APIs, or growing the runtime bundle.

This is not a security audit. This is a maintainability, naming, organization, and
architecture-simplification pass over the frontend code.

## Rules

- Start with a read-only audit. The first deliverable is a list of entropy findings and
  proposed simplifications.
- Do not change observable behavior unless the behavior is a clear, objective bug.
- Do not change public APIs unless that is explicitly approved for the specific change.
- Do not grow the bundle. Establish a before/after bundle-size baseline for any
  implementation pass, including gzipped size where useful.
- Do not move type-only generated contracts into runtime imports.
- Do not let dev-only HMR/Vite code leak into production bundles.
- Treat timing and identity as observable behavior. Navigation ordering, cancellation,
  supersession, request timing, component mount identity, state retention, CSS/head DOM
  effects, and HMR behavior are all part of the behavioral contract.
- Prefer naming, ownership, file organization, and direct simplification over new
  abstractions.
- Delete branches or duplication only when the behavior-equivalence argument is obvious
  and testable.
- If a clear bug is found, fix the bug directly and add a test that proves the intended
  behavior, not merely the current implementation.
- Do not use tests to bless an existing undesirable behavior.

## Audit Targets

- `packages/vorma/core/create_client_core.ts`: navigation lifecycle, client loaders, route
  commits, supersession, work indicators, HMR, and stale-client handling.
- `packages/vorma/core/*`: matcher readiness, CSS/head handling, request/resource helpers,
  generated contract boundaries, and client runtime state.
- `packages/vorma/ui/{react,preact,solid}`: adapter identity, mount behavior, state
  retention, and shared adapter semantics.
- `packages/vorma/vite/*`: dev-only gates, HMR boundaries, generated module exposure, and
  production bundle safety.
- `packages/vorma/tests/*`: whether tests prove intended frontend semantics without
  mirroring implementation details.

## Bundle Baseline

Command: `make ts-build`, followed by byte/gzip measurement of the built package
artifacts. These package artifacts are diagnostic. Final Vite production application
output is the runtime bundle metric that matters.

- `packages/vorma/.dist/core/_index.js`: 95,367 bytes, 22,290 gzipped
- `packages/vorma/.dist/ui/react/react.js`: 9,005 bytes, 2,525 gzipped
- `packages/vorma/.dist/ui/preact/preact.js`: 7,785 bytes, 2,222 gzipped
- `packages/vorma/.dist/ui/solid/solid.js`: 9,068 bytes, 2,499 gzipped
- `packages/vorma/.dist/core/vorma_client_wasm_bg.wasm`: 72,959 bytes, 33,345 gzipped

Latest checkpoint after the current frontend slices:

- `packages/vorma/.dist/core/_index.js`: 94,934 bytes, 22,265 gzipped (-433 bytes, -25
  gzipped)
- `packages/vorma/.dist/ui/react/react.js`: 8,985 bytes, 2,505 gzipped (-20 bytes, -20
  gzipped)
- `packages/vorma/.dist/ui/preact/preact.js`: 7,765 bytes, 2,193 gzipped (-20 bytes, -29
  gzipped)
- `packages/vorma/.dist/ui/solid/solid.js`: 9,048 bytes, 2,474 gzipped (-20 bytes, -25
  gzipped)
- `packages/vorma/.dist/core/vorma_client_wasm_bg.wasm`: unchanged from baseline.
- `packages/vorma/.dist/vite/vite.mjs`: 8,583 bytes, 2,915 gzipped.

## Open

- [x] Establish current frontend bundle-size baseline before frontend simplification
      edits.
- [x] Do a fresh read-only frontend entropy audit.
- [x] Record findings before editing.
- [ ] For each proposed code change, write the behavior-equivalence and bundle-size
      argument before implementing.
- [x] Run targeted frontend tests for any touched behavior.
- [x] Run `make ts-gate` after the frontend implementation is stable.
- [ ] Run `make e2e` if adapter, HMR, browser navigation, CSS/head, or dev-server behavior
      is touched.

## Findings

- [x] `packages/vorma/core/create_client_core.ts` contains two long active-route
      lifecycles: `run_active` and `run_active_with_url_guard`. They duplicate fetch
      result handling, build-skew reporting, redirect handling, payload decoding, module
      preparation, route preparation, publication, freshness marking, cleanup, work
      notification, and retry kicking. The intended difference is URL-path stability for
      revalidation, but an auditor has to diff two async state machines to prove that.
      Behavior-equivalence target: one lifecycle function with explicit guard hooks for
      the path-stability checks. Bundle expectation: flat or smaller because duplicated
      branches disappear. Before-edit equivalence argument: revalidation keeps its
      URL-without-hash guard after the fetch resolves and immediately before publish;
      navigation keeps its prepare-ready barrier for promoted prefetches and still
      resolves the navigation promise in the same finalization path; build-skew, redirect,
      payload decode, client-loader readiness, publication, freshness, work updates, and
      retry kicking become one shared lifecycle. Result: `run_active_with_url_guard` was
      deleted; `run_active` now owns the shared lifecycle and receives an explicit
      expected URL for revalidation. Targeted router/revalidation/ownership/core tests
      passed. `packages/vorma/.dist/core/_index.js` shrank by 999 bytes raw and 41 bytes
      gzipped; adapter entries and wasm stayed unchanged.

- [ ] `packages/vorma/core/create_client_core.ts` mixes many ownership areas inside one
      closure: browser history, scroll storage, route fetching, payload decoding, module
      imports, client-loader execution, prefetch promotion, revalidation retry state, API
      submissions, HMR, work indicators, boot, and public API methods. The "nine base
      facts" comment is useful, but the implementation still makes reviewers reconstruct
      the model by scanning local functions. Behavior-equivalence target: split only pure
      or state-owned chunks behind internal modules or clearly named local sections; do
      not change public exports or timing. Bundle expectation: flat if tsdown inlines the
      same code, smaller only where duplication is removed. Before-edit equivalence
      argument for the work-indicator slice: the timer/token controller is already a
      module-level subsystem with no access to route state, fetch state, submissions,
      browser history, or commits. Moving its public types and controller factory to an
      internal module preserves the same calls from `create_client_core.ts`: configure on
      boot, sync from derived work projections, expose `core.workIndicator`, and track
      external promises. Result so far: work-indicator public types and the timer/token
      controller factory now live in `packages/vorma/core/work_indicator.ts`. The
      `create_client_core.ts` route/fetch state machine still derives the active Vorma
      work projection and controls when the indicator is active. Before-edit equivalence
      argument for the scroll slice: scroll types, `apply_scroll`, and
      sessionStorage-backed scroll entries are a browser-scroll concern independent from
      route fetching and route publication. Moving them to an internal scroll module must
      preserve the same public exports from `create_client_core.ts`, the same
      `SCROLL_STORAGE_KEY`, the same `MAX_SCROLL_ENTRIES`, the same malformed-storage
      fallback behavior, the same hash decoding behavior, and the same `window.scrollTo`
      fallback. Result so far: scroll public types, `apply_scroll`, scroll-position
      capture, bounded sessionStorage entries, and lookup by history key now live in
      `packages/vorma/core/scroll.ts`; `create_client_core.ts` still owns when current
      scroll is saved and when restored scroll is applied.

- [ ] Client-loader prefetch logic is spread across `start_fetch`, `prepare_prefetch`, and
      `run_client_loaders`. The same concepts repeat: known matches, server state
      promises, existing prefetch retention, error-boundary truncation, later-loader
      aborts, and conversion into final client-loader data. Behavior-equivalence target:
      one internal client-loader execution/prefetch model that names "seed before server",
      "resolve after server", "retain promoted prefetch", and "truncate after first error"
      as explicit steps. Bundle expectation: flat or smaller if repeated logic collapses.
      Note: shared client-loader starter and shared abort-scope shapes were rejected
      because each grew the gzipped core bundle against the baseline. Before-edit
      equivalence argument for the naming slice: internal `cl_*` abbreviations in this
      path may be expanded to `client_loader_*` because they are local implementation
      names only. This must not change ordering, promise retention, abort propagation,
      server-state resolution, or public type names. Result so far: client-loader prefetch
      and result abbreviations were expanded in `create_client_core.ts`. Formatting, lint,
      core typecheck, typecheck project, and the targeted client-core Vitest files passed.
      Continuation equivalence argument: repeated internal trigger and result union shapes
      in this path may become named type aliases because aliases are erased from runtime
      output and preserve the same function signatures after type expansion.

- [x] Route-state projection is converted through several adjacent shapes:
      `DecodedPayload`, `DecodedRoute`, `RouteRecord`, `RouteMatchRecord`,
      `RouteRenderState`, `RouteState`, and adapter `DecomposedState`. The shapes are
      legitimate, but their ownership boundaries are not obvious from names alone.
      Behavior-equivalence target: keep the shapes, but move projection code into a small
      internal projection module with names that say which layer owns each shape. Bundle
      expectation: flat; this is mostly organization and naming. Before-edit equivalence
      argument: moving `HistoryPosition`, `RouteSnapshot`, `RouteRecord`,
      `RouteMatchRecord`, `RouteRenderEntry`, `RouteRenderState`, and the three route
      projection functions into one internal module preserves the same object fields, the
      same object-copy boundaries, the same call sites, and the same exported type names
      through `create_client_core.ts`. No timing, identity, or public API behavior should
      change. The change is accepted only if the rebuilt runtime bundle does not grow.
      Result: core route snapshot/record/render-state projection now lives in
      `packages/vorma/core/route_state_projection.ts`, while adapter decomposition stays
      in `ui_adapter_core.ts`. Targeted route/link tests passed. Core typecheck, core
      lint, TS formatting, and `make ts-build` passed. After rebuild,
      `packages/vorma/.dist/core/_index.js` is 940 bytes raw and 9 bytes gzipped below the
      baseline; adapter entries and wasm stayed unchanged.

- [ ] The three UI adapters duplicate the same public client surface: option splitting,
      boot render adaptation, route/work state hooks, view-data hooks, client-loader-data
      hooks, `useRouteSync`, link href construction, and returned client object shape.
      Some adapter-specific duplication is necessary because React, Preact, and Solid
      expose state differently. Behavior-equivalence target: extract only adapter-neutral
      pure wiring when it demonstrably does not add runtime weight; leave renderer
      identity, mount behavior, and state-retention code adapter-owned. Bundle
      expectation: must be measured per adapter before and after; no growth allowed.
      Before-edit equivalence argument for the work-state slice: the empty `WorkState`
      object shape is repeated in core and every adapter. A shared
      `create_empty_work_state()` helper may replace those object literals because each
      call must return the same fresh object shape, with no shared arrays and no changed
      update timing. The helper is internal and does not affect renderer identity. Result
      so far: renamed the adapter render/options type parameter from `App` to
      `RootOutletComponent`, and renamed each adapter's root-outlet wrapper away from
      `RootOutletApp` to `RootOutletComponent`. The generated app-config generics and test
      `App` aliases were left alone because those actually refer to app configuration.
      Work-state initialization now uses shared `create_empty_work_state()` in core,
      React, Preact, and Solid. Formatting, lint, core typecheck, typecheck project,
      `make ts-build`, and the three adapter Vitest files passed.

- [ ] `resolve_outlet_slot.ts` uses module-global stable component and error-boundary
      registries keyed by pattern. That is compact, but it hides identity ownership
      outside the client instance and forces reviewers to reason about cross-client and
      HMR identity globally. Behavior-equivalence target: make stable-wrapper ownership
      explicit at the adapter/client boundary if doing so preserves the tested no-remount
      behavior. Bundle expectation: flat or smaller; any change must pass the adapter HMR
      mount-count tests. Before-edit equivalence argument for the first slice: the stable
      wrapper entries store both `impl` and `holder.current`, but only `holder.current` is
      read after entry creation. Removing the unused `impl` field preserves wrapper
      identity and update behavior while reducing retained object shape. Result so far:
      outlet-slot and adapter HMR/mount tests passed. Core lint and TS formatting passed.
      After rebuild, `packages/vorma/.dist/core/_index.js` is 958 bytes raw and 13 bytes
      gzipped below the baseline; adapter entries and wasm stayed unchanged. Continuation
      equivalence argument: component and error-boundary stable-wrapper maps may share one
      generic wrapper-entry helper because the two current functions have identical
      ownership semantics: same key lookup, same holder mutation, same wrapper identity
      retention, and separate maps. Result so far: `resolve_outlet_slot.ts` now uses one
      generic stable-function helper while keeping separate component and error-boundary
      registries. Outlet-slot and adapter mount/HMR tests passed.

- [ ] Link behavior is concentrated in `make_link_props.ts`: public prop stripping,
      active/pending attributes, input-modality tracking, prefetch timers, event
      composition, pointer-down navigation, click navigation, and consumer-handler
      ordering. The behavior is well tested, but the code is dense enough that a reviewer
      must trace several concerns at once. Behavior-equivalence target: separate pure
      link-state/anchor-prop derivation from event-handler construction without changing
      consumer handler order or prefetch timing. Bundle expectation: flat or smaller.
      Before-edit equivalence argument for the first slice: click and pointerdown may
      share only the unmodified-primary-self-target predicate. Consumer handlers,
      `defaultPrevented` checks, pointer-type checks, prefetch timing, scroll saving, and
      navigation calls must keep their current order. Result so far: the shared
      unmodified-primary-self-target predicate passed the link tests, core typecheck, core
      lint, and TS formatting. After rebuild, `packages/vorma/.dist/core/_index.js` is 968
      bytes raw and 9 bytes gzipped below the baseline; adapter entries and wasm stayed
      unchanged. Rejected slice: internal links can semantically strip Vorma-owned props
      and composed event props in one pass instead of two, but the exact rebuild grew the
      gzipped core bundle against the baseline. The two-pass form stays. Rejected slice:
      moving adapter link-attribute matching into its own module was behavior-equivalent
      under targeted tests, but the exact rebuild grew
      `packages/vorma/.dist/core/_index.js` gzip by 4 bytes over the original baseline.
      The in-file form stays. Continuation equivalence argument: local `*_pf`
      abbreviations and private all-caps key-set constants may be renamed to
      intention-revealing snake-case names because they are private implementation
      identifiers only. Event handler order, prop stripping sets, prefetch timer behavior,
      active/pending attribute behavior, and public prop names must stay unchanged. Result
      so far: the link helper no longer uses the `*_pf` abbreviations or private all-caps
      key-set constants; link tests, core typecheck, lint, formatting, and `make ts-build`
      passed.

- [x] Frontend internals still had private all-caps constants outside public contract
      surfaces: revalidation result sentinels, URL pattern runes, the Vite `/@fs/` prefix,
      and client-WASM matcher statuses. Behavior-equivalence target: rename only the
      private identifiers, preserving exported constants and every string/status value.
      Bundle expectation: flat or smaller. Result: the private constants now use
      snake-case internal names; stale-name search, formatting, lint, core typecheck,
      targeted client-core/link/WASM tests, and `make ts-build` passed. The exact latest
      package checkpoint is below the original raw and gzipped diagnostic baseline.

- [x] `packages/vorma/core/types.ts` contains dense public type algebra for views,
      resources, navigation, links, API client args, decorators, client loaders, and view
      props. This does not affect runtime bundle size, but it is hard to audit as one
      file. Behavior-equivalence target: type-only organization by concept while keeping
      exported type names and generated declaration semantics stable. Bundle expectation:
      no runtime effect. Before-edit equivalence argument for the first slice: internal
      resource conditional types may rename inferred type variables from `Route` to
      `Resource` without changing any exported type name, any conditional structure, or
      any runtime output. Result so far: core and tests typechecks passed. Continuation
      equivalence argument: the remaining internal resource placeholder name `Act` may be
      renamed to `Resource` because it is a type-parameter name only; the conditional
      types, inferred fields, exported names, and runtime output stay identical. Result:
      the remaining `Act` placeholders are now `Resource`; core and typecheck project
      typechecks passed. Before-edit equivalence argument for the route/link type slice:
      route state, route transition hook, and link-base public types may move into a
      type-only module while `packages/vorma/core/types.ts` continues to re-export the
      same public names. Existing imports from `types.ts`, generated declaration names,
      runtime imports, and emitted browser JavaScript must stay stable. Result:
      `packages/vorma/core/route_types.ts` now owns route state, route transition hook,
      and link-base public types; `types.ts` re-exports the same names; core and typecheck
      project typechecks passed. `make ts-gate` passed after this batch, and exact browser
      runtime sizes stayed at the latest checkpoint. `make e2e` passed after this batch.
      Continuation equivalence argument: generated app-config primitives and extractors,
      view/component/client-loader public types, and resource/API-client public types may
      move into separate type-only modules while `types.ts` keeps re-exporting the same
      public names. Runtime imports must stay erased, generated declaration names must
      stay available through `vorma/__internal`, and the browser JavaScript output must
      not grow. Result: generated app-config primitives and extractors now live in
      `packages/vorma/core/generated_contract_types.ts`, view/component/client-loader
      public types now live in `packages/vorma/core/view_types.ts`, resource/API-client
      public types now live in `packages/vorma/core/resource_types.ts`, and `types.ts`
      remains the public barrel. Formatting, lint, core typecheck, typecheck project, and
      `make ts-build` passed.

- [x] `packages/vorma/vite/vite.ts` combines dev RPC, Vite config generation, public URL
      rewriting, CSS public URL rewriting, HMR self-accept injection, and Vite restart
      control. The file is not large, but it crosses several ownership boundaries.
      Behavior-equivalence target: split pure public-URL rewriting and HMR-injection
      helpers from plugin lifecycle code only if output snapshots prove the generated Vite
      config and transformed code are unchanged. Bundle expectation: not applicable to
      browser runtime; node plugin output must not grow materially. Before-edit
      equivalence argument for the public-URL slice: the Vite plugin lifecycle must still
      fetch config through the same tokenized RPC endpoint, produce the same dev/build
      config fields, rewrite `vormaPublicUrl(...)` calls to the same hashed string
      literals, rewrite CSS `url("@public/...")` references to the same quoted hashed
      URLs, mark the same resolved public URLs as external, append the same HMR preamble
      only to resolved view module IDs, and expose the same token-gated config-change
      control endpoint. Moving the pure public-URL parsing/rewrite functions is accepted
      only after direct plugin tests prove those generated outputs. Result: plugin
      contract constants and RPC/config types now live in
      `packages/vorma/vite/plugin_contract.ts`, public URL parsing and JS/CSS rewrite
      logic now lives in `packages/vorma/vite/public_url_resolution.ts`, and
      `packages/vorma/vite/vite.test.ts` proves dev/build config output, JS public URL
      rewriting, CSS public URL rewriting plus external marking, HMR injection, and the
      token-gated config-change endpoint. Browser runtime bundle sizes stayed exactly at
      the previous checkpoint. The Node-side Vite plugin output measured 8,583 bytes raw
      and 2,915 bytes gzipped after the split.

## Next

- Continue with the client-loader prefetch model only where a refactor is clearly
  behavior-equivalent and does not grow final Vite output.

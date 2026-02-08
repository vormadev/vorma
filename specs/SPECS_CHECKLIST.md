# Vorma Specs Checklist

This checklist is focused on **Vorma**. Checked items indicate a first
conformance-oriented draft spec exists; they may still need
expansion/refinement.

Out of scope for now:

- Wave internals (except Vorma/Wave integration boundaries)
- Individual `kit/*` package specs (except where they define Vorma-visible
  behavior or Vorma interop contracts)

## 0) Foundation

- [x] Vorma domain model and terminology
- [x] Spec template and RFC/change process
- [x] Public API surface map (Go + TypeScript)

## 1) Backend Runtime (Go) Spec Areas

- [x] App lifecycle and initialization contract
- [x] Configuration schema and validation contract (`Vorma` config block)
- [x] Router model and route pattern semantics (nested, params, splat, index
      rules)
- [x] Loader contract (registration, context, execution, return-value
      expectations)
- [x] Action contract (query/mutation split, method support, input parsing
      rules)
- [x] Error model (loader errors, client-safe vs server-private messaging)
- [x] Redirect and response propagation behavior
- [x] Route data assembly model (matching, ordering, parallelism, error cut-off
      behavior)
- [x] SSR template integration contract (required template data fields)
- [x] Head element integration contract (default head, dedupe, merge behavior)
- [x] Runtime reload endpoints contract (`/__vorma/reload-routes`,
      `/__vorma/reload-template`)
- [x] Runtime state/concurrency model (lock ownership and thread safety
      expectations)

## 2) Over-the-Wire Contract Spec Areas

- [x] HTML document response contract for route requests
- [x] JSON route-data request/response contract (`vorma_json` flow)
- [x] Header contract (`X-Vorma-Build-Id`, `X-Vorma-Reload`, redirect-related
      headers)
- [x] Redirect protocol contract (soft vs hard redirect semantics)
- [x] SSR bootstrap payload contract (global symbol keys, field meanings)
- [x] Route manifest contract (public JSON shape and semantics)
- [x] Cache-control and freshness contract (dev vs prod behavior)
- [x] Wire-compat guarantees and deprecation policy

## 3) Build/Dev Pipeline Spec Areas

- [x] Build architecture and process model (dev server process vs app process)
- [x] Development build flow and watch behavior
- [x] Fast route-rebuild flow and fallback behavior
- [x] Production build flow and phase ordering
- [x] Client route-definition DSL contract (`route(...)`) and static-analysis
      constraints
- [x] Route artifact generation contract (what is generated, when, and why)
- [x] Paths file contract (`stage 1` and `stage 2` schemas + lifecycle)
- [x] Build ID generation and invalidation rules
- [x] Route manifest generation and hashing rules
- [x] TypeScript generation contract (`vorma.gen` types/API guarantees)
- [x] Vite integration contract (plugin behavior, rollup inputs, dedupe
      behavior)
- [x] Output layout/naming contract (public/private dist, hashing prefixes)

## 4) Frontend Runtime Spec Areas (`internal/framework/_typescript`)

- [x] Client bootstrap lifecycle (`initClient`) and required SSR prerequisites
- [x] Global client state model (symbol namespace and invariant fields)
- [x] Navigation state machine
      (navigate/prefetch/revalidate/redirect/browser-history)
- [x] Client loader lifecycle (registration, execution, cancellation, error
      semantics)
- [x] Component/module loading contract and error-boundary selection rules
- [x] Link interaction contract (click interception, prefetch triggers,
      hash-only behavior)
- [x] URL/path resolution contract (params, splats, explicit index segment
      behavior)
- [x] History + scroll restoration contract
- [x] Head element reconciliation contract
- [x] Asset preloading/application contract (modulepreload + CSS bundle
      behavior)
- [x] HMR behavior contract and route-data refresh rules
- [x] Frontend event contract (`vorma:route-change`, `vorma:status`,
      `vorma:build-id`, `vorma:location`)
- [x] UI adapter contract parity (React/Preact/Solid entrypoints and expected
      behavior)
- [x] Global loading indicator helper contract (`setupGlobalLoadingIndicator`
      include filters, delay guards, cleanup semantics)
- [x] UI adapter helper/API shape contract (typed helper reactive return shapes,
      client-loader registration behavior, typed-link merge/URL composition,
      listener idempotency, outlet remount identity)

## 5) Cross-Cutting Spec Areas

- [x] Vorma↔Kit interop contract map (which kit behaviors are normative for
      Vorma)
- [x] Backend interop contracts inherited from kit (`mux`, `response`,
      `headels`, `matcher`, `validate`)
- [x] Frontend interop contracts inherited from `vorma/kit/*` (`matcher`, `url`,
      `json`, `debounce`, `listeners`)
- [x] Change-management policy for upstream kit behavior changes affecting Vorma
- [x] Security model (CSP expectations, safe HTML boundaries, input/body
      handling)
- [x] Performance model and target budgets (build/rebuild/navigation critical
      paths)
- [x] Observability/debug contract (logs, warnings, diagnostics hooks)
- [x] Testing strategy contract (unit/integration/e2e expectations by layer)

## 6) Conformance Assets and Traceability

- [x] Canonical requirement traceability matrix
      (`/Users/sjc/__code/river/specs/VORMA_TRACEABILITY_MATRIX.md`)
- [x] Concrete backend/wire/build conformance suite map (requirement
      group -> suite name -> test file path)
- [x] Backend/wire/build Go conformance suite skeletons
      (`/Users/sjc/__code/river/conformance/*`)
- [x] Conformance issue backlog
      (`/Users/sjc/__code/river/specs/VORMA_CONFORMANCE_ISSUES.md`)
- [x] Legacy Vorma test truth-mining pass (reviewed pre-conformance tests and
      promoted missing normative behavior into specs/conformance)
- [x] Source-driven runtime/build audit pass (line-by-line review of Vorma +
      inherited Wave control-plane behavior promoted into normative spec text)
- [x] Precision normalization pass (legacy/source timing constants, redirect
      caps, skip guards, and kit precedence semantics made explicit)
- [x] Deep-pass normalization (remaining legacy suites + runtime/build edge
      semantics promoted: stale-build reload URL preservation, loader
      placeholder alignment, cancellation unwrap rules, selective cleanup/hash
      specifics, and touch/prefetch/status nuances)
- [x] Adapter source deep-pass normalization (React/Preact/Solid
      root-outlet/listener/remount invariants and typed helper contracts
      promoted into frontend/API specs)
- [x] Vite plugin deep-pass normalization (transform fallback rules,
      method-agnostic invalidation endpoint/cache reset, and serve/build
      config-merge semantics promoted into build spec)
- [x] Proxy/dev/head-edge normalization (response-proxy cookie/redirect winner
      semantics, dev-only reload endpoint inertness, and whitespace-tolerant
      head-marker lookup promoted into backend/wire/frontend specs)
- [x] Legacy+source closure pass (same-target revalidation upgrade + clearAll
      abort semantics, title/route-change ordering, deployment request
      propagation on submit/revalidate, dev script variant selection, and
      explicit mux middleware/fallback interop constraints)
- [x] Wave tooling watcher/control-plane deep-pass normalization (default
      ignore/watched sets, watch-root normalization, refresh-server
      endpoint/shutdown guards, rebuilding-signal gating, batch
      run-on-change-only behavior, lock-preserving static cleanup, hash-input
      collision guard, and config-validation tightening)
- [x] Backend route-registry closure pass (explicit server-only-route
      placeholder injection semantics and traceability promotion from runtime
      `route_registry` behavior)
- [x] Backend init/loader bootstrap closure pass (active-mode stage artifact
      selection and nested-router client-pattern auto-registration semantics
      promoted into backend spec)
- [x] Frontend runtime closure pass (submit non-throw envelope semantics, submit
      loading-indicator opt-out behavior, redirect-handoff cleanup discipline,
      head fingerprint determinism/duplicate-winner rules, and proxy-safe
      error-export fallback semantics promoted into frontend spec +
      traceability)
- [x] Backend runtime metadata/head closure pass (head-dedupe bootstrap option
      behavior, deps/css ordering + dedupe semantics, SSR script hash integrity
      contract, and route-reload cache invalidation semantics promoted into
      backend spec + traceability)
- [x] Kit interop legacy-test closure pass (`kit/mux` + `kit/response`
      mount-root/task-context/middleware-halt/redirect-edge semantics promoted
      into Vorma interop spec)
- [x] Kit response header-operation precision pass (`kit/response`
      set-vs-add merge precedence across proxy boundaries and apply-time
      writer replacement/append semantics plus response-helper
      nil-request/client-redirect guard behavior promoted into interop spec)
- [x] Kit head-elements precision pass (`kit/headels` nil-element safety,
      duplicate class last-wins boundaries, content-distinct preservation, and
      boolean-rule strictness promoted into interop spec)
- [x] Kit validate URL-parser precision pass (`kit/validate` destination-shape
      guard, strict scalar parse failures, pointer empty-value nil semantics,
      non-pointer empty-value preservation, and repeated-key slice ordering
      with empty-filter behavior promoted into interop spec)
- [x] Kit tasks-runtime precision pass (`kit/tasks` type-safe
      `RunWithAnyInput` mismatch error semantics and per-request task+input
      memoization inheritance codified in interop task-execution contracts)
- [x] Nested-task result-shape precision pass (`mux.RunNestedTasks`
      index-aligned slice/map/proxy slot invariants and no-handler slot
      semantics promoted into interop spec)
- [x] Frontend client-loader payload/reuse closure pass (`serverDataPromise`
      resolved-shape + fallback object semantics and running-loader
      reuse/no-double-invoke contract promoted into frontend spec +
      traceability)
- [x] Frontend skip synthesis closure pass (skip-mode module-map completeness,
      server/non-server loader-data projection rules, and skip-path
      client-loader reuse semantics promoted into frontend spec + traceability)
- [x] Frontend skip guard source audit pass (outermost-vs-innermost loader guard
      discrepancy surfaced as open conformance issue `VCI-016`)
- [x] Frontend prefetch-lifecycle closure pass (completed-prefetch `stop()`
      cleanup/retire semantics promoted into frontend link contract +
      traceability)
- [x] Backend dev-reload dispatch/metadata closure pass (path-based
      method-agnostic reload dispatch and stage-one build/manifest metadata
      refresh semantics promoted into backend spec + traceability)
- [x] Frontend rerender scroll-payload closure pass (`__scrollState` selection
      rules for hash/scrollToTop/POP restore-state promoted into rendering
      spec + traceability)
- [x] Frontend init/ctx/asset precision closure pass (bootstrap module-map seed
      normalization defaults/filtering, non-blocking manifest fetch startup
      semantics, router-data fallback defaults, and empty-dev-url asset-base
      fallback promoted into frontend spec + traceability)
- [x] Frontend redirect-handoff option propagation closure pass (soft-redirect
      execution now explicitly preserves originating navigation
      `state`/`replace`/`scrollToTop` semantics in fetch contract +
      traceability)
- [x] Frontend navigation status/introspection closure pass (`getStatus()`
      immediate pre-debounce truth semantics plus `navigationStateManager`
      introspection/remove cleanup invariants promoted into frontend spec +
      traceability)
- [x] Frontend loading-continuity precision closure pass (submit→revalidate
      handoff, redirect-chain handoff, and waiting-phase status continuity
      bounds made explicit in navigation status contract)
- [x] Frontend global-loading-indicator bootstrap closure pass (setup-time
      status snapshot behavior promoted into helper contract + traceability)
- [x] Frontend redirect/event/loading-helper precision pass (submit no-response
      `unknown` envelope semantics, location-event unchanged-key suppression,
      and `include: []` loading-indicator disable semantics promoted into
      frontend spec + traceability)
- [x] Frontend redirect-priority edge-case audit pass (invalid-highest-signal
      fallback ambiguity documented as open conformance issue `VCI-017` instead
      of enshrining potentially buggy behavior)
- [x] Backend route-registry nil-path safety pass (route-sync nil-map handling
      and non-crashing empty-routing behavior promoted into backend runtime
      spec + traceability)
- [x] Backend route-sync cache-invalidation pass (`gmpd` snapshot invalidation
      semantics promoted so post-sync loader responses cannot reuse stale
      import/export/deps cache state)
- [x] Backend SSR/reload concurrency audit pass (potential unlocked SSR field
      reads during reload documented as open conformance issue `VCI-018`)
- [x] Frontend rendering-history intent pass (explicit no-history-mutation
      contract for revalidation/prefetch commits promoted into rendering spec +
      traceability)
- [x] Backend constructor-callback default pass (omitted app callbacks now
      explicitly specified as safe no-op/empty-map defaults in init spec +
      traceability)
- [x] Public API drift/signature pass (root Go surface re-verified against
      `/Users/sjc/__code/river/vorma.go`; constructor and alias signature
      contracts promoted into API spec)
- [x] Frontend accessor/scroll-helper closure pass (`getHistoryInstance()`
      singleton/surface contract and `__applyScrollState(undefined)`
      hash-fallback/no-op semantics promoted into runtime spec + traceability)
- [x] Frontend head-elements legacy parity pass (legacy head test corpus
      re-checked against `FE-REN-009..FE-REN-018`; no additional normative gaps
      found in this pass)
- [x] Frontend location-accessor precision pass (`getLocation().state` source
      pinned to runtime history instance state to prevent silent accessor drift
      during history refactors)
- [x] Backend loader cache-control precision pass (default-vs-preserve
      `Cache-Control` behavior promoted from source into common response
      contract as `BR-RESP-003` + traceability)
- [x] Frontend prefetch-upgrade/build-id ordering precision pass
      (prefetch-upgrade propagation of `state`/`replace`/`scrollToTop` and
      build-id update-before-event-dispatch semantics promoted into frontend
      fetch/navigation contract)
- [x] Frontend hash-only-link default-behavior pass (explicit
      non-`preventDefault` requirement for hash-only helper clicks promoted as
      `FE-LINK-011` + traceability)
- [x] Backend loaders-handler nil-router guard pass (`GetLoadersHandler(nil)`
      fail-fast panic behavior promoted into backend init contract as
      `BR-INIT-009` + traceability)
- [x] Frontend prefetch/revalidate URL-precision pass (explicit non-default
      prefetch-delay override semantics as `FE-LINK-012` and revalidate
      current-URL query-preservation semantics as `FE-FETCH-022`, with
      scenario/traceability promotion)
- [x] Frontend legacy-suite/source closure pass
      (`internal/framework/_typescript/client/src` non-conformance suites +
      runtime source fully re-read; no additional normative gaps identified
      beyond newly promoted precision contracts)
- [x] Backend route-data cache closure pass (tuple-boundary cache-key uniqueness
      and per-app cache-scope isolation promoted as `BR-LOAD-014/015`; current
      implementation risks tracked as open issues `VCI-020/021`)
- [x] Backend stale-JSON precedence closure pass (explicit stale-build sentinel
      precedence over route-match/not-found promoted as `BR-JSON-004` +
      traceability)
- [x] Wire stale-JSON precedence closure pass (query-level stale token behavior
      for unmatched routes promoted as `WIRE-Q-005` + traceability)
- [x] Public API method-surface inventory pass (explicit `Vorma` alias method
      inventory + `Loaders`/`Actions` wrapper method contracts promoted as
      `API-GO-011..013`)
- [x] Backend snapshot-alias safety pass (non-aliasing expectations for
      snapshot-style accessors like
      `GetPathsSnapshot`/`GetClientEntryDeps`/`GetDepToCSSBundleMap` promoted as
      `BR-CONC-003`; current mutable-alias implementation tracked as open issue
      `VCI-022`)
- [x] Backend init accessor semantics pass (`ServerAddr` derivation and
      constructor TS-payload getter passthrough promoted as `BR-INIT-010/011` +
      traceability)
- [x] Wire dev-reload semantics pass (method-agnostic dispatch and non-dev
      inertness promoted as `WIRE-DEV-004/005` + traceability)
- [x] Backend actions accessor immutability pass (`Actions().SupportedMethods()`
      non-aliasing contract promoted as `BR-ACT-005`; current mutable-map
      implementation tracked as open issue `VCI-023`)
- [x] Public API collection-ownership pass (map/slice-return accessor
      immutability intent codified as `API-GO-014`, aligned with open backend
      mutable-alias issues `VCI-022/023`)
- [x] Loader build-header scope refinement pass (`BR-RESP-001` / `WIRE-HDR-001`
      narrowed to non-control-path loader outcomes; dev reload endpoints
      explicitly governed by `BR-DEV-*` / `WIRE-DEV-*`)
- [x] Loader-error/HMR source-precision pass (backend loader-cutoff truncation
      scope vs `deps` retention promoted as `BR-ERR-005`; caller-vs-local
      hot-context fallback promoted as `FE-HMR-005` with traceability scenario
      additions)
- [x] Route-DSL signature strictness pass (`route(...)` first-arg pattern +
      required module-arg validity promoted as `BUILD-ROUTE-005`; current parser
      under-validation behavior tracked as open issue `VCI-024`)
- [x] Dev build-id prefix precision pass (standard dev build-id prefix `dev_`
      and fast-route rebuild prefix `dev_fast_` codified as `BUILD-ART-007` +
      traceability)
- [x] Submit non-`Error` fallback precision pass (explicit `submit()` catch
      fallback envelope for thrown non-`Error` values codified as
      `FE-FETCH-023` + scenario/traceability linkage)
- [x] Kit-response redirect-status interop precision pass (`clientRedirect`
      default-success/explicit-status preservation and `serverRedirect`
      error-status no-op behavior codified in kit interop contract)
- [x] Backend head-dedupe instance-isolation audit pass (cross-app head dedupe
      state contamination risk surfaced as open issue `VCI-025`; `BR-INIT-007`
      wording tightened for explicit app-instance isolation intent)
- [x] Wire deployment-id bootstrap gate precision pass
      (`WIRE-HTML-004`/`WRC-HTML-004` tightened for
      disabled/enabled+set/enabled+unset env combinations; missing traceability
      row backfilled)
- [x] Frontend history-baseline failure-path precision pass
      (`FE-SCROLL-009`/`FEC-SCROLL-007` codify that failed cross-document POP
      navigation MUST NOT commit a new last-known history baseline before a
      succeeding update)
- [x] Route-parser duplicate-collision precision pass
      (`BUILD-ROUTE-006`/`BDC-ROUTE-006` codify current last-write-wins behavior
      for duplicate patterns, with policy-risk surfaced as open issue `VCI-026`)
- [x] Watch reload-endpoint failure-classification precision pass
      (`BUILD-WATCH-012`/`BDC-WATCH-012` codify timeout/transport/non-200
      endpoint outcomes as callback failures that must trigger documented
      restart fallback semantics)
- [x] Template-watch injection precondition precision pass (`BUILD-WATCH-005`
      now explicitly includes `HTMLTemplateLocation`+private-static-dir gating,
      with `BDC-WATCH-005` added and traceability marked missing until coverage
      is added)
- [x] Backend terminal-outcome precedence pass (`BR-HEAD-004`/`BRC-HEAD-004`
      codify that terminal loader outcomes MUST NOT be overwritten by concurrent
      default-head callback failures; current runtime mismatch surfaced as open
      issue `VCI-028`)
- [x] Backend proxy-success status preservation pass
      (`BR-PROXY-006`/`BRC-PROXY-006` codify that non-redirect/non-error proxy
      success statuses remain in effect while normal route payload rendering
      continues)
- [x] Frontend navigation-introspection snapshot pass
      (`FE-NAV-018`/`FEC-NAV-013` extended with detached-map non-aliasing and
      deterministic same-URL slot-collision projection semantics)
- [x] Focus-revalidation listener inheritance pass (`FE-HMR-004`/`FEC-HMR-003` +
      kit interop section 6.4 now explicitly codify focus+visibility trigger
      sources, 30ms listener debounce inheritance, and cleanup-listener removal
      semantics)
- [x] Matcher edge-semantics precision pass (`BR-LOAD-016/017` + `FE-CL-012`
      added for trailing-slash/catch-all eligibility, gap-tolerant full-match
      validity, and deterministic longest-prefix client partial-match probing;
      matrix rows added as `missing` pending conformance suite expansion)
- [x] Task-middleware fan-out precision pass (kit interop section 5.1 now
      explicitly codifies no sibling short-circuit across applicable task
      middlewares, post-fan-out merge/halt decision point, and middleware
      Go-error precedence over proxy-set statuses)
- [x] Middleware predicate/tasks-context inheritance pass (kit interop section
      5.1 now explicitly codifies `MiddlewareOptions.If` skip semantics for
      HTTP/task middleware and regular-handler task-context availability when
      task middleware forces mux slow-path execution)
- [x] Matcher parse-normalization/root-candidate precision pass (kit interop
      section 5.1 now explicitly codifies repeated-slash normalization,
      root/trailing-empty-segment preservation, UTF-8 segment preservation,
      duplicate-param-name last-write binding, and root-parent vs
      false-positive full-match boundaries)
- [x] Frontend/backend/build closure precision pass (`FE-NAV-005` prefetch-cache
      retention semantics, `FE-COMP-001/002` component-import dedupe + slot
      alignment/null-slot behavior, `FE-HMR-002/003` query-stripped pathname
      normalization for update/registration matching, `BR-ERR-005` loader-head
      cutoff truncation semantics)
- [x] Dev watcher debounce/dedupe precision pass (`BUILD-EVT-001` now
      explicitly codifies non-overlapping debounced callback sequencing with
      queued follow-up flushes plus non-empty-key guardrails for any
      watched-pattern dedupe; current empty-key collapse behavior surfaced as
      open issue `VCI-029`)
- [x] Backend init snapshot-replacement precision pass (`BR-INIT-006` now
      explicitly requires init-time route-path snapshot replacement (not
      additive merge) and current re-init stale-entry behavior is surfaced as
      open issue `VCI-030`)
- [x] Frontend head/global-accessor strictness pass (`FE-CTX-001` now
      explicitly codifies `__getVormaClientGlobal().get/set` as direct
      symbol-store pass-through wrappers; `FE-REN-009/012` now explicitly
      codify reconciliation idempotence and managed-span non-element cleanup,
      inferred from legacy `head_elements` + `vorma_ctx` tests and source)
- [x] Backend dev route-reload replacement-snapshot pass (`BR-DEV-001` and
      `BRC-DEV-001` now explicitly codify that reload-sync replaces client-route
      metadata (removed client routes must drop out after reload), while
      documented server-task placeholder retention remains governed by
      `BR-LOAD-008`)
- [x] Kit mux interop default/duplicate-registration pass (kit interop section
      5.1 now explicitly codifies inherited nested-router default rune/segment
      values and duplicate nested-pattern panic compatibility, requiring Vorma's
      register-if-needed guard behavior)
- [x] TS generation rune-source + rollup-input determinism pass (`BUILD-ART-004`
      now explicitly codifies loader-entry param/splat typing must use loader
      rune settings (including client-defined loader paths), `BUILD-VITE-001`
      now explicitly codifies deduplicated deterministic sorted rollup-input
      emission; current mixed-rune client-path typing behavior is surfaced as
      open issue `VCI-050`; traceability rows for `BUILD-ART-004` and
      `BUILD-VITE-001` are now marked `missing` pending stronger scenario
      coverage)
- [x] Revalidation stale-origin no-commit boundary pass (`FE-NAV-009` now
      explicitly codifies stale-payload no-commit side-effect boundaries
      including module-map/CSS/title/head/route-state/event semantics; current
      late-guard behavior is surfaced as open issue `VCI-032`; `FE-NAV-009`
      traceability row is marked `missing` pending stricter scenario coverage)
- [x] Prefetch override-target coherence pass (`FE-LINK-013` + `FEC-LINK-011`
      now codify that prefetch `start`/`stop`/click-upgrade must share one
      effective URL key when search/hash overrides are provided; current stop
      key mismatch behavior is surfaced as open issue `VCI-033`; `FE-LINK-013`
      traceability row is marked `missing` pending scenario/test coverage)
- [x] RunOnChangeOnly callback-action propagation pass (`BUILD-EVT-010` and
      `BDC-EVT-010`, plus batch-wide `BUILD-EVT-013`/`BDC-EVT-013`, now
      explicitly codify that callback refresh actions must still be honored even
      when standard build is skipped; current early-return action-drop behavior
      is surfaced as open issue `VCI-034`; `BUILD-EVT-010`/`BUILD-EVT-013`
      traceability rows are marked `missing` pending stricter event-suite
      coverage)
- [x] Prefetch handler lifecycle strictness pass (`FE-LINK-014/015` +
      `FEC-LINK-012/013` now explicitly codify both upgraded-stop safety
      (`stop()` MUST NOT abort upgraded navigation) and per-handler
      single-shot begin/fetch lifecycle semantics; traceability rows are added
      as `missing` pending strict scenario coverage)
- [x] Navigation-failure snapshot-preservation pass (`FE-FETCH-024` +
      `FEC-FETCH-019` now explicitly codify failure invariants from legacy
      error tests: no location/title/head/route-state partial apply on failed
      destination and no poisoning of subsequent successful navigation;
      traceability row is added as `missing` pending strict scenario coverage)
- [x] Kit mux panic-recovery interop pass (Vorma↔Kit interop section 5.1 now
      explicitly codifies inherited compatibility for recovery-capable HTTP
      middleware wrapping downstream handlers and recovering handler panics
      before response finalization)
- [x] Kit mux nil-task-handler robustness pass (Vorma↔Kit interop section 5.1
      now explicitly codifies inherited nil task-handler safety as
      internal-server-error class response rather than process panic)
- [x] RunOnChangeOnly hook-timing closure pass (`BUILD-EVT-010/013` and
      `BDC-EVT-010/013` now explicitly codify that run-on-change-only skips
      standard build work but still executes supported callback-hook timing
      phases; current early-return callback-phase drop is surfaced as open issue
      `VCI-035`)
- [x] RunOnChangeOnly schema-vs-runtime timing closure pass
      (`BUILD-SCHEMA-005`/`BDC-SCHEMA-005` now explicitly codify that non-`pre`
      timing restriction applies only to command hooks, while callback hooks
      remain validation-legal for non-`pre` timings)
- [x] Cycle-vite reload-signal exclusivity closure pass
      (`BUILD-DEV-012`/`BDC-DEV-012` added to codify cycle-vite path
      reload-trigger exclusivity vs non-cycle Wave refresh signaling; existing
      broadcast mismatch remains tracked as open issue `VCI-014`)
- [x] Direct-link outcome callback/cleanup closure pass (`FE-LINK-016` +
      `FEC-LINK-014` now explicitly codify `__makeLinkOnClickFn` outcome
      branches from legacy/source behavior: aborted path cleanup without render
      callbacks, redirect path callback/cleanup ordering, and success-path
      conditional process semantics; traceability row added as `missing`
      pending strict scenario coverage)
- [x] Dev readiness retry-envelope precision pass (`BUILD-DEV-013` +
      `BDC-DEV-013` now explicitly codify readiness helper attempt cap,
      per-request timeout, linear backoff schedule, cumulative timeout budget,
      and early-success-on-200 short-circuit semantics inferred from
      `wave/tooling/devserver.go`; traceability row added as `missing` pending
      strict scenario coverage)
- [x] Link prefetch-mode gate closure pass (`FE-LINK-017` + `FEC-LINK-015`
      now explicitly codify that link prefetch lifecycle is enabled only for
      explicit `prefetch="intent"` and non-intent/absent cases must use
      click-only direct-navigation flow; companion API surface contract
      `API-UI-008` added for adapter-level prefetch gating semantics;
      traceability row added as `missing` pending strict scenario coverage)
- [x] Browser-phase signaling precision pass (`BUILD-EVT-014/015/016` +
      `BDC-EVT-014/015/016` now explicitly codify browser-phase
      short-circuit/no-op boundaries and CSS payload shape/order semantics:
      successful Vite invalidate short-circuit, non-Vite invalidate fallback
      conversion, critical/normal CSS payload sequencing/shape, builder-missing
      CSS emission guard, and non-browser-mode browser-phase no-op; traceability
      rows added as `missing` pending strict scenario coverage)
- [x] History state pass-through precision pass (`FE-REN-021` +
      `FEC-REN-015` now explicitly codify that navigate-intent rerender history
      push/replace calls must forward current commit `state` exactly and must
      not leak stale state from prior commits; traceability row added as
      `missing` pending strict scenario coverage)
- [x] Fetch module-preload source-selection precision pass (`FE-FETCH-025` +
      `FEC-FETCH-020` now explicitly codify build-mode source selection for
      preload candidates (`importURLs` in dev vs `deps` in prod), plus falsy
      candidate filtering and per-URL dedupe semantics; traceability row added
      as `missing` pending strict scenario coverage)
- [x] CSS preload failure-tolerance precision pass (`FE-ASSET-008` +
      `FEC-ASSET-006` now explicitly codify non-fatal navigation behavior when
      CSS preload promises reject during waiting phase; runtime must continue to
      render/commit while surfacing diagnostics; traceability row added as
      `missing` pending strict scenario coverage)
- [x] Backend nil-loader warning diagnostics pass (`BR-LOAD-018` +
      `BRC-LOAD-018` now explicitly codify warning-level diagnostics when a
      loader returns nil-like payload without error; warning must include matched
      pattern context; traceability row added as `missing` pending strict
      scenario coverage)
- [x] Backend dev-vs-prod import URL source pass (`BR-LOAD-019` +
      `BRC-LOAD-019` now explicitly codify mode-dependent `importURLs`
      derivation (`srcPath` in dev, `outPath` in non-dev) with preserved index
      alignment and leading-slash formatting; traceability row added as
      `missing` pending strict scenario coverage)
- [x] Missing-title no-op rendering pass (`FE-REN-022` + `FEC-REN-016` now
      explicitly codify that rerender commits with `title=undefined` must keep
      previously committed `document.title` unchanged; traceability row added as
      `missing` pending strict scenario coverage)
- [x] Global-state commit-field precision pass (`FE-REN-002`/`FEC-REN-002` now
      explicitly enumerate the full commit-critical client-global field set and
      require all fields to reflect current commit payload by route-change
      dispatch time, with no stale prior-commit residue)
- [x] Prefetch-redirect suppression precision pass (`FE-FETCH-026` +
      `FEC-FETCH-021` now explicitly codify source-observed behavior that
      pure-prefetch redirect outcomes are retired without redirect effectuation;
      traceability row added as `missing` pending strict scenario coverage)
- [x] Route-parser unresolved-diagnostic context precision pass
      (`BUILD-ROUTE-007` + `BDC-ROUTE-007` now explicitly codify unresolved
      route warning context payload expectations (pattern/file/expression/reason)
      and exclusion semantics for unresolved entries while resolvable entries
      continue; traceability row added as `missing` pending strict scenario
      coverage)
- [x] Dev config-reload failure continuity pass (`BUILD-DEV-015` +
      `BDC-DEV-015` now explicitly codify non-terminal reload-failure behavior:
      failed config parse/validation logs and the dev loop continues under the
      prior active config; traceability row added as `missing` pending strict
      scenario coverage)
- [x] Route-definitions transform-fatality pass (`BUILD-ROUTE-008` +
      `BDC-ROUTE-008` now explicitly codify that route DSL extraction must fail
      the build on transform/parsing-invalid route-definition source (no partial
      route-map emission); traceability row added as `missing` pending strict
      scenario coverage)
- [x] Backend idempotent pattern-registration API pass (`BR-LOAD-020` +
      `BRC-LOAD-020` now explicitly codify `RegisterPatternIfNeeded` semantics:
      missing pattern registration as no-handler plus duplicate-safe no-op on
      already-registered patterns; traceability row added as `missing` pending
      strict scenario coverage)
- [x] Skip-synthesized envelope-defaults precision pass (`FE-SKIP-010` +
      `FEC-SKIP-006` now explicitly codify client-only skip synthesized payload
      defaults (empty deps/css/error-key arrays, unset server/head error fields)
      plus synthetic JSON-200 response build-id continuity header semantics;
      traceability row added as `missing` pending strict scenario coverage)
- [x] Dev-control-plane filemap-invalidation call-contract pass
      (`BUILD-DEV-016` + `BDC-DEV-016` now explicitly codify Vite invalidation
      helper behavior: missing-context fail-fast, bounded-timeout POST request
      construction, and failure semantics for transport/non-200 outcomes;
      traceability row added as `missing` pending strict scenario coverage)
- [x] Route-change title-ordering precision pass (`FE-REN-023` +
      `FEC-REN-017` now explicitly codify source+legacy-observed ordering that
      decoded `document.title` updates are visible before route-change event
      dispatch for the same commit; traceability row added as `missing`
      pending strict scenario coverage)
- [x] Submit redirect-envelope precision pass (`FE-FETCH-027` +
      `FEC-FETCH-022` now explicitly codify source+legacy-observed submit
      return shape for redirect handoff: success envelope with
      `data: undefined`, without exposing redirect metadata as submit data;
      traceability row added as `missing` pending strict scenario coverage)
- [x] Watcher debounce/shutdown precision pass (`BUILD-EVT-017/018` +
      `BDC-EVT-017/018` now explicitly codify source-observed watch-loop
      debounce window (30ms timer-reset batching) plus deferred debouncer-stop
      shutdown cleanup so pending timer/event state cannot flush callbacks after
      watcher exit; traceability rows added as `missing` pending strict
      scenario coverage)
- [x] Same-target prefetch control-reuse pass (`FE-NAV-019` +
      `FEC-NAV-014` now explicitly codify source-observed cross-slot reuse
      semantics: prefetch begin against a URL already owned by active
      navigation or pending revalidation must return that existing control
      without spawning duplicate prefetch/fetch work; traceability row added as
      `missing` pending strict scenario coverage)
- [x] Root-template callback error + reserved-key precedence pass
      (`BR-HTML-009/010` + `BRC-HTML-010/011` now explicitly codify
      source-observed HTML-path behavior: `GetRootTemplateData` errors map to
      HTTP 500 without successful template rendering, and runtime-reserved
      template keys override colliding callback-provided keys before template
      execution; traceability rows added as `missing` pending strict scenario
      coverage)
- [x] Focus-revalidation staleness-clock scope pass (`FE-HMR-006` +
      `FEC-HMR-005` now explicitly codify source-observed clock-update
      boundaries used by `revalidateOnWindowFocus`: successful navigate-intent
      and revalidation completions refresh staleness freshness, while prefetch,
      redirect-handoff, aborted, and failed outcomes do not; traceability row
      added as `missing` pending strict scenario coverage)
- [x] Backend root-id + builder-orchestration closure pass
      (`BR-HTML-011` + `BRC-HTML-012` now explicitly codify fixed
      `VormaRootID=vorma-root`; `BUILD-BLD-001..005` + `BDC-BLD-001..005` now
      codify two-pass file processing around hooks, file-only short-circuit,
      dev-vs-prod compile tag semantics, dist keep-file priming, and
      schema-write warning-only continuity; traceability rows added as `missing`
      pending strict scenario coverage)
- [x] Backend dev-reload handler-preservation closure pass (`BR-DEV-008` +
      `BRC-DEV-008` now explicitly codify that route reloads must preserve
      existing nested task-handler bindings for surviving patterns across
      route-tree rebuild; traceability row added as `missing` pending strict
      scenario coverage)
- [x] Build CSS artifact/url-rewrite closure pass (`BUILD-CSS-001..005` +
      `BDC-CSS-001..005` now explicitly codify missing-entry no-op semantics,
      critical output path, normal hashed-output/ref-pointer rotation, URL-token
      rewrite boundaries, and fail-fast behavior for filemap lookup failure in
      CSS rewrite path; traceability rows added as `missing` pending strict
      scenario coverage)
- [x] Build runtime-validation-gate closure pass (`BUILD-VAL-001..003` +
      `BDC-VAL-001..003` now explicitly codify fail-fast validation gates for
      build/dev entrypoints, core required-field rules, and Vite
      `JSPackageManagerBaseCmd` requirement under runtime validation;
      traceability rows added as `missing` pending strict scenario coverage)
- [x] UI typed-helper matched-pattern reactivity closure pass
      (`FE-UI-010/011` + `FEC-UI-008/009` now explicitly codify that
      pattern-based typed loader and client-loader helper outputs must track
      live `routerData.matchedPatterns` snapshots and return `undefined` when
      pattern is absent; Preact memo-dependency divergence recorded as open
      issue `VCI-036`; traceability rows added as `missing` pending strict
      scenario coverage)
- [x] UI adapter identity-promotion closure pass (`FE-UI-012` +
      `FEC-UI-010` now explicitly codify that route-level component identity
      transitions from undefined->defined must promote outlet resolution rather
      than remaining stuck in absent/fallback state; current adapter guard-path
      risk recorded as open issue `VCI-037`; traceability row added as
      `missing` pending strict scenario coverage)
- [x] UI adapter terminal-empty-branch parity closure pass (`FE-UI-013` +
      `FEC-UI-011` now explicitly codify that terminal component-absent outlet
      branches must render node-empty across adapters (no synthetic wrapper
      insertion); current Preact empty-`div` divergence recorded as open issue
      `VCI-038`; traceability row added as `missing` pending strict scenario
      coverage)
- [x] Legacy timing-intent drift audit pass (legacy frontend suites with stale
      `5ms` debounce/coalescing commentary were reconciled against current
      source/spec `8ms` contract; discrepancy surfaced explicitly as
      open harness-constraint issue `VCI-039` rather than enshrining conflicting
      timing intent)
- [x] Client-global missing-bootstrap fail-fast closure pass (`FE-CTX-009` +
      `FEC-CTX-007` now explicitly codify that global accessor wrappers must
      throw synchronously if the symbol-keyed bootstrap object is absent, and
      must not auto-install/synthesize fallback global state; traceability row
      added as `missing` pending strict scenario coverage)
- [x] History last-known baseline success-commit closure pass
      (`FE-SCROLL-010` + `FEC-SCROLL-008` now explicitly codify that successful
      history-listener update paths must advance the last-known location
      baseline used by later key/path/search/hash comparisons; traceability row
      added as `missing` pending strict scenario coverage)
- [x] Wire bootstrap mutable-store shape closure pass (`WIRE-HTML-005` +
      `WRC-HTML-005` now explicitly codify that SSR bootstrap initializes
      `patternToWaitFnMap` as `{}` and `clientLoadersData` as `[]` (not
      null/undefined); traceability row added as `missing` pending strict
      scenario coverage)
- [x] Frontend request-buildid + skip-route-match closure pass
      (`FE-FETCH-028` + `FEC-FETCH-023` now explicitly codify `vorma_json`
      fallback to `"1"` when client build id is absent/empty, and
      `FE-SKIP-011` + `FEC-SKIP-007` now codify that skip optimization is
      disallowed when target pathname has no nested matcher result;
      corresponding traceability rows added as `missing` pending strict
      scenario coverage)
- [x] Scroll helper explicit-state branch closure pass (`FE-SCROLL-011` +
      `FEC-SCROLL-009` now explicitly codify that `__applyScrollState`
      coordinate input must call `window.scrollTo(x,y)`, explicit hash input
      must target that element via `scrollIntoView()`, explicit-state calls
      must not fall back to `window.location.hash`, and missing-target/empty
      explicit hash must no-op; corresponding traceability row added as
      `missing` pending strict scenario coverage)
- [x] Listener-adder window-target scenario precision pass (`FEC-EVT-002`
      wording now explicitly codifies both sides of `FE-EVT-002` in executable
      scenario form: listener registration must bind to `window` event keys and
      returned cleanup callbacks must remove those same window listeners)
- [x] Backend root-data-flag negative-branch precision pass (`BR-LOAD-004`
      now explicitly codifies the inverse branch already represented by
      `BRC-LOAD-004`: when root route matches without a runnable root loader
      handler, `hasRootData` must be false)
- [x] Asset preload duplicate-promise + selector-safety closure pass
      (`FE-ASSET-009/010` + `FEC-ASSET-007/008` now explicitly codify that
      repeated `preloadCSS` calls for an already-present preload link return an
      immediate resolved promise (without attaching additional handlers), and
      module/CSS preload dedupe selectors must apply escaped-href matching
      semantics for selector-safe exact-href dedupe)
- [x] Backend runtime line-pass verification checkpoint (reviewed
      `vormaruntime/get_root_handler.go`, `vormaruntime/gmpd.go`,
      `vormaruntime/vorma_init.go`, `vormaruntime/route_reload.go`,
      `vormaruntime/route_registry.go`, and `vormaruntime/get_deps.go`; no new
      uncataloged backend normative contract gaps were discovered beyond already
      tracked open issues in `VORMA_CONFORMANCE_ISSUES.md`)
- [x] Backend form-content-type parse-outcome precision pass (`BR-ACT-003` +
      `BRC-ACT-003` now explicitly codify default parser zero-value typed-input
      behavior for non-GET form content types, in addition to the existing
      non-JSON-decode rule)
- [x] Deployment-ID + redirect-handshake + submit-error precision pass
      (`FE-FETCH-002`, `FE-FETCH-003`, `FE-FETCH-016`, `FE-FETCH-019`,
      `FEC-FETCH-001`, `FEC-FETCH-002`, `FEC-FETCH-011`, `WIRE-HDR-008`,
      `WIRE-Q-004`, `WRC-HDR-009`, and `WRC-Q-004` now explicitly codify
      absent/empty deployment-id omission semantics for revalidation
      query/header propagation, redirect-handshake header override-to-`1`
      behavior, and explicit `"Unknown error"` fallback for non-`Error` submit
      failure throwables)
- [x] Frontend init/events/refresh-storage precision pass (`FE-INIT-014` +
      `FEC-INIT-011`, `FE-EVT-005` + `FEC-EVT-005`, and `FE-SCROLL-012` +
      `FEC-SCROLL-010` now explicitly codify one-shot touch-listener
      registration semantics, empty-detail `vorma:location` payload shape, and
      refresh-scroll entry non-consumption on stale/mismatched skip paths;
      traceability rows added as `missing` pending strict scenario coverage)
- [x] Normative-intent mining ledger established
      (`/Users/sjc/__code/river/specs/NORMATIVE_INTENT_MINING_LEDGER.md` is now
      the authoritative per-file tracker for `VERIFIED`/`HISTORICAL`/`PENDING`
      mining status and explicit next mining queue ordering)
- [x] Inherited dependency mining ledger expansion pass (`matcher`,
      `nested_mux`, `headels`, `response`, `validate`, `tasks`, and frontend
      `vorma/kit/*` TypeScript dependency sources/tests are now explicitly
      tracked as per-file rows and promoted to the front of the strict mining
      queue)
- [x] Nested-router rebuild-preservation closure pass (`BR-LOAD-021` +
      `BRC-LOAD-021` now explicitly codify `mux.NestedRouter`
      rebuild-preserving-handlers semantics used by route sync: handler-backed
      routes retained, no-handler routes replaced by refreshed pattern set, and
      matcher-option stability across rebuild)
- [x] `kit/mux` replay-verification completion pass (`mux.go`,
      `nested_mux.go`, `mux_test.go`, `mux_advanced_test.go`, and
      `nested_mux_test.go` fully re-read under ledger workflow; no additional
      uncataloged Vorma-visible interop gaps found beyond promoted
      `BR-LOAD-021/BRC-LOAD-021`)
- [x] Matcher explicit-index guard + ordering precision pass (`BR-INIT-012` +
      `BRC-INIT-012` now codify slash-bearing explicit-index rejection as
      fail-fast matcher-construction behavior; interop matcher semantics now
      explicitly include same-depth deterministic tie-break ordering and
      index-last ordering; traceability rows updated with `missing` coverage
      status pending conformance scenarios)
- [x] `kit/response` replay-verification completion pass (`response.go`,
      `proxy.go`, `response_test.go`, and `proxy_test.go` fully re-read under
      ledger workflow; interop response contract now explicitly codifies
      redirect-status normalization to `303 See Other` default/fallback when
      redirect code is omitted or non-`3xx`)
- [x] `kit/headels` replay-verification completion pass (`headblocks.go` and
      `headblocks_test.go` fully re-read under ledger workflow; interop
      head-elements contract now explicitly codifies concurrent mutation safety
      for `Add`/`AddElements` and clone-safe `Collect()` snapshot behavior)
- [x] `kit/validate` replay-verification completion pass (`validate.go`,
      `rules.go`, `search_params.go`, `error_collector.go`,
      `validate_test.go`, `rules_test.go`, `search_params_test.go`,
      `error_collector_test.go`, `error_acc_test.go`, and
      `more_error_collector_test.go` fully re-read under ledger workflow;
      interop validation contract now explicitly codifies nil-guard parse-entry
      behavior, recursive validator traversal over struct/map/slice graphs,
      `Object`/`Any` checker short-circuit/idempotence semantics for
      app-defined validators, and URL query decode shape details including
      dotted-map keys, pointer-container auto-init, and slice empty-value
      filtering)
- [x] `kit/tasks` replay-verification completion pass (`tasks.go` and
      `tasks_test.go` fully re-read under ledger workflow; interop task-runtime
      contract now explicitly codifies `RunWithAnyInput` typed-nil vs untyped
      nil behavior, sticky per-request success+error memoization under
      `NewCtx` (TTL disabled), cross-context cache isolation, shared-cache
      collapse across `RunParallel` fan-out, nil-bound-task filtering/empty
      no-op semantics, and cancellation-sensitive pre/post-run checks)
- [x] `kit/_typescript/matcher` replay-verification completion pass
      (`register.ts`, `parse_segments.ts`, `find_best_match.ts`,
      `find_nested_matches.ts`, and all matcher scenario tests fully re-read
      under ledger workflow; interop frontend matcher contract now explicitly
      codifies parse normalization parity, explicit-index guard/normalization
      behavior, static/dynamic/splat precedence and tie-break semantics,
      trailing-slash/static compatibility, dynamic-empty rejection,
      parameter/splat extraction details, root false-positive guard behavior,
      and nested parent/leaf gap + ordering rules)
- [x] `kit/_typescript/url` replay-verification completion pass (`url.ts` and
      `url.test.ts` fully re-read under ledger workflow; interop URL/link
      contract now explicitly codifies error-status class detection, GET/HEAD
      method normalization, internal-vs-external href resolution semantics,
      non-HTTP protocol exclusion, anchor interception eligibility filters,
      delayed prefetch handler start/stop behavior, and single-link prefetch
      dedupe replacement)
- [x] `kit/_typescript/json` replay-verification completion pass (`json.ts`,
      `deep_equals.ts`, `stringify_stable.ts`, `search_param_serializer.ts`,
      and all JSON helper tests fully re-read under ledger workflow; interop
      JSON contract now explicitly codifies sorted query-key emission + dot
      flattening, empty/null/undefined query value semantics, repeated-key array
      ordering, `jsonDeepEquals` type/array/object/NaN semantics used by client
      change detection, and `jsonStringifyStable` recursive key sorting +
      cycle-failure behavior)
- [x] `kit/_typescript/debounce` replay-verification completion pass
      (`debounce.ts` and `debounce.test.ts` fully re-read under ledger
      workflow; interop timing-helper contract now explicitly codifies
      reset-on-each-call window semantics, last-call-only execution behavior,
      delayed Promise resolution with callback return value, and cancellation of
      superseded invocations)
- [x] `kit/_typescript/listeners` replay-verification completion pass
      (`listeners.ts` fully re-read under ledger workflow; existing interop
      focus/visibility listener contract in `VORMA_KIT_INTEROP_SPEC.md` already
      captured the full Vorma-visible behavior: dual listener registration,
      visibility-state gating for `visibilitychange`, `30ms` debounce window,
      and cleanup removal of both listeners)
- [x] `wave/tooling/broadcast.go` replay-verification completion pass
      (`BUILD-DEV-017` + `BDC-DEV-017` added to codify refresh-manager
      non-blocking fan-out behavior under backpressured websocket clients and
      shutdown drain/close semantics before manager stop completion;
      traceability row added as `missing` pending strict coverage)
- [x] `wave/tooling/events.go` replay-verification completion pass
      (full watcher/classification/hook/build/browser-phase flow re-read under
      ledger workflow; no additional uncataloged spec gaps were found beyond
      already-tracked open conformance issues on run-on-change-only behavior,
      and existing `BUILD-EVT-*`/`BUILD-DEV-*` contracts remained aligned)
- [x] `wave/tooling/builder.go` replay-verification completion pass
      (`BUILD-BLD-006` + `BDC-BLD-006` added to codify browser-mode gating of
      file-processing stages and public-first ordering before parallel
      private/CSS processing; traceability row added as `missing` pending strict
      coverage)
- [x] `wave/tooling/css.go` replay-verification completion pass
      (`BUILD-CSS-006/007` + `BDC-CSS-006/007` added to codify CSS import-set
      refresh with absolute-path normalization for watcher classification and
      dev-vs-prod minification flag behavior; traceability rows added as
      `missing` pending strict coverage)
- [x] `wave/tooling/static.go` replay-verification completion pass
      (full static filemap/copy/atomic-write pipeline re-read under ledger
      workflow; existing `BUILD-STATIC-*` contracts already covered source
      behavior, so no additional requirement IDs were needed in this pass)
- [x] `wave/tooling/schema.go` replay-verification completion pass
      (schema writer + config-schema definition graph fully re-read under ledger
      workflow; existing `BUILD-SCHEMA-*` contracts remained aligned, with no
      additional requirement IDs needed in this pass)
- [x] `wave/tooling/hash.go` replay-verification completion pass
      (content-hash helper implementation fully re-read under ledger workflow;
      existing `BUILD-STATIC-009` collision-guard contract already covered the
      Vorma-visible behavior, so no additional requirement IDs were needed)
- [x] `wave/tooling/url.go` replay-verification completion pass
      (`BUILD-STATIC-011/012/013/014` + `BDC-STATIC-011/012/013/014` added to
      codify buildtime public URL helper panic-vs-error modes, load-or-build
      filemap fallback semantics, non-prehashed-only key/map helper filtering +
      deterministic key order, and public asset key TS const/type emission
      contract; traceability rows added as `missing` pending strict coverage)
- [x] `wave/tooling/cli.go` replay-verification completion pass
      (`BUILD-CLI-007/008/009` + `BDC-CLI-007/008/009` added to codify nil-log
      defaulting to `colorlog.New(\"wave\")`, hook-callback gate/precedence
      semantics (`--hook` callback-present vs callback-nil behavior), and
      fail-fast panic + production builder deferred-close lifecycle contract;
      traceability rows added as `missing` pending strict coverage)
- [x] `wave/tooling/lock.go`, `wave/tooling/lock_unix.go`,
      `wave/tooling/lock_windows.go` replay-verification completion pass
      (`BUILD-DEV-018/019` + `BDC-DEV-018/019` added to codify lock parent-dir
      creation, non-ENOENT read-failure behavior, malformed/non-positive PID
      stale takeover semantics, lock-file release removal, and cross-platform
      PID liveness probe primitives (Unix signal-0 vs Windows limited-info
      process-open); traceability rows added as `missing` pending strict
      coverage)
- [x] `internal/framework/_typescript/client/src/resolve_public_href.ts`
      replay-verification completion pass (full helper re-read under ledger
      workflow; existing frontend asset URL contracts `FE-ASSET-005/006/007`
      already codify dev `viteDevURL` precedence, empty-dev fallback to
      `publicPathPrefix`, and slash-boundary normalization semantics, so no new
      requirement IDs were needed in this pass)
- [x] `internal/framework/_typescript/client/src/static_route_defs/route_def_helpers.ts`
      replay-verification completion pass (`FE-UI-014` + `FEC-UI-012` added to
      codify public `route(...)` DSL helper key-domain typing contract
      (`componentKey`/`errorBoundaryKey` constrained to awaited module exports)
      and runtime no-op/void behavior; traceability row added as `missing`
      pending strict coverage)
- [x] `internal/framework/_typescript/client/src/ui_lib_impl_helpers/route_components.ts`
      replay-verification completion pass (`FE-UI-015/016` +
      `FEC-UI-013/014` added to codify typed route-prop alias shape
      (`VormaRoutePropsGeneric`, `VormaRouteGeneric`, `ParamsForPattern`) and
      `UseRouterDataFunction` overload/accessor wrapper typing contract;
      traceability rows added as `missing` pending strict coverage)
- [x] `internal/framework/_typescript/client/src/ui_lib_impl_helpers/typed_navigate.ts`
      replay-verification completion pass (full helper re-read under ledger
      workflow; existing typed-navigation contracts in
      `API-CLIENT-008`/`FE-UI-005` already codify loader-path resolution via
      `pattern` + optional `params`/`splatValues`, forwarding of
      `replace`/`scrollToTop`/`search`/`hash`/`state`, and `Promise<void>`
      delegation semantics, so no new requirement IDs were needed)
- [x] `internal/framework/_typescript/client/src/vorma_app_helpers/vorma_app_helpers.ts`
      replay-verification completion pass (`FE-UI-017/018/019` +
      `FEC-UI-015/016/017` added to codify typed pattern-prop conditional
      requirements (params/splat and explicit-index permissive loader patterns),
      query/mutation prop method/input optionality constraints, and
      phantom-metadata fallback typing (`null|undefined`/`POST`/`never`) for
      exported IO/method/param aliases; traceability rows added as `missing`
      pending strict coverage)
- [x] `internal/framework/_typescript/client/src/history/npm_history_types.ts`
      replay-verification completion pass (`FE-CTX-008`/`FEC-CTX-006` refined
      to codify full history-instance compatibility surface inherited from
      bundled `history` type shim: action enum class (`POP`/`PUSH`/`REPLACE`),
      location payload (`pathname/search/hash/state/key`), and
      create/navigation/listen/block methods)
- [x] `internal/framework/_typescript/client/src/client_loaders.ts` +
      `internal/framework/_typescript/client/src/component_loader.ts`
      replay-verification completion pass (full loader/error-boundary pipelines
      re-read under ledger workflow; existing `FE-CL-*` and `FE-COMP-*`
      contracts already captured partial-match fallback probing, server-error
      cutoff + child-abort behavior, running-loader reuse, `serverDataPromise`
      payload shape, component import/export-key mapping, and error-boundary
      fallback/resilience semantics; no additional requirement IDs were needed)
- [x] `internal/framework/_typescript/client/src/error_boundary.ts`
      replay-verification completion pass (`FE-COMP-006` + `FEC-COMP-005`
      added to codify built-in default boundary render contract
      (`"Route Error: " + error`) with corresponding traceability row marked
      `missing` pending strict coverage)
- [x] `internal/framework/_typescript/client/src/global_loading_indicator/global_loading_indicator.ts`
      replay-verification completion pass (full helper re-read under ledger
      workflow; existing `FE-GLI-001..007` + `FEC-GLI-001..005` already codify
      include-filter semantics, default/debounced timer discipline,
      start/stop guard checks, setup-time snapshot behavior, and `include: []`
      disable semantics, so no additional requirement IDs were needed)
- [x] `internal/framework/_typescript/client/src/head_elements/head_elements.ts`
      replay-verification completion pass (full reconciliation implementation
      re-read under ledger workflow; existing `FE-REN-008..018` +
      `FEC-REN-007..012` already codify marker-safe no-op behavior,
      fingerprint dedupe/reorder/last-wins semantics, attribute validation,
      non-element managed-span cleanup, boolean/dangerous HTML application,
      section isolation, and deterministic fingerprinting, so no additional
      requirement IDs were needed)
- [x] `internal/framework/_typescript/client/src/hmr/hmr.ts`
      replay-verification completion pass (`FE-HMR-007` + `FEC-HMR-006` added
      to codify non-dev `initHMR()` inertness (no `window.__waveRevalidate`
      assignment and no HMR listener-registration side effects); traceability
      row added as `missing` pending strict coverage)
- [x] `internal/framework/_typescript/client/src/rendering.ts`
      replay-verification completion pass (full rerender pipeline re-read under
      ledger workflow; existing `FE-REN-001..023` + `FEC-REN-001..013` already
      codify view-transition eligibility, route-global commit ordering,
      history push/replace/state pass-through and non-navigate no-mutation
      rules, title decode/order semantics, route-change dispatch + scroll
      payload selection, and conditional section-scoped head reconciliation, so
      no additional requirement IDs were needed)
- [x] `internal/framework/_typescript/client/src/ui_lib_impl_helpers/link_components.ts`
      replay-verification completion pass (`FE-LINK-018` + `FEC-LINK-016`
      added to codify non-click handler ordering contract (internal prefetch
      start/stop side effect precedes user handler forwarding for
      pointer-enter/focus/pointer-leave/blur/touch-cancel); traceability row
      added as `missing` pending strict coverage)
- [x] `internal/framework/_typescript/client/src/utils/errors.ts`
      replay-verification completion pass (`FE-ERR-001/002` +
      `FEC-ERR-001/002` added to codify abort-error shape classification and
      panic helper logging/throw semantics (including default `"panic"`
      message); traceability rows added as `missing` pending strict coverage)
- [x] `internal/framework/_typescript/client/src/vorma_ctx/vorma_ctx.ts`
      replay-verification completion pass (global symbol + router-data accessor
      implementation and type-surface re-read under ledger workflow; existing
      `FE-CTX-001..009` + `FEC-CTX-001..007` and API/wire cross-spec mappings
      already captured direct symbol get/set semantics, accessor fallback
      defaults, effective-error derivation, and history-instance compatibility
      surface, so no additional requirement IDs were needed)
- [x] `internal/framework/_typescript/client/src/window_focus_revalidation/window_focus_revalidation.ts`
      replay-verification completion pass (focus-revalidation helper re-read
      under ledger workflow; existing `FE-HMR-004/006` +
      `FEC-HMR-003/005` already codify idle-only revalidation gating,
      default/override stale-time threshold semantics, and staleness-clock
      coupling to nav/revalidate outcomes, so no additional requirement IDs
      were needed)
- [x] Adapter index export replay-verification pass
      (`internal/framework/_typescript/react/index.tsx`,
      `internal/framework/_typescript/preact/index.tsx`,
      `internal/framework/_typescript/solid/index.tsx` re-read under ledger
      workflow; existing public/API contracts `API-UI-001`/`API-UI-002` already
      codify shared export parity and framework-specific location export names,
      so no additional requirement IDs were needed)
- [x] `internal/framework/_typescript/solid/src/solid.tsx`
      replay-verification completion pass (Solid outlet implementation re-read
      under ledger workflow; existing adapter/runtime contracts
      `FE-UI-001..013` + `FEC-UI-001..011` already capture listener idempotency,
      route/global snapshot refresh, scroll apply timing, error/fallback outlet
      behavior, and remount-identity semantics; known open parity divergences
      remain tracked in `VCI-037`/`VCI-038` without narrowing specs)
- [x] `internal/framework/_typescript/vite/vite.ts`
      replay-verification completion pass (Vite plugin implementation re-read
      under ledger workflow; existing build/API contracts
      `BUILD-VITE-001..007`, `BDC-VITE-001..007`, and `API-VITE-001..003`
      already capture config merge/mode semantics, transform rewrite/fallback
      behavior, dev filemap cache discipline, and invalidate-endpoint side
      effects, so no additional requirement IDs were needed)
- [x] Corpus-reconciliation expansion pass (post-queue audit)
      (ledger reconciled against repository domains to prevent omission loops;
      added explicit tracking section for previously untracked but in-scope
      domains: root API entrypoint, client package entrypoint, wave core package and vormabuild package)
- [x] Root/client entrypoint replay-verification pass
      (`vorma.go`, `internal/framework/_typescript/client/index.ts`, and
      `internal/framework/_typescript/client/tsconfig.json` re-read under
      ledger workflow; existing public API contracts
      `API-GO-001..014`, `API-SOURCE-001..004`, and
      `API-CLIENT-001..008` already covered constructor/export inventory and
      symbol-shape contracts, so no additional requirement IDs were needed)
- [x] `wave/wave.go` replay-verification completion pass
      (`BR-STATIC-001..004` + `BRC-STATIC-001..004` added to codify
      `Vorma.ServeStatic()` intercept/pass-through behavior, immutable cache
      header contract, root-vs-non-root public-prefix predicate semantics, and
      prefix-stripped public-FS lookup behavior; traceability rows added as
      `missing` pending strict coverage)
- [x] `wave/types.go` replay-verification completion pass
      (`BUILD-DEV-020/021/022` + `BDC-DEV-020/021/022` added to codify
      watch-root default/normalization semantics, healthcheck-endpoint default
      semantics, and browser/vite mode-derivation helpers from parsed config;
      traceability rows added as `missing` pending strict coverage)
- [x] `wave/env.go` replay-verification completion pass
      (`BUILD-DEV-023/024` + `BDC-DEV-023/024` added to codify app-port
      resolution/latching semantics (`PORT`, `WAVE_PORT_HAS_BEEN_SET`, dev
      free-port probe fallback) plus `GetIsDev`/`SetModeToDev` environment
      helper behavior; traceability rows added as `missing` pending strict
      coverage)
- [x] `wave/refresh.go` replay-verification completion pass
      (`BUILD-DEV-025..031` + `BDC-DEV-025..031` expanded to codify dev-gated
      refresh-script port/hash derivation plus websocket message contracts for
      rebuilding overlay singleton/fade-in behavior, hard-reload
      scroll-persistence key semantics, normal/critical CSS hot-swap DOM
      behavior, revalidate hook behavior, and socket close/error unload
      fallback semantics; traceability rows added as `missing` pending strict
      coverage)
- [x] `wave/filemap.go` + `wave/css.go` replay-verification completion pass
      (`BR-ASSET-001..004` + `BRC-ASSET-001..004` added to codify embedded
      Wave asset-helper contracts inherited by `Vorma`: critical/non-critical
      CSS helper empty-output gates, fixed element IDs (`wave-critical-css`,
      `wave-normal-css`), prefix-resolved stylesheet/filemap URL derivation,
      runtime `window.__wave.getPublicURL` helper payload semantics, and
      filemap-script CSP hash accessor behavior; traceability rows added as
      `missing` pending strict coverage)
- [x] `wave/parse.go` replay-verification completion pass
      (`BUILD-VAL-004/005` + `BDC-VAL-004/005` added to codify config-parse
      helper minimal-safety contracts: invalid JSON/missing-`Core` fail-fast
      behavior, normalized `Dist.Root` derivation from `Core.DistDir`,
      `ParseConfigFile` read-failure behavior, and parse-delegation
      equivalence; traceability rows added as `missing` pending strict
      coverage)
- [x] Embedded-wave API inventory closure pass
      (`API-GO-015/016` added to codify full inherited `*wave.Wave` method
      surface on `vorma.Vorma` and delegated behavioral ownership across
      backend/build/interop specs, preventing silent API drift from embedded
      method additions/removals)
- [x] `vormabuild/vorma_build.go` replay-verification completion pass
      (`BUILD-CLEAN-003` + `BDC-CLEAN-003` added to codify cleanup fail-fast
      behavior when static-public output path exists as non-directory, and
      `BUILD-ROUTE-009` + `BDC-ROUTE-009` added to codify slash-normalized
      workspace-relative route-module `SrcPath` materialization; traceability
      rows added as `missing` pending strict coverage)
- [x] `vormabuild/vorma_gen_ts.go` replay-verification completion pass
      (`BUILD-ART-008/009` + `BDC-ART-008/009` added to codify generated
      `VormaRootData` type derivation semantics and `ExtraTSCode` passthrough
      insertion; `BUILD-VITE-008/009` + `BDC-VITE-008/009` added to codify
      generated Vite-helper module surface (`waveRuntimeURL`,
      `vormaViteConfig`, global buildtime URL declaration) and ignore-pattern
      baseline composition; traceability rows added as `missing` pending strict
      coverage)
- [x] `vormabuild/vite_cmd.go` replay-verification completion pass
      (`BUILD-STAGE2-005` + `BDC-STAGE2-005` added to codify stage-two build-id
      propagation invariants: computed production build id must be written both
      to stage-two payload and runtime state, and stage-two `routeManifestFile`
      must propagate current manifest reference at conversion time; traceability
      row added as `missing` pending strict coverage)
- [x] `vormabuild/rebuild_routes.go` replay-verification completion pass
      (fast-route rebuild flow re-read under ledger workflow; existing
      `BUILD-FAST-001/002` coverage already captured dev-only guard, `dev_fast_`
      build-id + route-sync + artifact rewrite semantics, and selective
      route-manifest cleanup scope; wording refined to make missing-dir cleanup
      non-fatal behavior explicit in requirement/scenario text)
- [x] `vormabuild/route_registry_build.go` replay-verification completion pass
      (artifact-writer + manifest generator re-read under ledger workflow;
      existing `BUILD-ART-001..003` + `BDC-ART-001..003` refined to codify
      stage-one `routeManifestFile` propagation within same write pass and
      route-manifest filename hash derivation
      (`base64url(first-8-bytes(sha256(manifestJSON)))`) without introducing
      new requirement IDs)
- [x] `vormabuild/configschema.go` replay-verification completion pass
      (`BUILD-SCHEMA-006` + `BDC-SCHEMA-006` added to codify `Vorma` schema
      section shape/defaults: required field set, `UIVariant` enum restriction,
      and defaults for `IncludeDefaults` + `BuildtimePublicURLFuncName`;
      traceability row added as `missing` pending strict coverage)
- [x] `vormabuild/fs_to_hash.go` replay-verification completion pass
      (stage-two build-id hash helper re-read under ledger workflow; existing
      `BUILD-STAGE2-004` + `BDC-STAGE2-004` refined to make FS-summary walk
      domain explicit (`"."`/dirs excluded) and tuple encoding contract explicit
      (`<path>|<size>`), with no new requirement IDs introduced)
- [x] Mining-ledger mechanical completeness pass
      (cross-checked in-scope source domains against ledger entries to ensure no
      silent omissions; newly created
      conformance harness files plus `internal/framework/_typescript/create/.gitignore`
      were explicitly classified `OUT-OF-SCOPE` to keep queue accounting stable
      under future context compaction)
- [x] Dependency-led inheritance reconciliation pass (`kit/middleware`,
      `kit/htmlutil`, `kit/envutil`, `kit/netutil`, `kit/reflectutil`)
      (narrowed runtime/build import graph was replay-audited;
      `BR-STATIC-005` + `BRC-STATIC-005` added for embedded Wave favicon-redirect
      middleware behavior, and interop contracts were expanded to codify HTML
      render-safety/trust precedence, env+port fallback semantics, reflection
      helper invariants, and endpoint-gated middleware semantics)
- [x] Ledger scope-expansion queue initialization pass
      (newly discovered utility/lab dependencies from import reconciliation were
      explicitly added to `NORMATIVE_INTENT_MINING_LEDGER.md` as `PENDING` with a
      strict ordered queue so future mining progress remains externally auditable
      and non-circular under context compaction)
- [x] Utility crypto/codec inheritance pass (`kit/bytesutil`,
      `kit/cryptoutil`)
      (queue items were replay-verified and interop spec expanded with
      Vorma-relevant hash/encoding/key-guard contracts (`Sha256Hash`,
      base64/base64url helpers, key-shape guards, and security-sensitive nil/size
      input validation semantics) while keeping non-consumed package surface out
      of primary conformance requirements)
- [x] Utility runtime-helper inheritance pass
      (`kit/fsutil`, `kit/executil`, `kit/id`, `kit/lru`, `kit/colorlog`)
      (line-by-line queue replay completed; interop spec expanded with filesystem
      copy/gob decode panic-helper contracts, shell/cmd execution contracts,
      rejection-sampling ID constraints, LRU/TTL recency/eviction semantics, and
      default logger formatting/thread-safety contracts used by Vorma/Wave when
      caller-provided logger is absent)
- [x] Supporting `lab/*` utility inheritance pass
      (`lab/parseutil`, `lab/tsgen`, `lab/tsgen/tsgencore`, `lab/viteutil`,
      `lab/jsonschema`, `lab/stringsutil`)
      (line-by-line queue replay completed; interop spec expanded with
      deterministic TS generation/type-mapping contracts, Vite manifest/dev-script
      orchestration contracts, schema helper description/constructor contracts,
      string helper contracts, and panic-class package version parser behavior)
- [x] Vite dev-port candidate propagation precision pass
      (`BUILD-VITE-010` + `BDC-VITE-010` added to codify configured/default
      candidate-port propagation into Vite dev-port selection and `__VITE_PORT`
      runtime bridge; mismatch between contract and current implementation logged
      as open conformance issue `VCI-040` and traceability row added as `missing`)
- [x] Mining-ledger queue drain pass (utility/lab)
      (`NORMATIVE_INTENT_MINING_LEDGER.md` now marks queued utility/lab files
      `VERIFIED` and collapses unresolved queue entries to none, preserving
      externally auditable anti-circular progress tracking)
- [x] Transitive utility reconciliation closure pass
      (`go list -deps` replay against core runtime/build/tooling packages exposed
      additional untracked transitive dependencies; ledger and interop spec were
      expanded for `kit/contextutil`, `kit/genericsutil`, `kit/grace`,
      `kit/set`, and `lab/esbuildutil` with corresponding behavior contracts and
      `VERIFIED` source rows)
- [x] Vite dev-start error-propagation precision pass
      (`lab/viteutil/cmd.go` replay surfaced logged-and-suppressed `cmd.Start()`
      failure semantics; build spec expanded with `BUILD-VITE-011` +
      `BDC-VITE-011` to require startup error propagation/no false-positive
      running context, traceability row added as `missing`, and divergence logged
      as open conformance issue `VCI-041`)
- [x] RunOnChangeOnly app-liveness precision pass
      (`wave/tooling/events.go` replay surfaced hard-reload pre-kill leakage into
      `RunOnChangeOnly` short-circuit paths; build spec refinements added under
      `BUILD-EVT-010` and `BUILD-EVT-013` to require no implicit app-stop unless
      callback actions request restart, and divergence logged as open conformance
      issue `VCI-042`)
- [x] Event dedupe signal-preservation precision pass
      (`wave/tooling/events.go` replay surfaced same-path last-write dedupe risk
      where trailing chmod-only events can mask earlier content-changing ops;
      `BUILD-EVT-001`/`BDC-EVT-001` refined to require content-change signal
      preservation across deduped same-path batches, and divergence logged as
      open conformance issue `VCI-043`)
- [x] Dev-loop Vite startup failure-state precision pass
      (`wave/tooling/devserver.go` replay surfaced log-only handling for
      Vite-start failure during initial startup and cycle restart; build spec
      expanded with `BUILD-DEV-032` + `BDC-DEV-032` to require actionable
      failure-state behavior, traceability row added as `missing`, and divergence
      logged as open conformance issue `VCI-044`)
- [x] Build-failure retry request-strength precision pass
      (`wave/tooling/devserver.go` replay surfaced retry-wait wake-up path
      dropping consumed restart flags; build spec expanded with
      `BUILD-DEV-033` + `BDC-DEV-033` to require preservation of effective
      `recompileGo` and config-restart intent, traceability row added as
      `missing`, and divergence logged as open conformance issue `VCI-045`)
- [x] Mining-ledger bootstrap-template scope-classification pass
      (`NORMATIVE_INTENT_MINING_LEDGER.md` now explicitly classifies
      `bootstrap/assets/*` and `bootstrap/tmpls/*` rows as `OUT-OF-SCOPE`
      scaffold/distribution templates to keep exhaustive queue accounting
      mechanically complete without re-introducing out-of-scope bootstrap spec
      drift)
- [x] App-start failure handling precision pass
      (`wave/tooling/devserver.go` replay surfaced log-only continuation when
      `startApp` process launch fails; build spec expanded with
      `BUILD-DEV-034` + `BDC-DEV-034` to require actionable startup-failure
      control-flow behavior, traceability row added as `missing`, and
      divergence logged as open conformance issue `VCI-046`)
- [x] RunOnChangeOnly mixed-batch callback-phase precision pass
      (`wave/tooling/events.go` replay surfaced that mixed batches skip
      concurrent/post phases for run-on-change-only entries via per-entry
      `continue` guards; `BUILD-EVT-010/013` and `BDC-EVT-010/013` were refined
      to require per-entry callback-phase execution in mixed batches, and open
      issue `VCI-035` was tightened accordingly)
- [x] Readiness-gate failure handling precision pass
      (`wave/tooling/devserver.go` replay surfaced warning-only continuation
      when `waitForApp`/`waitForVite` exhaust readiness budgets; build spec
      expanded with `BUILD-DEV-035` + `BDC-DEV-035` to require actionable
      readiness-failure control-flow behavior, traceability row added as
      `missing`, and divergence logged as open conformance issue `VCI-047`)
- [x] Event-phase failure and no-wait isolation precision pass
      (`wave/tooling/events.go` replay surfaced log-only continuation after
      blocking event-phase failures; build spec expanded with
      `BUILD-EVT-019` + `BDC-EVT-019` to require actionable failure-state
      handling and suppression of success browser signaling for failed cycles,
      plus `BUILD-EVT-020` + `BDC-EVT-020` to codify warning-only/non-blocking
      isolation semantics for concurrent-no-wait hook failures; traceability
      rows added as `missing`, and divergence for blocking paths logged as open
      conformance issue `VCI-048`)
- [x] Hook command-token resolution precision pass
      (`wave/tooling/events.go` command resolution path now explicitly codified
      as `BUILD-EVT-021` + `BDC-EVT-021`: literal `DevBuildHook` token must
      resolve to current `Core.DevBuildHook`, explicit commands remain verbatim,
      and empty resolved commands skip execution; traceability row added as
      `missing`)
- [x] Event classification ignore-gate precision pass
      (`wave/tooling/events.go` classification prefilter behavior now
      explicitly codified as `BUILD-EVT-022` + `BDC-EVT-022`: empty-path events
      and unmatched `other`-type events are ignored and omitted from downstream
      event-processing flow; traceability row added as `missing`)
- [x] Hashed-artifact cleanup failure policy precision pass
      (`wave/tooling/static.go` and `wave/tooling/css.go` replay surfaced
      warning-only continuation on old-hash cleanup failures during filemap/CSS
      rotation; strict spec behavior under `BUILD-STATIC-005` and
      `BUILD-CSS-003` was kept unchanged and divergence is now tracked as open
      conformance issue `VCI-049`)
- [x] TS-generation loader-rune parity audit pass
      (`vormabuild/vorma_gen_ts.go` replay surfaced that client-defined
      loader-only path typing currently derives params/splat with action matcher
      runes; strict `BUILD-ART-004` loader-rune contract remains unchanged and
      divergence is now tracked as open conformance issue `VCI-050`)
- [x] Scope-boundary realignment pass
      (out-of-scope release/distribution + TS packaging requirement/scenario
      rows were removed from active Vorma build spec, traceability matrix, and
      issue/checklist references; mining-ledger entries for create/bootstrap/
      buildts scaffolding/distribution domains were explicitly reclassified as
      `OUT-OF-SCOPE`)
- [x] Route-build replay closure pass
      (`vormabuild/vorma_build.go`, `vormabuild/rebuild_routes.go`, and
      `vormabuild/route_registry_build.go` were replay-read line-by-line after
      scope cleanup; no new in-scope requirement gaps were found beyond existing
      open issue `VCI-050`, and stale template-watch scenario labeling was
      collapsed into `BDC-WATCH-005`)
- [x] Concurrent-hook restart-strength arbitration pass
      (`wave/tooling/events.go` replay surfaced that concurrent restart actions
      are currently consumed in goroutine-completion order, which can
      nondeterministically downgrade `RecompileGo=true` to no-go restart;
      `BUILD-EVT-023` + `BDC-EVT-023` now codify deterministic
      strongest-intent arbitration, and divergence is tracked as open
      conformance issue `VCI-051`)
- [x] Config-reload full framework-field preservation pass
      (`wave/tooling/devserver.go` replay surfaced that reload currently
      preserves only watch/ignore/public-map framework fields and drops
      framework schema/build-hook fields; `BUILD-DEV-036` + `BDC-DEV-036` now
      codify full non-JSON framework-field preservation, traceability row added
      as `missing`, and divergence tracked as open conformance issue `VCI-052`)
- [x] Dev exit-override timer precision pass
      (`wave/tooling/devserver.go` replay promoted `WAVE_DEV_EXIT_AFTER_MS`
      semantics into explicit contract (`BUILD-DEV-037` + `BDC-DEV-037`):
      invalid/unset override disables timer, positive timeout exits cleanly with
      teardown, and restart requests retain precedence when received before
      timeout; traceability row added as `missing`)
- [x] App-port alias compatibility pass
      (`wave/env.go` replay promoted backward-compat alias semantics so
      `MustGetAppPort` is explicitly locked to `MustGetPort` behavior
      (`BUILD-DEV-038` + `BDC-DEV-038`); traceability row added as `missing`)
- [x] Embedded-wave asset-helper fs/cache precision pass
      (`wave/wave.go`, `wave/filemap.go`, and `wave/css.go` replay promoted
      missing contracts for server-side `GetPublicURL` passthrough/fallback
      semantics plus dev-vs-prod embedded-fs source and helper memoization
      behavior (`BR-ASSET-005/006` + `BRC-ASSET-005/006`); traceability rows
      added as `missing`)
- [x] Parsed-config helper normalization/default precision pass
      (`wave/types.go` replay promoted missing normalization/default contracts
      for `PublicPathPrefix()`, `WatchRoot()`, `HealthcheckEndpoint()`,
      `CriticalCSSEntry()`, and `NonCriticalCSSEntry()`
      (`BUILD-VAL-006` + `BDC-VAL-006`); traceability row added as `missing`)
- [x] Framework-injection helper accumulation precision pass
      (`wave/wave.go` replay promoted explicit append/override semantics for
      `AddFrameworkWatchPatterns`, `AddIgnoredPatterns`, and
      `SetPublicFileMapOutDir` (`BUILD-WATCH-013` + `BDC-WATCH-013`);
      traceability row added as `missing`)
- [x] Hook timing default-pre bucket precision pass
      (`wave/types.go` replay promoted explicit hook timing-bucket behavior for
      `WatchedFile.Sort` (empty/unknown timing defaults to pre; explicit
      `post`/`concurrent`/`concurrent-no-wait` map to corresponding phases)
      as `BUILD-EVT-024` + `BDC-EVT-024`; traceability row added as `missing`)
- [x] Refresh-script revalidate-rejection cleanup pass
      (`wave/refresh.go` replay surfaced missing rejection-path overlay cleanup
      for `__waveRevalidate()` promise failures; `BUILD-DEV-030`/`BDC-DEV-030`
      were tightened to require rejection-path cleanup and diagnostics, and
      divergence logged as open conformance issue `VCI-053`)
- [x] Revalidation target-canonicalization + 304-path divergence pass
      (`internal/framework/_typescript/client/src/client.ts` replay promoted
      missing explicit revalidation-target canonicalization behavior into
      `FE-NAV-020` + `FEC-NAV-015` + traceability row (`missing`), and surfaced
      a strict conformance divergence where currently allowed-304 handling still
      fails through `No JSON response` path (`VCI-054`))
- [x] Dev reload failure non-mutation precision pass
      (`vormaruntime/route_reload.go` replay promoted explicit failure-path state
      preservation for route/template reload into `BR-DEV-009` +
      `BRC-DEV-009` + traceability row (`missing`): failed reload attempts must
      leave previously active route/template runtime snapshot in effect)
- [x] History-listener event/scroll ordering precision pass
      (`internal/framework/_typescript/client/src/history/history.ts` and legacy
      history/events tests replay promoted two missing frontend runtime
      contracts: `FE-SCROLL-013` + `FEC-SCROLL-011` now lock the exact
      pre-branch scroll-save boundary (all non-same-document-POP updates save;
      same-document POP skips), and `FE-EVT-006` + `FEC-EVT-006` now lock
      changed-key location-event dispatch ordering before action-specific history
      branches, including cross-document POP failure fallback; traceability rows
      added as `missing`)
- [x] Kit mux mount-root helper argument parity pass
      (`kit/mux/mux.go` + `kit/mux/mux_test.go` replay tightened interop
      section 5.1 to explicitly lock inherited `MountRoot(...)` helper argument
      semantics: zero args returns canonical mount root, one arg appends via
      join, and extra args are ignored)
- [x] Frontend legacy replay closure pass (links/submissions/context tie-break precision)
      (`internal/framework/_typescript/client/src/client.events_system.test.ts`,
      `client.history_management.test.ts`, `client.scroll_restoration.test.ts`,
      `client.prefetching.test.ts`, `client.error_handling.test.ts`,
      `client.navigation_lifecycle.*.test.ts`,
      `client.core_navigation.*.test.ts`,
      `client.form_submissions.*.test.ts`,
      `client.component_module_loading.test.ts`, plus source replay in
      `client/src/links.ts` and `client/src/component_loader.ts` were
      line-by-line revalidated; missing strict contracts were added for
      effective-error tie precedence (`FE-CTX-010` + `FEC-CTX-008`) and
      repeated-prefetch timer cancellation semantics
      (`FE-LINK-019` + `FEC-LINK-017`), traceability rows were added as
      `missing`, and current implementation divergence for timer overwrite/stop
      under repeated start was logged as open conformance issue `VCI-055`)
- [x] Process and adapter-policy scope alignment pass
      (`VORMA_PUBLIC_API_SURFACE_SPEC.md` now explicitly codifies
      cross-adapter alignment as default with only model-required
      framework-specific differences (`API-UI-009`) plus parity gate
      requirement (`API-TEST-005`); `VORMA_SPEC_PROCESS_RFC_SPEC.md` was
      simplified to a sub-1.0 lightweight workflow (`Proposed`/`Final`/`Dropped`,
      in-place editing, checklist+traceability as canonical tracking, and
      explicit permission to prune obsolete proposal/history cruft))
- [x] Backend/wire `viteDevURL` semantics + ledger dependency-closure pass
      (`vormaruntime/get_root_handler.go` + `vormaruntime/vite_url.go` replay
      promoted explicit mode-dependent `viteDevURL` contract into backend and
      wire specs (`BR-LOAD-022` + `BRC-LOAD-022`,
      `WIRE-JSON-005` + `WRC-JSON-006`), with traceability rows added as
      `missing`; mechanical ledger audit over `kit/*` + `lab/*` then added
      explicit `OUT-OF-SCOPE` rows for non-imported packages using
      `go list -deps . ./vormaruntime ./vormabuild ./wave ./wave/tooling`
      closure to prevent future mining-loop ambiguity)
- [x] Build event pattern-dedupe strength-preservation pass
      (`wave/tooling/events.go` replay surfaced that same-pattern dedupe can
      nondeterministically drop stronger implicit work; strict build event
      contract was tightened under `BUILD-EVT-001` with scenario
      `BDC-EVT-025`, traceability mapping was updated, and implementation
      divergence was logged as open conformance issue `VCI-056` rather than
      codified as acceptable behavior)

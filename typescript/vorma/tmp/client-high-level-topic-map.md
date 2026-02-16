Vorma Browser Client High-Level Topic Map

Purpose

- Define complete broad-topic coverage for the Vorma browser-client audit before
  line-level specification extraction.
- Establish the authoritative topic index that the normative spec and regression
  diff will reference.

Guiding Principle

- Across client navigation, prefetching, loader execution, and revalidation, the
  design objective is to do only the minimum amount of work required to ensure
  data is not stale.

Documentation Discipline

- This document remains strictly declarative and present-tense.
- Historical narration, backward-looking commentary, conversational notes, and
  changelog-style accumulation are prohibited.
- Superseded statements are replaced in place rather than retained as history.

Scope Definition

- In scope: TypeScript behavior shipped to browsers for Vorma client runtime and
  UI adapters.
- Out of scope: `create`, `vite`, server-only runtime paths.
- UI adapter rule: behavior is specified as adapter-generic contracts; semantics
  are expected to match across all current and future adapters.

Branch Root Coverage

- Baseline branch (`main`) browser-client roots:
    - `internal/framework/_typescript/client`
    - `internal/framework/_typescript/react`
    - `internal/framework/_typescript/preact`
    - `internal/framework/_typescript/solid`
    - Supporting shared browser utility roots used by client runtime:
        - `kit/_typescript/url`
        - `kit/_typescript/matcher`
        - `kit/_typescript/json`
        - `kit/_typescript/debounce`
        - `kit/_typescript/listeners`
- Working branch (`refactor-2026-8`) browser-client roots:
    - `typescript/vorma/client`
    - `typescript/vorma/ui-adapters/react`
    - `typescript/vorma/ui-adapters/preact`
    - `typescript/vorma/ui-adapters/solid`
    - Supporting shared browser utility roots used by client runtime:
        - `typescript/kit/url`
        - `typescript/kit/matcher`
        - `typescript/kit/json`
        - `typescript/kit/debounce`
        - `typescript/kit/listeners`

Broad Behavior Topics

- `HL-01` Client bootstrap and initialization lifecycle
    - Runtime initialization ordering, history init, hard-reload query cleanup,
      initial module/loaders/error-boundary setup, first render, refresh scroll
      restore, touch detection.
- `HL-02` Global client state model and access surfaces
    - Global context shape, router data projection, runtime render-state reads,
      client loader registration map, navigation state access bridge.
- `HL-03` Navigation arbitration and lane ownership
    - Active/prefetch/revalidation lane selection, begin-navigation arbitration,
      reuse/promotion, abort supersession, operation ownership checks.
- `HL-04` Deterministic revalidation lane policy
    - In-flight coalescing, queued trailing pass, target-mismatch invalidation,
      reset/clear behavior.
- `HL-05` Route-data fetch pipeline
    - Request URL shaping, server fetch handling, response validation, parallel
      client-loader startup, preload plan derivation.
- `HL-06` Client-only skip/fetch-elision policy
    - Skip eligibility based on route manifest, match stability,
      param/splat/search constraints, module-map sufficiency, synthetic success
      outcome behavior.
- `HL-07` Redirect detection and effectuation
    - Redirect parsing (`X-Vorma-Reload`, browser redirect,
      `X-Client-Redirect`), hard vs soft strategy, effectuation cleanup,
      redirect depth guardrails.
- `HL-08` Successful navigation lifecycle checkpoints
    - Pre/post waiting checks, pre/post asset wait checks, render/no-render
      branching for prefetch and stale revalidation, cleanup invariants.
- `HL-09` Build ID synchronization and artifact gating
    - Build ID propagation points, build event emission, response artifact
      application only when build constraints are satisfied.
- `HL-10` Client loaders execution model
    - Pattern registration, server-data wiring, running-loader reuse,
      child-loader abort-on-parent-failure, result shaping, outermost client
      error derivation.
- `HL-11` Render commit pipeline
    - Route-state commit, error-state derivation, component/error-boundary
      activation, history+scroll derivation, title/head/css application,
      route-change dispatch.
- `HL-12` View transition policy
    - View-transition enablement criteria and exclusions by navigation type.
- `HL-13` Link click lifecycle
    - Anchor eligibility filtering, default-prevention rules, same-document/hash
      behavior, callback ordering, navigation outcome handling on click.
- `HL-14` Prefetch intent lifecycle
    - Delayed prefetch start/stop, hover/focus/touch interaction semantics,
      abort rules, prefetch-to-navigation upgrade behavior.
- `HL-15` History integration behavior
    - Location-change dispatching, serialized listener processing, POP
      within-same-document semantics, POP cross-document client navigation
      fallback and hard reload.
- `HL-16` Scroll state persistence and restoration
    - Scroll snapshot storage limits, per-history-key restores, hash-target
      scrolling, recent page-refresh restore constraints.
- `HL-17` Event model and status signaling
    - Route-change/status/build-id/location event contracts, status derivation,
      debounce and dedupe behavior.
- `HL-18` Submission lifecycle and staleness control
    - Submission dedupe, checkpointed staleness evaluation, response
      classification, redirect handling, optional automatic revalidation.
- `HL-19` Revalidation-on-focus policy
    - Focus/visibility triggering, stale-window gating, status-based blocking
      conditions.
- `HL-20` Head element managed-region reconciliation
    - Managed section boundaries, element fingerprinting/deduping, minimal DOM
      moves/removals, stable ordering guarantees.
- `HL-21` Asset preload and stylesheet application
    - Module preload dedupe, CSS preload dedupe, stylesheet injection and bundle
      marker behavior.
- `HL-22` Route matching/registration semantics
    - Pattern normalization and registration, nested match selection,
      dynamic/splat conflict resolution, index and trailing-slash behavior.
- `HL-23` URL and anchor classification semantics
    - Internal/external determination, data-target equality semantics, hash
      normalization, click eligibility constraints.
- `HL-24` Adapter-generic runtime contracts
    - Route outlet branching semantics, location/router-data subscription
      semantics, route-change-to-scroll timing, typed link and typed loader-hook
      integration.
- `HL-25` Development-time HMR behavior
    - Client-loader refresh behavior under module updates, pattern-to-module
      association, route-change emission after HMR loader recomputation.
- `HL-26` Public API behavior surface
    - `vormaNavigate`, `beginNavigation`, `revalidate`, `submit`, `getStatus`,
      `getLocation`, `getBuildID`, `getRootEl`, and internal testing/debug
      surfaces.

Cross-Branch Mapping Intention

- Each `HL-*` topic maps to a detailed normative spec section with stable
  requirement IDs.
- Branch comparison evaluates behavior parity by requirement ID, not by API
  naming or file layout.

Next-Step Boundary

- This document is the checkpoint output for broad-topic coverage only.
- Next step is line-level extraction of observable behavior stories under each
  `HL-*` topic for baseline branch first.

# Vorma Global Correctness Refactor Plan (2026-02-25)

Purpose: one handoff-safe master checklist for all agreed deep refactors,
implemented to final end-state with no compatibility layering.

Scope:

1. RouteScope + atomic snapshot model
2. Unified pure navigation/submission engine + command executor
3. Strict wire-boundary decoder/invariant firewall
4. Unified adapter host runtime (React/Preact/Solid shared core)

## Non-Negotiables

- [x] Keep `matchedPatterns` as canonical route identity on wire/runtime.
- [x] Keep canonical-pattern uniqueness as strict invariant.
- [x] No compatibility layers; ship direct end-state behavior.
- [x] Fail loud on invariant violations; no fallback behavior.
- [x] Keep progressive manifest optional perf-only.
- [x] Preserve client-loader parallel start and reuse traits.
- [x] Keep shared-layer logic authoritative (adapter thin wrappers only).

## Dependency Order

- [x] Phase A (wire decoder firewall) first.
- [x] Phase B (RouteScope + atomic snapshot) second.
- [x] Phase C (pure navigation/submission reducer + commands) third.
- [x] Phase D (shared adapter host extraction) fourth.
- [ ] Phase E (deletion/cleanup/perf gate/final verification) fifth.

## Phase A: Wire-Boundary Decoder / Invariant Firewall

Goal: decode and validate route JSON once at ingress, then run only on canonical
internal payloads.

### A1: Canonical decoded payload model

- [x] Introduce canonical decoded payload type in client runtime boundary.
- [x] Include all fields used by commit/render/client-loader paths.
- [x] Mark optional/required fields explicitly in one place.
- [x] Ensure decoded payload is immutable after decode.

### A2: Decode function and hard invariants

- [x] Add one decode function at route-data fetch boundary.
- [x] Validate array-length alignment with `matchedPatterns`: `loadersData`,
      `clientLoadersData` (if present), `importURLs`, `exportKeys`,
      `errorExportKeys`.
- [x] Validate pattern entries are non-empty strings.
- [x] Validate `outermost*ErrorIdx` are in-range when present.
- [x] Validate root-data/loader-index assumptions (`hasRootData` invariants).
- [x] Throw hard errors with deterministic messages on violation.

### A3: Replace downstream shape assumptions

- [x] Replace all direct raw JSON reads after fetch with decoded payload reads.
- [x] Remove duplicated downstream shape guards made redundant by decoder.
- [x] Ensure commit/render code never receives unvalidated payload.

### A4: Files to update (primary)

- [x] `typescript/vorma/client/src/core/navigation/fetch_route_data_server.ts`
- [x] `typescript/vorma/client/src/core/navigation/types.ts`
- [x] `typescript/vorma/client/src/core/render_commit_runtime.ts`
- [x] `typescript/vorma/client/src/core/render_client_loader_runtime.ts`

### A5: Phase-A tests

- [x] Add decode-pass tests for valid payloads.
- [x] Add decode-fail tests for every invariant violation class.
- [x] Add tests ensuring invalid payload never reaches commit/render.

## Phase B: RouteScope + Atomic Snapshot Ownership Model

Goal: ownership-safe selectors by construction, no token lifecycle.

### B1: Immutable snapshot contract

- [x] Define immutable `RenderSnapshot` as sole selector source.
- [x] Keep all route/render identity and data fields in snapshot.
- [x] Guarantee one atomic snapshot replacement per successful commit.
- [x] Enforce commit ordering: snapshot first, events second.
- [x] Add structural sharing for unchanged segments.

### B2: RouteScope contract

- [x] Define immutable `RouteScope` per matched route instance.
- [x] Include route-owned loader/client-loader slices and route-bound metadata.
- [x] Bind route scope at component mount site.
- [x] Ensure scope identity remount behavior is deterministic by route key.

### B3: Selector rewiring

- [x] Route-props selectors read only from bound scope.
- [x] Global selectors read only from current snapshot.
- [x] Remove any prior/current-window resolution logic from selector paths.

### B4: Delete token machinery (mandatory)

- [x] Delete token create/activate/dispose/sync helpers.
- [x] Delete token maps/sets and token-bearing route-props helpers.
- [x] Delete token contract branches and token-specific errors.
- [x] Delete removed-token internal exports.

### B5: Files to update (primary)

- [x] `typescript/vorma/client/src/app/context.ts`
- [x] `typescript/vorma/client/src/ui/typed_adapter_helpers_runtime.ts`
- [x] `typescript/vorma/client/src/ui/route_outlet_runtime.ts`
- [x] `typescript/vorma/ui-adapters/react/src/react.tsx`
- [x] `typescript/vorma/ui-adapters/preact/src/preact.tsx`
- [x] `typescript/vorma/ui-adapters/solid/src/solid.tsx`
- [x] `typescript/vorma/ui-adapters/*/src/helpers.ts`

### B6: Phase-B tests

- [x] Per-tick route-props transition tests (React/Preact/Solid).
- [x] Per-tick global selector coherence tests.
- [x] Aborted-render tests proving no ownership leak states.
- [x] Tests proving stale snapshot reads are unrepresentable after commit.

## Phase C: Unified Pure Navigation/Submission Engine + Command Executor

Goal: one reducer decides behavior for all lanes; side effects run only as
commands.

### C1: Unified state and transition model

- [x] Define one reducer state for lanes: `navigate`, `prefetch`, `revalidate`,
      `submit`.
- [x] Define explicit phase/ownership fields in reducer state.
- [x] Define strict transition table for stale/superseded outcomes.
- [x] Introduce explicit single-pass navigate reducer state with ownership and
      current-location fields (`targetUrl`, `entry`, `expectedOperationID`,
      `currentHref`).
- [x] Introduce single-pass transition table that deterministically maps
      resolved/rejected pass events to command plans.
- [x] Introduce successful-navigation checkpoint reducer transition table that
      deterministically maps checkpoint events to command plans.
- [x] Introduce submit runtime reducer state with explicit phase/ownership
      fields for request/classification/payload checkpoints.
- [x] Define canonical lane-state type including `navigate`/`prefetch`/
      `revalidate`/`submit` lanes in shared navigation types.

### C2: Unified event input model

- [x] Encode all external stimuli as reducer events: start, fetch-resolve,
      wait-resolve, redirect, abort, render-finish, timeout,
      external-location-change.
- [x] Remove direct mutation side paths outside reducer.
- [x] Encode single-pass navigate events as typed reducer events:
      `outcome_resolved` and `outcome_rejected`.
- [x] Route single-pass runtime handling through reducer transitions instead of
      direct resolved/rejected branch planning.
- [x] Encode successful-navigation checkpoint boundaries as typed reducer events
      (`pre_waiting`, `post_waiting`, `pre_asset_wait`, `post_asset`,
      `cleanup`).
- [x] Route successful-navigation checkpoint handling through reducer
      transitions instead of direct per-checkpoint plan branching.
- [x] Encode navigate-request runtime branch inputs as typed reducer events
      (`navigate_requested`) with explicit execution-plan outcomes.
- [x] Route `runtime.ts` navigate request branching through reducer+command
      planning seams instead of direct same-document/revalidation branching.
- [x] Encode navigation lifecycle-runtime mutation inputs as typed reducer
      events (`remove_navigation`, `transition_navigation_phase`,
      `begin_navigation_arbitrated`, `submission_state_transitioned`,
      `navigation_failed`, `clear_all`).
- [x] Route lifecycle-runtime mutation dispatch through shared transition
      helpers + journal dispatch (no redundant reducer-dispatch hop).
- [x] Encode submit runtime checkpoint inputs as typed reducer events
      (`request_resolved`, `response_classified`, `success_payload_parsed`).
- [x] Route submit runtime checkpoint branching through reducer transitions
      instead of inline request/classification/payload branch trees.
- [x] Encode deterministic revalidation-lane request handling as a typed reducer
      event (`revalidation_requested`) with explicit plans for idle start,
      mismatch restart, in-flight reuse, and trailing scheduling.
- [x] Encode runtime revalidation target mismatch handling as a typed reducer
      event (`revalidation_target_mismatch_notified`) instead of direct callback
      mutation branching.
- [x] Encode runtime navigation-removal handling as a typed reducer event
      (`navigation_removal_requested`) instead of direct remove-API imperative
      branching.
- [x] Encode runtime clear-all handling as a typed reducer event
      (`clear_all_requested`) instead of direct runtime imperative sequencing.

### C3: Command output model

- [x] Reducer outputs typed command list (no side effects inside reducer).
- [x] Command types include: fetch, start-client-loaders, preload-assets,
      commit-snapshot, history-write, redirect, sync-build-id, emit-events,
      cleanup-entry, log-error.
- [x] Define command ordering rules explicitly.
- [x] Introduce pure navigation-outcome command planner mapping outcome
      execution plans to ordered runtime commands.
- [x] Route `runtime_navigation_pass_runtime` through explicit command execution
      seam.
- [x] Add unit tests asserting redirect command ordering and aborted-outcome
      delete command planning.
- [x] Introduce successful-navigation checkpoint command planners for
      pre/post-waiting, pre/post-asset, and cleanup checkpoints.
- [x] Route `runtime_navigation_successful_runtime` checkpoint handling through
      command execution seam.
- [x] Add unit tests for successful-navigation checkpoint command planners.
- [x] Introduce successful-navigation checkpoint reducer seam over typed
      checkpoint events.
- [x] Route `runtime_navigation_successful_runtime` checkpoint execution through
      the checkpoint reducer seam.
- [x] Introduce submission lifecycle command planners for begin/finish seams.
- [x] Route submission begin/finish lifecycle through command execution seam.
- [x] Add unit tests for submission command planner ordering.
- [x] Introduce navigation-pass rejection execution planning + command plans.
- [x] Route navigation-pass rejection cleanup through explicit command execution
      seam.
- [x] Introduce submit post-classification runtime command planner.
- [x] Route submit post-classification side effects through command-plan
      execution.
- [x] Add unit tests for navigation-pass rejection and submit
      post-classification command planning.
- [x] Introduce submit runtime checkpoint command-planning seam for build-ID
      sync and post-classification command execution.
- [x] Introduce begin-navigation runtime command planner that maps
      begin-navigation execution plans to ordered runtime commands.
- [x] Route begin-navigation runtime side effects through explicit command-plan
      execution seam.
- [x] Introduce revalidation-lane runtime command planner that maps revalidation
      request execution plans to ordered runtime commands.
- [x] Introduce runtime revalidation target mismatch command planner for
      explicit abort+delete sequencing.
- [x] Introduce runtime navigation-removal command planner for explicit
      abort+delete sequencing.
- [x] Introduce runtime clear-all command planner for explicit reset, lane
      clear, and lifecycle-dispatch sequencing.

### C4: Command executor

- [x] Implement thin executor that runs commands in reducer order.
- [x] Guarantee idempotent ownership guard at command execution boundary.
- [x] Ensure command executor cannot perform side effects not present in plan.
- [x] Add explicit pass-rejection runtime command executor that only runs
      planned side effects.
- [x] Add explicit submit runtime command execution helper with stale-ownership
      checkpoints before/after command execution.
- [x] Route revalidation-lane request handling through an explicit command
      executor that runs only planned commands.
- [x] Route runtime revalidation-target-mismatch handling through an explicit
      command executor that runs only planned commands.
- [x] Route runtime navigation-removal handling through an explicit command
      executor that runs only planned commands.
- [x] Route runtime clear-all handling through an explicit command executor that
      runs only planned commands.
- [x] Remove `runtime.ts` passthrough re-exports of slot/pass/success internals
      used only by tests; import tests directly from authoritative modules.

### C5: Preserve parallelism traits

- [x] Keep speculative client-loader starts before final commit.
- [x] Keep running-loader map reuse across phases.
- [x] Keep `Promise.allSettled` fanout semantics for client loaders.
- [x] Ensure reducer/executor split does not serialize loader paths.
- [x] Add source tests proving speculative loader start before server-payload
      resolve in `startParallelClientLoaders`.
- [x] Add source tests proving `completeClientLoaders` starts matched loaders in
      parallel before earlier-settle completion.
- [x] Add source tests proving `runningLoaders` reuse avoids duplicate wait-fn
      invocation.

### C6: De-duplicate existing navigation seams

- [x] Remove scattered stale/ownership checks duplicated across files.
- [x] Collapse submit race handling into reducer transitions.
- [x] Collapse build-ID/redirect stale guard logic into reducer transitions.
- [x] Collapse pass-rejection stale ownership + delete/report branching into one
      shared planning seam.
- [x] Collapse submit post-classification side-effect branching into
      command-plan execution with centralized stale-ownership checkpoints.
- [x] Collapse single-pass resolved/rejected outcome planning into one shared
      reducer seam.
- [x] Collapse successful-navigation checkpoint plan branching into one shared
      reducer seam.
- [x] Collapse runtime navigate-request same-document/revalidation branch logic
      into one shared reducer seam.
- [x] Collapse lifecycle-runtime per-method transition/mutation branching into
      one shared reducer seam.
- [x] Delete `navigation_controls` abstraction and collapse begin-navigation
      control creation/fetch-rejection cleanup into the begin-navigation
      reducer+command execution seam.

### C7: Files to update (primary)

- [x] `typescript/vorma/client/src/core/navigation/runtime_navigation_outcome_state_machine.ts`
- [x] `typescript/vorma/client/src/core/navigation/runtime_navigation_successful_runtime.ts`
- [x] `typescript/vorma/client/src/core/navigation/runtime_navigation_pass_runtime.ts`
- [x] `typescript/vorma/client/src/core/navigation/runtime_submit.ts`
- [x] `typescript/vorma/client/src/core/navigation/runtime.ts`
- [x] `typescript/vorma/client/src/core/navigation/runtime_lifecycle_runtime.ts`
- [x] `typescript/vorma/client/src/core/navigation/runtime_lifecycle_transitions.ts`
- [x] `typescript/vorma/client/src/core/navigation/types.ts`
- [x] `typescript/vorma/client/src/core/navigation/begin_navigation.ts`
- [x] `typescript/vorma/client/src/core/navigation/runtime_engine_state_machine.ts`

### C8: Phase-C tests

- [x] Reducer transition tests for all lanes and race classes.
- [x] Command-plan tests verifying exact side-effect sequence.
- [x] Tests proving no side effect occurs without command.
- [x] Tests proving stale completions cannot commit side effects.
- [x] Tests proving parallel client-loader behavior unchanged.
- [x] Add command-plan tests for navigation-pass rejection and submit
      post-classification seams.
- [x] Add reducer transition tests for single-pass navigate resolved/rejected
      events.
- [x] Add reducer transition tests for successful-navigation checkpoint events.
- [x] Add transition-helper tests for lifecycle-runtime event builders/removals.
- [x] Add reducer+command-plan tests for runtime navigate-request
      same-document/revalidation execution paths.
- [x] Add reducer transition tests for submit runtime request/classification/
      payload checkpoints.
- [x] Add command-plan/runtime-execution tests for begin-navigation planner
      cases (reuse-promotion, create, and immediate-abort terminals).
- [x] Add reducer+command-plan tests for deterministic revalidation-lane request
      handling (`start`, `restart`, `reuse`, `trailing`).
- [x] Update begin-navigation runtime tests to validate direct control creation
      and fetch-rejection cleanup through the begin-navigation seam (no
      `createNavigationControls` layer).
- [x] Add reducer+command-plan/runtime-execution tests for runtime clear-all
      handling.
- [x] Add unified runtime-engine reducer tests covering stale ownership
      transitions and external-stimulus events.
- [x] Add unified runtime-engine executor tests covering command-order execution
      and ownership-guarded cleanup.
- [x] Add unified runtime-engine tests covering revalidation mismatch,
      navigation removal, and clear-all command planning/execution semantics.

## Phase D: Unified Adapter Host Runtime (Shared Core)

Goal: shared host owns behavior; framework packages are thin adapters.

### D1: Shared host API

- [x] Define framework-agnostic host contract for: snapshot subscription,
      route-scope binding, remount keying, outlet branch resolution, error
      boundary selection.
- [x] Define adapter bridge hooks for framework lifecycle entry points only.
- [x] Introduce initial shared adapter-host contract for outlet branch
      resolution, matched-pattern selection, and route-scope mount-props
      binding.
- [x] Introduce shared adapter render-model contract that unifies branch
      resolution, matched-pattern binding, and outermost-error payload wiring.
- [x] Introduce shared remount-policy bridge helper for adapter route-component
      mounts.

### D2: Host extraction

- [x] Move shared outlet/store synchronization logic into one host module.
- [x] Move shared remount/branch/scope binding logic into host module.
- [x] Remove duplicated logic from React/Preact/Solid implementations.
- [x] Extract initial shared host helpers into
      `route_outlet_adapter_host_runtime.ts` and consume them from
      React/Preact/Solid route-component mount paths.
- [x] Extract shared adapter render-model helper and migrate React/Preact/Solid
      branch/matched/error selection to that host seam.
- [x] Extract shared adapter branch-kind selector helpers and migrate
      React/Preact/Solid render-kind branching to host-owned selectors.
- [x] Centralize root-outlet mount lifecycle sync policy in host
      (`syncRootOutletMount` + `shouldSyncRouteOutletRootMount`) and migrate
      React/Preact/Solid root-mount entry paths to that shared seam.
- [x] Preserve render-abort remount recovery semantics in shared root-mount sync
      policy by syncing on each root mount (listeners remain once-only).
- [x] Remove redundant adapter-host helper exports superseded by the render
      model (`resolveRouteOutletBranchRenderStateForAdapter`,
      `resolveRouteOutletMatchedPatternForAdapter`).

### D3: Adapter thin wrappers

- [x] React wrapper: host->React lifecycle bindings only.
- [x] Preact wrapper: host->Preact lifecycle bindings only.
- [x] Solid wrapper: host->Solid lifecycle bindings only.
- [x] Ensure wrappers do not own state-machine logic.

### D4: Files to update (primary)

- [x] `typescript/vorma/client/src/ui/*` shared host files
- [x] `typescript/vorma/ui-adapters/react/src/*`
- [x] `typescript/vorma/ui-adapters/preact/src/*`
- [x] `typescript/vorma/ui-adapters/solid/src/*`

### D5: Phase-D tests

- [x] Cross-adapter parity tests for identical transition traces.
- [x] Adapter conformance tests for route-props/global selector behavior.
- [x] Tests proving remount semantics are identical across adapters.
- [x] Cross-adapter data-only update stability test proving no extra remounts
      when route identity stays stable.

## Phase E: Cleanup + Dead Path Deletion

- [x] Delete all legacy seams superseded by A-D.
- [x] Delete temporary migration toggles and dual-path branches.
- [x] Delete tests that only encode removed seams.
- [x] Delete legacy runtime revalidation-mismatch/removal/clear-all seam modules
      and route those behaviors through `runtime_engine_state_machine.ts`.
- [x] Delete legacy unit tests tied only to removed seam modules and replace
      coverage with unified runtime-engine tests.
- [x] Ensure internal exports surface only end-state APIs.
- [x] Shrink `vorma/client/__internal` exports to adapter/runtime-required
      symbols and delete stale route-outlet listener/runtime passthrough exports
      no longer used by adapters.
- [x] Delete redundant adapter host branch-selector helper exports and switch
      adapters to direct discriminated render-model branching.
- [x] Delete silent matched-pattern fallback in shared adapter host and enforce
      missing-pattern invariant via hard failure.
- [x] Remove ongoing legacy snapshot-field mirror writes to top-level globals;
      read/write snapshot fields through canonical `runtimeRouteSnapshot`
      accessors.
- [x] Update contract/unit harness expectations to assert snapshot-authoritative
      reads/writes (`runtimeRouteSnapshot` or `__vormaClientGlobal`) instead of
      raw legacy top-level field mutation.
- [x] Update dist adapter test harness/state helpers to patch
      `runtimeRouteSnapshot` only and delete legacy top-level route-data fields
      from dist global installation state.
- [x] Update contract test global installer to strip legacy top-level route-data
      mirrors and emit snapshot-authoritative runtime state.
- [x] Remove remaining shared-runtime per-field route snapshot alias reads
      (`__vormaClientGlobal.get("buildID"|"importURLs"|"rootElementID"|...)`)
      and read route data directly from `getRuntimeRouteSnapshot()`.
- [x] Tighten `__vormaClientGlobal.set(...)` so route snapshot fields are not
      writable by per-field key; route writes must go through atomic
      `set("runtimeRouteSnapshot", ...)`.
- [x] Remove legacy top-level route-data mirrors from remaining unit/global test
      installers so they install snapshot-authoritative state only.
- [x] Update contract/unit tests that previously used per-field route snapshot
      writes to patch `runtimeRouteSnapshot` atomically.
- [x] Tighten `__vormaClientGlobal.get(...)` so route snapshot fields are not
      read by per-field key; route reads must come from
      `get("runtimeRouteSnapshot")` (or `getRuntimeRouteSnapshot()` in runtime
      code).
- [x] Update contract/unit tests that previously used per-field route snapshot
      reads to read from `runtimeRouteSnapshot` explicitly.
- [x] Remove legacy context fallback that reconstructed runtime snapshots from
      top-level globals when `runtimeRouteSnapshot` was missing.
- [x] Require `runtimeRouteSnapshot` bootstrap invariant and fail loud when
      missing.
- [x] Make route-data fields snapshot-only at the `__vormaClientGlobal` typing
      layer (non-snapshot global type now excludes route-data keys).
- [x] Seed `runtimeRouteSnapshot` directly from SSR bootstrap script to satisfy
      runtime invariant by construction.
- [x] Delete duplicated top-level SSR bootstrap assignments for route-data
      fields now owned by `runtimeRouteSnapshot`.
- [x] Delete unused top-level SSR bootstrap `deps`/`cssBundles` assignments;
      keep route-asset data on navigation payloads only.
- [x] Update unit/global test installers to derive `runtimeRouteSnapshot` from
      installed global state overrides.
- [x] Preserve render-performance characteristics during cleanup (no accidental
      extra renders/remount churn).
- [x] Preserve component stability guarantees (unchanged route scope/component
      pairs must not remount spuriously).

## Cross-Phase Invariant Set

- [x] No mixed old/new snapshot reads in one render tick.
- [x] No route-props out-of-scope data read is representable.
- [x] No stale ownership side effects can commit after ownership loss.
- [x] No malformed wire payload reaches commit/render logic.
- [x] No adapter-specific behavior divergence for shared semantics.

## Global Verification Gates

### Gate 1 (after A+B)

- [x] Run targeted source tests for decoder + RouteScope ownership.
- [x] Run targeted dist adapter transition-tick suites.

### Gate 2 (after C)

- [x] Run reducer/command seam suites and race coverage.
- [x] Run targeted parallel-loader behavior tests.

### Gate 3 (after D)

- [x] Run cross-adapter parity suites.
- [x] Run render-performance guard suites.
- [x] Run component-stability/remount guard suites.

### Final gate

- [x] Run `make tstest-source`.
- [x] Run `GOCACHE=/tmp/go-build go run ./internal/cmd/buildts`.
- [x] Run `make tstest-dist`.
- [x] Run targeted Go matcher/routeparse tests for canonical uniqueness.

## Concrete Tradeoffs

- [x] Tradeoff: very large coordinated refactor surface. Impact: sequencing risk
      managed by strict phase gates and per-phase test reruns.
- [x] Tradeoff: immutable models increase allocation pressure. Impact:
      structural-sharing canonicalization is required on snapshots/store state.
- [x] Tradeoff: short-lived temporary dual paths may increase code size. Impact:
      dual paths were deleted at phase boundaries and not retained.
- [x] Tradeoff: strict decoder/invariant firewall increases hard failures.
      Impact: expected fail-loud behavior for malformed route-data payloads.
- [x] Benefit: broad elimination of race/stale/ownership bug classes. Impact:
      tighter reducer/command seams and fewer distributed guard branches.

## In-File Organization Pass (No New Files)

- [x] Replace filename-style section banners with concept-first labels in
      `vorma/client` runtime and shared types.
- [x] Add explicit section boundaries for public API, URL/API helpers, snapshot
      store, and runtime access bridge in `vorma/client` runtime.
- [x] Move navigation runtime reason-tag types next to the navigation runtime
      state-machine type section so the domain model stays contiguous.
- [x] Move remaining mixed-concern blocks into dedicated conceptual sections
      (navigation reducer/planner/executor seams).
- [x] Dedupe repeated reason-tag/command-plan boilerplate exposed by grouping.

## Phase E2: Runtime DRY / De-dup Consolidation

Goal: identify and remove duplication in shipped/runtime paths and adjacent
maintenance seams without weakening invariants.

### Runtime Literal Duplication

- [x] Unify navigation outcome reason-tag source of truth.
- [x] Unify build-ID sync path.
- [x] Delete redundant typed-adapter client-loader data-resolver wrapper
      (`registerTypedAdapterClientLoaderWithDataResolver`) and route shared
      value-hook factory directly through core registration + resolver
      primitives.
- [x] Delete legacy runtime `__*` alias export seams (`__applyScrollState`,
      `__resolvePath`, `__registerClientLoaderForAdapter`,
      `__makeFinalLinkProps`, `__loadRouteManifestProgressively`,
      `__registerClientLoaderPattern`) and bind contract harness internals to
      canonical symbol names.
- [x] Remove trivial delete-navigation passthrough executor wrapper and call
      delete side effects directly from command execution branches.
- [x] Remove redundant prefetch-match wrapper and route begin-navigation
      planning directly through `findMapEntryByNavigationTarget`.
- [x] Collapse typed-link default-prop merge wrapper into
      `resolveTypedAdapterLinkWithDefaults`.
- [x] Route lifecycle runtime through direct transition builders and journal
      dispatch, removing the redundant reducer-dispatch hop in runtime
      execution.

### Runtime Snapshot Field Duplication

- [x] Consolidate runtime route-snapshot field lists and per-field handling into
      one descriptor-driven mechanism.
- [x] Collapse route-outlet next-state canonicalization to router-data identity
      stabilization only; rely on canonical runtime snapshot references for the
      remaining fields.
- [x] Centralize effective-error projection onto runtime snapshots via one
      helper (`applyEffectiveErrorDataToRuntimeRouteSnapshot`) and reuse it in
      client-loader and commit paths.

### Command Plan / Executor Duplication

- [x] Centralize shared command-plan executor pattern (`commands[]` + terminal
      result) across navigation/submission subsystems.
- [x] Remove duplicated lane-idle initializers.
- [x] Centralize shared delete-navigation side effect execution used by
      navigation outcome/pass/success executors.
- [x] Introduce shared sync command-plan executor utility and route
      begin-navigation + revalidation command-plan execution through it.
- [x] Introduce shared sync command-list executor utility and route
      navigation-pass/submission-lifecycle/navigation-engine command loops
      through it.

### Adapter Surface Duplication (React / Preact / Solid)

- [x] Centralize typed hook factory semantics shared across adapters.
- [x] Centralize typed link API semantics shared across adapters.
- [x] Centralize route-outlet mount orchestration skeleton where framework
      abstraction already exists.
- [x] Centralize typed `addClientLoader` registration/data-resolver closure in
      shared runtime and route React/Preact/Solid adapters through it.
- [x] Delete adapter-only route-outlet wrapper bridges now redundant in shared
      runtime (`resolveRouteOutletBranchRenderStateForAdapter`,
      `resolveRouteOutletMatchedPatternForAdapter`,
      `shouldRemountRouteOutletComponentMountForAdapter` function wrapper).
- [x] Centralize React/Preact `VormaLink` anchor prop derivation in shared
      runtime (`buildNavigationLinkAnchorRenderProps`).

### Test/Harness Duplication

- [x] Source contract harness snapshot key stripping now derives from runtime
      snapshot key exports (`runtimeRouteSnapshotFieldKeys`).

### Types Surface Duplication (`client/types.ts`)

- [x] Dedupe successful-navigation checkpoint union/plan/event shapes through
      shared ownership state aliases and checkpoint-by-key maps.
- [x] Dedupe repeated navigation callback signatures (`findNavigationEntry`,
      delete-navigation variants, successful-navigation processor) via shared
      type aliases.
- [x] Delete alias-only type indirection for link callbacks, runtime-engine
      ownership/phase, and path-resolution config (`TypedLinkOnClickCallback`,
      `NavigationRuntimeEngineNavigationPhase`,
      `NavigationRuntimeEngineOwnership`, `PathResolutionConfig`).

### DRY Cleanup Sequence

- [x] Implement reason-tag single source of truth with runtime const + type
      reuse strategy.
- [x] Implement descriptor-driven runtime snapshot field
      normalization/canonicalization/freezing.
- [x] Introduce one shared command-plan execution utility where command loops
      are structurally identical.
- [x] Collapse adapter typed-hook/link factories into shared generic factories
      with tiny framework wrappers.
- [x] Re-run `make tscheck`, `make tstest-source`, and `make tstest-dist` after
      each consolidation slice.
- [x] Remove duplicate initial-bootstrap component imports by passing loaded
      modules through to error-boundary setup.
- [x] Remove deterministic revalidation lane copy of in-flight target URL and
      read canonical in-flight target ownership from runtime engine state.
- [x] Delete dead typed-link default-merge wrapper
      (`resolveTypedAdapterLinkWithDefaults`) and inline into shared typed-link
      factory.
- [x] Delete dead exported helper (`getEffectiveErrorData`) superseded by
      snapshot-effective-error projection seam.
- [x] Delete unused `vorma/client/__internal` exports not used by adapters
      (`registerTypedAdapterClientLoader`,
      `resolveTypedAdapterLinkWithDefaults`,
      `stripNavigationInternalLinkPropsForAnchor`).
- [x] Inline typed-adapter client-loader registration into the shared
      add-client-loader hook factory and delete pass-through registration helper
      indirection.

## Footprint / Performance Gate

- [x] Record baseline shipped bundle outputs and key runtime timing metrics.
      Baseline before this cleanup pass:
      `npm_dist/typescript/vorma/client/chunk-I7OG2DE6.js` = 268410 bytes;
      `npm_dist/typescript/vorma/client/internal.js` = 1634 bytes.
- [x] Record post-refactor outputs after full legacy deletion. Post-cleanup:
      `npm_dist/typescript/vorma/client/chunk-ONRUOZAR.js` = 267920 bytes;
      `npm_dist/typescript/vorma/client/internal.js` = 1394 bytes.
- [x] Require non-inferior shipped footprint or explicit exception.
- [x] Require no regression in client-loader parallel start/reuse traits.
- [ ] Require no regression in render throughput/tick latency under transition
      stress tests.
- [x] Require no regression in component stability/remount guarantees for
      unchanged route scopes.

## Success Criteria

- [x] RouteScope + atomic snapshot fully replaces token ownership machinery.
- [x] Unified reducer+commands controls all navigation/submission side effects.
- [x] Wire decoder is sole ingress and invariant firewall.
- [x] Shared adapter host yields parity across React/Preact/Solid.
- [x] Render-performance and component-stability guarantees are preserved.
- [x] Transition-window stale-value bugs are eliminated by model, not patches.
- [ ] `make full-gate` passes.

## Opus Audit Closure Checklist

Goal: close every audit item explicitly, with no untracked drift.

### runtime.ts items

- [x] Keep one canonical reducer/engine runtime state model; delete legacy
      duplicated lifecycle-runtime lane/ownership tracking.
- [x] Keep canonical symbol exports; delete legacy `__*` alias exports
      (`applyScrollState`, `resolvePath`, `registerClientLoaderPattern`,
      `registerClientLoaderForAdapter`, `loadRouteManifestProgressively`,
      `makeFinalLinkProps`).
- [x] Keep one canonical ownership helper abstraction; delete legacy duplicated
      navigation/submission ownership helper pairs.
- [x] Keep one canonical URL-target classification helper
      (`classifyNavigationTargetAgainstCurrentLocation`); delete legacy
      overlapping wrappers (`hasSameDataTarget`, `hasSameNavigationTarget`,
      `isSameDocumentLocation`, `isSameDocumentHashChange`) where redundant.
- [x] Keep one canonical navigation-entry lookup stack; delete legacy duplicated
      lookup wrappers (`findNavigationEntryInNavigationLanes`,
      `matchNavigationLaneByTargetURL`, `findMapEntryByNavigationTarget`).
- [x] Keep one canonical build-ID sync command path; delete legacy wrapper
      layering and duplicate guard paths.
- [x] Keep canonical runtime snapshot canonicalization; delete legacy duplicate
      outlet-layer per-field canonicalization.
- [x] Keep canonical effective-error projection helper; delete legacy repeated
      inline derivation in commit/client-loader flows.
- [x] Keep one canonical revalidation ownership/state model in engine; delete
      legacy deterministic-lane duplicate in-flight state tracking.
- [x] Keep direct canonical delete command execution; delete legacy trivial
      `executeDeleteNavigationRuntimeCommand` wrapper.
- [x] Keep canonical typed-link resolver path; delete legacy extra default-merge
      wrapper layer.
- [x] Keep canonical single module-load result during bootstrap; delete legacy
      duplicate component/error-boundary load path.

### types.ts items

- [x] Keep one canonical location-state type; delete legacy duplicate
      `RuntimeLocationState`/`RouteOutletLocationState` split.
- [x] Keep minimal canonical lane-state types; delete legacy alias chain
      (`NavigationRuntimeLaneState`, `NavigationRuntimeNavigationLaneState`,
      `NavigationLanes`, `RuntimeLanes`) where semantically redundant.
- [x] Keep one canonical lane phase model; delete legacy duplicated phase enums
      except strict specializations that remain necessary.
- [x] Keep one canonical delete-navigation callback signature; delete legacy
      duplicated callback type variants.
- [x] Keep one canonical branch render union base; delete legacy adapter-model
      re-expression of branch cases.
- [x] Keep minimal canonical route-data type stack; delete legacy alias hops
      (`RenderSnapshot`, extra pass-through aliases) that add no semantics.
- [x] Keep one canonical render-state type; delete legacy duplicate
      `RouteOutletRuntimeRenderState` pick-layer clone.
- [x] Keep shared canonical path/config/input bases; delete legacy duplicated
      path/config/input type variants.
- [x] Keep one canonical link callback model; delete legacy overlap and
      vestigial subset callback types.
- [x] Keep only semantically meaningful type names; delete legacy plain aliases
      (`EligibleAnchorTargetClassification`,
      `RouteOutletBranchInputStateWithMatchedPatterns`,
      `TypedAdapterLinkMergedProps`).
- [x] Keep one canonical build-ID sync decision representation; delete legacy
      redundant timing enum indirection if command planning fully determines it.
- [x] Keep one canonical ownership model across runtime and types; delete legacy
      split ownership representations between engine state and entry checks.

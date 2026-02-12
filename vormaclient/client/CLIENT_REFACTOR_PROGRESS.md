# vormaclient/client Refactor Progress

## Snapshot

- Date: 2026-02-12
- Phase: aggressive internal cleanup with strict first-principles tests
- Status: all gates currently passing

## Current Source Of Truth

- Public behavior: `src/tests/contracts/`
- Internal branch and race hardening: `src/tests/unit/`
- Legacy tests: removed
- Rule: tests assert objectively correct behavior, never implementation quirks

## Runtime Structure

- Production modules:
    - `src/app/`
    - `src/core/`
    - `src/platform/`
    - `src/ui/`
- Test modules:
    - `src/tests/contracts/`
    - `src/tests/unit/`

## Major Completed Cleanup

- Navigation bookkeeping, matching, phase transitions, and status signaling are
  centralized around shared slot primitives in
  `src/core/navigation/runtime_slots.ts`.
- Navigation runtime decomposition completed with clear internal boundaries:
    - `src/core/navigation/runtime.ts`: runtime orchestration
    - `src/core/navigation/runtime_slots.ts`: slot state, slot mutation, and
      status signaling primitives
    - `src/core/navigation/runtime_navigation_outcome.ts`: outcome handling,
      redirect execution, successful-navigation rendering pipeline, and build-id
      sync
    - `src/core/navigation/runtime_submit.ts`: submission lifecycle, stale
      checkpoint handling, response classification, and auto-revalidation
- Top-level navigation-type dispatch is now centralized in
  `src/core/navigation/begin_navigation.ts` via `beginNavigation(...)`, removing
  dispatch duplication from `runtime.ts`.
- Duplicated per-type entry-construction branches in
  `src/core/navigation/begin_navigation.ts` are now collapsed into one shared
  `createControlEntry(...)` primitive used by active, prefetch, and revalidation
  control creation paths.
- User-navigation reuse selection in `src/core/navigation/begin_navigation.ts`
  now runs through explicit decide/execute helpers
  (`decideReusableUserNavigationEntryCandidate` ->
  `executeReusableUserNavigationEntryCandidate`) for active/prefetch/pending
  reuse and create-new fallback.
- `beginPrefetch` and `beginRevalidation` in
  `src/core/navigation/begin_navigation.ts` now also run through explicit
  decide/execute helpers:
    - `decideBeginPrefetchAction` -> `executeBeginPrefetchAction`
    - `decideBeginRevalidationAction` -> `executeBeginRevalidationAction`
- Navigation-control factory logic moved from
  `src/core/navigation/begin_navigation.ts` into
  `src/core/navigation/navigation_controls.ts`, with re-exports kept in
  `begin_navigation.ts` so existing imports remain stable.
- Begin-navigation decision and matching helpers moved from
  `src/core/navigation/begin_navigation.ts` into
  `src/core/navigation/begin_navigation_flow.ts`, reducing `begin_navigation.ts`
  to context wiring/orchestration.
- Fetch-route-data skip/client-only logic moved from
  `src/core/navigation/fetch_route_data.ts` into
  `src/core/navigation/fetch_route_data_skip.ts`, while `fetch_route_data.ts`
  now focuses on server fetch orchestration.
- Fetch-route-data server request/result/loader orchestration moved from
  `src/core/navigation/fetch_route_data.ts` into
  `src/core/navigation/fetch_route_data_server.ts`, reducing
  `fetch_route_data.ts` to top-level fast-path/server-path coordination.
- Fetch-route-data skip logic is now split between
  `src/core/navigation/fetch_route_data_skip_match.ts` (matching/eligibility
  decisions) and `src/core/navigation/fetch_route_data_skip.ts`
  (skip-result/client-only outcome construction).
- Fetch-route-data skip/server internals now read runtime-global state via
  explicit per-flow snapshot helpers, and repeated matcher-loop invariants now
  route through shared `getMatchedPatternsOrThrow` to reduce branch drift
  between skip and parallel-loader paths.
- Begin-navigation flow deduplicated in
  `src/core/navigation/begin_navigation.ts` (shared prefetch matching and shared
  promotion path).
- Link click/prefetch flow deduplicated in `src/core/links.ts`:
    - single eligible-anchor gate
    - explicit `aborted/redirect/success` handling
    - impossible nullable `NavigationControl.promise` branch removed
    - duplicate idle-prefetch guard removed
- URL normalization deduplicated with shared `resolveAbsoluteHref` in
  `src/platform/url.ts`, now used by navigation runtime, begin-navigation,
  links, history, and redirect parsing.
- Navigation target matching deduplicated with shared `hasSameNavigationTarget`
  in `src/platform/url.ts`, now used by navigation runtime, begin-navigation,
  and link prefetch lookup flows.
- Prefetch-cache target-key matching deduplicated with shared
  `findMapEntryByNavigationTarget` in `src/platform/url.ts`, now used by
  navigation runtime and begin-navigation lookup paths.
- Redirect logic deduplicated in `src/core/redirects.ts` via shared parser and
  strategy executor primitives.
- Render/runtime/context safety hardening completed:
    - explicit route JSON vs runtime-only state separation
    - optional-safe initialization branches for partially initialized globals
    - cross-realm global-instance detection in `src/platform/safety.ts`
- Build-time API cut applied: `route` no longer exported from runtime client
  entrypoint.
- Render-runtime loader race coverage expanded:
    - `src/tests/unit/render_runtime_internal.test.ts` now pins downstream
      loader abort propagation when an earlier client loader fails with a
      non-abort error.
- History POP fallback coverage expanded:
    - `src/tests/unit/history_listener_prelude.test.ts` now pins both
      cross-document POP fallback behavior (reload path) and successful POP
      state synchronization behavior.
    - same suite now also pins:
        - hard-reload error logging when browser reload throws
        - same-document POP hash-removal scroll restoration from stored state
        - same-document POP hash-removal fallback to origin scroll when no
          stored state exists
    - `src/platform/history.ts` hard-reload fallback now skips reload invocation
      in JSDOM environments to keep non-browser test runtimes deterministic.
- Submit-race stale checkpoint coverage expanded:
    - `src/tests/unit/navigation_runtime_internal.test.ts` now pins stale
      deduped submit outcomes at all reachable async checkpoints:
        - after response receipt/before finalize (build-id listener replacement)
        - after JSON parsing (replacement during parse)
        - before auto-revalidation (replacement during method access)
    - removed one unreachable stale checkpoint branch in
      `src/core/navigation/runtime.ts` redirect path (no async boundary existed
      between stale checks).
- Runtime slot-matching cleanup expanded:
    - removed impossible nullable prefetch-entry branch in
      `src/core/navigation/runtime.ts` slot matching.
    - added explicit no-op contract for `removeNavigation` when target key is
      absent in `src/tests/unit/navigation_runtime_internal.test.ts`.
- Runtime defensive submit handling hardened:
    - added explicit guard + contract for impossible missing submit response
      objects from redirect handling.
- Begin-navigation and fetch-route-data defensive coverage expanded:
    - `src/tests/unit/navigation_runtime_internal.test.ts` now pins:
        - same-target prefetch dedupe reuse from active navigation
        - same-target prefetch dedupe reuse from pending revalidation
        - build-id fallback (`"1"`) in client-loader server-data handoff when
          response build-id header is missing
        - sparse partial-match invariants now fail fast with explicit errors
          before starting parallel client loaders
        - production dep preloading that ignores falsy dep entries
        - uncached client-loader reuse guard (missing current snapshot cache)
          runs correctly without incorrectly seeding stale loader results
        - skip-check invariants now fail fast for malformed matcher results
          (sparse or empty route-pattern entries)
        - skip-check eligibility explicitly preserves skip behavior when
          outermost dynamic/splat params are unchanged (non-regression guards)
- Runtime/buildtime API boundary contracts added:
    - `src/tests/contracts/client.utilities.contract.test.ts` now pins:
        - runtime client entrypoint does not export buildtime-only `route`
        - buildtime entrypoint exports callable `route` registration API
- Consumer package build gate re-validated:
    - `make npmbuild` surfaced strict typing regressions in `vormaclient/solid`
      and `vormaclient/preact` adapter wrappers.
    - Fixed adapter typing at first principles (explicit typed memo/accessor
      outputs, event typing alignment, and component typing for renderer
      adapters) without adding compatibility shims.
    - `make npmbuild` now passes end-to-end again.
- Render-runtime branch hardening expanded:
    - `src/tests/unit/render_runtime_internal.test.ts` now pins:
        - explicit `scrollToTop: false` user-navigation behavior
        - empty-title fallback when title HTML payload is missing
        - null head-array normalization to empty arrays
        - malformed client-loader result handling when data arrays are missing
- Runtime response-artifact fallback hardening expanded:
    - `src/tests/unit/navigation_runtime_internal.test.ts` now pins behavior
      when response artifact arrays and module map are missing.
- Render-runtime internal cleanup expanded:
    - normalized client-loader inputs once per execution in
      `src/core/render_runtime.ts` instead of repeatedly applying ad hoc
      fallback checks inside loader loops.
    - `setClientLoadersState` now uses normalized loader data consistently for
      both state assignment and error-index derivation, avoiding malformed-data
      crashes and invalid negative indexes.
- Scroll-state defensive coverage expanded:
    - `src/tests/unit/scroll_state_refresh_state.test.ts` now pins parseable
      non-object snapshot rejection.
    - `src/tests/unit/scroll_apply_state.test.ts` now pins empty-hash no-op
      behavior for both implicit and explicit hash scroll paths.
- Navigation state-machine coverage expanded:
    - `src/tests/contracts/client.navigation_state_machine.contract.test.ts` now
      includes one seeded randomized mixed prefetch+navigate model sequence that
      resolves requests in randomized order and pins the invariant that
      last-started navigation remains authoritative.
- Runtime submit lifecycle cleanup expanded:
    - `src/core/navigation/runtime.ts` now models submit execution through a
      single submission-lifecycle primitive (`begin/finish/isCurrent`) and
      shared auto-revalidate gating, replacing duplicated lifecycle and stale
      checkpoint branching.
- Runtime slot bookkeeping cleanup expanded:
    - `src/core/navigation/runtime.ts` now keeps navigation slot state in one
      in-scope `slots` store inside `createNavigationRuntime`, removing the
      previous `createNavigationBookkeeping` wrapper layer while preserving the
      existing slot helper primitives and behavior.
- Begin-navigation branch cleanup expanded:
    - `src/core/navigation/begin_navigation.ts` now routes repeated target
      identity checks through a single `hasEntryWithSameNavigationTarget`
      helper, reducing duplicate conditional branching across user-navigation,
      prefetch, and revalidation paths.
- Runtime outcome pipeline cleanup expanded:
    - `src/core/navigation/runtime.ts` now resolves the target navigation entry
      once in `handleNavigationOutcome` and reuses shared entry-state predicates
      for redirect/success branch decisions, reducing duplicated entry lookup
      and branch logic.
- Runtime successful-navigation phase cleanup expanded:
    - `src/core/navigation/runtime.ts` now drives successful navigation through
      explicit decide/execute stage helpers across pre-waiting, post-waiting,
      and post-asset checkpoints, replacing ad hoc inline branching in
      `processSuccessfulNavigationRuntime`.
- Runtime redirect transition cleanup expanded:
    - `src/core/navigation/runtime.ts` now executes redirect outcomes through an
      explicit step helper (`ignore/effectuate`) in
      `handleRedirectOutcomeForEntry`, reducing inline side-effect/cleanup
      sequencing in `handleNavigationOutcome`.
- Runtime outcome decision cleanup expanded:
    - `src/core/navigation/runtime.ts` now computes a single explicit navigation
      outcome action (`deleteAndStop/stop/redirect/success`) in
      `decideNavigationOutcomeAction` and executes that action in
      `handleNavigationOutcome`, replacing ad hoc inline branching.
- Runtime outcome execution cleanup expanded:
    - `src/core/navigation/runtime.ts` now routes outcome handling through an
      explicit decide/execute pipeline (`decideNavigationOutcomeAction` ->
      `executeNavigationOutcomeAction`), separating decision logic from
      side-effect execution.
- Runtime outcome staleness guards expanded:
    - `src/core/navigation/runtime_navigation_outcome.ts` now requires
      control-promise identity match before applying aborted/redirect/success
      side-effects, preventing stale same-target outcomes from mutating or
      rendering over newer entries.
    - `src/core/navigation/runtime.ts` now also requires control-promise
      ownership in `navigate(...)` rejection cleanup before deleting target
      entries, preventing stale same-target fetch failures from deleting newer
      replacement entries.
    - `src/core/links.ts` now also requires control-promise ownership before
      link-outcome removal/processing, preventing stale same-target click
      outcomes from mutating newer entries.
    - `src/core/links.ts` non-prefetch link-click flow now catches rejected
      navigation promises and applies ownership-safe cleanup for current failed
      entries (while skipping stale failed-entry cleanup).
    - `src/tests/unit/navigation_runtime_internal.test.ts` now pins both stale
      and current control-promise behavior for aborted/success outcome handling.
    - `src/tests/unit/navigation_runtime_internal.test.ts` now also pins stale
      `navigate(...)` rejection behavior for same-target replacement flows.
    - `src/tests/unit/links_internal.test.ts` now pins stale aborted click
      outcomes as strict no-op behavior for navigation-state mutation.
    - `src/tests/unit/links_internal.test.ts` now also pins failed-link
      rejection cleanup behavior for both current and stale ownership cases.
    - `src/tests/contracts/client.link_click.contract.test.ts` now also pins
      non-throw link-click behavior and cleared navigating status when link
      navigation fetch fails.
    - `src/tests/unit/navigation_runtime_internal.test.ts` now also pins
      current-ownership cleanup in `navigate(...)` catch handling for reused
      prefetch rejection paths, covering the remaining runtime catch branch.
    - `src/core/navigation/runtime_navigation_outcome.ts` now gates successful-
      navigation phase transitions (`waiting` / `rendering` / `complete`) on
      current-entry identity, preventing stale same-target render callbacks from
      mutating newer replacement entry phases.
    - `src/tests/unit/navigation_runtime_internal.test.ts` now pins stale render
      `onFinish` callbacks as strict no-ops for newer same-target replacement
      entries.
    - `src/core/links.ts` now re-checks ownership after async `beforeRender`
      callbacks before applying redirect/render side effects, preventing stale
      outcomes from mutating replacement entries.
    - `src/tests/unit/links_internal.test.ts` now pins stale redirect outcomes
      as strict no-ops when ownership changes during awaited `beforeRender`
      callbacks.
- Redirect execution correctness expanded:
    - `src/core/redirects.ts` soft redirect execution now returns `did` only
      when `navigate(...)` reports `didNavigate: true`; failed soft redirects
      now resolve as non-effectuated (`null`) instead of being reported as
      completed redirects.
    - `src/core/navigation/runtime_submit.ts` now treats submit redirect
      non-effectuation as explicit failure
      (`{ success: false, error: "Redirect failed" }`) instead of returning
      success when redirect side effects did not happen.
    - `src/tests/unit/redirects_internal.test.ts` now pins both soft-redirect
      non-effectuation (`didNavigate: false`) and successful did-redirect
      (`didNavigate: true`) behavior.
    - `src/tests/unit/navigation_runtime_internal.test.ts` now pins explicit
      submit failure when redirect effectuation returns `null`.
    - `src/tests/contracts/client.submit_and_redirect.contract.test.ts` now pins
      explicit submit failure when redirect navigation fails, and updated
      overlapping different-key redirect-submit expectations now treat
      superseded stale redirect as explicit redirect failure.
- Redirect parsing robustness expanded:
    - `src/core/redirects.ts` now treats invalid redirect target URLs as
      non-redirect cases instead of throwing (covers malformed
      `X-Client-Redirect` values and malformed native `response.url` redirects).
    - `src/tests/unit/redirects_internal.test.ts` now pins invalid header/native
      redirect URL handling as no-throw null-redirect behavior.
- Navigation control/error staleness guards expanded:
    - `src/core/navigation/navigation_controls.ts` now requires entry-identity
      ownership checks before stale fetch-error cleanup mutates
      active/prefetch/revalidation slots.
    - `src/core/navigation/runtime_navigation_outcome.ts` now also requires
      entry-identity ownership checks across successful-navigation pre-waiting,
      post-waiting, post-asset, and final cleanup stages.
    - `src/tests/unit/navigation_runtime_internal.test.ts` now pins stale
      error-completion behavior for active/prefetch/revalidation same-target
      replacement flows, plus stale late-success non-deletion/non-rendering of
      newer same-target entries.
- Runtime submit classification staleness guards expanded:
    - `src/core/navigation/runtime_submit.ts` now performs an additional stale
      ownership check immediately after response classification and before
      executing classified response side effects.
    - `src/tests/unit/navigation_runtime_internal.test.ts` now pins response
      classification side effects that start replacement submits (`response.ok`)
      so stale submissions abort instead of returning classified error outcomes.
    - `src/core/navigation/runtime_submit.ts` now also performs stale ownership
      checks after awaited redirect effectuation and after awaited
      auto-revalidation navigation, so replacement submissions supersede
      in-flight stale work as `Aborted` outcomes.
    - `src/tests/unit/navigation_runtime_internal.test.ts` now also pins stale
      replacement takeover during redirect effectuation and during awaited
      auto-revalidation navigation.
- Runtime submit dead-branch cleanup expanded:
    - removed one unreachable stale-check branch in
      `src/core/navigation/runtime_submit.ts` where no async boundary existed
      between stale checks.
    - `src/tests/unit/navigation_runtime_internal.test.ts` now pins ownership
      loss during successful-navigation waiting-transition as a strict
      non-rendering stop path.
- Fetch-route-data boundary cleanup expanded:
    - removed internal re-export facades from
      `src/core/navigation/fetch_route_data.ts` and
      `src/core/navigation/fetch_route_data_skip.ts` that only mirrored skip
      internals.
    - `src/tests/unit/navigation_runtime_internal.test.ts` now imports
      skip-eligibility and skip-check APIs directly from `fetch_route_data_skip`
      / `fetch_route_data_skip_match`.
- HMR robustness expanded:
    - `src/core/extras.ts` now tracks HMR patterns by module pathname so one
      updated module shared by multiple route patterns refreshes matched client
      loaders for any tracked matching pattern.
    - `src/core/extras.ts` now sanitizes the runtime `matchedPatterns` snapshot
      during `vite:afterUpdate` handling and safely no-ops when unavailable,
      preventing HMR refresh crashes from malformed global state.
    - `src/tests/contracts/client.history_and_init.contract.test.ts` now pins
      same-path multi-pattern HMR refresh behavior while preserving one listener
      registration per module pathname.
    - `src/tests/contracts/client.history_and_init.contract.test.ts` now also
      pins no-throw/no-refresh behavior when `matchedPatterns` is unavailable.
    - `src/tests/contracts/client.history_and_init.contract.test.ts` now also
      pins no-throw behavior when `__runClientLoadersAfterHMRUpdate` receives a
      malformed `hot` runtime object without callable `.on`.
    - `src/core/extras.ts` now dedupes HMR listener registration per hot-runtime
      identity and pathname (via runtime-keyed weak maps), so same-path HMR
      listeners register correctly for new runtime instances across repeated
      init flows.
    - `src/core/extras.ts` hot-runtime resolution now validates `importMeta.hot`
      and falls back to `import.meta.hot` if the provided runtime object is
      malformed.
    - `src/tests/contracts/client.history_and_init.contract.test.ts` now also
      pins same-path HMR listener registration for new hot-runtime identities
      across repeated `initClient` calls.
- Global loading-indicator robustness expanded:
    - `src/tests/contracts/client.loading_and_focus.contract.test.ts` now pins
      pending start-timer cancellation when work finishes before start delay
      elapses, preventing delayed false-positive indicator start.
- Init robustness expanded:
    - `src/app/init.ts` progressive route-manifest loading now validates
      manifest payload shape and value domain (`0 | 1` flags), rejecting invalid
      payloads without mutating runtime manifest/pattern state.
    - `src/app/init.ts` progressive route-manifest loading now ignores stale
      older in-flight manifest responses from prior init calls.
    - `src/app/init.ts` now applies `useViewTransitions` explicitly on every
      init call to avoid stale option carry-over across repeated init runs.
    - `src/tests/contracts/client.history_and_init.contract.test.ts` now pins:
      invalid manifest-payload rejection, stale older-manifest suppression, and
      repeated-init `useViewTransitions` option application semantics.
- Init manifest-path robustness expanded:
    - `src/app/init.ts` now treats non-OK progressive manifest HTTP responses as
      manifest failures and leaves runtime manifest/registry state unchanged.
    - removed an unreachable null-registry defensive branch in `src/app/init.ts`
      (registry is established synchronously in init flow).
    - `src/tests/contracts/client.history_and_init.contract.test.ts` now pins:
      non-OK manifest response handling and invalid loader-flag rejection for
      manifest entries outside `0 | 1`.
    - `src/tests/contracts/client.history_and_init.contract.test.ts` now also
      pins pattern-registry replacement mid-flight so stale progressive manifest
      payloads are ignored when registry identity no longer matches.
- Head reconciliation robustness expanded:
    - `src/ui/head.ts` now validates managed section-marker ordering before any
      mutation and no-ops for malformed ranges (for example end marker before
      start marker), preventing accidental unrelated head-node deletion.
    - `src/ui/head.ts` now selects the nearest valid managed marker pair among
      head siblings, allowing valid section updates even when stray earlier
      markers exist.
    - `src/tests/contracts/head_elements.contract.test.ts` now pins malformed
      marker-range no-op behavior.
    - `src/tests/contracts/head_elements.contract.test.ts` now also pins
      nearest-valid-marker-pair selection when earlier stray markers exist.
- Runtime submit response cleanup expanded:
    - `src/core/navigation/runtime.ts` now routes submit response handling
      through an explicit decide/execute pipeline (`decideSubmitResponseAction`
      -> `executeSubmitResponseAction`), separating response classification from
      side-effect execution.
- Runtime submit runtime cleanup expanded:
    - `src/core/navigation/runtime.ts` now routes submit request lifecycle
      execution through explicit prepare/decide/execute helpers
      (`prepareSubmitRequest`, `decideSubmitPostRequestAction`,
      `executeSubmitPostRequestAction`, `decideSubmitRuntimeErrorAction`,
      `executeSubmitRuntimeErrorAction`), separating request preparation,
      post-request transitions, and error mapping.
- Runtime slot mutation cleanup expanded:
    - `src/core/navigation/runtime.ts` now routes slot deletion and phase
      transition primitives through explicit decide/execute helpers
      (`decideDeleteNavigationSlotAction`, `executeDeleteNavigationSlotAction`,
      `decideTransitionNavigationPhaseAction`,
      `executeTransitionNavigationPhaseAction`), separating match/decision from
      mutation side-effects.
- Link callback completion semantics hardened:
    - `src/core/links.ts` now gates redirect-path `afterRender` on redirect
      effectuation result `status: "did"` (no callback on non-effectuation
      `null` redirects).
    - `src/core/links.ts` now gates success-path `afterRender` on entry phase
      `complete` after navigation processing, preventing callback execution when
      stale/replaced flows stop before completion.
    - `src/tests/unit/links_internal.test.ts` now pins both cases:
      non-effectuated redirect and non-complete successful-processing paths must
      not execute `afterRender`.
- Local import-resolution tooling hardened:
    - `tsconfig.base.json` now maps `vorma/*` package subpaths directly to
      source paths via `compilerOptions.paths`.
    - `vitest.config.ts` now mirrors these aliases for deterministic test
      resolution without requiring `npm_dist` to exist.
    - mapping is compatible with both `tsc` and `tsgo` (no `baseUrl` usage).
- Navigation ownership checks deduplicated:
    - `src/core/navigation/types.ts` now exports
      `hasNavigationControlPromiseOwnership(...)` as the shared primitive for
      control-promise identity checks.
    - `src/core/navigation/runtime.ts`,
      `src/core/navigation/runtime_navigation_outcome.ts`, and
      `src/core/links.ts` now route ownership checks through this shared helper
      instead of ad hoc `entry.control.promise === controlPromise` branches.

## Current Test Inventory

- `28` test files
- `459` tests

## Latest Verified Gate (2026-02-12)

- `pnpm oxlint vormaclient/client/src`
- `pnpm tsc --noEmit --project vormaclient/client`
- `pnpm tsgo --noEmit --project vormaclient/client`
- `pnpm vitest --run vormaclient/client/src`
- `pnpm vitest --run vormaclient/client/src --coverage --coverage.reporter=text-summary`
- `make npmbuild`

Result:

- tests: pass (`28` files, `459` tests)
- coverage summary:
    - statements: `92.96%`
    - branches: `87.87%`
    - functions: `95.57%`
    - lines: `93.31%`

## Remaining Work

- Continue shrinking internal complexity in runtime/navigation/render code while
  keeping behavior pinned by strict contracts.
- Add targeted tests only where uncovered defensive branches correspond to real
  runtime risk (especially async ordering/race paths).
- Keep docs lean: update this file and `HIGH_LEVEL_PLAN.md` only with current
  state, not historical churn.

## Takeover Checklist

1. Read `HIGH_LEVEL_PLAN.md`.
2. Read this file.
3. Re-run the full validation gate above.

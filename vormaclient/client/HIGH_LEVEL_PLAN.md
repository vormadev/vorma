# vormaclient/client High-Level Plan

Purpose: keep the sequence explicit so takeover is safe and work does not drift.

## End State

- Maintainable client runtime with clear boundaries and lower internal
  complexity.
- Strict first-principles tests as the only behavioral source of truth.
- No legacy-test dependency and no compatibility cruft.

## Sequence

- [x]   1. Test dedup + strengthen contracts
- [x]   2. Add navigation model/state-machine coverage
- [x]   3. Add race-focused regressions
- [ ]   4. Continue aggressive internal refactor cleanup (**YOU ARE HERE**)
    - 2026-02-11: navigation/begin-navigation/link/redirect internals moved to
      shared primitives to remove duplicated branch trees.
    - 2026-02-11: shared `resolveAbsoluteHref` primitive now replaces repeated
      `new URL(..., window.location.href).href` normalization paths across
      runtime/begin-navigation/links/history/redirect parsing.
    - 2026-02-11: runtime branch handling moved to explicit discriminated
      `aborted/redirect/success` control flow.
    - 2026-02-11: runtime/global safety hardening added for optional init-order
      state and cross-realm body detection.
    - 2026-02-11: redirect parsing/execution moved to shared parser+executor
      primitives.
    - 2026-02-11: render-runtime now has explicit unit coverage for downstream
      client-loader abort propagation after upstream non-abort loader failures.
    - 2026-02-11: history runtime now has explicit POP fallback coverage for
      both failed client-nav reload path and successful cross-document POP state
      synchronization.
    - 2026-02-11: history hard-reload fallback now no-ops in JSDOM so tests stay
      deterministic while browser runtime behavior remains unchanged.
    - 2026-02-11: submit stale-checkpoint coverage now pins all reachable async
      dedupe race windows, and one unreachable redirect stale-checkpoint branch
      was removed from runtime submit flow.
    - 2026-02-11: runtime slot matching removed an impossible nullable prefetch
      lookup branch, and unit coverage now pins no-op `removeNavigation`
      behavior for absent keys.
    - 2026-02-11: runtime submit flow now has explicit coverage for impossible
      missing-response defensive handling and full runtime branch coverage.
    - 2026-02-11: render-runtime now has added contracts for explicit
      `scrollToTop: false` behavior, empty-title fallback, and null head-array
      normalization.
    - 2026-02-11: render-runtime loader execution now normalizes inputs once and
      uses consistent normalized loader state when deriving client-loader error
      indices, with strict malformed-data contracts added.
    - 2026-02-11: link click/prefetch internals simplified; impossible nullable
      `NavigationControl.promise` branch removed and duplicate idle-prefetch
      guard removed.
    - 2026-02-12: begin-navigation prefetch dedupe reuse paths are now pinned by
      strict unit coverage (active-navigation reuse and pending-revalidation
      reuse).
    - 2026-02-12: fetch-route-data defensive behavior now has strict unit
      coverage for build-id fallback in server-data handoff, sparse
      partial-match invariant enforcement before client-loader startup,
      production dep preloading that ignores falsy dep entries, and
      unseeded-cache client-loader behavior when current snapshots do not
      contain cached data.
    - 2026-02-12: contract coverage now explicitly enforces the
      runtime/buildtime API boundary for `route` (absent from runtime entry,
      present in buildtime entry).
    - 2026-02-12: matcher contracts in skip checks now fail fast on malformed
      matcher output (sparse matches and empty route-pattern entries) rather
      than silently continuing.
    - 2026-02-12: skip-eligibility contracts now explicitly pin unchanged
      outermost dynamic and splat params as non-violations, pushing
      `fetch_route_data.ts` to full branch coverage.
    - 2026-02-12: history POP contracts now also pin same-document hash-removal
      scroll restoration/fallback behavior and hard-reload error logging when
      browser reload throws.
    - 2026-02-12: scroll-state contracts now pin parseable non-object refresh
      snapshot rejection and empty-hash no-op behavior in `__applyScrollState`.
    - 2026-02-12: navigation state-machine contracts now include one seeded
      randomized mixed prefetch+navigate model sequence that resolves requests
      in randomized order and explicitly pins last-started navigation authority.
    - 2026-02-12: runtime/begin-navigation/link target matching now uses shared
      `hasSameNavigationTarget` to eliminate ad hoc exact-vs-data-target
      comparison logic.
    - 2026-02-12: prefetch-cache target-key lookup now uses shared
      `findMapEntryByNavigationTarget` in runtime/begin-navigation, replacing
      duplicated map-iteration matching loops.
    - 2026-02-12: runtime submit flow now uses an explicit submission-lifecycle
      primitive (`begin/finish/isCurrent`) and centralized auto-revalidate
      gating, replacing duplicated lifecycle/stale-check branching.
    - 2026-02-12: runtime navigation state now uses a single in-scope `slots`
      store in `createNavigationRuntime`, removing the extra
      `createNavigationBookkeeping` facade layer while preserving slot helper
      primitive behavior.
    - 2026-02-12: begin-navigation target comparison branches now share
      `hasEntryWithSameNavigationTarget`, removing repeated ad hoc
      `entry && hasSameNavigationTarget(...)` branching.
    - 2026-02-12: runtime outcome handling now resolves target entry once and
      routes redirect/success branches through shared entry-state predicates
      (`isIdlePrefetchEntry`, `shouldIgnoreRedirectOutcomeForEntry`).
    - 2026-02-12: successful-navigation phase progression now uses explicit
      pipeline helpers in runtime (`beforeWaiting`, `afterWaiting`,
      `postAssetStep`), reducing ad hoc branching in
      `processSuccessfulNavigationRuntime`.
    - 2026-02-12: redirect outcome execution now uses an explicit step helper
      (`ignore/effectuate`) through `handleRedirectOutcomeForEntry`, keeping
      side-effects and cleanup sequencing centralized.
    - 2026-02-12: `handleNavigationOutcome` now executes a compact outcome
      decision action (`deleteAndStop/stop/redirect/success`) from
      `decideNavigationOutcomeAction`, replacing ad hoc branch sequencing.
    - 2026-02-12: runtime outcome handling now has an explicit two-step pipeline
      (`decideNavigationOutcomeAction` then `executeNavigationOutcomeAction`) so
      decision logic and side-effects are isolated.
    - 2026-02-12: successful-navigation handling now also uses explicit
      decide/execute stage helpers at pre-waiting, post-waiting, and post-asset
      checkpoints in `processSuccessfulNavigationRuntime`.
    - 2026-02-12: submit response handling now also uses explicit decide/execute
      helpers (`decideSubmitResponseAction` then `executeSubmitResponseAction`)
      so response classification and side-effects are separated.
    - 2026-02-12: submit runtime request/error handling now also uses explicit
      prepare/decide/execute helpers (`prepareSubmitRequest`,
      `decideSubmitPostRequestAction`, `executeSubmitPostRequestAction`,
      `decideSubmitRuntimeErrorAction`, `executeSubmitRuntimeErrorAction`).
    - 2026-02-12: slot mutation primitives now also use explicit decide/execute
      helpers for deletion and phase transitions
      (`decideDeleteNavigationSlotAction`, `executeDeleteNavigationSlotAction`,
      `decideTransitionNavigationPhaseAction`,
      `executeTransitionNavigationPhaseAction`).
    - 2026-02-12: navigation runtime internals are now split into three cohesive
      modules: orchestration+slots in `src/core/navigation/runtime.ts`,
      outcome/success processing in
      `src/core/navigation/runtime_navigation_outcome.ts`, and submit lifecycle
      execution in `src/core/navigation/runtime_submit.ts`.
    - 2026-02-12: slot/status bookkeeping primitives are now extracted to
      `src/core/navigation/runtime_slots.ts`; `runtime.ts` now re-exports slot
      helper APIs used by unit tests while keeping orchestration focused.
    - 2026-02-12: top-level navigation-type dispatch moved into
      `src/core/navigation/begin_navigation.ts` as `beginNavigation(...)`,
      removing the final begin-dispatch switch from `runtime.ts`.
    - 2026-02-12: duplicated per-type entry-construction helpers in
      `begin_navigation.ts` were collapsed into one shared
      `createControlEntry(...)` primitive used by active/prefetch/revalidation
      control creation paths.
    - 2026-02-12: user-navigation reuse selection in `begin_navigation.ts` now
      uses explicit decide/execute helpers
      (`decideReusableUserNavigationEntryCandidate`,
      `executeReusableUserNavigationEntryCandidate`) for
      active/prefetch/pending/create-new paths.
    - 2026-02-12: `beginPrefetch` and `beginRevalidation` now also use explicit
      decide/execute helpers in `begin_navigation.ts`
      (`decideBeginPrefetchAction`/`executeBeginPrefetchAction`,
      `decideBeginRevalidationAction`/`executeBeginRevalidationAction`).
    - 2026-02-12: navigation-control factory logic was extracted from
      `begin_navigation.ts` into `src/core/navigation/navigation_controls.ts`,
      then re-exported from `begin_navigation.ts` to keep existing import sites
      stable.
    - 2026-02-12: begin-navigation decision/matching helpers were extracted into
      `src/core/navigation/begin_navigation_flow.ts`, leaving
      `begin_navigation.ts` focused on context wiring and high-level
      orchestration.
    - 2026-02-12: fetch-route-data skip/client-only logic was extracted into
      `src/core/navigation/fetch_route_data_skip.ts`; `fetch_route_data.ts` now
      focuses on server fetch orchestration.
    - 2026-02-12: fetch-route-data server request/result/loader orchestration
      was extracted into `src/core/navigation/fetch_route_data_server.ts`;
      `fetch_route_data.ts` now primarily coordinates skip fast-path vs
      server-path execution.
    - 2026-02-12: fetch-route-data skip logic was split into
      matching/eligibility decisions in
      `src/core/navigation/fetch_route_data_skip_match.ts` and
      skip-result/client-only outcome construction in
      `src/core/navigation/fetch_route_data_skip.ts`.
    - 2026-02-12: fetch-route-data skip/server internals now read runtime-global
      state through explicit snapshot helpers, and repeated matcher-loop
      invariants now route through shared `getMatchedPatternsOrThrow`.
    - 2026-02-12: navigation outcome handling now requires control-promise
      identity match before applying aborted/redirect/success side-effects,
      preventing stale same-target outcomes from deleting or rendering over
      newer entries.
    - 2026-02-12: stale fetch-error cleanup in navigation control factories
      (active/prefetch/revalidation) now enforces entry identity before slot
      mutation, and successful-navigation processing now enforces entry identity
      at pre-waiting, post-waiting, post-asset, and final cleanup.
    - 2026-02-12: submit runtime now enforces staleness again immediately after
      response classification (before classified response side effects),
      preventing replacement-triggered stale submits from returning classified
      error/redirect outcomes.
    - 2026-02-12: removed an unreachable submit stale-check branch with no async
      boundary in `runtime_submit.ts`, and added explicit waiting-transition
      ownership-loss coverage so successful-navigation processing stops without
      render/cleanup when entry ownership is lost during phase transition.
    - 2026-02-12: removed internal fetch-route-data re-export facades
      (`canSkipServerFetch`/`isSkipEligibilityViolated` passthroughs) and moved
      tests to direct module imports (`fetch_route_data_skip` /
      `fetch_route_data_skip_match`) so boundaries stay explicit.
    - 2026-02-12: HMR loader refresh tracking now handles multiple route
      patterns bound to the same updated module pathname by tracking all
      patterns per pathname while still deduping listener registration.
    - 2026-02-12: init-time progressive route-manifest loading now validates
      payload shape/values, rejects invalid manifest responses, and ignores
      stale older in-flight manifest responses from prior init runs.
    - 2026-02-12: init options now apply `useViewTransitions` explicitly on
      every init call (`true` only when requested), avoiding stale option carry
      over across repeated init calls.
    - 2026-02-12: route-manifest progressive loading now also treats non-OK HTTP
      responses as manifest failures (no state mutation), and an unreachable
      registry-null defensive branch in `init.ts` was removed.
    - 2026-02-12: head reconciliation now validates section-marker ordering
      before mutation; malformed marker ranges (for example end-before-start)
      now safely no-op instead of risking unrelated head-node deletion.
    - 2026-02-12: head reconciliation marker lookup now selects the nearest
      valid start/end marker pair among head siblings so stray earlier markers
      do not block valid section updates.
    - 2026-02-12: HMR route-loader refresh now sanitizes the `matchedPatterns`
      snapshot and safely no-ops when unavailable instead of throwing during
      `vite:afterUpdate` processing.
    - 2026-02-12: `navigate(...)` rejection cleanup now requires control-promise
      ownership before deleting a target entry, preventing stale same-target
      fetch failures from deleting newer replacement entries.
    - 2026-02-12: link-click outcome handling now also requires control-promise
      ownership before removing or processing target entries, preventing stale
      same-target click outcomes from mutating newer replacement entries.
    - 2026-02-12: non-prefetch link-click flow now catches rejected navigation
      promises, performs ownership-safe cleanup for current failed entries, and
      skips cleanup for stale rejected entries.
    - 2026-02-12: link-click contracts now pin non-throw behavior and cleared
      navigating state when link navigation fetch fails.
    - 2026-02-12: runtime navigate-catch ownership branch for reused prefetch
      failures is now explicitly pinned, ensuring current-entry cleanup when the
      rejected promise still owns the target slot.
    - 2026-02-12: soft redirect execution now returns `did` only when
      `navigate(...)` reports `didNavigate: true`; failed soft redirects now
      resolve as non-effectuated (`null`) instead of being misreported as
      completed redirects.
    - 2026-02-12: progressive manifest loading now has explicit contract
      coverage for pattern-registry replacement mid-flight, ensuring stale
      manifest payloads are ignored when registry identity changes.
    - 2026-02-12: redirect target parsing now ignores invalid URL targets
      (header/native redirect values) without throwing, preserving base response
      handling when redirect metadata is malformed.
    - 2026-02-12: submit redirect handling now treats non-effectuated redirect
      execution as explicit failure (`"Redirect failed"`) instead of reporting
      success when redirect side effects did not happen.
    - 2026-02-12: submit/redirect contracts now pin redirect-navigation failure
      as explicit submit failure, and overlapping different-key submit redirects
      now explicitly expect superseded redirect failure for the stale submit.
    - 2026-02-12: HMR hook contracts now pin invalid hot-runtime no-op behavior
      (`hot` without callable `.on`) so update hooks remain safe under malformed
      dev-runtime metadata.
    - 2026-02-12: global loading-indicator contracts now pin start-timer
      cancellation when work completes before the configured start delay,
      preventing delayed false-positive loading starts.
    - 2026-02-12: HMR listener tracking now dedupes per hot-runtime identity
      plus pathname (not pathname alone), so repeated init flows with a new
      runtime re-register same-path listeners correctly.
    - 2026-02-12: hot-runtime resolution now validates `importMeta.hot` and
      falls back to `import.meta.hot` when the provided runtime object is
      malformed.
    - 2026-02-12: history/init contracts now pin same-path listener
      re-registration when hot-runtime identity changes across repeated init
      calls.
    - 2026-02-12: successful-navigation phase transitions now use current-entry
      identity checks before mutating phase state, preventing stale render
      callbacks from marking newer same-target entries as complete.
    - 2026-02-12: runtime unit coverage now pins stale render `onFinish`
      callbacks as strict no-ops for newer same-target entries.
    - 2026-02-12: link click outcome handling now re-checks navigation entry
      ownership after awaiting `beforeRender`, preventing stale callbacks from
      applying redirect/render side effects to replacement entries.
    - 2026-02-12: links unit coverage now pins stale redirect outcomes caused by
      ownership changes during async `beforeRender` callbacks as strict no-ops.
    - 2026-02-12: submit runtime now re-checks submission ownership after
      awaiting redirect effectuation, so deduped replacement submits can
      supersede in-flight redirect handling as `Aborted` outcomes.
    - 2026-02-12: submit runtime now re-checks submission ownership after
      awaited auto-revalidation navigation, so deduped replacement submits
      supersede in-flight revalidation follow-ups as `Aborted` outcomes.
    - 2026-02-12: submit stale-checkpoint unit coverage now also pins
      replacement takeover during redirect effectuation and during awaited
      auto-revalidation navigation.

## Current Validation Gate

- `pnpm oxlint vormaclient/client/src`
- `pnpm tsc --noEmit --project vormaclient/client`
- `pnpm tsgo --noEmit --project vormaclient/client`
- `pnpm vitest --run vormaclient/client/src`
- `pnpm vitest --run vormaclient/client/src --coverage --coverage.reporter=text-summary`

## Non-Negotiable Rules

- Tests must assert first-principles-correct behavior only.
- If strict test fails and behavior is objectively wrong, fix production code.
- Do not weaken tests to mirror implementation quirks.
- For genuinely ambiguous behavior, ask the user immediately.
- Do not add back-compat adapters while sub-1.0 unless explicitly requested.

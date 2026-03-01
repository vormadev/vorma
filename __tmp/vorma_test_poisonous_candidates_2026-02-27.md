# Vorma Poisonous Test Candidate Shortlist (Phase 1)

Generated from `vorma_test_should_mapping_2026-02-27.md`.

- Candidate backend-contract distrust rows: 12
- Candidate non-browser resiliency rows: 84
- Candidate internal-coupled rows: 322

This is a triage list for discussion before any contract expectation changes.

## Candidate: Backend-Contract Distrust Assertions

- typescript/vorma/client/src/tests/contracts/client.error_and_edge.contract.test.ts
    - line 99: treats empty JSON response as failed and then recovers
- typescript/vorma/client/src/tests/contracts/client.prefetch.contract.test.ts
    - line 378: recovers from beforeBegin prefetch callback failures and allows
      retry
- typescript/vorma/client/src/tests/dist/npm_dist_adapters_root_outlet_branches.test.ts
    - line 120: react covers fallback, empty, custom error boundary, and default
      error branches
    - line 219: preact covers fallback, empty, custom error boundary, and
      default error branches
    - line 311: solid covers fallback, empty, custom error boundary, and default
      error branches
- typescript/vorma/client/src/tests/unit/navigation_runtime_internal.test.ts
    - line 3409: uses build-id fallback in server route-data request URLs
- typescript/vorma/client/src/tests/unit/render_runtime_internal.test.ts
    - line 305: builds active components using default export keys and null
      fallback
- typescript/vorma/client/src/tests/unit/route_outlet_adapter_host_runtime_internal.test.ts
    - line 27: resolves adapter render models for component/fallback/error
      branches
- typescript/vorma/client/src/tests/unit/route_outlet_runtime_internal.test.ts
    - line 125: resolves branch render state for component, fallback, and error
      branches
- typescript/vorma/client/src/tests/unit/scroll_state_refresh_state.test.ts
    - line 7: removes malformed page-refresh snapshots
- typescript/vorma/client/src/tests/unit/scroll_state_storage.test.ts
    - line 15: drops malformed stored maps instead of keeping corrupt state
    - line 22: can save fresh state after malformed stored data is cleared

## Candidate: Non-Browser Resiliency Assertions

- typescript/vorma/client/src/tests/contracts/client.error_and_edge.contract.test.ts
    - line 280: rejects client-loader serverDataPromise with AbortError when
      required server loader payload is missing
- typescript/vorma/client/src/tests/contracts/client.history_and_init.contract.test.ts
    - line 114: applies hash scroll on same-document POP updates
    - line 151: applies decoded hash scroll on same-document POP updates
    - line 262: triggers browser-history navigation fetch for cross-document POP
    - line 301: follows cross-document POP redirects and renders redirected
      destination
    - line 372: uses listener location payload as the source of truth for
      cross-document POP target
    - line 413: saves scroll state before moving to a different document
    - line 479: saves scroll state on cross-document POP before restoring the
      target document
    - line 519: restores saved scroll position when POP removes a hash from the
      same document
- typescript/vorma/client/src/tests/contracts/client.link_click.contract.test.ts
    - line 148: does not save scroll state for modifier-key same-document hash
      clicks
    - line 192: handles same-document hash removal links without navigation
      fetch
    - line 217: does not trigger navigation for same-document no-op hash links
    - line 239: prevents default for same-document no-op links without hash and
      does not fetch
    - line 277: does not treat cross-origin hash links as same-document hash
      changes
- typescript/vorma/client/src/tests/contracts/client.loading_and_focus.contract.test.ts
    - line 1116: resets focus stale-time window after successful navigation
    - line 1157: does not reset focus stale-time window for hash-only
      programmatic navigations
    - line 1231: resets focus stale-time window after successful revalidation
    - line 1330: stops listening after cleanup from revalidateOnWindowFocus
- typescript/vorma/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts
    - line 62: sets active error boundary from server error index and error
      export key
    - line 92: falls back to default error boundary when server index has no
      error component
    - line 163: passes matched server data into registered client wait functions
    - line 203: uses server fetch when current client snapshot lacks required
      loader data
- typescript/vorma/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts
    - line 376: uses document.startViewTransition for user navigation when
      enabled
    - line 616: decodes HTML entities before updating document.title
- typescript/vorma/client/src/tests/contracts/client.navigation_modes.contract.test.ts
    - line 85: follows server redirects and renders redirected destination
    - line 177: does not re-follow X-Client-Redirect targets that are
      same-document current locations
    - line 458: does not fetch on programmatic same-document hash-only
      navigation
    - line 487: treats programmatic same-document no-op targets as no-op without
      fetch
- typescript/vorma/client/src/tests/contracts/client.prefetch.contract.test.ts
    - line 250: cancels pending prefetch timer on same-document hash removal
      click
    - line 277: cancels pending prefetch timer on same-document no-op hash click
    - line 306: prevents default on same-document no-op prefetch clicks without
      hash
    - line 736: rejects serverDataPromise with AbortError for failed prefetch
      responses
    - line 767: rejects serverDataPromise with AbortError when server omits a
      prestarted matched pattern
- typescript/vorma/client/src/tests/contracts/client.state_and_revalidation.contract.test.ts
    - line 214: keeps same-target revalidation status when user navigation is a
      same-document no-op
- typescript/vorma/client/src/tests/contracts/client.utilities.contract.test.ts
    - line 122: registers listeners on window for all event types
    - line 175: uses configured root element id from SSR runtime state
    - line 385: prevents default for same-document no-op clicks through
      \_\_makeFinalLinkProps
    - line 403: prevents default for same-document no-op prefetch clicks through
      \_\_makeFinalLinkProps
- typescript/vorma/client/src/tests/dist/npm_dist_adapters_root_outlet_runtime_state.test.ts
    - line 3163: react useClientLoaderData(routeProps) stays fresh across
      transition windows for a stable route scope
    - line 3375: react useLoaderData(routeProps) tracks current route scope
      through transition windows
    - line 3465: preact useLoaderData(routeProps) tracks current route scope
      through transition windows
    - line 3549: preact useClientLoaderData(routeProps) stays fresh across
      transition windows for a stable route scope
    - line 3636: solid useLoaderData(routeProps) tracks current route scope
      through transition windows
    - line 3722: solid useClientLoaderData(routeProps) stays fresh across
      transition windows for a stable route scope
- typescript/vorma/client/src/tests/unit/events_platform.test.ts
    - line 10: dispatchStatusEvent does not throw when window is unavailable
    - line 25: addStatusListener returns a safe cleanup when window is
      unavailable
    - line 36: dispatches status detail to listeners when window is available
- typescript/vorma/client/src/tests/unit/fetch_route_data_server_internal.test.ts
    - line 341: returns aborted outcome when server response is missing
    - line 518: starts matched loader wait functions and forwards resolved
      server data
    - line 589: starts speculative loader work before the server route-data
      promise resolves
    - line 661: converts server promise rejection into unavailable-server-data
      abort errors
- typescript/vorma/client/src/tests/unit/hash_fragment.test.ts
    - line 177: detects same-document hash-only transitions
    - line 212: detects same-document location no-op targets
    - line 237: classifies same-document targets as noop, hash-change, or
      navigate
- typescript/vorma/client/src/tests/unit/history_listener_prelude.test.ts
    - line 77: treats POP updates on the same data target as same-document
    - line 116: never flags non-POP actions as same-document POP
    - line 135: reloads the browser when cross-document POP navigation cannot be
      handled by client navigation
    - line 228: keeps history location in sync after successful cross-document
      POP navigation
    - line 322: restores stored scroll coordinates when same-document POP
      removes hash
    - line 359: falls back to origin scroll when same-document POP removes hash
      without stored state
- typescript/vorma/client/src/tests/unit/links_internal.test.ts
    - line 87: treats same-document hash changes as runtime-only navigations
      without link callbacks
    - line 112: treats same-document no-op targets as runtime-only no-op
      navigations without link callbacks
- typescript/vorma/client/src/tests/unit/navigation_revalidation_lane_internal.test.ts
    - line 102: plans trailing-pass scheduling when the trailing window is open
    - line 158: reuses in-flight promise before trailing window opens
    - line 186: queues at most one trailing pass once trailing window opens
- typescript/vorma/client/src/tests/unit/navigation_runtime_internal.test.ts
    - line 2096: commits same-document hash-only navigations without fetch and
      without resolving freshness intent
    - line 2118: treats programmatic same-document no-op targets as no-op
      without fetch
    - line 3216: always fetches server route data even for client-only manifest
      routes
    - line 3280: uses the current route-data payload server error index for
      fetch-time client-loader skipping
    - line 3337: handles server fetch outcomes when client-loader map is
      undefined
    - line 3373: fails fast when server JSON omits required importURLs
    - line 3409: uses build-id fallback in server route-data request URLs
    - line 3451: falls back to build-id 1 when loader server-data response
      header is missing
    - line 3587: aborts waiting client loaders when server JSON omits required
      matched loader arrays
    - line 3639: maps server fetch rejections to unavailable server-data for
      client loaders
    - line 3892: throws explicit error when server route-data request returns
      304
- typescript/vorma/client/src/tests/unit/render_runtime_internal.test.ts
    - line 212: derives effective error from the lower server/client error index
    - line 274: clears derived error state when no server/client error exists
    - line 392: returns null when client-loader server data is missing pattern
      match
    - line 446: uses the current payload server error index for client-loader
      skip decisions
    - line 750: passes unavailable server data to client loaders when required
      server payload is missing
- typescript/vorma/client/src/tests/unit/revalidation_focus_trigger_policy_state_machine_internal.test.ts
    - line 50: blocks focus revalidate when stale window has not elapsed
    - line 65: allows focus revalidate exactly at stale window boundary
- typescript/vorma/client/src/tests/unit/scroll_state_refresh_state.test.ts
    - line 45: restores recent snapshots for encoding-equivalent same-document
      hash URLs

## Candidate: Internal-Coupled Coverage (Likely Rewrite/Delete)

- Files with highest internal-coupled concentration (top 20 by case count):
- typescript/vorma/client/src/tests/unit/navigation_runtime_internal.test.ts: 78
- typescript/vorma/client/src/tests/unit/render_runtime_internal.test.ts: 35
- typescript/vorma/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts:
  26
- typescript/vorma/client/src/tests/unit/fetch_route_data_server_internal.test.ts:
  20
- typescript/vorma/client/src/tests/unit/redirects_internal.test.ts: 14
- typescript/vorma/client/src/tests/unit/route_outlet_runtime_internal.test.ts:
  14
- typescript/vorma/client/src/tests/unit/submission_lifecycle_commands_internal.test.ts:
  12
- typescript/vorma/client/src/tests/unit/begin_navigation_state_machine_internal.test.ts:
  11
- typescript/vorma/client/src/tests/unit/typed_adapter_helpers_runtime_internal.test.ts:
  11
- typescript/vorma/client/src/tests/unit/navigation_runtime_engine_state_machine_internal.test.ts:
  10
- typescript/vorma/client/src/tests/unit/begin_navigation_runtime_internal.test.ts:
  9
- typescript/vorma/client/src/tests/unit/navigation_revalidation_lane_internal.test.ts:
  9
- typescript/vorma/client/src/tests/unit/navigation_lifecycle_transitions_internal.test.ts:
  8
- typescript/vorma/client/src/tests/unit/links_internal.test.ts: 7
- typescript/vorma/client/src/tests/unit/route_outlet_adapter_host_runtime_internal.test.ts:
  7
- typescript/vorma/client/src/tests/unit/navigation_outcome_runtime_internal.test.ts:
  6
- typescript/vorma/client/src/tests/unit/navigation_runtime_slots_internal.test.ts:
  5
- typescript/vorma/client/src/tests/unit/revalidation_focus_trigger_policy_state_machine_internal.test.ts:
  5
- typescript/vorma/client/src/tests/contracts/client.link_click.contract.test.ts:
  4
- typescript/vorma/client/src/tests/unit/client_runtime_initialization.test.ts:
  4

## Next Step

- Manually review each candidate row and decide: keep, rewrite black-box, or
  delete.
- Pause for user sign-off on any ambiguous semantic behavior before changing
  contract expectations.

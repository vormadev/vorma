# Vorma "Should" Audit Tracker (2026-02-27)

This tracker keeps the migration explicitly tied to the full extracted "should"
statements.

Primary source of truth:

- `__tmp/vorma_test_should_mapping_2026-02-27.md`
- two-category full audit: `__tmp/vorma_two_category_full_audit_2026-02-27.md`
- must-be-ported execution tracker:
  `__tmp/vorma_must_be_ported_to_black_box_tracker_2026-02-27.md`

Current denominator model (collapsed to two categories):

- `zero_value_due_to_duplication_or_non_observability`: `134`
- `must_be_ported_to_black_box`: `684`
- total rows: `818`

Handled accounting rules:

- `zero_value_due_to_duplication_or_non_observability` is implicitly handled by
  final categorization rule.
- `must_be_ported_to_black_box` is handled only when work status is `handled` in
  `vorma_must_be_ported_to_black_box_tracker_2026-02-27.md`.

Current handled snapshot:

- `zero_value_due_to_duplication_or_non_observability`: `134/134`
- `must_be_ported_to_black_box`: `425/684`
- overall handled: `559/818`

Full-audit validation checks (completed):

- `__tmp/vorma_test_should_mapping_2026-02-27.md` rows: `818`
- `__tmp/vorma_two_category_full_audit_2026-02-27.md` rows: `818`
- `zero_value_due_to_duplication_or_non_observability` rows are all in legacy
  `unit` tests (`134/134`), with `0` contract/dist rows miscategorized there.
- `must_be_ported_to_black_box` tracker rows: `684`, exactly matching non-zero
  category rows in the full audit file.
- Dist execution is restored for newly ported adapter link rows in
  `npm_dist_black_box_authoritative.test.ts`; previously blocked rows are now
  marked `handled`.

## Current Ported Black-Box Coverage

Black-box suite:

- `typescript/vorma/client/src/tests/black_box/client.black_box_authoritative.test.ts`

Currently ported and passing:

- navigation commit updates location/title
- location listener dispatch + cleanup behavior
- public history instance is exposed and usable
- direct history pushes stay outside Vorma navigation lifecycle
- location events fire when direct history pushes change history keys
- direct history push state is reflected through `getLocation`
- direct history replaces stay outside Vorma navigation lifecycle
- same-document POP applies hash-element scroll
- same-document POP hash lookup uses one decode step
- same-document POP resolves encoded unicode hash targets
- same-document POP does not re-scroll for encoding-equivalent hash targets
- cross-document POP fetches route data and commits destination snapshot
- cross-document POP follows redirects and commits redirected destination
- cross-document POP uses listener payload URL as fetch source-of-truth
- cross-document POP saves outgoing scroll state before commit
- same-document POP restores saved scroll state when hash is removed
- same-document POP restores saved scroll state for empty-fragment (`#`) targets
- programmatic navigation saves outgoing scroll state before pushing a new entry
- malformed client-side scroll-state storage entries are ignored while valid
  entries remain usable
- scroll-state storage keeps a 50-entry cap with oldest-first eviction
- route-change fires after title commit
- route-change emits hash scroll state for hash-only navigation
- route-change emits top scroll state for standard navigation
- route-change emits undefined scroll state when `scrollToTop` is disabled
- latest-started navigation wins race
- same-document no-op navigation does not fetch
- same-document no-op with `replace=true` updates history state without fetch
- `replace=true` programmatic navigation uses history replace semantics
- cross-origin `vormaNavigate` fails loud and does not fetch
- cross-origin `submit` fails loud and does not fetch
- no duplicate idle status snapshots for same-document no-op
- hash-only same-document navigation commits without fetch
- navigating status reports while navigation is in flight
- submitting status reports during submit without auto-revalidate
- `getStatus()` returns synchronous in-flight status snapshots
- revalidation status in-flight then idle
- navigation and revalidation statuses can be simultaneously active
- same-document no-op navigations do not clear in-flight revalidation status
- rapid same-target revalidate calls coalesce into one fetch
- revalidation target change does not coalesce with older in-flight revalidate
- revalidation does not push/replace browser history
- revalidation uses current URL including search params
- revalidation redirects are followed when still current
- stale revalidation redirects are dropped after ownership moves to newer target
- stale revalidation hard-reload/build-id side effects are dropped after
  ownership moves
- stale revalidation data/head side effects are dropped after external location
  changes
- stale revalidation responses do not update build ID after external location
  changes
- stale revalidation responses do not trigger hard reload after external
  location changes
- stale revalidation redirects are dropped after external location changes
- stale native revalidation redirects are dropped after external location
  changes
- in-flight revalidation commits still apply across hash-only location changes
- search-param location changes invalidate in-flight revalidation commits
- revalidate-on-focus honors staleTime threshold
- revalidate-on-focus cleanup detaches listener behavior
- URL builders produce expected query/mutation targets
- navigation route-data requests include `vorma_json` query marker
- programmatic navigation applies explicit search/hash URL parts
- typed navigate path building + options forwarding
- typed API client query/mutate submit result contract + decorator behavior
- default error boundary formats route errors
- `getRootEl` resolves default root id and throws when missing
- `initClient` can rebind root element id for `getRootEl`
- `initClient` removes internal `vorma_reload` query parameter from URL
- `initClient` sets browser history `scrollRestoration` to `manual` when
  supported
- repeated `initClient` calls do not multiply beforeunload scroll-persistence
  writes
- `initClient` restores recent page-refresh scroll state for same-URL snapshots
- `initClient` ignores page-refresh scroll snapshots for different URLs
- `initClient` ignores stale page-refresh scroll snapshots older than the
  freshness window
- global loading indicator start/stop around navigation
- global loading indicator respects navigation-only include filters
- global loading indicator respects revalidation-only include filters
- global loading indicator respects submitting-only include filters
- navigate `skipGlobalLoadingIndicator` suppresses indicator start/stop
- submit `skipGlobalLoadingIndicator` suppresses indicator start/stop
- submit success contract (JSON)
- submit deduplicates same-key in-flight operations (older aborted, newer wins)
- submit dedupe handoff keeps loading continuous (no idle gap before final)
- stale same-key deduped submit superseded during json parsing resolves as
  `Aborted`
- submitting state clears after deduped replacement submission failure
- submit does not deduplicate across different dedupe keys
- stale older submit redirects are dropped when newer submit already won
- stale older submit hard-reload/build-id side effects are dropped when newer
  submit already won
- stale same-key deduped redirect responses are ignored after replacement wins
- stale same-key deduped responses cannot change build ID or trigger hard reload
- submit without dedupe key runs independently (no forced dedupe)
- submit failure contract (non-OK)
- submit preserves `BodyInit` payloads and serializes object bodies
- successful non-JSON submit returns text payload
- 204 submit returns success with undefined payload
- null-body 200 submit without content type returns success with undefined
  payload
- default non-GET submit auto-revalidates
- stale same-key deduped submit responses do not trigger revalidation
- GET submit does not auto-revalidate
- submit omits body for GET/HEAD/implicit-GET methods
- submit -> auto-revalidate keeps loading continuity
- build-id listener old/new event contract
- getBuildID reflects new value by the time build-id listener runs
- build-id listener cleanup prevents further event delivery
- status listener cleanup prevents further deliveries
- route-change listener cleanup prevents further event delivery
- `getLocation` returns current pathname/search/hash/state snapshot
- `getRouterData` updates after successful navigation
- failed navigation does not mutate established router-data snapshot
- abort-style navigation failures clear navigating state back to idle
- non-abort navigation failures clear loading state and allow recovery
  navigations
- non-OK navigation responses clear loading state and allow recovery navigations
- redirect follow to final destination commit
- navigation redirect headers are honored even on non-OK responses
- external navigation redirects fall back to hard location redirects
- navigation `X-Vorma-Reload` performs hard reload redirect with `vorma_reload`
  build marker
- relative `X-Vorma-Reload` resolves against redirecting request URL path
- `X-Vorma-Reload` is prioritized over `X-Client-Redirect` for navigation
- build ID is updated before following navigation redirect targets
- native fetch redirects are followed for GET navigations
- native redirects landing on encoding-equivalent current hash targets are not
  re-followed
- submit redirect headers are honored even on non-OK responses
- external submit redirects fall back to hard location redirects
- submit `X-Vorma-Reload` performs hard reload redirect with `vorma_reload`
  build marker
- `X-Vorma-Reload` is prioritized over `X-Client-Redirect` for submit
- build ID is updated before following submit redirect targets
- submit soft-redirect follow failures return explicit `Redirect failed` result
- native fetch redirects are followed for non-GET submits
- submit redirects that only change hash do not trigger extra route-data fetches
- submit redirects to the current path are not re-followed
- navigation and submit fetches include redirect-accept header
- loading continuity through redirect chain
- stale route-data completion does not override winner commit

## Audit Workflow (No Plan Change)

- Continue iterating mapping table entries in order and port them to black-box
  observable assertions.
- For each legacy test "should":
    - keep: already covered in black-box suite
    - rewrite: valid semantics but internal-coupled
    - delete: poisonous (backend-contract distrust or non-browser resiliency)
- Do not modify legacy test files during porting unless explicitly requested.
- Keep dist-only adapter validation rule unchanged.

## Dist Adapter Seed Coverage

Dist black-box suite:

- `typescript/vorma/client/src/tests/dist/npm_dist_black_box_authoritative.test.ts`

Currently ported and passing:

- `vorma/react` typed link resolves href/class through dist output
- `vorma/preact` typed link resolves href/class through dist output
- `vorma/solid` typed link resolves href/class through dist output
- `vorma/react` intent-prefetch starts only after configured delay and does not
  commit navigation state
- `vorma/preact` intent-prefetch starts only after configured delay and does not
  commit navigation state
- `vorma/solid` intent-prefetch starts only after configured delay and does not
  commit navigation state
- `vorma/react` intent-prefetch cancels on blur and a later fresh intent
  prefetch succeeds
- `vorma/preact` intent-prefetch cancels on blur and a later fresh intent
  prefetch succeeds
- `vorma/solid` intent-prefetch cancels on blur and a later fresh intent
  prefetch succeeds
- `vorma/react` `prefetch=none` does not start prefetch network work
- `vorma/preact` `prefetch=none` does not start prefetch network work
- `vorma/solid` `prefetch=none` does not start prefetch network work
- `vorma/react` modified clicks are not intercepted
- `vorma/react` `target=_blank` clicks are not intercepted
- `vorma/preact` modified clicks are not intercepted
- `vorma/solid` modified clicks are not intercepted
- `vorma/preact` `target=_blank` clicks are not intercepted
- `vorma/solid` `target=_blank` clicks are not intercepted
- `vorma/preact` normal internal clicks run `beforeBegin` lifecycle callback
- `vorma/react` normal internal clicks run `beforeBegin` lifecycle callback
- `vorma/solid` normal internal clicks run `beforeBegin` lifecycle callback

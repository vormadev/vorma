# ASSERTIONS

This is the main human-readable, deduplicated VORMA runtime assertion list. Process and mining notes live in `ASSERTIONS_NOTES.txt`.

Working draft. It started as a 10-bullet sample and is being expanded incrementally from mined VORMA runtime tests.

## Frontend Runtime

### Client Navigation Requests

#### Concurrent Navigation Requests and Winning Commits

- Assertion 1
  - Statement: The latest started navigation must remain authoritative when concurrent navigation responses resolve out of order.
  - Good example(s):
    - Navigation A starts for `/race-first`, then navigation B starts for `/race-second`; B resolves first and commits `/race-second`, and A resolving later does not overwrite the committed pathname or title.
  - Bad example(s):
    - `/race-second` commits first, but a later `/race-first` response overwrites the pathname, title, or router data.

- Assertion 2
  - Statement: Starting a new user navigation must abort the superseded in-flight navigation request.
  - Good example(s):
    - A fetch started for `/state-first` is marked aborted as soon as navigation to `/state-second` begins.
  - Bad example(s):
    - The old `/state-first` request stays live after `/state-second` begins and is still allowed to complete normally.

- Assertion 3
  - Statement: Superseded navigations may run client-loader logic speculatively, but stale navigation results must not commit UI state.
  - Good example(s):
    - A stale navigation runs its client loader and then gets aborted; a later fresh navigation commits `/fresh-target`, and the stale loader result never changes the pathname, title, or visible route state.
  - Bad example(s):
    - A client loader from an aborted navigation commits stale data or UI after a newer navigation has already won.

### Runtime Status Flags

#### Navigation and Revalidation Can Be In Flight Together

- Assertion 4
  - Statement: The runtime must be able to report navigation and revalidation as simultaneously in flight.
  - Good example(s):
    - While a fetch-backed navigation is active, `revalidate()` begins and status reports `isNavigating: true` and `isRevalidating: true` until both operations settle.
  - Bad example(s):
    - Starting a revalidation during navigation incorrectly clears the navigation flag, or starting navigation incorrectly hides the active revalidation flag.

### Revalidation Requests

#### Revalidation Status Lifecycle

- Assertion 5
  - Statement: Same-document no-op navigations must not clear or mask an in-flight revalidation state.
  - Good example(s):
    - The app is already at `/noop-while-revalidating`; a revalidation starts, then `vormaNavigate("/noop-while-revalidating")` runs, and status remains revalidating until the fetch completes.
  - Bad example(s):
    - A same-URL no-op navigation resets status to idle or otherwise interrupts an in-flight revalidation.

- Assertion 6
  - Statement: Revalidation must set `isRevalidating` while work is in flight and clear it after completion.
  - Good example(s):
    - `revalidate()` starts, status shows `isRevalidating: true`, the response resolves, the new title is committed, and status returns to idle.
  - Bad example(s):
    - Revalidation never exposes an in-flight state, or it leaves `isRevalidating` stuck on after the response settles.

#### Revalidation Result Ownership and Search-Param Stale Boundaries

- Assertion 7
  - Statement: Revalidation redirects must be followed only while the current location still owns the result.
  - Good example(s):
    - Revalidation begins at `/revalidate-start`, the response returns `X-Client-Redirect: /revalidate-target`, and the runtime follows it because the location has not changed underneath the request.
  - Bad example(s):
    - A stale revalidation that started for an older location still forces a redirect after the user has already moved somewhere else.

- Assertion 8
  - Statement: Search-param changes must define a stale boundary for revalidation results, so older revalidation responses cannot overwrite the newer location.
  - Good example(s):
    - Revalidation starts at `/revalidate-search-boundary?x=1`, the location changes to `?x=2` before the response settles, and the old response is discarded without changing title or committed state.
  - Bad example(s):
    - A revalidation started for `?x=1` is allowed to update title or route data after the app has already moved to `?x=2`.

### Document Title and Managed Head

#### Title Commitment and Head Reconciliation

- Assertion 11
  - Statement: Server route-data must own `document.title` on commit, including decoding HTML entities and clearing stale titles when the new route omits a title.
  - Good example(s):
    - Navigating to a route with title payload `Fish &amp; Chips &lt;3` commits `Fish & Chips <3`, and a later route with no title payload clears the previous title instead of leaving it behind.
  - Bad example(s):
    - Escaped entities remain visible in the tab title, or a stale title survives after navigating to a route that omits title data.

- Assertion 12
  - Statement: Managed head reconciliation must use valid marker pairs, fail loudly on invalid marker structure, deterministically reuse/update/reorder/remove managed nodes, keep `meta` and `rest` sections isolated, and remain idempotent for identical updates.
  - Good example(s):
    - Within the managed `meta` section, an existing description node is reused and updated, a stale canonical link is removed, a new viewport node is inserted in the requested order, and identical follow-up updates do not create extra nodes.
  - Bad example(s):
    - Stray nodes outside the managed marker pair are mutated, duplicate managed nodes accumulate across navigations, or invalid markers cause silent partial head mutations instead of a hard failure.

### Runtime Events, History, and Scroll State

#### Committed Events and History Lifecycle

- Assertion 13
  - Statement: Runtime listeners must fire only for committed state changes, expose coherent event detail, and stop delivering events after their cleanup function is called.
  - Good example(s):
    - A location listener receives `/location-a?one=1#hash-a` only after commit, a route-change listener sees the new title and router snapshot already in place, and no additional events arrive after cleanup.
  - Bad example(s):
    - Route-change fires before the committed title or router snapshot is visible, or a cleaned-up listener still receives later events.

- Assertion 14
  - Statement: Direct browser-history pushes and replaces are outside the VORMA navigation lifecycle, while same-document programmatic navigation must classify no-op, hash-only, and full navigation correctly and apply `replace` semantics without unnecessary fetches.
  - Good example(s):
    - `history.push("/history-direct-push")` updates location without fetching route data, `vormaNavigate("/same-noop")` performs no fetch or history mutation, a hash-only navigate updates `#details` without refetching, and `replace: true` uses `history.replace`.
  - Bad example(s):
    - A direct history push starts the VORMA loading lifecycle, a same-document hash-only navigation fetches route data, or `replace: true` still pushes a new entry.

#### POP Navigation and Scroll Restoration

- Assertion 15
  - Statement: Same-document POP transitions must restore hash or coordinate scroll correctly, while cross-document POP transitions must fetch route data from the POP payload URL, follow redirects, and fall back to a hard reload with logging if recovery fails.
  - Good example(s):
    - Going back from `/page` to `/page#section` scrolls the `#section` element exactly once after a single decode step, and going back to `/target?q=1` fetches `/target?q=1` even if `window.location` was changed manually before the POP handler ran.
  - Bad example(s):
    - POP navigation decodes hash fragments twice, ignores saved scroll coordinates, fetches using the wrong URL source of truth, or silently leaves the app broken after a cross-document POP fetch failure.

- Assertion 16
  - Statement: Scroll-state persistence must save outgoing positions, tolerate malformed or failing session-storage access, cap stored history entries to the newest 50, and restore page-refresh scroll only from recent snapshots whose href still matches.
  - Good example(s):
    - Leaving a page records its `{x,y}` under the current history key, malformed stored entries are ignored instead of crashing the runtime, and a fresh exact-href page-refresh snapshot restores scroll once and then clears itself.
  - Bad example(s):
    - Bad session storage data crashes navigation, stale or mismatched refresh snapshots still restore scroll, or old scroll entries grow without eviction.

### Runtime Reset and Failure Recovery

#### Terminal Outcomes, Reset, and Stale Suppression

- Assertion 17
  - Statement: Settled operations must surface explicit terminal outcomes, and `clearAll` must abort in-flight work, suppress late side effects from ignored aborts, and leave the runtime able to handle fresh work afterward.
  - Good example(s):
    - A superseded submit resolves as `{ success: false, error: "Aborted" }`, `clearAll` aborts an in-flight navigation so its late title and CSS never apply, and a later fresh navigation still commits normally.
  - Bad example(s):
    - Aborted work hangs without a terminal result, stale side effects still land after reset, or the runtime stays unusable after `clearAll`.

- Assertion 18
  - Statement: Navigation failure paths must clear loading state, avoid corrupting committed router data, remain recoverable, and contain stale late successes or failures without unhandled rejections.
  - Good example(s):
    - A 500 route-data response throws while preserving the previously committed router snapshot, a later successful navigation still works, and a stale earlier response resolving late cannot overwrite the winner's title, CSS, or build ID.
  - Bad example(s):
    - A failed navigation mutates committed router data, leaves `isNavigating` stuck on, or a stale late completion overrides the already-committed winner.

### Revalidation Scheduling and Focus

#### Coalescing, Ownership, and Focus Gating

- Assertion 19
  - Statement: Revalidation must coalesce repeated same-target requests into one in-flight fetch plus at most one trailing pass, but it must start fresh work when the underlying data target changes.
  - Good example(s):
    - Three rapid `revalidate()` calls against `/posts?tab=all` result in one active fetch and, if more requests arrived during it, at most one trailing revalidation after settlement; switching to `/posts?tab=mine` creates a new fetch.
  - Bad example(s):
    - Every rapid revalidation issues its own fetch, or a revalidation for `?tab=all` is reused after the app has moved to `?tab=mine`.

- Assertion 20
  - Statement: Revalidation side effects must apply only while the current location still owns the result; hash-only location changes preserve ownership, while search-param changes and external location changes invalidate stale results.
  - Good example(s):
    - An in-flight revalidation started at `/revalidate-hash-apply#initial` can still update title after the hash changes to `#next`, but a revalidation started at `/revalidate-search-boundary?x=1` is discarded after the app moves to `?x=2`.
  - Bad example(s):
    - A stale revalidation updates title, CSS, build ID, or redirect behavior after the user has already moved to a different search-param or external location.

- Assertion 21
  - Statement: Focus-triggered revalidation must honor `staleTime`, reset freshness only after successful navigation or revalidation, ignore hash-only or aborted work, avoid firing during active navigation/submit/revalidation, and stop entirely after cleanup.
  - Good example(s):
    - A focus listener with `staleTimeMS: 100` ignores focus at 50ms, revalidates after 101ms, does not reset the timer for a hash-only navigation, and never revalidates while a submit is already in flight.
  - Bad example(s):
    - Focus always triggers immediate revalidation, aborted work refreshes the stale-time window, or the listener keeps revalidating after cleanup.

### Loading, Status, CSS, and Client Loaders

#### Loading Indicators and Status Snapshots

- Assertion 22
  - Statement: Global loading indicators and status snapshots must respect inclusion filters, delays, and per-operation opt-outs, hide hidden submissions, cancel timers safely, and keep loading continuous across overlapping work.
  - Good example(s):
    - A navigation-only indicator starts for a slow navigation but ignores submissions, a hidden submit never sets visible `isSubmitting`, a pending stop timer is canceled when new work begins, and overlapping work does not create an idle gap between loading phases.
  - Bad example(s):
    - Instant work flickers the indicator, hidden submissions still surface as `isSubmitting`, or overlapping work produces start-stop thrash or a false idle gap.

- Assertion 23
  - Statement: Navigation loading must stay active through the full render pipeline, including CSS preload and client-loader settlement, and CSS/dependency artifacts must be deduplicated and applied only when appropriate.
  - Good example(s):
    - Navigating to a route with `/style1.css` keeps `isNavigating` true until the CSS preload settles and client loaders finish, repeated `/dup.css` ends with one stylesheet node, and prefetched CSS becomes a stylesheet only after the actual navigation commit.
  - Bad example(s):
    - Navigation flips idle before CSS or client loaders settle, CSS preload errors abort an otherwise successful navigation, or duplicate bundle stylesheets accumulate.

- Assertion 24
  - Statement: Matched client loaders must start in parallel, reuse already-running promises, receive matched server loader data, block route-change until they settle, and rerun on HMR only for opted-in matched JS updates.
  - Good example(s):
    - Two matched client loaders both start before either resolves, a second hash-only navigation reuses the running loader promise instead of reinvoking it, the loader receives the matched server `loaderData`, and a JS HMR update reruns only the opted-in matched loader.
  - Bad example(s):
    - Client loaders run serially, duplicate invocations happen while the original is still running, route-change fires before client-loader settlement, or CSS-only HMR reruns opted-in loaders.

### Runtime Surface and Accessors

#### Public API Shape and Snapshot Access

- Assertion 25
  - Statement: The public runtime surface must expose safe defaults and coherent accessors, while keeping buildtime-only and unstable internal helpers out of the runtime client entry.
  - Good example(s):
    - Before any commit, `getRouterData()` returns a safe empty snapshot, `getBuildID()` and `getLocation()` reflect the current committed state, `getRootEl()` returns the configured root element and throws clearly when missing, and `vorma/client` does not export internal `__*` helpers.
  - Bad example(s):
    - Accessors return incoherent state before init, the runtime silently accepts a missing root element, or buildtime/internal helpers leak from the runtime client entry.

- Assertion 26
  - Statement: Typed runtime helpers, typed client APIs, compiled path-resolution helpers, and typed link factories must resolve typed paths and options consistently, infer the correct result and prop types, enforce route param, splat, input, and method contracts at compile time, and merge or override defaults in the expected direction instead of clobbering caller intent.
  - Good example(s):
    - Typed navigate resolves `/docs/*` with `["guides","intro"]` into `/docs/guides/intro?mode=full#overview`, compiled path resolution turns `/products/:id/_index` with `id = "42"` into `/products/42`, querying `/health` allows omitted or `null` input while querying `/users/:userID` requires matching params plus an object input, a `PATCH` mutation requires `requestInit.method = "PATCH"` while a `POST` mutation can default naturally, and a typed link factory default class applies when omitted while a per-link class overrides that default when provided.
  - Bad example(s):
    - Missing or wrong param keys still type-check, splat routes accept non-string-array `splatValues`, non-GET queries allow `requestInit.method = "POST"`, non-POST mutations omit or mismatch the declared method, typed-link factory defaults cannot be overridden per link, decorator defaults overwrite caller headers, or typed submit/query no longer return the normal `{ success, data | error }` contract.

### Adapter Runtime Semantics

#### Root Outlets, Selectors, and Identity Across Adapters

- Assertion 27
  - Statement: Across React, Preact, and Solid, root outlets and typed adapter surfaces must render the correct fallback and error branches, expose the expected framework-specific hook, signal, or accessor shapes, respect server error index/export-key selection, preserve active error boundaries across hash-only navigations, default root `idx` to `0`, and initialize root listeners only once for idx `0` outlets.
  - Good example(s):
    - A child route with `outermostServerErrorIdx = 1` renders the child `ErrorBoundary` under a live parent layout, remounting the root outlet does not double-register root event listeners, React and Preact typed loader hooks return direct typed data, and Solid typed loader hooks return typed accessors.
  - Bad example(s):
    - The wrong boundary handles the error, hash-only navigation clears the active error boundary, omitted `idx` is not treated as `0`, nested outlets initialize root listeners, or adapter-specific typed hooks expose the wrong value shape for their framework.

- Assertion 28
  - Statement: Across adapters, location hooks/signals must update only on location events, route/data selectors must update only on route changes, selector ownership must stay coherent across route-index changes and render-abort-before-commit, and data-only updates must keep stable component mounts while data stays fresh.
  - Good example(s):
    - `useLocation` ignores a pure route-change event, a pattern-based selector returns `probe-a`, then `none`, then `probe-c` as route ownership changes, and a data-only revalidation updates loader data without remounting the stable root component.
  - Bad example(s):
    - Location hooks rerender from route-change alone, selector values mix old and new owners during a transition, or data-only updates remount components that should stay mounted.

- Assertion 29
  - Statement: Across adapters, component identity must be preserved until module identity or matched-pattern ownership actually changes, while still using the freshest loaded module/export implementation on later commits.
  - Good example(s):
    - Changing only a child route keeps the parent component instance and its local input state, but changing the child export key or deepest matched pattern remounts the owned child, and a later commit using the same import URL still renders the freshest module implementation.
  - Bad example(s):
    - Child-only route changes remount the parent unnecessarily, deep pattern changes fail to remount the owned child, or stale module implementations keep rendering after newer ones are loaded.

### Link Clicks and Prefetch

#### Interception, Prefetch, and Upgrade Behavior

- Assertion 30
  - Statement: Eligible internal primary link clicks must be intercepted and turned into VORMA navigation with the normal internal click callbacks, while modified, non-primary, consumer-canceled, blank-target, and external clicks must fall through to the browser; same-document hash/no-op clicks must avoid fetch and preserve committed metadata and scroll rules.
  - Good example(s):
    - Left-clicking an internal typed link prevents default, runs `beforeBegin`, and navigates; `meta`-click or `target="_blank"` does not intercept; external hrefs are treated as external; and clicking `/page#details` on the current page updates the hash without fetching route data or changing committed title/meta.
  - Bad example(s):
    - Modifier clicks trigger client navigation, a consumer `onClick` that calls `preventDefault()` is ignored, same-document hash/no-op clicks fetch route data, or cross-origin hash links are mistakenly treated as same-document hash navigations.

- Assertion 31
  - Statement: Intent prefetch must warm internal artifacts without committing navigation or loading state, run only for eligible internal targets, skip current-page no-ops, and support `prefetch=none`.
  - Good example(s):
    - Focusing an internal link starts prefetch after the configured delay, fetches route data without changing pathname, title, or loading status, skips prefetch when the href is already the current page, and does nothing when `prefetch="none"`.
  - Bad example(s):
    - Prefetch mutates URL or title before click, external or current-page links still prefetch, or `prefetch="none"` still starts work.

- Assertion 32
  - Statement: Prefetch work must deduplicate same-data targets, support cancellation with correct timer and touch-modality behavior, upgrade cleanly into navigation, and drop stale or failed prefetched work without leaking unhandled rejections.
  - Good example(s):
    - Two prefetches for `/prefetch-dedupe#first` and `/prefetch-dedupe#second` share one request, blur cancels a pending prefetch timer, touch pointerleave does not abort an in-flight touch prefetch until fine-pointer modality resumes, clicking during an in-flight prefetch upgrades it into navigation, and a stale settled prefetch is dropped quietly after stop.
  - Bad example(s):
    - Same-data prefetches refetch independently, stop aborts an upgraded navigation, pointerleave always aborts touch-intent prefetches, or failed/redirecting/stale prefetch work leaks unhandled rejections or commits stale page state.

### Loader-Backed Route Rendering and Persisted State

#### End-to-End Route Data and Persisted Mutations

- Assertion 33
  - Statement: Loader-backed routes must render nested/root loader data end-to-end and expose decoded dynamic params and query values rather than raw encoded strings.
  - Good example(s):
    - Visiting `/users/a%2Fb?q=hello%20world` renders `a/b` and `hello world`, and the home route renders its nested loader-backed count and message on first load.
  - Bad example(s):
    - Encoded params like `a%2Fb` or query values like `hello%20world` show up raw in the UI, or nested/root loader-backed route state fails to render on first visit.

- Assertion 34
  - Statement: Successful mutations must persist backend state across later client-side navigation into loader-backed routes.
  - Good example(s):
    - Incrementing state twice on `/mutation-lab` and then navigating home shows the home route count as `2`.
  - Bad example(s):
    - A mutation appears to succeed locally, but a later loader-backed route still shows the old server state.

- Assertion 35
  - Statement: Backend loader failures must surface through route error boundaries and suppress unreachable success-path UI.
  - Good example(s):
    - Navigating to a failing route renders the route boundary text like `explode-boundary:...` and the normal success-path content never appears.
  - Bad example(s):
    - A backend loader failure leaves the page blank, bypasses the route boundary, or renders success-path content that should be unreachable.

### Dev and Stress Resilience

#### HMR, Mutation Stress, and Payload Preservation

- Assertion 36
  - Statement: Rapid dev-time file-change races must settle to the latest HMR-visible state, not an intermediate stale version.
  - Good example(s):
    - Two quick edits replace an HMR token twice, and the page eventually shows only the second token.
  - Bad example(s):
    - After rapid successive file changes, the UI sticks on the first token or oscillates instead of settling to the latest source.

- Assertion 37
  - Statement: Randomized mixed-operation sweeps and broader concurrency bursts must finish without sticky runtime state, rejected navigations, or count drift away from the authoritative committed result.
  - Good example(s):
    - A seeded mixed-operation sweep completes every planned step, returns to `/`, leaves the runtime idle, and the home count matches the expected count derived from the successful mutations; a concurrent burst of slow mutations plus navigations finishes with zero mutation failures, zero rejected navigations, and a final home count equal to the highest committed mutation count.
  - Bad example(s):
    - Randomized or bursty mixed operations leave the runtime stuck loading, reject navigations unexpectedly, drift to a count that does not match the authoritative committed result, or fail to converge back to a stable home state.

- Assertion 38
  - Statement: Mutation workflows must preserve structured payloads, surface forced failures explicitly, respect actual completion order in concurrent mutation races, and remain stable across deterministic stress matrices.
  - Good example(s):
    - An echo mutation round-trips `payload-[test]-42|42`, a forced failure reports `failed:500`, a slow/fast increment race reports `slow:2|fast:1`, and a deterministic stress run completes all `101` cases with zero failures and a matching total count.
  - Bad example(s):
    - Structured mutation input is coerced incorrectly, forced failures disappear or look successful, concurrent mutation results report an impossible order, or deterministic stress runs leak failures or count drift.

- Assertion 39
  - Statement: Loader payloads with unusual shapes must be rendered without stale coercion or silent normalization.
  - Good example(s):
    - An odd-shape payload preserves empty arrays, matrix row shape `2,0,3`, `null` optional values, nested collection lengths, negative mixed numbers like `-42.75`, and timeline signatures like `boot:1|load:2|render:3`.
  - Bad example(s):
    - Empty arrays become missing data, `null` is coerced into some other default, nested shapes are flattened, or numeric/string payload values are normalized into the wrong form.

- Assertion 40
  - Statement: Under forced request abort and timeout chaos, the runtime must recover to idle without sticky loading state, and the final committed state must stay within the feasible bounds of the mutations that could actually have succeeded.
  - Good example(s):
    - A chaos run includes aborted fetches and synthetic `504` timeouts, some operations fail as expected, the app returns to idle, and the final observed count lands between the minimum and maximum count implied by the successful mutation outcomes.
  - Bad example(s):
    - Abort/timeout chaos leaves the runtime stuck loading, crashes follow-up navigation, or commits a final count that could not have been produced by any feasible set of successful mutations.

### Shared Session and Process Recovery

#### Cross-Page Convergence and Runtime Restart Recovery

- Assertion 41
  - Statement: Shared-session state must converge across multiple tabs under concurrent mutations.
  - Good example(s):
    - Two tabs perform independent mutation sequences, a later authoritative sync mutation runs, and fresh verification tabs converge on the same final home count.
  - Bad example(s):
    - Different tabs drift to permanently different committed counts after concurrent mutations against the same shared session.

- Assertion 42
  - Statement: The system must recover from a backend restart during in-flight navigation so fresh pages can reconnect and continue mutating and navigating successfully.
  - Good example(s):
    - A navigation race is in flight, the backend restarts, a fresh page reconnects, runs additional shared-session operations successfully, and later home visits still show a valid numeric count while the runtime remains idle.
  - Bad example(s):
    - After a backend restart during navigation, fresh pages cannot reconnect cleanly, later mutations fail systematically, or follow-up visits stay stuck instead of returning to a healthy idle runtime.

## Backend Runtime

### E2E Fixture Runtime Harness

#### Stable Port-Pinning Contract

- Assertion 43
  - Statement: The VORMA e2e fixture runtime harness must pin Wave port resolution with `__WAVE_PORT_HAS_BEEN_SET=true` in its base runtime environment.
  - Good example(s):
    - The e2e runtime harness declares the `__WAVE_PORT_HAS_BEEN_SET` variable name and includes it as `"true"` in the base runtime env used to boot the fixture runtime.
  - Bad example(s):
    - The fixture runtime harness omits the pin variable entirely, or defines it but fails to set it in the base runtime environment.

## Frontend/Backend Network Contract

### Client Query and Mutation Request Construction

- Assertion 44
  - Statement: Query and mutation URL builders must resolve dynamic route params correctly and preserve query inputs.
  - Good example(s):
    - Building URLs for pattern `/users/:id` with `id = "a/b"` produces `/api/users/a%2Fb`, and query input like `{ tab: "activity", page: 2 }` is preserved as search params on the query URL.
  - Bad example(s):
    - Dynamic params are inserted without encoding, or query inputs are dropped, mutated, or incorrectly added to mutation URLs.

### Request Markers, Build IDs, and Deployment Propagation

- Assertion 45
  - Statement: Client requests must include the runtime markers and metadata the backend contract expects, including route-data markers, current build identity, redirect-accept headers, and deployment propagation where configured.
  - Good example(s):
    - A navigation request for `/page?x=1` includes `vorma_json`, later route-data requests carry the current build token, revalidation includes the deployment query param when deployment ID is configured, and navigation/submit fetches send `X-Accepts-Client-Redirect: 1` while submit preserves caller headers plus `x-deployment-id`.
  - Bad example(s):
    - Route-data requests omit `vorma_json`, build or deployment metadata disappears between requests, or redirect-capable requests are sent without the redirect-accept header.

### Body Resolution and Submit Result Decoding

- Assertion 46
  - Statement: Body resolution and submit result decoding must preserve `BodyInit`-compatible inputs, serialize plain objects predictably, omit bodies for GET and HEAD-style submits, preserve explicit content types, and decode success/failure payloads deterministically.
  - Good example(s):
    - `URLSearchParams`, `Blob`, `ArrayBuffer`, `ArrayBufferView`, and `ReadableStream` bodies pass through unchanged, object bodies serialize to JSON, GET/HEAD submits omit the body entirely, `204` returns `{ success: true, data: undefined }`, and a plain-text `500` returns a failure with the response text.
  - Bad example(s):
    - Binary bodies are stringified, GET submits send a body anyway, a caller-provided content type gets overwritten, or empty/text/non-OK submit responses decode inconsistently.

### Navigation Redirect and Reload Responses

- Assertion 47
  - Statement: Navigation response handling must follow internal redirects and reload headers correctly, even from non-OK responses, resolve relative targets against the redirecting request URL, update build ID before follow-up, prefer hard reload over soft redirect, hard-navigate external targets, reject non-http redirect schemes, and follow native GET redirects only while still relevant.
  - Good example(s):
    - A navigation returning `500` plus `X-Client-Redirect: child/final` is followed relative to the redirecting request path, `X-Wave-Framework-Reload` sends the browser to a hard-reload URL that includes the new build ID, and a native GET redirect to a fresh internal page is followed into a final route-data commit.
  - Bad example(s):
    - Non-OK redirects are ignored, soft redirects win over `X-Wave-Framework-Reload`, build ID changes after the follow-up has already started, `mailto:` redirects are treated as valid navigation targets, or equivalent current-hash native redirects are re-followed.

### Submit Redirect and Reload Responses

- Assertion 48
  - Statement: Submit response handling must follow internal redirects and reload headers correctly, even from non-OK responses, avoid extra route-data fetches for same-document hash-only or same-path redirects, update build ID before follow-up, report redirect-follow failures explicitly, prefer hard reload over soft redirect, hard-navigate external targets, and drop stale submit side effects once a newer winner exists.
  - Good example(s):
    - A POST returning `500` plus `X-Client-Redirect: /final` still lands on `/final`, a native `HTMLFormElement.requestSubmit()` action redirect can land on a loader-backed user route, a redirect to `/current#frag` updates only the hash with no extra route-data fetch, a failed follow-up returns `{ success: false, error: "Redirect failed" }`, and a stale older submit cannot later change build ID or trigger reload/redirect behavior.
  - Bad example(s):
    - Submit ignores valid redirect headers on non-OK responses, fetches route data for a same-page hash redirect, soft redirect beats `X-Wave-Framework-Reload`, or a stale submit later forces redirect, reload, or build-ID changes after a newer result already won.

### Redirect Chains

- Assertion 49
  - Statement: Redirect chains must keep loading continuous through successful follow-up requests and must stop after ten hops with a clear idle failure outcome.
  - Good example(s):
    - A chain like `/redirect-chain/start -> /redirect-chain/middle -> /redirect-chain/end?hop=from-middle` stays in loading until the terminal loader-backed route commits, while a redirect loop is capped at ten follows, logs a clear error, returns a non-navigation outcome, and exits back to idle.
  - Bad example(s):
    - Loading drops to idle in the middle of a redirect chain, or redirect loops continue indefinitely without a hard cap and clear termination.

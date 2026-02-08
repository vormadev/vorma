# Vorma Frontend Runtime Conformance Specification

Status: Draft  
Last Updated: 2026-02-08  
Applies To: Vorma TypeScript client runtime behavior as observed via public client APIs, browser APIs, DOM/events, and network I/O

## 1. Why This Spec Exists

This is a black-box conformance spec for Vorma frontend runtime behavior.

It is intended to:

- support large frontend/runtime refactors without reading implementation internals,
- drive test suites from behavior requirements instead of mirroring implementation,
- provide a single behavioral contract across React, Preact, and Solid adapters.

## 2. Conformance Boundaries

### 2.1 Normative Terms

The terms **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

### 2.2 Allowed Test Observations

Conformance tests for this spec MUST rely on observable behavior only:

- exported `vorma/client` API behavior,
- browser history/location changes,
- window events and event payloads,
- DOM mutations (`title`, `head`, CSS links, route outlet rendering side effects),
- HTTP request/response behavior (URL, headers, body, redirects),
- global bootstrap symbol contract (`globalThis[Symbol.for("__vorma_internal__")]`).

### 2.3 Disallowed Assertions

Conformance tests MUST NOT require:

- package-private TypeScript symbols,
- source file/module boundaries,
- private helper function call graphs.

## 3. Terminology

- Client Global: `globalThis[Symbol.for("__vorma_internal__")]` bootstrap/runtime object.
- Active Navigation: the single in-progress user/browser/redirect navigation slot.
- Prefetch Entry: cached prefetch navigation for a target URL.
- Pending Revalidation: coalesced revalidation entry for current page.
- Soft Redirect: client-side navigation redirect.
- Hard Redirect: browser location replacement/reload redirect.

## 4. Requirement Catalog

## 4.1 Bootstrap and Initialization

### FE-INIT-001: `initClient` Must Initialize Runtime Primitives

Given `initClient` is called with valid options  
When initialization starts  
Then it MUST initialize HMR integration, runtime options, and history handling before first interactive navigation.

### FE-INIT-002: Scroll Save Hook

Given client initialization completed  
When browser `beforeunload` fires  
Then current scroll position MUST be saved for page-refresh restoration.

### FE-INIT-003: Initial Client Module Map Seed

Given server bootstrap contains initial matched patterns/import metadata  
When `initClient` runs  
Then client module map MUST be seeded so existing matched routes can resolve modules before first navigation.

### FE-INIT-004: Pattern Registry Initialization

Given `vormaAppConfig` defines dynamic/splat/index runes  
When `initClient` runs  
Then matcher pattern registry MUST be initialized with those runes.

### FE-INIT-005: Route Manifest Progressive Enhancement

Given bootstrap contains `routeManifestURL`  
When manifest fetch succeeds  
Then manifest MUST be stored and patterns registered into matcher registry.

Given manifest fetch fails  
When initialization continues  
Then runtime MUST continue operating (progressive enhancement, no fatal init failure).

Failure diagnostics:

- runtime SHOULD emit a warning-level diagnostic for manifest fetch failure.

### FE-INIT-006: Error Boundary Defaulting

Given `initClient` options include `defaultErrorBoundary`  
When initialization runs  
Then that boundary MUST become the default runtime boundary.

Given `defaultErrorBoundary` is not provided  
When initialization runs  
Then runtime MUST use built-in default error boundary.

### FE-INIT-007: View Transition Opt-In

Given `useViewTransitions` option is true  
When initialization runs  
Then runtime MUST enable view-transition behavior for eligible navigations.

### FE-INIT-008: Hard-Reload Query Cleanup

Given current URL contains query key `vorma_reload`  
When `initClient` runs  
Then runtime MUST remove that key and `history.replace` the cleaned URL.

### FE-INIT-009: Initial Component and Loader Warmup Order

Given bootstrap contains initial route data  
When `initClient` runs  
Then runtime MUST:

1. load initial route components,
2. execute initial client loaders,
3. resolve initial error boundary component,
4. call user `renderFn`.

### FE-INIT-010: Page Refresh Scroll Restore

Given page-refresh scroll state exists for same URL and recent timestamp  
When `initClient` finishes initial render  
Then runtime MUST restore that scroll state and clear one-time refresh state.

### FE-INIT-011: Touch Device Detection

Given `touchstart` occurs after initialization  
When first touch event fires  
Then runtime MUST set touch-device flag and retain touch semantics for link prefetch behavior.

### FE-INIT-012: Initial Module-Map Seed Normalization

Given bootstrap pattern/import metadata arrays may contain partial tuple positions  
When `initClient` seeds initial `clientModuleMap`  
Then runtime MUST ignore tuple positions missing either `pattern` or `importURL`.

Given a seeded tuple omits `exportKey` or `errorExportKey`  
When map entry is created  
Then entry defaults MUST be `exportKey="default"` and `errorExportKey=""`.

### FE-INIT-013: Manifest Fetch Must Not Block First Render

Given bootstrap contains `routeManifestURL` and manifest fetch remains pending  
When initial startup warmup runs  
Then runtime MUST still complete initial component/loaders/error-boundary
warmup and invoke `renderFn` without awaiting manifest completion.

### FE-INIT-014: Touch Detection Listener Must Be One-Shot

Given touch-device detection listener is registered during init  
When runtime attaches the `touchstart` listener  
Then listener registration MUST use one-shot semantics (`once: true`).

Given multiple `touchstart` events occur after init  
When first and subsequent events fire  
Then `isTouchDevice` MUST become true on first touch and remain true for later
touches (no reset to false).

## 4.2 Client Global and Public Accessors

### FE-CTX-001: Global Symbol Namespace

Given Vorma client bootstrap script is present  
When runtime executes  
Then client global MUST be addressable at `globalThis[Symbol.for("__vorma_internal__")]`.

Accessor wrapper contract:

- `__getVormaClientGlobal().get(key)` MUST read directly from that symbol-keyed
  global object.
- `__getVormaClientGlobal().set(key, value)` MUST write directly to that
  symbol-keyed global object so subsequent `get(key)` reflects the same value
  identity/reference.

### FE-CTX-002: Router Data Accessor Contract

Given current route state  
When `getRouterData()` is called  
Then returned object MUST include:

- `buildID`,
- `matchedPatterns`,
- `params`,
- `splatValues`,
- `rootData` (`null` when `hasRootData` is false).

### FE-CTX-003: Location Accessor Contract

Given current browser location/history state  
When `getLocation()` is called  
Then returned object MUST include `pathname`, `search`, `hash`, and `state`.

Field-source contract:

- `pathname`, `search`, and `hash` MUST reflect current `window.location`,
- `state` MUST reflect runtime history instance state (`getHistoryInstance().location.state`).

### FE-CTX-004: Build ID Accessor Contract

Given current runtime build id  
When `getBuildID()` is called  
Then it MUST return the client global build id.

### FE-CTX-005: Root Element Accessor Contract

Given DOM contains Vorma root element  
When `getRootEl()` is called  
Then it MUST return element with id `vorma-root`.

### FE-CTX-006: Effective Error Selection

Given both server and client loader errors may exist  
When runtime derives effective error  
Then effective error index MUST be the outermost (lowest index) error, and effective error message MUST come from that selected side.

### FE-CTX-007: Router Data Fallback Defaults

Given `getRouterData()` is called while optional route-state fields are absent in client global  
When accessor builds return payload  
Then fallback defaults MUST be:

- `buildID`: empty string,
- `matchedPatterns`: empty array,
- `params`: empty object,
- `splatValues`: empty array.

Root-data projection:

- when `hasRootData=false`, `rootData` MUST be `null`,
- when `hasRootData=true`, `rootData` MUST be projected from `loadersData[0]`.

### FE-CTX-008: History Instance Accessor Contract

Given runtime history accessor is used  
When `getHistoryInstance()` is called repeatedly  
Then it MUST return the singleton browser-history instance used by runtime
history management.

Given that returned object  
When inspected  
Then it MUST expose history navigation surface compatible with runtime usage,
including:

- `action` with history-action class values compatible with
  `POP`/`PUSH`/`REPLACE`,
- `location` payload carrying `pathname`, `search`, `hash`, `state`, and `key`,
- navigation methods `createHref`, `push`, `replace`, `go`, `back`, `forward`,
- observer/control methods `listen` and `block`.

### FE-CTX-009: Missing Bootstrap Global Must Fail Fast

Given the symbol-keyed bootstrap global has not been installed  
When `__getVormaClientGlobal().get(key)` or `__getVormaClientGlobal().set(key, value)` is called  
Then the accessor call MUST fail synchronously (throw) and MUST NOT synthesize
or auto-install a fallback global object.

## 4.3 Navigation State Machine and Status

### FE-NAV-001: Supported Navigation Types

Runtime navigation MUST support these types:

- `userNavigation`
- `browserHistory`
- `revalidation`
- `redirect`
- `prefetch`
- `action`

### FE-NAV-002: Single Active Navigation Slot

Given multiple navigations are triggered  
When runtime tracks in-flight navigations  
Then at most one active user/browser/redirect navigation entry MUST exist.

### FE-NAV-003: User Navigation Abort Rules

Given a new user navigation to URL `B`  
When active navigation/pending revalidation target URL is different  
Then conflicting active navigation and conflicting revalidation MUST be aborted.

Given in-flight prefetches for other URLs  
When new user navigation starts  
Then unrelated prefetches MUST be aborted.

### FE-NAV-004: User Navigation Reuse for Same Target

Given active navigation already targets URL `U`  
When user navigation to `U` starts  
Then runtime MUST reuse existing control/promise.

### FE-NAV-005: Prefetch Upgrade

Given prefetch entry exists for URL `U`  
When user navigation to `U` starts  
Then runtime MUST upgrade that prefetch entry to real navigation intent without issuing a second fetch.

Upgrade propagation contract:

- user-navigation options (`state`, `replace`, `scrollToTop`) MUST be applied to
  the upgraded entry before completion/render.
- successful pure-prefetch entries (intent still `none`) MUST remain cached for
  same-URL upgrade reuse and MUST NOT be auto-removed solely because fetch/wait
  completed.

### FE-NAV-006: Prefetch Deduplication

Given repeated prefetch requests to same URL  
When prefetch is already in cache/in-flight  
Then runtime MUST return existing control and MUST NOT duplicate fetch.

### FE-NAV-007: No-Prefetch for Current Document

Given prefetch target equals current document URL (ignoring hash)  
When prefetch starts  
Then runtime MUST not perform network fetch.

Conformance-visible behavior:

- returned prefetch control MUST resolve as a non-navigation outcome (no render),
- global status flags/events MUST remain unaffected (pure prefetch exclusion).

### FE-NAV-008: Revalidation Coalescing

Given multiple revalidation calls in a short window  
When revalidation requests are coalescible  
Then runtime MUST reuse single pending revalidation control.

Coalescing contract:

- reuse window MUST be 8ms,
- coalescing applies only for pending revalidation targeting current URL,
- if pending revalidation targets a different URL, it MUST be aborted/replaced.

### FE-NAV-009: Revalidation Origin Safety

Given revalidation started on origin URL `O`  
When current URL no longer equals `O` before render  
Then revalidation result MUST NOT render stale page data onto the new location.

No-commit side-effect boundary:

- stale-origin revalidation payload MUST NOT commit route-state snapshot changes
  (`matchedPatterns`, `loadersData`, `importURLs`, `exportKeys`,
  client/server/effective error projections),
- stale-origin revalidation payload MUST NOT dispatch route-change for that
  stale payload,
- stale-origin revalidation payload MUST NOT mutate `document.title` or managed
  head sections,
- stale-origin revalidation payload MUST NOT mutate `clientModuleMap` or apply
  CSS bundles derived from that stale payload.

### FE-NAV-010: Status Flag Semantics

`getStatus()` and status events MUST follow:

- `isNavigating`: true only for active navigation with navigate intent and incomplete phase,
- `isRevalidating`: true only while pending revalidation is incomplete,
- `isSubmitting`: true while at least one non-skipped submission is active.

Status flags MUST NOT treat pure prefetch entries as navigating/revalidating work.

### FE-NAV-011: Status Event Debounce and Dedup

Given rapid internal state transitions  
When status events are emitted  
Then runtime MUST debounce status dispatches using an 8ms window and MUST NOT
emit duplicate consecutive status payloads (value-equal payloads MUST be
suppressed even when generated from separate state writes).

### FE-NAV-012: Status Cleanup on Failure/Abort

Given navigation/submission fails or aborts  
When cleanup completes  
Then status flags MUST return to non-loading state unless another operation is still active.

### FE-NAV-013: Continuous Loading During Chained Work

Given chained operations (submit->revalidate, soft redirect chains, asset wait, client-loader wait)  
When operations are in flight  
Then runtime MUST NOT introduce intermediate loading gaps (all status flags false) before final completion.

Continuity bounds:

- submit->revalidate handoff MUST transition directly between active-loading
  states (for example `isSubmitting=true` to `isRevalidating=true`) without a
  transient all-false state,
- soft-redirect handoff (including multi-hop redirect chains) MUST keep loading
  truthy until terminal destination completes/fails,
- navigation wait phases (`waiting` for CSS/module/client-loader dependencies)
  MUST keep navigation loading truthy until those waits settle.

### FE-NAV-014: Same-Target Revalidation Upgrade on User Navigation

Given pending revalidation already targets URL `U`  
When user navigation to `U` starts  
Then runtime MUST reuse that in-flight entry and upgrade it to user-navigation
intent/semantics (including history options) without issuing duplicate fetch.

### FE-NAV-015: Global Clear-All Abort Contract

Given active navigation, prefetch cache entries, pending revalidation, and/or
submissions may exist  
When `navigationStateManager.clearAll()` runs  
Then runtime MUST abort all tracked in-flight controls, clear all tracking
slots, and schedule status update so loading flags settle to all-false once no
new work is started.

### FE-NAV-016: Submission Loading-Indicator Opt-Out

Given one or more in-flight submissions are started with
`skipGlobalLoadingIndicator=true`  
When status is computed  
Then those submissions MUST NOT contribute to `isSubmitting=true`.

Given mixed in-flight submissions where at least one submission does not opt out  
When status is computed  
Then `isSubmitting` MUST remain true until all non-opted-out submissions settle.

### FE-NAV-017: `getStatus()` Must Reflect Immediate Runtime State

Given navigation/revalidation/submission state has just changed  
When `getStatus()` is called synchronously before debounced status-event
dispatch  
Then returned status MUST reflect current in-memory runtime state immediately
and MUST NOT wait for event debounce window.

### FE-NAV-018: Navigation Introspection and Cleanup Surface

Given active navigation, prefetch cache entries, and/or pending revalidation
exist  
When `navigationStateManager.getNavigationsSize()` is called  
Then result MUST equal the number of currently tracked entries across those
slots.

Given those entries are present  
When `navigationStateManager.getNavigations()` is called  
Then returned map MUST contain:

- active navigation entry (if present) keyed by its target URL,
- each prefetch cache entry keyed by target URL,
- pending revalidation entry (if present) keyed by target URL.

Snapshot and collision semantics:

- returned map MUST be a detached snapshot view; mutating the returned map MUST
  NOT mutate runtime-internal tracking slots,
- because returned map is keyed by target URL, projection MUST expose at most one
  entry per URL key,
- when multiple slots target the same URL during overlap windows, projection
  winner for that URL key MUST follow slot write order:
  active navigation -> prefetch -> pending revalidation (later write wins).

Size-view nuance:

- `getNavigationsSize()` reports slot-cardinality (active + prefetch count +
  pending revalidation as separate slots),
- `getNavigations().size` reports unique URL keys after projection,
- therefore same-URL overlaps MAY yield `getNavigationsSize() >
  getNavigations().size`.

Given `navigationStateManager.removeNavigation(key)` is called with a tracked
entry key  
When removal executes  
Then runtime MUST abort that entry's controller and remove the entry from
tracking.

Given active-navigation or pending-revalidation work has settled and cleanup has
completed  
When a later navigation to the same URL starts  
Then runtime MUST create fresh control/promise state and MUST NOT reuse stale
settled active/revalidation entries.

Completed-prefetch lifecycle reuse/retire semantics remain governed by
`FE-LINK-010`.

### FE-NAV-019: Prefetch Must Reuse Same-Target Active/Revalidation Control

Given prefetch is requested for URL `U` while active navigation already targets
`U`  
When prefetch begin path runs  
Then runtime MUST return the existing active-navigation control and MUST NOT
create a separate prefetch control or duplicate fetch.

Given prefetch is requested for URL `U` while pending revalidation already
targets `U`  
When prefetch begin path runs  
Then runtime MUST return the existing pending-revalidation control and MUST NOT
create a separate prefetch control or duplicate fetch.

## 4.4 Request Construction, Redirects, and Build ID Tracking

### FE-FETCH-001: JSON Mode Query Injection

Given route-data navigation request  
When fetch URL is built  
Then query key `vorma_json` MUST be set to current client build id.

### FE-FETCH-002: Revalidation Deployment ID Propagation

Given client global has deployment id and navigation type is revalidation  
When fetch URL is built  
Then query key `dpl` MUST be included.

Given client global deployment id is absent/empty and navigation type is
revalidation  
When fetch URL is built  
Then query key `dpl` MUST be omitted.

### FE-FETCH-003: Redirect Handshake Header

Given runtime fetches route/action data  
When request is sent  
Then header `X-Accepts-Client-Redirect: 1` MUST be present.

Caller-header override refinement:

- if caller-provided request headers already contain `X-Accepts-Client-Redirect`
  with any other value, runtime request construction MUST overwrite it to `1`.

### FE-FETCH-004: Submit Body Serialization

Given `submit()` request body  
When body is `FormData` or string  
Then body MUST be sent as-is.

Given submit body is non-string object (and not binary/form stream types)  
When request is sent  
Then body MUST be JSON-stringified.

### FE-FETCH-005: Redirect Signal Priority

Given response contains multiple redirect signals  
When client resolves redirect intent  
Then priority MUST be:

1. `X-Vorma-Reload`,
2. native `response.redirected`,
3. `X-Client-Redirect`.

### FE-FETCH-006: Redirect Eligibility

Given redirect target URL is non-HTTP (for example `mailto:`)  
When redirect is parsed  
Then redirect MUST be ignored.

### FE-FETCH-007: Redirect Strategy Selection

Given redirect target is internal HTTP URL  
When redirect is effectuated  
Then strategy MUST be soft redirect (client navigation).

Given redirect target is external HTTP URL  
When redirect is effectuated  
Then strategy MUST be hard redirect via browser location.

### FE-FETCH-008: Forced Hard Reload Semantics

Given redirect source `X-Vorma-Reload` is present  
When redirect is effectuated  
Then client MUST perform hard redirect and, for internal targets, append `vorma_reload=<latest build id>`.

### FE-FETCH-009: Redirect Loop Limit

Given redirects chain repeatedly  
When redirect count reaches max threshold  
Then runtime MUST stop redirect recursion and report error.

Conformance bounds:

- max redirect threshold MUST be 10 attempts in a single navigation chain,
- once threshold is reached runtime MUST stop follow-up fetches for that chain,
- failure diagnostics MUST include a "Too many redirects" signal.

### FE-FETCH-010: Build ID Eventing

Given response header `X-Vorma-Build-Id` differs from client global build id  
When response is processed  
Then runtime MUST update build id and dispatch `vorma:build-id` event containing `{ oldID, newID }`.

Dispatch ordering contract:

- global build-id storage MUST be updated before dispatch so listeners can
  synchronously observe the new value through accessor APIs.

This contract MUST apply for both route-navigation fetch responses and
`submit()` response handling.

### FE-FETCH-011: Build ID Event Before Redirect Handoff

Given response both updates build id and triggers redirect  
When redirect flow starts  
Then build-id update event MUST be dispatched before redirect handoff completes.

### FE-FETCH-012: Network Failure Safety

Given fetch fails, response is invalid, or JSON parsing fails  
When navigation resolves  
Then runtime MUST clean navigation state and keep current page stable (no partial apply of failed destination state).

HTTP status handling refinement:

- non-OK response statuses MUST fail navigation except `304`,
- `304` MUST be treated as non-fatal for route-data flow.

### FE-FETCH-013: Submission Deduplication by Key

Given multiple in-flight `submit()` calls share the same non-empty `dedupeKey`  
When a later submission starts  
Then runtime MUST abort the previous in-flight submission for that key and keep only the latest keyed submission active.

Given submissions have no `dedupeKey` or different `dedupeKey` values  
When submissions run concurrently  
Then runtime MUST treat them as independent operations.

### FE-FETCH-014: Submit Auto-Revalidation Policy

Given a `submit()` call succeeds without redirect handoff and request method is non-GET  
When `options.revalidate` is not explicitly `false`  
Then runtime MUST trigger exactly one revalidation before submit flow completes.

Given request method is GET, submit handled redirect handoff, or `options.revalidate=false`  
When submit flow completes  
Then runtime MUST NOT auto-trigger revalidation.

When submit triggers redirect handoff  
Then submit result MUST resolve as success and MUST NOT also resolve redirect payload data.

### FE-FETCH-015: Native Redirect Applicability by Method

Given fetch response reports native redirect via `response.redirected=true`  
When originating request method is GET  
Then runtime MUST treat native redirect as eligible redirect signal under redirect priority rules.

Given fetch response reports native redirect and originating request method is non-GET  
When redirect parsing runs  
Then runtime MUST ignore that native redirect signal and continue submit result handling as non-redirect response.

### FE-FETCH-016: Submit Deployment ID Header Propagation

Given client global has deployment id and `submit()` sends a request  
When request headers are built  
Then header `x-deployment-id` MUST be included with that deployment id value.

Given client global deployment id is absent/empty and `submit()` sends a request  
When request headers are built  
Then header `x-deployment-id` MUST be omitted.

### FE-FETCH-017: Stale-Build Apply Guard for Module Map and CSS

Given navigation response build id differs from current client build id  
When success outcome is applied  
Then runtime MUST NOT mutate `clientModuleMap` from that response and MUST NOT apply that response CSS bundle list.

### FE-FETCH-018: Native Redirect to Current URL Is Terminal

Given fetch response reports native redirect and `response.url` resolves to current document URL  
When redirect parsing runs  
Then runtime MUST treat redirect as already completed (`did`) and MUST NOT schedule an additional redirect navigation.

### FE-FETCH-019: Submit Result Envelope and Abort Semantics

Given `submit()` completes successfully with JSON response  
When promise resolves  
Then result MUST be `{ success: true, data: <parsed JSON> }`.

Given `submit()` fails due to HTTP non-OK response  
When promise resolves  
Then result MUST be `{ success: false, error: <status code string> }` (no throw).

Given `submit()` is aborted  
When promise resolves  
Then result MUST be `{ success: false, error: "Aborted" }` (no throw).

Given `submit()` fails due to non-abort runtime/network error represented as an
`Error` instance  
When promise resolves  
Then result MUST be `{ success: false, error: <message> }` (no throw).

Given `submit()` fails due to non-abort runtime/network error represented as a
non-`Error` thrown value  
When promise resolves  
Then result MUST be `{ success: false, error: "Unknown error" }` (no throw).

Given `submit()` fails before a concrete HTTP response exists (for example
redirect-loop guard termination)  
When promise resolves  
Then result MUST be `{ success: false, error: "unknown" }` (no throw).

### FE-FETCH-020: Redirect Handoff Cleanup Discipline

Given redirect handoff is being effectuated  
When runtime prepares redirect execution  
Then runtime MUST abort and remove tracked in-flight redirect/revalidation
entries before handoff so loading status cannot remain stuck on stale work.

### FE-FETCH-021: Soft Redirect Navigation-Option Propagation

Given redirect handoff selects soft-redirect strategy  
When runtime schedules redirect navigation  
Then redirect navigation MUST preserve original navigation options when present:

- `state`,
- `replace`,
- `scrollToTop`.

### FE-FETCH-022: Revalidate Target URL and Query Preservation

Given `revalidate()` is invoked  
When runtime builds the revalidation request URL  
Then request target MUST be current `window.location.href` and existing query
parameters MUST be preserved while `vorma_json=<current build id>` is
added/updated.

### FE-FETCH-023: Submit Non-Error Throw Fallback Message

Given `submit()` catch path receives a thrown value that is not an `Error`
instance  
When promise resolves  
Then result MUST be `{ success: false, error: "Unknown error" }` (no throw).

### FE-FETCH-024: Failed Navigation Must Preserve Current Route Snapshot

Given `vormaNavigate(...)` fails (network failure, non-OK response except 304,
invalid/empty JSON payload, or parsing failure)  
When failure path settles  
Then runtime MUST preserve current-page route snapshot and MUST NOT partially
apply destination route state.

Failure-preservation invariants:

- current browser location/history entry MUST remain unchanged by the failed
  destination,
- title/head/active-component/params state from current committed route MUST
  remain unchanged by the failed destination,
- failure cleanup MUST NOT poison subsequent successful navigations (later
  successful navigation MUST still fully commit).

### FE-FETCH-025: Module-Preload Candidate Source Selection by Build Mode

Given successful route-data fetch in development mode  
When runtime selects JavaScript module-preload candidates  
Then candidate source MUST be `json.importURLs` (with duplicate URLs treated as
one effective preload target).

Given successful route-data fetch in production mode  
When runtime selects JavaScript module-preload candidates  
Then candidate source MUST be `json.deps` (not `json.importURLs`).

Candidate hygiene rules:

- falsy/empty candidate entries MUST be ignored,
- repeated candidate URLs MUST result in at most one effective preload operation
  per URL.

### FE-FETCH-026: Pure-Prefetch Redirect Outcomes Must Not Effectuate Redirects

Given navigation outcome is redirect and the owning entry is still a pure
prefetch (`type="prefetch"`, `intent="none"`)  
When navigation outcome handling runs  
Then runtime MUST remove that prefetch entry and MUST NOT effectuate the
redirect (no soft-redirect navigation and no hard redirect side effect).

### FE-FETCH-027: Submit Redirect Success Envelope Uses Undefined Data

Given `submit()` receives redirect-handoff outcome (`redirectData.status="should"`)  
When submit promise resolves  
Then result MUST be `{ success: true, data: undefined }`.

Redirect-envelope guard:

- redirect handoff path MUST NOT expose redirect metadata payload as submit data.

### FE-FETCH-028: Request Build-ID Fallback Default

Given route-data navigation or revalidation request URL is being constructed  
When client global build id is absent or empty  
Then runtime MUST set `vorma_json=1` as fallback build-id value.

Given client global build id is present and non-empty  
When request URL is built  
Then runtime MUST set `vorma_json` to that current build id value.

## 4.5 Client-Only Skip Optimization Contract

### FE-SKIP-001: Skip Is Optional and Observable

Given runtime has enough local data to satisfy navigation  
When skip path is eligible  
Then runtime MAY skip server fetch and synthesize route-data locally.

Conformance observable: destination navigation succeeds without network fetch for that transition.

Scope guard:

- skip optimization MUST apply only to navigation flows (not `revalidation` or
  `action` flows).

### FE-SKIP-002: Skip Requires Local Manifest and Registry

Given route manifest or pattern registry is unavailable  
When navigation starts  
Then runtime MUST NOT use skip path.

### FE-SKIP-003: Skip Must Not Hide Server Loader Removal

Given current route includes a server loader that would be removed in target match  
When evaluating skip path  
Then runtime MUST NOT skip server fetch.

### FE-SKIP-004: Skip Must Not Hide New Client Loader Introduction

Given target introduces a client loader pattern not currently matched  
When evaluating skip path  
Then runtime MUST NOT skip server fetch.

### FE-SKIP-005: Skip Parameter Change Guards

Given route has loader-relevant params/splats/search changes  
When evaluating skip path  
Then runtime MUST NOT skip server fetch.

Loader-relevant means:

- when at least one matched route has a server or client loader, any search
  param change MUST disable skip,
- dynamic param changes on the outermost loader-bearing matched route MUST
  disable skip,
- splat-value changes on that outermost loader-bearing route MUST disable skip
  when the route pattern ends with splat.

Given there is no loader-bearing matched route  
When only search params change  
Then skip MAY still be allowed.

### FE-SKIP-006: Skip Output Alignment

Given skip path is used  
When synthetic route-data is produced  
Then arrays (`matchedPatterns`, `loadersData`, `importURLs`, `exportKeys`) MUST remain index-aligned.

### FE-SKIP-007: Skip Requires Module Map Coverage

Given skip path candidate has target matched patterns  
When synthetic route-data is being assembled  
Then each matched pattern MUST have client module metadata
(`importURL`/`exportKey`) available in local client module map.

Given any matched pattern is missing module metadata  
When skip eligibility is evaluated  
Then runtime MUST abort skip and perform server fetch.

### FE-SKIP-008: Skip Loader Data Projection Rules

Given skip path is used and synthetic `loadersData` is projected  
When a projected matched pattern corresponds to a server loader route  
Then projected loader value MUST be copied from currently active `loadersData`
for that same pattern.

Given projected matched pattern does not correspond to a server loader route  
When `loadersData` is projected  
Then projected value for that index MUST be `undefined`.

Given server-loader projection cannot find a current matched source entry for
that pattern  
When evaluating skip  
Then runtime MUST abort skip and perform server fetch.

### FE-SKIP-009: Skip-Path Client Loader Reuse

Given skip path is used and pattern `P` has client loader with existing
non-`undefined` current `clientLoadersData` value  
When client-loader completion phase runs for that skip navigation  
Then runtime MUST reuse that existing value for `P` and MUST NOT invoke the
client loader again for that navigation.

### FE-SKIP-010: Skip-Synthesized Payload Defaults and Build-ID Continuity

Given skip path is used and runtime synthesizes route-data locally  
When synthesized payload is emitted to downstream loader/render phases  
Then synthesized defaults MUST include:

- `deps=[]` and `cssBundles=[]`,
- `errorExportKeys=[]`,
- `outermostServerError` and `outermostServerErrorIdx` unset,
- `title`, `metaHeadEls`, and `restHeadEls` unset.

Given skip path synthesizes local response metadata  
When synthetic response object is constructed  
Then response MUST be JSON `200` and include
`X-Vorma-Build-Id=<current client build id>` (or `"1"` fallback when build id
is absent/empty).

### FE-SKIP-011: Skip Requires Target Route Match

Given skip eligibility evaluation is running  
When target URL pathname has no nested pattern match in the client matcher
registry  
Then runtime MUST NOT use skip optimization and MUST perform server fetch.

## 4.6 Client Loader Contract

### FE-CL-001: Pattern Registration API

Given `__registerClientLoaderPattern(pattern)` is called  
When registration succeeds  
Then pattern MUST be registered into client matcher registry.

### FE-CL-002: Partial Match Fallback

Given full nested path match is absent for a pathname  
When `findPartialMatchesOnClient` runs  
Then runtime MUST search progressively shorter path prefixes and return longest available parent match.

### FE-CL-003: Loader Execution Input Contract

Given a matched pattern has client loader function  
When loader is invoked  
Then invocation payload MUST include:

- `params`,
- `splatValues`,
- `serverDataPromise`,
- `signal`.

### FE-CL-004: Server Error Cutoff for Client Loaders

Given server outermost error index `i`  
When client loaders execute for same navigation  
Then client loader at index `i` and deeper MUST be skipped.

### FE-CL-005: Child Abort on First Non-Abort Error

Given parallel client loaders and loader `i` fails with non-abort error  
When execution continues  
Then deeper child loaders MUST be aborted, and returned client loader data MUST stop at first error.

### FE-CL-006: Abort Errors Are Non-Fatal

Given loader promise rejects with abort-like error  
When collecting results  
Then runtime MUST treat it as cancellation, not a client-error message source.

### FE-CL-007: Initial Loader Setup

Given `initClient` executes  
When setup phase runs  
Then initial client loaders MUST execute once and populate `clientLoadersData` before first render.

### FE-CL-008: Navigation Wait Contract

Given route navigation with client loaders  
When navigation completes  
Then runtime MUST await client loader completion before final render commit.

### FE-CL-009: Client Loader Error State Projection

Given client loader execution yields first true error at index `i`  
When client loader state is stored  
Then runtime MUST set:

- `outermostClientError` message,
- `outermostClientErrorIdx=i`.

### FE-CL-010: `serverDataPromise` Resolution Shape and Fallback

Given client loader invocation payload includes `serverDataPromise`  
When the promise resolves  
Then resolved object MUST include:

- `matchedPatterns: string[]`,
- `loaderData` for that loader's matched pattern (or `undefined` when absent),
- `rootData` as root loader data when `hasRootData=true`, otherwise `null`,
- `buildID`.

Given server route-data is unavailable for that invocation (for example fetch
failure, non-OK terminal branch, or parse failure)  
When `serverDataPromise` resolves  
Then runtime MUST resolve (not reject) with fallback:

- `matchedPatterns: []`,
- `loaderData: undefined`,
- `rootData: null`,
- `buildID: "1"`.

### FE-CL-011: Running-Loader Reuse Contract

Given a client loader for pattern `P` is already running for the same
navigation (started in parallel with fetch)  
When loader completion phase runs  
Then runtime MUST reuse the existing promise for `P` and MUST NOT invoke that
loader function a second time.

### FE-CL-012: Partial-Match Probe Order and Termination

Given full nested path match exists for a pathname  
When `findPartialMatchesOnClient` runs  
Then runtime MUST return the full-match result directly and MUST NOT rely on
shorter-prefix fallback selection.

Given full nested path match is absent  
When `findPartialMatchesOnClient` runs  
Then runtime MUST probe progressively shorter slash-delimited prefixes from
longest to shortest and return the first successful (longest) partial match.

Given input pathname ends with trailing slash  
When prefix probing runs  
Then probe segmentation MUST treat trailing slash as non-segment-bearing
terminator (same probe sequence as equivalent non-trailing path).

Given no full or prefix match exists  
When helper completes  
Then it MUST return `null`.

## 4.7 Component and Error Boundary Resolution

### FE-COMP-001: Dynamic Import URL Resolution

Given import URLs from route-data  
When components load  
Then URLs MUST resolve through `resolvePublicHref` using dev/prod base rules.

Import execution behavior:

- runtime MUST deduplicate repeated import URLs before issuing dynamic
  import execution for that navigation pass.

### FE-COMP-002: Component Mapping by Export Key

Given route-data `importURLs` and `exportKeys`  
When component map resolves  
Then each route index MUST map to module export key, defaulting to `default` when omitted.

Positional slot behavior:

- active component slots MUST remain aligned to original route-index ordering
  from route-data (not deduped URL ordering),
- repeated route slots referencing the same import URL MUST reuse that module
  resolution while still producing per-index component slots,
- unresolved import URL or unresolved export key for a slot MUST produce `null`
  for that slot rather than shifting/removing indices.

### FE-COMP-003: Component State Update Minimization

Given resolved component array equals current active component array  
When navigation updates run  
Then runtime SHOULD avoid unnecessary active component state replacement.

### FE-COMP-004: Error Boundary Module Resolution

Given effective error index exists and module exposes matching error export key  
When error boundary resolves  
Then runtime MUST use route-specific error component.

Given route-specific error component is missing  
When error boundary resolves  
Then runtime MUST fall back to default error boundary.

### FE-COMP-005: Error Boundary Export Access Resilience

Given module namespace access for the configured error export key throws (for
example proxy-backed module namespaces)  
When error boundary resolves  
Then runtime MUST treat that export as unresolved and MUST fall back to default
error boundary instead of propagating the access error.

### FE-COMP-006: Built-In Default Error Boundary Render Contract

Given built-in `defaultErrorBoundary` is active for an error branch  
When boundary renders with error payload `E`  
Then rendered output MUST prefix payload with `"Route Error: "` and include
`E` without additional wrapper formatting requirements.

## 4.8 Rendering, History, Scroll, and Head Reconciliation

### FE-REN-001: View Transition Eligibility

Given `useViewTransitions` is enabled and browser supports `document.startViewTransition`  
When navigation type is neither prefetch nor revalidation  
Then rerender MUST run inside view transition.

Given navigation type is prefetch or revalidation  
When rerender runs  
Then view transition MUST NOT be used.

### FE-REN-002: Global State Apply on Navigation

Given successful route-data payload  
When rerender pipeline starts  
Then runtime MUST update core client global route-data fields before route-change event dispatch.

Core field-set requirement:

- `outermostServerError`
- `outermostServerErrorIdx`
- `errorExportKeys`
- `matchedPatterns`
- `loadersData`
- `importURLs`
- `exportKeys`
- `hasRootData`
- `params`
- `splatValues`

Commit-consistency refinement:

- for a given commit, each field above MUST reflect that commit payload by the
  time `vorma:route-change` is dispatched,
- rerender MUST NOT preserve stale prior-commit values for any field listed
  above when the current commit provides a replacement value.

### FE-REN-003: History Push/Replace Contract

Given user navigation or redirect with `runHistoryOptions`  
When destination URL differs from current URL and `replace` is false  
Then runtime MUST `history.push`.

Given destination is same URL or `replace` is true  
When history updates  
Then runtime MUST `history.replace`.

### FE-REN-020: No History Mutation for Non-Navigate Intents

Given rerender commit originates from revalidation or pure prefetch intent  
When commit pipeline executes  
Then runtime MUST NOT invoke history push/replace mutations for that commit.

### FE-REN-021: History State Pass-Through

Given navigate-intent rerender commit includes `runHistoryOptions.state`  
When runtime executes history push/replace  
Then the same state payload MUST be forwarded to the history API call for that
commit.

Given navigate-intent rerender commit omits `runHistoryOptions.state`  
When runtime executes history push/replace  
Then runtime MUST NOT substitute stale state from prior commits.

### FE-REN-004: Browser-History Scroll Restore

Given browser-history navigation (`POP`) with saved scroll state  
When rerender completes  
Then route-change scroll detail MUST include saved scroll state for restore.

### FE-REN-005: Title Update Semantics

Given route-data contains title head element with `dangerousInnerHTML`  
When applying title  
Then runtime MUST decode HTML entities and update `document.title`.

Ordering refinement:

- title apply MUST occur after history push/replace decision for that commit.

### FE-REN-023: Title Apply Precedes Route-Change Dispatch

Given successful commit includes a title payload  
When runtime dispatches `vorma:route-change` for that commit  
Then listeners observing the event MUST see `document.title` already updated to
the decoded title value for that same commit.

### FE-REN-022: Missing Title Payload Is No-Op

Given render commit payload omits title (`title` is `undefined`)  
When rerender applies commit  
Then runtime MUST leave current `document.title` unchanged.

### FE-REN-006: Route Change Event Contract

Given successful render commit  
When route changes are applied  
Then runtime MUST dispatch `vorma:route-change` with detail key `__scrollState`
(when applicable), and dispatch MUST occur after history/title apply for that
commit.

### FE-REN-007: Head Updates Are Conditional

Given route-data `metaHeadEls`/`restHeadEls` are `undefined`  
When rerender runs  
Then corresponding head section MUST remain unchanged.

Given those fields are provided (including empty array)  
When rerender runs  
Then corresponding head section MUST be reconciled to provided value.

### FE-REN-008: Missing Head Markers Is Safe No-Op

Given expected head marker comments for section are missing  
When `updateHeadEls` runs  
Then update MUST no-op without throwing.

### FE-REN-009: Head Fingerprint Dedup and Reorder

Given desired head blocks include duplicates/reordering  
When section reconciliation runs  
Then final section MUST:

- deduplicate by element fingerprint,
- preserve semantic order of desired block list,
- remove stale nodes no longer present.

Idempotence refinement:

- applying reconciliation repeatedly with an identical desired block list MUST
  NOT accumulate duplicate managed nodes across updates.

### FE-REN-010: Head Attribute Validation

Given `attributesKnownSafe` includes null/undefined value  
When head element is created  
Then runtime MUST fail loudly (panic/error) rather than silently emit invalid attributes.

### FE-REN-011: Head Blocks Without Tag Are Ignored

Given a head block is missing `tag`  
When section reconciliation runs  
Then runtime MUST ignore that block and MUST continue processing remaining valid blocks.

### FE-REN-012: Managed Section Text-Node Cleanup

Given managed head section contains text nodes between start/end markers  
When section reconciliation runs  
Then runtime MUST remove those inter-marker text nodes and keep only reconciled element nodes in managed span.

Managed-span hygiene refinement:

- any non-element nodes between section markers (not only text nodes) MUST be
  removed from the managed span during reconciliation.

### FE-REN-013: Boolean Attributes and Dangerous HTML Application

Given head block contains `booleanAttributes`  
When element is created  
Then each boolean attribute MUST be emitted as present empty attribute.

Given head block contains `dangerousInnerHTML`  
When element is created  
Then runtime MUST set element `innerHTML` to provided value.

### FE-REN-014: Fingerprint-Stable Node Reuse

Given existing managed head element fingerprint equals desired block fingerprint  
When section reconciliation runs  
Then runtime SHOULD reuse the existing DOM element node instead of replacing it.

### FE-REN-015: Route-Change Event Dispatch Precedes Head Reconciliation

Given a successful navigation updates route globals and head payload  
When render pipeline commits  
Then `vorma:route-change` dispatch MUST occur before `metaHeadEls`/`restHeadEls` reconciliation for that payload.

### FE-REN-016: Head Marker Comment Whitespace Tolerance

Given managed head marker comments include leading/trailing whitespace in comment
text  
When marker lookup and reconciliation run  
Then marker matching MUST trim comment text and still resolve section boundaries.

### FE-REN-017: Head Fingerprint Determinism

Given head element fingerprinting is used for dedup/reuse  
When fingerprint is computed  
Then fingerprint MUST be based on:

- uppercased tag name,
- sorted attribute key/value pairs (attribute-order insensitive),
- trimmed `innerHTML`.

Given desired head block list contains duplicate fingerprints  
When reconciliation runs  
Then only one node for that fingerprint MUST remain in the managed section and
the last desired occurrence MUST win.

### FE-REN-018: Section-Scoped Head Reconciliation

Given managed head supports distinct `meta` and `rest` sections  
When `updateHeadEls(sectionType, blocks)` runs  
Then reconciliation MUST modify only nodes between that section's marker
comments and MUST NOT mutate nodes in the other managed section.

### FE-REN-019: Route-Change Scroll Payload Selection Rules

Given user/redirect navigation with `runHistoryOptions`  
When rerender computes route-change scroll payload  
Then selection MUST be:

- if destination URL has hash fragment, `__scrollState` MUST be `{ hash }`,
- else if `scrollToTop` is not explicitly `false`, `__scrollState` MUST be
  `{ x: 0, y: 0 }`,
- else `__scrollState` MUST be omitted (`undefined`).

Given browser-history navigation (`POP`)  
When rerender computes route-change scroll payload  
Then selection MUST be:

- `scrollStateToRestore` when provided,
- otherwise `{ hash }` when destination has hash,
- otherwise omitted (`undefined`).

## 4.9 Asset Management Contract

### FE-ASSET-001: Module Preload Dedup

Given module dependency URL `U`  
When `AssetManager.preloadModule(U)` is called repeatedly  
Then runtime MUST create at most one `link[rel="modulepreload"][href=U]` element.

Dependency-source selection for preload inputs:

- in dev, preload candidates MUST come from de-duplicated `importURLs`,
- in non-dev, preload candidates MUST come from server-provided `deps`.

### FE-ASSET-002: CSS Preload Dedup

Given CSS bundle URL `C`  
When `AssetManager.preloadCSS(C)` is called repeatedly  
Then runtime MUST create at most one `link[rel="preload"][as="style"][href=C]` element.

### FE-ASSET-003: CSS Preload Promise Semantics

Given CSS preload link is created  
When browser emits load/error  
Then returned promise MUST resolve/reject accordingly.

### FE-ASSET-004: CSS Apply Contract

Given CSS bundle list from route-data  
When CSS apply runs  
Then runtime MUST append stylesheet links with `data-vorma-css-bundle` marker and MUST avoid duplicates by marker check.

Apply timing and keying:

- stylesheet append MUST run on next animation frame,
- dedupe key MUST be the original bundle identifier stored in
  `data-vorma-css-bundle`.

### FE-ASSET-005: Public URL Resolution Base

Given runtime is in dev with `viteDevURL`  
When resolving public href for module/CSS asset  
Then dev URL base MUST be used.

Given runtime is non-dev  
When resolving public href  
Then `publicPathPrefix` base MUST be used.

### FE-ASSET-006: Public Path Join Normalization

Given asset href is built from configured public prefix plus relative bundle path  
When either side includes a boundary slash (`.../` + `/...`)  
Then final resolved href MUST normalize to a single slash boundary (no duplicate `//` caused by join).

### FE-ASSET-007: Public Href Base Empty-Dev Fallback

Given runtime has `viteDevURL` set to a non-empty value  
When resolving public href for module/CSS assets  
Then that value MUST be selected as base.

Given `viteDevURL` is empty or absent  
When resolving public href  
Then runtime MUST fall back to `publicPathPrefix` base.

### FE-ASSET-008: CSS Preload Failure Tolerance During Navigation

Given navigation has pending CSS preload promises  
When one or more CSS preload promises reject  
Then runtime MUST treat CSS preload failure as non-fatal for navigation
completion: it MUST log/report the preload failure and continue navigation
commit instead of aborting successful route-data application.

### FE-ASSET-009: Duplicate CSS Preload Calls Return Immediate Resolved Promise

Given a CSS preload link for resolved href `H` already exists in document head  
When `AssetManager.preloadCSS(...)` is called again for that same resolved href  
Then call MUST return an already-resolved promise and MUST NOT register new
load/error handlers on the existing preload link.

Given the original preload link later resolves or rejects  
When duplicate-call promise outcome is observed  
Then duplicate-call promise MUST remain resolved independently of the original
preload-link terminal event.

### FE-ASSET-010: Escaped-Href Selector Safety for Preload Deduplication

Given resolved public href contains CSS-selector-sensitive characters  
When module/CSS preload dedupe query selectors are evaluated  
Then runtime MUST use escaped selector-safe href matching semantics equivalent to
`CSS.escape(resolvedHref)` and dedupe by exact resolved href identity.

## 4.10 Link Interaction and Prefetch Contract

### FE-LINK-001: Eligible Prefetch Targets

Given `__getPrefetchHandlers({ href })`  
When href is non-HTTP or external  
Then function MUST return no handlers.

Given href is internal HTTP target  
When called  
Then function MUST return `start`, `stop`, `onClick` handlers.

### FE-LINK-002: Prefetch Intent Delay

Given prefetch handlers start is called  
When no explicit delay provided  
Then prefetch MUST start after default intent delay of 100ms.

### FE-LINK-003: Prefetch Cancellation

Given delayed prefetch timer exists  
When handler `stop()` is called before start  
Then pending timer MUST be canceled and no fetch issued.

Given in-flight prefetch is still pure prefetch intent  
When `stop()` is called  
Then prefetch navigation MUST be aborted and removed.

Touch nuance:

- pointer-leave MUST NOT cancel prefetch on touch-device mode,
- blur/touch-cancel cancellation behavior MUST still apply.

### FE-LINK-004: Hash-Only Link Behavior

Given clicked link changes only hash within same document  
When link handler runs  
Then runtime MUST save scroll state and MUST NOT trigger route-data navigation fetch.

### FE-LINK-005: Internal Link Default Prevention

Given click is eligible for internal client navigation  
When link handler runs  
Then runtime MUST prevent default browser navigation and perform Vorma navigation flow.

Given click is ineligible (external/modifier/new-tab semantics)  
When link handler runs  
Then runtime MUST NOT hijack default browser behavior.

### FE-LINK-006: Callback Ordering

Given link/prefetch callbacks are provided  
When navigation flow runs  
Then callbacks MUST run in this order:

1. `beforeBegin` (once per actual begin),
2. `beforeRender`,
3. `afterRender`.

If prefetch has already begun before click-upgrade, click handling MUST NOT
invoke duplicate `beforeBegin` for that same begin.

### FE-LINK-007: Framework Link Parity

Given React/Preact/Solid `VormaLink` components  
When rendered with equivalent props  
Then they MUST expose equivalent navigation/prefetch behavior through shared link helper contract.

### FE-LINK-008: Framework Link Anchor Wiring

Given framework `VormaLink` components render anchor elements  
When final link props are derived  
Then rendered anchors MUST:

- expose helper-derived external marker through `data-external`,
- wire helper-provided handlers for pointer-enter/focus/pointer-leave/blur/touch-cancel/click,
- avoid forwarding Vorma-only control props (`prefetch`, `scrollToTop`, `replace`, `state`) as plain DOM attributes.

### FE-LINK-009: User Handler Composition and Click Cancellation Override

Given user-provided link event handlers and internal prefetch/navigation handlers  
When pointer-enter/focus/pointer-leave/blur/touch-cancel handlers run  
Then internal prefetch start/stop behavior MUST run and user handler MUST also be
invoked for that event.

Given click handler runs  
When both user `onClick` and internal link flow exist  
Then user `onClick` MUST run first.

Given user `onClick` sets `event.defaultPrevented=true`  
When internal click flow executes  
Then internal prefetch/navigation click handling MUST no-op and MUST NOT trigger
route-data navigation.

### FE-LINK-018: Pointer/Focus/Blur/Touch Handler Ordering Contract

Given user-provided handlers are present for pointer-enter/focus/pointer-leave/blur/touch-cancel  
When final link-props handlers execute  
Then internal prefetch start/stop logic MUST execute before forwarding that same
event to the corresponding user handler.

### FE-LINK-010: Completed Prefetch Stop Cleanup

Given prefetch for URL `U` has completed and remains cached for click-upgrade
reuse  
When corresponding handler `stop()` is called  
Then runtime MUST abort/retire that prefetch control and remove cached entry for
`U` so a later new prefetch to `U` starts fresh work (new control/fetch) rather
than reusing stale completed state.

### FE-LINK-011: Hash-Only Click Must Preserve Native Default Behavior

Given clicked link is hash-only within current document  
When link helper detects hash-only case  
Then helper MUST return without calling `preventDefault`, allowing browser
native in-document hash navigation while still honoring scroll-save behavior.

### FE-LINK-012: Prefetch Delay Override Semantics

Given prefetch handlers are created with explicit `delayMs` value `D`  
When `start()` is called and no canceling stop occurs before timer expiry  
Then prefetch begin MUST wait for `D` (instead of default 100ms) and MUST NOT
begin before that configured delay elapses.

### FE-LINK-013: Prefetch Target URL Override Coherence

Given prefetch handlers are created with `href` plus explicit `search` and/or
`hash` overrides  
When `start()`, `stop()`, and click-upgrade flow (`onClick`) operate on that
handler  
Then all three paths MUST resolve and use the same effective target URL key
(normalized relative href plus override search/hash) for navigation lookup.

Given a pure prefetch entry exists for that effective target URL  
When `stop()` is called  
Then runtime MUST abort/remove that exact prefetch entry and MUST NOT leave an
orphan cached prefetch keyed by a different URL composition.

### FE-LINK-014: `stop()` Must Not Abort Upgraded Navigation

Given a prefetch entry for target URL `U` has already been upgraded to user
navigation intent  
When the originating prefetch handler `stop()` is called  
Then `stop()` MUST NOT abort/remove that upgraded navigation entry and in-flight
upgraded navigation for `U` MUST continue.

### FE-LINK-015: Prefetch Begin Must Be Single-Shot Per Handler Lifecycle

Given repeated prefetch `start()` triggers occur for the same handler lifecycle
(for example pointer/focus duplicate intent signals)  
When begin path runs  
Then runtime MUST perform at most one actual prefetch begin/fetch for that
handler lifecycle until `stop()` resets lifecycle state.

### FE-LINK-016: Direct Link Click Outcome Handling Must Enforce Cleanup and Callback Boundaries

Given `__makeLinkOnClickFn(...)` handles an eligible internal click and
navigation outcome resolves as `aborted`  
When outcome branch is processed  
Then runtime MUST remove navigation entry for the target URL and MUST NOT run
`beforeRender`/`afterRender` callbacks for that aborted outcome.

Given same helper handles outcome `redirect`  
When branch is processed  
Then runtime MUST execute `beforeRender`, remove target navigation entry before
redirect effectuation, run redirect effectuation, and then execute `afterRender`.

Given same helper handles outcome `success`  
When branch is processed  
Then runtime MUST execute `beforeRender`, process successful navigation only if
target entry still exists, and then execute `afterRender`.

### FE-LINK-017: Prefetch Behavior Must Be Explicit `intent` Opt-In

Given link helper wiring derives behavior from link props  
When `prefetch` is absent or not equal to `"intent"`  
Then runtime MUST NOT create prefetch handlers and MUST route click handling
through direct-navigation helper flow only.

Given `prefetch="intent"` with internal HTTP href  
When helper wiring is created  
Then runtime MUST create prefetch-capable handlers and use prefetch lifecycle
(`start`/`stop`/click-upgrade) semantics for that link.

## 4.11 History and Scroll Persistence Contract

### FE-SCROLL-001: Scroll Storage Keys

Runtime MUST use session storage keys:

- `__vorma__scrollStateMap` for history-entry scroll map,
- `__vorma__pageRefreshScrollState` for refresh restore state.

### FE-SCROLL-002: Scroll Map Capacity

Given more than 50 scroll entries are saved  
When new entries are added  
Then oldest entries MUST be evicted (FIFO) to cap storage.

### FE-SCROLL-003: Save Before Navigation Away

Given non-same-document POP or navigation away  
When history listener runs  
Then current scroll position MUST be saved against prior history key.

### FE-SCROLL-004: POP Same-Document Hash Handling

Given POP action within same document path/search:

- adding/updating hash MUST scroll into target hash element,
- removing hash MUST restore saved scroll state for destination key or fallback to top.

### FE-SCROLL-005: POP Different-Document Handling

Given POP action to different document  
When history listener runs  
Then runtime MUST run `browserHistory` navigation flow using current window URL.

Given that navigation fails  
When failure is detected  
Then runtime MUST trigger hard browser reload of current URL to avoid URL/UI divergence.

### FE-SCROLL-006: Page Refresh Restore Window

Given refresh scroll state was captured for same URL within 5 seconds  
When client initializes  
Then runtime MUST restore coordinates on next animation frame.

Given restore succeeds  
When refresh state is consumed  
Then runtime MUST remove the one-shot refresh state entry from session storage.

Given URL mismatch or stale timestamp  
When initializing  
Then refresh scroll MUST NOT be restored.

Given refresh/map storage payload is malformed or cannot be parsed  
When restore/load runs  
Then runtime MUST treat storage as absent and continue without throwing.

### FE-SCROLL-007: Manual Scroll Restoration Mode

Given history supports `scrollRestoration` property  
When history initializes  
Then runtime MUST set it to `manual`.

### FE-SCROLL-008: Scroll-Apply Helper Hash Fallback Contract

Given `__applyScrollState(undefined)` is invoked  
When current URL contains hash fragment `#id`  
Then runtime MUST attempt element scroll by resolving `document.getElementById("id")` and calling `scrollIntoView()` if found.

Given same call and URL has no hash (or target element is absent)  
When helper runs  
Then helper MUST no-op (no coordinate scroll fallback).

### FE-SCROLL-009: Failed Cross-Document POP Must Not Commit Last-Known Location

Given cross-document `POP` triggers browser-history navigation flow and that flow
fails (requiring hard-reload fallback)  
When failure path executes  
Then runtime MUST NOT commit failed destination as new last-known custom-history
location baseline.

Given later history updates run after such failure path  
When same runtime process is still active  
Then comparisons against last-known location MUST continue to use the previous
successfully committed baseline until a succeeding history update commits.

### FE-SCROLL-010: Successful History Updates Must Commit Last-Known Location

Given history listener processes an update path that completes without failed
cross-document `POP` fallback  
When listener commit phase completes  
Then runtime MUST update last-known history location baseline to the new
location snapshot.

Given a later history update  
When key/path/search/hash comparisons run  
Then comparisons MUST use the most recently committed successful baseline.

### FE-SCROLL-011: Scroll-Apply Helper Explicit-State Semantics

Given `__applyScrollState({ x, y })` is invoked  
When helper executes  
Then runtime MUST call `window.scrollTo(x, y)`.

Given `__applyScrollState({ hash: "id" })` is invoked with non-empty hash  
When helper executes  
Then runtime MUST attempt `document.getElementById("id")?.scrollIntoView()`.

Given explicit state argument is provided  
When helper executes  
Then helper MUST use explicit-state branch semantics and MUST NOT fall back to
reading `window.location.hash` for that call.

Given explicit hash-state target is missing (or explicit hash is empty)  
When helper executes  
Then helper MUST no-op (no coordinate fallback).

### FE-SCROLL-012: Refresh Scroll Entry Consumption Boundary

Given refresh scroll state exists but init skip conditions are met (URL mismatch
or stale timestamp)  
When init evaluates refresh restore path  
Then runtime MUST NOT consume/remove the stored refresh entry for that skip
path.

## 4.12 Events Contract

### FE-EVT-001: Event Names

Runtime MUST emit/listen on window for these event names:

- `vorma:status`
- `vorma:route-change`
- `vorma:location`
- `vorma:build-id`

### FE-EVT-002: Listener Adders Return Cleanup

Given `addStatusListener`, `addRouteChangeListener`, `addLocationListener`, or `addBuildIDListener`  
When called with listener function  
Then each MUST register listener on `window` and return a cleanup function that
removes that same listener from `window`.

### FE-EVT-003: Location Event Trigger

Given history update changes location key  
When history listener runs  
Then `vorma:location` MUST be dispatched.

### FE-EVT-004: Location Event Suppression on Same Key

Given history listener runs for update where location key is unchanged from
last-known key  
When listener executes  
Then runtime MUST NOT dispatch `vorma:location`.

### FE-EVT-005: Location Event Payload Shape

Given `vorma:location` is dispatched  
When listeners inspect event payload  
Then event detail payload MUST be empty/undefined (no structured detail object).

## 4.13 HMR and Focus Revalidation

### FE-HMR-001: Dev Global Revalidate Hook

Given runtime is in dev mode  
When HMR initializes  
Then `window.__waveRevalidate` MUST be assigned to client `revalidate()`.

### FE-HMR-002: JS Update Loader Refresh

Given HMR `vite:afterUpdate` event for a registered module path and currently matched pattern  
When update event is received  
Then runtime MUST debounce (10ms window), rerun client loaders, and dispatch
route-change event.

Only `js-update` events for the same module pathname as the registered module
MUST trigger this refresh behavior.

Pathname comparison normalization:

- registration and update URL comparison MUST strip query strings before
  pathname equality checks.

### FE-HMR-003: One-Time Registration per Module URL

Given repeated HMR setup for same module URL  
When setup runs  
Then runtime SHOULD avoid duplicate event registration for same path.

Registration-key normalization:

- duplicate-registration suppression key SHOULD be the module pathname after
  query stripping so semantically identical module URLs do not register twice.

### FE-HMR-004: Focus Revalidation Policy

Given `revalidateOnWindowFocus({ staleTimeMS })` is active  
When window regains focus  
Then runtime MUST revalidate only if:

- no navigation/submission/revalidation currently active, and
- elapsed time since last nav/revalidate is at least `staleTimeMS` (default 5000ms).

Listener/timing inheritance (from `vorma/kit/listeners`):

- focus revalidation trigger source MUST include both window `focus` and
  `visibilitychange` transitions where document becomes `visible`,
- callback execution MUST be debounced through kit listener helper timing
  (current inherited debounce window: 30ms),
- helper return value MUST be cleanup callback that unregisters the installed
  focus/visibility listeners.

### FE-HMR-005: HMR Hot-Context Resolution Fallback

Given HMR update registration helper receives caller `importMeta` context  
When runtime resolves hot-update event source  
Then it MUST prefer caller `importMeta.hot` and fallback to local module
`import.meta.hot` when caller context is absent.

Given both caller and local hot contexts are unavailable  
When helper runs  
Then runtime MUST no-op registration without throwing.

### FE-HMR-006: Focus-Revalidation Staleness Clock Update Scope

Given navigation outcome commits with navigate intent or pending revalidation
commits successfully  
When outcome finalization runs  
Then focus-revalidation staleness clock MUST advance to current time.

Given outcome is pure prefetch completion, redirect handoff, aborted outcome, or
failed outcome  
When outcome finalization runs  
Then focus-revalidation staleness clock MUST NOT be advanced by that outcome.

### FE-HMR-007: Non-Dev HMR Initialization Inertness

Given runtime is not in dev mode  
When `initHMR()` runs  
Then initialization MUST be inert:

- MUST NOT assign `window.__waveRevalidate`,
- MUST leave HMR registration helper behavior as no-op (no listener
  registration side effects).

## 4.14 UI Adapter Parity (React/Preact/Solid)

### FE-UI-001: Root Outlet Functional Parity

Given equivalent client global state and events  
When React/Preact/Solid root outlet trees render  
Then they MUST follow equivalent behavior for:

- nested outlet recursion,
- error boundary rendering at outermost error index,
- fallback outlet rendering when parent component missing but deeper route exists.

### FE-UI-002: Route-Change Driven State Sync

Given `vorma:route-change` dispatch  
When adapter listeners run  
Then each adapter MUST update loader/client-loader/router/error/component snapshots from client global.

### FE-UI-003: Scroll Apply Timing

Given route-change event includes `__scrollState`  
When root outlet index is 0  
Then adapters MUST apply scroll restoration on animation frame after event-driven update.

### FE-UI-004: Location API Parity

Given location changes  
When adapter-level location state is observed  
Then React `useLocation`, Preact `location` signal, and Solid `location` signal MUST reflect same location contract.

### FE-UI-005: Typed Link/Typed Navigate Path Parity

Given typed helper usage (`makeTypedLink`, `makeTypedNavigate`) with same pattern/params/splat/search/hash/state  
When helpers resolve destination URL  
Then navigation target resolution MUST match base runtime path resolution rules.

### FE-UI-006: Adapter Listener Initialization Must Be Idempotent

Given root outlet trees mount/remount or rerender repeatedly in the same page lifetime  
When adapters initialize route/location subscriptions  
Then each adapter runtime instance MUST avoid duplicate listener registration and MUST keep route/location updates single-applied per dispatched event.

### FE-UI-007: Root Render Snapshot Refresh Contract

Given root outlet index `0` renders after client-global route state has changed  
When root outlet computes render inputs  
Then adapter snapshots (`loadersData`, `clientLoadersData`, `routerData`, error/component/import/export state) MUST refresh from client global before descendant outlet selection.

### FE-UI-008: Child Outlet Remount Identity Contract

Given parent route level stays active while next-level route module/export changes  
When outlet recursion renders child level  
Then child outlet identity MUST include next-level `importURL` + `exportKey` so stale child instance state is not reused across module/export swaps.

### FE-UI-009: Missing Error-Boundary Fallback Text Contract

Given outermost error index is active and no error-boundary component is available  
When root outlet renders error branch  
Then output MUST contain text prefix `Error: ` and MUST use `unknown` when effective error payload is falsy.

### FE-UI-010: Pattern-Based Typed Loader Helper Match Reactivity Contract

Given `makeTypedUsePatternLoaderData()(pattern)` is used and route snapshots
change while `pattern` input is unchanged  
When helper output is observed after route-change synchronization  
Then helper MUST resolve against the current `routerData.matchedPatterns`
snapshot and:

- return loader data for the first exact-match pattern index,
- return `undefined` when no exact-match index exists.

### FE-UI-011: Pattern-Based Typed Client-Loader Accessor Reactivity Contract

Given accessor returned by `makeTypedAddClientLoader(...)` is used without route
props  
When route snapshots change while registered loader pattern is unchanged  
Then accessor MUST resolve against the current `routerData.matchedPatterns`
snapshot and:

- return client-loader data for the first exact-match pattern index,
- return `undefined` when no exact-match index exists.

### FE-UI-012: Undefined-to-Defined Route-Component Identity Transition Contract

Given a route level `idx` has no current component identity metadata
(`importURLs[idx]`/`exportKeys[idx]` absent) and a later route-change snapshot
introduces identity metadata for that same `idx`  
When adapters process route-change updates  
Then React/Preact/Solid outlet resolution at `idx` MUST transition from
component-absent state to component-present state using the new identity
metadata, and MUST NOT remain stuck due solely to prior undefined identity.

### FE-UI-013: Component-Absent Terminal Branch Must Render No Wrapper Node

Given a route level where no active component is available at `idx` and no
deeper fallback outlet exists (`idx + 1 >= loadersData.length`)  
When adapter outlet renders that branch  
Then rendered output for that branch MUST be node-empty (no synthetic wrapper
element inserted solely for absence handling).

### FE-UI-014: Route-DSL Helper Type Constraint and Runtime No-Op Contract

Given consumer route-definition files call `route(pattern, importPromise,
componentKey, errorBoundaryKey?)` from `vorma/client`  
When TypeScript type-checking evaluates callsite arguments  
Then:

- `componentKey` MUST be constrained to keys of `Awaited<importPromise>`,
- optional `errorBoundaryKey` MUST use that same key domain.

Given route helper executes at runtime  
When function is invoked with any valid argument values  
Then helper MUST be side-effect free and MUST return `undefined` (`void`
contract).

### FE-UI-015: Typed Route Component Prop Alias Contract

Given `VormaRoutePropsGeneric<JSXElement, App, Pattern>` type usage in adapter
route components  
When props are type-checked  
Then props contract MUST include:

- `idx: number`,
- `Outlet: (props: Record<string, any>) => JSXElement`,
- `__phantom_pattern: Pattern`,
- allowance for additional passthrough props.

Given `VormaRouteGeneric<JSXElement, App, Pattern>` and
`ParamsForPattern<App, Pattern>` type aliases  
When aliases are resolved  
Then:

- route component alias MUST resolve to function signature
  `(props: VormaRoutePropsGeneric<...>) => JSXElement`,
- params alias MUST resolve to pattern-derived route params typing.

### FE-UI-016: `UseRouterDataFunction` Overload and Accessor Wrapper Contract

Given `UseRouterDataFunction<App, UseAccessor>` typed helper is consumed  
When overload variants are selected  
Then helper type contract MUST support:

- props-driven pattern inference overload,
- explicit-pattern generic overload without props argument,
- no-arg fallback overload using string-param shape.

Given `UseAccessor=false`  
When return type is resolved  
Then helper MUST expose direct router-data value shape.

Given `UseAccessor=true`  
When return type is resolved  
Then helper MUST expose accessor wrapper `() => routerDataValue`.

### FE-UI-017: Pattern-Props Conditional Typing and Explicit-Index Permissive Loader Contract

Given `PatternBasedProps<App, Pattern>` usage for typed route/query/mutation
helper calls  
When TypeScript resolves prop requirements for the selected pattern metadata  
Then:

- `params` field MUST be required only when pattern metadata declares params,
- `splatValues` field MUST be required only when pattern metadata is splat-enabled.

Given `PermissivePatternBasedProps<App, Pattern>` usage for typed loader helpers  
When pattern ends with explicit loader index segment
`/<loadersExplicitIndexSegment>`  
Then accepted pattern type MUST include both explicit-index form and stripped
prefix form (`/` fallback for empty prefix).

### FE-UI-018: Typed Query/Mutation Prop Method and Input Optionality Contract

Given `VormaQueryProps<App, Pattern>` type usage  
When request init and input fields are type-checked  
Then:

- `requestInit.method` MUST be constrained to `GET` when present,
- `input` MUST be required when query input type is non-empty,
- `input` MUST be optional when query input type is empty (`null`/`undefined`/`never`).

Given `VormaMutationProps<App, Pattern>` type usage  
When request init and input fields are type-checked  
Then:

- for `POST` mutations, `requestInit` MUST remain optional with method optional
  `POST`,
- for non-`POST` mutations, `requestInit` MUST be required and MUST carry exact
  mutation method,
- `input` required/optional behavior MUST follow the same empty-vs-non-empty
  input rule as query props.

### FE-UI-019: Typed Route IO and Param Extraction Fallback Contract

Given route IO helper aliases (`VormaLoaderOutput`, `VormaQueryInput`,
`VormaQueryOutput`, `VormaMutationInput`, `VormaMutationOutput`)  
When selected route metadata lacks corresponding phantom input/output type fields  
Then helper alias resolution MUST fall back to `null | undefined`.

Given mutation method alias (`VormaMutationMethod`)  
When route metadata omits explicit method or method is non-string  
Then resolved method type MUST default to `POST`.

Given route param extraction aliases (`GetParams`, `VormaRouteParams`)  
When selected route metadata omits params array  
Then extracted params type MUST resolve to `never`.

## 4.15 Global Loading Indicator Helper Contract

### FE-GLI-001: Include Filter Semantics

Given `setupGlobalLoadingIndicator(config)` is called with no `include` option or `include="all"`  
When status changes are observed  
Then helper MUST treat any of (`isNavigating`, `isSubmitting`, `isRevalidating`) as working state.

Given `include` is an explicit subset array  
When status changes are observed  
Then helper MUST treat only selected status categories as working state.

### FE-GLI-002: Start/Stop Delay Defaults and Timer Discipline

Given helper setup omits delay options  
When timers are scheduled  
Then both start and stop delay defaults MUST be 12ms.

Given helper transitions between working and non-working states  
When scheduling timers  
Then helper MUST clear opposing timer (`start` clears pending stop, `stop` clears pending start) and keep at most one pending timer per direction.

### FE-GLI-003: Deferred Start Guard

Given status indicates working state for configured includes  
When start-delay timer fires  
Then helper MUST call `config.start()` only if indicator is not already running and status is still working at fire time.

### FE-GLI-004: Deferred Stop Guard

Given status indicates non-working state for configured includes  
When stop-delay timer fires  
Then helper MUST call `config.stop()` only if indicator is running and status is still non-working at fire time.

### FE-GLI-005: Cleanup Behavior

Given setup function return cleanup callback is invoked  
When cleanup executes  
Then helper MUST remove status listener, clear pending timers, and call `stop()`
if indicator is still running.

### FE-GLI-006: Setup-Time Status Evaluation

Given helper setup occurs while runtime status is already in working or
non-working state  
When `setupGlobalLoadingIndicator` initializes  
Then helper MUST immediately evaluate current status (without waiting for a new
status event) and schedule start/stop behavior using that current snapshot.

### FE-GLI-007: Empty-Include Disable Semantics

Given helper is configured with `include: []`  
When status changes are observed  
Then helper MUST treat all status combinations as non-working (no included
categories).

Given same config and indicator is not running  
When status changes are observed  
Then helper MUST NOT call `start()`.

Given same config and indicator is running  
When stop-delay guard conditions are met  
Then helper MUST stop using normal deferred-stop guard semantics.

## 4.16 Error Utility Contract

### FE-ERR-001: Abort Error Classification Contract

Given `isAbortError(value)` receives an `Error` instance with `name === "AbortError"`  
When classification runs  
Then helper MUST return `true`.

Given helper receives a non-`Error` object with property `name === "AbortError"`  
When classification runs  
Then helper MUST return `true`.

Given helper receives values that do not satisfy either shape above  
When classification runs  
Then helper MUST return `false`.

### FE-ERR-002: Panic Logging and Throw Contract

Given `panic(msg)` is called  
When helper executes  
Then helper MUST log panic-level error context via `logError("Panic")` and MUST
throw `Error(msg)`.

Given `panic()` is called without message  
When helper executes  
Then thrown error message MUST default to `"panic"`.

## 5. Executable Conformance Scenario Catalog

This section defines concrete black-box scenarios that SHOULD be used as the
default test vectors for requirements above.

Scenario IDs are stable references for test planning and CI reporting.

## 5.1 Bootstrap and Initialization Scenarios

### FEC-INIT-001 (covers FE-INIT-001, FE-INIT-009)

Given `initClient` is called with valid options  
When startup runs  
Then runtime MUST initialize core primitives and execute startup pipeline in
order: initial component load, initial client loaders, error boundary
resolution, then `renderFn`.

### FEC-INIT-002 (covers FE-INIT-002, FE-INIT-010)

Given refresh scroll state exists and `beforeunload` is triggered later  
When init completes and then page unloads  
Then runtime MUST restore refresh scroll during init and save fresh refresh
state on unload.

### FEC-INIT-003 (covers FE-INIT-003, FE-INIT-004)

Given bootstrap route metadata and app config runes  
When init runs  
Then module map and pattern registry MUST be initialized from bootstrap/config.

### FEC-INIT-004 (covers FE-INIT-005)

Given `routeManifestURL` is present  
When fetch succeeds  
Then route manifest MUST be stored and all manifest patterns registered.

Given fetch fails  
When init continues  
Then runtime MUST remain usable (no fatal init failure).

Given manifest fetch fails  
When diagnostics are observed  
Then runtime SHOULD emit warning-level diagnostic.

### FEC-INIT-005 (covers FE-INIT-006)

Given one init with explicit `defaultErrorBoundary` and one without  
When both complete  
Then explicit boundary MUST win in first case and built-in default in second.

### FEC-INIT-006 (covers FE-INIT-007)

Given `useViewTransitions=true`  
When init completes  
Then runtime MUST enable transition mode flag for eligible navigations.

### FEC-INIT-007 (covers FE-INIT-008)

Given URL contains `vorma_reload=<id>` plus other query keys  
When init runs  
Then runtime MUST remove only `vorma_reload` and history-replace cleaned URL.

### FEC-INIT-008 (covers FE-INIT-011)

Given first `touchstart` event occurs  
When event fires after init  
Then runtime MUST set touch-device flag and keep touch-aware prefetch behavior.

### FEC-INIT-009 (covers FE-INIT-012)

Given bootstrap tuple arrays include missing `pattern`/`importURL` slots and
missing export keys  
When `initClient` seeds `clientModuleMap`  
Then only complete tuples MUST be materialized and defaults MUST apply for
missing `exportKey`/`errorExportKey`.

### FEC-INIT-010 (covers FE-INIT-013)

Given `routeManifestURL` fetch remains unresolved during startup  
When `initClient` completes startup warmup  
Then initial render path MUST complete without waiting for manifest fetch
resolution.

### FEC-INIT-011 (covers FE-INIT-014)

Given `initClient` registers touch detection  
When `touchstart` listener registration is observed and multiple touch events
fire  
Then listener registration MUST use one-shot semantics and
`isTouchDevice` MUST remain true after the first touch.

## 5.2 Client Global and Accessor Scenarios

### FEC-CTX-001 (covers FE-CTX-001)

Given bootstrap script executed  
When reading `globalThis[Symbol.for("__vorma_internal__")]`  
Then runtime global object MUST exist.

Given `__getVormaClientGlobal().set(key, value)` followed by
`__getVormaClientGlobal().get(key)`  
When accessor wrappers are exercised  
Then accessors MUST behave as direct pass-throughs to the symbol-keyed global
store.

### FEC-CTX-002 (covers FE-CTX-002)

Given route state with and without root data  
When `getRouterData()` is called  
Then output MUST include `buildID`, `matchedPatterns`, `params`,
`splatValues`, and `rootData` (`null` when `hasRootData=false`).

### FEC-CTX-003 (covers FE-CTX-003, FE-CTX-004, FE-CTX-005)

Given active runtime  
When calling `getLocation()`, `getBuildID()`, `getRootEl()`  
Then outputs MUST match browser location, current build id, and `#vorma-root`.

Given history state was set through runtime history APIs  
When `getLocation()` is called  
Then returned `state` MUST match current runtime history instance state.

### FEC-CTX-004 (covers FE-CTX-006)

Given server/client error index combinations  
When effective error is derived  
Then chosen index MUST be outermost (lowest) and message source MUST come from
selected side.

### FEC-CTX-005 (covers FE-CTX-007)

Given router accessor runs with absent optional fields and with `hasRootData`
true vs false  
When `getRouterData()` is called  
Then payload MUST apply documented default fallbacks and `rootData` projection
rules.

### FEC-CTX-006 (covers FE-CTX-008)

Given accessor usage across multiple calls  
When `getHistoryInstance()` is invoked repeatedly  
Then returned object MUST remain stable as a singleton and expose at least
the documented history action/location payload plus navigation and
observer/control method surface.

### FEC-CTX-007 (covers FE-CTX-009)

Given the symbol-keyed bootstrap global is absent from `globalThis`  
When `__getVormaClientGlobal().get(...)` or `.set(...)` is invoked  
Then invocation MUST throw synchronously and runtime MUST NOT materialize a
fallback symbol-keyed global store.

## 5.3 Navigation State Machine Scenarios

### FEC-NAV-001 (covers FE-NAV-001)

Given navigation API usage across all declared types  
When operations are triggered  
Then runtime MUST accept and process all supported navigation types.

### FEC-NAV-002 (covers FE-NAV-002, FE-NAV-004)

Given repeated user navigation to same URL and overlapping attempts  
When tracking in-flight work  
Then runtime MUST maintain one active navigate slot and reuse same-target
control.

### FEC-NAV-003 (covers FE-NAV-003)

Given active navigate/revalidate/prefetch entries  
When new user navigation to different target starts  
Then conflicting active/revalidation/prefetch work MUST be aborted.

### FEC-NAV-004 (covers FE-NAV-005, FE-NAV-006)

Given prefetch exists for target URL  
When user navigation to same URL begins  
Then prefetch MUST upgrade to navigation and duplicate fetch MUST NOT occur.

### FEC-NAV-005 (covers FE-NAV-007)

Given prefetch target equals current document URL (ignoring hash)  
When prefetch is requested  
Then no network prefetch MUST be issued.

### FEC-NAV-006 (covers FE-NAV-008, FE-NAV-009)

Given multiple revalidations and URL change during one revalidation  
When operations complete  
Then revalidations MUST coalesce and stale-origin revalidation MUST NOT render
into new location.

Given same stale-origin revalidation case  
When post-completion state is inspected  
Then stale payload MUST remain no-commit for route-state snapshots, route-change
dispatch, title/head updates, module-map writes, and CSS-apply side effects.

### FEC-NAV-007 (covers FE-NAV-010, FE-NAV-012)

Given mixes of navigation/revalidation/submission/abort/failure  
When status is observed via `getStatus()` and events  
Then flags MUST reflect true active work and clear after completion/failure.

### FEC-NAV-008 (covers FE-NAV-011, FE-NAV-013)

Given rapid chained transitions (submit->revalidate, redirect chains, wait
phases)  
When status events emit  
Then events MUST be debounced/deduped and MUST NOT show transient no-loading
gap before final completion.

### FEC-NAV-009 (covers FE-NAV-014)

Given an in-flight revalidation already targeting URL `U`  
When user navigation to `U` starts before revalidation completes  
Then runtime MUST upgrade/reuse that in-flight work as user navigation and MUST
NOT issue a second fetch for `U`.

### FEC-NAV-010 (covers FE-NAV-015)

Given mixed in-flight active navigation, prefetch entries, pending revalidation,
and submissions  
When `navigationStateManager.clearAll()` is invoked  
Then all tracked controls MUST be aborted/cleared and status MUST converge to
non-loading after debounce.

### FEC-NAV-011 (covers FE-NAV-016)

Given concurrent submissions with and without `skipGlobalLoadingIndicator=true`  
When status snapshots are observed  
Then only non-opted-out submissions MUST contribute to `isSubmitting=true`.

### FEC-NAV-012 (covers FE-NAV-017)

Given navigation/revalidation/submission transitions are started  
When `getStatus()` is sampled synchronously before debounce window elapses  
Then values MUST immediately reflect live in-memory state transitions without
waiting for status-event emission.

### FEC-NAV-013 (covers FE-NAV-018)

Given active navigation/revalidation entries are created, removed, and settled
across lifecycle transitions  
When `getNavigationsSize()`, `getNavigations()`, and `removeNavigation(key)` are
used  
Then introspection view MUST reflect exactly tracked entries and explicit remove
MUST abort+retire the targeted entry.

Given returned map from `getNavigations()` is mutated by caller and overlap cases
include duplicate target URLs across slots  
When introspection is re-read  
Then internal tracking MUST remain unaffected by caller-map mutation and URL-key
projection MUST remain deterministic under documented slot-collision order.

Given same-URL overlap exists across slots  
When `getNavigationsSize()` and `getNavigations().size` are compared  
Then slot-count view MUST be allowed to exceed projected-unique-key map size.

### FEC-NAV-014 (covers FE-NAV-019)

Given active navigation to URL `U` and separate pending revalidation to URL
`R` are each established in isolation  
When prefetch begin is requested for the same URL as the active slot and then
for the same URL as the revalidation slot  
Then runtime MUST return the already-existing control for each case and MUST
not start duplicate fetch/prefetch work for that target.

## 5.4 Request, Redirect, and Build-ID Scenarios

### FEC-FETCH-001 (covers FE-FETCH-001, FE-FETCH-002)

Given navigation/revalidation fetches  
When request URLs are built  
Then `vorma_json` MUST contain current build id and revalidation MUST include
`dpl` when deployment id exists and MUST omit `dpl` when deployment id is absent/empty.

### FEC-FETCH-002 (covers FE-FETCH-003)

Given runtime route/action fetch  
When request headers are inspected  
Then `X-Accepts-Client-Redirect` MUST be set to `1`, including overwrite of any
pre-existing caller header value.

### FEC-FETCH-003 (covers FE-FETCH-004)

Given submit calls with body variants (`FormData`, string, object)  
When request is issued  
Then `FormData`/string MUST pass through and object MUST JSON-stringify.

### FEC-FETCH-004 (covers FE-FETCH-005)

Given response includes all redirect signals  
When redirect parser runs  
Then selected redirect MUST follow precedence:
`X-Vorma-Reload` -> `response.redirected` -> `X-Client-Redirect`.

### FEC-FETCH-005 (covers FE-FETCH-006, FE-FETCH-007)

Given redirect targets that are non-HTTP, internal HTTP, and external HTTP  
When redirect decision is made  
Then non-HTTP MUST be ignored, internal MUST be soft redirect, external MUST be
hard redirect.

### FEC-FETCH-006 (covers FE-FETCH-008)

Given `X-Vorma-Reload` redirect to internal URL  
When redirect executes  
Then hard redirect URL MUST include `vorma_reload=<latest build id>`.

### FEC-FETCH-007 (covers FE-FETCH-009)

Given redirect chain exceeds max redirect threshold  
When redirect handling continues  
Then recursion MUST stop and runtime MUST report redirect-loop failure.

### FEC-FETCH-008 (covers FE-FETCH-010, FE-FETCH-011)

Given response build id differs from client build id and redirect may also
occur  
When response is processed  
Then `vorma:build-id` event MUST dispatch with `{oldID,newID}` before redirect
handoff completes.

### FEC-FETCH-009 (covers FE-FETCH-012)

Given fetch/network/JSON parsing failures  
When navigation resolves  
Then runtime MUST clean state and leave current page stable (no partial target
apply).

### FEC-FETCH-010 (covers FE-FETCH-013, FE-FETCH-014, FE-FETCH-015)

Given keyed/non-keyed submit calls, method variants, and native redirected
responses  
When submit flow is observed  
Then keyed duplicates MUST abort prior keyed submission, auto-revalidation MUST
follow policy by method/options/redirect outcome, and native redirect signal
MUST apply only to GET-origin requests.

### FEC-FETCH-011 (covers FE-FETCH-016)

Given submit flow with deployment id present in client global  
When request headers are inspected  
Then `x-deployment-id` MUST be sent on submit requests.

Given submit flow with deployment id absent/empty in client global  
When request headers are inspected  
Then `x-deployment-id` MUST be omitted.

### FEC-FETCH-012 (covers FE-FETCH-017)

Given navigation response has stale build id relative to current client build id  
When successful outcome is processed  
Then runtime MUST avoid both `clientModuleMap` mutation and CSS apply from that
stale response payload.

### FEC-FETCH-013 (covers FE-FETCH-018)

Given native fetch redirect points to current document URL  
When redirect parser runs  
Then redirect status MUST be terminal (`did`) and no extra redirect navigation
MUST be scheduled.

### FEC-FETCH-014 (covers FE-FETCH-019)

Given submit success, non-OK HTTP response, abort, non-abort error, and
no-response failure cases  
When submit promises settle  
Then submit MUST resolve with documented success/error envelopes and MUST NOT
throw.

### FEC-FETCH-015 (covers FE-FETCH-020)

Given redirect handoff begins while redirect/revalidation entries may still be
tracked  
When handoff is effectuated  
Then those stale redirect/revalidation entries MUST be aborted/removed before
handoff completes.

### FEC-FETCH-016 (covers FE-FETCH-021)

Given a navigation with non-default history options (`state`, `replace`,
`scrollToTop`) receives a soft redirect  
When redirect navigation is executed  
Then redirected navigation commit MUST preserve those options.

### FEC-FETCH-017 (covers FE-FETCH-022)

Given current location already has non-empty query params  
When `revalidate()` request URL is inspected  
Then existing query params MUST remain present and `vorma_json` MUST be
added/updated on that same current-location target URL.

### FEC-FETCH-018 (covers FE-FETCH-023)

Given submit path catches a thrown non-`Error` value (for example string/object)
When submit promise settles  
Then runtime MUST resolve with `{ success: false, error: "Unknown error" }` and
MUST NOT throw.

### FEC-FETCH-019 (covers FE-FETCH-024)

Given navigation failures across network rejection, non-OK status, and
invalid/empty JSON payload cases  
When each failure settles  
Then runtime MUST preserve current committed route snapshot (location/title/head
and route-state globals) and later successful navigation MUST still commit
normally.

### FEC-FETCH-020 (covers FE-FETCH-025)

Given equivalent successful route-data payloads observed in both development and
production mode with overlapping `importURLs`/`deps` lists and duplicate/falsy
entries  
When module-preload side effects are observed  
Then development mode MUST preload from `importURLs`, production mode MUST
preload from `deps`, and effective preload operations MUST be deduplicated and
ignore falsy candidates.

### FEC-FETCH-021 (covers FE-FETCH-026)

Given a prefetch request returns redirect outcome while entry intent remains
`none`  
When navigation outcome handling settles  
Then runtime MUST retire that prefetch entry and MUST NOT run redirect
effectuation side effects.

### FEC-FETCH-022 (covers FE-FETCH-027)

Given submit request returns redirect-handoff outcome  
When submit promise resolves  
Then result MUST report success with `data` equal to `undefined`.

### FEC-FETCH-023 (covers FE-FETCH-028)

Given one request build path where client global build id is empty and another
where build id is non-empty  
When route/revalidation request URLs are inspected  
Then empty build-id case MUST emit `vorma_json=1`, and non-empty case MUST emit
the current build id value.

## 5.5 Client-Only Skip Optimization Scenarios

### FEC-SKIP-001 (covers FE-SKIP-001)

Given eligible transition with full local data  
When skip optimization is used  
Then navigation MUST succeed without server fetch.

### FEC-SKIP-002 (covers FE-SKIP-002)

Given missing route manifest or pattern registry  
When transition is attempted  
Then skip optimization MUST NOT execute.

### FEC-SKIP-003 (covers FE-SKIP-003, FE-SKIP-004, FE-SKIP-005)

Given transitions introducing server-loader removal, new client loader, or
loader-relevant search/param/splat changes  
When evaluating skip  
Then runtime MUST force server fetch.

### FEC-SKIP-004 (covers FE-SKIP-006, FE-SKIP-008)

Given skip path produces synthetic route data  
When arrays are inspected  
Then index alignment MUST match runtime route depth ordering.

Given projected loader data is inspected by pattern role  
When one route has server loader and sibling route has no server loader  
Then server-loader route MUST retain current cached loader data for that
pattern, and non-server-loader route MUST project `undefined`.

### FEC-SKIP-005 (covers FE-SKIP-007, FE-SKIP-009)

Given skip candidate has missing client module metadata for a matched pattern  
When skip eligibility is evaluated  
Then runtime MUST reject skip and perform server fetch.

Given skip path with existing non-`undefined` client-loader data for matched
pattern `P`  
When skip navigation completes  
Then runtime MUST reuse current client-loader value for `P` without re-invoking
that loader for that navigation.

### FEC-SKIP-006 (covers FE-SKIP-010)

Given skip path synthesizes route-data and local response metadata  
When synthesized payload defaults are inspected  
Then `deps`/`cssBundles` and `errorExportKeys` MUST be empty and server/head
error fields MUST remain unset.

When synthesized response metadata is inspected  
Then response MUST be JSON `200` with
`X-Vorma-Build-Id=<current client build id>` (or `"1"` fallback when build id
is absent/empty).

### FEC-SKIP-007 (covers FE-SKIP-011)

Given skip optimization is evaluated for a target pathname that has no nested
route match in client matcher registry  
When transition executes  
Then runtime MUST not take skip path and MUST perform server fetch.

## 5.6 Client Loader Scenarios

### FEC-CL-001 (covers FE-CL-001, FE-CL-002)

Given registered client loader patterns and partially matchable paths  
When registration and partial matching run  
Then patterns MUST register and longest valid parent partial match MUST be
returned when full match is absent.

### FEC-CL-002 (covers FE-CL-003)

Given client loader executes during navigation  
When loader invocation is observed  
Then payload MUST include `params`, `splatValues`, `serverDataPromise`, and
`signal`.

### FEC-CL-003 (covers FE-CL-004, FE-CL-005)

Given server outermost error index and parallel client loaders with one failing
loader  
When execution proceeds  
Then loader at server-error index and deeper MUST be skipped and deeper child
loaders MUST abort after first non-abort client error.

### FEC-CL-004 (covers FE-CL-006)

Given loader rejects with abort-like error  
When results are projected  
Then runtime MUST treat abort as cancellation, not a client error message.

### FEC-CL-005 (covers FE-CL-007, FE-CL-008)

Given init and later route navigation  
When loaders run  
Then initial loaders MUST complete before first render and navigation MUST wait
for loader completion before final commit.

### FEC-CL-006 (covers FE-CL-009)

Given first true client loader error occurs at index `i`  
When loader state is projected  
Then `outermostClientErrorIdx=i` and `outermostClientError` MUST match error
message.

### FEC-CL-007 (covers FE-CL-010)

Given client loader receives `serverDataPromise`  
When both successful and unavailable-server-data branches are observed  
Then promise resolution MUST match required shape and MUST use fallback object
(`[]`, `undefined`, `null`, `"1"`) for unavailable-server-data branch.

### FEC-CL-008 (covers FE-CL-011)

Given a loader pattern is started during fetch and is still pending  
When completion phase executes  
Then runtime MUST reuse the running promise and MUST NOT call that loader twice
for one navigation.

### FEC-CL-009 (covers FE-CL-012)

Given registered patterns where pathname `/parent/123/details/extra` has no full
match but prefix `/parent/123` is matchable  
When `findPartialMatchesOnClient` is called  
Then helper MUST return the longest successful prefix match and MUST NOT return
a shorter fallback if a longer one succeeds.

Given pathname has full match  
When helper is called  
Then helper MUST return that full match (not downgraded prefix match).

Given pathname has no matching prefix  
When helper is called  
Then helper MUST return `null`.

## 5.7 Component and Error Boundary Scenarios

### FEC-COMP-001 (covers FE-COMP-001, FE-COMP-002)

Given route-data import/export metadata  
When components are loaded  
Then imports MUST resolve through public-href rules and component lookup MUST
map by export key (default fallback when omitted).

### FEC-COMP-002 (covers FE-COMP-003)

Given resolved active component list equals current list  
When component handler runs  
Then active component state SHOULD remain unchanged.

### FEC-COMP-003 (covers FE-COMP-004)

Given effective error index with and without route-specific error export  
When boundary resolves  
Then route-specific boundary MUST win when present, otherwise default boundary
MUST be used.

### FEC-COMP-004 (covers FE-COMP-005)

Given module namespace throws during configured error-export access  
When boundary resolution runs  
Then runtime MUST treat export as unresolved and fall back to default error
boundary without propagating the thrown access error.

### FEC-COMP-005 (covers FE-COMP-006)

Given built-in default boundary is selected for an active error route branch  
When boundary render output is observed  
Then output MUST match `"Route Error: <error>"` prefix contract.

## 5.8 Rendering, History, and Head Scenarios

### FEC-REN-001 (covers FE-REN-001)

Given view transitions enabled and browser support present  
When rerender runs for navigate vs prefetch/revalidation  
Then navigate-like rerenders MUST use transitions and prefetch/revalidation
MUST NOT.

### FEC-REN-002 (covers FE-REN-002, FE-REN-006)

Given successful navigation payload  
When rerender pipeline runs  
Then client-global route data MUST apply before `vorma:route-change` dispatch
and route-change detail MUST carry `__scrollState` when applicable.

Given that payload mutates one or more core route-data fields  
When `vorma:route-change` listeners inspect client globals synchronously during
dispatch  
Then each documented core field MUST already reflect the current commit payload
(no stale prior-commit field residue).

### FEC-REN-003 (covers FE-REN-003)

Given navigation history options (`replace` and target/current URL equality)  
When history updates execute  
Then runtime MUST push or replace according to contract.

### FEC-REN-004 (covers FE-REN-004)

Given browser-history (`POP`) navigation with saved scroll state  
When rerender completes  
Then route-change scroll detail MUST include restore state.

### FEC-REN-014 (covers FE-REN-020)

Given rerender commits from revalidation and pure prefetch outcomes  
When history side effects are observed  
Then runtime MUST apply no history push/replace mutation for those commits.

### FEC-REN-005 (covers FE-REN-005)

Given route-data title with HTML entities  
When title updates  
Then `document.title` MUST be decoded text.

### FEC-REN-017 (covers FE-REN-023)

Given rerender commit includes new title and a route-change listener inspects
`document.title` during event callback  
When route-change event dispatch occurs  
Then observed title value MUST equal that commit's decoded title.

### FEC-REN-016 (covers FE-REN-022)

Given successful rerender commits where one payload includes title and a later
payload omits title (`undefined`)  
When commits apply in sequence  
Then omitted-title commits MUST preserve the previously committed
`document.title` value.

### FEC-REN-006 (covers FE-REN-007, FE-REN-008)

Given head payloads are undefined, empty, present, and/or marker comments are
missing  
When head update runs  
Then undefined sections MUST stay unchanged, provided sections MUST reconcile,
and missing markers MUST no-op safely.

### FEC-REN-007 (covers FE-REN-009, FE-REN-010)

Given desired head blocks with duplicates/reordering and invalid
`attributesKnownSafe` values  
When reconciliation runs  
Then DOM MUST dedupe/reorder/remove stale nodes and MUST fail loudly on
null/undefined safe attributes.

### FEC-REN-008 (covers FE-REN-011, FE-REN-012, FE-REN-013, FE-REN-014)

Given head payloads include missing-tag blocks, boolean attributes,
dangerous-innerHTML blocks, and fingerprint-equivalent updates over existing
nodes (with inter-marker text nodes present)  
When reconciliation runs  
Then missing-tag blocks MUST be ignored, inter-marker text nodes MUST be
removed, boolean/innerHTML semantics MUST apply, and fingerprint-equivalent
nodes SHOULD preserve identity.

Given repeated reconciliation with unchanged desired blocks  
When managed head updates run multiple times  
Then no duplicate managed nodes MUST accumulate across runs.

### FEC-REN-009 (covers FE-REN-015)

Given successful navigation with head payload and a route-change listener
observing head state at dispatch time  
When render commit occurs  
Then route-change event MUST fire before that payload's head reconciliation
effects are visible.

### FEC-REN-010 (covers FE-REN-016)

Given marker comments with surrounding whitespace around marker tokens  
When head reconciliation runs  
Then runtime MUST still find boundaries and apply managed updates within the
correct section span.

### FEC-REN-011 (covers FE-REN-017)

Given equivalent head elements with differing attribute order and desired-list
duplicate fingerprints  
When reconciliation runs  
Then fingerprint matching MUST be attribute-order insensitive, node reuse MUST be
possible for fingerprint matches, and duplicate desired fingerprints MUST resolve
with last-occurrence-wins semantics.

### FEC-REN-012 (covers FE-REN-018)

Given both managed sections exist with different current content  
When reconciliation runs for exactly one section type  
Then only that section's marker-bounded node span MUST change and the other
section MUST remain unchanged.

### FEC-REN-013 (covers FE-REN-019)

Given user/redirect and browser-history navigation variants with combinations of
hash, `scrollToTop=false`, and provided/missing restore-state  
When route-change events are observed  
Then dispatched `__scrollState` payload MUST follow the documented selection
rules exactly.

### FEC-REN-015 (covers FE-REN-021)

Given navigate-intent rerender commits that execute both push and replace
branches with explicit and omitted history state payloads  
When history calls are observed  
Then state arguments passed to history APIs MUST match the current commit inputs
exactly and MUST NOT leak prior commit state.

## 5.9 Asset Management Scenarios

### FEC-ASSET-001 (covers FE-ASSET-001, FE-ASSET-002)

Given repeated module/CSS preload calls for same URL  
When preloading executes  
Then runtime MUST create at most one corresponding preload link per resource.

### FEC-ASSET-002 (covers FE-ASSET-003)

Given CSS preload link load and error outcomes  
When promise resolves  
Then preload promise MUST resolve on load and reject on error.

### FEC-ASSET-003 (covers FE-ASSET-004, FE-ASSET-005)

Given CSS bundle application and public-href resolution in dev/prod  
When assets are applied  
Then stylesheet links MUST dedupe via `data-vorma-css-bundle` and href base
MUST follow dev `viteDevURL` vs prod `publicPathPrefix` rules.

### FEC-ASSET-004 (covers FE-ASSET-006)

Given public prefix and bundle path that both include boundary slashes  
When stylesheet href is applied  
Then resulting href MUST not contain duplicate slash separators at join
boundary.

### FEC-ASSET-005 (covers FE-ASSET-007)

Given one case with non-empty `viteDevURL` and one with empty `viteDevURL` plus
`publicPathPrefix`  
When asset URLs are resolved  
Then base selection MUST follow `viteDevURL` first, else `publicPathPrefix`.

### FEC-ASSET-006 (covers FE-ASSET-008)

Given navigation waits on multiple CSS preload promises with at least one
rejection  
When waiting phase resolves  
Then navigation MUST still proceed to render/commit (non-fatal CSS preload
failure path), while the preload failure is emitted as diagnostics.

### FEC-ASSET-007 (covers FE-ASSET-009)

Given first `preloadCSS(U)` call creates a preload link and a second
`preloadCSS(U)` call runs before the first link settles  
When second-call promise and first-call promise are observed  
Then second-call promise MUST already be resolved while first-call promise
continues following link load/error outcome semantics.

### FEC-ASSET-008 (covers FE-ASSET-010)

Given module/CSS preload URLs whose resolved href contains selector-sensitive
characters  
When preload dedupe is exercised with repeated calls  
Then dedupe lookup MUST remain selector-safe (no selector parse failure) and
MUST create at most one preload link per resolved href.

## 5.10 Link and Prefetch Interaction Scenarios

### FEC-LINK-001 (covers FE-LINK-001)

Given internal HTTP, external HTTP, and non-HTTP href values  
When prefetch handlers are requested  
Then only internal HTTP targets MUST return handlers.

### FEC-LINK-002 (covers FE-LINK-002, FE-LINK-003)

Given delayed prefetch start and stop operations  
When stop occurs before and after fetch begin  
Then timer MUST cancel pre-start and pure prefetch entry MUST abort/remove
post-start.

### FEC-LINK-003 (covers FE-LINK-004)

Given hash-only in-document link click  
When click handler runs  
Then scroll state MUST be saved and route-data navigation MUST NOT fetch.

### FEC-LINK-004 (covers FE-LINK-005, FE-LINK-006)

Given eligible vs ineligible clicks with callback hooks  
When link flow runs  
Then eligible internal clicks MUST prevent default and run callbacks in order:
`beforeBegin` -> `beforeRender` -> `afterRender`; ineligible clicks MUST not
be hijacked.

### FEC-LINK-005 (covers FE-LINK-007)

Given equivalent link props in React/Preact/Solid link adapters  
When interactions are exercised  
Then navigation/prefetch behavior MUST be equivalent across adapters.

### FEC-LINK-006 (covers FE-LINK-008)

Given equivalent props rendered through React/Preact/Solid `VormaLink`  
When inspecting anchor attributes and event wiring points  
Then adapters MUST expose equivalent helper-derived anchor wiring semantics (`data-external`, interaction handlers, and control-prop stripping).

### FEC-LINK-007 (covers FE-LINK-009)

Given user-provided link handlers plus internal prefetch/navigation handlers  
When pointer/blur/touch and click interactions are exercised  
Then user handlers MUST compose with internal handlers under documented ordering,
and user click cancellation (`defaultPrevented`) MUST suppress internal click
navigation.

### FEC-LINK-008 (covers FE-LINK-010)

Given a prefetch completes for URL `U` and handler `stop()` is invoked
afterward  
When a new prefetch to `U` starts later  
Then runtime MUST create fresh prefetch control/work (no stale completed-entry
reuse).

### FEC-LINK-009 (covers FE-LINK-011)

Given hash-only in-document click through link helper flow  
When click is processed  
Then `event.defaultPrevented` MUST remain false and browser hash navigation MUST
remain native while route-data fetch is still skipped.

### FEC-LINK-010 (covers FE-LINK-012)

Given prefetch helper is configured with explicit non-default `delayMs`  
When `start()` is called  
Then prefetch begin MUST be delayed by that configured value and MUST not fire
earlier.

### FEC-LINK-011 (covers FE-LINK-013)

Given prefetch handlers are created with explicit search/hash overrides and a
prefetch is started  
When `stop()` and click-upgrade flow run on that same handler  
Then they MUST target the same effective URL key used by prefetch start (no
override-loss key mismatch).

### FEC-LINK-012 (covers FE-LINK-014)

Given prefetch for URL `U` starts and click-upgrade turns that same entry into
user navigation intent  
When the originating handler `stop()` is invoked after upgrade  
Then upgraded navigation for `U` MUST remain active (no abort/remove from
`stop()`) and completion flow MUST continue.

### FEC-LINK-013 (covers FE-LINK-015)

Given a single prefetch handler receives repeated `start()` intent events before
completion  
When begin/fetch side effects are observed  
Then at most one actual prefetch begin/fetch MUST occur for that lifecycle
until `stop()` reset.

### FEC-LINK-014 (covers FE-LINK-016)

Given direct link-click helper path (`__makeLinkOnClickFn`) with fixtures for
`aborted`, `redirect`, and `success` outcomes  
When callback and cleanup ordering is observed  
Then aborted path MUST cleanup without render callbacks, redirect path MUST run
`beforeRender` -> cleanup -> redirect effectuation -> `afterRender`, and
success path MUST run `beforeRender` -> conditional process -> `afterRender`.

### FEC-LINK-015 (covers FE-LINK-017)

Given one link-helper case with `prefetch="intent"` and another with prefetch
absent/non-intent  
When generated handler wiring is exercised for pointer/focus/click interactions  
Then prefetch lifecycle behavior MUST exist only for the `"intent"` case, while
non-intent cases MUST use click-only direct-navigation helper flow.

### FEC-LINK-016 (covers FE-LINK-018)

Given link helper props include both internal prefetch behavior and user event
handlers for pointer-enter/focus/pointer-leave/blur/touch-cancel  
When each corresponding event handler is invoked  
Then internal prefetch lifecycle side effect for that event MUST run before user
handler invocation.

## 5.11 History and Scroll Persistence Scenarios

### FEC-SCROLL-001 (covers FE-SCROLL-001, FE-SCROLL-002)

Given many saved scroll entries  
When storage state is inspected  
Then runtime MUST use documented storage keys and cap map at 50 entries via
oldest eviction.

### FEC-SCROLL-002 (covers FE-SCROLL-003)

Given navigation away from current document  
When history listener runs  
Then current scroll position MUST be saved for prior history key.

### FEC-SCROLL-003 (covers FE-SCROLL-004)

Given POP within same pathname/search with hash add/update/remove variants  
When listener runs  
Then add/update MUST scroll to hash target and hash removal MUST restore saved
state or top fallback.

### FEC-SCROLL-004 (covers FE-SCROLL-005)

Given POP to different document and navigation failure  
When browser-history flow runs  
Then runtime MUST trigger browser-history navigation and hard-reload current URL
if that navigation fails.

### FEC-SCROLL-005 (covers FE-SCROLL-006, FE-SCROLL-007)

Given refresh scroll state with fresh/stale timestamps  
When init runs  
Then fresh same-URL state MUST restore on RAF, stale/mismatched state MUST not,
and history scroll restoration mode MUST be manual.

### FEC-SCROLL-006 (covers FE-SCROLL-008)

Given `__applyScrollState(undefined)` runs with hash-present, hash-absent, and
missing-target variants  
When helper executes  
Then hash-present with existing target MUST scroll that element, and other
variants MUST no-op.

### FEC-SCROLL-007 (covers FE-SCROLL-009)

Given cross-document `POP` where browser-history navigation fails before
hard-reload fallback  
When runtime history baseline state is observed before/after failure  
Then last-known location baseline MUST remain at pre-failure committed location
until a succeeding history update commits.

### FEC-SCROLL-008 (covers FE-SCROLL-010)

Given one successful history update followed by another update with different
location metadata  
When history listener comparison and dispatch logic runs  
Then the second update MUST be evaluated against the first successful commit
baseline, confirming last-known location advanced on success.

### FEC-SCROLL-009 (covers FE-SCROLL-011)

Given `__applyScrollState` is invoked with explicit coordinate state,
explicit hash state with present target, and explicit hash state with missing
target  
When helper executes  
Then coordinate state MUST call `window.scrollTo(x, y)`, explicit hash state
MUST target that hash element via `scrollIntoView()`, and missing-target hash
state MUST no-op.

### FEC-SCROLL-010 (covers FE-SCROLL-012)

Given refresh scroll state exists but init restore is skipped due to URL
mismatch or stale timestamp  
When init completes  
Then refresh scroll entry MUST remain in session storage (not consumed by the
skip path).

## 5.12 Events Scenarios

### FEC-EVT-001 (covers FE-EVT-001)

Given runtime lifecycle actions  
When observing window events  
Then runtime MUST use event names:
`vorma:status`, `vorma:route-change`, `vorma:location`, `vorma:build-id`.

### FEC-EVT-002 (covers FE-EVT-002)

Given listener adders (`addStatusListener`, `addRouteChangeListener`,
`addLocationListener`, `addBuildIDListener`)  
When adders are invoked and later cleanup callbacks are invoked  
Then listeners MUST be registered on `window` for their corresponding event
keys and cleanup callbacks MUST remove those same listeners from `window`.

### FEC-EVT-003 (covers FE-EVT-003)

Given history location key changes  
When history listener executes  
Then runtime MUST dispatch `vorma:location`.

### FEC-EVT-004 (covers FE-EVT-004)

Given history listener receives update where location key does not change  
When listener executes  
Then runtime MUST NOT dispatch `vorma:location`.

### FEC-EVT-005 (covers FE-EVT-005)

Given history listener triggers a `vorma:location` dispatch  
When listener receives the event  
Then event detail payload MUST be empty/undefined.

## 5.13 HMR and Focus Revalidation Scenarios

### FEC-HMR-001 (covers FE-HMR-001, FE-HMR-003)

Given dev mode HMR setup repeated for same module path  
When setup runs  
Then `window.__waveRevalidate` MUST point to `revalidate` and module update
listeners SHOULD register at most once per module pathname.

### FEC-HMR-002 (covers FE-HMR-002)

Given HMR `vite:afterUpdate` js-update for matched pattern module  
When event fires  
Then runtime MUST debounce rerun of client loaders and dispatch route-change.

### FEC-HMR-003 (covers FE-HMR-004)

Given focus revalidation helper is active  
When window regains focus  
Then revalidation MUST run only when runtime is idle and elapsed stale time is
at least configured threshold.

Given helper cleanup callback is invoked after registration  
When subsequent focus/visible events occur  
Then no additional focus-triggered revalidation callbacks MUST fire.

### FEC-HMR-004 (covers FE-HMR-005)

Given caller module provides `importMeta.hot` while helper module local hot
context is unavailable  
When HMR registration helper runs  
Then update listener registration MUST still succeed using caller context.

Given neither hot context is available  
When helper runs  
Then registration MUST no-op without throwing.

### FEC-HMR-005 (covers FE-HMR-006)

Given navigate-intent success, revalidation success, prefetch success,
redirect-handoff, and aborted/failure outcomes are exercised in sequence  
When focus-revalidation staleness decisions are sampled across those outcomes  
Then only navigate-intent and revalidation successful completions MUST advance
staleness-clock freshness; other outcomes MUST not reset staleness age.

### FEC-HMR-006 (covers FE-HMR-007)

Given production/non-dev runtime environment  
When `initHMR()` is invoked and HMR registration helper is called  
Then no dev revalidate global or hot-update listener side effects MUST occur.

## 5.14 UI Adapter Parity Scenarios

### FEC-UI-001 (covers FE-UI-001)

Given equivalent runtime global state  
When React/Preact/Solid outlets render  
Then nested outlet recursion, outermost-error boundary behavior, and fallback
outlet behavior MUST be equivalent.

### FEC-UI-002 (covers FE-UI-002)

Given `vorma:route-change` events  
When adapter listeners apply state  
Then all adapters MUST synchronize loader/client-loader/router/error/component
snapshots from client global.

### FEC-UI-003 (covers FE-UI-003, FE-UI-004)

Given route-change scroll details and location changes  
When adapter state updates  
Then scroll apply timing and location API outputs MUST remain equivalent across
React hook and Preact/Solid signals.

### FEC-UI-004 (covers FE-UI-005)

Given typed helpers (`makeTypedLink`, `makeTypedNavigate`) across adapters  
When resolving destinations from same inputs  
Then resolved target URL MUST match base runtime path-resolution semantics.

### FEC-UI-005 (covers FE-UI-006, FE-UI-007)

Given repeated root-outlet mount/rerender cycles with route/location event dispatch  
When observing adapter snapshot updates  
Then per-event updates MUST remain single-applied (no duplicate-listener multiplication) and root snapshot reads MUST reflect latest client-global state before descendant selection.

### FEC-UI-006 (covers FE-UI-008)

Given same parent route with changed next-level module/export identity  
When route-change updates import/export metadata  
Then child outlet subtree MUST remount and reset child-instance-local state across adapters.

### FEC-UI-007 (covers FE-UI-009)

Given active outermost error index without error-boundary component and truthy/falsy error payload variants  
When root outlet renders error branch  
Then output MUST include `Error: <message>` and fallback to `Error: unknown` for falsy payload.

### FEC-UI-008 (covers FE-UI-010)

Given fixed `pattern` input for `makeTypedUsePatternLoaderData` and sequential
route-change snapshots that move/remove that pattern from matched outputs  
When helper output is observed after each snapshot commit  
Then helper output MUST track first-match index in the current snapshot and
MUST become `undefined` once pattern is absent.

### FEC-UI-009 (covers FE-UI-011)

Given accessor from `makeTypedAddClientLoader(...)` is read without route props
and sequential route-change snapshots that move/remove that registered pattern  
When accessor output is observed after each snapshot commit  
Then accessor output MUST track first-match index in the current snapshot and
MUST become `undefined` once pattern is absent.

### FEC-UI-010 (covers FE-UI-012)

Given a route level starts with missing `importURL`/`exportKey` identity and a
later route-change snapshot introduces identity at that same level  
When adapters render after the update  
Then outlet resolution at that level MUST promote from absent-component/fallback
state to rendering the newly available component identity.

### FEC-UI-011 (covers FE-UI-013)

Given a route level with no component identity and no deeper fallback outlet
across React/Preact/Solid harnesses  
When outlet renders terminal absent-component branch  
Then each adapter output MUST be node-empty and MUST NOT emit synthetic wrapper
elements.

### FEC-UI-012 (covers FE-UI-014)

Given one type-valid and one type-invalid `route(...)` callsite fixture in a
route-definition file  
When TypeScript compile-time checking runs  
Then valid key usage MUST type-check and invalid key usage MUST fail type-check.

Given runtime invocation of `route(...)` helper in a harness context  
When call returns  
Then return value MUST be `undefined` and no side effects MUST be observed.

### FEC-UI-013 (covers FE-UI-015)

Given compile-time route-component type fixtures using
`VormaRoutePropsGeneric`, `VormaRouteGeneric`, and `ParamsForPattern`  
When TypeScript checks fixture assignments/usages  
Then required prop keys, alias function signature, and pattern-param typing MUST
match contract.

### FEC-UI-014 (covers FE-UI-016)

Given compile-time fixtures exercising all `UseRouterDataFunction` overload
forms for `UseAccessor=false` and `UseAccessor=true`  
When TypeScript checks fixture call/return types  
Then overload selection and direct-value vs accessor-wrapper return typing MUST
match contract.

### FEC-UI-015 (covers FE-UI-017)

Given compile-time fixtures for patterns with and without params/splat metadata,
plus explicit-index and non-index loader pattern variants  
When TypeScript checks `PatternBasedProps` and `PermissivePatternBasedProps`
assignments  
Then conditional param/splat requirements and explicit-index permissive pattern
acceptance MUST match contract.

### FEC-UI-016 (covers FE-UI-018)

Given compile-time fixtures for query/mutation props across empty/non-empty input
types and POST/non-POST mutation methods  
When TypeScript checks `VormaQueryProps` and `VormaMutationProps` assignments  
Then method constraints and input optionality rules MUST match contract.

### FEC-UI-017 (covers FE-UI-019)

Given compile-time fixtures covering routes with and without phantom IO/method/params
metadata  
When TypeScript resolves exported IO/method/param helper aliases  
Then fallback defaults (`null|undefined`, `POST`, `never`) MUST match contract.

## 5.15 Global Loading Indicator Scenarios

### FEC-GLI-001 (covers FE-GLI-001)

Given helper setups using `include="all"` and explicit include subsets  
When status events vary across navigating/submitting/revalidating combinations  
Then working-state detection MUST follow include-filter contract.

### FEC-GLI-002 (covers FE-GLI-002, FE-GLI-003, FE-GLI-004)

Given rapid status flips with configurable/default delays  
When timers fire  
Then helper MUST enforce timer-discipline and call start/stop only under guarded
running/still-working predicates.

### FEC-GLI-003 (covers FE-GLI-005)

Given helper is active with pending timers and/or running indicator  
When cleanup callback runs  
Then listener MUST be removed, timers cleared, and running indicator stopped.

### FEC-GLI-004 (covers FE-GLI-006)

Given helper is initialized while status is already active-working and while it
is already idle across separate setups  
When no new status events are emitted immediately after setup  
Then helper behavior MUST still be driven from setup-time status snapshot.

### FEC-GLI-005 (covers FE-GLI-007)

Given helper is configured with `include: []` across running/not-running start
states  
When status changes are emitted  
Then helper MUST behave as fully-disabled include filter (no start; stop path
still allowed under stop-delay guard when initially running).

## 5.16 Error Utility Scenarios

### FEC-ERR-001 (covers FE-ERR-001)

Given `isAbortError` receives fixtures spanning native `AbortError` instances,
AbortError-shaped plain objects, and non-abort values  
When classification results are observed  
Then helper MUST return true only for the documented abort shapes.

### FEC-ERR-002 (covers FE-ERR-002)

Given `panic(msg)` and `panic()` invocations are exercised  
When error and logging side effects are observed  
Then helper MUST log `"Panic"` and throw `Error` with provided message or
default `"panic"` message when omitted.

## 6. Conformance Test Guidance

Frontend conformance suites SHOULD:

1. Keep each requirement ID and scenario ID independently reportable.
2. Validate ordering-sensitive behavior (status transitions, callback ordering,
   rerender phase boundaries) with deterministic async harnesses.
3. Validate redirect precedence and hard/soft strategy with controlled response
   fixtures.
4. Include same-document hash and cross-document POP scenarios.
5. Assert adapter parity by running equivalent fixtures in React, Preact, and
   Solid harnesses.
6. Keep tests independent from package-private state except public bootstrap
   symbol contract.
7. Track traceability with IDs (`FE-*`, `FEC-*`) in CI output.

## 7. Relation to Other Specs

- Backend runtime behavior:
  `/Users/sjc/__code/river/specs/VORMA_BACKEND_RUNTIME_SPEC.md`
- Wire protocol behavior:
  `/Users/sjc/__code/river/specs/VORMA_WIRE_CONTRACT_SPEC.md`
- Build/dev behavior:
  `/Users/sjc/__code/river/specs/VORMA_BUILD_DEV_CONFORMANCE_SPEC.md`
- Kit interop map: `/Users/sjc/__code/river/specs/VORMA_KIT_INTEROP_SPEC.md`
- Testing strategy:
  `/Users/sjc/__code/river/specs/VORMA_TESTING_STRATEGY_SPEC.md`
- Roadmap/checklist: `/Users/sjc/__code/river/specs/SPECS_CHECKLIST.md`

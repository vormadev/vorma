# vormaruntime Conformance Specification

Status: Active  
Last Updated: 2026-02-09  
Applies To: vormaruntime package behavior as observed through public APIs and
HTTP I/O

Current evidence status (epoch `E2-R2`):

- this catalog was imported from prior backend-runtime spec material and is under
  owner-package replay validation,
- no legacy `vormaruntime` tests outside `conformance/**` were found during this
  replay pass; current requirement support is therefore source-only until
  external legacy evidence exists.

## 1. Why This Spec Exists

This is a **conformance spec**, not an architecture note.

It is intended to:

- enable large refactors without reading implementation internals,
- drive black-box test suites,
- prevent test suites from mirroring current implementation details.

## 2. Conformance Rules

### 2.1 Normative Terms

The terms **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

### 2.2 Allowed Test Observations

Conformance tests for this spec MUST rely only on:

- public Go API behavior (`vorma` / `vormaruntime` entrypoints),
- HTTP status, headers, body,
- template-rendered output,
- externally visible logging/errors (when explicitly specified).

Conformance tests MUST NOT depend on:

- private struct fields,
- lock internals,
- private helper functions,
- package-internal implementation layout.

### 2.3 System Under Test (SUT)

The SUT is a Vorma app instance that has:

- valid Wave integration,
- build artifacts present on disk,
- loaders/actions optionally registered by the test harness.

## 3. Terminology

- Loader Route: a nested route resolved by the loaders router.
- Action Route: a method-based route resolved by the actions router.
- JSON Route-Data Mode: request mode triggered by query param `vorma_json`.
- Current Build JSON Request: `vorma_json=<current build id>`.
- Stale Build JSON Request: `vorma_json` present but not equal to current build
  id.

## 4. Requirement Catalog

## 4.1 App Construction and Initialization

### BR-INIT-001: Wave Is Required

Given `NewVormaApp` is called with `Wave=nil`  
When app construction is attempted  
Then construction MUST fail immediately (panic/fatal construction error).

### BR-INIT-002: Required Config Keys

Given config omits any of:

- `MainBuildEntry`
- `UIVariant`
- `HTMLTemplateLocation`
- `ClientEntry`
- `ClientRouteDefsFile`
- `TSGenOutDir`

When app construction is attempted  
Then construction MUST fail immediately (panic/fatal construction error).

### BR-INIT-003: Config Defaults

Given optional config fields are omitted  
When app is constructed  
Then defaults MUST apply:

- `BuildtimePublicURLFuncName="waveBuildtimeURL"`
- loader explicit index segment default `_index`
- actions mount root default `/api/`
- default supported actions methods include `GET`, `POST`, `PUT`, `DELETE`,
  `PATCH`.

### BR-INIT-004: Init Fails Without Required Artifacts

Given required runtime artifacts are missing/invalid for the active mode  
When `Init()` is called  
Then initialization MUST fail immediately (panic/error surfaced by `Init` path).

### BR-INIT-005: InitWithDefaultRouter Mounts Endpoints

Given app initializes successfully  
When `InitWithDefaultRouter()` is called  
Then returned router MUST expose:

- loaders handler on `GET /*`
- actions handler on actions mount root for each supported method.

### BR-INIT-006: Mode-Selected Paths Artifact

Given runtime initializes in dev mode  
When base paths artifact is loaded  
Then runtime MUST read stage-one paths artifact (`vorma_paths_stage_1.json`).

Given runtime initializes in non-dev mode  
When base paths artifact is loaded  
Then runtime MUST read stage-two paths artifact (`vorma_paths_stage_2.json`).

Snapshot replacement semantics:

- initialization-time in-memory route paths snapshot MUST be replaced by the
  selected artifact snapshot for that init run, not additive-merged with stale
  entries from prior init runs on the same app instance.

### BR-INIT-007: Head Dedupe Rule Bootstrap

Given app construction provides a `GetHeadDedupeKeys` callback  
When `Init()` runs  
Then runtime MUST initialize head dedupe/unique rules from callback-provided
definitions before serving requests.

Given `GetHeadDedupeKeys` is not provided  
When `Init()` runs  
Then runtime MUST retain default head dedupe/unique rules.

App-instance isolation requirement:

Given multiple Vorma app instances are initialized in the same process with
different head dedupe rule setups  
When either instance serves requests  
Then each instance's head dedupe behavior MUST remain isolated to that instance
and MUST NOT be overwritten by another app instance's initialization.

### BR-INIT-008: Constructor Callback Defaults

Given `NewVormaApp` is called without one or more optional callbacks
(`GetDefaultHeadEls`, `GetHeadDedupeKeys`, `GetRootTemplateData`)  
When app is initialized and request lifecycle executes  
Then runtime MUST supply safe defaults:

- default-head callback MUST behave as no-op success (no added head elements, no
  error),
- head-dedupe callback MUST behave as no-op (default unique rules remain),
- root-template-data callback MUST return empty map with nil error.

### BR-INIT-009: Loaders Handler Requires Non-Nil Nested Router

Given loaders handler is requested via `GetLoadersHandler`  
When caller passes `nil` nested router  
Then runtime MUST fail immediately (panic) rather than proceed with undefined
route-registration behavior.

### BR-INIT-010: Server Address Derivation After Init

Given runtime initialization succeeds and effective app port is `P`  
When `ServerAddr()` is queried  
Then returned address MUST equal `":" + P`.

### BR-INIT-011: Constructor TS-Generation Payload Passthrough

Given `NewVormaApp` receives `AdHocTypes` and `ExtraTSCode` in app config  
When `GetAdHocTypes()` / `GetExtraTSCode()` are queried before or after init  
Then returned values MUST reflect constructor-provided payload unchanged by
runtime route/template reload operations.

### BR-INIT-012: Explicit-Index Segment Must Be Slash-Free

Given loaders-router options specify an explicit index segment token containing
`/`  
When app construction initializes matcher-backed loader routing  
Then initialization MUST fail fast (panic-class construction failure) and MUST
NOT continue with invalid matcher configuration.

### BR-INIT-013: Vorma Paths Helpers Must Resolve Canonical Artifact Paths

Given callers invoke `VormaPaths.StageOneJSON()` and
`VormaPaths.StageTwoJSON()`  
When return values are observed  
Then returned paths MUST equal:

- `path.Join("vorma_out", "vorma_paths_stage_1.json")` for stage one, and
- `path.Join("vorma_out", "vorma_paths_stage_2.json")` for stage two.

### BR-INIT-014: `SetIsDev` Must Drive Mode Getter State

Given runtime mode is toggled through `SetIsDev(true)` or `SetIsDev(false)`  
When `GetIsDevMode()` is queried afterward  
Then returned mode value MUST reflect the most recent setter value.

## 4.2 Common Response Contract

### BR-RESP-001: Build ID Header on Loader Responses

Given a non-control-path request handled by loaders handler  
When response is produced  
Then header `X-Vorma-Build-Id` MUST be present.

Control-path refinement:

- this requirement applies to normal loader routing outcomes (document, JSON,
  stale-build sentinel, loader 404, proxy-short-circuit outcomes),
- dev reload control endpoints (`/__vorma/reload-routes`,
  `/__vorma/reload-template`) are governed by `BR-DEV-*` contracts and are
  excluded from `BR-RESP-001`.

### BR-RESP-002: Build ID Header on Action Responses

Given any request handled by actions handler  
When response is produced  
Then header `X-Vorma-Build-Id` MUST be present.

### BR-RESP-003: Default Loader Cache-Control Preservation/Injection

Given a request handled by loaders handler and response headers currently have
no `Cache-Control`  
When final response is produced (JSON or HTML path)  
Then runtime MUST set:
`Cache-Control: private, max-age=0, must-revalidate, no-cache`.

Given loaders flow where `Cache-Control` was already set (for example through
response proxy)  
When final response is produced  
Then runtime MUST preserve existing value and MUST NOT override it.

## 4.3 Loaders: JSON Route-Data Negotiation

### BR-JSON-001: JSON Mode Detection

Given a loaders request with query key `vorma_json` set to a non-empty value  
When request is handled  
Then runtime MUST treat request as JSON route-data mode.

### BR-JSON-002: Stale Build Redirect Signal

Given a stale build JSON request  
When request is handled  
Then response MUST be:

- HTTP 200
- header `X-Vorma-Reload` set to same URL without `vorma_json`
- JSON body indicating success (current behavior: `{"ok":true}`).

Reload URL refinement:

- non-`vorma_json` query params MUST be preserved,
- path MUST remain unchanged.

### BR-JSON-003: Current Build JSON Returns Route Data

Given a current build JSON request and matching route  
When request is handled  
Then response MUST be JSON route-data payload (not HTML).

Given JSON route-data marshaling fails  
When request is handled  
Then runtime MUST return HTTP 500 (no partial JSON payload).

### BR-JSON-004: Stale Build Signal Precedes Route-Match/Not-Found Evaluation

Given request is JSON mode with stale build token (`vorma_json` non-empty and
not equal to current build id)  
When loaders handler processes request  
Then stale-build sentinel response MUST be returned before route-match/not-found
evaluation, including for paths that would otherwise return loader 404.

## 4.4 Loaders: Routing and Matching

### BR-LOAD-001: Not Found Behavior

Given no loader route matches request path  
When loaders handler processes request  
Then response MUST be HTTP 404.

### BR-LOAD-002: Matched Pattern Ordering

Given a route match with nested parent/child loader paths  
When JSON route-data is returned  
Then `matchedPatterns` MUST be ordered outermost-to-innermost.

Matching precedence inherited from kit matcher/router MUST also hold:

- static segment matches MUST beat param matches at equivalent depth,
- ordering MUST be deterministic across runs (no map-iteration nondeterminism),
- splat matching MUST preserve decoded path-segment semantics.

### BR-LOAD-003: Params and Splat Exposure

Given a matching dynamic/splat route  
When JSON route-data is returned  
Then resolved params and splat values MUST be present in payload.

### BR-LOAD-004: Root Data Flag

Given the root route has a server loader handler  
When that route matches  
Then `hasRootData` MUST be true in route-data payload.

Given root route matches but no root server loader handler is present/runnable  
When route-data is returned  
Then `hasRootData` MUST be false.

### BR-LOAD-016: Trailing-Slash and Catch-All Eligibility Semantics

Given loader routes include exact, dynamic, and non-root catch-all siblings at
the same depth  
When nested loader matching resolves a request path  
Then matcher-interop eligibility MUST hold:

- dynamic segments MUST NOT match an empty trailing segment (for example,
  `/users/:id` MUST NOT match `/users/`),
- non-root catch-all patterns (for example `/users/*`) MUST NOT match `/users`,
  but MUST match `/users/` with empty remainder represented as `[""]`,
- with explicit index segment configuration, explicit index routes (for example
  `/users/_index`) MUST match both `/users` and `/users/`,
- exact/static matches at equivalent depth MUST beat dynamic/catch-all
  candidates,
- root-path precedence between root, root catch-all, and explicit root index
  patterns (when configured) MUST remain deterministic and matcher-compatible.

### BR-LOAD-017: Full-Match Validity with Gap-Tolerant Parent/Leaf Stacks

Given nested routes include a parent route and a deeper leaf route while one or
more intermediate route depths are unregistered  
When request path exactly reaches the deeper registered leaf  
Then `matchedPatterns` MAY include both parent and leaf (gap-tolerant stack).

Given request path ends at an unregistered intermediate depth  
When nested loader matching is evaluated  
Then route resolution MUST fail as not found (no non-consuming deeper partial
match).

## 4.5 Loaders: Execution and Data Assembly

### BR-LOAD-005: Parallel Loader Execution

Given multiple matched loader handlers that block for controlled durations  
When request is processed  
Then total execution time SHOULD reflect parallel execution (bounded near max
duration, not sum).

### BR-LOAD-006: Loader Data Index Alignment

Given N matched patterns  
When route-data is returned  
Then `loadersData`, `importURLs`, `exportKeys`, `errorExportKeys` MUST align by
index to `matchedPatterns`.

Placeholder refinement:

- if matched route has no resolvable client module source, corresponding
  `importURLs`/`exportKeys`/`errorExportKeys` slots MUST remain present as empty
  placeholders to preserve positional alignment.

### BR-LOAD-007: Client-Only Route Entries

Given a matched route exists without a server loader  
When route-data is returned  
Then loader slot MUST remain index-aligned with empty/no-server placeholders
rather than removing the entry.

### BR-LOAD-008: Server-Only Route Placeholder Injection

Given a server loader route is registered in nested router but absent from
client paths metadata  
When route registry sync/reload occurs  
Then runtime MUST inject a placeholder route-path entry so server-only route
remains resolvable in runtime route-data assembly.

Placeholder contract:

- `originalPattern` MUST match server route pattern,
- `srcPath` MUST be empty,
- `exportKey` MUST default to `"default"`,
- `errorExportKey` MUST be empty.

### BR-LOAD-009: Loader Handler Pattern Registration Bootstrap

Given loaders handler is constructed with nested router lacking one or more
route patterns present in runtime paths metadata  
When handler bootstrap runs  
Then runtime MUST register those missing patterns (without requiring task
handlers) so route matching can include client-defined patterns.

### BR-LOAD-020: `RegisterPatternIfNeeded` Idempotent Registration Contract

Given runtime API `RegisterPatternIfNeeded(pattern)` is called  
When pattern is not currently registered in nested loader router  
Then runtime MUST register it as a no-handler nested pattern.

Given same API is called for an already-registered pattern  
When call executes  
Then runtime MUST no-op and MUST NOT surface duplicate-registration failure.

### BR-LOAD-021: Nested-Router Rebuild Must Preserve Handler Routes

Given route-registry sync refreshes client pattern input while nested loader
router already contains a mix of handler-backed and no-handler patterns  
When nested-router rebuild path executes  
Then runtime MUST preserve handler-backed patterns, MUST replace handler-less
patterns with exactly the refreshed pattern set, and MUST keep matcher-option
semantics stable across rebuild (`DynamicParamPrefixRune`,
`SplatSegmentRune`, `ExplicitIndexSegment`).

Conformance-visible invariants:

- handler-backed patterns remain runnable after rebuild even if omitted from the
  refreshed client-only pattern set,
- no-handler patterns omitted from refreshed set no longer match post-rebuild,
- new refreshed patterns become match-eligible no-handler entries.

### BR-LOAD-023: Concurrent `RegisterPatternIfNeeded` Idempotency Contract

Given two or more concurrent callers invoke
`RegisterPatternIfNeeded(pattern)` for the same currently-unregistered pattern  
When registration race occurs  
Then runtime MUST converge to one registered pattern and MUST NOT panic or
surface duplicate-registration failure.

Given the same API is called concurrently for a pattern already registered
with a task handler  
When calls execute  
Then API MUST remain no-op and MUST preserve existing handler-backed
registration (it MUST NOT replace or downgrade handler state).

### BR-LOAD-010: Dependency List Ordering and Deduplication

Given route-data dependencies are computed  
When response payload is assembled  
Then `deps` MUST:

- start with client-entry dependency list in declared order,
- append matched-route dependency lists in matched-pattern order,
- preserve first occurrence and deduplicate later duplicates.

### BR-LOAD-011: CSS Bundle Derivation Ordering and Deduplication

Given route-data CSS bundles are computed from dependency->CSS map  
When response payload is assembled  
Then `cssBundles` MUST:

- include bundles mapped from client entry output first,
- then include bundles mapped from `deps` in dependency order,
- preserve first occurrence and deduplicate later duplicates.

### BR-LOAD-012: Nil Paths Snapshot Safety During Route Sync

Given route-registry sync receives a nil paths map (for example artifact decode
produces `null` paths map)  
When sync executes  
Then runtime MUST treat nil as empty map and continue route sync without panic.

Conformance-visible effect:

- subsequent loader requests MUST return normal not-found/empty-routing behavior
  instead of crashing due to nil map dereference.

### BR-LOAD-013: Route-Sync Cache Invalidation

Given route registry sync updates runtime paths metadata  
When subsequent loader requests reuse the same normalized matched-pattern set  
Then route-data snapshot fields derived from paths (`importURLs`, `exportKeys`,
`errorExportKeys`, `deps`) MUST reflect post-sync metadata and MUST NOT reuse
stale pre-sync cached values.

### BR-LOAD-014: Route-Data Cache Key Must Be Tuple-Unambiguous

Given route-data snapshot caching is enabled  
When cache keys are derived from ordered matched route patterns  
Then keying strategy MUST uniquely distinguish different ordered pattern tuples
and MUST NOT permit collisions caused by ambiguous concatenation boundaries.

Conformance-visible invariant:

- for two requests with different ordered matched-pattern tuples, cached
  snapshot metadata (`importURLs`, `exportKeys`, `errorExportKeys`, `deps`) from
  one tuple MUST NOT leak into the other tuple.

### BR-LOAD-015: Route-Data Cache Scope Must Be App-Instance Isolated

Given multiple Vorma app instances exist in the same process  
When they serve loader requests with overlapping matched-pattern shapes  
Then route-data snapshot cache entries MUST be isolated per app instance (and
its current route metadata state), so one app's cached metadata MUST NOT affect
another app's responses.

### BR-LOAD-018: Nil-Loader-Payload Warning Diagnostics

Given a matched loader runs successfully (no error) but returns nil-like data
that resolves to nil pointer/value semantics (excluding allowed empty-struct
cases)  
When route-data assembly inspects loader outputs  
Then runtime SHOULD emit warning diagnostics indicating nil loader return is
non-ideal and include the matched route pattern context.

### BR-LOAD-019: Mode-Dependent Import URL Source Selection

Given matched route metadata includes both `srcPath` and `outPath` for a route  
When route-data `importURLs` is assembled  
Then import URL source selection MUST be mode-dependent:

- in dev mode, each matched route import URL MUST derive from `srcPath`,
- in non-dev mode, each matched route import URL MUST derive from `outPath`.

Both modes MUST preserve per-match positional alignment and prefix emitted URLs
with leading `/`.

### BR-LOAD-022: Mode-Dependent `viteDevURL` Emission Contract

Given route-data assets are assembled in dev mode  
When route-data payload (`JSON`) or SSR bootstrap payload (`HTML`) is emitted  
Then `viteDevURL` MUST be emitted as `http://localhost:<vite-port>` where
`<vite-port>` is the currently selected Vite dev-server port for that process.

Given route-data assets are assembled in non-dev mode  
When payload is emitted  
Then `viteDevURL` MAY be omitted (`omitempty`) or, if present, MUST be emitted
as an empty string.

## 4.6 Loader Error Behavior

### BR-ERR-001: Outermost Error Cutoff

Given multiple matched loaders where errors occur  
When route-data is returned  
Then payload MUST be truncated at:

- the first non-cancellation loader error index, or
- if all loader errors are cancellation-only (`context.Canceled` /
  `context.DeadlineExceeded`), the first cancellation error index.

Cancellation-only detection MUST unwrap typed `LoaderError` wrappers and inspect
their server-side error value.

### BR-ERR-002: Typed LoaderError Client Message

Given a loader returns `LoaderError{Client, Server}`  
When request is handled  
Then payload MUST expose `Client` message to client-facing error fields.

### BR-ERR-003: Typed LoaderError Server Logging

Given a loader returns `LoaderError{Client, Server}`  
When request is handled  
Then server-side logged error MUST use `Server` error.

### BR-ERR-004: Generic Loader Error Message

Given a loader returns a non-`LoaderError` error  
When request is handled  
Then client-facing message MUST be generic and MUST NOT leak raw server error
text by default.

### BR-ERR-005: Error-Cutoff Truncation Scope and Dependency Retention

Given loader error cutoff index `i` is selected  
When route-data payload is assembled  
Then route-index-aligned fields MUST be truncated to `i+1`:

- `matchedPatterns`
- `loadersData`
- `importURLs`
- `exportKeys`
- `errorExportKeys`

Given same error-cutoff payload  
When dependency metadata is emitted  
Then `deps` MUST remain the full dependency snapshot for the matched tuple and
MUST NOT be truncated by loader cutoff.

Given same error-cutoff payload  
When loader-contributed head elements are aggregated  
Then only loader head contributions from indices strictly less than `i` MUST be
retained (errored loader index `i` and deeper indices MUST NOT contribute head
elements to the terminal error payload).

## 4.7 Redirect and Proxy Propagation

### BR-PROXY-001: Loader Proxy Merge Controls Final Response

Given matched loaders emit response proxy side effects  
When request is handled  
Then merged proxy output MUST influence final HTTP response before payload
rendering.

Merge semantics MUST preserve kit-response precedence rules:

- first error status wins,
- otherwise last success status wins,
- first redirect wins only when merged status is non-error,
- header op order (set/add) MUST be preserved.

### BR-PROXY-002: Redirect Short-Circuit

Given merged proxy indicates redirect  
When loaders request is handled  
Then runtime MUST short-circuit and MUST NOT emit normal JSON/HTML route-data
payload.

### BR-PROXY-003: Error Short-Circuit

Given merged proxy indicates error status  
When loaders request is handled  
Then runtime MUST short-circuit and MUST NOT emit normal JSON/HTML route-data
payload.

### BR-PROXY-004: Cookie Name-Collision Merge Contract

Given multiple matched loaders set response cookies and at least two cookies
share the same name  
When merged proxy output is applied  
Then final response MUST keep one winning cookie per cookie name, using value
from later proxy merge position for that name.

Distinct cookie names MUST remain present.

### BR-PROXY-005: Client Redirect Header Single-Value Winner Contract

Given merged non-error proxy output includes one or more client redirects  
When response is emitted  
Then `X-Client-Redirect` MUST be single-valued and MUST equal winning redirect
target from first redirect in merge order.

### BR-PROXY-006: Non-Redirect Success Status Preservation with Normal Payload

Given merged proxy output produces a non-error, non-redirect success status (for
example `201`)  
When loaders flow continues through normal JSON/HTML route-data rendering path  
Then final response MUST preserve that success status while still emitting
normal route payload for the request mode.

## 4.8 HTML Response Contract

### BR-HTML-001: HTML Mode Returns HTML

Given a non-JSON loaders request with successful route resolution  
When request is handled  
Then response MUST be HTML.

### BR-HTML-002: Required Template Data Keys

Given successful HTML rendering path  
When root template executes  
Then template data MUST include:

- `VormaHeadEls`
- `VormaSSRScript`
- `VormaSSRScriptSha256Hash`
- `VormaRootID`
- `VormaBodyScripts`.

Given root-template-data callback returns error  
When HTML render path executes  
Then runtime MUST return HTTP 500 and MUST NOT emit partial rendered HTML body.

### BR-HTML-003: Production Body Script

Given prod mode and successful HTML rendering  
When response is generated  
Then `VormaBodyScripts` MUST include module script pointing to client entry
output under public path prefix.

### BR-HTML-004: Development Body Scripts

Given dev mode and successful HTML rendering  
When response is generated  
Then `VormaBodyScripts` MUST include Vite dev scripts and Wave refresh script.

### BR-HTML-005: Default Cache-Control

Given loader response does not already set `Cache-Control`  
When response is generated  
Then runtime MUST set
`Cache-Control: private, max-age=0, must-revalidate, no-cache`.

Given response already has `Cache-Control` (for example via loader proxy)  
When response is generated  
Then runtime MUST preserve existing value and MUST NOT override it.

### BR-HTML-006: Development Script Variant Selection

Given dev mode and successful HTML rendering  
When Vite dev scripts are generated  
Then script generation variant MUST follow configured `UIVariant` (`react`
variant for React UI, non-react variant for other UI adapters).

### BR-HTML-007: SSR Bootstrap Script Integrity

Given successful HTML rendering path  
When template data is produced  
Then `VormaSSRScript` MUST be a module script element and
`VormaSSRScriptSha256Hash` MUST equal the CSP-style SHA-256 hash of that exact
rendered script element content.

### BR-HTML-008: Root Template Data Nil-Map Safety

Given `GetRootTemplateData` callback returns `(nil, nil)`  
When HTML render path enriches template data with required Vorma keys  
Then runtime MUST treat nil map as empty map and continue rendering (no panic).

### BR-HTML-009: Root Template Data Callback Error Mapping

Given `GetRootTemplateData` callback returns a non-nil error  
When HTML render path executes  
Then runtime MUST return HTTP 500 and MUST NOT execute root-template rendering
for that request.

### BR-HTML-010: Reserved Root-Template Key Precedence

Given `GetRootTemplateData` returns keys colliding with runtime-reserved Vorma
template keys (`VormaHeadEls`, `VormaSSRScript`, `VormaSSRScriptSha256Hash`,
`VormaRootID`, `VormaBodyScripts`)  
When HTML render path enriches template data  
Then runtime-computed reserved-key values MUST overwrite callback-provided
colliding values before template execution.

### BR-HTML-011: Root Element ID Value Contract

Given successful HTML render path  
When runtime populates reserved root-template keys  
Then `VormaRootID` MUST equal `vorma-root`.

## 4.9 Actions Contract

### BR-ACT-001: GET Input Parsing

Given an action route with typed input and method GET  
When query params are supplied  
Then action input MUST be parsed from URL search params.

### BR-ACT-002: Non-GET JSON Parsing

Given an action route with typed input and method in supported set (non-GET)  
When content type is JSON and body is valid  
Then action input MUST be parsed from JSON body.

Default supported-method baseline (when not explicitly configured) MUST include:
`GET`, `POST`, `PUT`, `DELETE`, `PATCH`.

### BR-ACT-003: Form Content-Type Pass-Through

Given an action route with non-GET form content type
(`application/x-www-form-urlencoded` or `multipart/form-data`)  
When request is handled  
Then default parser MUST not JSON-decode body automatically.

Default parse-outcome refinement:

- under default action parser flow (no custom parse override), typed action input
  value observed by handler MUST remain the target type's zero-value unless
  explicitly populated by user-defined parsing/middleware behavior.

### BR-ACT-004: Validation Error Mapping

Given action input parsing/validation fails with validation-class error  
When request is handled  
Then response MUST be HTTP 400.

### BR-ACT-005: Supported-Methods Accessor Immutability

Given caller retrieves action supported-methods via
`Actions().SupportedMethods()`  
When caller mutates returned map  
Then runtime-supported method behavior and mount semantics MUST remain based on
internal authoritative config and MUST NOT be altered by caller-side map
mutation.

### BR-ACT-006: HEAD Fallback Parsing Parity for GET Actions

Given a typed GET action route is resolved via router `HEAD` fallback  
When default action input parsing executes for that request  
Then parsing semantics MUST remain equivalent to GET query parsing and MUST NOT
fail with unsupported-method parse errors solely because transport method is
`HEAD`.

## 4.10 Head Elements Contract

### BR-HEAD-001: Default + Loader Head Aggregation

Given default head elements and loader-contributed head elements exist  
When route renders  
Then final head output MUST include both sources after dedupe/sort.

Ordering and cutoff:

- default head elements MUST be ordered before loader-contributed head elements,
- when loader error cutoff index `i` applies, loader-contributed head from index
  `i` and deeper MUST be excluded.

Given default-head callback fails  
When route-data assembly runs on a non-terminal render path  
Then runtime MUST return HTTP 500 and MUST abort normal payload rendering.

Non-terminal render path means request processing has not already reached a
terminal loader outcome (`not found`, merged-proxy redirect, or merged-proxy
error short-circuit).

### BR-HEAD-002: Wave Style Integration

Given HTML render path  
When head is rendered  
Then Wave critical CSS/style-sheet elements MUST be included.

Production HTML-only augmentation:

- non-JSON prod responses MUST append dependency `modulepreload` links and CSS
  bundle stylesheet links with public-path-prefixed hrefs,
- CSS bundle links MUST carry `data-vorma-css-bundle=<bundle>`.

### BR-HEAD-003: Production Head Augmentation Ordering

Given non-JSON production head augmentation runs  
When head output is assembled  
Then ordering MUST be:

- default + loader head elements first (after dedupe/sort semantics),
- then `modulepreload` links in `deps` order,
- then CSS bundle stylesheet links in `cssBundles` order.

### BR-HEAD-004: Terminal Loader Outcomes Must Not Be Masked by Default-Head Failure

Given loader processing has already reached a terminal non-render outcome
(`not found`, merged-proxy redirect, or merged-proxy error)  
When default-head callback fails concurrently in the same request lifecycle  
Then terminal outcome semantics MUST be preserved and runtime MUST NOT
retroactively convert the response into HTTP 500.

## 4.11 Development Reload Endpoints

### BR-DEV-001: Route Reload Endpoint

Given dev mode and valid stage-1 artifacts on disk  
When `GET /__vorma/reload-routes` is called  
Then runtime MUST reload route/build metadata from disk and respond with success
(`200` + `ok` body).

Route-set replacement refinement:

- route metadata after reload MUST be derived from the reloaded artifact as a
  replacement snapshot (not additive merge with pre-reload client route set),
- client routes removed from artifact MUST stop participating in route matching
  after reload,
- server-only task routes MAY remain resolvable via documented placeholder
  injection semantics (`BR-LOAD-008`).

### BR-DEV-002: Template Reload Endpoint

Given dev mode and valid template file on disk  
When `GET /__vorma/reload-template` is called  
Then runtime MUST reload template and respond with success (`200` + `ok` body).

### BR-DEV-003: Reload Endpoint Failure

Given dev mode and reload operation fails  
When reload endpoint is called  
Then runtime MUST return HTTP 500 with error text.

### BR-DEV-004: Reload Endpoints Must Be Dev-Only

Given runtime is not in dev mode  
When request path is `/__vorma/reload-routes` or `/__vorma/reload-template`  
Then runtime MUST NOT execute reload operations and MUST follow normal
loader-route handling semantics for those paths.

### BR-DEV-005: Route Reload Must Invalidate Route-Data Caches

Given dev route reload succeeds after route metadata changed on disk  
When subsequent loader requests are served  
Then route-data fields derived from route metadata (including `importURLs`,
`exportKeys`, `errorExportKeys`, `deps`, and `cssBundles`) MUST reflect reloaded
artifact state and MUST NOT remain stale from pre-reload cache entries.

### BR-DEV-006: Reload Endpoint Dispatch Is Path-Based (Method-Agnostic)

Given runtime is in dev mode  
When request path is `/__vorma/reload-routes` or `/__vorma/reload-template`
under any HTTP method  
Then runtime MUST execute corresponding reload operation based on path match
alone (method does not gate reload dispatch).

### BR-DEV-007: Route Reload Refreshes Build and Manifest Metadata

Given dev route reload succeeds  
When subsequent loader/JSON responses are served  
Then runtime MUST use build/manifest metadata loaded from stage-one artifact,
including updated `buildID` header and updated route-manifest file reference
used for SSR bootstrap.

### BR-DEV-008: Route Reload Preserves Existing Task Handler Bindings

Given dev mode with loader task handlers already registered on nested routes  
When route reload succeeds and resulting route set still includes those patterns  
Then subsequent requests to surviving patterns MUST continue to execute the same
task handlers (handler bindings MUST be preserved across route-tree rebuild).

### BR-DEV-009: Reload Failure Must Preserve Prior Runtime Snapshot

Given dev mode and a reload operation fails (for example invalid stage artifact
or template parse failure)  
When `/__vorma/reload-routes` or `/__vorma/reload-template` returns failure  
Then previously active route/template runtime state MUST remain in effect (failed
reload MUST be non-mutating for authoritative in-memory snapshot state).

Failure-preservation scope:

- failed route reload MUST NOT partially overwrite active `_paths`, `buildID`,
  route-manifest reference, or live route-registry state,
- failed template reload MUST NOT replace the currently active parsed root
  template.

### BR-DEV-010: Direct Route Reload API Uses Stage-One Artifacts

Given `ReloadRoutesFromDisk()` is invoked directly  
When route artifact loading succeeds  
Then runtime MUST:

- load stage-one paths artifact semantics (same source used by dev route reload),
- replace in-memory route/build/manifest snapshot with loaded artifact values,
- synchronize route registry from that snapshot before returning success.

Given stage-one artifact loading fails  
When `ReloadRoutesFromDisk()` returns  
Then method MUST return non-nil error and MUST leave previously active
authoritative runtime snapshot unchanged.

### BR-DEV-011: Direct Template Reload API Is Parse-Then-Swap

Given `ReloadTemplateFromDisk()` is invoked directly  
When template parsing fails  
Then method MUST return non-nil error and MUST NOT mutate active root template.

Given template parsing succeeds  
When method returns success  
Then active root template snapshot MUST be replaced atomically for subsequent
requests.

## 4.12 Wave Delegation Surface

### BR-STATIC-001: `ServeStatic` Delegates to Wave Static Middleware

Given a Vorma app instance with embedded `*wave.Wave`  
When `Vorma.ServeStatic()` is called  
Then runtime MUST return the middleware produced by
`Wave.ServeStatic(true)` without introducing additional Vorma-owned static
serving semantics.

Behavior ownership note:

- static-serving behavior details (asset predicate, cache header policy, prefix
  semantics, and static file lookup semantics) are owned by Wave requirement IDs
  `WAVE-RT-010` and `WAVE-RT-011` in `specs/packages/wave/SPEC.md`.

## 4.13 Concurrency and Thread-Safety (Black-Box)

### BR-CONC-001: Concurrent Request Safety

Given concurrent loader/action traffic under race detector or equivalent
stress  
When requests are handled  
Then runtime MUST behave correctly without data races or corrupted response
payloads.

### BR-CONC-002: Route Reload Safety

Given concurrent traffic and dev route reload operations  
When reloads occur  
Then runtime MUST maintain valid routing behavior (no crash, no malformed
route-data alignment).

### BR-CONC-003: Snapshot Accessors Must Not Expose Mutable Internal State

Given caller obtains runtime metadata via snapshot-style accessor APIs (for
example `GetPathsSnapshot`, `GetClientEntryDeps`, `GetDepToCSSBundleMap`,
`GetAdHocTypes()`, `LoadersRouter().AllRoutes()`, `ActionsRouter().AllRoutes()`)  
When caller mutates returned value (including collections returned through
router accessors)  
Then runtime-internal state used for request handling MUST remain unaffected and
thread-safe (no external mutable alias into lock-protected internals).

### BR-CONC-004: `WithRLock` Must Enforce Read-Only Access

Given caller invokes `WithRLock` and receives `LockedVorma` callback access  
When callback execution occurs  
Then lock-protected runtime state MUST be read-only for that callback scope.

Mutation operations over lock-protected runtime fields (for example
`SetPaths`, `SetBuildID`, `SetRouteManifestFile`, `SetRootTemplate`) MUST NOT
be permitted under `WithRLock` (for example unavailable in API surface or
fail-fast at runtime).

Given caller invokes `WithLock` and mutates lock-protected fields inside the
callback  
When callback returns  
Then those mutations MUST be committed atomically before `WithLock` returns.

## 5. Executable Conformance Scenario Catalog

This section defines concrete black-box scenarios that SHOULD be used as the
default test vectors for requirements above.

Scenario IDs are stable references for test planning and CI reporting.

## 5.1 Construction and Init Scenarios

### BRC-INIT-001 (covers BR-INIT-001)

Given `NewVormaApp` is called with `Wave=nil`  
When the constructor is invoked  
Then it MUST panic with an initialization error.

### BRC-INIT-002 (covers BR-INIT-002)

Given a table-driven set of config payloads where each case omits one required
field (`MainBuildEntry`, `UIVariant`, `HTMLTemplateLocation`, `ClientEntry`,
`ClientRouteDefsFile`, `TSGenOutDir`)  
When `NewVormaApp` is invoked for each case  
Then each case MUST fail immediately (panic).

### BRC-INIT-003 (covers BR-INIT-003)

Given minimal valid config without optional values  
When app construction completes  
Then defaults MUST be externally observable:

- `BuildtimePublicURLFuncName` resolves to `waveBuildtimeURL`.
- loaders explicit index behavior uses `_index` semantics.
- actions mount root is `/api/`.
- actions supported methods include `GET`, `POST`, `PUT`, `DELETE`, `PATCH`.

### BRC-INIT-004 (covers BR-INIT-004)

Given runtime artifacts are missing (for active dev/prod stage)  
When `Init()` is called  
Then init MUST panic and app MUST NOT continue serving.

### BRC-INIT-005 (covers BR-INIT-005)

Given successful app initialization with at least one loader and one action  
When `InitWithDefaultRouter()` is used and requests are sent  
Then:

- `GET` loader path MUST route via loaders handler,
- action methods MUST route under actions mount root.

### BRC-INIT-006 (covers BR-INIT-006)

Given fixture with distinct stage-one and stage-two paths artifacts  
When runtime initializes in dev vs non-dev mode  
Then dev init MUST load stage-one values and non-dev init MUST load stage-two
values.

### BRC-INIT-007 (covers BR-INIT-007)

Given one app with custom `GetHeadDedupeKeys` and one app with default dedupe
setup  
When both render equivalent head contributions with duplicate candidates  
Then custom app MUST reflect custom dedupe behavior and default app MUST retain
default dedupe behavior.

### BRC-INIT-008 (covers BR-INIT-008)

Given app is constructed without explicit optional callbacks
(`GetDefaultHeadEls`, `GetHeadDedupeKeys`, `GetRootTemplateData`)  
When init and HTML route handling execute  
Then lifecycle MUST complete using safe callback defaults without callback-nil
crash and without implicit non-empty callback side effects.

### BRC-INIT-009 (covers BR-INIT-009)

Given initialized app instance  
When `GetLoadersHandler(nil)` is invoked  
Then invocation MUST panic with nil-router guard failure.

### BRC-INIT-010 (covers BR-INIT-010)

Given app init succeeds under known port `P`  
When `ServerAddr()` is read  
Then value MUST equal `":" + P`.

### BRC-INIT-011 (covers BR-INIT-011)

Given constructor receives non-empty `AdHocTypes` and `ExtraTSCode` payloads  
When those getters are read before/after init and after dev reload operations  
Then returned payloads MUST remain constructor-equivalent.

### BRC-INIT-012 (covers BR-INIT-012)

Given app construction uses loaders-router options with explicit index segment
token containing `/` (for example `"bad/idx"`)  
When constructor/init path is invoked  
Then construction MUST fail fast with panic-class invalid matcher configuration
error.

### BRC-INIT-013 (covers BR-INIT-013)

Given direct helper calls to `VormaPaths.StageOneJSON()` and
`VormaPaths.StageTwoJSON()`  
When helper output strings are observed  
Then outputs MUST exactly match canonical joined artifact paths under
`vorma_out` for stage-one and stage-two JSON files.

### BRC-INIT-014 (covers BR-INIT-014)

Given an initialized runtime instance  
When `SetIsDev(true)` is called and then `GetIsDevMode()` is read  
Then getter MUST return `true`.

Given same instance after `SetIsDev(false)`  
When `GetIsDevMode()` is read  
Then getter MUST return `false`.

## 5.2 Common Response Header Scenarios

### BRC-RESP-001 (covers BR-RESP-001)

Given loader requests for both matched and unmatched paths  
When responses are returned  
Then every response MUST include `X-Vorma-Build-Id`.

### BRC-RESP-002 (covers BR-RESP-002)

Given action requests for both matched and unmatched paths  
When responses are returned  
Then every response MUST include `X-Vorma-Build-Id`.

### BRC-RESP-003 (covers BR-RESP-003)

Given loader requests across JSON and HTML flows with and without pre-set
`Cache-Control` values  
When responses are observed  
Then missing values MUST receive default loader cache-control and pre-set values
MUST remain unchanged.

## 5.3 JSON Negotiation Scenarios

### BRC-JSON-001 (covers BR-JSON-001)

Given requests:

- `/page` (no `vorma_json`)
- `/page?vorma_json=` (empty value)
- `/page?vorma_json=abc` (non-empty value)

When loaders handler processes them  
Then only the non-empty `vorma_json` request MUST be treated as JSON mode.

### BRC-JSON-002 (covers BR-JSON-002)

Given current build id is `b123` and request is
`/page?x=1&vorma_json=stale&y=2`  
When handled  
Then response MUST be:

- `200`,
- `Content-Type: application/json`,
- body `{"ok":true}`,
- header `X-Vorma-Reload` set to `/page?x=1&y=2`.

### BRC-JSON-003 (covers BR-JSON-003)

Given a current-build JSON request for a matched route  
When handled  
Then response MUST be JSON and include route-data contract keys for that route
result (`matchedPatterns`, `loadersData`, `importURLs`, `exportKeys`, `params`,
`splatValues`), subject to `omitempty`.

### BRC-JSON-004 (covers BR-JSON-004)

Given current build id `b123` and request path with no matching loader route  
When request `/missing?vorma_json=stale` is handled  
Then response MUST be stale-build sentinel (`200` with `X-Vorma-Reload`) rather
than loader `404`.

## 5.4 Loader Routing and Match Scenarios

### BRC-LOAD-001 (covers BR-LOAD-001)

Given no registered/matched loader for request path  
When request is handled  
Then response MUST be `404`.

### BRC-LOAD-002 (covers BR-LOAD-002)

Given nested matched routes (for example `/`, `/users`, `/users/:id`)  
When requesting `/users/42` in JSON mode  
Then `matchedPatterns` MUST be ordered from outermost to innermost.

### BRC-LOAD-003 (covers BR-LOAD-003)

Given dynamic/splat pattern (for example `/files/*`)  
When requesting `/files/a/b/c`  
Then response payload MUST include expected `params` and `splatValues`.

### BRC-LOAD-004 (covers BR-LOAD-004)

Given one app with root loader handler and one without root loader handler  
When both serve equivalent matching route requests  
Then `hasRootData` MUST be `true` only for app with root loader handler.

### BRC-LOAD-016 (covers BR-LOAD-016)

Given loader-route fixtures including `/users/:id`, `/users/*`, and exact
`/users` variants (including explicit-index fixture `/users/_index` when index
segment is configured)  
When requests `/users`, `/users/`, and `/users/123` are served in JSON mode  
Then matching MUST satisfy:

- `/users/:id` does not match `/users/`,
- `/users/*` does not match `/users` but does match `/users/` with empty splat
  remainder,
- explicit index route `/users/_index` matches both `/users` and `/users/`,
- exact `/users` beats sibling catch-all on exact-equivalent request targets.

Given root fixtures with root, root catch-all, and explicit root-index
registrations  
When request `/` is served  
Then selected root matches MUST follow deterministic matcher precedence (no
ambiguous winner drift across runs).

### BRC-LOAD-017 (covers BR-LOAD-017)

Given registered routes include `/team` and `/team/members/profile` but not
`/team/members`  
When `/team/members/profile` is requested in JSON mode  
Then `matchedPatterns` MUST include both `/team` and
`/team/members/profile`.

When `/team/members` is requested  
Then response MUST be `404` (no deeper-route partial acceptance).

## 5.5 Loader Data Assembly Scenarios

### BRC-LOAD-005 (covers BR-LOAD-005)

Given two matched loaders with controlled sleeps (`50ms`, `50ms`)  
When JSON request is executed repeatedly  
Then elapsed time SHOULD be near one sleep interval (parallel) rather than sum.

### BRC-LOAD-006 (covers BR-LOAD-006)

Given a request with `N` matched patterns  
When JSON payload is returned  
Then lengths of `matchedPatterns`, `loadersData`, `importURLs`, `exportKeys`,
and `errorExportKeys` MUST be index-aligned.

### BRC-LOAD-007 (covers BR-LOAD-007)

Given a matched client route with no server loader task  
When JSON payload is returned  
Then slot MUST remain present and index-aligned (no array compaction).

### BRC-LOAD-008 (covers BR-LOAD-008)

Given a route pattern with server loader task but no client-path metadata
entry  
When route registry sync/reload runs and matching JSON request executes  
Then response MUST keep route resolvable with server-only placeholder semantics
(`srcPath`/import slot empty, default export key behavior preserved).

### BRC-LOAD-009 (covers BR-LOAD-009)

Given runtime paths metadata contains client route pattern that is not yet
registered in provided nested router  
When loaders handler is constructed and matching request executes  
Then pattern MUST be auto-registered for matching and request MUST avoid false
not-found solely due to missing pre-registration.

### BRC-LOAD-020 (covers BR-LOAD-020)

Given `RegisterPatternIfNeeded(P)` is called twice for the same pattern `P`  
When second call executes  
Then runtime MUST keep router registration stable and MUST NOT panic/fail due to
duplicate registration attempt.

### BRC-LOAD-021 (covers BR-LOAD-021)

Given nested loader router has a handler-backed route `/persist` and a
no-handler route `/old`, and route-sync refresh input contains only `/new`  
When route-sync rebuild executes and subsequent loader matching runs  
Then `/persist` MUST remain match-eligible with handler execution,
`/new` MUST become match-eligible as a no-handler pattern, and `/old` MUST no
longer match post-rebuild.

### BRC-LOAD-010 (covers BR-LOAD-010)

Given client-entry deps and matched-route deps contain overlapping entries in a
known order  
When JSON route-data is returned  
Then `deps` MUST equal first-seen deduplicated order: client-entry deps first,
then matched-route deps by matched depth.

### BRC-LOAD-011 (covers BR-LOAD-011)

Given dependency->CSS mapping where client-entry and route deps overlap on CSS
bundle names  
When JSON route-data is returned  
Then `cssBundles` MUST be deduplicated with first-seen precedence and preserve
ordering: client-entry mapped bundles first, then bundles mapped from `deps`.

### BRC-LOAD-012 (covers BR-LOAD-012)

Given runtime route sync path input is nil  
When loader handler serves subsequent requests  
Then runtime MUST remain alive and serve normal non-crashing responses
(including expected not-found behavior where routes are absent).

### BRC-LOAD-013 (covers BR-LOAD-013)

Given one route-sync state is served and then route-sync updates path/dependency
metadata for the same matched-pattern set  
When loader JSON route-data is requested again  
Then `importURLs`/`exportKeys`/`errorExportKeys`/`deps` MUST reflect updated
state rather than prior cached values.

### BRC-LOAD-014 (covers BR-LOAD-014)

Given two distinct ordered matched-pattern tuples whose naive string
concatenations can alias  
When loader JSON route-data requests are served for both tuples  
Then each tuple MUST return its own correct metadata snapshot and MUST NOT reuse
cross-tuple cached metadata.

### BRC-LOAD-015 (covers BR-LOAD-015)

Given two separate Vorma app instances with different route metadata for the
same matched-pattern tuple  
When each app serves loader JSON route-data  
Then each response MUST reflect that app's own metadata snapshot, with no
cross-instance cache contamination.

### BRC-LOAD-018 (covers BR-LOAD-018)

Given a matched loader returns a nil-like payload without returning an error  
When loader JSON response path executes  
Then runtime SHOULD emit a warning diagnostic that includes nil-loader guidance
and matched route pattern context.

### BRC-LOAD-019 (covers BR-LOAD-019)

Given equivalent matched route metadata served once in dev mode and once in
non-dev mode  
When loader JSON route-data is inspected  
Then `importURLs` entries MUST use `srcPath`-derived values in dev and
`outPath`-derived values in non-dev, while preserving index alignment and
leading-`/` URL formatting.

### BRC-LOAD-022 (covers BR-LOAD-022)

Given one request served in dev mode and one request served in non-dev mode  
When route-data payloads are inspected (`JSON` response and HTML bootstrap
payload)  
Then dev-mode payloads MUST emit `viteDevURL` using
`http://localhost:<vite-port>` form, and non-dev payloads MUST either omit
`viteDevURL` (`omitempty`) or emit it as empty string.

### BRC-LOAD-023 (covers BR-LOAD-023)

Given a fresh app/router where pattern `P` is initially unregistered  
When multiple concurrent goroutines call `RegisterPatternIfNeeded(P)`  
Then operation MUST complete without panic and exactly one stable registration
for `P` MUST be present.

Given an already handler-backed pattern `H`  
When concurrent `RegisterPatternIfNeeded(H)` calls execute  
Then handler-backed route behavior for `H` MUST remain intact and non-downgraded
after the concurrent call burst.

## 5.6 Loader Error Scenarios

### BRC-ERR-001 (covers BR-ERR-001)

Given three matched loaders where second loader returns error  
When JSON route-data is returned  
Then route-index-aligned arrays (`matchedPatterns`, `loadersData`, `importURLs`,
`exportKeys`, `errorExportKeys`) MUST be truncated to `idx+1` (include errored
route, drop deeper routes).

### BRC-ERR-002 (covers BR-ERR-002)

Given loader returns `LoaderError{Client: "safe msg", Server: err}`  
When JSON payload is returned  
Then `outermostServerError` MUST be `"safe msg"`.

### BRC-ERR-003 (covers BR-ERR-003, BR-ERR-004)

Given one case with `LoaderError` and one with plain error  
When errors are triggered  
Then:

- `LoaderError` case MUST log server-side error details from `Server`,
- plain error case MUST expose generic client message (`An error occurred`).

### BRC-ERR-004 (covers BR-ERR-001)

Given an outer loader fails with cancellation-only error and a deeper loader
fails with a non-cancellation error  
When JSON route-data is returned  
Then cutoff index MUST be the non-cancellation error index (not the earlier
cancellation-only index).

### BRC-ERR-005 (covers BR-ERR-005)

Given three matched loaders where second loader fails and all three matched
routes contribute dependency graph entries  
When JSON route-data is returned  
Then `matchedPatterns`/`loadersData`/`importURLs`/`exportKeys`/`errorExportKeys`
MUST be truncated to `idx+1`, while `deps` MUST remain full matched-tuple
dependency snapshot order.

## 5.7 Proxy and Redirect Scenarios

### BRC-PROXY-001 (covers BR-PROXY-001)

Given multiple matched loaders each mutating response proxy headers/cookies  
When request completes without proxy error/redirect  
Then merged proxy mutations MUST appear in final response.

### BRC-PROXY-002 (covers BR-PROXY-002)

Given a matched loader sets redirect via response proxy  
When request is handled  
Then runtime MUST return redirect response and MUST NOT append normal route-data
JSON/HTML body.

### BRC-PROXY-003 (covers BR-PROXY-003)

Given a matched loader sets error status via response proxy  
When request is handled  
Then runtime MUST return error response and MUST NOT append normal route-data
JSON/HTML body.

### BRC-PROXY-004 (covers BR-PROXY-004)

Given multiple matched loaders set cookies with colliding names and distinct
names  
When response is emitted  
Then wire-visible cookie set MUST include distinct names and single winning
value per colliding name using later-proxy precedence.

### BRC-PROXY-005 (covers BR-PROXY-005)

Given multiple matched loaders attempt client redirects in non-error flow  
When response is emitted  
Then `X-Client-Redirect` MUST appear as one winning value (no multi-value
append).

### BRC-PROXY-006 (covers BR-PROXY-006)

Given matched loaders set merged proxy success status `201` without
redirect/error short-circuit  
When request completes through normal route-data path  
Then response status MUST be `201` and body MUST still contain normal route-data
payload for the request mode.

## 5.8 HTML Rendering Scenarios

### BRC-HTML-001 (covers BR-HTML-001)

Given non-JSON matched loader request  
When response is generated  
Then `Content-Type` MUST be `text/html`.

### BRC-HTML-002 (covers BR-HTML-002)

Given root template that renders probe markers for required keys  
When HTML response is generated  
Then probe output MUST confirm availability of:

- `VormaHeadEls`,
- `VormaSSRScript`,
- `VormaSSRScriptSha256Hash`,
- `VormaRootID`,
- `VormaBodyScripts`.

### BRC-HTML-003 (covers BR-HTML-003)

Given production mode  
When HTML response is generated  
Then `VormaBodyScripts` MUST include module script URL that begins with public
path prefix and targets built client entry output.

### BRC-HTML-004 (covers BR-HTML-004)

Given development mode  
When HTML response is generated  
Then `VormaBodyScripts` MUST include Vite dev script(s) and Wave refresh script.

### BRC-HTML-005 (covers BR-HTML-005)

Given no proxy/header override for cache control  
When loader response is generated  
Then `Cache-Control` MUST equal `private, max-age=0, must-revalidate, no-cache`.

### BRC-HTML-006 (covers BR-HTML-005)

Given loader/proxy explicitly sets `Cache-Control: public, max-age=60`  
When response is generated  
Then runtime MUST preserve that value (must not overwrite with default).

### BRC-HTML-007 (covers BR-HTML-006)

Given two dev-mode apps with different `UIVariant` values (React vs non-React)  
When HTML responses are generated  
Then Vite dev-script output MUST follow the corresponding variant strategy for
each app and both MUST include Wave refresh script.

### BRC-HTML-008 (covers BR-HTML-007)

Given successful HTML rendering  
When `VormaSSRScript` and `VormaSSRScriptSha256Hash` are observed in template
data output  
Then script element MUST be module-typed and hash value MUST match recomputed
SHA-256 over the emitted script element content.

### BRC-HTML-009 (covers BR-HTML-008)

Given app callback `GetRootTemplateData` returns `(nil, nil)`  
When document-mode HTML response is generated  
Then request MUST complete without panic and required Vorma template keys MUST
still be present in rendered output.

### BRC-HTML-010 (covers BR-HTML-009)

Given app callback `GetRootTemplateData` returns a non-nil error  
When document-mode HTML response is requested  
Then response MUST be HTTP 500 and MUST not include successful template-rendered
document payload.

### BRC-HTML-011 (covers BR-HTML-010)

Given app callback `GetRootTemplateData` returns colliding values for reserved
Vorma template keys  
When HTML response is rendered  
Then rendered output MUST reflect runtime-computed reserved-key values, not the
callback-provided colliding values.

### BRC-HTML-012 (covers BR-HTML-011)

Given successful HTML response with root-template probe output for `VormaRootID`  
When rendered document is inspected  
Then `VormaRootID` probe value MUST equal `vorma-root`.

## 5.9 Action Route Scenarios

### BRC-ACT-001 (covers BR-ACT-001)

Given GET action with typed input  
When query params are supplied  
Then handler input MUST reflect parsed query values.

### BRC-ACT-002 (covers BR-ACT-002)

Given POST/PUT/PATCH/DELETE action with JSON body  
When valid JSON payload is supplied  
Then handler input MUST reflect parsed JSON values.

### BRC-ACT-003 (covers BR-ACT-003)

Given non-GET action with form content type (`application/x-www-form-urlencoded`
or `multipart/form-data`)  
When request is handled  
Then default parser MUST not JSON-decode body automatically and handler-observed
typed input MUST remain zero-valued under default parser flow.

### BRC-ACT-004 (covers BR-ACT-004)

Given action input fails validation/parsing with validation-class error  
When request is handled  
Then response MUST be `400`.

### BRC-ACT-005 (covers BR-ACT-005)

Given caller reads `Actions().SupportedMethods()` and mutates returned map
values  
When subsequent action-route behaviors are exercised  
Then runtime-supported method behavior MUST remain unchanged from configured
defaults/options.

### BRC-ACT-006 (covers BR-ACT-006)

Given a typed GET action route and a `HEAD` request with query params that
matches it through router fallback  
When request parsing/dispatch runs through the actions handler  
Then request MUST complete without unsupported-method parse failure and
handler-observed typed input semantics MUST remain GET/query-equivalent.

## 5.10 Head Integration Scenarios

### BRC-HEAD-001 (covers BR-HEAD-001)

Given default head callback adds elements and matched loaders add head
elements  
When HTML response is rendered  
Then final head output MUST contain merged set subject to dedupe rules.

### BRC-HEAD-002 (covers BR-HEAD-002)

Given HTML response path  
When head content is generated  
Then output MUST include Wave critical CSS style element and stylesheet link.

### BRC-HEAD-003 (covers BR-HEAD-003)

Given non-JSON production rendering with known `deps` and `cssBundles` order  
When head output is inspected  
Then `modulepreload` links MUST appear before CSS bundle stylesheet links, and
within each class ordering MUST match `deps`/`cssBundles`.

### BRC-HEAD-004 (covers BR-HEAD-004)

Given request path has no matching loader route (or otherwise reaches proxy
redirect/error short-circuit) and default-head callback is configured to fail  
When request is handled  
Then response MUST preserve terminal loader outcome semantics (for unmatched
path, `404`) and MUST NOT be rewritten to `500` solely due to default-head
callback failure.

## 5.11 Dev Reload Endpoint Scenarios

### BRC-DEV-001 (covers BR-DEV-001)

Given dev mode and updated stage-one route artifact on disk  
When `GET /__vorma/reload-routes` is called  
Then response MUST be `200` with body `ok`, and subsequent loader responses MUST
use reloaded build/route metadata.

Given same scenario and the updated artifact removes a previously client-defined
route pattern  
When requests are issued after reload  
Then that removed client route MUST no longer resolve as a client route
(except documented server-task placeholder retention cases).

### BRC-DEV-002 (covers BR-DEV-002)

Given dev mode and changed template file on disk  
When `GET /__vorma/reload-template` is called  
Then response MUST be `200` with body `ok`, and subsequent HTML responses MUST
reflect template change.

### BRC-DEV-003 (covers BR-DEV-003)

Given dev mode and reload operation fails (invalid/missing source)  
When reload endpoint is called  
Then response MUST be `500` and include error text.

### BRC-DEV-004 (covers BR-DEV-004)

Given runtime is not in dev mode and no loader route is registered at reload
paths  
When `/__vorma/reload-routes` or `/__vorma/reload-template` is requested  
Then response MUST follow normal loader path resolution behavior (typically
`404`) rather than executing dev reload operations.

### BRC-DEV-005 (covers BR-DEV-005)

Given dev mode with route reload where route metadata changes but matched
pattern set stays constant  
When `/__vorma/reload-routes` succeeds and same route is requested again  
Then route-data metadata fields (`importURLs`, `exportKeys`, `errorExportKeys`,
`deps`, `cssBundles`) MUST reflect post-reload artifacts rather than pre-reload
cached values.

### BRC-DEV-006 (covers BR-DEV-006)

Given dev mode and reload endpoints are called with non-GET methods (for example
`POST`)  
When request path is `/__vorma/reload-routes` or `/__vorma/reload-template`  
Then runtime MUST still execute corresponding reload and return reload success
or failure according to artifact/template validity.

### BRC-DEV-007 (covers BR-DEV-007)

Given dev mode where stage-one artifact changes both `buildID` and
`routeManifestFile`  
When `/__vorma/reload-routes` succeeds and route is requested again  
Then response `X-Vorma-Build-Id` and SSR bootstrap `routeManifestURL` MUST
reflect updated artifact values.

### BRC-DEV-008 (covers BR-DEV-008)

Given dev mode where loader task handlers are registered for patterns that
remain present before and after route reload  
When `/__vorma/reload-routes` succeeds and those routes are requested  
Then matched handlers MUST still execute (no dropped handler bindings due to
route-tree rebuild).

### BRC-DEV-009 (covers BR-DEV-009)

Given dev mode with a known-good active route/template state and a subsequent
reload request that fails (invalid artifact/template)  
When failure response is observed and normal requests continue afterward  
Then runtime MUST continue serving the previously active route/template behavior
without partial adoption of failed reload inputs.

### BRC-DEV-010 (covers BR-DEV-010)

Given a runtime instance where stage-one artifact values differ from current
in-memory snapshot  
When `ReloadRoutesFromDisk()` is called directly  
Then subsequent route/build/manifest observations MUST reflect stage-one artifact
values.

Given direct-call artifact load failure  
When `ReloadRoutesFromDisk()` returns error  
Then pre-call runtime snapshot behavior MUST remain unchanged.

### BRC-DEV-011 (covers BR-DEV-011)

Given active template `T0` and on-disk replacement `T1`  
When `ReloadTemplateFromDisk()` succeeds  
Then subsequent HTML rendering MUST reflect `T1`.

Given parse-failing template source for direct call  
When `ReloadTemplateFromDisk()` returns error  
Then subsequent HTML rendering MUST continue using previously active template.

## 5.12 Delegated Static Middleware Scenario

### BRC-STATIC-001 (covers BR-STATIC-001)

Given the same underlying Wave instance is observed through:

- direct `Wave.ServeStatic(true)` middleware usage, and
- `Vorma.ServeStatic()` middleware usage,

When equivalent static-asset and non-asset requests are exercised  
Then Vorma-observed static middleware behavior MUST be delegation-equivalent to
the Wave-owned static middleware contract (`WAVE-RT-010`, `WAVE-RT-011`).

## 5.13 Concurrency Scenarios

### BRC-CONC-001 (covers BR-CONC-001)

Given high-concurrency mixed loader/action traffic  
When run under race detector / stress harness  
Then no data races, panics, or malformed payloads MUST occur.

### BRC-CONC-002 (covers BR-CONC-002)

Given concurrent request traffic plus repeated `reload-routes` calls in dev  
When stress test executes  
Then service MUST remain available and payload index alignment MUST remain
valid.

### BRC-CONC-003 (covers BR-CONC-003)

Given a caller reads snapshot-style metadata accessors and mutates returned
map/slice values (including router route-collection accessors exposed via
`LoadersRouter().AllRoutes()` / `ActionsRouter().AllRoutes()`, and
`GetAdHocTypes()` payload slices)  
When subsequent loader requests are served  
Then runtime route-data behavior MUST remain based on internal authoritative
state and MUST NOT reflect caller-side mutation of snapshot return values.

### BRC-CONC-004 (covers BR-CONC-004)

Given callback logic is executed under `WithRLock`  
When callback attempts mutation through lock-protected setter APIs on
`LockedVorma`  
Then runtime MUST reject that mutation path (compile-time unavailability or
fail-fast runtime guard) and post-callback observable runtime state MUST remain
unchanged.

Given callback logic is executed under `WithLock` and applies lock-protected
mutations  
When callback returns  
Then subsequent getter observations MUST reflect those writes atomically.

## 6. Conformance Test Suite Guidance

Conformance suites derived from this spec SHOULD:

1. Keep each requirement ID and scenario ID independently reportable.
2. Use deterministic fixture apps with explicit route graphs.
3. Assert only public observations (status, headers, body, template output, log
   lines where required).
4. Include both positive and negative cases for every requirement family.
5. Treat concurrency timing checks as statistical with slack, not single-run
   exact deadlines.
6. Run all scenario groups in both dev and prod where behavior differs.
7. Track requirement-to-test traceability in CI output using IDs (`BR-*`,
   `BRC-*`).

## 7. Relation to Other Specs

- Traceability matrix: `specs/packages/vormaruntime/TRACEABILITY_MATRIX.md`
- Conformance issues: `specs/packages/vormaruntime/CONFORMANCE_ISSUES.md`
- Frontend runtime:
  `specs/packages/vormaclient/client/SPEC.md`
- Build/dev conformance:
  `specs/packages/vormabuild/SPEC.md`
- Interop contracts: `specs/packages/vorma/SPEC.md`
- Wave runtime-serving owner contracts:
  `specs/packages/wave/SPEC.md`
- Testing strategy:
  `specs/SPEC_GOVERNANCE.md`
- Checklist/roadmap: `specs/packages/vormaruntime/SPEC_CHECKLIST.md`

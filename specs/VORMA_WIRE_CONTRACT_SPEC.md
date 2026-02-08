# Vorma Wire Contract Conformance Specification

Status: Draft  
Last Updated: 2026-02-07  
Applies To: HTTP protocol contract between Vorma backend runtime and Vorma client/runtime consumers

## 1. Why This Spec Exists

This is a black-box wire conformance spec.

It is intended to:

- make backend/frontend interoperability testable without implementation coupling,
- enable refactors that preserve protocol behavior,
- define externally visible contract semantics (headers, query params, payloads).

## 2. Conformance Boundaries

### 2.1 Normative Terms

The terms **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

### 2.2 Allowed Observations

Conformance tests MUST use observable HTTP I/O only:

- request method/path/query/header/body,
- response status/header/body,
- full-document HTML output (including script/comment markers).

Tests MUST NOT assert:

- internal structs/locks/functions,
- private runtime storage,
- package-internal implementation details.

## 3. Protocol Terms

- Document Mode Request: loaders request without active `vorma_json` mode.
- JSON Route-Data Request: loaders request with `vorma_json=<non-empty token>`.
- Current Build JSON Request: `vorma_json` equals current server build ID.
- Stale Build JSON Request: `vorma_json` non-empty and not equal to current build ID.
- Action Request: request handled by actions router mount root.

## 4. Requirement Catalog

## 4.1 Header Contract Requirements

### WIRE-HDR-001: Build ID Header (Loaders)

Given a non-control-path request handled by loaders route handler  
When a response is produced  
Then header `X-Vorma-Build-Id` MUST be present.

Scope refinement:

- includes normal loader wire outcomes (document mode, current/stale JSON,
  loader not-found, proxy redirect/error outcomes),
- excludes dev reload control endpoints (`/__vorma/reload-routes`,
  `/__vorma/reload-template`), which are governed by `WIRE-DEV-*`.

### WIRE-HDR-002: Build ID Header (Actions)

Given a request handled by actions route handler  
When a response is produced  
Then header `X-Vorma-Build-Id` MUST be present.

### WIRE-HDR-003: Stale JSON Reload Signal

Given a stale build JSON request  
When loaders handler responds  
Then header `X-Vorma-Reload` MUST be set to same URL with `vorma_json` removed.

### WIRE-HDR-004: Client Redirect Header

Given backend sets a client redirect via response API  
When response is emitted  
Then header `X-Client-Redirect` MUST carry redirect target URL.

### WIRE-HDR-005: Default Cache-Control for Loader Success

Given loaders handler produces non-short-circuited success response and no
existing `Cache-Control` header  
When response is written  
Then header MUST be:

`Cache-Control: private, max-age=0, must-revalidate, no-cache`.

### WIRE-HDR-006: Set-Cookie Collision Resolution

Given multiple loader-side proxy mutations set cookies with colliding names  
When final response is emitted  
Then wire output MUST contain one winning cookie value per colliding cookie name
using later-proxy precedence, while preserving distinct cookie names.

### WIRE-HDR-007: Client Redirect Header Cardinality

Given final non-error response includes client redirect signaling  
When header output is observed  
Then `X-Client-Redirect` MUST be single-valued and MUST carry the winning
redirect target.

### WIRE-HDR-008: Submit Deployment Header Propagation

Given client global contains deployment id and `submit()` issues an action
request  
When request headers are sent  
Then request header `x-deployment-id` MUST be present and equal that deployment
id value.

Given client global deployment id is absent/empty and `submit()` issues an
action request  
When request headers are sent  
Then request header `x-deployment-id` MUST be omitted.

## 4.2 Query Parameter Contract Requirements

### WIRE-Q-001: JSON Route-Data Query Key Activation

Given loaders request query key `vorma_json` has a non-empty value  
When request is processed  
Then backend MUST interpret request as JSON route-data mode.

### WIRE-Q-002: Current vs Stale Build Semantics

Given `vorma_json` query value  
When value equals current build ID  
Then server MUST return route-data JSON payload.

Given `vorma_json` query value  
When value differs from current build ID  
Then server MUST return stale-build reload signal response (see
`WIRE-HDR-003` and `WIRE-JSON-002`).

### WIRE-Q-003: Hard Reload Query Key Compatibility

Given client performs hard reload flow  
When redirect URL is constructed client-side  
Then query key `vorma_reload` MAY be added with latest build ID.

Server implementations MUST tolerate this key on incoming requests.

### WIRE-Q-004: Revalidation Deployment Query Propagation

Given client global contains deployment id and a revalidation request is issued  
When request URL is constructed  
Then query key `dpl` MUST be present and equal deployment id value.

Given client global deployment id is absent/empty and a revalidation request is
issued  
When request URL is constructed  
Then query key `dpl` MUST be omitted.

### WIRE-Q-005: Stale-Build Sentinel Precedes Loader Not-Found

Given request carries stale `vorma_json` token  
When request path has no matching loader route  
Then wire response MUST still be stale-build sentinel semantics (`200` + reload
header) rather than loader `404`.

## 4.3 Content Type Contract Requirements

### WIRE-CT-001: JSON Responses

Given any Vorma route-data JSON response (including stale-build OK response)  
When response is emitted  
Then `Content-Type` MUST be `application/json`.

### WIRE-CT-002: HTML Document Responses

Given successful document-mode loaders response  
When response is emitted  
Then `Content-Type` MUST be `text/html`.

## 4.4 JSON Route-Data Payload Contract

## 4.4.1 Top-Level Shape

### WIRE-JSON-001: Route-Data Object

Given current build JSON request with matched route  
When response is emitted  
Then body MUST be a JSON object representing route-data.

### WIRE-JSON-002: Stale-Build Response Body

Given stale build JSON request  
When response is emitted  
Then body MUST be JSON success sentinel (`{"ok":true}`) and MUST NOT be
route-data payload.

## 4.4.2 Route-Data Fields

For route-data payloads, fields below are normative with `omitempty` semantics
(field MAY be omitted when empty/null):

- `matchedPatterns: string[]`
- `loadersData: any[]`
- `importURLs: string[]`
- `exportKeys: string[]`
- `errorExportKeys: string[]`
- `hasRootData: boolean`
- `params: Record<string,string>`
- `splatValues: string[]`
- `deps: string[]`
- `cssBundles: string[]`
- `viteDevURL: string`
- `outermostServerError: string`
- `outermostServerErrorIdx: number`
- `title: HeadEl`
- `metaHeadEls: HeadEl[]`
- `restHeadEls: HeadEl[]`

Where `HeadEl` supports:

- `tag?: string`
- `attributes?: Record<string,string>`
- `attributesKnownSafe?: Record<string,string>`
- `booleanAttributes?: string[]`
- `textContent?: string`
- `dangerousInnerHTML?: string`

### WIRE-JSON-003: Index Alignment Invariant

Given route-data payload  
When arrays `matchedPatterns`, `loadersData`, `importURLs`, `exportKeys`,
`errorExportKeys` are present  
Then entries MUST be index-aligned by route depth order.

### WIRE-JSON-004: Error Truncation Invariant

Given loaders produce outermost server error  
When route-data payload is emitted  
Then route-data arrays MUST be truncated at outermost error index and include:

- `outermostServerError`
- `outermostServerErrorIdx`.

### WIRE-JSON-005: `viteDevURL` Mode Contract

Given JSON route-data payload is emitted in dev mode  
When `viteDevURL` field is inspected  
Then value MUST be `http://localhost:<vite-port>` for the currently selected
Vite dev-server port.

Given JSON route-data payload is emitted in non-dev mode  
When `viteDevURL` field is inspected  
Then value MUST be empty string.

## 4.5 HTML Document Payload Contract

### WIRE-HTML-001: SSR Bootstrap Global Symbol

Given successful document-mode loaders response  
When HTML is emitted  
Then document MUST include bootstrap script initializing:

`globalThis[Symbol.for("__vorma_internal__")]`.

### WIRE-HTML-002: Bootstrap Keys

Bootstrap payload MUST include keys required by client runtime:

- `patternToWaitFnMap`
- `clientLoadersData`
- `isDev`
- `viteDevURL`
- `buildID`
- `publicPathPrefix`
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
- `deps`
- `cssBundles`
- `deploymentID`
- `routeManifestURL`

### WIRE-HTML-003: Head Marker Comments

Given server-rendered head element block  
When document is emitted  
Then HTML head MUST contain marker comments:

- `data-vorma="meta-start"` / `data-vorma="meta-end"`
- `data-vorma="rest-start"` / `data-vorma="rest-end"`

### WIRE-HTML-004: Deployment ID Bootstrap Population Rules

Given document bootstrap payload includes `deploymentID` key  
When backend skew-protection env gate is disabled  
Then `deploymentID` MUST be empty string, even if deployment-id env value is
set.

Given skew-protection env gate is enabled and deployment-id env value is set  
When document bootstrap payload is emitted  
Then `deploymentID` MUST equal that deployment-id env value.

Given skew-protection env gate is enabled and deployment-id env value is unset  
When document bootstrap payload is emitted  
Then `deploymentID` MUST be empty string.

### WIRE-HTML-005: Bootstrap Mutable-Store Initialization Shapes

Given successful document-mode bootstrap payload  
When mutable runtime store keys are initialized  
Then bootstrap assignment shapes MUST be:

- `patternToWaitFnMap` as an object literal (`{}`),
- `clientLoadersData` as an array literal (`[]`).

These keys MUST NOT initialize to `null`/`undefined`.

## 4.6 Route Manifest Contract

### WIRE-MAN-001: Manifest URL in Bootstrap

Given successful document-mode loaders response  
When bootstrap payload is emitted  
Then `routeManifestURL` MUST reference a public URL path to route manifest JSON.

Composition refinement:

- `routeManifestURL` MUST be composed from active `publicPathPrefix` and active
  route-manifest filename using single-boundary path-join semantics (no
  duplicate slash seams and no missing separator).

### WIRE-MAN-002: Manifest JSON Shape

Given route manifest is fetched  
When response is parsed  
Then body MUST be JSON object:

`Record<string, number>`

where values are:

- `1` => pattern has server loader,
- `0` => pattern has no server loader.

### WIRE-MAN-003: Manifest Filename Prefix

Given build produces route manifest  
When filename is generated  
Then filename MUST begin with:

`vorma_out_vorma_internal_route_manifest_`

and end with `.json`.

## 4.7 Redirect Protocol Contract

### WIRE-REDIR-001: Client Redirect Handshake Header

Given client navigation/action fetch requests  
When request is sent  
Then client MAY set `X-Accepts-Client-Redirect: 1` to request redirect via
header protocol.

### WIRE-REDIR-002: Redirect Signal Priority (Client Consumer Contract)

Given response contains multiple redirect indicators  
When client interprets response  
Then precedence MUST be:

1. `X-Vorma-Reload` (highest)
2. fetch `response.redirected`
3. `X-Client-Redirect` (lowest)

### WIRE-REDIR-003: Internal Hard Reload Semantics

Given client receives `X-Vorma-Reload`  
When client effects redirect  
Then redirect strategy MUST be hard reload and SHOULD include
`vorma_reload=<latest build id>` on internal targets.

## 4.8 Dev Reload Endpoint Wire Contract

### WIRE-DEV-001: Reload Routes Endpoint

Given dev mode  
When `GET /__vorma/reload-routes` succeeds  
Then response MUST be HTTP `200` with body `ok` (text payload).

### WIRE-DEV-002: Reload Template Endpoint

Given dev mode  
When `GET /__vorma/reload-template` succeeds  
Then response MUST be HTTP `200` with body `ok` (text payload).

### WIRE-DEV-003: Reload Endpoint Failure Contract

Given dev mode and reload operation fails  
When either reload endpoint is called  
Then response MUST be HTTP `500` with error text body.

### WIRE-DEV-004: Reload Endpoint Method-Agnostic Path Dispatch

Given dev mode  
When `/__vorma/reload-routes` or `/__vorma/reload-template` is called with a
non-GET method  
Then wire behavior MUST match path-targeted reload semantics (success/failure)
rather than method-gated not-found semantics.

### WIRE-DEV-005: Reload Endpoints Are Inert Outside Dev Mode

Given runtime is not in dev mode  
When `/__vorma/reload-routes` or `/__vorma/reload-template` is requested  
Then response MUST follow normal loader-path wire behavior (typically `404`) and
MUST NOT execute reload side effects.

## 5. Executable Conformance Scenario Catalog

This section defines concrete black-box scenarios that SHOULD be used as the
default test vectors for requirements above.

Scenario IDs are stable references for test planning and CI reporting.

## 5.1 Header Scenarios

### WRC-HDR-001 (covers WIRE-HDR-001)

Given loaders requests in modes:

- document success,
- current-build JSON success,
- stale-build JSON,
- loader 404,
- loader proxy error/redirect.

When responses are captured  
Then all MUST include `X-Vorma-Build-Id`.

### WRC-HDR-002 (covers WIRE-HDR-002)

Given action requests covering success, validation failure, and not-found paths  
When responses are captured  
Then all MUST include `X-Vorma-Build-Id`.

### WRC-HDR-003 (covers WIRE-HDR-003)

Given request `/users/1?x=1&vorma_json=stale&y=2`  
When stale JSON response is returned  
Then `X-Vorma-Reload` MUST equal `/users/1?x=1&y=2`.

### WRC-HDR-004 (covers WIRE-HDR-004)

Given backend emits client redirect through response proxy  
When response is observed  
Then header `X-Client-Redirect` MUST carry exact redirect target.

### WRC-HDR-005 (covers WIRE-HDR-005)

Given non-short-circuited loaders success without explicit cache header  
When response is emitted  
Then `Cache-Control` MUST equal
`private, max-age=0, must-revalidate, no-cache`.

### WRC-HDR-006 (covers WIRE-HDR-005)

Given loaders/proxy sets explicit `Cache-Control` value  
When response is emitted  
Then runtime MUST preserve explicit value.

### WRC-HDR-007 (covers WIRE-HDR-006)

Given matched loaders set cookies with colliding names and distinct names  
When response headers are captured  
Then `Set-Cookie` output MUST reflect distinct-name preservation and one
later-winner value per colliding name.

### WRC-HDR-008 (covers WIRE-HDR-007)

Given matched loaders emit multiple client redirects in non-error flow  
When response headers are captured  
Then `X-Client-Redirect` MUST contain one winning value only.

### WRC-HDR-009 (covers WIRE-HDR-008)

Given client has deployment id and issues `submit()`  
When outbound request is inspected  
Then header `x-deployment-id` MUST be present with exact deployment id value.

Given client deployment id is absent/empty and issues `submit()`  
When outbound request is inspected  
Then header `x-deployment-id` MUST be omitted.

## 5.2 Query Semantics Scenarios

### WRC-Q-001 (covers WIRE-Q-001)

Given requests:

- `/page`
- `/page?vorma_json=`
- `/page?vorma_json=abc`

When processed by loaders handler  
Then only non-empty `vorma_json` request MUST activate JSON route-data mode.

### WRC-Q-002 (covers WIRE-Q-002)

Given current build ID `b123`  
When `/page?vorma_json=b123` is requested  
Then response MUST be route-data JSON.

Given `/page?vorma_json=other`  
When requested  
Then response MUST be stale-build sentinel with reload header.

### WRC-Q-003 (covers WIRE-Q-003)

Given request includes `vorma_reload=<token>` query key  
When server handles request  
Then normal response semantics MUST continue unchanged (server tolerates key).

### WRC-Q-004 (covers WIRE-Q-004)

Given client has deployment id and triggers revalidation  
When outbound request URL is inspected  
Then query key `dpl` MUST be present and equal deployment id.

Given client deployment id is absent/empty and triggers revalidation  
When outbound request URL is inspected  
Then query key `dpl` MUST be omitted.

### WRC-Q-005 (covers WIRE-Q-005)

Given current build id `b123` and request `/missing?vorma_json=stale`  
When response is observed  
Then response MUST be stale-build sentinel with reload header and MUST NOT be
`404`.

## 5.3 Content-Type Scenarios

### WRC-CT-001 (covers WIRE-CT-001)

Given JSON route-data and stale-build sentinel responses  
When inspected  
Then `Content-Type` MUST be `application/json`.

### WRC-CT-002 (covers WIRE-CT-002)

Given successful document-mode response  
When inspected  
Then `Content-Type` MUST be `text/html`.

## 5.4 JSON Payload Scenarios

### WRC-JSON-001 (covers WIRE-JSON-001)

Given current-build JSON request for matched route  
When response body is parsed  
Then top-level type MUST be JSON object.

### WRC-JSON-002 (covers WIRE-JSON-002)

Given stale-build JSON request  
When response body is parsed  
Then body MUST equal `{"ok":true}` and MUST NOT include route-data fields.

### WRC-JSON-003 (covers WIRE-JSON-003)

Given response with multiple matched patterns  
When payload arrays are inspected  
Then `matchedPatterns`, `loadersData`, `importURLs`, `exportKeys`,
`errorExportKeys` MUST have aligned indices.

### WRC-JSON-004 (covers WIRE-JSON-004)

Given a loader error occurs at index `i`  
When payload is emitted  
Then arrays MUST be truncated to `i+1` and include
`outermostServerError` + `outermostServerErrorIdx`.

### WRC-JSON-005 (covers WIRE-JSON-001)

Given payload includes head element arrays  
When `metaHeadEls` / `restHeadEls` are present  
Then elements MUST conform to `HeadEl` field contract.

### WRC-JSON-006 (covers WIRE-JSON-005)

Given one JSON route-data response served in dev mode and one in non-dev mode  
When `viteDevURL` is inspected  
Then dev response MUST use `http://localhost:<vite-port>` form and non-dev
response MUST emit empty string.

## 5.5 HTML Payload Scenarios

### WRC-HTML-001 (covers WIRE-HTML-001)

Given document-mode loaders success  
When HTML is parsed  
Then bootstrap script MUST initialize
`globalThis[Symbol.for("__vorma_internal__")]`.

### WRC-HTML-002 (covers WIRE-HTML-002)

Given document-mode loaders success  
When bootstrap object is inspected  
Then required runtime keys listed by `WIRE-HTML-002` MUST be present.

### WRC-HTML-003 (covers WIRE-HTML-003)

Given document-mode loaders success  
When `<head>` content is inspected  
Then marker comments for meta/rest blocks MUST be present.

### WRC-HTML-004 (covers WIRE-HTML-004)

Given one run with skew-protection env gate disabled, one run with gate enabled
plus deployment-id env set, and one run with gate enabled plus deployment-id
env unset  
When bootstrap payload `deploymentID` is inspected  
Then disabled run MUST emit empty string, enabled+set run MUST emit configured
deployment-id value, and enabled+unset run MUST emit empty string.

### WRC-HTML-005 (covers WIRE-HTML-005)

Given document-mode loaders success  
When bootstrap script assignments are inspected  
Then `patternToWaitFnMap` MUST be object-literal initialized and
`clientLoadersData` MUST be array-literal initialized (not null/undefined).

## 5.6 Manifest Scenarios

### WRC-MAN-001 (covers WIRE-MAN-001)

Given document-mode loaders success  
When bootstrap payload is inspected  
Then `routeManifestURL` MUST be non-empty, public-path-resolvable, and path-join
equivalent to `{publicPathPrefix}/{routeManifestFile}` under normalized
single-boundary slash semantics.

### WRC-MAN-002 (covers WIRE-MAN-002)

Given `routeManifestURL` is fetched  
When parsed as JSON  
Then payload MUST be `Record<string, number>` with values only `0` or `1`.

### WRC-MAN-003 (covers WIRE-MAN-003)

Given build output includes route manifest filename  
When filename is inspected  
Then it MUST match prefix
`vorma_out_vorma_internal_route_manifest_` and suffix `.json`.

## 5.7 Redirect Scenarios

### WRC-REDIR-001 (covers WIRE-REDIR-001)

Given client fetch request initiated by runtime navigation/actions  
When request headers are inspected  
Then `X-Accepts-Client-Redirect` SHOULD be set to `1`.

### WRC-REDIR-002 (covers WIRE-REDIR-002)

Given controlled response has all redirect indicators set:

- `X-Vorma-Reload`,
- `response.redirected=true`,
- `X-Client-Redirect`

When redirect parser runs  
Then selected redirect MUST come from `X-Vorma-Reload`.

### WRC-REDIR-003 (covers WIRE-REDIR-002)

Given response has:

- no `X-Vorma-Reload`,
- `response.redirected=true`,
- `X-Client-Redirect` present

When redirect parser runs  
Then selected redirect MUST come from `response.redirected`.

### WRC-REDIR-004 (covers WIRE-REDIR-002)

Given response has only `X-Client-Redirect`  
When redirect parser runs  
Then redirect MUST be sourced from `X-Client-Redirect`.

### WRC-REDIR-005 (covers WIRE-REDIR-003)

Given redirect handling chooses hard redirect to internal URL  
When URL is constructed  
Then resulting URL SHOULD include `vorma_reload=<latest build id>`.

## 5.8 Dev Reload Endpoint Scenarios

### WRC-DEV-001 (covers WIRE-DEV-001)

Given dev mode and route reload succeeds  
When `GET /__vorma/reload-routes` is called  
Then response MUST be `200` with body `ok`.

### WRC-DEV-002 (covers WIRE-DEV-002)

Given dev mode and template reload succeeds  
When `GET /__vorma/reload-template` is called  
Then response MUST be `200` with body `ok`.

### WRC-DEV-003 (covers WIRE-DEV-003)

Given dev mode and reload operation fails  
When either reload endpoint is called  
Then response MUST be `500` with error text body.

### WRC-DEV-004 (covers WIRE-DEV-004)

Given dev mode and valid reload artifacts/template  
When `POST /__vorma/reload-routes` and `POST /__vorma/reload-template` are called  
Then responses MUST follow success semantics (`200` with body `ok`) equivalent to
method-agnostic path dispatch.

### WRC-DEV-005 (covers WIRE-DEV-005)

Given runtime is non-dev and no explicit loader route exists at reload paths  
When `/__vorma/reload-routes` or `/__vorma/reload-template` is requested  
Then response MUST be loader-path normal behavior (`404`) and MUST NOT return dev
reload success sentinel.

## 6. Conformance Test Suite Guidance

Wire conformance tests SHOULD:

1. Keep each requirement ID and scenario ID independently reportable.
2. Validate mode-specific header presence and values for every response class.
3. Validate stale-build and current-build JSON behaviors separately.
4. Validate route-data schema with optional-field (`omitempty`) handling.
5. Validate bootstrap script keys and head marker comments in HTML mode.
6. Validate redirect precedence with controlled response fixtures.
7. Validate route manifest URL, shape, and value semantics.
8. Track traceability via IDs (`WIRE-*`, `WRC-*`) in CI output.

## 7. Relation to Other Specs

- Backend runtime behavior:
  `/Users/sjc/__code/river/specs/VORMA_BACKEND_RUNTIME_SPEC.md`
- Frontend runtime behavior:
  `/Users/sjc/__code/river/specs/VORMA_FRONTEND_RUNTIME_SPEC.md`
- Build/dev conformance:
  `/Users/sjc/__code/river/specs/VORMA_BUILD_DEV_CONFORMANCE_SPEC.md`
- Kit dependency boundaries:
  `/Users/sjc/__code/river/specs/VORMA_KIT_INTEROP_SPEC.md`
- Testing strategy:
  `/Users/sjc/__code/river/specs/VORMA_TESTING_STRATEGY_SPEC.md`
- Overall roadmap:
  `/Users/sjc/__code/river/specs/SPECS_CHECKLIST.md`

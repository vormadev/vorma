# kit/response Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/response`

## Scope

This package owns HTTP response helper behavior and proxy/merge behavior used by
handlers that need deferred/parallel response composition.

## Requirement Catalog

### Response Helpers

#### KIT-RESPONSE-001: Response Construction and Commit State

Given `New(w)`  
When called  
Then it MUST return a `Response` bound to `w` with `isCommitted=false`.

Given `IsCommitted()`  
When any commit-capable helper has not yet run  
Then it MUST return `false`; after commit-capable helpers run it MUST return
`true`.

#### KIT-RESPONSE-002: Header Mutation Helpers

Given `SetHeader(key, value)` and `AddHeader(key, value)`  
When called  
Then they MUST mutate writer headers immediately and MUST NOT commit the
response by themselves.

#### KIT-RESPONSE-003: Status/Error Commit Semantics

Given `SetStatus(status)`  
When called  
Then it MUST write the status and mark response committed.

Given `Error(status, reasons...)`  
When reasons are supplied  
Then joined reason text (`strings.Join(reasons, " ")`) MUST be used.

Given no reasons are supplied  
When called  
Then HTTP status text MUST be used.

#### KIT-RESPONSE-004: Contentful Helper Content-Type and Body Semantics

Given `JSONBytes`, `JSON`, `Text`, `HTMLBytes`, or `HTML`  
When called  
Then each helper MUST set its expected content type and write content, and MUST
mark response committed.

`JSON(v)` MUST use `json.Encoder.Encode(v)` semantics.

#### KIT-RESPONSE-005: Canonical OK Helpers

Given `OK()`  
When called  
Then it MUST emit status `200`, content type `application/json`, and body
`{"ok":true}`.

Given `OKText()`  
When called  
Then it MUST emit status `200`, content type `text/plain`, and body `OK`.

#### KIT-RESPONSE-006: Status and Error Convenience Mappings

Given convenience helpers  
When called  
Then mappings MUST be:

- `NotModified` -> 304, `NotFound` -> 404,
- `Unauthorized` -> 401, `InternalServerError` -> 500,
- `BadRequest` -> 400, `TooManyRequests` -> 429,
- `Forbidden` -> 403, `MethodNotAllowed` -> 405.

#### KIT-RESPONSE-007: Redirect Status Normalization

Given redirect status spread argument to `Redirect`/`ServerRedirect`  
When no code is provided  
Then default status MUST be `303 See Other`.

Given provided code is outside `3xx` range  
When normalized  
Then status MUST fall back to `303 See Other`.

#### KIT-RESPONSE-008: Redirect Path Selection

Given `Redirect(r, url, code...)`  
When request indicates client-redirect support  
Then it MUST attempt client redirect path and return `(true, nil)` on success.

Given request does not indicate client-redirect support (including nil request)  
When called  
Then it MUST use server redirect path and return `(false, nil)`.

#### KIT-RESPONSE-009: Server Redirect Behavior

Given `ServerRedirect(r, url, code...)` with non-nil request  
When called  
Then it MUST delegate to `http.Redirect` and mark committed.

Given nil request  
When called  
Then it MUST set `Location` header, set status directly, and commit.

#### KIT-RESPONSE-010: Client Redirect Guard and Validation

Given `ClientRedirect(url)`  
When URL fails validation  
Then it MUST return an error and MUST NOT commit response.

Given response is already committed  
When called  
Then it MUST return an error.

Given URL is valid and response is not committed  
When called  
Then it MUST set `X-Client-Redirect` header, set status `200`, and commit.

#### KIT-RESPONSE-011: Redirect Accept/URL Validation Helpers

Given `doesAcceptClientRedirect(r)`  
When request is nil or header is not parseable bool  
Then it MUST return `false`.

Given parseable true bool header value in `X-Accepts-Client-Redirect`  
When called  
Then it MUST return `true`.

Given `validateURL(location)`  
When location is empty, parse-fails, or absolute with non-HTTP(S) scheme  
Then it MUST return `false`; otherwise `true`.

### Proxy Helpers

#### KIT-RESPONSE-012: Proxy Construction and Concurrency Model

Given `NewProxy()`  
When called  
Then it MUST initialize header-op storage and zero-value response state.

Proxy instances are per-scope/per-goroutine state holders and are not thread
safe for shared concurrent mutation.

#### KIT-RESPONSE-013: Proxy Status and Classification Semantics

Given `SetStatus(status, text...)`  
When text is omitted  
Then stored status text MUST be cleared.

Given status classification helpers  
When called  
Then `IsError`/`IsSuccess`/redirect helpers MUST reflect status-class logic in
package helpers.

#### KIT-RESPONSE-014: Proxy Header Operation Semantics

Given proxy header ops for a key  
When computed  
Then `set` MUST reset accumulated values and `add` MUST append values.

Given `GetHeader(key)`  
When values exist  
Then it MUST return first computed value; `GetHeaders` returns full computed
slice.

#### KIT-RESPONSE-015: Proxy Cookies and Head Elements

Given `SetCookie(nil)`  
When called  
Then it MUST no-op.

Given non-nil cookies  
When set  
Then they MUST be appended in order.

Given `AddHeadEls`/`GetHeadEls`  
When called  
Then head-element storage MUST be lazily initialized and merged.

#### KIT-RESPONSE-016: Proxy Redirect Semantics

Given `Redirect(r, url, code...)`  
When request accepts client redirects  
Then proxy MUST perform client redirect path.

Given request does not accept client redirects  
When called  
Then proxy MUST perform server redirect path.

Given proxy already represents error status  
When server redirect is requested  
Then server redirect MUST NOT override error status.

#### KIT-RESPONSE-017: ApplyToResponseWriter Write Ordering

Given `ApplyToResponseWriter(w, r)`  
When called  
Then application order MUST be:

1. headers (with set/add semantics),
2. cookies,
3. server redirect path (if active and non-error; return early),
4. status/error write.

Given error status with custom text  
When applied  
Then `http.Error` MUST use custom text; otherwise default status text.

#### KIT-RESPONSE-018: MergeProxyResponses Arbitration and Merge Order

Given `MergeProxyResponses(proxies...)`  
When merging  
Then behavior MUST be:

- nil proxies ignored,
- head elements merged in proxy order,
- header operations appended in proxy order,
- cookies deduped by cookie name with later proxy value winning,
- status arbitration: first error status wins, otherwise last success status,
- redirect arbitration (when merged status is not error): first redirect wins.

When winning redirect is client redirect, merged result MUST contain one
`X-Client-Redirect` header value from winning redirect proxy.

## Scenario Catalog

- `KRS-001`: construct response/proxy and initial state semantics.
- `KRS-002`: header mutation and set/add semantics.
- `KRS-003`: commit-state and status/error helpers.
- `KRS-004`: JSON/text/html helper output and content types.
- `KRS-005`: canonical OK helper payloads.
- `KRS-006`: convenience status/error mapping helpers.
- `KRS-007`: redirect code normalization.
- `KRS-008`: redirect path selection including nil request behavior.
- `KRS-009`: server/client redirect guard behavior.
- `KRS-010`: accept-header and URL validation helper behavior.
- `KRS-011`: proxy status/classification behavior.
- `KRS-012`: proxy cookie/head-element behavior.
- `KRS-013`: apply-to-writer ordering and redirect/error precedence.
- `KRS-014`: merge arbitration and ordered merge semantics.

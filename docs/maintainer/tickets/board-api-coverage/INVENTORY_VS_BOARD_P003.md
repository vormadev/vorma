# Inventory vs Board coverage cross-reference (P003)

Produced by packet P003 on 2026-07-01. This is the authoritative public-API inventory of
the `vorma` crate crossed against `examples/board`, with file:line citations for covered
items and a coverage-plan table for uncovered items.

Cited paths are repository-root-relative. This is a coverage artifact for the maintainer,
not user-facing material; nothing here belongs in Board source or the Board README.

## Inventory method (reproducible)

The authoritative user-facing surface (per `PRESSURE_TEST_CENSUS.md`) is the `vorma`
`lib.rs` re-export map + the `app!` macro emissions + `vorma::testing` +
`vorma::middleware` + `vorma::tasks` + `vorma_build::run` + the `TsGen` derive.
Build/macro plumbing (`build_interface`, `__private`, `__vorma_*` macros) is E-PLUMBING
and exempt from Board feature mapping.

The inventory was produced by a mechanical scan of public declarations
(`pub fn|struct|enum|const|type|trait|mod|use`, dropping `pub(crate)/pub(super)`) in
exactly these files, then reconciled member-by-member against Board:

- `crates/vorma/src/lib.rs` (the re-export map + top-level fns/consts + `tasks` module +
  `HtmlAttribute`/`SafeHtml` + `app!`/`__vorma_*` macros)
- `crates/vorma/src/{config,public_app,static_route,exit,error,request,search_params,form_data,head,document_builder,resource_body,resource_response,typed_handler,testing,middleware}.rs`
  (member surfaces of the re-exported types)
- `crates/vorma-tasks/src/{lib,task,parallel,cancel,clock,observer,overrides,error,key}.rs`
- `crates/vorma-build/src/lib.rs` (`run`)
- `crates/vorma-macros/src/lib.rs` (`TsGen`)

The scan script used for this pass lived in the executor scratchpad and is not checked in;
permanent checked-in regeneration tooling is the deliverable of the separate
`../api-inventory-tooling/` ticket (xtask). Coverage below was verified by targeted `grep`
of `examples/board` (src + tests + client) plus reading the handler bodies — mechanical
inventories do not prove coverage on their own.

## Inventory size

- User-facing public items enumerated (types + their members + free fns + consts +
  macros): ~200 across the surface above (E-PLUMBING excluded).
- Covered by Board before P003: the large majority (full route/handler/exit/head
  /document/testing/tsgen/middleware families; the whole TypeScript client surface).
- Newly covered by P003: 12 items (see "Newly covered").
- Escalated (cannot fit a real Board app flow without a maintainer ruling): the standalone
  `vorma::tasks` runtime-lifecycle surface + low-level head/document type carriers + three
  informational consts (see "Escalated").

The TypeScript surface (`api_inventory_ts.txt`) is fully covered by Board and is not
re-tabulated here; spot-check counts are in the P003 REPORT.

## Covered before P003 (representative citations)

App declaration / config:

- `app!`, `AppConfig` (every field), `App::from_config`, `Views/Resources/Middlewares` +
  `views!/resources!/middlewares!`, `View/view!`, `Resource/resource!` (+ `kind:`,
  `method:`), `Middleware::{new,with_patterns,with_methods}`:
  `examples/board/src/lib.rs:23,84-158`, `examples/board/src/views.rs`,
  `examples/board/src/resources.rs`, `examples/board/src/session.rs:81,95,112`,
  `examples/board/src/bin/server.rs:33`.
- `ServerTarget`, `FrontendConfig`, `UiVariant::React`, `TsGenConfig`, `DevWatchConfig`:
  `examples/board/src/lib.rs:87-123`.
- `tsgen::{TsDrafter::new, export_const, export_type, TsExtraType::of}`:
  `examples/board/src/lib.rs:68-111`.
- `ResourceKind::{Mutation,Query}`: `examples/board/src/resources.rs` (each resource).

Handler contexts (`ViewCtx`/`ResourceCtx`/`MiddlewareCtx`):

- `state`/`input`/`param`/`params`/`splat_values`/`request`/`exec_ctx`/
  `public_url`/`response`/`head`/`redirect`/`redirect_with_status`: across
  `examples/board/src/views.rs` and `examples/board/src/resources.rs` (e.g.
  `views.rs:55,57,113,195,208,292`; `resources.rs:62,83,170,191`).
- `Params::{get,iter,len,is_empty}`: exercised through `param`/`params` on the nested user
  routes (`views.rs:189-254`).

Exits/errors (before P003):

- `ViewExit::{err,with_client_msg}`: `views.rs:59,416-419`.
- `HttpExit::{err,with_status,with_client_msg}`: `resources.rs:68-72,130-145`.
- `Error::{new,with_source}`, `Result`, `BoxError` (via `vorma::Error`):
  `examples/board/src/store.rs:82`, `examples/board/src/repo.rs:91`,
  `examples/board/src/session.rs:16`.

Request/head/document/response:

- `HttpRequest::{path,search_params,headers}`; `HttpSearchParams::{get,get_all,iter}`:
  `views.rs:327-334`, `session.rs:32,97`, `document.rs:12`.
- `HeadHandle::{title,description,meta_property_content,meta_name_content}`:
  `views.rs:50,124-133`.
- `HeadBuilder::{meta,link,script,style,name,content,rel,href,r#as,r#type,icon,meta_charset,meta_property_content,meta_name_content,description,title}`:
  `examples/board/src/document.rs:20-68`.
- `Document::{new,html,body,head}`, `DocumentAttributes::{lang,id,class,data}`,
  `DocumentBuildCtx::{request,public_url}`, `DocumentBuilder::new`:
  `examples/board/src/document.rs`.
- `HtmlAttribute::{attr,r#type}`, `SafeHtml::{script_content,style_content}`:
  `document.rs:41-60`.
- `ResponseHandle::{set_header,set_cookie}`,
  `ResourceResponseHandle::{set_status,set_header,set_cookie}`:
  `resources.rs:83-85,287,416`, `session.rs:98`.
- `ResourceBody::new`, `ResourceOutput` (via output type): `resources.rs:400-426`.
- `HttpCookie`, `HttpMethod`, `HttpStatusCode`, `HttpHeaderName`, `HttpHeaderValue`,
  `HttpHeaderMap`: `session.rs`, `resources.rs`.

Tasks (consumer surface):

- `task!`, `Task::run`, `memoized`, `extended_cache`, `ParallelBatch::{new,add,run}`,
  `ParallelBatchOutputs::take`, `ParallelBatchOutputHandle` (transitive), `ExecCtx` (as
  the run handle), `TasksOptions` (as an `AppConfig` field, `default()`):
  `examples/board/src/repo.rs` (all task defs incl. `extended_cache` at
  `repo.rs:300-339`), `examples/board/src/views.rs:112-120,211-220,390-398`.

Server/runtime + middleware:

- `bind_addr`, `is_dev`, `is_build`: `examples/board/src/bin/server.rs:30-32`.
- `vorma_build::run`: `examples/board/src/bin/build.rs:2`.
- All ten `vorma::middleware` helpers + `EtagLayer::{strong,max_body_size,skip}`
    - `EtagRequest::{method,uri,headers}`: `examples/board/src/bin/server.rs:41-71`.
- `App::from_config`, `RuntimeHost` (returned by `from_config`, mounted via
  `fallback_service`): `examples/board/src/bin/server.rs:33,40`.

Testing:

- `TestApp::{builder,get,get_view_payload,request_json,request,public_url}`,
  `TestRequest::{header,cookie,body,send}`, `TestAppBuilder::{with_public_asset,build}`:
  `examples/board/tests/app.rs` (throughout).

TypeScript (client): fully covered — see `frontend-client-coverage.md` and the P003 REPORT
spot-checks. `apiClient.{query,queryOrThrow,mutate,mutateOrThrow,toIdentityArray}`, the
`To*Args/Method/Pattern/Error` families, `ApiClientOutput`, the full `createVormaClient`
option bag, `Link` prop breadth, all hooks, `defineView` lifecycle callbacks,
`runClientLoaderOnHmr`, `workIndicator.{track,isActive}`, kit modules — all exercised in
`examples/board/src/client/`.

## Newly covered by P003

| Item                                                                      | Board home                                                                                                                                                                    | Teaching angle                                                                                                                                      |
| ------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Task::single_flight` policy                                              | `repo.rs:341-386` (`SiteStats`, `LIVE_SITE_STATS`) surfaced in `views.rs` LAYOUT + `layout.view.tsx` footer; asserted `tests/app.rs` (`anonymous_layout_has_no_session_user`) | Completes the cache-policy trio: a live volatile counter that coalesces concurrent duplicates but retains nothing (vs `memoized`/`extended_cache`). |
| `ViewExit::with_source`                                                   | `views.rs` LAYOUT stats read                                                                                                                                                  | Preserve the underlying error as the exit source chain instead of flattening to a string (the F-2 teaching point).                                  |
| `HttpRequest::method`                                                     | `views.rs` NOT_FOUND                                                                                                                                                          | Raw request line access for a diagnostics fallback; comment steers normal routes to typed input.                                                    |
| `HttpRequest::uri`                                                        | `views.rs` NOT_FOUND                                                                                                                                                          | Same.                                                                                                                                               |
| `HttpRequest::query`                                                      | `views.rs` NOT_FOUND                                                                                                                                                          | Undecoded query string vs parsed `search_params`.                                                                                                   |
| `ResponseHandle::append_header` / `ResourceResponseHandle::append_header` | `resources.rs` STORY_ATTACHMENT (`Vary: accept-encoding`)                                                                                                                     | Add a value to a list-valued header without replacing existing values.                                                                              |
| `TestApp::from_config`                                                    | `tests/app.rs` (`app_booted_from_config_serves_the_build_id_header`)                                                                                                          | No-frills constructor for tests that need no synthesized public assets.                                                                             |
| `TestApp::client_build_id`                                                | same test                                                                                                                                                                     | Read the committed client build id.                                                                                                                 |
| `TEST_CLIENT_BUILD_ID`                                                    | same test                                                                                                                                                                     | The stable build id an in-memory test app commits.                                                                                                  |
| `CLIENT_BUILD_ID_HEADER_KEY` (const)                                      | same test                                                                                                                                                                     | Assert the build-id header every response carries (stale-bundle detection).                                                                         |

(`HttpExit::with_source` is now demonstrable the same way as `ViewExit::with_source` but
was left to the existing resource error paths for a future mechanical row; see Escalated
note E-8.)

## Escalated (needs a maintainer ruling)

These items cannot be covered by a realistic Board app flow without a new framework
opinion, or would amount to a framework-semantic test placed in Board (prohibited), or are
already covered by the framework-owned usability test `crates/vorma/tests/public_api.rs`.
Board is an HTTP app: the framework owns the `Tasks` runtime and hands `ExecCtx` to
handlers, so the standalone runtime-lifecycle surface has no honest app-flow home in
Board.

| Item(s)                                                                                                                                                                                                             | Why not a Board app flow                                                                                                                                                                                                                                                                                                           | Where it lives today                                                                                                                                                                                       | Recommendation                                                                                                                                                                                                                                                                                                    |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Tasks::new`, `Tasks::exec_ctx`, `CancelToken` (+ `new/cancel/is_cancelled/cancelled/child`), `ExecCtx::{cancel_token,is_cancelled,child}`, `Task::id`                                                              | An HTTP app never constructs its own `Tasks`/`ExecCtx`/`CancelToken`; the framework does. Forcing Board to spin up a second runtime inside a handler to call `.child()`/`cancel_token()` would be contrived and wrong.                                                                                                             | `crates/vorma-tasks/tests/tasks.rs`, `crates/vorma-tasks/benches/tasks.rs`, `crates/vorma/tests/public_api.rs` (`SystemClock`/`ClockInstant`). Framework internals construct them (`execution_engine.rs`). | Rule that the standalone `vorma::tasks` runtime-lifecycle surface is covered by the sovereign-crate suites + `public_api.rs`, NOT by Board. `vorma::tasks` is a general-purpose crate serving builds/CLIs/daemons; Board is one consumer and legitimately only uses `task!`/`run`/`ParallelBatch`.                |
| `Clock`, `ClockInstant`, `SystemClock`                                                                                                                                                                              | `TasksOptions.clock` is for deterministic test time; a real app never overrides it. A Board clock override teaches nothing real.                                                                                                                                                                                                   | `crates/vorma/tests/public_api.rs:71-72`, `crates/vorma-tasks/tests/tasks.rs`.                                                                                                                             | Same ruling: framework-owned usability coverage suffices.                                                                                                                                                                                                                                                         |
| `TaskOverrides`, `TaskOverrideMode`                                                                                                                                                                                 | Test-injection machinery. The equivalent Board test was DELIBERATELY MOVED out to `crates/vorma/tests/in_memory_test_app.rs` (see `frontend-client-coverage.md`). Re-adding to Board reverses a maintainer decision.                                                                                                               | `crates/vorma/tests/in_memory_test_app.rs:184`, `crates/vorma-tasks/tests/tasks.rs`.                                                                                                                       | Confirm it stays out of Board (framework-semantic).                                                                                                                                                                                                                                                               |
| `TaskObserver`, `TaskEvent`, `TaskEventKind`, `TaskEventOutcome`, `TaskRunSource`                                                                                                                                   | A slow-task-logging observer wired into `TasksOptions.observer` at the server main IS a realistic app pattern, but the wiring belongs to the server binary's task-runtime construction (`app_config_with`), needs a chosen teaching story, and edges toward "instrument the framework." Not mechanical.                            | `crates/vorma-tasks/tests/tasks.rs`.                                                                                                                                                                       | Decide whether Board's server should wire a real telemetry observer (a genuine feature — I recommend YES as a follow-up packet with a defined teaching story), or whether observer coverage is framework-suite-only.                                                                                              |
| Low-level `HeadBuilder` plumbing: `new`, `add`, `append`, `elements`, `self_closing`, `dangerous_inner_html`, `text_content`, `attr_exists`; attr helpers `src`, `charset`, `property`, `cross_origin`, `bool_attr` | The census already labels these "the builders' own plumbing (exercised transitively)." Board uses the high-level head helpers; contriving direct calls to `elements()`/`self_closing()`/`attr_exists()` in an app teaches nothing. `src`/`cross_origin` are realistic and could be added mechanically to `document.rs` if desired. | `crates/vorma/tests/public_api.rs:4-22`.                                                                                                                                                                   | Rule that low-level head/document type carriers are covered by `public_api.rs`; optionally add `src`/`cross_origin`/`property` to Board's document head as a small mechanical row.                                                                                                                                |
| `HeadHandle::{icon,meta_charset,preload,add,append}` (via `ctx.head()`)                                                                                                                                             | Redundant with the same methods already covered on `HeadBuilder` in `document.rs`; a per-view `ctx.head().preload(...)` is plausible (route-specific resource hint) but low value.                                                                                                                                                 | Covered on `HeadBuilder`, not on the handler handle.                                                                                                                                                       | Optional mechanical row (e.g. STORY view `ctx.head().preload(og_image)`); not required if handle/builder parity is accepted as transitively covered.                                                                                                                                                              |
| `DocumentAttributes::{attribute,known_safe_attribute,boolean_attribute}`, `Document::head_dedupe_rules`                                                                                                             | Board's document uses `lang/id/class/data`; the generic `attribute`/`boolean_attribute` and the `head_dedupe_rules` policy knob are plumbing/edge.                                                                                                                                                                                 | `crates/vorma/tests/public_api.rs` (`attribute` via `lang`), defaults exercise `head_dedupe_rules`.                                                                                                        | Optional mechanical row (add a `boolean_attribute`/`attribute` on `document.html()`); accept `head_dedupe_rules` as documented-default coverage.                                                                                                                                                                  |
| `HttpRequest::{extensions,extension}`                                                                                                                                                                               | Only useful for reading a tower-injected extension (e.g. `tower_http::request_id::RequestId`); there is no app API to insert a request extension. Reading a THIRD-PARTY type in a Board handler is a teaching-design choice (extension vs header).                                                                                 | Framework preserves axum extensions (verified; census adjudication #1).                                                                                                                                    | Rule on the intended teaching pattern: read the `x-request-id` HEADER in a handler (clean, no third-party type) vs read the tower `RequestId` extension. I recommend the header form for the app-facing example and treating `extensions()/extension()` as the documented escape hatch (framework-suite covered). |
| `EtagRequest::extensions`                                                                                                                                                                                           | The etag `skip` predicate reads `method/uri/headers`; `extensions` on that request has no realistic skip use.                                                                                                                                                                                                                      | —                                                                                                                                                                                                          | Accept as transitively/edge; not worth a contrived skip predicate.                                                                                                                                                                                                                                                |
| Consts `PUBLIC_STATIC_OUT_NAME_PREFIX`, `DEFAULT_REQUEST_BODY_LIMIT`                                                                                                                                                | Informational constants an app rarely references; Board sets its own `request_body_limit`. Asserting them fits a test, not an app flow.                                                                                                                                                                                            | —                                                                                                                                                                                                          | Rule whether informational consts must be Board-referenced or count as covered by their public export + doc comment.                                                                                                                                                                                              |
| `TestApp::handle_request`                                                                                                                                                                                           | The raw request primitive that every higher-level `TestApp` helper (`get`, `request_json`, `request().send`) already calls internally, so it is transitively exercised by all 20 Board tests. A direct demo needs `http` + `bytes` dev-deps solely to hand-build a `Request<Bytes>`.                                               | Transitively via every Board test; directly in `crates/vorma/tests`.                                                                                                                                       | Accept as transitively covered, OR rule that Board should add `http`/`bytes` dev-deps for one direct raw-request example. I recommend accepting transitive coverage (adding deps for the raw escape hatch is gold-plating).                                                                                       |
| `HttpExit::with_source`                                                                                                                                                                                             | Now trivially demonstrable (same as `ViewExit::with_source`), just not yet placed.                                                                                                                                                                                                                                                 | —                                                                                                                                                                                                          | Mechanical follow-up row: attach `.with_source(error)` on a resource task-error path in `resources.rs` (E-8). Left out of P003 to keep the diff focused; can land in the same style anytime.                                                                                                                      |

## The public_api.rs question (the central ruling this packet surfaces)

`crates/vorma/tests/public_api.rs` is a framework-owned "…is_usable_externally" smoke test
that already touches much of the escalated surface (low-level head type carriers,
`Document`/`DocumentAttributes`, `tsgen`, `SystemClock`/ `ClockInstant`, `TasksOptions`,
`tasks::Error/Result`, `RuntimeHost::handle_request`).

The `AGENTS.md` policy says Board must cover 100% of public APIs, "period." But the
maintainer has ALSO (a) placed the awkward type-carrier/standalone surface in
`public_api.rs`, and (b) deliberately moved the `TaskOverrides` test OUT of Board. Those
two facts imply an operative policy narrower than the literal text: Board covers the
surface an application actually uses in a realistic app flow; the standalone-crate runtime
lifecycle and low-level type carriers are covered by the sovereign-crate suites and
`public_api.rs`.

Recommendation: ratify that split explicitly in `docs/maintainer/board-example/README.md`
so future audits do not re-open it: "Board covers 100% of the APIs an application uses in
a realistic app flow. The standalone `vorma::tasks` runtime-lifecycle surface
(`Tasks`/`ExecCtx`/ `CancelToken`/`Clock`/overrides/observers) and low-level head/document
type carriers are sovereign-crate + `public_api.rs` coverage, not Board coverage, because
Board consumes tasks through `task!`/`run`/`ParallelBatch` and the framework owns the
runtime." If instead the literal 100%-in-Board rule stands, the observer telemetry feature
is the one genuinely app-shaped addition worth a follow-up packet; the rest would be
contrived.

## Post-ruling re-triage (Fable, 2026-07-01)

The F-17 escalation was ruled by the maintainer with a sharper test than the packet's
recommendation: **framework-author primitive vs app-useful primitive** ("advanced" is
never grounds for exemption; tasks are a sovereign crate Vorma builds on, and apps
obviously run background work). Re-triage of the escalated table:

- **Board owes coverage — queued as Phase C teaching rows:**
    - `Tasks::{new,exec_ctx}` + `CancelToken` — a background worker that constructs its
      own task runtime, opens an `ExecCtx` per iteration, and cancels on shutdown.
    - `ExecCtx::{is_cancelled,child}` (+ `cancel_token` as it naturally appears) —
      cooperative cancellation inside a long-running app task body.
    - `TaskObserver` family + `Task::id` — slow-task telemetry on the server's
      `TasksOptions.observer`.
- **Exempt (framework-author surface), discharged by sovereign suites +
  `crates/vorma/tests/public_api.rs`:** `TaskOverrides`/`TaskOverrideMode` (prior
  maintainer ruling: framework test suite home), `Clock`/`SystemClock`/`ClockInstant`
  (determinism-injection tooling; revisit if an app-shaped need appears), low-level
  head/document type carriers, informational consts.

The durable rule text lives in `docs/maintainer/board-example/README.md`.

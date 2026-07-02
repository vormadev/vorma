# Pressure-Test App Census

Task-local context for `__TICKET.md`. This file records the Board coverage design and
finding ledger. It includes resolved findings and stale observations; do not treat every
row as open work without checking current code and the current ticket.

Current maintainer ruling: Board is the living coverage app for Vorma. Board must cover
100% of public Vorma APIs. Period. Board is allowed to grow whatever feature is needed to
cover Vorma. If an API has no existing Board feature, add or invent a Board feature that
uses it in a realistic app flow. When Board exposes framework friction, fix the framework
and keep Board coverage complete.

Backing inventories (regenerated 2026-06-23 from the current tree):
`api_inventory_rust.txt`, `api_inventory_ts.txt`. The authoritative Rust surface is the
`vorma` lib.rs re-export map + the `app!` macro emissions + `vorma::testing` +
`vorma_build::run`; `build_interface`, `__private`, and the `__vorma_*` macros are
macro/build plumbing, not user API, and are exempt from feature mapping (E-PLUMBING
below).

## The app (working name: "Vorma Board" — name TBD)

An HN-shaped community: stories, threaded comments, votes, users, moderation. Real
persistence (SQLite via rusqlite), passwordless demo auth, replacing the earlier legacy
example wholesale.

### Features

- F1 Front page `/_index`: score-ranked story list, page pagination via typed search
  params (corrected 2026-07-01, P005 audit: current code is page-only —
  `FrontInput { page: Option<i64> }` — no cursor pagination exists anywhere in board; the
  prior "cursor/page" line described a feature that was never built), story rows with vote
  buttons, relative timestamps, domain extraction.
- F2 Story page `/s/:story_id`: story + threaded comments, comment composer (auth-gated
  UI), per-comment collapse state, og/meta head per story.
- F3 Submit `/submit`: auth-gated route (middleware), URL-or-text story form, optional
  file attachment (multipart), server validation rejections with explicit client messages,
  redirect to the new story on success.
- F4 Votes: POST resources for story/comment votes; optimistic UI; revalidation semantics;
  one-vote-per-user constraint enforced by SQLite UNIQUE -> rejection envelope.
- F5 Auth: `/login` (pick any username — demo auth), session cookie (HttpOnly), logout;
  `current_user` is loaded ONCE per request by a global middleware running a Task that
  handlers re-run for the deduped result (the task-sharing idiom, finally demonstrated).
- F6 User pages `/u/:username` (+ nested `/u/:username/comments`): profile layout view
  with child tabs — real nested-route usage with a parent that owns data.
- F7 Moderation `/mod` + `/mod/*`: scoped middleware (`patterns: ["/mod", "/mod/*"]`) —
  anonymous -> redirect, banned -> 401 envelope; kill/restore items; mod log table.
- F8 Docs `/docs/*`: splat route serving seeded pages from SQLite (slug chain from splat
  values).
- F9 Search `/search`: GET resource with schema'd input (query, tags, pagination), used by
  a search view via client-side query.
- F10 SQLite-via-Tasks: rusqlite behind a small repo layer; every read is a `Task`
  (spawn_blocking inside) so concurrent needs dedupe per request; story+comments+author
  load in parallel on F2.
- F11 Dev loop realism: seed-data files under `seed-data/` watched via
  `on_change_client_revalidate`; schema bootstrap on boot; `is_dev`/`is_build`/`bind_addr`
  in the server main; `vorma_build::run` in the build binary.
- F12 Error story end-to-end: constraint violations and validation -> `HttpExit` +
  envelope; dead/killed story -> soft 200 rendered state; a deliberately failing segment
  (mod-only diagnostics panel) -> `ViewExit` with client msg, parents still render.
- F13 Head/document breadth: per-story og/twitter tags, preload, SafeHtml boot script +
  critical style, html/body attributes.
- F14 TS gen extras: exported consts (e.g. vote deltas, tag list) via
  `TsDrafter::export_const` + `TsExtraType` consumed by the client.
- F15 Client breadth (framework surface only): layout work indicator, an api-client
  decorator (app-minted csrf-style token header on mutations), search box synced to the
  URL via `useRouteSync({debounceMs})`, prefetch on intent + explicit prefetch, error
  boundaries, client loader deriving per-story read-state from localStorage, app-owned
  theme toggle and relative-time helpers (plain platform APIs — deliberately NOT a
  framework concern).
- F16 Tests: full `vorma::testing::TestApp` suite against a tmp SQLite file per test —
  auth cookies, multipart submit, vote uniqueness envelope, redirects, splat docs, mod
  gating, payload nesting.
- F17 Ecosystem integration (maintainer-ratified expansion; FULLY LANDED as of P005,
  2026-07-01 — the two previously-pending homes are closed: `useApiQuery`/
  `apiQueryOptions` ship with F9 search, `apiQueryOptions` is exported from `api.ts` and
  composed with `query_client.prefetchQuery`/`ensureQueryData` on the search view's
  intent/submit actions, and `workIndicator.track` ships with the F3 import parse — no
  dead surface exists): the app runs popular libraries against Vorma's lifecycles, as
  applications will — _ nprogress driven by the `workIndicator` client option
  (`{start, stop, startDelayMs, stopDelayMs}` IS the nprogress contract). Vorma API calls
  are ALREADY included in the bar (WorkState.apiRequests) no matter who orchestrates them
  — react-query included. `workIndicator.track(promise)` is for NON-vorma async the app
  wants reflected (home: the client-side FileReader parse of an import attachment before
  upload); _ @tanstack/react-query over the apiClient, BOTH halves, via app-owned TYPED
  WRAPPER HOOKS (the deliverable idiom, per maintainer): `useApiQuery(args)` = useQuery
  with `queryKey: apiClient.toIdentityArray(args)` + queryFn delegating to `queryOrThrow`
  (search view), and `useApiMutation()` = useMutation with full typed args at mutate()
  call time (vote targets vary per row). NO manual revalidation wiring: Vorma mutations
  AUTO-revalidate route data by default (args carry `revalidate?: boolean` to opt out), so
  the cooperation story is cleaner than first drafted — react-query owns mutation state
  (isPending, errors) and invalidates only ITS OWN derived caches. Vorma's typed args are
  the entire call-site interface; the wiring lives in one place. Wrapper facts vs the
  Go-era consumer reference: typed args extend RequestInit so react-query's per-query
  `signal` threads straight through ({...args, signal}); ToQueryArgs/ToMutationArgs are
  already KIND-split (the old `Extract<..., {method?: "GET"}>` hack is obsolete — and was
  subtly wrong: a POST query exists); `queryOrThrow` throws QueryError carrying the typed
  result (richer than the old `new Error(result.error)`); the options-builder layer
  (`apiQueryOptions(args)`, `examples/board/src/client/api.ts`) is exported separately
  from `useApiQuery` and composed with `prefetchQuery`/`ensureQueryData` on the search
  view's hover/focus and submit-click intent signals
  (`examples/board/src/client/views/search.view.tsx`), landed P005; _ jotai + `kit/theme`:
  the TS kit stays generic and owns the theme state/storage API. For flash-free first
  paint, the TS kit docs show the exact HTML to inline, and Vorma provides
  `vorma::kit::theme::script(None)` as the native Rust document injection convenience for
  that same snippet. The important boundary is that the TS kit does not depend on Vorma;
  the Rust helper is a Vorma-app convenience, not a separate theme system. Board's jotai
  atom wraps `initTheme()` + `addThemeChangeListener` (cross-tab for free); _
  `useViewTransitions: true` on the client (View Transitions on navigations).
  Division-of-responsibility is the deliverable: route/view data is Vorma's; ad-hoc API
  data is react-query's; React state is jotai's; persistence/broadcast is kit's; the bar
  is nprogress's.

## Rust surface -> features

App declaration (`vorma::app!`, `AppConfig` & co):

- `app!` macro, `App`, `AppConfig` (every field), `Views`/`views!`,
  `Resources`/`resources!`, `Middlewares`/`middlewares!`, `View`/`view!`,
  `Resource`/`resource!` (+ `kind:`), `Middleware::new` -> F1–F9 (the whole route table).
- `Middleware::with_patterns`/`with_methods` -> F7 (mod gate), F5 (current-user preload is
  global: documents the omitted-filter form), plus a method-scoped CSRF-header echo
  middleware on mutations (`with_methods([POST, DELETE])`, app-owned token logic) -> F15.
- `ServerTarget`, `FrontendConfig`, `UiVariant`, `TsGenConfig`, `DevWatchConfig`,
  `tsgen::{TsDrafter, TsExtraType}` -> F11/F14.
- `DocumentBuilder`, `Document`, `DocumentBuildCtx` -> F13.
- `ResourceKind` (explicit Query/Mutation) -> F4/F9.

Handler contexts (`ViewCtx`/`ResourceCtx`/`MiddlewareCtx`):

- `state` -> everywhere; `input` -> F1/F3/F9; `param` -> F2/F6; `params` (struct +
  `get`/`iter`/`len`/`is_empty`) -> F6 tabs; `splat_values` -> F8; `request` -> F5 (cookie
  read) and F6 (canonical path redirect); `head` -> F2/F13; `response` (headers/cookies)
  -> F5; `resource_response().set_status` -> F3/F4 (201s); `exec_ctx` -> F10; `public_url`
  -> F13; `redirect`/`redirect_with_status` -> F3 explicit post-submit redirect, F6 view
  canonicalization redirect, and F7 middleware redirect.
- `ViewInput`/`ResourceInput` traits: macro bounds, exercised by every typed route
  (E-PLUMBING for direct use).

Exits & errors:

- `ViewExit::err/with_client_msg/with_source` -> F12.
- `HttpExit::err/with_status/with_client_msg/with_source` -> F3/F4/F7.
- `Error::new/with_source`, `BoxError`, `Result` -> repo layer (F10) and config assembly
  (F11).

Request types:

- `HttpRequest::{method,uri,path,query,search_params,headers,body, extensions,extension}`
  -> F5 (headers/cookies), F9 (query), F3 (body via FormData path);
  `extensions`/`extension` -> F11 (tower bridge: request-id read in handler log lines).
- `HttpSearchParams::{get,get_all,iter}` -> F9.
- `FormData`/`FormField`/`FormFile` -> F3.
- `HttpMethod`, `HttpStatusCode`, `HttpHeaderName`, `HttpHeaderValue`, `HttpHeaderMap`,
  `HttpCookie` -> F3–F7 (statuses, headers, cookies).

Head/document builders:

- `HeadBuilder` breadth (title/description/icon/meta\_\*/link/script/ style + attr
  helpers), `HtmlAttribute::{attr,r#type,r#as,...}`,
  `SafeHtml::{style_content,script_content}` -> F13.
- `HeadAttr`, `HeadBooleanAttribute`, `HeadInnerHtml`, `HeadSelfClosing`, `HeadTag`,
  `HeadTextContent`, `HtmlElementDef`, `HeadHandle` -> type-level carriers of the same
  calls (exercised transitively by F13; no separate homes needed).

Tasks:

- `task!`, `Task::{id, run}`, `ParallelBatch::{new, add, run}`,
  `ParallelBatchOutputs::take`, `Tasks`, `TasksOptions`, `ExecCtx`, `Result`, `Error`,
  `TaskId` -> F10/F5 (the load-bearing family).
- `CancelToken` -> F10 (cancellation propagation noted in repo docs).
- `TaskObserver`/`TaskEvent`/`TaskEventKind`/`TaskEventOutcome`/ `TaskRunSource` ->
  slow-query logging observer wired in the server main (F11).
- `Clock`/`ClockInstant`/`SystemClock`, `TaskOverrides`/ `TaskOverrideMode` -> the
  `TasksOptions` field surface (corrected 2026-07-01, P005 audit: this line previously
  claimed an F16 failure-injection test via `TaskOverrides::replace` inside board; that
  test does not exist in `examples/board/tests/app.rs` and never did in current code —
  `TaskOverrides` usage lives exclusively in
  `crates/vorma/tests/in_memory_test_app.rs:185`, consistent with F17's own later ruling
  that `TaskOverrides` was deliberately moved out of Board. The F16 bullet below was
  simply never updated after that move; see the F16 row correction).

Server/runtime:

- `RuntimeHost` -> F11 (axum mount in server main), `bind_addr`, `is_dev`, `is_build` ->
  F11; `vorma_build::run` -> F11 build bin.
- `vorma::middleware` module (tower layers, full set of TEN, verified by untruncated
  listing: request_id, sensitive_headers, panic_recovery, request_body_limit,
  handler_timeout, request_body_timeout, response_body_timeout, compression, etag,
  secure_headers) -> F11: the server main composes all ten.

Testing:

- `TestApp::{from_config,builder,handle_request,get,get_view_payload, request_json,request,client_build_id,public_url}`,
  `TestRequest::{header,cookie,body,send}`, `TestAppBuilder::{with_public_asset,build}`,
  `TEST_CLIENT_BUILD_ID` -> F16 (every one has a direct test).

Derives/macros:

- `TsGen` derive -> every shared type. `__vorma_view`, `__vorma_resource`,
  `build_interface`, `__private` -> E-PLUMBING.

## TypeScript surface -> features

`vorma/react` (and preact/solid twins): `createVormaClient` returning

- `boot`, `RootOutlet`, `defineView` (component, `clientLoader`, `errorBoundary`,
  `beforeRouteCommit`, `beforeRouteYield`) -> F2/F15; `beforeRouteYield` ->
  unsaved-comment guard on F2.
- `Link` (+ `linkDefaultProps` prefetch), `navigate`, `prefetch`, `cancelPrefetch`,
  `toHref` -> F1/F2/F15 (toHref: canonical/share URLs on story rows).
- `useViewData`, `useClientLoaderData`, `useRouteState`, `useWorkState`, `useRouteSync` ->
  F1/F2/F15.
- `usePatternViewData`, `usePatternClientLoaderData` -> layout reads child data for the
  title bar on F6 (parent shows active tab counts).
- `apiClient` (`query`, `queryOrThrow`, `mutate`, `mutateOrThrow`), `MutationError`,
  `QueryError`, decorator types (`ToApiDecorator`, `ToApiDecoratorContext`) -> F4/F9/F15
  (decorator = csrf header).
- `revalidate`, `getRouteState`, `getWorkState`, `workIndicator`,
  `revalidateOnWindowFocus` option, `defaultErrorBoundary` option,
  `BuildSkewDetectedEvent`/`onBuildSkew`-class options -> F15/F11 (dev skew banner).
  Imperative `getRouteState`/`getWorkState` -> analytics hook in F15.
- Generated seed (`vormaClientSeed`) + `vorma.gen.ts` types/consts -> F14.

`vorma/client` + `vorma/__internal`: what the official adapters are built from. The `__`
prefix is itself the stance — internal, unstable, undocumented, by name and convention;
nothing to adjudicate. Exempt like E-PLUMBING. If the app ever wants something from in
there, that is a promotion request (move it out under a real name), never an import.

`vorma/vite` plugin -> F11 (vite.config.ts).

Kit (`vorma/kit/*`): historical note — this was once treated as out of scope for the Board
coverage pass. That is no longer current under the 100% public API coverage rule. Board
now consumes the public kit entry points as normal app utilities: `vorma/kit/theme` in the
theme atom/document pre-paint path, and converters, cookies, csrf, debounce, fmt, json,
listeners, and result in the mod diagnostics browser utility path. The boundary still
matters: kit must stay generic and must not grow Board-specific or Vorma-backend-specific
helpers.

## Prior Adjudication List — Resolved

This section records items that were once questioned under older framing. Every candidate
was resolved once the facts were on the table; none remains open:

1. `HttpRequest::extensions/extension` — the standard tower bridge, verified working
   (runtime_app preserves axum request extensions). Keep; mapped to F11 (handlers read the
   request id from `vorma::middleware::request_id()` for log lines).
2. `Clock`/`ClockInstant`/`SystemClock`/`TaskOverrides`/ `TaskOverrideMode` — the named
   types of public `TasksOptions` fields; cutting them would cut legitimate capabilities
   (deterministic test time, task-body substitution). Keep; mapped — corrected 2026-07-01
   (P005 audit): the failure-injection test via `TaskOverrides::replace` lives in
   `crates/vorma/tests/in_memory_test_app.rs`, not in board's F16 test suite
   (`TaskOverrides` has zero call sites anywhere under `examples/board/`); this is
   consistent with, not contradicting, F17's later ruling that the standalone tasks
   runtime-lifecycle surface (which includes `TaskOverrides`) is sovereign-crate +
   `public_api.rs` coverage, not Board coverage.
3. The ten `vorma::middleware` tower layers (request_id, sensitive_headers,
   secure_headers, panic_recovery, request_body_limit, handler_timeout,
   request_body_timeout, response_body_timeout, compression, etag) — standard production
   hygiene, maintainer-confirmed good. Keep all; the server main composes all ten (F11).

Prior dissolutions: `__internal` (the `__` prefix IS the stance). Every public framework
API has a named feature home in F1-F16, and current Board code covers the public kit entry
points as app utilities.

## Sequencing

1. App skeleton: schema + repo-as-Tasks + auth + F1/F2 vertical slice.
2. Remaining features; the legacy example is removed in the same change that the new app's
   tests go green.
3. Docs initiative begins, with this app as the running example.

## Member-level surface (second sweep — the nested API census)

The first sweep enumerated entry-point exports and type names; tons of the real surface is
NESTED (members of returned objects, option bags, prop types). Second sweep, member by
member:

TS — `createVormaClient` options (`ClientOptions` + adapter extras): `render`,
`workIndicator` (-> F17 nprogress), `revalidateOnWindowFocus` (bool | {staleTimeMs,
skipWorkIndicator} — F15), `defaultErrorBoundary` (F15), `useViewTransitions` (-> F17),
`onRouteUpdate` (-> F15: SPA page-view analytics hook), `onWorkUpdate` (low-level twin of
workIndicator; documented as the escape hatch, not separately exercised),
`onBuildSkewDetected` (-> F11 dev skew notice), `linkDefaultProps` (F15).

TS — client object members: `boot`, `defineView` (`pattern`, `component`, `clientLoader`,
`errorBoundary`, `beforeRouteCommit`, `beforeRouteYield`, `runClientLoaderOnHmr` -> F11
dev loop), `RootOutlet`, `Link`, hooks (`useRouteSync`, `useRouteState`, `useWorkState`,
`useViewData`, `usePatternViewData`, `useClientLoaderData`, `usePatternClientLoaderData`),
`navigate`, `prefetch`, `cancelPrefetch`, `toHref`, `revalidate`, `getRouteState`,
`getWorkState`, `workIndicator` ({track, isActive}: track = non-vorma async only; vorma
calls are auto-included -> F17), `apiClient`.

TS — `apiClient` members: `query`, `queryOrThrow`, `mutate`, `mutateOrThrow`,
`toIdentityArray` (-> F17 react-query keys).

TS — `Link` props (`LinkPropsBase`): `prefetch`, `prefetchDelayMs`, `attributeMatchRules`
(-> F15 active-nav styling in the layout), `visitOnPointerDown` (-> F15 on story-list
links), `replace` (-> F15 profile tabs and search box sync/imperative navigate; corrected
2026-07-01, P005 audit: the prior "post-login" attribution does not match current code —
sign-in's `onSuccess` only clears the username input, there is no post-login
`navigate`/`Link` call anywhere in board; `replace` is genuinely demonstrated on
`user.view.tsx`'s profile tabs and `search.view.tsx`'s `useRouteSync`/`navigate`, which is
where this line should have pointed), `scrollToTop` (-> F15 pagination),
`skipWorkIndicator` (-> F17: background-ish links that must not flash the bar), `state`.

TS — read models: `RouteState` {href, historyState, clientBuildId, params, splatValues,
matches[], error} and `WorkState` {navigation, revalidation, prefetch, apiRequests[]} —
consumed via the hooks/ selectors throughout F15/F17.

Rust — `HeadBuilder` members: title/description/icon/meta_charset/
meta_name_content/meta_property_content/meta/link/script/style/preload plus attr helpers
(attr/bool_attr/property/name/content/rel/href/src/ r#type/charset/r#as/cross_origin) ->
F13; low-level defs (new/known_safe/add/append/elements/self_closing/dangerous_inner_html/
text_content/attr_exists) are the builders' own plumbing (exercised transitively).

Rust — `Document`/`DocumentAttributes`/`DocumentBuildCtx`: html()/body()/head() +
lang/id/class/data/attribute/ known_safe_attribute/boolean_attribute -> F13;
`head_dedupe_rules` (policy knob; exercised by defaults, documented);
`DocumentBuildCtx::{request, public_url}` -> F13: Board's document shell reads the current
request path for app-visible shell metadata and resolves public assets through the
committed manifest.

Rust — `FormData` members: fields/files/field/text/fields_named/texts/ file/files_named;
`FormField` {name, value}; `FormFile` {name, file_name, content_type, body, into_body} ->
F3 import/attachment.

Rust — tasks: `task!`, `Task::{id, run}`, `ParallelBatch::{new, add, run}`,
`ParallelBatchOutputs::take`, `Tasks::{new, exec_ctx}`,
`ExecCtx::{cancel_token, is_cancelled, child}` -> F10 (the story page should batch
independent reads with `ParallelBatch`; that is the task-runtime form that preserves typed
outputs and spawned sibling execution).

## FINDINGS LEDGER (the fussy developer's defect list)

Every friction met while building Vorma Board, filed — never silently absorbed. Severity:
DESIGN (needs a ruling) / PAPERCUT (mechanical).

- F-1 current resolution (vorma-tasks): task errors implement `From<E>`, so fallible task
  bodies and `TaskOverrides::replace` bodies can return application errors with `?` or
  `Into::into`. `repo::blocking_task` remains a database blocking adapter, not a task
  error-shim.

- F-2 DESIGN (vorma / vorma-tasks): every `Task::run` call site in a handler maps its
  error by hand — `.map_err(|e| ViewExit::err(e.to_string()))` — which both repeats
  boilerplate (six call sites in the F1/F2 slice alone) and FLATTENS the error chain to a
  string, losing the source chain the exit types were designed to carry. Want:
  `From<vorma::tasks::Error<vorma::Error>>` for `ViewExit`/`HttpExit` (preserving source),
  so `?` just works on task runs the way it already does on plain `vorma::Error`.
- F-2 RESOLVED (P008, 2026-07-01): landed exactly as wanted — concrete (not
  blanket-generic) `From<vorma_tasks::Error<crate::Error>>` impls on both exit types, in
  `crates/vorma/src/exit.rs`. `Failed` boxes the `Arc<crate::Error>` itself rather than a
  re-stringified copy, so a source attached to the application error stays reachable one
  more `source()` hop away — nothing flattened. The four payload-free runtime variants
  (`Cancelled`, `Cycle`, `MissingOverride`, `TypeMismatch`) get the exit's plain default
  form (their `Display` text as the server record, no source to attach). `Cancelled`
  specifically was verified against the engine rather than left an accident: it is only
  ever produced when a resolving `ExecCtx`'s cancellation token is already set, and this
  engine only ever cancels an invocation's context after its own output is already decided
  — either a losing same-phase sibling whose result the engine already discards
  positionally, or a next-phase invocation that never starts at all — so a still-mattering
  handler can never observe its own context cancelled; pinned by both a unit test
  (`exit.rs`) and an engine-level position-race test pair (`execution_engine.rs`). Twelve
  Board `Task::run`/`ParallelBatch::run` call sites converted from manual string mapping
  to bare `?`; one (`LAYOUT`'s site stats read in `examples/board/src/views.rs`) kept its
  explicit `.with_source(...)` form as the taught custom-message contrast.
- F-2b POSITIVE (recorded for contrast): `?` on plain `vorma::Error` returns inside
  exit-returning handlers is exactly right — repo write calls (`repo::login(...).await?`)
  needed zero ceremony.
- F-3 PAPERCUT (vorma::testing): `TestRequest::cookie` writes cookies nicely, but READING
  a Set-Cookie back means parsing the raw header string by hand (tests grew a
  `cookie_pair` helper). A small jar/continuation story on TestApp ("carry cookies from
  this response") would close the auth-flow loop tests obviously want.
- F-3 RESOLVED (P008, 2026-07-01): explicit-continuation shape, as ruled. Response side:
  `vorma::testing::TestResponseCookies::set_cookie_headers()`, an extension trait on
  `http::Response<Bytes>` returning parsed `vorma::HttpCookie` values (attributes
  included, not just a name/value pair). Continuation side: `TestApp::session()` returns a
  `TestSession`, stateless `TestApp` untouched (no hidden jar, no cross-session sharing),
  exposing the same request-building verbs (`get`/`get_view_payload`/`request_json`/
  `request`) the app does — `TestSessionRequest` delegates to the same underlying
  `TestRequest` a bare app call would build. The jar honors standard overwrite-by-name and
  real cookie clearing (`Max-Age <= 0` or an `Expires` in the past, exactly what
  `HttpCookie::make_removal` produces), verified with dedicated jar-accumulation,
  overwrite, clearing, and multi-cookie tests in
  `crates/vorma/tests/in_memory_test_app.rs` — including two tests pinning a merge bug
  caught during implementation review (a session request combining the jar with an
  explicit `.cookie(...)` addition must land as ONE `Cookie:` header, not two: `HeaderMap`
  only ever exposes the first of a repeated header name to real app code, so a second line
  would have silently hidden either the jar or the explicit addition). Board's hand-rolled
  `cookie_pair` helper is gone; all 20 `examples/board/tests/app.rs` tests read through
  `TestSession` where that teaches better. One test
  (`logout_clears_the_session_cookie_and_the_session_row`) deliberately stayed on the
  explicit, stateless `TestApp` `.cookie(name, value)` form instead of a session, because
  it exists specifically to replay a stale, already-cleared token — a session's own jar
  would have honestly forgotten that cookie the moment it saw the logout response, which
  would have silently weakened the test to something
  `anonymous_layout_has_no_session_user` already covers.

- F-4 POSITIVE: `Link`'s navigation-target union makes `href` + typed `search`
  UNREPRESENTABLE (href strings cannot typecheck search params) — the compiler pushed the
  app from `href="/"` to the honest `pattern="/_index"` form for paginated links.
  Unrepresentability doing its job; no change wanted.
- F-5 PAPERCUT (scaffolding, create-vorma's future job): a new app needs
  `src/client/vite.d.ts` copied by hand for CSS side-effect imports, plus the per-example
  `node_modules/vorma` symlink and a pnpm-workspace entry. All one-time setup that the
  (backlogged) create-vorma rewrite should own. Also fixed in passing: pnpm-workspace.yaml
  still listed examples/minimal post-rename.

- F-6 META (census method): the first census had a systematic blind spot one level below
  its sweep — nested members of returned objects and option bags (`workIndicator`,
  `toIdentityArray`, `useViewTransitions`, the observer callbacks, Link prop breadth all
  went uncensused until the maintainer surfaced them). Fixed by the member-level second
  sweep above; lesson: census the MEMBERS, not the exports.
- F-7 PAPERCUT (discoverability): `workIndicator`'s nprogress-shaped contract and
  `toIdentityArray`'s react-query purpose are invisible until you read source — names
  alone did not teach the integration pattern even to a motivated reader. The F17
  implementations become the canonical examples; docs must lead with them.

- F-8 DESIGN-ADJACENT (discoverability, with my own misuse as the evidence): Vorma
  mutations AUTO-revalidate route data by default — pinned in router_submit tests, opt-out
  via `revalidate: false` on the args — and I did not know it: the F1/F2 board code
  hand-called `revalidate()` after `mutateOrThrow` (layout sign-in/out, front-page vote),
  which double-schedules. FIXED with F17: the `useApiMutation` conversion deleted every
  manual call; route data refreshes purely through vorma's auto-revalidation. Docs must
  state the default loudly, and "manual revalidate() after a mutation" belongs in a
  do-not-do-this docs box.

- F-9 PROCESS (recorded so the docs era can learn from it): the theme thread produced
  wrong proposals before settling. The durable lessons: (a) do not invert dependency
  direction — generic packages must never know about specific backends; (b) names must say
  what the value actually is; (c) when the gap is "JS runs too late," the user-facing
  contract is the documented pre-paint HTML snippet. Current state: the TS kit documents
  the snippet and owns the generic theme API; Vorma exposes `vorma::kit::theme::script` as
  the Rust document-builder convenience for injecting that documented snippet.

- F-10 RETRACTED (was: typed-args family gap): both the finding and the `(A, M, P)`
  narrowing params it spawned were scaffolding for a WRONG wrapper idiom (explicit
  per-route generics at call sites); reverted. The contract was then DERIVED
  maintainer-led and landed:
    - RULING — GET is implicit only for QUERIES. Mutations always name their method (the
      kind-overridden GET-mutation optionality was fixed; ResolvedResourceKind now gates
      the method field in args construction; typecheck pins flipped, incl. an
      @ts-expect-error pinning omission as illegal).
    - RULING — wrapper contract: queries take FULL args at the hook (args ARE the cache
      identity; queryKey = toIdentityArray); mutations take ENDPOINT IDENTITY ({method,
      pattern}) at the hook and mutate(whatever-varies-at-the-call-site) — params/input as
      react-query TVariables; one hook serves a list.
    - Framework deltas (both landed): exporting `ApiClientOutput<A, Args>` (core + all
      three adapters), and — maintainer-ratified follow-up — per-route narrowing as
      OPTIONAL params on the existing symbols: `ToQueryArgs<A, M?, P?>` /
      `ToMutationArgs<A, M?, P?>`, matching the family's bare (A, M, P) keying (no new
      names; the wrapper's Extract moved into the framework). Mutation filter is exact
      ({method; pattern}) since mutations always name their method; the query filter keeps
      the optional-method arm. Pinned in typecheck.ts incl. the PATCH-vs-POST
      "/users/:userID" disambiguation; board's local alias deleted in favor of
      `ToMutationArgs<V, M, P>`.
    - Landed in board: `useApiMutation` (const type params for literal inference; the
      single reassembly cast lives in the wrapper), sign-in/out + vote converted
      (isPending, envelope text via error.message); `useApiQuery` ships with F9 search per
      no-dead-surface.

- F-11 APP BUG + A WORSE FRAMEWORK "FIX", REVERTED: before API mount removal, the mod
  gate's scope patterns omitted the api mount, so the gate missed the /mod RESOURCES
  (their URLs were /api/mod/...). That was an app-authored pattern mistake — the fix was
  writing the mount where the URL carried it
  (`["/mod", the /mod splat, the /api/mod splat]`). The attempted framework fix
  (mount-stripping resource paths before scope matching) was REVERTED as strictly worse:
  it made one pattern silently denote two URL spaces, made views-only vs resources-only
  scopes unrepresentable, and traded four typed characters for a hidden rewrite. RULING
  REAFFIRMED: scope patterns are URL patterns, one observable space, no rewriting — pinned
  by `scope_patterns_match_urls_with_no_hidden_rewrites` (the splat scope without the
  mount does NOT cover the api path; the one with it does). The follow-up question was
  whether the api mount concept should exist at the framework level at all; it was
  resolved by the next bullet.
    - RESOLVED — THE API MOUNT IS DEAD (maintainer-ratified, landed): resources declare
      full URL patterns (`/api/...` is the app's own spelling, not framework structure),
      and the one-observable-space ruling now covers every route kind. The enabling
      primitive is `vorma_matcher::find_overlap(a, b) -> Option<Overlap>` — exact overlap
      detection between any two matchers (the dual-mode `Matcher` was split into
      `FlatMatcher` / `NestedMatcher` typed instances first, per maintainer intent
      matching the Go Router/NestedRouter shape). find_overlap is correct by construction:
      a provably complete finite candidate family (bounded lengths, trailing-slash counts
      0–2, literal-pinned positions, a placeholder symbol fresh against both matchers) is
      evaluated by the real matchers as built, with a brute-force enumeration oracle
      pinning completeness across all four type pairings.
    - DOCTRINE CORRECTED BY THE MAINTAINER, twice, before this settled: my first
      validation treated any view/resource intersection as a conflict ("one owner per
      path"), then I tried to special-case only the root catch-all. Both were the same
      category error: the matcher's organizing principle is SPECIFICITY — overlap is the
      normal condition, adjudicated by one total order (segment score, then the leftmost
      differing rank, then splat-last loses, then length), and the only illegal state is a
      TIE on a shared path, i.e. identical shape, the very thing in-table registration
      already rejects as a route shape collision. The order now has one public home,
      `vorma_matcher::compare_specificity(a, b) -> Ordering` — `better_than` delegates to
      it and the walk-time score plumbing was deleted (DRY: one ordering, nothing to
      drift), with a pinned agreement oracle proving dispatch-by-comparison equals the
      matcher's own pick.
    - The landed semantics: views and GET/HEAD resources share one URL space. Graph
      compile errors only on a cross-table specificity tie (`ResourceViewSpecificityTie`,
      witness path + both patterns), so view `/s/:story_id` + GET `/s/export` coexist (the
      static owns its path), GET `/bob/sally` lives under view `/bob/*` (the splat takes
      the rest), and a root catch-all view coexists with GET resources with no special
      case at all — the floor of the order loses to everything by construction. The
      validation axis is the HTTP method, never ResourceKind: a GET-method kind=Mutation
      tie still errors, a POST-method kind=Query resource at a view URL is untouched
      protocol. classify() adjudicates GET/HEAD requests by comparing the best view claim
      against the best resource match with the same public order (pinned in all three
      directions, including a static view beating a dynamic resource). The public static
      base stays a reserved prefix (`ResourceInsidePublicStaticBase`, the successor of the
      static-under-api config rule — a space partition for manifest paths, not a
      specificity question; root base keeps its historical opt-out). The mount-partition
      terminals (`ApiNotFound`, `UnsupportedApiMethod`) folded away — 405 is computed per
      path (views/assets contribute GET/HEAD to Allow) and every 404 is the
      framework-finalized empty response with the client build id header. `api_base` is
      gone from app config, the manifest, live-state (protocol bumped to 3), the
      projection bundle, and the TS client (`build_resource_url`, `toIdentityArray` keys,
      and the generated seed lost their mount parts). Board/notes/fixtures migrated;
      board's 17 TestApp tests pass byte-identical requests against the full-URL
      declarations.
- F-12 MATCHER FACT (recorded, no change): splat patterns match one-or-more segments,
  never their bare root — bare "/docs" does not match the "/docs" splat form. The honest
  structure is a nested pair (a plain "/docs" index parent + the splat child rendering in
  its outlet), which the board now demonstrates; docs-era material should state this fact
  next to the splat docs.
- F-13 OBSERVATION (updated after the catch-all correction in F-15 and the later
  response-protocol ruling): the branded "not found" story is a root catch-all view, but
  it is an app-level fallback, not an HTTP 404. On a dead URL with no app fallback the
  framework sends a finalized empty 404 (status + client build id header, no body). An app
  wanting branded content declares a "/\*" view; once that view claims the path, the route
  is found and the view renders normally with HTTP 200. This is spiritually similar to an
  SPA router fallback page whose UI says "404" while the document request itself succeeds.
  The catch-all coexists with GET/HEAD resources by construction (the floor of the order
  loses every shared path to every other route — pinned at graph compile and in dispatch).
  Two docs-era facts to state next to the idiom: (1) the catch-all yields only to a
  covering match — a chain that actually completes through the path. A dead URL under a
  matched-but-uncovered view prefix (e.g. /foo/bar with a /foo view and no covering child)
  gets the branded fallback: the dead prefix drops out and the catch-all claims the path
  (pinned in `catch_all_takes_paths_where_no_chain_completes` and
  `nested_catch_all_yields_only_to_covering_matches`; the pre-correction behavior — prefix
  hits suppressing the fallback — was one of the Go-inherited bugs F-15 records). (2)
  splat fallback views must not become inferred layout parents for every other view;
  generated client metadata should list the catch-all under `/`, not list `/*` as a parent
  of ordinary routes. (3) views have no HTTP status surface. Do not tell future agents to
  set a 404 status from a view; resources, middleware, and framework faults own HTTP
  status semantics.
- F-14 GRAMMAR DIVERGENCE FROM GO (recorded; tightening is correct): the Go matcher
  accepted empty and duplicate dynamic param names ("/a/b/:", "/f/g/h/i/:/:",
  "/j/k/l/m/n/:_/:_") and its fixtures pinned the resulting collapse semantics (params
  keyed "" and last-wins value loss). The Rust port rejects all of it at registration
  (non-empty, valid-identifier, unique names — pinned in the grammar suite) but the
  divergence lived only in a silently rewritten test fixture until the matcher review
  surfaced it. The tightening stands on the merits: the typed TS client requires valid,
  unique param identifiers, and last-wins collapse is silent data loss, not a feature.
- F-15 MATCHER CORRECTIONS FROM GO (maintainer-ratified as corrections, not divergences:
  bugs the original Go missed and would have fixed had it seen them; each was proven with
  a red pin before the fix, and the property model re-derives all of them independently):
  (1) catch-all cover rule — a prefix hit is not a match; the root catch-all yields only
  to an entry whose chain completes through the path, so dead prefixes drop out and the
  branded fallback claims the path (see F-13). (2) Dirty-path rule — at most ONE trailing
  slash is tolerated as noise; an empty segment never matches anything, so a doubled slash
  anywhere means no match, and params and splat values never contain empty strings; the
  root catch-all matches the root path with empty splat values. (3) The
  `params.is_empty()` gate in nested flatten was unreachable in both implementations and
  is deleted. (4) Index patterns claim their parent path on their own shape — a dynamic
  index registered without its non-index sibling (an index file under a dynamic directory
  with no layout) now matches; Go required the parent registration for dynamic indexes
  while accepting the static equivalent, an asymmetry with no principled basis (probed
  empirically against the Go binary; pinned in
  `nested_index_claims_its_parent_path_without_the_parent_registered`). Same review's
  performance verdict: after restructuring (registered patterns read straight off tree
  nodes, statics riding the tree in nested matching, walks tracking pattern refs with
  captures rebuilt positionally at emit, inline-capacity stacks and candidate lists,
  shared param-name keys, single-buffer splat captures), every benchmark row beats the Go
  baselines by ~1.2–2.0×; the public matcher surface remains std/vorma-owned types only,
  with all storage choices internal.

- F-16 COVERAGE (P003, resolved): the third task cache policy, `single_flight`, had NO
  real-world consumer anywhere in the repo — only the `vorma-tasks` crate's own tests.
  Board demonstrated `memoized` and `extended_cache` but not `single_flight`. Closed by
  P003: `repo::LIVE_SITE_STATS` is a `single_flight` task backing a live site-activity
  counter in the shell footer (coalesce concurrent duplicates, retain nothing — the honest
  shape for a volatile counter). The full cache-policy trio now lives in `repo.rs`.

- F-17 DESIGN (P003, escalated to the maintainer): the standalone `vorma::tasks`
  runtime-lifecycle surface has no honest Board app-flow home. An HTTP app never
  constructs its own `Tasks`/`ExecCtx`/`CancelToken` — the framework owns the runtime and
  hands `ExecCtx` to handlers — so `Tasks::{new,exec_ctx}`, `CancelToken`,
  `ExecCtx::{child, cancel_token,is_cancelled}`, `Task::id`,
  `Clock`/`SystemClock`/`ClockInstant`, `TaskOverrides`/`TaskOverrideMode`, and the
  `TaskObserver` family cannot be covered by Board without contrivance. These are
  exercised by the sovereign-crate suites (`crates/vorma-tasks/tests/tasks.rs`) and the
  framework-owned usability test (`crates/vorma/tests/public_api.rs`), and `TaskOverrides`
  was already deliberately MOVED out of Board into
  `crates/vorma/tests/in_memory_test_app.rs`. This exposes a tension in the literal "Board
  must cover 100%, period" rule versus the maintainer's own placement decisions.
  Position/recommendation: ratify in `docs/maintainer/board-example/README.md` that Board
  covers the surface an application uses in a realistic app flow, while the standalone
  tasks runtime-lifecycle surface and low-level head/document type carriers are
  sovereign-crate + `public_api.rs` coverage, not Board coverage. Full detail and the
  item-by-item table are in `INVENTORY_VS_BOARD_P003.md`. The one genuinely app-shaped
  member of this set is a slow-task-logging `TaskObserver` wired into the server main's
  `TasksOptions.observer` — a real telemetry feature worth a follow-up packet if the
  literal rule stands.
- F-17 RESOLVED (P006, 2026-07-02, per the maintainer's framework-author-vs-app-useful
  ruling and its 2026-07-02 "contrived is never grounds for exemption" restatement): the
  runtime-lifecycle surface this line flagged as having "no honest Board app-flow home" is
  app-useful, not framework-author-only, and now has one. Three landed teaching rows:
    - `Tasks::{new,exec_ctx}`, `CancelToken` (`new`/`cancel`/`is_cancelled`/`cancelled`/
      `child`): `examples/board/src/maintenance.rs` (`run_session_pruner`) — a background
      worker the server main (`examples/board/src/bin/server.rs`) spawns alongside axum,
      constructing its OWN `Tasks<vorma::Error>` runtime (independent of the framework's
      request-serving one) and opening a fresh `ExecCtx` per sweep via `Tasks::exec_ctx`,
      doing real maintenance (pruning `sessions` rows past a retention window via a
      `task!`), wired to graceful shutdown through a `CancelToken` cancelled at the same
      `select!` point axum stops accepting connections.
    - `ExecCtx::{is_cancelled,child}` (+ `cancel_token` as it appears on the parent):
      `examples/board/src/repo.rs` (`MOD_EXPORT_SCAN`), exposed via a new mod-only
      resource `examples/board/src/resources.rs` (`MOD_EXPORT`, `POST /api/mod/export`)
      and a client trigger in `examples/board/src/client/views/mod.view.tsx`. A
      moderator-triggered bulk audit scan over every story checks `ctx.is_cancelled()`
      once per chunk and returns whatever it already gathered (`complete: false`) instead
      of running to completion unconditionally; each chunk resolves its stories' comment
      counts through `ctx.child()`, a fresh child execution context per chunk.
    - `TaskObserver` family + `Task::id`: `examples/board/src/maintenance.rs`
      (`SlowTaskObserver`, `slow_task_observer`) — a slow-task-logging observer matching
      `TaskEventKind::RunCompleted` past a threshold and logging task name +
      `Task::id()` + duration, wired into `TasksOptions.observer` in
      `examples/board/src/bin/server.rs` for BOTH the framework's request-serving `Tasks`
      runtime and the standalone worker's runtime from one shared instance. Exempt surface
      (per the ruling, unchanged by this packet): `TaskOverrides`/ `TaskOverrideMode`
      (prior maintainer ruling: framework test suite home, deliberately moved out of Board
      — F-20) and `Clock`/`SystemClock`/`ClockInstant` (determinism-injection tooling; a
      real app never overrides it) stay sovereign-crate + `public_api.rs` coverage; not
      contrived into Board. `docs/maintainer/board-example/ README.md`'s durable coverage
      rule already states this split; no further README change needed by this packet.

- F-18 PAPERCUT (P003, gate friction, recorded): `make ts-fmt` runs `oxfmt --write .`, and
  oxfmt's `proseWrap: "always"` / `printWidth: 90` reflows ALL markdown — including the
  maintainer docs under `docs/maintainer/` that had drifted from that config. Running the
  full `ts-fmt` write mode therefore dirties 13 out-of-scope, Fable-owned files (STATE.md,
  ROADMAP.md, LEARNINGS.md, packet REPORT/REVIEW/INSTRUCTIONS, three tickets), so
  `make ts-fmt-check` (and thus the aggregate `ts-gate`) is red at HEAD independent of any
  code change. An executor scoped to Board must NOT run `oxfmt --write .` globally and
  must use `--check` or a scoped write. Recommendation: either normalize the maintainer
  docs once through the oxfmt config (a Fable-owned housekeeping pass, out of scope for a
  Board packet), or exclude `docs/maintainer/**` from oxfmt if the intent is that
  hand-maintained process docs are not machine-prose-wrapped.

- F-19 (P005, census-accuracy correction, recorded so the pattern is visible): two
  descriptive lines in this census claimed features that never existed in current code. F1
  said "cursor/page pagination"; board's pagination is page-only
  (`FrontInput { page: Option<i64> }`, `examples/board/src/views.rs`) and no cursor
  concept exists anywhere in the app. The `Link` member sweep attributed `replace` to
  "post-login"; sign-in's `onSuccess` handler only clears the username input and never
  navigates — `replace` is genuinely demonstrated on the profile tabs
  (`examples/board/src/client/views/user.view.tsx`) and the search box's
  `useRouteSync`/`navigate` calls (`examples/board/src/client/views/search.view.tsx`).
  Both corrected in place at F1 and the `Link` props line above. Neither was a code gap;
  both were the census's own prose drifting from the app it describes. Full detail:
  `CENSUS_COMPLETION_P005.md`.
- F-20 (P005, census-accuracy correction): the tasks-member-sweep line and "Prior
  Adjudication List" item 2 both claimed board's F16 suite carries a
  `TaskOverrides::replace` failure-injection test. It does not and never did — that test
  lives in `crates/vorma/tests/in_memory_test_app.rs`, consistent with F17's own later
  ruling that `TaskOverrides` was deliberately moved out of Board. The claim was simply
  never updated after that move; corrected in place at both locations.
- F-21 PAPERCUT (P005, found, not fixed — escalated):
  `DocumentAttributes::known_safe_attribute`/`boolean_attribute`
  (`crates/vorma/src/document_builder.rs:158, 168`) have no call site in board and no call
  site in `crates/vorma/tests/public_api.rs` (which exercises only `lang`/`data`/`class`,
  the plain-`attribute()` wrappers). These are distinct from `HeadBuilder`'s own
  `known_safe`/`bool_attr`, which board does exercise transitively through `.script()`/
  `.link()`. No non-contrived real-app use for a boolean or trusted-unescaped attribute on
  the root `<html>`/`<body>` element presented itself during the P005 audit — inventing
  one (e.g. a functionless boolean flag) would violate the board contract's own rule that
  a feature added purely to touch an API must still honestly explain when a real app would
  use it. Recommendation: fold into `public_api.rs`'s existing
  `root_document_helpers_are_usable_externally` test as a type-usability check (matching
  how `HeadBuilder`'s equally low-level defs are already handled one test above it), since
  this pair is genuinely plumbing-shaped rather than app-flow-shaped. Not actioned in
  P005: deciding the coverage home is a judgment call beyond "small mechanical gap, same
  pattern as existing code," and `public_api.rs` sits outside `examples/board`.
- F-22 PAPERCUT (P005, found, not fixed — escalated): `FormData`'s multi-value/grouped
  accessors (`fields`, `fields_named`, `texts`, `files`, `files_named`) and
  `FormFile::into_body` (owned-body extraction, vs. the borrowed `body()` board already
  uses) have no call site in board. The current submit form only ever needs one text field
  read at a time (`text(name)`) and one optional attachment (`file(name)`), so there is no
  honest single-attachment-shaped way to reach the grouped accessors. Recommendation:
  revisit if/when a future packet adds multi-attachment support to submit — a real product
  feature, not a same-pattern mechanical addition to current code, so it was not forced
  into this packet.

- F-21/F-22 RULINGS (Fable, P005 review, 2026-07-01): F-21 — `known_safe_attribute`/
  `boolean_attribute` are discharged by `crates/vorma/tests/public_api.rs`; coverage added
  as a P008 rider. F-22 — accepted as recorded: the multi-value `FormData` accessors get a
  board home only if/when multi-attachment submit becomes a real feature; no contrived
  call site.
- F-21 RIDER LANDED (P008, 2026-07-01): `root_document_helpers_are_usable_externally`'s
  test suite gained a companion test exercising both helpers through the real public call
  site (`vorma::DocumentAttributes::known_safe_attribute`/`::boolean_attribute`) and
  asserting the documented rendered HTML forms — a known-safe value renders unescaped, a
  boolean attribute renders bare with no `="..."` at all — via
  `vorma_contract::document_renderer::render_document` on a contract built from the
  identity the real call produced, so the check proves the actual wiring rather than
  restating the flags. No new `vorma` public surface was needed: `vorma_contract`'s
  renderer and contract types were already a direct dependency.

- F-22 RULING CORRECTED (Fable, 2026-07-02, superseding the prior line): the earlier
  "revisit only when a real feature exists" ruling misapplied the standing policy — the
  census itself sanctions ADDING OR INVENTING a realistic board feature to cover an API,
  and the multi-value `FormData` accessors are app-useful primitives under the F-17 test.
  Board owes them a home: multi-attachment submit (repeated file field + a
  repeated-checkbox tag group for `texts`), authored as packet P009.

- F-21 RULING CORRECTED (Fable, 2026-07-02, superseding the P005-review line): same
  category error as F-22 — `DocumentAttributes::boolean_attribute` and
  `known_safe_attribute` are app-facing document APIs, so board owes them call sites
  (landing in P009, document shell, with trust-boundary teaching); the `public_api.rs`
  usability coverage (P008 rider) stays as additional coverage, not a substitute. The
  governing principle is now stated with force in the board README (2026-07-02): contrived
  is never grounds for exemption; board is a teaching tool.

- F-21 LANDED (P009, 2026-07-02): `examples/board/src/document.rs:15-43` (the shared
  document shell, every HTML/JSON response) calls both.
  `.boolean_attribute("data-server-rendered")` renders name-only on `<body>`, teaching the
  HTML sense of "boolean attribute" against the value-carrying `.data(...)` call right
  above it; a real client CSS/JS hook can target `[data-server-rendered]` to tell a hard
  load from a client-committed route, since the document builder only ever runs for a
  genuine server response.
  `.known_safe_attribute("data-built-with", format!("{APP_NAME} & Rust"))` teaches the
  escaping trust boundary directly at the call site — comments explain when bypassing
  escaping is legitimate (a compile-time-owned literal, never user input) and why it is
  dangerous otherwise (an unescaped `"` breaks out of the attribute). Proven end-to-end,
  not just unit-level: `examples/board/tests/app.rs`'s
  `document_shell_renders_the_boolean_and_known_safe_body_attributes` fetches the real
  page (`app.get("/")`, full HTML, not the JSON view-payload path every other board test
  uses) and asserts the exact rendered `<body>` byte-for-byte — the bare
  `data-server-rendered` with no `="..."`, and the literal unescaped `&` in
  `data-built-with="Vorma Board & Rust"` — through the app's actual document builder, its
  actual runtime pipeline, and the actual renderer, matching the trust-boundary claim
  live-verified in a real browser session during this packet's own manual check (the
  parsed DOM decodes `&amp;` back to `&` either way, which is why the test asserts on raw
  response bytes rather than `Element.getAttribute`). The `public_api.rs` usability
  coverage from the P008 rider is untouched and stays as additional, builder-level
  coverage per the corrected ruling above.

- F-22 LANDED (P009, 2026-07-02): every member of the multi-value family gets its own
  direct, individually-commented call site in `examples/board/src/resources.rs`'s
  `SUBMIT_STORY` handler — "no honest home" was never invoked; the feature (multiple
  attachments per story plus a repeated tag checkbox group) was designed so each accessor
  is the natural way to write the code that needed writing, per the packet's own design
  mandate.
    - `fields()` (`resources.rs:151-163`): a blanket sweep rejecting any submitted text
      field over a byte cap, regardless of name — closes a real pre-existing gap (the
      story `body` field had no length cap at all) while demonstrating the
      field-name-agnostic accessor's natural use: never assume a client sends only the
      fields the handler expects.
    - `fields_named(name)` (`resources.rs:194-198`): a shape check on the `tag` group
      (rejects more `tag` fields than checkboxes exist) — distinct from `texts` below by
      reading the `FormField`s themselves for a property of the group, not their values.
    - `texts(name)` (`resources.rs:207-212`): extracts the checked tag values from the
      same `tag` group, filtered against the fixed vocabulary (`resources.rs:15`,
      `STORY_TAGS`, also exported to TypeScript so the checkbox UI and server validation
      share one source).
    - `files()` (`resources.rs:221-238`): the aggregate view enforcing a submission-wide
      attachment count cap and combined byte cap — a property of the whole submission, not
      any one field name.
    - `files_named(name)` (`resources.rs:258-279`): the multipart repeated-file loop
      itself — one `attachment` field per selected file under
      `<input type="file" multiple name="attachment">`, the shape a repeated file field is
      actually built on.
    - `FormFile::into_body` (`resources.rs:271`): consumes an owned clone of each matched
      file to hand its body to the storage write by value, with a comment explaining the
      honest reason to reach for it (`ctx.input()` only ever lends a borrow, so owning a
      file at all means cloning first; `into_body` then avoids a second borrow-and-copy
      through `.body()` on top of that). Every validation rejection path (oversized field,
      too many tags, unknown tag value, too many attachments, attachments too large
      together) and the multi-file/multi-tag success path are covered by dedicated tests
      in `examples/board/tests/app.rs` (`submit_rejects_a_field_value_that_is_too_long`,
      `submit_rejects_more_tag_fields_than_the_vocabulary_has`,
      `submit_rejects_a_tag_outside_the_known_vocabulary`,
      `submit_rejects_too_many_attachments`,
      `submit_rejects_attachments_that_together_exceed_the_byte_cap`,
      `submit_tags_round_trip_through_the_checkbox_group`, and the rewritten
      `attachments_upload_with_the_story_and_download_back`, which now submits two files
      under one field name, downloads each independently by its own attachment id via the
      new nested route `/api/stories/:story_id/attachments/:attachment_id`, and confirms a
      valid attachment id 404s under the wrong story id). Manually verified live in a real
      browser session during this packet (sign in, check two tag boxes, attach two files,
      submit, confirm both attachments list with independent per-item load/download state
      and the tags render on the story page) in addition to the automated suite. Storage:
      board's own SQLite schema gained its own `id` column on `attachments` (was one row
      per story, now one row per file, `examples/board/src/schema.sql`) and a new
      `story_tags` join table — board owns its schema, no framework change. The
      pre-existing single-value accessors (`FormData::field`/`FormData::file`) remain
      exercised as before via `text("title")`, `text("url")`, and `text("body")`
      (`resources.rs:165-167`).

- F-23 DESIGN (vorma middleware, found running board's real production server, P007
  2026-07-02): `vorma::middleware::etag()` composed after (inner to)
  `vorma::middleware::response_body_timeout()` in a `tower::ServiceBuilder` chain silently
  produces no ETag on any response, ever, regardless of body size or content. Root cause:
  `response_body_timeout`'s underlying `tower_http` body wrapper never forwards the
  original body's exact `size_hint`, and `etag` only tags a response when it can see an
  exact size. Confirmed on board's actual running prod server before the fix (every 200
  response had a correct `Content-Length` but no `ETag`) and via a minimal isolated
  reproduction pinpointing `response_body_timeout` as the sole cause among every other
  layer in the stack. Board's own fix (declare `response_body_timeout` before, not after,
  `etag`, with a teaching comment) is landed and verified end to end: a strong ETag is now
  generated for both a 223-byte and a 277KB static asset, and a follow-up `If-None-Match`
  request correctly returns `304 Not Modified`. The application-level fix is sufficient
  for board's own stack but does not close the underlying footgun — another app (or a
  future board edit) can silently reintroduce it by reordering two `.layer(...)` calls,
  with no warning, no lint, and no test failure. Full root-cause trace, reproduction
  transcript, and four ranked remediation options (documentation only / make `EtagLayer`
  resilient to an unknown size hint within its existing body cap / fix `tower-http`'s
  `TimeoutBody` upstream / offer a pre-ordered composed helper) are in ticket
  `etag-response-body-timeout-ordering-footgun`. No option is recommended over the others
  beyond ruling out documentation alone, which conflicts with this project's own
  no-footgun-smoothed-by-docs rule; the maintainer's call.

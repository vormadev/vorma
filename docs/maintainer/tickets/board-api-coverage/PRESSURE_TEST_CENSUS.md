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

- F1 Front page `/_index`: score-ranked story list, cursor/page pagination via typed
  search params, story rows with vote buttons, relative timestamps, domain extraction.
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
- F17 Ecosystem integration (maintainer-ratified expansion; LANDED except the parts whose
  homes arrive with later features — `useApiQuery`/`apiQueryOptions` ship with F9 search
  and `workIndicator.track` ships with the F3 import parse, so no dead surface ever
  exists): the app runs popular libraries against Vorma's lifecycles, as applications will
  — _ nprogress driven by the `workIndicator` client option
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
  (`apiQueryOptions(args)`) stays exported separately for prefetchQuery/ensureQueryData
  composition; _ jotai + `kit/theme`: the TS kit stays generic and owns the theme
  state/storage API. For flash-free first paint, the TS kit docs show the exact HTML to
  inline, and Vorma provides `vorma::kit::theme::script(None)` as the native Rust document
  injection convenience for that same snippet. The important boundary is that the TS kit
  does not depend on Vorma; the Rust helper is a Vorma-app convenience, not a separate
  theme system. Board's jotai atom wraps `initTheme()` + `addThemeChangeListener`
  (cross-tab for free); _ `useViewTransitions: true` on the client (View Transitions on
  navigations). Division-of-responsibility is the deliverable: route/view data is Vorma's;
  ad-hoc API data is react-query's; React state is jotai's; persistence/broadcast is
  kit's; the bar is nprogress's.

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
  `TasksOptions` field surface; F16 failure-injection test via `TaskOverrides::replace`.

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
   (deterministic test time, task-body substitution). Keep; mapped —
   `TaskOverrides::replace` gets a failure-injection test in F16.
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
links), `replace` (-> F15 post-login), `scrollToTop` (-> F15 pagination),
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
- F-2b POSITIVE (recorded for contrast): `?` on plain `vorma::Error` returns inside
  exit-returning handlers is exactly right — repo write calls (`repo::login(...).await?`)
  needed zero ceremony.
- F-3 PAPERCUT (vorma::testing): `TestRequest::cookie` writes cookies nicely, but READING
  a Set-Cookie back means parsing the raw header string by hand (tests grew a
  `cookie_pair` helper). A small jar/continuation story on TestApp ("carry cookies from
  this response") would close the auth-flow loop tests obviously want.

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

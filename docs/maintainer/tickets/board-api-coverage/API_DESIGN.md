# Vorma Public API Design Pass

Task-local context for `__TICKET.md`. This file records the API design pass that produced
many current rulings, but it is not the live plan. Reconcile every claim against current
code before relying on it.

Full-surface pass (not example-driven) over every public item in the Rust crates and the
TS package, evaluated against the ruled principles. Census inputs:
[api_inventory_rust.txt](api_inventory_rust.txt),
[api_inventory_ts.txt](api_inventory_ts.txt). The error/response protocol is one section
here; its locked rulings live in
[RESPONSE_PROTOCOL_DESIGN.md](RESPONSE_PROTOCOL_DESIGN.md).

## Current reconciliation status, 2026-06-23

This file is historical design context, not the live plan. The current forward plan lives
in `__TICKET.md` and sibling tickets.

- The old "example expansion" section started from the earlier pre-Board example
  workout. Current truth is that `examples/board` is the canonical pressure-test app.
- The old kit carve-out from the coverage pass is stale under the current 100% public API
  coverage rule. Current Board covers public kit entry points as app utilities; kit still
  must remain generic and not know about Board or Vorma backend internals.
- The response/exit protocol rulings in this file landed; do not treat the older
  `RESPONSE_PROTOCOL_DESIGN.md` "Open rulings" heading as live.
- The initial findings here are not all open:
    - F1 raw identifier naming landed.
    - F2 hand-rolled example fixture friction is resolved for current Board request tests;
      current Board uses `vorma::testing::TestApp`.
    - F3 config reshape landed.
    - F4 positional `ResourceKind` plumbing was accepted as macro-target plumbing after
      the const-builder attempt failed Rust inference.
- The current Board server/request slice exists and has focused `TestApp` coverage. The
  browser/Vite/client breadth described later in this file is not current Board truth; the
  current Board browser/client coverage snapshot lives in `frontend-client-coverage.md`.
- `create-vorma` remains out of this ticket and is tracked by `../create-vorma-rewrite/`.

## Principles (user-ruled, binding)

1. Semantic coherence and unrepresentability beat "simple to learn" — every time. If sugar
   is tempting, the normal API is wrong.
2. Different semantics → different types. Types track semantics, not call sites.
3. Nothing reaches the client unless explicitly marked client-facing.
4. Views are a framework-owned rendering protocol, not HTTP documents.
5. Names must not imply what isn't true ("msg" implying transmission). Corollaries
   (user-ruled 2026-06-11): a field name must answer "what do I put here?" standalone — so
   semantic words are NEVER dropped (`server_config: ServerConfig` is correct; `server:`
   would claim to hold a server; type/field "stutter" is two independent true statements).
   WITHIN a name, prefer canonical abbreviations when unambiguous in register (`err`,
   `msg`, `dir`, `src`, `cmd`): abbreviation shortens words, never removes them. Register
   overrides exist in both directions: std's `source()` stays (std name; `src` means file
   paths here), ecosystem terms stay (`errorBoundary`), and `config` never abbreviates to
   `cfg` in Rust (collides with `#[cfg]`).
6. Wire details must be coherent and non-misleading; otherwise they are under-the-hood.
7. Two-register TS naming: camelCase for the user-facing surface, snake_case for
   adapter-facing internals.
8. Consistency across the system is itself an API property: a reader should predict the
   API they haven't seen from the API they have.

## Cross-cutting conventions (the consistency story)

**Verified holding:**

- Rust ctx read surface is uniform across View/Resource/Middleware:
  `input() / state() / params() / param() / splat_values() / request() / public_url() / head() / response() / exec_ctx()`.
- Rust builder verbs: `with_*` consumes-and-returns (Error builders, TestAppBuilder),
  `set_*` mutates a handle (ResponseHandle). Holds everywhere checked.
- The TS `To*` type-helper family (`ToViewPattern`, `ToNavigateArgs`, `ToQueryInput`,
  `ToMutationOutput`, `ToClientLoaderArgs`, `ToDefineViewArgs`, …) is uniformly named and
  uniformly keyed on `<A extends AppConfig, P extends pattern>`. This is the strongest
  consistency asset in the TS surface — protect it.
- Two-register TS naming holds INCLUDING the spots I suspected it didn't: `defineView`
  accepts camelCase (`clientLoader`, `errorBoundary`, `beforeRouteCommit`,
  `runClientLoaderOnHmr`); snake_case `ViewDefinition` is the internal normalized shape.
  Hook family (`useViewData` / `usePatternViewData` / `useClientLoaderData` /
  `usePatternClientLoaderData` / `useRouteState` / `useWorkState` / `useRouteSync`) is
  uniform across all three adapters; adapter deltas are honest (solid signals / preact /
  react).
- Package entry points: there is NO root `"vorma"` export. Users import
  `vorma/react|preact|solid`, `vorma/vite`, `vorma/kit/*`; `vorma/__internal` is the
  honestly-named adapter contract (an earlier scrutiny note claiming root exports existed
  was wrong — corrected).
- The vite plugin's public surface is a zero-options default export. Correct and final.

**Broken / inconsistent (findings):**

- F1 (naming): `HtmlAttribute::type_("module")` (trailing underscore) vs
  `head.r#type("font/woff2")` (raw ident) — the SAME concept spelled two ways in one
  head-building session. Also `head.r#as(...)`. One convention must win across the whole
  surface.
- F2 (testing): the example app's tests hand-fabricate a manifest + on-disk public outputs
  (~90 lines of fixture plumbing) instead of using
  `vorma::testing::TestApp::builder().with_public_asset(...)`, which exists for exactly
  this. Either TestApp can't express what the example needs (testing-API gap to close) or
  the example is teaching the hard way (example bug). Investigate, then fix whichever is
  true.
- F3 (config paths): `root_dir: PathBuf` but every other path-ish config field is
  `String`. RESOLVED BY RULE: root is the absolute anchor (`PathBuf`); all other config
  paths are root-relative `String` fragments — document in field docs.
  `public_static_src_dir` is CORRECT as-is under the abbreviation-first rule (an earlier
  draft proposed un-abbreviating; reversed by user ruling). The AppConfig field names
  (`server_config`, `tasks_options`, …) are CORRECT as-is under the answers-what-goes-here
  rule (a de-stutter proposal was considered and withdrawn).
- F4 (semi-public plumbing): `Resource::from_static(method, pattern, None, …)` takes a
  bare positional `Option` (the search-schema slot). Macro-emitted code is the only
  intended caller, but the constructor is reachable; a bare `None` in a signature is
  exactly the kind of unreadable-at-call-site shape principle 8 forbids. Low priority;
  align when the declaration layer is next touched.

## The error / exit protocol (per-protocol types — the centerpiece)

Locked by prior rulings: separate types for view vs HTTP-boundary errors; no status
surface on views; explicit-only client text; rejection travels ONLY as a returned value;
resource errors become a JSON envelope the TS client parses (killing `res.statusText`
reading); the interim single `vorma::Error` is scaffolding.

Proposed concrete shape (rulings R-A/R-B below):

- `vorma::Error` returns to being the plain framework/setup error (config validation,
  `bind_addr`, `public_url`); constructor renamed `Error::new(message)` — nothing
  transmits, nothing implied.
- View handlers return `Result<O, ViewExit>`; resource and middleware handlers return
  `Result<O, HttpExit>` (working names — R-A):
    - `ViewExit::err(server_record).with_client_msg(...) .with_source(...)` — no status
      field EXISTS (unrepresentable). (`err` constructor per the abbreviation-first
      ruling, mirroring Go's `LoaderError { ClientMsg, Err }` field vocabulary.)
    - `HttpExit::err(server_record).with_status(...).with_client_msg(...) .with_source(...)`
      — status defaults 500.
    - Both types carry a framework-constructed Redirect variant reached only via
      `return ctx.redirect("/login");` — handle-style ergonomics, early-exit mechanics, no
      fabricated `Ok` data ever (the fabricated-return disease dies in both its forms:
      ECHO-style errors AND redirect-after-data).
    - `?` works: `From<vorma::Error>` and `From<BoxError>` for both exit types (server
      record from Display, source chained).
- Resource wire: success = bare JSON (Go parity); error = `{"error": string}` envelope,
  application/json, status from the exit. TS `submissions.ts` parses the envelope on
  `!res.ok` (fallback generic on parse failure); `QueryResult`/`MutationResult.error`
  finally carries the server-authored text.
- `set_status` survives ONLY on the resource ctx, asserted `< 400` (201-class success
  statuses). The view ctx has no status method.
- Middleware document-mode short-circuit: framework-owned minimal response with the exit's
  status/client text (text/plain now, HTML shell upgradeable later without protocol
  change).

## Domain verdicts (full census, condensed)

| Surface                               | Verdict                                                                                                                       |
| ------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `AppConfig` + config types            | GOOD (exhaustive literal, ruled). `Default` exists and is validation-backed (`from_config` rejects empty) — fine. F3 applies. |
| `app!` / `view!` / `resource!` macros | GOOD (ruled; diagnostics compile-fail-pinned).                                                                                |
| Ctx read surface                      | GOOD; uniform. `exec_ctx()` is the task-spawn surface — needs register-clarifying docs, not reshaping.                        |
| ResponseHandle / HeadHandle           | GOOD shape; loses error vocabulary per protocol design.                                                                       |
| Head/Document builders                | GOOD post-`Into` fix; F1 naming conflict to settle.                                                                           |
| `vorma::Error` + Result               | Reshaped per protocol section.                                                                                                |
| `vorma::middleware` tower layers      | GOOD; richer than Go; names consistent.                                                                                       |
| `vorma::testing`                      | Right idea; F2 must resolve (example must use it or it must grow).                                                            |
| `RuntimeHost` / `App::from_config`    | GOOD; two-mode binary documented at the seam.                                                                                 |
| Tasks re-exports (`Task*`)            | GOOD; now concretely `TasksOptions<Error>` etc. — kit stays generic, vorma pins.                                              |
| `vorma_build::run`                    | GOOD; one function, final.                                                                                                    |
| TS core `__internal` (~90 exports)    | GOOD as the adapter contract; `To*` family is the spine.                                                                      |
| Adapters (react/preact/solid)         | GOOD; options = ClientOptions + `linkDefaultProps` + `apiDecorator` + `render`; uniform hooks.                                |
| `ClientOptions` (boot)                | GOOD; camelCase; every callback names its subject (`onRouteUpdate`, `onBuildSkewDetected`).                                   |
| `apiClient`                           | GOOD verbs (`query`/`queryOrThrow`/`mutate`/`mutateOrThrow`); error objects gain real messages via the envelope.              |
| `vorma/kit/*` (9 modules)             | KEEP all; camelCase holds; `fmt`/`debounce` are generic-utility scope creep accepted as kit philosophy (Go parity).           |
| Vite plugin                           | GOOD; zero-options default export.                                                                                            |
| create-vorma                          | OUT OF SCOPE (ruled: Rust rewrite, sequenced last).                                                                           |

## Rulings — ALL RESOLVED (user, 2026-06-11)

- **R-A — RATIFIED: `ViewExit` / `HttpExit`** with `::err(server_record)` constructors
  (Go's `LoaderError { ClientMsg, Err }` field vocabulary),
  `with_client_msg`/`with_source` (+ `with_status` on HttpExit only), and the
  framework-constructed Redirect variant reached via `return ctx.redirect(...)`.
- **R-B — RATIFIED: raw idents** (`r#type`, `r#as`) everywhere; `HtmlAttribute::type_`
  renames.
- **R-C — RESOLVED.** Naming rule finalized as principle 5's corollaries; `PathBuf`-anchor
  / `String`-fragment is the documented rule (write into field docs during execution).
- **Config reshape — RATIFIED (targeted dissolution).** `path_config` dissolves into
  top-level `public_static_base` + `api_base` (kills the fs-path/url-path register
  collision); `ServerConfig`/`server_config` → `ServerTarget`/`server_target` (names the
  value: a cargo target; aligns with the contract crate's `ServerBuildTarget`);
  frontend/ts_gen/dev_watch groups STAY (cohesion + compositional defaults); canonical
  field order declared in the struct (filesystem anchors → cargo target → URL mounts →
  domain groups → app values → tunables). Rejected for the record: full flattening (kills
  per-group `..Default::default()` composition and the documentation value of grouping);
  two-value project/app split (breaks the one-value keystone the collapse architecture
  depends on); `js_package_manager_base_cmd` as Vec (ceremony for the 99% case — String +
  documented split semantics).

## Execution order (all ratified — campaign running)

1. ✅ DONE — Error/exit protocol, landed end-to-end and e2e-proven:
    - `ViewExit`/`HttpExit` with `::err` constructors; view handlers return
      `Result<O, ViewExit>`, resource AND middleware handlers `Result<O, HttpExit>`;
      conversions keep `?` working.
    - `ctx.redirect(...)`/`redirect_with_status(...)` on all three ctxs — channel-riding
      (no fabricated returns); regenerated wire fixtures prove redirect bytes unchanged
      (303/location; client-redirect header).
    - `set_client_error` deleted everywhere; response handles split: shared handle =
      headers/cookies; `ResourceResponseHandle` adds `set_status` asserted `< 300` (NOT
      the doc's earlier `< 400`: a 3xx via set_status would bypass the redirect door and
      could emit a Location-less redirect — each outcome has exactly one door).
    - `is_terminal` redefined: engine-owned terminal_error flag or a real redirect; bare
      statuses are NEVER control flow.
    - Resource errors are the JSON envelope `{"error": text}` (application/json, status
      from the exit); framework faults (decode 400 / missing-handler 500) envelope on wire
      requests, plain on document requests; view-path terminal stays plain by design.
    - TS submissions parse the envelope (`result.error` finally carries server-authored
      text; non-JSON bodies fall back to "Request failed (STATUS)", pinned);
      `res.statusText` reading is gone from the submit path.
    - Wire-contract fixtures regenerated (resource_error fixture now pins the envelope
      cross-runtime); ECHO/FAIL fixtures and the example teach the new idioms;
      vorma::Error back to plain `new(message)` + `with_source`.
    - HISTORICAL INTERIM NOTE: create_client_core's route-fetch error path
      (view-navigation faults) still read statusText at this point. This was resolved in
      step 3 below as write-only dead freight, not as a live follow-up.
2. ✅ DONE — Config reshape: `path_config` dissolved into top-level `public_static_base` +
   `api_base` (the fs-path/url-path register collision is gone);
   `ServerConfig`/`server_config` → `ServerTarget`/`server_target`; canonical field
   order + the PathBuf-anchor/String-fragment rule documented in both `AppConfig` and
   `Config`; example literal teaches the order; whole tree migrated; e2e-smoke green on
   the reshape.
3. ✅ DONE — R-B raw idents (`HtmlAttribute::r#type`; head constructors already
   conformed). The route-fetch `statusText` sites turned out to be WRITE-ONLY dead freight
   (nothing consumed status/status_text on the nav error variant) — variant slimmed to
   `{kind:"error", response}` rather than envelope-parsing a channel nobody reads. F4
   RE-DISPOSITIONED: the `with_kind` const-builder was implemented and REVERTED — expected
   types do not flow through method-call receivers, so the builder broke S-inference into
   the macro closures (`const X: app::Resource = resource!{...}` is the only inference
   source). The positional `Option<ResourceKind>` stays; the macro emits explicit
   `Option::Some/None`; the only bare-`None` call sites are crate-internal tests. Accepted
   as macro-target plumbing.
4. **Historical Notes-era example expansion:** the example grows to exercise the FULL API
   surface. The "minimal" framing went away here as part of the earlier Notes workout.
   Current canonical example work has since moved to Board; see `__TICKET.md` and
   `frontend-client-coverage.md`. Historical coverage contract from this stage:
    - Rust: nested views + index semantics; splat route; search-schema view input;
      ctx.redirect on view, resource, AND middleware; a middleware that actually gates
      (cookie check → redirect for document requests / 401 HttpExit); FormData resource;
      explicit ResourceKind; set_status(201) (kept); set_cookie/append_header; exec_ctx
      task spawn; ts_gen extra_types/extra_ts; non-default DevWatchConfig; head breadth
      (preload helper, script/style, SafeHtml, HtmlAttribute);
      param()/params()/splat_values.
    - TS: clientLoader (incl. serverPromise prestart), errorBoundary,
      beforeRouteYield/Commit, explicit prefetch/cancelPrefetch,
      useRouteState/useWorkState/useRouteSync/usePattern\* hooks, apiClient.query +
      queryOrThrow + mutate, submit, work indicator, revalidateOnWindowFocus,
      vormaPublicUrl in client code, RootOutlet nesting, kit usage (theme + cookies at
      minimum).
    - F2 resolves here: ALL example tests convert to vorma::testing::TestApp (growing
      TestApp where it cannot express what the hand-rolled fixtures did). Contrived
      corners acceptable; awkwardness found there is still real.

    HISTORICAL RUST SLICE LANDED (client components + TS breadth still pending at that
    point). Workout findings from that stage:
    - POSITIVE: the full Rust surface composed almost first-try — nested views + explicit
      index + splat + cached Task subtask + FormData + cookies + both exit types +
      redirects from all three handler kinds compiled in one pass and 11/12 request-level
      tests passed on first boot.
    - SUPERSEDED BY RULING (was: leaf-pattern fix): the accessor
      `MiddlewareCtx::matched_pattern()` is RETIRED — "the one matched pattern" is an
      unaskable question for a middleware (requests match chains), and its only consumer
      was imperative scoping, which the ruling below makes declarative.

        RATIFIED AND IMPLEMENTED — scoped middlewares (restores Go capability, upgraded).
        Landed end-to-end: declaration filters
        (`Middleware::new(...).with_patterns([...]).with_methods([...])`) -> contract
        `MiddlewareDeclaration` -> per-middleware flat matchers compiled into the
        ExecutionPlan -> request-time `matching_middleware_ids(method, path)` filter in
        the engine. The old attach-middleware-to-route-nodes plumbing
        (`middleware_ids_for_pattern`, node `middleware_ids`, graph scope containment) is
        DELETED — net less machinery. Live-state protocol bumped to 2 (middleware wire
        shape changed). Pinned by `middleware_filters_select_by_pattern_and_method`
        (pattern/method/ AND/declaration-order) and
        `scoped_middleware_runs_only_inside_its_patterns` (engine-level run/skip), plus
        the example's admin-gate test through TestApp. `MiddlewareCtx::matched_pattern()`
        retired; the example's gate is declarative
        (`.with_patterns(["/admin", "/admin" + splat])`) with a body that is only the
        cookie check. Design facts:
        - One `middlewares![...]` array. The declaration carries optional PLURAL filters:
          `patterns` and `methods`. A provided field restricts to its listed values; an
          omitted field means unrestricted; provided fields AND together. (Go forced one
          axis per registration call — global XOR by-method XOR by-pattern; this composes
          them.)
        - Per-middleware matcher: each middleware with `patterns` gets its OWN flat
          Matcher instance (same pattern language as routes, no `_index` handling — the
          resource configuration), and the run decision is
          `find_best_match(request_path).is_some()`. Subtree gating is written in the
          pattern language itself ("/admin" + "/admin/\*"), NOT via invented
          chain-membership semantics. There is exactly one matcher grammar in the system
          (execution_plan.rs builds resources/views from the same crate; views differ only
          by `_index` + nested-chain assembly).
        - Scope is decoupled from the route tree: a scope pattern need not correspond to
          any registered route. Tradeoff accepted: a typo'd scope silently never matches
          (a future lint may flag scopes that match no route; optional polish).
        - Sequence (ratified): (1) match the request against view/resource matchers —
          params union / splat established here; (2) select middlewares via their filters
          (boolean only — scope matchers contribute NO captures; ctx params/splat remain
          the route's facts); (3) run middlewares (parallel phase), then handlers.
          Corollary: no route match -> no Vorma middlewares; transport concerns
          (logging/compression/headers on 404s) belong to the tower/axum layer.
        - Dropped from Go: `MiddlewareOptions.If` predicate (early-return in the handler
          is the one way) and typed task-middleware outputs (incidental to Go's chassis;
          the Rust idiom is a shared `Task` — middleware preloads, handler reruns the same
          task, the task store dedupes; pinned by
          `route_and_middleware_share_one_request_task_scope`).
        - Confirmed intact (the "whole point" check): middlewares hold the request ExecCtx
          (`MiddlewareCtx::exec_ctx`), run parallel by default, and share ONE request task
          scope with handlers.

    - FIXED (exports): `HttpHeaderMap` was missing from the http re-exports —
      header-reading helpers outside handlers had no name for the type without adding an
      `http` dependency.
    - FIXED (head API): `SafeHtml::script_content` did not exist (style had no script
      twin), and `style_content`'s `[HtmlElementDef; 1]` return composed badly inside
      mixed attribute arrays. Both now return one `HtmlElementDef`; callers write the
      array.
    - GROWN (testing): `TestApp::request(method, path)` builder with
      `.header/.cookie/.body(content_type, bytes).send()` — cookie and multipart requests
      no longer force `http` + `bytes` dev-deps onto every app's test suite.
    - F2 RESOLVED: all example tests are now request-level `vorma::testing::TestApp` tests
      (12 of them; no hand-rolled manifest fixtures anywhere in the example).
    - Trap noted: Rust block comments NEST — a literal splat pattern ("/tags" + star)
      inside a block comment opens a nested comment.

    HISTORICAL TS SLICE LANDED in the Notes-era workout (full pipeline: tsgen + vite build
    green; tsgo/oxlint clean). Client now exercises: nested Outlet under a layout view,
    clientLoader + serverPromise, errorBoundary + defaultErrorBoundary, beforeRouteCommit,
    useRouteSync (debounced ?draft= sync),
    useViewData/useClientLoaderData/useRouteState/useWorkState (work indicator), explicit
    prefetch/cancelPrefetch alongside linkDefaultProps prefetch:"intent",
    revalidateOnWindowFocus, apiClient mutateOrThrow (JSON + FormData + DELETE +
    redirect-following) and queryOrThrow, kit/theme + kit/cookies, and the
    extra_types/export_const pipeline (`keyboard_shortcuts` consumed from vorma.gen.ts).
    usePattern\* variants and raw submit() remain covered by the adapter suites; the
    example teaches the primary forms. TS-slice findings:
    - FIXED (type bug): `ViewInputWithParentViews` intersects the input types of the whole
      matched chain (a URL's query feeds every matched view), but a no-input parent
      (`input: ()` -> undefined) ANNIHILATED the intersection — every child of a plain
      layout typed its `search` as `undefined`. Normalized undefined -> {} before
      intersecting.
    - FIXED (example): `TsDrafter::const_` vs `export_const` — emitting a consumable
      constant requires the export\_ variant; the un-exported one fails at vite import
      time, not generation time.
    - RETRACTION — there was NO clientLoader port regression. The public type carried
      `viewData: ToViewOutput<A, P>` at HEAD all along (view_types.ts:123); the Go parity
      (old name `loaderData`) was preserved by the port. The false "PORT REGRESSION"
      finding came from a chain of maintainer errors: broken example code
      (`await serverPromise` used AS the view data), a truncated read of the type that
      stopped two lines above the field, and a "receipts" check that verified only the Go
      side. The codebase was healthy AND fully pinned the whole time: typecheck.ts pinned
      `Awaited<serverPromise>["viewData"]` to the view's own output at HEAD
      (`_cl_props_view_data`), the runtime populated it, and the field sat in the public
      type. Net real artifacts of the episode: the example's stats loader fixed to the
      honest `(await serverPromise).viewData` form, plus a redundant-but- harmless
      value-level pin in the adapter suite. Process lesson for this file: never claim
      absence from a truncated read — verify with the full definition and `git show HEAD:`
      before writing "regression".
    - Awkward (logged): `useRouteSync`'s `search` rightly carries the chain-merged input
      type, but nothing in the name says so — field docs will need to teach it.

5. Re-run the full gate + e2e matrix; update `docs/maintainer/ARCHITECTURE.md` if the
   architecture changed; THEN user-facing docs unblock.

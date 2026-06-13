# Fable API, Notes, and Board Notes

These notes come from the large `9e3bfc5a...` Fable main history after the initial
architecture/regression restoration work. They capture forward-going API decisions and
process lessons, not a changelog.

## Source Coverage

- Main transcript ranges: `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7.jsonl` lines 2,777-8,600
  cover the client-core tranche, `CURRENT_PLAN_2`, single-binary collapse, API campaign,
  full-surface Notes workout, scoped middleware, `api_base` removal discussion, and the
  start of the matcher overlap initiative.
- Current reconciliation: `API_DESIGN.md`, `CURRENT_PLAN_3.md`,
  `PRESSURE_TEST_CENSUS.md`, `crates/vorma/src/config.rs`,
  `crates/vorma-contract/src/graph_validation.rs`, and current TypeScript view/client
  loader types.

## Planning Docs Are Layered, Not Equally Current

`CURRENT_PLAN_3.md` was accurate when written, but later API and Board work superseded some
of its statements. In particular, `CURRENT_PLAN_3.md` still contains older language about
`PathConfig`, `api_base`, `api_mount_root`, and Notes-era API scrutiny. Current truth lives
in the newer API and pressure-test trackers:

- `API_DESIGN.md` records the error/exit protocol, config reshape, scoped middleware, and
  the corrected client-loader facts.
- `PRESSURE_TEST_CENSUS.md` records the Board pressure-test initiative, the API mount
  removal, matcher specificity doctrine, and the current fussy-app ledger.
- Current `crates/vorma/src/config.rs` has `root_dir`, `dist_dir`, `server_target`,
  `public_static_base`, `frontend_config`, `ts_gen_config`, and `dev_watch_config`. There
  is no public `api_base`.

When a future doc conflicts with these notes, do not assume the older plan is still
current. Reconcile against code and the newest tracker.

## Single-Binary Collapse

The settled design is not a CLI and not a non-app-linked shim:

- The public build entry is `vorma_build::run(app_config)`.
- The entry binary links app config and reads it in-process at session start. This is
  acceptable because the entry is launched once per dev session; the hot loop no longer
  relinks the entry.
- The app server binary is the live-state emitter after startup. The runtime checks the
  live-state env key, serializes the build contract, and exits instead of serving.
- The server target is single-sourced in app config. It is read freshly at session start.
  A mid-session server-target change must fail with a restart instruction rather than
  silently compiling or running the wrong target.
- `BuildOptions` died because any separate `cargo_package`/`cargo_bin` option would be a
  second source of truth.
- The design deliberately mirrors the useful half of the Go bootstrap pattern: first
  read the freshly linked config directly, then use rebuilt child processes for later
  live-state reads. The difference is that Rust re-runs the server binary, not a separate
  build-entry binary.

The collapse also removed the earlier combined dual-bin cargo build machinery. Any future
attempt to reintroduce a separate build-entry compile needs to justify why the single
app-server child cannot satisfy live state.

## Vite Freshness and Dev Loop

The Vite public-asset rule is now a maintainer invariant:

- Public-asset saves must never restart Vite.
- Freshness comes from plugin-recorded asset-to-module edges at transform time and a
  targeted `/assets-changed` control message that invalidates exactly referencing modules.
- `/cfg-changed` is reserved for actual `VitePluginConfig` payload changes: entry module,
  view-module list, ignored patterns, dedupe list.
- Filemap-only changes and unconditional app-rebuild notifications must not restart Vite.

The Go audit clarified that Go's static path had a bug: cached Vite transforms could keep
old hashed URLs and 404 after asset-only changes. The Rust target is neither Go's stale
behavior nor restart-per-save; it is targeted invalidation without restart.

## Error and Exit Protocol

The protocol decision is semantic, not ergonomic sugar:

- Views are a framework-owned nested rendering protocol, not HTTP documents. A view
  rejection does not have an HTTP status field.
- Resources and middlewares speak HTTP and use status-bearing exits.
- The public types are `ViewExit` and `HttpExit`.
- `ViewExit::err(server_record).with_client_msg(...).with_source(...)` is the view path.
- `HttpExit::err(server_record).with_status(...).with_client_msg(...).with_source(...)`
  is the HTTP path.
- Nothing reaches the client unless explicitly marked client-facing.
- Redirects ride the exit channel through `ctx.redirect(...)`; no fabricated `Ok` data
  should be returned after a redirect or rejection.
- Bare statuses are not control flow. `set_status` survives only as a resource success
  status door and is asserted below redirect/error ranges.
- Resource errors use a JSON envelope such as `{"error": "..."}`. TypeScript parses the
  envelope and falls back to a generic `Request failed (STATUS)` message for non-envelope
  bodies.

The user specifically rejected the idea that documenting a confusing API is sufficient.
If another confusing rejection path is found, redesign the API so misuse is impossible.

## Config Reshape

The config naming and shape decisions:

- `root_dir` is the one absolute path anchor.
- Other path-ish config fields are root-relative `String` fragments.
- `ServerTarget`/`server_target` names the cargo package/bin target, not a running server.
- `path_config` was dissolved. Later Board work removed `api_base` entirely, leaving
  `public_static_base` as the only public URL-space config field of that kind.
- `FrontendConfig`, `TsGenConfig`, and `DevWatchConfig` stay grouped because their fields
  compose as coherent defaults.
- `AppConfig` stays an exhaustive struct literal. This is intentional for pre-1.0:
  field additions should be loud and create-vorma should scaffold them.

Older docs that mention `PathConfig { public_static_base, api_base }` are stale for the
API mount piece.

## Scoped Middleware

The transcript went through a false fix before landing the right model:

- `MiddlewareCtx::matched_pattern()` was the wrong abstraction. A middleware request
  matches a chain, not one pattern, and the accessor was being used to simulate scoping.
- The correct model is declarative middleware filtering at registration.
- A single `middlewares![...]` array remains.
- Each middleware can carry optional plural filters: `.with_patterns([...])` and
  `.with_methods([...])`.
- Omitted filter means unrestricted on that axis.
- Provided filters combine with AND semantics.
- A middleware with patterns gets its own flat matcher. That scope matcher is only a
  boolean selection mechanism. It contributes no params or splats to the handler context.
- Route match happens first; middleware selection happens after the route facts exist;
  middlewares then run in their parallel phase before handlers.
- No route match means no Vorma middleware. Transport-wide concerns for 404s belong in
  the tower/axum layer.
- Go's `If` predicate was intentionally dropped. Runtime ad-hoc checks belong as early
  returns inside the middleware handler.
- Go's typed task-middleware outputs were considered incidental to its task chassis.
  Rust's durable idiom is shared `Task`: middleware preloads, handler reruns the same
  task, and the request task store dedupes.

The "whole point" check passed and should remain true: middleware has `exec_ctx()`, runs
parallel by default, and shares the same request task scope with handlers.

## Client Loader `viewData` Retraction

There was a false "clientLoader regression" claim. The final truth:

- The public TypeScript type already included typed `viewData: ToViewOutput<A, P>`.
- The runtime already populated the value.
- Typecheck pins already covered it.
- The bad example code had treated `await serverPromise` itself as the view data, then a
  truncated read stopped before the `viewData` field and produced a false regression.
- The example should use `(await serverPromise).viewData`.

Process lesson: never claim absence from a truncated type read. Read the full type and
check HEAD before writing "regression".

## Notes Workout

The Notes example was expanded as a full-surface forced workout before the Board pivot.
Forward facts:

- The exercise intentionally used some contrived cases because its job was to make API
  awkwardness visible.
- `TestApp` grew a request builder and the example tests became request-level
  `vorma::testing::TestApp` tests.
- It found real issues: search input intersections with no-input parents, missing
  `HttpHeaderMap` export, head builder asymmetries, and the middleware scoping problem.
- It also demonstrated that examples can expose API flaws before docs freeze them.

Do not treat Notes as the final running example for docs. The later pressure-test
direction is Vorma Board.

## Vorma Board Pressure Test

The Board initiative is not "make an example app eventually"; it is a census-first API
pressure test:

- The app is HN-shaped: stories, threaded comments, votes, users, moderation, search,
  docs splats, auth, and SQLite persistence.
- Every user-reachable Rust and TypeScript framework API must map to a named realistic
  feature before code.
- Anything without a realistic feature home goes to a scrutinize-or-cut list.
- `vorma/kit/*` is out of scope as a framework design target, but the app may consume
  generic kit utilities the way normal apps do.
- Popular ecosystem integrations are deliberate: nprogress via `workIndicator`,
  react-query over `apiClient`, jotai with local theme state, and view transitions.

Important Board-specific API conclusions:

- `apiClient.toIdentityArray(args)` is the react-query key bridge.
- Queries take full args at the hook because args are cache identity.
- Mutations take endpoint identity at the hook and the varying params/input at `mutate`.
- Vorma mutations auto-revalidate route data by default. Manual `revalidate()` after
  mutation is usually double-scheduling and should be documented as a thing not to do.
- `workIndicator.track(promise)` is for non-Vorma async that should appear in global work
  state. Vorma API calls are already included.
- `kit/theme` remains generic. Apps own the inline first-paint HTML snippet. Do not add a
  Rust-side Vorma-specific theme helper that makes a generic kit depend on one backend.

Code reconciliation from the `fable-1` commit adds a specific unresolved Board finding:
the attachment download path is not yet a valid non-JSON resource-body proof. The
resource sets `content-disposition` and returns `Ok(())`; the public typed handler path
serializes typed output to JSON and exposes no public raw-byte body handle; the test
checks only status plus disposition. Treat this as an API-design gap, not as a passing
download feature.

The same `fable-1` reconciliation adds a second Board gap: the committed tree declares
client files for `/submit`, `/u/:username`, `/u/:username/comments`, `/docs`, `/docs/*`,
`/search`, `/mod`, and `/mod/diagnostics`, but those eight modules are absent. Board's
request tests are still valuable server/API evidence, but they do not prove the full
browser/Vite app until those client modules exist and a frontend build path covers them.

## API Mount Removal

The API mount was removed after a long design correction. This is current truth:

- Resources declare full URL patterns. `/api/...` is app-authored spelling, not a
  framework partition.
- Views and GET/HEAD resources share one URL space.
- Dispatch is not "all overlaps are conflicts." Specificity is the doctrine.
- A conflict exists only when a view pattern and a GET/HEAD resource pattern overlap with
  equal specificity: a tie on a shared path.
- `vorma_matcher::find_overlap` supplies the witness path, and
  `vorma_matcher::compare_specificity` supplies the one public ordering used by both
  validation and runtime choice.
- A root catch-all view coexists with resources. It is the floor of specificity and loses
  to every covering route by construction.
- 405 is computed by path, with views/assets contributing GET/HEAD to `Allow`.
- 404 is a finalized empty response with the client build id header unless the app
  declares a catch-all view.
- Public static remains a reserved prefix because it is a manifest/static-file space,
  not an API/resource/view specificity question.
- `api_base` is gone from public app config, runtime manifest, live-state, projection
  bundle, and TypeScript client URL/key building.

Pitfall: Older Fable notes or docs that say scope patterns must include the API mount are
stale after the mount removal. The deeper rule remains: scope patterns are URL patterns in
the one observable URL space, with no hidden rewriting.

## Naming and Process Lessons

- Do not say a value is a "theme" if it is actually a collection of helpers. Names must
  describe the value, not the domain vibe.
- Do not infer what the maintainer remembers or means. State only what the source proves.
- Do not present open questions as homework. Give a strong recommendation from first
  principles, then ask for approval only when approval is actually needed.
- Do not edit during a "do not edit, just respond" turn. In the histories, repeated
  premature edits caused avoidable loss of trust.

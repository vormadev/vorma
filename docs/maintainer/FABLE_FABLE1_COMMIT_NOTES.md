# Fable-1 Preliminary Code Map

Status: preliminary. This file was produced from a selective first pass over the
`fable-1` commit and must not be read as proof that the commit has been fully
absorbed. Use `FABLE_FABLE1_RECONCILIATION_LEDGER.md` as the authoritative
record of which Fable-note claims have been reconciled against committed code.

These notes record facts observed in the committed tree at commit
`eff2c2f8d6edeb0f98c62af7cd9c46c183175844` (`fable-1`, authored
2026-06-13). They are not a changelog. They are a starting map for the
notes-led, diff-verified reconciliation pass, not its completion.

This pass intentionally reviewed the `HEAD` tree and `HEAD^..HEAD` diff only.
Post-commit staged or working-tree changes are outside this note.

## How This Note Should Be Used

- Treat this file as a useful but incomplete map. When a transcript note says
  what was intended and this file records committed-tree facts, the
  reconciliation ledger decides whether the claim has been verified, corrected,
  or left unresolved.
- Several comments and old docs still use obsolete words such as
  "build-entry" or "API mount root". Those words are stale unless the current
  code facts below explicitly preserve them.
- The most important unresolved fable-1 gap is not architectural. It is that
  Board's attachment download claims to pressure-test a non-JSON resource body,
  but the public typed resource surface does not actually emit the stored bytes.

## Build And Dev Architecture

- `crates/vorma-build` has no user-facing CLI. The public entrypoint is
  `vorma_build::run(app_config)`, with `build_production` and
  `start_dev_server` available as harness-oriented helpers.
- The single app-linked binary collapse is implemented. During dev rebuilds,
  `crates/vorma-build/src/app_server_build.rs` builds the configured app-server
  target once with `cargo build -p <package> --bin <bin>
  --message-format=json-render-diagnostics`, finds the produced executable from
  Cargo JSON artifacts, then runs that same executable in live-state mode.
- There is no separate hot-loop build-entry compile. Per-app `src/bin/build.rs`
  remains only a thin process entry that calls `vorma_build::run(app_config)`.
- `crates/vorma/src/live_state_emit.rs` is the runtime side of the collapse:
  `App::from_config` checks `__VORMA_LIVE_BUILD_STATE=1`; in that mode it emits
  the live build-state JSON to stdout and exits with status 0. Application
  errors are also emitted as the live-state JSON error envelope and still exit
  0, so the build side distinguishes success from app failure by payload, not
  process status.
- `crates/vorma-contract/src/live_state.rs` defines
  `LIVE_BUILD_STATE_PROTOCOL` as `3`. Earlier v2 references are historical.
  Emitters built before the protocol field existed deserialize as protocol 0
  and are rejected by the same mismatch check.
- Live-state decoding first checks the `{ "error": ... }` envelope, then
  deserializes the state, checks the protocol, recompiles the graph, and
  requires a non-empty `root_document_hash_source`.
- The server cargo target is session-bound. The dev loop reads it in-process at
  session start and later checks new live-state against that same target. A
  mid-session target change is a hard restart-required condition, not a live
  retarget.
- `crates/vorma-build/src/live_state_command.rs` still has stale comments,
  test names, error variants, and messages saying "build-entry executable" and
  "build-entry live-state command". The implementation is generic enough to
  run any executable, but post-collapse terminology should say app-server
  executable. Do not copy the stale wording into new architecture.
- `crates/vorma-build/src/output_lock.rs` holds a process lock at
  `<dist>/.vorma/build.lock` for output layout mutation. Dev holds the lock for
  process lifetime; production holds it for build lifetime. Live-state children
  must not acquire it because the parent may be holding it while asking for
  live-state.
- Output layout moves are safe by acquisition order: the new lock is acquired
  before the old lock is released. The old output layout path is returned for
  caller-owned cleanup only after the new generation commits.
- `crates/vorma-build/src/build_output.rs` writes generated files atomically
  through temp files, skips byte-identical rewrites, rejects symlinked
  directory components, and writes a `.gitignore` containing `*` inside the
  Vorma output directory. Do not remove the `.gitignore` write as "noise";
  generated layout ownership depends on it.
- `crates/vorma-build/src/generation_epoch.rs` makes generation candidates
  immutable compiled units: graph, projection bundle, build plan, generated TS,
  static artifacts, manifest, and a validating `RuntimeSnapshot`. Commit-time
  effects are derived by comparing the committed generation to the candidate.
- `GenerationEffects` detects public-asset changes, critical-CSS changes, and
  client revalidation requirements from manifest filepaths/filemaps, view
  module outputs, critical CSS, and view/resource contracts. These effects are
  the basis for avoiding unnecessary Vite/app-server work.

## Dev Loop And Vite Control

- The dev loop in `crates/vorma-build/src/entrypoint.rs` is event-driven. It
  reacts to filesystem events, child-exit events, and shutdown, with a tiny
  post-event settle debounce. It must not be replaced by polling or periodic
  wakeup loops.
- App-server generation changes cancel an in-flight app-server rebuild and
  queue the merged changes. Static or client-revalidation changes queue without
  cancelling the current app-server rebuild.
- A dev generation activation publishes candidate files, starts a candidate
  app server on a fresh port, updates the Vite RPC generation contract, notifies
  the Vite plugin, switches the dev mux backend, commits the generation, then
  terminates the old app server.
- `crates/vorma-build/src/dev_build.rs` uses `DevDiskRollback` to snapshot
  generated TS, manifest, and candidate static outputs before activation. If
  Vite notification or dev-mux switching fails, it restores the previous RPC
  contract, terminates the uncommitted app-server process, and restores/removes
  exactly the captured files.
- `crates/vorma-build/src/dev_mux.rs` is the stable browser-facing loopback
  server. It proxies every non-refresh request to the current active app-server
  port stored in an atomic, supports HTTP upgrade proxying, and is the
  zero-downtime swap point.
- `crates/vorma-build/src/process_runner.rs` clears Vorma role env vars by
  default before applying explicit env. That prevents parent build/dev role
  variables from leaking into child roles.
- Build process cancellation terminates the child process group. Unix child
  unexpected-exit monitoring is implemented; Windows remains an intentional
  no-op stub until Windows dev support can be tested.
- `crates/vorma-build/src/dev_watcher.rs` separates static OS watch roots from
  the swappable classification plan. Per-generation critical-CSS imports extend
  the classification plan without replacing the whole watcher.
- `BuildDevWatchPlan::derive` inserts all three app-config dev-watch vectors:
  `watch_patterns`, `on_change_recompile_server`, and
  `on_change_client_revalidate`. A Board client-revalidate pattern does not
  need to be duplicated in `watch_patterns` to be watched.
- `crates/vorma-build/src/static_outputs.rs` canonicalizes and validates public
  static roots, rejects symlink source entries, rejects non-file source entries,
  skips `.DS_Store`, and rejects output layouts inside source dirs.
- Public static output names use a `blake3` content hash plus source path,
  encoded base32 lower and truncated to 12 chars. A process-wide file hash cache
  keys on source path, modified time, and size, with a 65,536-entry cap.
- Stale framework-owned public outputs are intentionally retained. Cached Vite
  transforms may still reference an older hashed URL after a public-asset edit,
  so deleting old generated assets immediately can create 404s.
- Critical CSS uses `lightningcss`; imports are canonicalized under the app
  root, `@public/...` URLs are rewritten through the filemap, absolute/external
  URLs pass through, and relative non-public URLs are errors.
- The Rust Vite plugin contract lives in
  `crates/vorma-build/src/vite_plugin_contract.rs`. The control paths are
  `/cfg-changed` and `/assets-changed`; the token header is
  `x-vorma-vite-plugin-token`; public source URLs use the `@public/` prefix.
- The build-side Vite RPC server accepts `cfg`, `hash`, and `set_port`.
  `set_port` probes the reported control port with a short TCP connect before
  accepting it.
- The TypeScript Vite plugin in `packages/vorma/vite/vite.ts` records
  asset-to-module edges at transform time. `/assets-changed` invalidates only
  modules known to consume changed public source paths. `/cfg-changed` restarts
  Vite and is reserved for actual plugin config changes.
- JavaScript public URL calls must be exactly a static string literal or a
  no-expression template literal. CSS `url("@public/...")` handling preserves
  query and fragment portions.

## Contract Crate And Graph Truth

- `crates/vorma-contract` is the shared truth crate for graph declarations,
  validation, execution plans, runtime manifests, TypeScript-generation models,
  document-renderer pieces, wire types, live-state, and constants.
- `extern crate self as vorma` in `vorma-contract` is intentional. It lets local
  tsgen macro paths that spell `::vorma::tsgen::*` work without depending on
  the runtime crate.
- `FrameworkConfig` has `public_static_base` plus optional `BuildInputConfig`.
  The old API mount/base field is absent.
- `BuildInputConfig` carries the server target, root/dist dirs, frontend
  inputs, generated TypeScript output, extra TypeScript, and dev-watch config.
- `ResourceKind` is a generated-client/revalidation classification only. It is
  not a dispatch fact and must not be used to decide URL conflicts.
- Middlewares carry optional plural `patterns` and `methods`. Empty filters
  mean unrestricted; provided filters combine with AND semantics.
- Every scoped middleware gets its own flat matcher. Scope captures are only a
  boolean run/skip test; they do not become `ctx.params()` or splat values.
- Views and GET/HEAD resources share one observable URL space. The graph only
  rejects a view/resource overlap when `compare_specificity` produces an equal
  tie on an overlapping path.
- The static public base remains a reserved manifest/static-file prefix when it
  is not `/`. A root static base opts out of that partition.
- `ExecutionPlan` compiles flat resource matchers per method, one nested view
  matcher, route maps, and middleware plan nodes. HEAD falls back to GET if no
  HEAD-specific resource exists. `allowed_resource_methods` adds HEAD wherever
  GET exists.
- `RuntimeManifest` derives `client_build_id` from the stable serialized
  manifest using `blake3`, base32 lower, truncated to 24 chars. It skips that
  field in JSON and recomputes it on load.
- Runtime manifest JSON is tab-indented stable JSON. CSP hashes use sha256
  because CSP is an external protocol; that is the correct exception to the
  project default of `blake3`.
- `RuntimeSnapshot` validates that graph and manifest agree on search schemas
  and view modules, and that client entry/core/view module URLs are listed in
  manifest public outputs unless they are dev Vite module URLs.

## Runtime Request Semantics

- The public runtime crate `crates/vorma` reexports the intended app API:
  `App`, `AppConfig`, `View`, `Resource`, `Middleware`, contexts, exit types,
  typed HTTP aliases, `ResourceKind`, `RuntimeHost`, document/head helpers,
  task types, and testing harness pieces.
- `Config` uses `root_dir` as the absolute anchor. Other path-like config
  fields are root-relative string fragments. `api_base` is gone.
- `AppConfig` is an exhaustive struct literal and includes filesystem anchors,
  cargo target, public static base, frontend/tsgen/dev-watch groups, state,
  views, resources, middlewares, tasks options, document, and request body
  limit.
- `crates/vorma/src/public_app.rs` still has a stale docstring:
  `Resource::pattern()` says "relative to the configured API mount root". That
  is false in fable-1; resource patterns are full URL patterns in the shared URL
  space.
- Request classification in `crates/vorma/src/execution_engine.rs`:
  public asset first for GET/HEAD if the path is in static space; then resource
  and view matching; if both match, `compare_specificity` chooses; equal ties
  should have been rejected at graph compile time. Non-GET/HEAD requests only
  match resources.
- 405 is computed by path. Resources contribute their methods; views and public
  assets contribute GET/HEAD.
- The execution engine selects Vorma middlewares after route matching. If no
  Vorma route/asset target exists, no Vorma middleware runs; transport-wide 404
  concerns belong in the outer Tower/Axum stack.
- Middleware execution is a phase barrier. Middlewares launch in parallel, then
  commit in declaration order. A terminal middleware outcome suppresses later
  middleware commits and all handlers.
- Handler execution for a route also launches in parallel and commits in order.
  View handlers and middlewares being parallel is a contract, not a mere
  optimization.
- Middleware and handler phases share the same request `ExecCtx`, which is why
  the preload-current-user pattern works: middleware starts a task and the
  handler later reads the same task key from the request task store.
- For resources, input is decoded once for the route and reused by middleware as
  default decoded input. For views, inputs decode per view segment.
- Middleware scope-pattern captures do not replace route captures. Resource
  middleware sees resource route params/splats; view middleware sees view route
  params/splats.
- View errors without terminal effects produce a server-error entry at the view
  index, using the explicit client message when provided and the generic
  fallback otherwise. View redirects are terminal.
- Resource and middleware non-redirect errors become terminal HTTP error
  effects. Resource finalization renders those errors as JSON
  `{ "error": text }`.
- `ResponseEffects::set_status` is not control flow. `set_status_with_text` is
  the path that creates a terminal error. `ResourceResponseHandle::set_status`
  asserts status below 300; resource errors use `HttpExit`, redirects use
  `ctx.redirect`.
- `ViewExit` has no HTTP status. `HttpExit` does. Nothing server-side reaches
  the client unless explicitly set as client-facing text.
- Resource success output from public typed handlers is serialized JSON. The
  runtime has an internal `HandlerOutput::body(Bytes)` path, and
  `resource_response.rs` can finalize it, but the fable-1 public typed macro
  path uses `serde_json::to_value(output)` and exposes no public body-writing
  handle.
- Invalid UTF-8 in the request path is a Bad Request. View document handling
  prepares the document and execution concurrently.
- `HEAD` suppresses the body while preserving `Content-Length` for public
  assets, resource success, and resource terminal errors.

## TypeScript Client Core

- `packages/vorma/core/create_client_core.ts` is now a composition root over a
  small set of explicit base facts, not an everything-owning monolith. The
  base-fact model is the architecture to preserve.
- The base facts called out in the file are: phase, browser history entry,
  route snapshot, active fetch, prefetch, refresh scheduler, API requests,
  deferred submit redirect, and sequence number.
- `work_projection.ts` derives work state from explicit sources: navigation,
  active revalidation, scheduler refresh, submissions, and prefetch. Do not
  scatter derived "is busy" state through unrelated modules.
- `client_loaders.ts` owns loader registry, search-schema registry, WASM
  matcher readiness, route-pattern queueing, loader prestart, prefetch
  reconciliation, and loader execution.
- Client loader `prestart` begins known parent loaders before the server payload
  arrives when current known view patterns prove they are needed. Later loaders
  discovered from the server payload run in parallel before commit.
- Loader reconciliation aborts at and after the outermost server error, seeds
  server state into retained prefetches, and aborts unclaimed prefetches.
- `route_modules.ts` owns dynamic imports, dev module cache, HMR version stamps,
  loader registration from modules, route-record construction, and HMR rerun
  policy.
- `submissions.ts` owns concurrent API submissions, dedupe, JSON body
  normalization, redirect/build-skew handling, and revalidation scheduling.
- TypeScript resource error parsing reads the JSON envelope and falls back to
  `Request failed (STATUS)`. It must not rely on `res.statusText`.
- `api_client.ts` exposes typed query/mutation helpers plus
  `toIdentityArray(args)`, using stable JSON stringification for cache identity.
- `QueryError` and `MutationError` carry the full failed result. Do not reduce
  them to strings in wrapper APIs.

## Matcher Baseline

- `vorma-matcher` is sovereign, not framework-private. Vorma layers its own
  route policy on top; the matcher crate should not grow Vorma-only pet
  semantics.
- The public matcher split is `FlatMatcher` for single best match and
  `NestedMatcher` for nested route chains. The builder remains the shared
  grammar authority.
- `find_overlap` is a free function over either matcher type through a sealed
  trait. It returns a deterministic witness path and winning patterns based on
  real matcher evaluation.
- Specificity is centralized in `compare_specificity`. Runtime matching,
  graph validation, and resource/view dispatch must not reimplement local
  score systems.
- Dirty path semantics in fable-1: one trailing slash may be tolerated as noise;
  doubled slashes elsewhere do not match; params and splats never contain empty
  strings.
- A root catch-all `/*` matches the root path with no splat values. A prefix hit
  is not itself a match. Nested matching accepts dynamic index routes without
  an explicitly registered dynamic parent.
- Matcher internals use cheap pattern/result storage and fast hash maps for
  registered route-pattern keys. Do not transfer that hasher reasoning to
  attacker-influenced task-cache keys.

## Board Pressure-Test Facts

- `examples/board` is the fable-1 pressure rig replacing the older contrived
  Notes workout. It is HN-shaped: stories, comments, votes, users, moderation,
  search, docs splats, auth, SQLite persistence, sessions, attachments, and
  realistic client integrations.
- Board app config uses `root_dir` from the crate manifest dir, `dist_dir` `.`,
  `server_target` for package/bin `board-server`, `public_static_base`
  `/assets/`, React, pnpm exec, Vite entry/config, public static dir, and
  critical CSS.
- Board dev-watch config watches `src/**/*` and `public/**/*`, recompiles the
  server for `src/**/*.rs` and `src/schema.sql`, and client-revalidates for
  `seed-data/**/*`. The build-plan lowering inserts client-revalidate patterns
  directly, so the seed-data pattern is watched even though it is not duplicated
  in the general watch list.
- The server binary wraps `App::from_config` in a real Axum/Tower stack with
  health, robots, tracing, sensitive headers, request IDs, secure headers,
  panic recovery, body limit, etag, compression, body timeout, and handler
  timeout middleware. `App` is the fallback service.
- Board session auth is passwordless demo auth with a `board_session` cookie.
  `current_user_preload` middleware starts the session-user task, and handlers
  read the same task from the request `ExecCtx`.
- `mod_gate` uses scoped patterns `"/mod"`, `"/mod/*"`, and `"/api/mod/*"`.
  That spelling is deliberate because there is no hidden API mount rewriting:
  mod resources really live under `/api/mod/...`.
- Board views exercise nested data, splats, soft not-found states, segment
  errors, head effects, parallel task prewarming, and URL-state-only search.
  `/docs` and `/docs/*` are separate because the splat pattern is one-or-more,
  not the bare docs root.
- Board resources exercise login/logout, FormData story submission, redirects,
  vote conflicts, comments, query resources, moderation mutations, and
  attachment lookup.
- Board's repo layer makes reads `Task`s and writes plain async functions. This
  is intentional: dedupe is correct for reads and wrong for mutations.
- `repo::DbInput<I>` carries the `Arc<Db>` but hashes/compares only the typed
  `input`. That is correct for Board's single database per process. Do not copy
  that wrapper blindly into a multi-database app where the database handle must
  be part of the task key.
- `repo::blocking_task` is stale call-site friction in `fable-1`: its comment
  says it exists because `TaskError<E>` lacks `From<E>`, but the committed
  task error type already implements `From<E>`.
- Board client code demonstrates an app-owned `useApiMutation` wrapper over
  react-query. Endpoint identity is fixed at hook creation; mutate-time
  variables supply params/input. Query wrappers were not landed in fable-1.
- Board client code wires nprogress through Vorma's work state, uses a shared
  react-query client, applies default link prefetch props, enables focus
  revalidation and view transitions, and uses app-owned theme state. These are
  ecosystem-cooperation pressure tests, not new Vorma framework dependencies.
- `examples/board/tests/app.rs` uses `vorma::testing::TestApp` with an isolated
  SQLite file per test. It tests sessions, submit auth, redirects, story
  rendering, soft not-found pages, duplicate-vote envelopes, ranking, logout,
  validation envelopes, comments, search, scoped moderation middleware, docs
  splats, segment errors, attachment headers, nested user pages, and task
  overrides.
- The committed Board frontend is incomplete. `views.rs` declares client files
  for `/submit`, `/u/:username`, `/u/:username/comments`, `/docs`, `/docs/*`,
  `/search`, `/mod`, and `/mod/diagnostics`, but those eight client modules are
  absent from the `fable-1` tree. The request tests do not exercise Vite module
  import existence.

## Board Attachment Gap

- `examples/board/src/resources.rs` documents `STORY_ATTACHMENT` as a
  non-JSON resource response whose typed output is "the raw bytes path".
- The actual fable-1 handler fetches the attachment, sets only
  `content-disposition`, and returns `Ok(())`.
- The public typed handler adapter in `crates/vorma/src/typed_handler.rs`
  serializes typed outputs with `serde_json::to_value(output)`. With
  `output: ()`, the committed resource body is JSON `null` unless later effects
  suppress it.
- The resource finalizer can emit raw byte bodies only when it receives internal
  `HandlerOutput::body(Bytes)`. The public Board resource macro path does not
  expose a way to create that body.
- The Board attachment test checks status and `content-disposition` only. It
  does not assert the bytes `hello attachment` nor the stored content type.
- This is a real unresolved pressure-test failure: either the public resource
  API needs an intentional raw-body/download surface, or Board should stop
  claiming it tested one. Documentation cannot fix this because the current API
  shape makes the intended behavior unrepresentable through the public typed
  path.

## Active Follow-On Work From This Checkpoint

- Clean stale live-state terminology in `crates/vorma-build/src/live_state_command.rs`.
  Use app-server executable language after the single-binary collapse.
- Clean stale API-mount wording in `crates/vorma/src/public_app.rs` and any
  docs that still describe resource patterns as relative to an API mount.
- Resolve the Board attachment gap from first principles. The framework design
  question is how resources should intentionally return non-JSON bodies without
  weakening typed JSON ergonomics or introducing a footgun.
- Continue the active `vorma-tasks` workstream recorded in
  `FABLE_TASKS_NOTES.md`, but keep its timeline distinct from `fable-1`:
  constructor split and `Task::singleflight` are still absent in the committed
  tree, while spawned `run_parallel` and SipHash-family `DefaultHasher`
  fingerprints are already present in `fable-1`.
- Preserve the dev-loop invariants from this commit when making build changes:
  no separate build-entry compile, no Vite restart for public asset saves,
  transactional activation with rollback, and event-driven file/shutdown/child
  handling.
- Preserve the client base-fact architecture. Split or move TypeScript code only
  when a module owns real state or behavior, not because a file is long.
- Preserve matcher sovereignty. Framework-specific routing policy belongs in
  Vorma graph/runtime code, not in matcher APIs unless the matcher concept is
  independently general.

## Checks Not Run For This Note

This was a committed-tree review and maintainer-doc augmentation pass. No Rust,
TypeScript, e2e, or gate checks were run for fable-1 itself during this note
write. The only appropriate verification for this pass is docs hygiene after
the note is written.

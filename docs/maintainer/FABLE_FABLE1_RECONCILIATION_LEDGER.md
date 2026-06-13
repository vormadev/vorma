# Fable-1 Claim Reconciliation Ledger

This ledger is the authoritative record for the notes-led, diff-verified
`fable-1` review. The unit of coverage is a Fable-note claim or workstream, not
raw lines of diff. A claim is only "confirmed" after the referenced committed
code and tests have been read closely enough to explain the behavior and its
pitfalls.

Commit under review:
`eff2c2f8d6edeb0f98c62af7cd9c46c183175844` (`fable-1`, authored 2026-06-13).
The current staged/working-tree diff is intentionally excluded.

## Status Vocabulary

- `not-started`: claim identified from maintainer notes, no committed-code
  reconciliation yet.
- `in-progress`: relevant code paths are being read, but the claim has not been
  reduced to confirmed/corrected/unresolved.
- `confirmed`: the code and tests support the Fable-note claim as written.
- `corrected`: the code differs from the note or earlier preliminary map; the
  corrected forward fact is recorded.
- `unresolved`: the code/test evidence exposes a real gap, contradiction, or
  missing proof that future work must handle.
- `mechanical`: generated, vendored, or purely mechanical churn inspected only
  as needed because no Fable-note claim depends on its internals.

## Claim Coverage

| Claim / workstream | Source note anchor | Code evidence read | Status | Forward fact or gap |
| --- | --- | --- | --- | --- |
| Single app-linked binary collapse: dev rebuild compiles the app-server target once, then runs that executable for live-state and serving; no separate hot-loop build-entry compile exists. | `FABLE_HANDOFF.md`, `FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md`, `FABLE_API_BOARD_NOTES.md`, `REMINDERS.md` | `crates/vorma-build/src/app_server_build.rs`, `crates/vorma-build/src/entrypoint.rs`, `crates/vorma/src/live_state_emit.rs`, `crates/vorma/src/public_app.rs`, `crates/vorma/src/lib.rs` `build_interface` module | confirmed | Dev rebuild builds exactly one cargo bin target from app config, reads live-state from that executable, then uses the same executable as the prebuilt app server. The build entry remains the session-start orchestrator surface, not a second hot-loop app-linked target. |
| Live-state protocol is app-server-emitted JSON with a protocol check and error envelope; protocol version in committed code must be reconciled against older v2 references. | `FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md`, `FABLE_API_BOARD_NOTES.md`, `REMINDERS.md` | `crates/vorma-contract/src/live_state.rs`, `crates/vorma/src/live_state_emit.rs`, `crates/vorma-build/src/live_state_command.rs`, `crates/vorma-contract/src/live_state.rs` tests | confirmed | Committed protocol is `LIVE_BUILD_STATE_PROTOCOL = 3`. App errors are JSON envelopes on stdout with exit 0. Decoding checks envelopes before state shape, rejects protocol mismatch precisely, recompiles the graph, and rejects empty root-document hash source. |
| Dev loop is event-driven, cancellable, transactional, and uses a stable mux for zero-downtime backend swaps. | `FABLE_HANDOFF.md`, `FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md`, `REMINDERS.md` | `crates/vorma-build/src/entrypoint.rs`, `crates/vorma-build/src/dev_build.rs`, `crates/vorma-build/src/dev_mux.rs`, `crates/vorma-build/src/dev_watcher.rs`, related Rust tests | confirmed | File events, shutdown, and child exits drive the loop. App-server rebuilds can be cancelled; static/revalidation changes queue without cancellation. Activation starts the candidate backend before switching the mux and rolls disk/RPC state back on failure. |
| Public-asset saves must not restart Vite; freshness comes from asset-to-module edges and targeted `/assets-changed` invalidation. | `FABLE_HANDOFF.md`, `FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md`, `FABLE_API_BOARD_NOTES.md`, `REMINDERS.md` | `crates/vorma-build/src/dev_build.rs`, `crates/vorma-build/src/vite_plugin_control.rs`, `crates/vorma-build/src/vite_plugin_rpc.rs`, `crates/vorma-build/src/vite_plugin_contract.rs`, `packages/vorma/vite/vite.ts`, `packages/vorma/vite/public_url_resolution.ts`, `packages/vorma/vite/vite.test.ts`, dev-build tests | confirmed | Rust sends `/cfg-changed` only when `VitePluginConfig` changes. Public filemap-only changes update the RPC filemap and send changed source keys to `/assets-changed`; the TS plugin invalidates exactly modules that recorded those assets as transform-time consumers. |
| API mount/root was removed: resources declare full URL patterns in the same observable URL space as views. | `FABLE_HANDOFF.md`, `FABLE_API_BOARD_NOTES.md` | `crates/vorma/src/config.rs`, `crates/vorma-contract/src/framework_graph.rs`, `crates/vorma-contract/src/execution_plan.rs`, `crates/vorma/src/execution_engine.rs`, routing tests | confirmed | No `api_base`/mount field exists in public config, graph config, execution plan, or classifier. Resource patterns are full route patterns such as `/api/...` only because the app spells them that way. |
| GET/HEAD resource and view conflicts are only illegal when overlapping patterns tie under `compare_specificity`; dispatch uses the same ordering. | `FABLE_HANDOFF.md`, `FABLE_API_BOARD_NOTES.md`, `FABLE_MATCHER_NOTES.md` | `crates/vorma-contract/src/graph_validation.rs`, `crates/vorma/src/execution_engine.rs`, routing tests | confirmed | Graph validation checks only GET/HEAD resources against views and rejects equal-specificity overlaps with a witness path. Runtime compares best view leaf pattern and resource pattern with `compare_specificity`; equal is unreachable except as deterministic fallback. |
| Scoped middleware is declarative via optional plural `patterns` and `methods`, with AND semantics; scope captures do not become route params. | `FABLE_HANDOFF.md`, `FABLE_API_BOARD_NOTES.md` | `crates/vorma/src/public_app.rs`, `crates/vorma-contract/src/framework_graph.rs`, `crates/vorma-contract/src/execution_plan.rs`, `crates/vorma/src/execution_engine.rs`, middleware tests | confirmed | Middleware declarations keep optional pattern and method filters; execution uses one flat matcher per scoped middleware and preserves declaration order. Selection matches request URL exactly with no hidden rewriting. Handler params/splats remain route facts. |
| Middlewares, view handlers, and client loaders are parallel by contract, not only as an optimization. | `REMINDERS.md`, `FABLE_HANDOFF.md` | `crates/vorma/src/execution_engine.rs`, middleware/view execution tests, `packages/vorma/core/create_client_core.ts`, `packages/vorma/core/client_loaders.ts`, `packages/vorma/core/route_modules.ts`, `packages/vorma/core/core_client_loaders.test.ts` | confirmed | Runtime middlewares and route handlers launch phase siblings concurrently and commit in order. Client loaders start known matches before the server response when possible; discovered loaders launch together after module import and are awaited before route commit. |
| `vorma-contract` owns framework graph, validation, execution plan, runtime manifest, wire/live-state contracts, and tsgen model; remaining `__private` use is macro ABI only. | `FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md`, `FABLE_HANDOFF.md` | `crates/vorma-contract/src/lib.rs`, `framework_graph.rs`, `execution_plan.rs`, `runtime_manifest.rs`, `wire.rs`, `contracts.rs`, `tsgen.rs`, `live_state.rs`, `crates/vorma/src/lib.rs`, `crates/vorma-macros/src/app_decl.rs` | confirmed | `vorma-contract` is the shared build/runtime truth crate. `vorma::build_interface` is the hidden named contract for `vorma-build`; `vorma::__private` remains only for macro-emitted paths such as route input resolvers and static handler runners. |
| Runtime error/exit protocol: `ViewExit` has no status, `HttpExit` carries HTTP status, redirects ride exit/effects, and resource errors use JSON `{ "error": ... }`. | `FABLE_HANDOFF.md`, `FABLE_API_BOARD_NOTES.md` | `crates/vorma/src/exit.rs`, `crates/vorma/src/response_finalizer.rs`, `crates/vorma/src/execution_engine.rs`, `crates/vorma/src/resource_response.rs`, exit/finalizer/resource-response tests | confirmed | `ViewExit` cannot carry status; `HttpExit` can. Terminal resource/middleware errors become response effects and finalize as JSON envelopes. Redirects are terminal effects, not fabricated success data. |
| TypeScript client core preserves a base-fact architecture and satellite modules own real state/behavior; do not split by line count alone. | `FABLE_HANDOFF.md`, `FABLE_WORKSTREAMS.md` | `packages/vorma/core/create_client_core.ts`, `packages/vorma/core/work_projection.ts`, `packages/vorma/core/client_loaders.ts`, `packages/vorma/core/route_modules.ts`, `packages/vorma/core/submissions.ts`, client-core tests | confirmed | `create_client_core.ts` still owns the transaction lifecycle and exactly nine named base facts. Work projection, client loader orchestration, route module materialization, and submissions are separate behavioral modules with their own state and tests. |
| TypeScript API wrappers and react-query bridge: `toIdentityArray` is the library-neutral cache key primitive; mutations fix endpoint identity at hook creation. | `FABLE_HANDOFF.md`, `FABLE_API_BOARD_NOTES.md` | `packages/vorma/core/api_client.ts`, `packages/vorma/core/api_client.test.ts`, `examples/board/src/client/api.ts` | corrected | Library-neutral `apiClient.toIdentityArray(args)` exists and is tested with stable identity serialization. Board's `useApiMutation` wrapper landed with endpoint identity fixed at hook creation. Board's `useApiQuery` wrapper did not land in `fable-1`; the committed comment says it lands with the search feature. |
| Matcher split and specificity doctrine: `FlatMatcher`, `NestedMatcher`, `find_overlap`, and `compare_specificity` are the public concepts; Vorma-specific policy stays layered above. | `FABLE_HANDOFF.md`, `FABLE_MATCHER_NOTES.md` | `crates/vorma-matcher/src/lib.rs`, `crates/vorma-matcher/src/builder.rs`, `crates/vorma-matcher/src/matcher.rs`, `crates/vorma-matcher/src/overlap.rs`, `crates/vorma-matcher/src/pattern.rs`, matcher tests | confirmed | The public matcher surface is typed flat/nested matchers plus sealed overlap sides and one specificity ordering. `find_overlap` checks candidate witnesses by replaying the real matchers, and specificity tests assert agreement between `compare_specificity` and flat dispatch. |
| Board pressure-test landed as the realistic HN-shaped app and should expose framework/API gaps rather than forcing app code through bad APIs. | `FABLE_HANDOFF.md`, `FABLE_API_BOARD_NOTES.md`, `FABLE_WORKSTREAMS.md` | `examples/board/src/lib.rs`, `examples/board/src/views.rs`, `examples/board/src/resources.rs`, `examples/board/src/session.rs`, `examples/board/src/repo.rs`, `examples/board/tests/app.rs`, Board client files | corrected | Board landed as a substantial server/request pressure test with SQLite, sessions, docs, moderation, search, FormData, redirects, scoped middleware, tasks, and request tests. `fable-1` does not prove the full browser/frontend app: eight declared `client_file` modules are absent from the committed tree. |
| Board attachment download claims a non-JSON resource body, but committed code/test evidence may not prove the bytes/content type. | `FABLE_FABLE1_COMMIT_NOTES.md`, `FABLE_API_BOARD_NOTES.md` | `examples/board/src/resources.rs`, `examples/board/src/repo.rs`, `examples/board/tests/app.rs`, `crates/vorma/src/typed_handler.rs`, `crates/vorma/src/resource_response.rs` | unresolved | The stored attachment has `file_name`, `content_type`, and `body`, but the resource handler returns `Ok(())` after setting only `content-disposition`. The test asserts status and filename header only. `fable-1` does not prove raw bytes or stored content type can be emitted through the public typed resource path. |
| Board task usage exposes task API friction: `Task::new(Duration::ZERO, ...)`, missing `From<E>` ergonomics, override wrapping, and `DbInput` keying caveat. | `FABLE_TASKS_NOTES.md`, `FABLE_HANDOFF.md` | `examples/board/src/repo.rs`, `examples/board/tests/app.rs`, `crates/vorma-tasks/src/error.rs`, `crates/vorma-tasks/src/task.rs`, `crates/vorma-tasks/src/overrides.rs` | corrected | `Task::new(Duration::ZERO, ...)` sentinel friction and `DbInput` keying caveat are real. The missing-`From<E>` claim is stale for `fable-1`: `TaskError<E>` implements `From<E>`, so Board's `blocking_task` helper and manual override wrapping are stale call-site friction, not missing core capability. |
| Active tasks-crate workstream after `fable-1`: constructor split, `singleflight`, spawned `run_parallel`, SipHash restoration, loom proof preservation, benchmark artifact shape. | `FABLE_TASKS_NOTES.md`, `FABLE_WORKSTREAMS.md` | `crates/vorma-tasks/src/task.rs`, `crates/vorma-tasks/src/key.rs`, `crates/vorma-tasks/tests/tasks.rs`, `crates/vorma-tasks/Cargo.toml`, `Makefile` at `fable-1` | corrected | In the committed `fable-1` tree, `run_parallel` already spawns siblings with `tokio::task::JoinSet`, and task key fingerprints use `std::collections::hash_map::DefaultHasher`. Constructor split and `Task::singleflight` remain absent. Loom models, task benches, and direct mimalloc task-bench artifacts are not present in `fable-1`; they belong to later current-tree work, not this commit. |
| Stale comments/docs that still say "build-entry" or "API mount root" must be identified so future work does not revive old architecture. | `FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md`, `FABLE_API_BOARD_NOTES.md` | `crates/vorma-build/src/live_state_command.rs`, `crates/vorma-contract/src/live_state.rs`, `crates/vorma/src/public_app.rs`, `examples/board/src/session.rs`, `examples/board/src/resources.rs` | corrected | Confirmed stale "build-entry" wording remains in live-state command comments/errors/tests and live-state contract comments. Confirmed stale API-mount wording remains on `Resource::pattern()` and in Board comments. These are stale comments/docs, not runtime truth. |

## Reconciliation Notes

The notes below record the confirmed/corrected/unresolved facts from each
claim cluster reconciled against committed code.

### Single-Binary And Live-State

Confirmed against committed code:

- `app_server_build_command` builds one target with `cargo build -p <package>
  --bin <bin> --message-format=json-render-diagnostics`.
- `build_app_server_and_read_live_state_until_cancelled` parses Cargo artifact
  JSON for the requested bin, disambiguates by package name where Cargo exposes
  it, then calls the supplied live-state reader on the produced executable.
- `entrypoint.rs` obtains `server_cargo_package` and `server_cargo_bin` from
  the committed generation's build plan, calls the app-server build/live-state
  helper, and wraps the resulting executable in `PrebuiltAppServer`.
- `entrypoint.rs` rejects a live-state server target that differs from the
  session target with a restart-required error. It does not silently retarget
  the dev session.
- `public_app.rs` makes `App::from_config` check live-state mode before
  constructing `RuntimeHost`; in live-state mode it emits and exits.
- `live_state_emit.rs` emits successful live state or an app-error envelope to
  stdout and exits 0 in both cases.
- `live_state.rs` in `vorma-contract` owns the wire shape and protocol version.
  Protocol 3 is the committed truth; older v2 references are stale.
- The `build_interface` surface is an inline module in `crates/vorma/src/lib.rs`.
  It is the hidden but named contract for paired `vorma`/`vorma-build` versions;
  `__private` is still reserved for macro ABI paths.

Concrete cleanup from this cluster:

- `crates/vorma-build/src/live_state_command.rs` still says "build-entry
  executable" in module docs, function docs, enum docs, display strings, and
  test names.
- `crates/vorma-contract/src/live_state.rs` still says the env key requests
  live-state JSON from "the build entry" and that facts are emitted by a
  "build-entry process". Those comments are stale after the single-binary
  collapse.

### Dev Loop, Watcher, Mux, And Vite Freshness

Confirmed against committed code:

- `entrypoint.rs` starts one filesystem event thread, one child-exit callback
  path, and one shutdown listener. The main loop waits on these event channels;
  there is no polling loop.
- The dev settle debounce is 10ms and has a unit test asserting it stays at or
  below 10ms.
- `entrypoint.rs` coalesces queued file changes before starting a rebuild.
  During an in-flight rebuild, app-server generation changes cancel the current
  build and queue the merged change; static generation and client revalidation
  changes queue without cancelling the in-flight app-server rebuild.
- `dev_watcher.rs` compiles watch roots from every non-excluded watch entry in
  the build plan, including general watch patterns, server-recompile patterns,
  client-revalidation patterns, frontend inputs, view modules, public static
  inputs, declared assets, and critical CSS inputs. This confirms that a
  client-revalidation path such as Board's `seed-data/**/*` is watched even if
  it is not duplicated in the general `watch_patterns` vector.
- The watcher supports explicit parent-directory and absolute watch paths
  outside the app root. The root dir is the relative-resolution anchor, not a
  containment boundary.
- Running watcher roots are static. `DevFileWatchPlanHandle::replace` swaps the
  classification plan so generation-specific facts, especially critical-CSS
  imports, stay current without restarting the OS watcher.
- `dev_mux.rs` keeps browser traffic on a stable loopback port and proxies to
  the active app-server port stored in an atomic. Tests cover switching the
  active backend, streaming chunked bodies without buffering until end, and
  bidirectional upgrade proxying.
- `dev_build.rs` captures generated TypeScript, manifest, and candidate static
  outputs before activation. Rollback restores old bytes or removes files that
  did not exist before the candidate publish.
- Live-generation activation starts the candidate app server first, updates the
  Vite RPC generation contract, notifies the TS plugin, switches the mux
  backend, commits the generation, then terminates the retired app server. On
  plugin notification or mux-switch failure it restores the previous RPC
  contract, terminates the uncommitted candidate process, and rolls disk back.
- Static fast-path activation cannot change plugin config. It updates the Vite
  RPC filemap only when changed public source keys exist and sends
  `/assets-changed`; it does not send `/cfg-changed`.
- Rust tests pin the distinction:
  `view_module_list_change_restarts_vite_instead_of_invalidating_assets`
  expects `/cfg-changed` and no asset invalidation; public static update tests
  expect `/assets-changed` and no config-change restart.
- `vite_plugin_rpc.rs` token-gates RPC, serves `cfg` and `hash`, rejects
  unknown public assets with 404, limits request bodies to 64 KiB, and refuses
  to record an unreachable control port.
- `vite_plugin_control.rs` posts tokenized control requests to
  `/cfg-changed` and `/assets-changed`. The assets body is a JSON string array
  of changed public source keys.
- `packages/vorma/vite/vite.ts` records normalized public source path to
  consumer-file edges when transforming JavaScript `vormaPublicUrl(...)` calls
  and CSS `url("@public/...")` declarations. `/assets-changed` validates a JSON
  string array and invalidates only modules for recorded consumers. Unrelated
  assets cause no invalidation.
- `/cfg-changed` in the TS plugin calls `vite_server.restart()`. The control
  server intentionally stays open during restart via `vite_restart_in_progress`.
- JavaScript public URL calls must have exactly one static string argument.
  CSS public URLs preserve query and fragment suffixes after hash resolution.

### API Mount Removal, Specificity, Middleware, And Exits

Confirmed against committed code:

- Public `Config` has `root_dir`, `dist_dir`, `server_target`,
  `public_static_base`, `frontend_config`, `ts_gen_config`, and
  `dev_watch_config`. There is no `api_base` or API mount field.
- `FrameworkConfig` carries only `public_static_base` plus optional build
  inputs. `ResourceDeclaration::pattern()` and `ResourceNode::pattern()` are
  route patterns, not mount-relative fragments.
- `ExecutionPlan` registers resources by their declared full URL patterns.
  Its test `plan_matches_resources_at_full_url_patterns_and_head_get_fallback`
  verifies `/api/stories/:id` is matched as the actual route pattern and that
  HEAD falls back to GET when no HEAD resource exists.
- `validate_resource_reachability` checks only GET/HEAD resources against
  views. It first compares normalized view and resource patterns with
  `compare_specificity`; only equal-specificity pairs get overlap-checked with
  `find_overlap`, and only an actual overlap becomes
  `ResourceViewSpecificityTie`.
- Public static base conflicts are separate from route specificity. When the
  base is not `/`, GET/HEAD resources under that static prefix are rejected as
  `ResourceInsidePublicStaticBase`.
- `ExecutionEngine::classify` serves manifest-listed public assets first for
  GET/HEAD, then compares the best resource and view match with
  `compare_specificity`. Equal falls to the resource only as deterministic
  exhaustiveness because equal overlaps should not compile.
- 405 is path-derived: resource methods contribute allowed methods, and views
  or public assets contribute GET/HEAD.
- Middleware scoping is declarative. `Middleware::with_patterns` and
  `with_methods` lower to graph middleware declarations. Execution filters in
  declaration order; method and pattern filters AND together.
- Scope patterns match the request URL exactly. The test
  `scope_patterns_match_urls_with_no_hidden_rewrites` proves that a `/mod`
  scope does not cover `/api/mod/...`; apps must spell `/api/mod/*` if their
  resource URL lives there.
- Scope-pattern captures never become handler params. Resource middleware uses
  the resource route's params/splats; view middleware uses the matched view
  route's params/splats. In `view_execution`, middleware invocations record
  the leaf view pattern only as log/context text.
- Runtime middleware execution is phase-gated but parallel within the phase.
  `execute_invocation_phase` launches all siblings in a `FuturesUnordered`,
  stores outputs by phase position, and commits in declaration/route order.
- A terminal middleware suppresses later middleware and all handlers. A terminal
  view/resource handler suppresses later handlers in that handler phase.
- `ViewExit` carries no status by type. `HttpExit` carries optional status.
  Client-visible text is only `with_client_msg`; server record and source stay
  server-side.
- `ResponseEffects::set_status` is not control flow. `set_status_with_text`
  marks a terminal error. Redirects are terminal effects produced by context
  helpers and finalized through the same effects path.
- `resource_response.rs` turns terminal resource/middleware errors into
  `{"error": text}` JSON. Terminal redirects keep redirect responses. HEAD
  suppresses the body while preserving the would-have-been content length.

Concrete cleanup from this cluster:

- `crates/vorma/src/public_app.rs` still documents `Resource::pattern()` as
  "relative to the configured API mount root." That is false after the API
  mount removal and should be corrected before anyone uses that docstring as
  design evidence.
- `examples/board/src/session.rs` still says mod resources live under the "api
  mount", and `examples/board/src/resources.rs` says mod actions live under
  `/mod` while the actual resource patterns are `/api/mod/...`. The scoped
  middleware code is correct because it spells concrete URL patterns; the
  comments are stale API-mount-era language.

### TypeScript Client Core, Loaders, Submissions, And API Client

Confirmed against committed code:

- `create_client_core.ts` declares the base-fact model in code: phase, browser
  history entry, route snapshot, one active nav/revalidation fetch, one
  prefetch, refresh scheduler state, concurrent API requests, deferred submit
  redirect, and monotonic sequence. `WorkProjection` and `WorkState` are
  derived from those facts; `RouteRenderState` is only derived at commit time.
- The client split is behavioral, not cosmetic. `work_projection.ts` is a pure
  projection from an explicit `WorkSources` struct. `client_loaders.ts` owns
  the loader registry, search-schema registry, WASM matcher readiness, early
  prestart, prefetch reconciliation, server-state seeding, and loader result
  ordering. `route_modules.ts` owns dynamic imports, dev module cache, HMR
  version stamps, loader registration discovered during import, and immutable
  `RouteRecord` construction. `submissions.ts` owns API request lifecycle,
  dedupe replacement, JSON body normalization, redirect handling, build-skew
  reporting, and submission-triggered revalidation.
- Route preparation imports all unique view modules with `Promise.all`, then
  runs client loaders, then waits for CSS before commit. Loader registration
  happens only after a module import has materialized the view definition.
- Client-loader early start uses the currently known matcher facts. If the
  full target path has no known match, `find_view_match` tries shorter parent
  prefixes so a known parent loader can begin before the server payload for a
  deeper child path returns.
- After the server payload arrives, `client_loaders.run` reconciles retained
  prefetches by route pattern, starts missing route loaders without awaiting
  each one sequentially, then awaits the set through `Promise.allSettled`.
  A non-abort loader failure aborts later route loader signals; abort errors
  are swallowed and do not become route error state.
- Client loaders at and after the outermost server error are suppressed and
  their prefetches are aborted. `route_modules.build_route_record` prefers an
  outermost server error over any client-loader error; otherwise it records the
  outermost client-loader error.
- The loader tests prove the parallel contract directly:
  `starts all client loaders in parallel before any resolves`,
  `prestarts parent client loaders when navigating to deeper unregistered path`,
  `route update blocked by client loaders`, and server-error cancellation tests
  all assert observable ordering or abort behavior.
- `ClientLoaderServerState` includes `clientBuildId`, every matched pattern and
  input, every server `viewData`, `outermostServerError`, and the current
  route's own `viewData`. The older claim that typed own server data was lost
  is stale for `fable-1`.
- `submissions.ts` reads server resource failures from the JSON `{ "error": ... }`
  envelope and falls back to `Request failed (<status>)` for non-envelope error
  bodies. It intentionally does not rely on `statusText`.
- Submissions accept same-origin URLs only, default GET/HEAD to query semantics
  and other methods to mutation semantics, allow explicit `resourceKind` and
  `revalidate` overrides, abort and settle prior same-dedupe-key requests, and
  keep API-request work state continuous through dedupe replacement and
  submit-to-revalidation handoff.
- Submit bodies are stripped for GET/HEAD. Plain objects become JSON bodies on
  non-GET/HEAD requests unless the body is already a streaming/form/blob/buffer
  type; caller-provided content type is preserved.
- `api_client.ts` keeps the generated typed API client library-neutral. It
  normalizes method and pattern, builds resource URLs, merges decorator
  headers/credentials with per-call overrides, exposes `query`, `queryOrThrow`,
  `mutate`, `mutateOrThrow`, and preserves failed result objects on
  `QueryError`/`MutationError`.
- `apiClient.toIdentityArray(args)` returns a stable identity array prefixed by
  `API_IDENTITY_ARRAY_PREFIX` and containing normalized method, normalized
  pattern, stable JSON for params, stable JSON for splat values, and stable JSON
  for input. Tests assert nested-key stable serialization.
- Board's committed react-query bridge contains only `useApiMutation`. Its hook
  argument fixes method and pattern; `mutate()` variables carry params/input.
  The same file explicitly states the query twin belongs to the later search
  feature and should use `apiClient.toIdentityArray` for the query key.

Forward-going correction from this cluster:

- Do not say `fable-1` landed `useApiQuery`. It landed the typed cache-key
  primitive and the Board mutation wrapper. The query wrapper remained future
  work at the commit under review.

### Contract Crate And Hidden Runtime Interfaces

Confirmed against committed code:

- `crates/vorma-contract/src/lib.rs` defines the crate as the Vorma
  build/runtime boundary contract. It publicly owns constants, route/document
  contracts, trusted document rendering, the execution plan, canonical
  framework graph, live-state protocol, runtime manifest, TypeScript generation
  model, and wire payload types.
- `framework_graph.rs` owns declaration normalization for framework config,
  build inputs, middleware declarations, view declarations, resource
  declarations, static assets, document contracts, type definitions, and graph
  validation errors. There is no `api_base` in the graph config; resources and
  views are both route-pattern declarations.
- `execution_plan.rs` compiles the graph into one flat resource matcher per
  HTTP method, one nested view matcher, route handler maps, and scoped
  middleware plan nodes. Middleware scope matching is exact URL-space matching
  through a flat matcher, and HEAD falls back to GET resources when no HEAD
  resource exists.
- `runtime_manifest.rs` owns the server/browser manifest contract: client build
  id derivation, Vorma version, dev ports and refresh token, public static
  base, root document shell hash, public source map, critical CSS, search
  schemas, client entry/core assets, and per-pattern client view modules.
- Runtime manifest client build IDs are derived with blake3 over the serialized
  manifest and base32-nopad encoded. Critical CSS CSP hashes use SHA-256
  because CSP requires that hash family.
- `wire.rs` owns the browser payload and header/query constants:
  client-build-id header, build-skew header, client redirect header,
  accepts-redirect header, `vorma-json` query key, `HeadElement`,
  `ViewPayload`, and `SsrPayload`.
- `contracts.rs` owns the route type contract model, TypeScript type-reference
  model, TypeScript definition model, document contract model, and tab-indented
  stable JSON rendering used by generated artifacts.
- `tsgen.rs` owns the public TypeScript collection surface and trait model:
  `Type`, `TypePhase`, `TypeRegistry`, `TsExtraType`, and `TsDrafter`.
  It reexports contract `TypeDef`, `FieldDef`, raw TS parts, and type refs.
- `live_state.rs` owns the live build-state wire protocol and graph recompiles.
  Protocol 3 is current for `fable-1`.
- `crates/vorma/src/lib.rs` reexports `vorma_contract::tsgen` publicly and
  imports selected contract modules internally. Build tooling consumes
  `vorma::build_interface`, not `vorma::__private`.
- `vorma::build_interface` is the hidden named contract between paired
  `vorma` and `vorma-build` versions. It exposes build-facing assets,
  document contracts, config lowering, build/dev env helpers, facade types,
  route input resolvers, app-build-contract helpers, live-state construction,
  graph construction, and runtime snapshot internals.
- `vorma::__private` reexports only the path-stable macro ABI required by
  `vorma-macros`: matcher params, route input resolvers, input error,
  path-params trait, and static view/resource runners. The macro expansion in
  `crates/vorma-macros/src/app_decl.rs` uses those exact paths.

Forward-going pitfall from this cluster:

- Do not move build-facing surface back into `__private`. The current split is
  intentional: `build_interface` is the paired-crate contract and `__private`
  is macro ABI. Mixing them again would make future contract audits harder and
  would blur which internals are consumed by generated user code.

### Matcher Split, Overlap, And Specificity

Confirmed against committed code:

- `vorma-matcher` is a standalone crate with a small public surface:
  `MatcherBuilder`, `FlatMatcher`, `NestedMatcher`, `find_overlap`,
  `Overlap`, `OverlapSide`, `Pattern`, `compare_specificity`, options,
  segments, and match result types. The crate root does not expose Vorma
  framework concepts.
- `MatcherBuilder` is the shared grammar authority. It validates and normalizes
  route patterns, rejects normalized-pattern collisions, rejects same-shape
  dynamic route collisions, and finishes into either `FlatMatcher` or
  `NestedMatcher`.
- `FlatMatcher` exposes only whole-path best-match lookup through
  `find_best_match`. `NestedMatcher` exposes only ordered route-chain lookup
  through `find_nested_matches`. The shared `MatcherEngine` remains internal.
- Dirty-path behavior is part of matcher semantics: doubled slashes do not
  match, one trailing slash is tolerated as noise for non-index forms, empty
  path segments never become params or splat values, and splats require a real
  captured segment except the root catch-all's special root handling.
- `compare_specificity` is the single ordering over registered patterns: total
  segment score first, then the leftmost differing segment rank, then a
  trailing splat loses to a non-splat ending, then longer pattern wins.
  `Ordering::Equal` is not a runtime tiebreaker; with a shared path it names
  the same no-principled-winner condition that registration rejects inside one
  matcher and that Vorma graph validation rejects between views and resources.
- `find_overlap` accepts either matcher type in either position through a
  sealed `OverlapSide`. It returns a concrete example path plus the normalized
  pattern each side actually resolved for that path.
- `find_overlap` is exact by construction in the committed implementation. It
  enumerates a finite witness family from registered pattern pairs and tests
  each candidate by calling the real flat or nested matcher. This keeps overlap
  semantics aligned with matcher behavior rather than duplicating a separate
  router model.
- The overlap tests cover nested-vs-flat pins, flat-vs-flat pins,
  multi-pattern attribution, catch-all yielding only to covering matches,
  dynamic index overlap, fresh placeholder selection, and brute-force oracles
  across all four flat/nested pairings.
- The specificity tests pin the doctrine cases used by Vorma: static beats
  dynamic and splat, dynamic beats splat, root catch-all is the floor, and
  equal shape is a tie. The agreement oracle builds co-registered flat
  matchers and proves the winner for an overlap path is the one named by
  `compare_specificity`.
- `matching_semantics.rs` includes property-model checks for best-match and
  nested semantics, registration-order independence, generated pattern cases,
  explicit index options, and collision rejection. That test suite is not just
  compatibility with old Go behavior; it is the current matcher semantic proof.

Forward-going pitfall from this cluster:

- Do not put Vorma resource/view policy into `vorma-matcher`. Vorma may use
  `find_overlap` and `compare_specificity` to validate its graph, but matcher
  docs and APIs should stay in matcher vocabulary: flat matcher, nested
  matcher, registered pattern, overlap witness, and specificity.
- API-mount-free Vorma routing depends on matcher correctness. Weakening
  overlap exactness or introducing a second specificity implementation would
  directly undermine resource/view conflict validation and runtime dispatch.

### Board Pressure Test And Its Real Gaps

Confirmed against committed code:

- Board is a real HN-shaped server/request pressure test, not a toy route list.
  `app_config_with` wires SQLite-backed state, generated extra TypeScript,
  full frontend config, dev watch patterns, eleven views, nine resources, two
  Vorma middlewares, a document, request body limit, and injectable task
  options.
- Board's dev-watch shape exercises the single-binary/dev-loop work: general
  watches cover `src/**/*` and `public/**/*`; server recompilation covers Rust
  and `schema.sql`; client revalidation covers `seed-data/**/*`.
- Board server startup wraps the Vorma app as an Axum fallback service and
  layers real transport middleware: sensitive headers, request id, trace,
  secure headers, panic recovery, request body limit, etag, response body
  timeout, compression, request body timeout, and handler timeout.
- Session auth is passwordless demo auth stored in SQLite with an HttpOnly
  `board_session` cookie. `current_user_preload` starts the session-user task
  in middleware; layout/resources call the same task and get request-scope
  dedupe.
- `mod_gate` is the scoped middleware proof. It gates `/mod`, `/mod/*`, and
  `/api/mod/*` explicitly. Anonymous requests redirect home; banned users get a
  status-bearing HTTP error envelope.
- Views exercise nested layout data, front-page search params, soft not-found
  rendering, head effects, parallel task prewarming, user profile nesting,
  docs splats, URL-state-only search shell, mod queue data, and a deliberate
  child-segment failure whose parent data still renders.
- Resources exercise login/logout, FormData story submission, redirect after
  mutation, story voting with duplicate conflict, comment creation, GET query
  resource search, scoped moderation mutations, and attachment lookup.
- The repo layer intentionally makes reads `Task`s and writes plain async
  functions. Reads use `Duration::ZERO` for request-scope dedupe except docs
  pages/index, which use a 30-second TTL because seeded docs are deliberately
  stale-tolerant.
- Board request tests cover session layout, anonymous layout, auth rejection,
  submit redirect and story rendering, missing story soft state, vote dedupe
  envelope, ranking, logout, validation envelopes, comments, search,
  moderation gate/kill/restore/banned envelope, docs splats, segment error
  isolation, attachment header status, user-page nesting, and task override
  failure text.
- Client code that exists proves the app-owned mutation wrapper idiom: layout
  uses login/logout mutations, front page uses one vote mutation hook for a
  whole list, and nprogress is wired through Vorma's work indicator. Jotai owns
  theme state separately from route state.

Corrected facts and unresolved gaps:

- `fable-1` does not prove the full Board browser app. `views.rs` declares
  client files for `/submit`, `/u/:username`, `/u/:username/comments`, `/docs`,
  `/docs/*`, `/search`, `/mod`, and `/mod/diagnostics`, but the committed tree
  contains only `layout.view.tsx`, `front.view.tsx`, and `story.view.tsx`.
  The Board request tests instantiate `TestApp` and do not exercise Vite
  module imports, so they cannot catch missing client modules.
- The generated `vorma.gen.ts` contains type entries for all eleven views and
  all nine resources. That proves Rust graph/type generation knew about the
  routes; it does not prove the corresponding client modules existed.
- The attachment download resource stores uploaded file bytes and content type
  in `repo::Attachment`, but `STORY_ATTACHMENT` returns `Ok(())` after setting
  only `content-disposition`. The request test uploads `hello attachment` and
  asserts only status plus filename header. It does not assert response bytes
  or `content-type`.
- The public typed resource path has no Board-side evidence for returning raw
  bytes from a handler. Future work must design and implement an intentional
  raw-body/download surface, or remove the claim that Board covers non-JSON
  resource responses.
- Board comments still contain stale API-mount language. `session.rs` says mod
  resources live under the "api mount"; `resources.rs` says mod actions live
  under `/mod` while their actual patterns are `/api/mod/...`. The code is
  correct because it spells concrete URL patterns; the comments should be
  corrected so they do not revive the deleted API mount concept.

### Tasks Crate And Board Task Call Sites

Confirmed or corrected against the committed `fable-1` tree:

- `vorma-tasks` in `fable-1` is a standalone async task runtime with
  execution-context memoization, optional cross-execution-context caching,
  cancellation, typed overrides, parallel prepared-task execution, and passive
  observation.
- `Task::new(cross_exec_ctx_cache_ttl, f)` is still the only constructor.
  Board therefore has many `Task::new(Duration::ZERO, ...)` call sites for
  request-scope-only dedupe and two nonzero TTL docs tasks. The constructor
  split discussed in the Fable notes did not land in `fable-1`.
- `Task::singleflight` does not exist in `fable-1`. Cross-execution-context
  coalescing is tied to nonzero TTL caching; there is no public constructor for
  coalescing concurrent duplicate work without retaining the completed success.
- `ExecCtx::run_parallel` in the committed tree uses a single-task fast path and
  otherwise spawns siblings into `tokio::task::JoinSet`. The first failing
  sibling cancels the shared child token, panics are resumed, and non-panic join
  errors become cancellation if no application error already won.
- The `run_parallel` tests prove concurrent start, result sinks, request-scope
  memoization of prepared results, original error preservation, sibling
  cancellation, and parent cancellation propagation.
- `key.rs` uses `std::collections::hash_map::DefaultHasher` for task key
  fingerprints and then verifies equality through the stored typed key value.
  The test `equal_hashes_still_use_input_equality` proves hash collisions do
  not alias task inputs. The Fable-note claim that `fable-1` still used
  `rustc_hash::FxHasher` is stale for the committed tree.
- `TaskError<E>` implements `From<E>` in `crates/vorma-tasks/src/error.rs`.
  Board's `blocking_task` helper comment says it exists because `From<E>` is
  missing, but that is false for `fable-1`; the helper and comment are stale
  call-site friction.
- Board's task override test manually returns
  `TaskError::Failed(Arc::new(vorma::Error::new(...)))`. Because `From<E>`
  exists, the core capability for `?`/`into()` is present; the call site simply
  has not been simplified.
- Board's `DbInput<I>` intentionally hashes and compares only `input`, not the
  `Arc<Db>`. That is safe for Board's single-database-per-process shape. It is
  not a general-purpose pattern for apps with multiple independent database
  handles in one task runtime.
- The committed `fable-1` tree has no `crates/vorma-tasks/src/loom_tests.rs`,
  no `crates/vorma-tasks/benches/tasks.rs`, no `loom-tasks` Makefile target,
  no task-bench target, and no direct `mimalloc` dependency in `vorma-tasks` or
  default-on `mimalloc` feature in `vorma`. Those artifacts appear in the later
  current tree, not in the commit under this ledger.

Forward-going task facts:

- The remaining `fable-1`-visible tasks API design work is constructor naming
  and `singleflight`. Do not describe spawned `run_parallel` restoration or
  SipHash restoration as missing from `fable-1`; they are already true in the
  committed tree.
- The current maintainer task notes contain statements that align with later
  working-tree changes, not the `fable-1` committed tree. Before changing
  tasks, reconcile against the actual worktree/diff in that moment rather than
  using this `fable-1` ledger as a statement about uncommitted task changes.

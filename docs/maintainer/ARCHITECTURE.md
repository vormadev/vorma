# Vorma Architecture

Maintainer-facing map of the current system. Keep this file updated in the same change as
architecture-level edits so later agents do not have to reconstruct current truth from
historical plans.

## Crate and package layout

- **`vorma`** — the runtime and the app declaration API. Apps declare
  views/resources/middleware/document through `AppConfig` and serve via `App::from_config`
  → `RuntimeHost` (a Tower service mounted in the user's own axum stack). Also exposes:
    - `vorma::middleware` — optional Tower layers (etag, secure headers, body limits,
      timeouts, compression, request id, panic recovery).
    - `vorma::build_interface` — the named contract `vorma-build` consumes (asset
      capabilities, document contracts, config lowering, the facade that compiles
      declarations, route-input resolvers, runtime snapshot types, `AppBuildContract` and
      the live-state constructors). Real and documented, but not for app code; the two
      crates ship version-locked.
    - `vorma::__private` — macro ABI only: the exact paths `vorma-macros` emits into
      application crates. doc(hidden), path-stable.
- **`vorma-contract`** — shared truth between runtime and build: the canonical
  `framework_graph`, compiled `execution_plan`, `runtime_manifest`, declaration
  `contracts`, `tsgen` type model, `document_renderer`, `wire` (browser protocol),
  `live_state` (the app-binary → build protocol), and shared constants.
  `extern crate self as vorma` satisfies the TsGen macro ABI locally.
- **`vorma-build`** — the build/dev orchestrator library. No CLI, by ruling: the public
  surface is `run(app_config)` (plus `build_production`/`start_dev_server` for harnesses).
- **`vorma-macros`** — `TsGen` derive and the view/resource declaration macros.
  Diagnostics are compile-fail-pinned in `crates/vorma/tests/compile_fail/`.
- **`vorma-matcher`, `vorma-tasks`, `vorma-client-wasm`** — standalone route matcher
  (shared with the browser via WASM), task runtime, and the WASM bridge.
- **`packages/vorma`** — the TS package: `core/` (client runtime),
  `ui/{react,preact,solid}` adapters, `vite/` plugin, `kit/`. Generated wire contracts
  land in `core`/`vite` as `*.gen.ts` goldens (`VORMA_UPDATE_WIRE_CONTRACTS=1`
  regenerates).

## The single-binary model

There is exactly one app-linked binary: the user's server. It has two modes, switched by
one env key:

1. **Live-state mode** (`__VORMA_LIVE_BUILD_STATE=1`): `App::from_config` serializes the
   declaration graph + root-document hash source to stdout and exits
   (`vorma_contract::live_state`). Errors travel as an error envelope with exit 0; the
   build side distinguishes by payload.
2. **Serve mode**: normal `RuntimeHost` construction.

The per-app build entry (`src/bin/build.rs`) is a thin shim calling
`vorma_build::run(app_config)`. It never answers live-state and is never rebuilt
mid-session. Process roles per dev iteration:

- build entry (long-lived orchestrator)
    - `cargo build -p <server-pkg> --bin <server-bin>` (one compile, artifact path from
      cargo's JSON messages)
    - server binary, run once with the env key → live state
    - server binary, run again → the actual app server for the generation

The server cargo target comes from app config (`ServerBuildTarget`), read in-process at
session start (the Go bootstrap pattern: the first read is guaranteed fresh because it IS
the current process's config). A mid-session target change fails the rebuild with a
restart instruction rather than silently compiling the stale target.

## Dev loop

Event-driven, no polling anywhere:

- `notify` watcher with session-static OS roots and a **swappable classification plan**
  (`DevFileWatchPlanHandle`): per-generation facts — view modules, declared assets,
  critical-CSS imports — re-enter the classifier after every rebuild without touching the
  OS watcher.
- Changes coalesce in the loop (`drain_pending`); app-server-class changes cancel the
  in-flight build, static/revalidate changes queue and merge.
- Work classification: server recompile → full generation; public-static / critical-CSS
  intents → static fast path (no app-server restart); client-revalidate → broadcast only.
- Child processes are exit-watched without reaping (linux `waitid`+WNOWAIT, macOS kqueue
  NOTE_EXIT) with a terminate-intent flag; an unexpected app-server or Vite death fails
  the session loudly.
- Generations are epochal: a candidate is fully prepared and published (typescript
  contracts, static outputs, manifest) before the supervisor commits it; the
  browser-facing dev mux atomically switches backends, so rebuilds are zero-downtime
  (deliberately better than Go's stop-then-start on a stable port). Failed activations
  roll back disk and contract state; the previous generation keeps serving. When a
  `dist_dir` move commits, the orphaned previous output dir is deleted only then — never
  at build time, because the old generation serves from it until the swap.

## Vite freshness model

The plugin (one Vite plugin, dev and prod) resolves `vormaPublicUrl()` in JS and
`url(...)` in CSS through a JIT `hash` RPC against the dev server, recording asset→module
edges as it goes. Public-asset saves NEVER restart Vite:

- changed source keys go to the plugin's `/assets-changed` control endpoint, which
  invalidates exactly the recorded consumer modules;
- retained stale hashed outputs keep already-cached transforms resolving;
- `/cfg-changed` (full Vite restart) fires only when the `VitePluginConfig` payload itself
  changed (entry module, view-module set, ignored patterns, dedupe list, public base).

This closes the Go era's staleness/404 window (GO-BUG, fixed properly) and removes its
restart-per-rebuild bluntness.

## Runtime

`RuntimeHost` serves committed generations from the manifest: hashed public statics
(immutable cache headers), the document shell rendered from `DocumentContract`
(CSP-hashable critical CSS), and the execution engine driving middlewares → view/resource
handlers with parallel client-loader semantics per the REMINDERS contracts.

Response protocol (post API-campaign): views are a framework-owned rendering protocol — a
view segment rejects with `ViewExit` (explicit `client_msg` or a generic; page still
commits, HTTP 200) and only framework faults 500 on the view path. Resources and
middlewares speak real HTTP via `HttpExit` (status + client msg), and resource-path errors
reach the wire as the JSON envelope `{"error": text}` that the TS client parses. Redirects
ride the exit channel (`ctx.redirect`). Statuses are never control flow: terminal =
engine-owned error flag or a real redirect.

Middlewares are selected per request AFTER route matching: each declaration carries
optional plural `patterns`/`methods` filters (omitted = unrestricted, provided AND
together), compiled into per-middleware flat matchers in the ExecutionPlan; selection is a
boolean path/method check (declaration order preserved), then the selected middlewares run
as the parallel pre-handler phase sharing the request's single task scope with handlers.
No route match -> no Vorma middlewares; transport-level concerns belong to the tower/axum
layer. The dev-only pieces (refresh websocket, `/.vorma/healthz`, build overlays) are
env-gated and compiled out of the prod path where possible.

## Client core

`packages/vorma/core` is decomposed into focused modules (revalidation scheduler, work
projection, redirects, wire payload, client loaders, route modules, submissions, history
position, head, css). The old monolithic companion test suite has been decomposed into
focused suites such as `core_boot.test.ts`, `core_client_loaders.test.ts`,
`core_navigation.test.ts`, `core_revalidation_and_work.test.ts`,
`core_scroll_and_history.test.ts`, and the lower-level module tests beside them.

## Known asymmetries (deliberate)

- Windows: no child-exit watcher yet (needs a SYNCHRONIZE-handle wait); dev works, death
  detection degrades to next-request failure.
- The OS watch-root set is session-static; a brand-new watch ROOT (e.g. a config change
  pointing at a new directory) needs a dev restart — same reach as Go, guarded by the
  explicit config-transition error.
- The `runtime_*` module prefix in `vorma` is deliberate: it keeps crate-private runtime
  layers visually grouped and distinct from the public app-declaration surface.

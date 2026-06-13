# Current Plan — First-Principles Improvements

Source: the full-repo entropy review performed on this branch. The predecessor initiative
— the rewrite regressions audit and restorations — is complete; see
[REGRESSIONS_AUDIT.md](REGRESSIONS_AUDIT.md).

Ratified execution order: A → B → E → C → D → F (letters are stable labels from the
review, not the run order; E runs directly after B because it is the highest-entropy item
and is independent of the Rust-side phases C/D). Rulings: full `vorma-contract` crate
extraction (not a minimal `__private` shrink).

## Review TL;DR (preserved)

The post-rewrite skeleton is strong — the event-driven dev loop is disciplined, the
wire-contract golden-test pattern is excellent, and the small crates (`vorma-matcher`,
`vorma-tasks`) are clean. Entropy concentrates in four places, and three share one
disease: **the framework's real internal contracts are implicit instead of owned.** The
single highest-entropy artifact is
[create_client_core.ts](../../packages/vorma/core/create_client_core.ts) (a ~2,580-line
closure that is the entire browser runtime). The wrongest center of gravity is the
`__private` module pair — the de-facto architecture pretending to be a hidden
implementation detail.

## Phase A — Quick wins

- [x] Replace the "Isolated build/dev rewrite holding ground" crate doc with a real one
- [x] Remove the dangling `TEST_README.md` link in `tests/framework/README.md`
- [x] Convert the `testing.rs` ` ```ignore ` doc example into a **running** in-memory
      doc-test (the workspace's one standing ignored test is gone)
- [x] Restructure this document into the phase tracker
- [x] Delete dead `vorma_build::__private` — **done.** The deletion unmasked the crate's
      real internal surface (~35 dead-code findings the glob re-export had kept "publicly
      reachable" and therefore invisible to dead-code analysis); all of them are
      adjudicated in the Phase B opener record below, and the module is gone for good.

### Phase B opener: dead-surface adjudication — COMPLETE

`vorma_build::__private` is deleted for good. The crate's public surface is exactly
`pub use entrypoint::{BuildOptions, run}` (matching both external consumers:
`tests/public_api.rs` and `examples/minimal/src/bin/build.rs`), and
`cargo clippy -p vorma -p vorma-build --all-targets -- -D warnings` is clean with no
keep-alive re-exports masking anything.

How every flagged item resolved:

- **Deleted (true cruft, zero users anywhere):** the in-process
  `activate_next_generation`/`AppBuildContract` dev-update path; the duplicate
  `DevBrowserRefreshMessage` representation (payloads are the single refresh truth; tests
  rewritten against `committed_generation().browser_refresh_effects()`);
  `BuildOutputWriteReport::vorma_output_gitignore_path`;
  `DevGenerationError::GenerationInputs` and `ProductionGenerationError::BuildInputs`
  (uncreatable variants); `GenerationCandidate::from_live_graph` + supervisor
  `build_next_live_graph_candidate` + `activate_next_complete_live_graph_candidate`
  (superseded by the precomputed-projection path); candidate getters
  `id`/`graph`/`projections`/`artifacts`; `GenerationArtifacts::with_dev_metadata` and
  `client_entry`; `DevMuxServer.refresh_token` field+getter (the token reaches clients via
  the endpoint path; the mux tests pin it behaviorally); `DevFileChange::paths` and the
  watch plan's `root_dir` getter; `vite_plugin_rpc::update_from_prepared_build_inputs`
  (both activation sites call `update_generation_contract` directly).
- **`#[cfg(test)]` (legitimate test observers/infrastructure):** report getters in
  `build_output` (4), `production_build` (2), `production_generation` (2),
  `dev_generation` (2); `AppBinaryExecutables::build_entry`; `OutputLockGuard::lock_path`;
  `read_live_build_state_from_executable`; `RefreshPayload::critical_css`/`build_error`
  and `DevRefreshClientSubscription::try_recv`; `DevMuxServer::port` (reads a retained
  `_port` field)/`active_app_server_port`/`add_test_client`; `ViteInputPlan::ui_variant`;
  `StaticInputPlan::declared_assets`; `DevFileChange::requires_only_client_revalidation`;
  static-output getters (`source_path`, `public_path`, `imports`, publish-report trio);
  `{RESOURCE,VIEW}_CONTRACTS_TYPE_NAME` (negative-assertion pins: tests assert the names
  do NOT appear in generated source); `VitePluginRpcServerHandle::new` (test-mock
  constructor); `BrowserRefreshEffects::generation_id`; committed generation
  `id`/`typescript_contracts`/`effects`/`runtime_snapshot{,_input}`; the supervisor's
  simple-candidate chain — `GenerationCandidate::new`, `from_app_build_contract`,
  `committed`, `build_next_candidate`, `build_next_app_candidate`,
  `activate_next_complete_{candidate,app_candidate}` — plus
  `GenerationError::{BuildPlan,TypeScriptContracts}` (constructed only by that chain);
  projection test observers `document`, `view_payload_patterns`, `dev_watch_plan` +
  `DevWatchPlan`, `ResourceContract::kind`/`input_schema`,
  `GenerationArtifacts::with_client_entry`/`with_client_core_assets`. Production code
  constructs generations exclusively through the `*_with_precomputed_projections`
  builders + `activate`; the simple chain is the epoch test suite's harness.
- **`_`-prefix (ownership-only):** `DevGenerationHandle._vite_plugin_rpc_server` (Drop
  shuts the RPC server down with the generation); `DevMuxServer._port` (production callers
  know the port they asked the mux to bind).
- **Rewired (duplicate eliminated):** `static_outputs` now imports
  `VITE_PLUGIN_PUBLIC_URL_PREFIX` from `vite_plugin_contract` instead of redefining the
  `"@public/"` string as `CRITICAL_CSS_PUBLIC_URL_PREFIX`.

Verification at close: clippy `-D warnings` clean across both crates (all targets),
vorma-build 130+1 tests ok, vorma 237+1+5+6+1+1 tests ok, fmt clean. One residual
`vorma::__private::runtime` import in `generation_epoch.rs` is load-bearing
(`RuntimeSnapshot::compile` validates candidates) — it moves into the contract crate in
Phase B proper.

## Phase B — The contract crate — COMPLETE

The build↔runtime boundary now lives in `crates/vorma-contract`, a dedicated crate both
`vorma` and `vorma-build` depend on:

- **Moved wholesale** (the files formed a closed dependency island): `framework_graph`,
  `execution_plan`, `runtime_manifest`, `contracts`, `constants`, `tsgen`,
  `document_renderer`. Internal `crate::X` paths inside `vorma` keep resolving through one
  `pub(crate) use vorma_contract::{…}` shim. `FORM_DATA_TYPE_NAME` moved into
  `vorma_contract::constants`; the `impl Type for FormData` moved next to `FormData`
  (orphan rules). `ExecutionPlan::compile_with_api_mount_root`, `trusted_element_parts`,
  and `TrustedElementParts` were promoted from `pub(crate)` to documented `pub` (their
  callers are now cross-crate).
- **Wire extraction:** the five wire header/query constants plus
  `HeadElement`/`ViewPayload`/`SsrPayload` moved out of `response_finalizer` into
  `vorma_contract::wire`; `response_finalizer` re-exports them for its own runtime use.
  The TsGen derive emits `::vorma::tsgen::*` paths (the macro ABI), which `vorma-contract`
  satisfies locally via `extern crate self as vorma` — the same self-alias trick the
  runtime crate already used. The wire-fixture golden test passes unchanged, proving
  emissions are byte-identical.
- **tsgen relocated out of root exports:** the flat root names (`TsError`, `TypeRef`,
  `TypeDef`, `FieldDef`, `TypePhase`, `TypeRegistry`, `TsDrafter`, `TsExtraType`,
  `RawTsPart`, `Type`, `TsResult`) are gone; the public surface is the `vorma::tsgen`
  module (re-export of `vorma_contract::tsgen`), and the derive macro now emits module
  paths. `__private::tsgen` is deleted.
- **`vorma::__private` shrunk:** `graph`, `manifest`, `constants`, `forms`, `wire`,
  `tsgen`, and the legacy `document` submodule are all deleted; `__private::contracts` no
  longer globs the moved contract types and now exposes only the hidden runtime
  document-builder surface. `vorma-build` imports
  `vorma_contract::{framework_graph, runtime_manifest, contracts, constants, wire}`
  directly. What remains in `__private` is runtime-internal by nature — `assets`,
  `config`, `env`, `facade`, `route_input`, `runtime`, the document-builder contracts,
  `AppBuildContract` + `app_build_contract`/`app_build_graph`, and the erased static-route
  machinery — and is Phase D's layering subject.
- **Pins updated:** `public_api.rs` now exercises
  `vorma::tsgen::{TsDrafter, Result, TsExtraType}` and
  `__private::contracts::DocumentBuildIdentity`.

Verification at close: workspace clippy `-D warnings` clean, 467 tests passing across all
crates (including wire goldens and the example app), rustdoc clean, fmt clean.

## Phase E — Client-core decomposition (tranche 1 landed; runs before C/D)

Goal: the browser runtime's implicit internal contracts become owned modules with explicit
interfaces and unit tests — the TS-side analog of the `__private` cure. A file-split that
keeps the shared-mutable-closure shape does not count.

Tranche 1 (landed): four units extracted from `create_client_core.ts` (2,764 → 2,461
lines), each with its own unit suite (+33 tests; suite now 835 across 33 files):

- `revalidation_scheduler.ts` — owns the `refresh` base fact as an explicit state machine
  (idle/debouncing/pending/retrying). Decides WHEN a revalidation runs and with what
  demand facts; the core launches fetches via the `start_revalidation(run)` dep. Owns the
  demand/backoff constants and the `RefreshState`/`RefreshDemand`/`RefreshWaiter` types
  (moved out of `client_core_types.ts`); the core re-exports the constants so existing
  test imports stay valid. Unit-pinned: waiter carry-over across collapsed debounces, the
  AND-merge of `skip_work_indicator`, seq-gated `mark_fresh`, exponential backoff through
  exhaustion, and the skipped-retry-re-enters- backoff behavior.
- `work_projection.ts` — pure `WorkSources → WorkState/WorkProjection` derivations. The
  core assembles one explicit `collect_work_sources()` value instead of the projections
  reading router internals ad hoc.
- `redirects.ts` — the redirect classification ladder (`classify_redirect_target`,
  parameterized by current origin) plus `detect_redirect` and `is_http`, shared by nav and
  submit paths.
- `wire_payload.ts` — `decode_payload` as a pure wire-boundary function; the search-schema
  registry is a callback owned by the core.

Tranche 2 (landed): `client_loaders.ts` — one owned unit for everything client loaders
need across navigations: the loader registry, the search-schema registry, the WASM matcher
with its pre-ready pattern queue (registration and pattern-queueing fused into one
`register`), prefetch prestart (with the matcher-ready deferral and the lazily-computed
history state), the shared reconcile ownership algorithm, server-state building, and `run`
with its cascade-abort. Consumed by the nav fetch path, the prefetch path, HMR, and boot
through one explicit interface. The shared abort/error helpers moved to `abort_error.ts`
(now multi-module). Core is down to 2,150 lines; suite is 840 tests across 34 files
(reconcile suppression/retention/orphan-abort and server-state shape are unit-pinned).

Tranche 3 (landed): `route_modules.ts` — owns view-module materialization: dynamic imports
with the dev module cache, HMR version stamping + `hmr_update`, the loader-rerun-on-HMR
policy, and `build_route_record` (payload + modules → immutable RouteRecord, with
server-error-over-loader- error precedence unit-pinned). Loader discovery flows out
through a `register_loader` dep. Core is down to 2,031 lines; suite 844 tests / 35 files.

Tranche 4 (landed): `submissions.ts` — owns concurrent API submissions: the dedupe-keyed
in-flight map, dispatch with JSON body normalization, redirect/build-skew handling on
responses, and per-submission revalidation scheduling. Route state never enters the unit;
soft redirects flow out through a `redirect_to` dep (the core decides
navigate-vs-defer-until-boot) and skew reports through `report_resource_build_skew`. The
subsystem is pinned by the 1,613-line `router_submit` suite through the public API;
`make_deferred` moved to `deferred.ts` (now multi-module). Core is down to 1,765 lines
(from 2,764 at Phase E start).

Tranche 5 (landed) — PHASE E COMPLETE: `history_position.ts` owns the `browser` base fact
and every window.history write that moves it: key-stamped push/replace commits, popstate
adoption, the boot-time `ensure_window_key` guarantee (preserving foreign state objects),
and key-scoped scroll saving. Unit-pinned: push/replace stamping, foreign-state
preservation, key idempotence, and user-state round-trips.

Phase E end state: `create_client_core.ts` is 1,703 lines (from 2,764), and every base
fact except the transaction set is owned by a tested unit — scheduler (`refresh`),
submissions, history position, loaders/schemas/ matcher, module caches/HMR — plus pure
work projection, redirect classification, and wire decode. What remains in the core is the
fetch transaction (`start_fetch`/`run_active`/`publish`), navigation entry, popstate,
boot, and the public API: the composition root. Suite: 849 tests across 36 files (was
802/29 at phase start); tsgo + oxlint clean.

## Phase C — Vite plugin contract onto the golden-pin generator — COMPLETE

The Vite plugin contract is now rendered from Rust and golden-pinned, mirroring the
wire-contract pattern:

- **Generated:** `packages/vorma/vite/plugin_contract.gen.ts` is rendered by
  `crates/vorma-build/src/vite_plugin_ts_contracts.rs` — the eight transport constants,
  `VitePluginConfig` (via the TsGen type model), and the `RpcRequest` union rendered from
  real serde serialization of every variant (an exhaustive match makes adding a variant a
  compile error until its sample exists). Golden-pinned with the same
  `VORMA_UPDATE_WIRE_CONTRACTS=1` regeneration flow.
- **snake_case:** the five `PascalCase` serde renames on `VitePluginConfig` are gone; the
  JSON wire fields are the Rust field names. Rust RPC pins and the TS plugin/tests updated
  (breaking change per ratified policy).
- **Hand-mirror sweep:** `plugin_contract.ts` shrank to the three TypeScript-owned facts
  (`plugin_name`, `public_url_parse_base`, `pub_url_fn_name`). The dev-refresh
  `ChangeType` strings consumed by the embedded `refresh_script.js` are pinned by a drift
  test in `dev_refresh.rs` (the script is hand-authored JS, so it gets a pin rather than
  codegen). The `"@public/"` duplication was already eliminated in the Phase B opener
  (`static_outputs` imports `VITE_PLUGIN_PUBLIC_URL_PREFIX`).

Verification at close: workspace 470 Rust tests, TS suite 849 across 36 files, clippy
`-D warnings` clean, fmt clean, tsgo + oxlint clean.

## Phase D — `vorma` internal layering

- [x] Dedupe the `HEAD_TAG_*`/`HEAD_ATTR_*` constants defined in three files — the union
      vocabulary now lives once in `head.rs` as `pub(crate)` consts; all three consumers
      import it (the `HEAD_ATTR_ANY_VALUE` sentinel stays in `view_response.rs`, which
      owns that matching semantic)
- [x] Dedupe route-contract assembly — `route_input.rs` (home of the resolvers) now owns
      the single `RouteContractFacts` + `route_contract_from_resolvers` assembly;
      `facade.rs` keeps only a thin generic adapter mapping into `FacadeError`, and
      `public_app.rs`'s copy is gone
- [x] Unify the handler-context layers around one effects/head API — the semantics now
      exist exactly once: `head.rs` owns the element constructors (`title_element`,
      `description_element`, `icon_element`, `preload_element`,
      `meta_{name,property}_content_element`, `meta_charset_element`), the second-tier
      vocabulary (`HEAD_REL_*`, `META_DESCRIPTION_NAME` — previously tripled), and the
      shared `prepare_head_element` step (previously duplicated with divergent error
      types); `ResponseEffects::apply_head_element` owns the tag routing (meta/title/rest
      — previously written twice, plus per-method in `HandlerContext`). Both handle pairs
      (`handler_context`'s `&mut` handles and `typed_handler`'s lock-based handles, which
      also serve `static_route`'s ctx types) are now thin adapters over the shared
      semantics; their genuinely different access models stay. Public API unchanged. The
      known parity gap stays as-is (typed head has `append(HeadBuilder)`; the contextual
      handle does not) — adding API was out of scope.
- [x] Split `framework_graph.rs` (now in `vorma-contract`): the file keeps the data model,
      `FrameworkGraph`, and `GraphError` (1,411 lines incl. tests); shape/contract/asset
      validation moved to `graph_validation.rs` (412) and route-pattern semantics +
      matcher integration to `graph_patterns.rs` (98), both crate-private modules with
      `pub(crate)` surfaces
- [x] `runtime_*` layering — investigated; structure is already a clean declare → contract
      → serve chain with strictly inward dependencies (`runtime_snapshot` ← `runtime_app`
      ← `runtime_service` ← `runtime_host`, with `runtime_assets` as the host's filesystem
      boundary and `runtime_document` the provider seam; `runtime_document` is adjudicated
      kept — seven consumers post-rewrite). The soup was narrative, not architecture:
      every `runtime_*` module doc now states its layer position and neighbors. **Open
      (needs ratification):** optional renames if the prefix itself should go —
      candidates: `runtime_snapshot` → `snapshot`, `runtime_app` → `request_boundary`,
      `runtime_service` → `serve_stack`, `runtime_host` stays (public name). Renames are
      visible naming decisions; not executed unilaterally.

## Phase E — `create_client_core` decomposition

The entire browser runtime is one function: matcher registration, URL/history, payload
decoding, client-loader prefetch, fetch lifecycle, redirect policy, revalidation
scheduling, HMR, and scroll restoration as nested closures over ~15 mutable variables. The
internal `///////` section comments name the modules that should exist. The rest of
`core/` (url, head, make_link_props, work_indicator) is the proof of the target style.
RESOLVED LATER: the source decomposition landed in Phase E, and the 4,932-line companion
suite was split into 7 themed integration files
(core_boot/navigation/client_loaders/revalidation_and_work/prefetch_and_css/hmr/
scroll_and_history) during the CURRENT_PLAN_2 finale — the split exposed and fixed one
latent order-dependent prestart test.

- [ ] Lift closure state into an explicit, typed `ClientCoreState`
- [ ] Extract pure modules (history commit, payload decode, redirect classification,
      revalidation backoff, loader-prefetch reconciliation, fetch lifecycle) taking
      state/deps as parameters
- [ ] Reduce `create_client_core` to thin assembly
- [ ] Decompose the test monolith along the same seams
- [ ] Resolve the `Symbol.for("vorma-data-revalidate-fn")` global escape hatch and the
      mixed `camelCase`/`snake_case` keys on the returned client object

## Phase F — Enforcement and docs

- [ ] trybuild compile-fail tests for `vorma-macros` (error spans/messages are public API;
      currently zero tests)
- [~] CI workflow: one job running `make gate` — **deferred by ruling** (tooling overhaul
  planned later; revisit then)
- [ ] Architecture prose that currently exists nowhere: the two-process model, build
      generations/epochs, the live-state protocol, the contract boundaries, and the watch
      → rebuild → refresh pipeline

## Cleared during review/audit — no action

- `sha2` + `blake3` coexistence is justified: `critical_css_content_sha256` serves CSP,
  which mandates SHA-2; blake3 is internal content hashing.
- `go.mod`/`go.sum` were already deleted on this branch.
- `.dist/` and `*.gen.ts` files are not git-tracked (initially suspected otherwise).
- Committed-wasm staleness self-heals: the gate runs in write mode and
  `rust-build-client-wasm` regenerates the binary inside both gate steps.
- The wire-fixtures golden vs formatter fight is resolved: generated JSON is tab-indented
  at the source, and the generated fixtures file is oxfmt-ignored (single writer).

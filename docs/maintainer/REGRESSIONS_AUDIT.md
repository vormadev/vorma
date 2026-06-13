# Rewrite Regressions Audit

Live tracker for restoring behavior silently lost in the full-framework rewrite landed on
the `rust-init` branch.

- Baseline (pre-rewrite): commit `4b793aa0` ("audit wip").
- Subject: the working tree / rewrite as it exists on this branch.
- Scope: the entire diff — `crates/vorma` (full rewrite), `crates/vorma-build` (full
  rewrite), `crates/vorma-client-wasm`, `crates/vorma-macros`, `packages/vorma`,
  `tests/framework`, `examples/minimal`, maintainer docs, and all dependency changes (217
  files, ~42.8k insertions / ~34.6k deletions).

Classification used below, as requested by the maintainer: **likely intentional** (the new
architecture demonstrably owns the concern another way) vs **likely accidental** (the
concern has no surviving owner). Findings are an exhaustive plain list; nothing has been
triaged away.

## Method

1. Dependency census: `git diff 4b793aa0 -- Cargo.toml 'crates/*/Cargo.toml'`; every
   removed dependency traced to its old call sites, and each call site's responsibility
   located (or not) in the new tree.
2. Test census: every `#[test]`/`#[tokio::test]` function name at the baseline vs the
   working tree, per crate; same for TS `it(...)`/`test(...)` names. Regenerate with:

    ```
    git ls-tree -r 4b793aa0 --name-only -- crates | grep '\.rs$' | while read f; do
      git show "4b793aa0:$f" | grep -A3 -E '^\s*#\[(tokio::)?test\]' \
        | grep -oE 'fn [a-zA-Z0-9_]+' | sed "s|fn |$f\t|"; done | sort -u
    ```

    (Equivalent command over the working tree, then `comm` the name columns.)

3. Capability sweep: every deleted module mapped to its new owner by reading module docs
   at the baseline and grepping the new tree for the concept (not the name).

## Restoration Status

| Capability                                                    | Verdict                                           | Status                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| ------------------------------------------------------------- | ------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Output-layout file lock (`paranoid::local_lock::ProcessLock`) | accidental drop                                   | **RESTORED** as `crates/vorma-build/src/output_lock.rs`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `dist/.vorma/.gitignore` self-ignore write                    | accidental drop                                   | **RESTORED** in `build_output.rs` (`write_generation_candidate_outputs`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| Symlink defense for output _directory components_             | accidental narrowing                              | **RESTORED** — `create_dir_all_no_symlinks` ported into `build_output.rs`, used by `write_file_atomically`, public-static publish, and the lock dir                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| Live-state env key in child-process env clearing              | accidental narrowing                              | **RESTORED** — `LIVE_BUILD_STATE_ENV_KEY` added back to `std_command`'s clear set with a pinning test                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| Windows new-process-group creation flag                       | accidental drop                                   | **RESTORED** — `cfg(windows)` branch mirrors the baseline `CREATE_NEW_PROCESS_GROUP` flag exactly; note this branch is not compiled on the unix-only local gate, so it carries baseline-parity confidence, not local test confidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| Combined-cargo parallel bin builds (REMINDERS contract)       | accidental drop — maintainer-confirmed regression | **RESTORED** — `app_binaries_build.rs`: one `cargo build --bin <entry> --bin <server> --message-format=json-render-diagnostics` per server-recompile rebuild, executables extracted from cargo's JSON messages and run directly (no second `cargo run` compile); includes identical-target rejection and cross-package bin-name disambiguation. Streaming refinement also landed: `run_streaming_stdout_lines_until_cancelled` (defaulted trait method; true incremental override on the std runner) feeds cargo artifact messages to the combined build as they arrive, and the live-state read starts on a scoped worker the moment the build-entry artifact resolves — baseline `dev_artifact_collector_reports_build_entry_before_app_server_finishes` parity, pinned by a deterministic channel-handshake overlap test. |
| Watch patterns explicitly escaping the root dir               | accidental drop — maintainer-confirmed regression | **RESTORED** — root is a resolution anchor, not a boundary; explicit `../` watch paths work again                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| Production asset byte caching                                 | accidental drop — maintainer-confirmed regression | **RESTORED** — non-dev asset bodies cached in memory after first read                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| Serve-time symlink-escape rejection for public assets         | verified carried                                  | already present — `load_public_asset_body` canonicalizes (resolving symlinks) and rejects paths escaping the canonical root; now pinned by `public_asset_directory_rejects_symlink_escape`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| Dev refresh-script CSP hash gating outside dev                | accidental drop — small regression                | **RESTORED** — hash returns empty outside dev so production CSP never whitelists the dev-only inline script; pinned by test                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| Vercel current-dir manifest fallback                          | accidental drop — maintainer-confirmed regression | **RESTORED** — falls back to runtime-relative manifest when the configured path is missing; errors report both paths                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| Prehashed-directory public input feature                      | **intentional removal (maintainer-confirmed)**    | no action — feature deliberately dropped in the rewrite                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| Middleware filter predicates → route-shape scope patterns     | **design decision (maintainer-ratified)**         | no action — scopes are the model; in-body early return is the conditional idiom; revisit a pre-spawn predicate only with concrete evidence of waste                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| Dropped Rust test behaviors (census below)                    | adjudicated                                       | see Cluster Adjudication + Port Queue                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |

## Confirmed Dropped Capabilities

1. **Workspace output-layout lock.** At baseline, `pipeline.rs` acquired a `ProcessLock`
   at `dist/.vorma/dev.lock` inside `prepare_generation_candidate` — on the path of
   **both** `BuildCommand::Build` and dev — held it in the session runtime, and
   `dev_lifecycle.rs` handed the lock off (acquire-new-before-release-old) when `dist_dir`
   changed mid-session. The live-state child process never acquired it (early exit before
   any session), which is what prevented the dev server from deadlocking against its own
   child. The rewrite dropped all of it; the orphaned `paranoid` workspace dependency was
   the only trace. **Restored** in `crates/vorma-build/src/output_lock.rs`
   (`OutputLayoutLock`, lock file `dist/.vorma/build.lock` — renamed because it was never
   dev-only):
    - acquired in `start_and_activate_dev_generation` and
      `build_and_activate_production_generation` after build inputs are prepared and
      before the first disk write (`write_typescript_contracts`);
    - held by `StartedDevGeneration` (declared as the last field so the layout stays
      locked until all child processes are dropped) and for the production build's
      duration with explicit release;
    - handoff preserved: both live-state update builders call
      `reacquire_if_output_layout_moved` after the new plan is known and before any
      writes;
    - live-state mode remains lock-free by construction (`entrypoint.rs` early-exits
      before either acquisition path). Covered by four unit tests in the module; full
      `vorma-build` suite passes.

2. **`dist/.vorma/.gitignore` write.** Baseline `build_layout.rs` exposed
   `gitignore_out()` (`dist/.vorma/.gitignore`, content `*\n`) and the layout preparation
   wrote it so generated outputs self-ignore in user repos. The new tree contained zero
   references to writing any `.gitignore`. **Restored** in `build_output.rs`:
   `write_generation_candidate_outputs` (the choke point both dev and production publishes
   flow through) writes `dist/.vorma/.gitignore` atomically alongside the manifest; public
   outputs live under `dist/.vorma/static/public`, so the single self-ignore covers every
   generated file. Asserted by the existing writer test.

3. **Output-directory symlink defense narrowed.** Baseline `utils.rs`
   `create_dir_all_no_symlinks` refused to create output paths through symlinked directory
   components (tested, including a symlink-component escape case). The new tree kept
   symlink rejection for public-static _source_ entries and for the _output file_ path
   before overwrite, but created output directories with plain `fs::create_dir_all`.
   **Restored**: the helper is ported into `build_output.rs` and used by
   `write_file_atomically` parent creation, the public-static publish directory, and the
   output-lock directory; baseline tests ported (nested creation/reuse, file-at- path
   rejection, symlink-component rejection).

4. **Live-state key missing from child env clearing.** Baseline `clear_vorma_runtime_env`
   removed `IS_BUILD`, `IS_DEV`, the live-state mode key, and both Vite plugin keys from
   every spawned child. The new `std_command` clear set omitted the live-state key, so an
   externally exported live-state variable could leak into children. **Restored** with a
   test pinning the cleared set and verifying explicit `with_env` values still apply after
   clearing.

5. **Windows new-process-group creation flag dropped.** Baseline `supervisor.rs`
   `prepare_child_process` set `CREATE_NEW_PROCESS_GROUP` (0x200) on Windows; the new
   `configure_process_group` only handles unix and is a no-op elsewhere, so on Windows
   child-tree termination loses group semantics. Adjudicate: if Windows dev-server support
   is out of scope for the rewrite, record that decision here; otherwise port the flag.

## Removed Dependencies — Adjudicated

| Dependency                  | Old call sites                                                        | Verdict                                                                                                                                                                                                                                                                        |
| --------------------------- | --------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `paranoid` (`local-lock`)   | `pipeline.rs`, `dev_lifecycle.rs`, `runtime.rs`                       | accidental drop — **restored** (see above).                                                                                                                                                                                                                                    |
| `lru`                       | `staticproc.rs` global 10k-entry hash cache                           | intentional — replaced by a better `(mod_time, size)`-revalidated cache with a 65,536-entry backstop in `static_outputs.rs`. Cleared.                                                                                                                                          |
| `html-escape`               | `htmlutil.rs`                                                         | intentional — replaced by hand-rolled, tested escapers in `document_renderer.rs` (`escape_attribute_value`, `escape_text`, `escape_style_raw_text`). Open verification: parity sweep against the old `htmlutil.rs` test matrix (11 dropped test names).                        |
| `serde_urlencoded`          | `api.rs`                                                              | intentional — replaced by `url::form_urlencoded` in `request.rs` and `input_decoder.rs`, with decode tests present. Cleared.                                                                                                                                                   |
| `path-clean`                | `config.rs`, `globset.rs`, `public_static_inputs.rs`, `ts_modules.rs` | mostly intentional — new tree canonicalizes roots and uses `path-slash`/`pathdiff`; runtime asset serving has explicit traversal rejection tests (`asset_body_provider.rs`). Open verification: glob/path normalization parity for watch patterns on Windows-style separators. |
| `same-file`                 | `staticproc.rs` (now dev-deps only)                                   | intentional — the temp-file + atomic-rename copy path makes the same-file truncation hazard structurally impossible. Cleared.                                                                                                                                                  |
| `libc` (from `vorma-build`) | old `signals.rs` support                                              | intentional — signal handling now via `tokio::signal` (+ `nix` for tests); old and new both hook ctrl-c + SIGTERM with the same non-unix fallback. Cleared.                                                                                                                    |

## Capability Map — `crates/vorma-build` (deleted module → new owner)

All verdicts below are "carried" unless annotated.

| Baseline module                                                 | New owner                                                                                                                                 |
| --------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `activation.rs`                                                 | `generation_epoch.rs` commit/activate + `dev_build.rs` activations                                                                        |
| `browser_sync.rs`                                               | `dev_refresh.rs` + `dev_mux.rs` — open verification: refresh message/`ChangeType` protocol parity with the baseline browser-sync messages |
| `build_cancel.rs`                                               | `process_runner.rs` `BuildProcessCancel`                                                                                                  |
| `build_layout.rs`                                               | `build_output.rs` + `vorma::__private::assets` path fns — **except** lock (restored) and `.gitignore` (dropped, finding 2)                |
| `cargo_target.rs`                                               | `entrypoint.rs` `BuildOptions`                                                                                                            |
| `command_runner.rs` / `process_wait.rs`                         | `process_runner.rs` / `process_ready.rs`                                                                                                  |
| `config.rs`                                                     | `build_plan.rs` + `vorma::__private::config` — 24 dropped test names to adjudicate                                                        |
| `constants.rs`                                                  | `tokens.rs`, `vite_plugin_contract.rs`, `vorma` constants                                                                                 |
| `cssbundle.rs`                                                  | `static_outputs.rs` `bundle_critical_css` (lightningcss retained)                                                                         |
| `dev_lifecycle.rs` / `dev_loop.rs`                              | `entrypoint.rs` dev loop — **except** lock handoff (restored)                                                                             |
| `document_hash.rs`                                              | live-state `root_document_hash_source` + generation effects — open verification: document-change → build-id/refresh parity                |
| `frontend_toolchain.rs`                                         | `vite_command.rs` + `command_base_parts` (package manager stays config-driven)                                                            |
| `fswatcher.rs` / `watch_config.rs` / `watch_plan.rs`            | `dev_watcher.rs` + `build_plan.rs` dev-watch intents                                                                                      |
| `generation.rs` / `generation_workspace.rs`                     | `generation_epoch.rs` + `generation_inputs.rs`                                                                                            |
| `globset.rs`                                                    | direct `globset` use in `build_plan.rs`/`dev_watcher.rs` — see `path-clean` row above                                                     |
| `live_refresh.rs`                                               | `dev_refresh.rs`                                                                                                                          |
| `local_cargo.rs`                                                | `live_state_command.rs` + `app_server_process.rs` — 11 dropped test names; open verification: cargo invocation env/target-dir parity      |
| `manifest.rs` + `manifest/{public_output,vite_projection}.rs`   | `vite_manifest.rs` + `build_output.rs` + `vorma` `runtime_manifest.rs` — 15 dropped test names                                            |
| `pipeline.rs`                                                   | `production_build.rs` + `generation_inputs.rs` — **except** lock (restored)                                                               |
| `public_static_inputs.rs` / `staticproc.rs` / `static_build.rs` | `static_outputs.rs` + `production_generation.rs`                                                                                          |
| `runtime.rs` / `session.rs` / `supervisor.rs`                   | `EpochSupervisor` + `StartedDevGeneration` — **except** lock holder (restored)                                                            |
| `signals.rs`                                                    | `dev_signal.rs`                                                                                                                           |
| `ts_gen.rs` / `ts_modules.rs`                                   | `typescript_contracts.rs` — 10 dropped test names                                                                                         |
| `utils.rs`                                                      | scattered — **except** `create_dir_all_no_symlinks` (finding 3)                                                                           |
| `vite_plugin.rs` / `viteutil.rs`                                | `vite_plugin_contract.rs` + `vite_plugin_control.rs` + `vite_plugin_rpc.rs` + `vite_command.rs`                                           |
| `work_queue.rs`                                                 | `entrypoint.rs` coalescing/queue logic                                                                                                    |

## Capability Map — `crates/vorma` (deleted module → new owner)

| Baseline module                                                | New owner                                                                                                                                 |
| -------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `api.rs`                                                       | `public_app.rs`, `facade.rs`, `typed_handler.rs`, `resource_response.rs`                                                                  |
| `core/{context,contract,pattern,runner,runtime}.rs`            | `execution_engine.rs`, `execution_plan.rs`, `framework_graph.rs`, `handler_context.rs`                                                    |
| `document.rs`                                                  | `document_builder.rs` + `document_renderer.rs`                                                                                            |
| `htmlutil.rs`                                                  | `document_renderer.rs` escapers — parity sweep open (see deps table)                                                                      |
| `handler.rs` / `static.rs`                                     | `typed_handler.rs` / `static_route.rs`                                                                                                    |
| `init/{public_asset_service,resource_service,view_service}.rs` | `runtime_app.rs`, `runtime_service.rs`, `asset_body_provider.rs` (traversal-rejection tests present)                                      |
| `manifest.rs`                                                  | `runtime_manifest.rs`                                                                                                                     |
| `mux/*` incl. `ordered_parallel.rs`                            | `execution_engine.rs` — parallel-middleware contract verified carried (`middleware_invocations_execute_in_parallel_before_handler_phase`) |
| `response.rs` / `view_payload.rs`                              | `response_finalizer.rs` + `payload_projection.rs`                                                                                         |
| `tsgen/{identifier,render,resolve}.rs`                         | `tsgen.rs` (consolidated)                                                                                                                 |
| `searchparams.rs`                                              | `search_params.rs` (rename)                                                                                                               |

## Test Census

Rust, baseline → working tree (test function count):

| Crate                      | Baseline | Now | Net |
| -------------------------- | -------- | --- | --- |
| `crates/vorma`             | 266      | 236 | −30 |
| `crates/vorma-build`       | 203      | 109 | −94 |
| `crates/vorma-client-wasm` | 7        | 8   | +1  |
| `crates/vorma-matcher`     | 28       | 28  | 0   |
| `crates/vorma-tasks`       | 33       | 33  | 0   |

Name-level churn across crates: 443 baseline test names absent now; 316 new names added.
Counts alone overstate the loss (many are renames/merges), which is why each cluster below
needs behavioral adjudication: confirm an equivalent assertion exists in the new suites
(in-module tests, `tests/in_memory_test_app.rs`, `tests/app_declaration_contract.rs`,
golden/fixture tests, or bombadil e2e), or port the test.

TypeScript: 650 → 672 test names; only 4 baseline names absent, all in files that were
modified (not deleted) and likely renames — adjudicate the same way:

- `cancels pending calls` (kit/debounce)
- `only the last call within the delay window executes` (kit/debounce)
- `handles null boolean_attributes field gracefully` (core/head)
- `starts route fetches before matcher wasm is ready but waits to prestart client loaders`
  (core/wasm_readiness)

Dropped-name clusters by baseline file (≥3 names; regenerate the full per-name lists with
the Method §2 commands):

| Baseline file                                                  | Dropped names |
| -------------------------------------------------------------- | ------------- |
| `crates/vorma/src/init_tests/public_assets_and_resources.rs`   | 24            |
| `crates/vorma-build/src/config.rs`                             | 24            |
| `crates/vorma/src/mux_tests/middleware.rs`                     | 20            |
| `crates/vorma/src/init_tests/middleware_and_effects.rs`        | 19            |
| `crates/vorma/src/api.rs`                                      | 18            |
| `crates/vorma-build/src/staticproc.rs`                         | 18            |
| `crates/vorma/src/static.rs`                                   | 16            |
| `crates/vorma/src/handler_tests/view_response.rs`              | 16            |
| `crates/vorma-build/src/manifest.rs`                           | 15            |
| `crates/vorma-build/src/live_state.rs`                         | 13            |
| `crates/vorma/src/init_tests/service_body_and_context.rs`      | 12            |
| `crates/vorma/src/response.rs`                                 | 11            |
| `crates/vorma/src/init_tests/document_and_view_shell.rs`       | 11            |
| `crates/vorma/src/htmlutil.rs`                                 | 11            |
| `crates/vorma/src/handler_tests/api_response.rs`               | 11            |
| `crates/vorma-build/src/vite_plugin.rs`                        | 11            |
| `crates/vorma-build/src/local_cargo.rs`                        | 11            |
| `crates/vorma-build/src/ts_gen.rs`                             | 10            |
| `crates/vorma-build/src/supervisor.rs`                         | 10            |
| `crates/vorma/src/mux_tests/nested_views.rs`                   | 9             |
| `crates/vorma/src/core_tests.rs`                               | 9             |
| `crates/vorma-build/src/fswatcher.rs`                          | 9             |
| `crates/vorma/src/init_tests/method_and_request_boundaries.rs` | 8             |
| `crates/vorma-build/src/globset.rs`                            | 8             |
| `crates/vorma/src/handler_tests/dev_assets.rs`                 | 7             |
| `crates/vorma/src/head.rs`                                     | 6             |
| `crates/vorma-build/src/dev_loop.rs`                           | 6             |
| `crates/vorma-build/src/cssbundle.rs`                          | 6             |
| `crates/vorma-build/src/activation.rs`                         | 6             |
| `crates/vorma/src/manifest.rs`                                 | 5             |
| `crates/vorma/src/lib.rs`                                      | 5             |
| `crates/vorma/src/document.rs`                                 | 5             |
| `crates/vorma/src/tsgen/resolve.rs`                            | 4             |
| `crates/vorma-build/src/static_build.rs`                       | 4             |
| `crates/vorma-build/src/document_hash.rs`                      | 4             |
| `crates/vorma-build/src/dev_lifecycle.rs`                      | 4             |
| `crates/vorma-build/src/build_layout.rs`                       | 4             |
| `crates/vorma-build/src/browser_sync.rs`                       | 4             |
| `crates/vorma/src/tsgen.rs`                                    | 3             |
| `crates/vorma/src/config.rs`                                   | 3             |

Counterweight: e2e coverage grew in the rewrite (`tests/framework/dev_hmr_check.mjs` +409
lines, `bombadil.rs` reworked, new golden/fixture suites: `wire_contract_fixtures`,
`search_param_contract_vectors`, `matcher_decode`). E2e growth mitigates but does not
discharge unit-level adjudication.

## Cluster Adjudication (complete)

Every dropped-name cluster was adjudicated by mapping old test names (long descriptive
names per house rule) onto the full new-suite inventory. Verdict classes: **mapped**
(equivalent assertion exists in a named new test), **obsolete** (the mechanism the test
pinned no longer exists by design), **port** (real coverage gap — see Port Queue),
**feature?** (the _feature_ itself appears dropped — maintainer decision needed).

Fully mapped (no action): `vorma` — `init_tests/document_and_view_shell` (→
`runtime_app`/`document_builder` dynamic-provider suite),
`init_tests/service_body_and_ context` (→ `runtime_service` + `typed_handler`),
`init_tests/public_assets_and_ resources` (→ `asset_capabilities` +
`asset_body_provider` + `runtime_assets` + `runtime_app` + `in_memory_test_app`;
init-rejection granularity reduced 7→2 but the concern is covered),
`handler_tests/api_response` (→ `resource_response` + `runtime_app` resource suite),
`document.rs`, `head.rs`, `htmlutil.rs`, `manifest.rs`, `response.rs` (→
`response_finalizer`, one nuance in Port Queue), `core_tests.rs`, `tsgen*`, `config.rs`,
`envutil.rs` (API removed), `lib.rs`, `searchparams.rs`. `vorma-build` — `activation.rs`
(→ `production_build`/`production_vite`; prod-cancel tests obsolete: production builds are
one-shot now), `build_layout.rs` (tmp-manifest promotion obsolete: `production_vite` reads
the manifest directly), `command_runner.rs` (→ restored env test), `cssbundle.rs` (→
`static_outputs` critical-CSS, two gaps in Port Queue), `dev_lifecycle.rs` (→ restored
lock tests + `supervisor_failed_candidate_ does_not_replace_committed_generation`),
`dev_loop.rs` (→ `refresh_payloads_derive_ from_committed_effects` + entrypoint inflight
suite), `dev_mux.rs`, `dev_watcher.rs`, `fswatcher.rs` (dir-minimization tests obsolete if
the new watcher design watches roots and filters — see Behavior Questions),
`generation*.rs` (→ `generation_epoch` + restored gitignore test), `live_refresh.rs`,
`local_cargo.rs` (cargo-metadata artifact extraction obsolete: new arch uses `cargo run`
per target — see Behavior Questions for the parallelism consequence), `manifest.rs` +
`manifest/*` (→ `vite_manifest` + `projection_compiler` rejection suite),
`process_wait.rs`, `runtime.rs`, `session.rs`, `signals.rs`, `static_build.rs`,
`staticproc.rs` (copy/same-file/symlink-output semantics obsoleted by temp+rename atomic
design; hash cache mapped; one edge gap in Port Queue), `supervisor.rs` (→
`process_ready` + command-projection tests; start-rejects-active obsolete under ownership
model), `ts_gen.rs`/`ts_modules.rs` (→ `typescript_contracts` + `runtime_snapshot`
view-module rejections), `utils.rs` (→ restored symlink helper + `tokens`),
`vite_plugin.rs` (→ `vite_plugin_rpc` suite, one verify in Port Queue), `viteutil.rs`,
`watch_plan.rs`, `work_queue.rs`, `globset.rs` (custom rule engine deleted; semantics now
plain `globset` via `build_plan` — one behavior change in Behavior Questions).

TS: all 4 dropped names adjudicated — debounce ×2 are renames
(`rejects cancelled pending calls`,
`settles every coalesced call with the last call result` — stronger than baseline), head
null-vs-absent `boolean_attributes` is now governed by the generated wire contract +
fixtures; `wasm_readiness` "starts route fetches before matcher wasm is ready but waits to
prestart client loaders" has no obvious successor — listed in Port Queue.

## Port Queue (real gaps — port or consciously decline)

1. RESOLVED — all five behaviors verified carried in code: the JSON decode path has no
   content-type gate at all (the leniency trio holds by construction), unit-input empty
   bodies are explicitly accepted (`input_decoder.rs` Unit + empty-body branch), and
   multipart parts without names are rejected via dedicated
   `MultipartFieldMissingName`/`MultipartFieldEmptyName` variants (stronger than
   baseline). Optional follow-up: add explicit pin tests for the leniency and empty-body
   cases.
2. RESOLVED — six pins landed in `execution_engine.rs` and the engine verified to uphold
   every baseline concurrency contract via its in-order commit discipline (out-of-order
   completions buffer until priors commit):
   `later_middleware_redirect_waits_for_prior_sibling_success_commit`,
   `middleware_error_short_circuits_without_waiting_for_later_pending_sibling`,
   `prior_middleware_redirect_completes_without_waiting_for_pending_sibling` (the later
   sibling may not even start — stronger than baseline's cancel),
   `deepest_view_success_status_wins_over_parent_status`,
   `child_view_redirect_waits_for_parent_success_commit`,
   `middleware_head_elements_merge_before_handler_head_elements`. Adjudication notes: a
   non-2xx status effect is terminal (so "deepest status wins" applies to success
   statuses, matching baseline's last-success-wins rule), and a later error waiting on an
   earlier pending sibling is the deterministic-merge contract working as baseline
   intended (baseline's cancel targeted _later_ siblings only).
3. RESOLVED — pinned at the merge level by
   `merged_set_header_replaces_prior_values_before_later_adds_append` in
   `response_finalizer.rs` (middleware add → handler set replaces it; handler's later add
   appends), alongside the existing single-effects set-then-add pin.
4. RESOLVED — head elements live inside `ResponseEffects` and merge in commit order,
   pinned by `middleware_head_elements_merge_before_handler_head_elements`.
5. Build-target validation: reject identical build-entry and app-server cargo targets (old
   `cargo_build_args_reject_same_build_entry_and_app_server_target`) — nothing in
   `build_plan`/`entrypoint` validates this now.
6. RESOLVED — containment verified carried: `RootedCssFileProvider` canonicalizes and
   root-checks the entry before bundling and every import inside `SourceProvider::read`
   (one shared mechanism guards both). Pinned by
   `critical_css_bundle_rejects_imports_outside_root_dir`.
7. RESOLVED — semantics verified identical and ported as
   `hashed_output_names_strip_one_suffix_and_keep_dotfile_and_trailing_dot_exts` in
   `static_outputs.rs`.
8. RESOLVED — already implemented: `from_json_bytes` rejects blank hash sources with
   `MissingRootDocumentHashSource` (no pin test yet; optional follow-up).
9. RESOLVED — `vite_dev_origin` carries all three baseline behaviors (client-entry origin
   extraction, loopback fallback for relative entries, ipv6 brackets preserved via
   `Url::origin`); pinned by
   `vite_dev_origin_uses_client_entry_origin_with_loopback_fallback` and
   `refresh_script_inner_html_substitutes_all_dev_manifest_placeholders`.
10. RESOLVED — this was a real gap: the new `set_port` handler recorded any port
    unconditionally. **Restored**: reachability is probed on the loopback host before
    recording; unreachable ports return 500 and are not recorded. Pinned by
    `vite_plugin_rpc_records_reachable_control_port` and
    `vite_plugin_rpc_does_not_record_unreachable_control_port`.
11. RESOLVED — behavior verified carried (route fetch proceeds while loader prestart
    defers on `route_matcher_ready`); pinned by
    `waits for matcher wasm before prestarting known client loaders` in
    `wasm_readiness.test.ts`, which proves the prestart fires after wasm resolution and
    before the server response.

## Behavior Questions — RESOLVED (maintainer rulings recorded)

- **Prehashed-directory input feature**: intentionally dropped in the rewrite
  (maintainer-confirmed). No restoration; baseline tests obsolete by decision.
- **Combined cargo build**: confirmed regression of the REMINDERS "live-state build
  parallel with app-server build" contract. Restoration tracked in Restoration Status.
  Design note: the two bins must be built by ONE cargo invocation
  (`cargo build --bin <build-entry> --bin <server> --message-format=json`) with executable
  paths extracted from cargo's JSON messages — concurrent `cargo run` processes serialize
  on cargo's own target-dir lock, so they cannot provide the contracted parallelism.
- **Watch patterns escaping root**: confirmed regression. The root dir is a _resolution
  anchor_ for relative config paths, not a containment boundary; explicit `../` watch
  paths must work to support arbitrary monorepo layouts. Restored.
- **Middleware filter predicates → scope patterns**: ratified as a design decision. Scope
  patterns are the model (statically analyzable, per-route precomputation, least-work
  dispatch); in-body early return is the conditional idiom. A narrow pre-spawn predicate
  may be layered on top of scopes later, only with concrete evidence of measurable
  no-op-task waste. `middleware_filter_controls_execution` and
  `public_middlewares_wire_once_and_filter_from_request_context` are obsolete by decision.
- **Asset serving (byte cache + serve-time symlink stat)**: both confirmed regressions
  ("wastefulness is incorrectness"; defense-in-depth is nearly free). Restored.
- **Browser-sync slow-client handling**: verified — see Cleared.
- **Vercel manifest fallback**: confirmed regression. Restored.
- **Dev refresh-script CSP hash gating outside dev**: verified — see Cleared.

## Deleted Maintainer Docs

Deleted at baseline → branch: `auditor-friendliness-initiative.md`,
`frontend-audit-friendliness.md`, `security-audit.md`, `specs/BACKEND_RUNTIME.md`,
`specs/README.md`.

`specs/BACKEND_RUNTIME.md` swept: it was a **stub** — three filled prose sections (runtime
identity/ownership) followed by ~30 empty headings. Deletion judged intentional. The
filled prose invariants all have surviving coverage in the new runtime: request
classification and method/not-found boundaries
(`runtime_app_finalizes_method_not_allowed_and_not_found`), build-skew protection
(`runtime_app_build_skew_bypasses_view_execution`,
`runtime_app_build_skew_bypasses_dynamic_document_provider`,
`finalizer_build_skew_response_preserves_contract_headers`), redirect semantics (seven
`finalizer_*redirect*` tests), and request-body-limit defaults
(`app_assembly_defaults_static_document_and_request_body_limit`). The three other deleted
docs were initiative/working notes; no behavior contract content to restore.

## Cleared (verified carried, no action)

- Event-driven dev loop with coalescing, cancellation, and shutdown handling
  (`entrypoint.rs`), including the 10ms settle-debounce pin test.
- Parallel middleware execution contract (`execution_engine.rs` via `FuturesUnordered`,
  dedicated test).
- Package-manager-agnostic frontend commands (config-driven
  `js_package_manager_base_cmd`).
- Signal handling parity (ctrl-c + SIGTERM, non-unix fallback).
- Public-static hash caching (improved stat-revalidated design).
- Form-urlencoded decoding via `url::form_urlencoded` (tested).
- Runtime asset path-traversal rejection (tested).
- Atomic output copies (temp + rename), stale-output retention.
- `sha2` + `blake3` coexistence in `vorma` is justified (CSP requires SHA-256; blake3 is
  internal content hashing) — not a regression.
- Escaping parity: baseline `htmlutil.rs` test matrix maps one-to-one onto
  `document_renderer.rs` tests (attribute/text escaping, known-safe attribute
  preservation, style raw-text end-tag escaping, void elements + boolean attributes,
  invalid tag/attribute-name rejection), plus a duplicate-attribute rejection that is
  stronger than baseline.
- Refresh protocol parity: the baseline `browser_sync.rs` `ChangeType` enum carried
  verbatim into `dev_refresh.rs` — identical six variants and serde renames.
- Document-hash parity: `root_document_hash_source` flows through live state into
  `projection_compiler` manifest projection, so document changes alter `client_build_id`
  and produce refresh effects.
- Cargo/child-process invocation parity: process-group isolation and grandchild
  termination carried with tests (unix); env hygiene carried (and the live-state key gap
  restored — finding 4).
- Browser-refresh slow-client handling: carried — `dev_refresh` broadcast uses bounded
  per-client queues and removes clients on `TrySendError::Full` as well as `Closed`,
  matching the baseline disconnect-slow-clients semantics.
- Serve-time symlink-escape rejection: carried via canonicalize + containment check in
  `load_public_asset_body`; now pinned by an explicit escape test.

## Restoration / Adjudication Checklist

- [x] Output-layout file lock (restored; `cargo test -p vorma-build` green)
- [x] `dist/.vorma/.gitignore` write in `build_output.rs` (restored, tested)
- [x] Output-directory symlink-component defense (restored at all three creation sites,
      tested)
- [x] Live-state key restored to the child env-clear set (tested)
- [x] Sweep `specs/BACKEND_RUNTIME.md` invariants against the new runtime (stub; prose
      invariants covered — see Deleted Maintainer Docs)
- [x] Escaping parity sweep (parity confirmed — see Cleared)
- [x] Refresh-protocol parity (identical enum — see Cleared)
- [x] Cargo invocation parity (carried; env gap restored — see Cleared/finding 4)
- [x] Document-hash parity (carried — see Cleared)
- [x] Adjudicate the 41 dropped-test clusters (complete — see Cluster Adjudication; gaps
      extracted to Port Queue)
- [x] Adjudicate the 4 dropped TS test names (3 renames/contract-governed; 1 in Port
      Queue)
- [x] Behavior Questions: all eight resolved (maintainer rulings recorded above)
- [x] Watcher root-escape restoration (explicit `../` and absolute outside-root sources;
      two tests)
- [x] Production asset byte cache restoration (content-hash-keyed, dev exempt; two tests)
- [x] Serve-time symlink-escape pin test
- [x] Vercel current-dir manifest fallback restoration (both-paths error; test)
- [x] Dev refresh-script CSP hash gating restoration (test)
- [x] Combined-cargo parallel bin build restoration (one invocation builds both bins;
      prebuilt executables run directly; live-state target-move falls back to `cargo run`
      for one activation; Port Queue item 5 — identical-target rejection — landed with it)
- [ ] Streaming refinement for the combined build: hand off the build-entry executable
      from cargo's artifact stream before the server bin finishes linking
- [x] Windows new-process-group flag ported (baseline-exact `cfg(windows)` branch)
- [x] Port Queue items 1, 7, 8 resolved (verified carried / ported — see Port Queue)
- [x] Port Queue items 6, 9, 10 resolved (6 and 9 verified carried + pinned; 10 was a real
      gap, restored + pinned — see Port Queue)
- [x] Items 3 and 4 API-level verification: `set_header`/`append_header` exist on both
      handler context types (ordered `Set|Add` op model in `response_finalizer`), and
      middleware contexts expose head access — the _merge-order_ pins for both fold into
      the matrix session below since they share the execution-engine harness
- [x] Effect-merge concurrency matrix session complete (Port Queue 2, 3, 4): seven pins
      landed; engine verified to uphold all baseline contracts — see Port Queue
- [x] Streaming refinement for the combined cargo build: early build-entry handoff from
      cargo's artifact stream, live-state read overlapping the app-server link
      (deterministic overlap test; std-runner incremental-read test)
- [x] TS `wasm_readiness` prestart pin (Port Queue 11 — verified carried + pinned)

All audit findings are now restored, pinned, or recorded as maintainer decisions.
Remaining standing recommendation: run `make gate` (including the bombadil e2e dev-changes
variants) before committing the branch.

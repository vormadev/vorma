# Framework Audit Evidence (Append-Only)

## Usage

- Append-only log of concrete audit evidence.
- Do not delete prior entries in this file.
- Reference entries from `FRAMEWORK_AUDIT.md` matrix completion decisions.

## Entry Format

- ID: `EV-YYYYMMDD-###`
- Package group
- Pass name
- Evidence
    - files reviewed
    - commands/tests run
    - findings/fixes or explicit none-found statement

---

### EV-20260214-001

- Package group: tracker/process
- Pass: audit process reset
- Evidence:
    - Reset `FRAMEWORK_AUDIT.md` to high-signal state-only model.
    - Added explicit pass gate requiring evidence IDs.
    - Expanded scope to full repo (`kit/*`, `lab/*`, `internal/*`, and framework
      packages).
    - Added explicit non-negotiable: no perf regressions without user approval.

### EV-20260214-002

- Package group: tracker/process
- Pass: performance policy clarification
- Evidence:
    - Updated `FRAMEWORK_AUDIT.md` non-negotiables to enforce: performance
      regressions are unacceptable unless required for correctness or explicitly
      approved.
    - Scope of policy explicitly covers both dev-time and runtime behavior.

### EV-20260214-003

- Package group: tracker/process
- Pass: perf-evidence methodology clarification
- Evidence:
    - Added `Performance Evaluation Policy` to `FRAMEWORK_AUDIT.md`.
    - Policy now explicitly requires benchmarks for hot paths and allows
      first-principles logical analysis for non-hot paths.

### EV-20260214-004

- Package group: tracker/process
- Pass: tracker section restoration
- Evidence:
    - Restored `Audit Questions` section in `FRAMEWORK_AUDIT.md`.
    - Restored `Change Authorization Policy` section in `FRAMEWORK_AUDIT.md`.

### EV-20260214-005

- Package group: `wave/*`
- Pass: surface/api, correctness, fragility, dry, performance, test-quality,
  failure-modes
- Evidence:
    - Files reviewed:
        - Runtime/config/API files: `wave/wave.go`, `wave/parse.go`,
          `wave/types.go`, `wave/types_methods.go`,
          `wave/parsed_config_clone.go`, `wave/framework_runtime_state.go`,
          `wave/runtime_framework.go`, `wave/runtime_fs.go`,
          `wave/runtime_assets.go`, `wave/runtime_static.go`,
          `wave/runtime_refs.go`, `wave/css.go`, `wave/refresh.go`,
          `wave/cache_internal.go`, `wave/env.go`, `wave/filemap.go`,
          `wave/internal/pathnorm/pathnorm.go`.
        - Test files: `wave/wave_runtime_test.go`,
          `wave/wave_additional_test.go`, `wave/wave_cache_behavior_test.go`,
          `wave/parse_test.go`, `wave/types_test.go`,
          `wave/framework_runtime_state_test.go`, `wave/refresh_test.go`,
          `wave/env_test.go`, `wave/cache_test.go`,
          `wave/flexibility_contract_test.go`, `wave/fuzz_test.go`,
          `wave/internal/pathnorm/pathnorm_test.go`,
          `wave/test_helpers_test.go`.
    - Commands/tests run:
        - `go test ./wave ./wave/internal/pathnorm -count=1` (pass).
    - Findings/fixes:
        - Fixed public URL ref-path normalization to prevent `../` traversal
          from escaping configured public prefix in runtime URL generation
          (`wave/runtime_refs.go`).
        - Added traversal regression coverage for stylesheet and public-filemap
          internal ref files (`wave/wave_additional_test.go`).
        - Fixed `FaviconRedirect` map-resolution behavior to support valid
          identity-mapped entries (for example, nohash/prehashed `favicon.ico`)
          while still returning 404 when unmapped (`wave/runtime_static.go`).
        - Added favicon identity-mapping regression coverage
          (`wave/wave_additional_test.go`).
        - Open finding retained: `Wave.IsPublicAsset` caches negative results
          for unbounded distinct paths in production (`wave/runtime_assets.go`,
          `wave/cache_internal.go`).

### EV-20260214-006

- Package group: `wave/tooling/*`
- Pass: surface/api, correctness, fragility, dry, performance, test-quality,
  failure-modes
- Evidence:
    - Files reviewed:
        - Core API/build/devserver orchestration: `wave/tooling/builder.go`,
          `wave/tooling/builder_validation.go`, `wave/tooling/builder_build.go`,
          `wave/tooling/devserver.go`, `wave/tooling/devserver_runtime.go`,
          `wave/tooling/devserver_config.go`,
          `wave/tooling/devserver_app_process.go`,
          `wave/tooling/devserver_vite_process.go`,
          `wave/tooling/devserver_refresh_server.go`,
          `wave/tooling/devserver_restart.go`,
          `wave/tooling/devserver_readiness.go`,
          `wave/tooling/devserver_builder_accessors.go`.
        - Event pipeline/workset/hook execution: `wave/tooling/events.go`,
          `wave/tooling/events_execution.go`,
          `wave/tooling/events_build_phase.go`,
          `wave/tooling/events_execution_decisions.go`,
          `wave/tooling/events_execution_continuation.go`,
          `wave/tooling/events_execution_app_stop.go`,
          `wave/tooling/events_commands.go`,
          `wave/tooling/events_hook_executor.go`, `wave/tooling/events_plan.go`,
          `wave/tooling/events_plan_hooks.go`,
          `wave/tooling/events_plan_behavior.go`,
          `wave/tooling/events_watcher_intake.go`,
          `wave/tooling/events_classification.go`,
          `wave/tooling/events_classification_pre.go`,
          `wave/tooling/events_classification_post.go`,
          `wave/tooling/events_classification_mapping.go`,
          `wave/tooling/events_dedup.go`, `wave/tooling/events_dedup_index.go`,
          `wave/tooling/events_dedup_paths.go`,
          `wave/tooling/events_workset_implicit.go`,
          `wave/tooling/events_workset_browser.go`,
          `wave/tooling/events_workset_refresh.go`,
          `wave/tooling/timeout_policy.go`.
        - Watch/static/CSS/filemap path: `wave/tooling/watcher.go`,
          `wave/tooling/watcher_dirs.go`, `wave/tooling/watcher_setup.go`,
          `wave/tooling/watcher_matching.go`,
          `wave/tooling/watcher_debouncer.go`, `wave/tooling/static.go`,
          `wave/tooling/static_full_build.go`,
          `wave/tooling/static_incremental.go`,
          `wave/tooling/static_file_processing.go`,
          `wave/tooling/static_filemap_io.go`,
          `wave/tooling/static_resolution.go`,
          `wave/tooling/static_resolution_collision.go`,
          `wave/tooling/static_resolution_fileinfo.go`,
          `wave/tooling/static_artifacts.go`, `wave/tooling/css.go`,
          `wave/tooling/css_build_context.go`,
          `wave/tooling/css_build_entries.go`,
          `wave/tooling/css_build_execution.go`, `wave/tooling/css_output.go`,
          `wave/tooling/css_resolver.go`, `wave/tooling/url.go`,
          `wave/tooling/builder_runtime_outputs.go`, `wave/tooling/hash.go`,
          `wave/tooling/hashed_artifact.go`, `wave/tooling/lock.go`,
          `wave/tooling/lock_unix.go`, `wave/tooling/lock_windows.go`,
          `wave/tooling/cli.go`.
    - Commands/tests run:
        - `go test ./wave/tooling -count=1` (pass).
        - `go test -race ./wave/tooling -count=1` (pass).
    - Findings/fixes:
        - Fixed build-time public URL fallback normalization to prevent `../`
          traversal from escaping configured public path prefix when file map
          loading fails (`wave/tooling/url.go`,
          `wave/tooling/builder_runtime_outputs.go`).
        - Added regression coverage for traversal-safe fallback behavior in both
          `Builder.GetPublicURLBuildtime` and cached CSS resolver fallback
          (`wave/tooling/url_filemap_test.go`).
        - No additional open findings recorded for `wave/tooling/*` in this
          pass.

### EV-20260214-007

- Package group: `wave/*`
- Pass: correctness/test-quality follow-up for open finding closure
- Evidence:
    - Files reviewed:
        - `wave/cache_internal.go`
        - `wave/wave.go`
        - `wave/wave_cache_behavior_test.go`
    - Commands/tests run:
        - `go test . ./vormabuild ./wave ./wave/internal/pathnorm ./wave/tooling -count=1`
          (pass).
    - Findings/fixes:
        - Closed the previously open `Wave.IsPublicAsset` negative-lookup cache
          growth finding by adding cache-map policy support and using it for
          `Wave.isAsset` so only positive asset hits are retained.
        - Added regression coverage that verifies negative production lookups
          are re-evaluated and do not remain cached after the file later exists
          (`wave/wave_cache_behavior_test.go`).
        - No remaining open `wave/*` findings or test gaps from this issue.

### EV-20260214-008

- Package group: `vorma.go`
- Pass: surface/api, correctness, fragility, dry, performance, test-quality,
  failure-modes
- Evidence:
    - Files reviewed:
        - `vorma.go`
        - Existing runtime registration coverage:
          `vormabuild/discovered_registration_runtime_test.go`
    - Commands/tests run:
        - `go test . -count=1` (pass).
        - `go test -race . -count=1` (pass).
        - `go test . ./vormabuild ./wave ./wave/internal/pathnorm ./wave/tooling -count=1`
          (pass).
    - Findings/fixes:
        - Added fail-fast argument validation for public wrapper constructors:
          `NewLoader` and `NewAction` now panic with explicit messages when
          handler or decorator inputs are nil (`vorma.go`).
        - Added fail-fast app/argument validation for discovered runtime
          registration helpers: `Internal__RegisterDiscoveredLoader` and
          `Internal__RegisterDiscoveredAction` (`vorma.go`).
        - Added direct root-package regression tests for facade behavior and
          failure modes (`vorma_test.go`), including:
            - loader/action decorated-context execution behavior.
            - nil-argument panic guards.
            - embedded package version extraction consistency.
        - No additional open findings recorded for `vorma.go` in this pass.

### EV-20260214-009

- Package group: `bootstrap/*`
- Pass: surface/api, correctness, fragility, dry, performance, test-quality,
  failure-modes
- Evidence:
    - Files reviewed:
        - `bootstrap/bootstrap.go`
        - `bootstrap/utils.go`
        - `bootstrap/ux_conformance_test.go`
        - Template and asset set under `bootstrap/tmpls/*` and
          `bootstrap/assets/*`
    - Commands/tests run:
        - `go test ./bootstrap -count=1` (pass).
        - `go test -race ./bootstrap -count=1` (pass).
        - `go test . ./bootstrap ./vormabuild ./wave ./wave/internal/pathnorm ./wave/tooling -count=1`
          (pass).
    - Findings/fixes:
        - Fixed `Options.derived()` to fail fast on unknown `JSPackageManager`
          values instead of allowing partial scaffold writes before a later
          install-time panic (`bootstrap/bootstrap.go`).
        - Added regression coverage for unknown package-manager validation in
          derived options (`bootstrap/ux_conformance_test.go`).
        - No additional open findings recorded for `bootstrap/*` in this pass.

### EV-20260214-010

- Package group: `vormabuild/*`
- Pass: correctness/failure-modes (in-progress package row)
- Evidence:
    - Files reviewed:
        - `vormabuild/build_cli.go`
        - `vormabuild/build_cli_test.go`
    - Commands/tests run:
        - `go test ./vormabuild -count=1` (pass).
        - `go test -race ./vormabuild -count=1` (pass).
        - `go test . ./bootstrap ./vormabuild ./wave ./wave/internal/pathnorm ./wave/tooling -count=1`
          (pass).
    - Findings/fixes:
        - Fixed build CLI argument handling to reject unexpected positional
          arguments instead of silently ignoring them
          (`vormabuild/build_cli.go`).
        - Added regression coverage for positional-argument rejection in build
          flag parsing (`vormabuild/build_cli_test.go`).
        - `vormabuild/*` full multi-pass package audit remains in progress.

### EV-20260214-011

- Package group: tracker/process
- Pass: full-gate enforcement and matrix granularity correction
- Evidence:
    - Commands/tests run:
        - `make gotest`:
            - first run in sandbox failed when `httptest.NewServer` attempted to
              bind `tcp6 [::1]:0` in `wave/tooling` tests.
            - rerun outside sandbox completed successfully across all Go
              packages.
        - `make tstest` (pass).
        - `make tscheck` (pass).
        - `make tslint` (pass with warnings, no errors).
        - `go list ./kit/... ./lab/... ./internal/...` (package discovery for
          matrix expansion).
    - Findings/fixes:
        - Added explicit full-gate requirement to tracker non-negotiables in
          `FRAMEWORK_AUDIT.md`: `make gotest`, `make tstest`, `make tscheck`,
          `make tslint`.
        - Replaced coarse matrix rows `kit/*`, `lab/*`, and `internal/*` with
          per-package rows derived from `go list` output in
          `FRAMEWORK_AUDIT.md`.

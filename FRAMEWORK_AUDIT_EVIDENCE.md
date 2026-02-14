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

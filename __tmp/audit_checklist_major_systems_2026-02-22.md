# Open Items

Scope: `wave/*`, `internal/vormaruntime/*`, `kit/mux/*`, `vormabuild/*`,
`vormagogen/*`

## Current Ownership Baseline

### `vormabuild/artifactio` package-owned tests

- `vormabuild/artifactio/atomic_file_write_test.go`
- `vormabuild/artifactio/artifact_snapshot_test.go`
- `vormabuild/artifactio/resource_runner_test.go`
- `vormabuild/artifactio/fs_to_hash_test.go`

### `vormabuild/artifactcleanup` package-owned tests

- `vormabuild/artifactcleanup/artifactcleanup_test.go`

### `vormabuild/buildlifecycle` package-owned tests

- `vormabuild/buildlifecycle/lifecycle_state_machine_test.go`
- `vormabuild/buildlifecycle/rollback_transaction_test.go`
- `vormabuild/buildlifecycle/route_sync_test.go`
- `vormabuild/buildlifecycle/runtime_state_commit_test.go`
- `vormabuild/buildlifecycle/runtime_state_snapshot_test.go`
- `vormabuild/buildlifecycle/runtime_state_guard_test.go`

### `vormabuild/backendroutes` package-owned tests

- `vormabuild/backendroutes/backend_route_discovery_test.go`
- `vormabuild/backendroutes/backend_route_registration_generation_test.go`

### `vormabuild/backendroutes/sourceparse` package-owned tests

- `vormabuild/backendroutes/sourceparse/sourceparse_test.go`

### `vormabuild/backendroutes/registraroverlay` package-owned tests

- `vormabuild/backendroutes/registraroverlay/registraroverlay_test.go`

### `vormabuild/backendroutes/registrationgraph` package-owned tests

- `vormabuild/backendroutes/registrationgraph/registrationgraph_test.go`

### `vormabuild/routeartifacts` package-owned tests

- `vormabuild/routeartifacts/routeartifacts_test.go`
- `vormabuild/routeartifacts/stage_one_paths_writer_test.go`
- `vormabuild/routeartifacts/stage_two_paths_test.go`
- `vormabuild/routeartifacts/stage_two_error_paths_test.go`

### `vormabuild/devreload` package-owned tests

- `vormabuild/devreload/rebuild_routes_test.go`
- `vormabuild/devreload/reload_endpoint_test.go`
- `vormabuild/devreload/build_watch_test.go`
- `vormabuild/devreload/watch_hooks_test.go`

### `vormabuild/buildenv` package-owned tests

- `vormabuild/buildenv/buildenv_test.go`

### `vormabuild/buildinner` package-owned tests

- `vormabuild/buildinner/buildinner_test.go`
- `vormabuild/buildinner/build_artifacts_test.go`
- `vormabuild/buildinner/route_sync_merge_test.go`
- `vormabuild/buildinner/build_id_test.go`

### `vormabuild/buildflow` package-owned tests

- `vormabuild/buildflow/build_runtime_test.go`
- `vormabuild/buildflow/vite_cmd_test.go`
- `vormabuild/buildflow/vite_cmd_error_paths_test.go`

### `vormagogen` package-owned tests

- `vormagogen/vormagogen_test.go`
- `vormagogen/runtime_registration_test.go`

### `vormabuild` root tests that intentionally stay in root

- `vormabuild/build_cli_test.go` (build command flag parsing and hook dispatch
  decisions)
- `vormabuild/multipackage_runtime_registration_e2e_test.go` (cross-package
  registration discovery e2e ownership)

### `internal/vormaruntime` root tests that intentionally stay in root

- `internal/vormaruntime/get_root_handler_test.go` (end-to-end request pipeline
  orchestration)
- `internal/vormaruntime/route_reload_test.go` (runtime dev-reload
  orchestration/state updates)
- `internal/vormaruntime/http_contract_matrix_test.go` (cross-route HTTP
  contract integration behavior)
- `internal/vormaruntime/vorma_init_test.go` (runtime init orchestration and
  lifecycle semantics)
- `internal/vormaruntime/glue_test.go` (public runtime API glue behavior and
  config boundaries)
- `internal/vormaruntime/vorma_core_test.go` (root runtime state/read APIs and
  lifecycle guarantees)
- `internal/vormaruntime/route_registry_test.go` (root route/action registry
  integration behavior)
- `internal/vormaruntime/ssr_test.go` (root runtime-state to SSR payload wiring
  semantics)
- `internal/vormaruntime/runtime_artifacts_test.go` (root
  runtime/internal-artifact conversion helpers)
- `internal/vormaruntime/errors_test.go` (root error-type behavior)
- `internal/vormaruntime/vormaruntime_bench_test.go` (root runtime benchmark
  ownership)

## Active TODOs

- local workspace hygiene: remove stray nested
  `wave/wavedev/devserver/__LLM_CONCAT.local` artifact directory and ensure the
  concat tooling writes only to the intended workspace-level output location.

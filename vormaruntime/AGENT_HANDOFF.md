# AGENT_HANDOFF

## Scope

- Edit only `vormaruntime/`.
- Tests and benchmarks must encode first-principles observable behavior.
- Keep frontend/backend public contract shape stable while tightening
  correctness.

## Current Status (2026-02-12)

- The package has broad contract coverage for:
    - loader/action HTTP behavior and JSON/HTML response shape
    - redirect/error short-circuit behavior
    - `HEAD` semantics via the default router
    - action parsing/content-type and method-mount behavior
    - startup/init validation and route synchronization
    - dev reload guards, endpoint behavior, and reload/read coherence
    - SSR payload generation and serializability guards
    - static middleware serving and passthrough behavior
- New correctness fixes in this batch:
    1. `getUIRouteData` now short-circuits before `GetDefaultHeadEls`;
       default-head hook failures no longer override
       redirect/proxy-error/not-found responses.
    2. Loader-error route data now trims returned `deps` to the same outermost
       error-boundary scope as returned route match data.
    3. `Init()` re-init path now replaces parsed routes, rebuilds nested-route
       registrations against current parsed paths while preserving server
       handlers, and clears route-data cache.
    4. `Init()` is now failure-atomic for fallible load/parse steps; failed
       re-init no longer partially mutates runtime route/build/template state.
- New regression tests in this batch:
    - `TestLoadersHandler_DefaultHeadErrorsDoNotOverrideShortCircuitResponses`
    - `TestLoadersHandler_LoaderErrorDepsAreTrimmedToOutermostBoundary`
    - `TestInit_ReinitReplacesRemovedClientRoutes`
    - `TestInit_ReinitInvalidatesRouteDataCacheWhenBuildIDUnchanged`
    - `TestInit_ReinitPreservesServerOnlyHandlerRoutes`
    - `TestInit_ReinitFailureDoesNotPartiallyMutateRuntimeState`
    - `TestInit_ReinitMalformedStageFileDoesNotPartiallyMutateRuntimeState`

## Verification (Latest Run)

- `go test ./vormaruntime -count=1`
- `go test -race ./vormaruntime -count=1`
- `go test ./vormaruntime -cover -count=1`
- `go test ./kit/mux ./vormaruntime -cover -count=1`
- Latest coverage: `97.2%` statements.

## Benchmark Artifacts Policy

- Store benchmark raw `go test -bench` stdout only under
  `vormaruntime/benchmark_results/`.
- Keep only standard Go benchmark output (`goos/goarch/cpu` header + benchmark
  lines).
- If host load is unstable, defer benchmark capture and note it here.

## Next Highest-Leverage Work

1. Continue matrix-style loader/action contracts only where remaining branches
   can hide realistic regressions.
2. Expand re-init and dev-reload contract coverage around remaining observable
   state boundaries without coupling tests to internals.
3. Keep this handoff concise and remove stale notes each update batch.

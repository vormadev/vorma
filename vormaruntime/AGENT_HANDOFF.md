# AGENT_HANDOFF

## Scope

- Edit only `vormaruntime/`.
- Keep tests and benchmarks focused on first-principles observable behavior.
- Keep frontend/backend contract shape stable while tightening correctness.

## Current Status (2026-02-12)

- Test harness is broad across loader/action contracts, init/re-init, dev
  reload, cache invalidation, and concurrent reload/request behavior.
- Coverage: `97.1%` statements.
- Aggressive internal refactors continue with behavior locked by tests.

## Latest Completed Work

- Loader handling now fails safely (HTTP 500) instead of panicking when a
  request reaches `GetLoadersHandler` without `TasksCtx` injection.
- `LockedVorma.SetPaths` now defensively clones caller-provided path maps and
  invalidates route-data cache entries, preventing stale route-data cache hits
  after in-place runtime path updates.
- Route-data CSS-bundle resolution is now snapshot-coherent with stage-1 route
  selection; requests no longer mix old build route data with new build CSS
  during a mid-request dev reload.
- Stage-1 loader prep now minimizes `v.mu` read-lock scope by taking coherent
  runtime snapshots under lock and building route-data cache subsets after
  unlock.
- Loader build/header coherence is enforced under concurrent dev reload.
- Stale JSON requests (`vorma_json` mismatch) are handled via a single stage-1
  stale path and return `X-Vorma-Reload` without running loaders.
- Stage-1 loader-error cut behavior is explicit via `routeErrorCutPlan`: route
  data cuts at the outermost error boundary, and route head elements include
  only successful ancestor loaders.
- Route loader head elements are collected using a bounded prefix collector,
  replacing the previous collect-then-slice-then-flatten path.
- Route-registry and init/runtime snapshots clone caller-owned state to prevent
  caller mutation leakage across reloads.
- Loader error handling keeps generic client fallback semantics and avoids
  leaking server error text.
- Route-data cache keys now use per-app precomputed identity
  (`_routeDataCacheAppIdentity`) to avoid pointer-to-hex encoding work on every
  request.

## Key Regression Tests

- `TestRouteRegistrySyncFromDevReload_ClonesPathEntries`
- `TestRouteRegistryReplaceParsedPathsForInit_ClonesPathEntries`
- `TestLoaderErrorError_NilServerFallsBackToClientMessage`
- `TestLoaderErrorError_NoServerOrClientUsesSafeFallback`
- `TestLoaderErrorError_NilReceiverUsesSafeFallback`
- `TestLoadersHandler_RuntimeDoesNotMutateAppTemplateDataMap`
- `TestLoadersHandler_EmptyLoaderErrorClientMessageFallsBackToGeneric`
- `TestLoadersHandler_ConcurrentReloadAndRequests_OnlyServeCoherentArtifactSets`
  (extended to assert build-header/artifact coherence)
- `TestLoadersHandler_ConcurrentReloadAndStaleJSONRequests_DoNotSilentlyServeNewBuildData`
- `TestLoadersHandler_ConcurrentReloadAndStaleJSONRequests_WithRedirectingLoaders_DoNotBypassReload`
- `TestLoadersHandler_JSONBuildAndRouteDataBehavior/stale_build_json_request_returns_reload_header`
  (extended to assert stale JSON requests do not run loaders)
- `TestLoadersHandler_HTMLExcludesFailingRouteHeadElementsOnLoaderError`
- `TestLoadersHandler_ReloadDuringRequest_DoesNotMixCSSFromNewBuild`
- `TestLockedVormaSetPaths_InvalidatesRouteDataCacheAndClonesInput`
- `TestLoadersHandler_MissingTasksCtxDoesNotPanicAndReturns500`

## Verification (Latest)

- `go test ./vormaruntime -count=1`
- `go test -race ./vormaruntime -count=1`
- `go test ./vormaruntime -cover -count=1`
- `go test ./vormaruntime -run='^$' -bench='Benchmark(LoadersHandler_JSONCurrentBuild|RouteDepsAndCSSResolution|LoadersHandler_HTMLCurrentBuild|LoadersHandler_JSONCurrentBuild_ColdCache)$' -benchmem -count=3`
- `go test ./vormaruntime -run='^$' -bench='Benchmark(ActionsHandler_POSTJSONCurrentBuild|ActionsHandler_GETQueryCurrentBuild)$' -benchmem -count=3`
- `go test ./vormaruntime -run='^$' -bench='BenchmarkSSRInnerHTMLGeneration$' -benchmem -count=3`

All passed.

## Benchmark Baseline And Current References

ORIGINAL:

- `benchmark_results/ORIGINAL_raw.txt` (captured `2026-02-11 15:43:53`)

SECOND_TO_LATEST:

- `benchmark_results/SECOND_TO_LATEST_raw.txt` (captured `2026-02-12 10:14:35`)

FRESH:

- `benchmark_results/FRESH_raw.txt` (captured `2026-02-12 10:20:18`)

FRESH vs ORIGINAL (`ns/op`):

- `BenchmarkLoadersHandler_JSONCurrentBuild`: `-47.6%`
- `BenchmarkLoadersHandler_JSONCurrentBuild_ColdCache`: `-33.0%`
- `BenchmarkRouteDepsAndCSSResolution`: `-12.3%`
- `BenchmarkLoadersHandler_HTMLCurrentBuild`: `-11.9%`
- `BenchmarkActionsHandler_POSTJSONCurrentBuild`: `-8.2%`
- `BenchmarkActionsHandler_GETQueryCurrentBuild`: `-7.1%`
- `BenchmarkSSRInnerHTMLGeneration`: `-3.9%`

FRESH vs SECOND_TO_LATEST (`ns/op`):

- `BenchmarkLoadersHandler_JSONCurrentBuild`: `-6.2%`
- `BenchmarkLoadersHandler_JSONCurrentBuild_ColdCache`: `-5.5%`
- `BenchmarkRouteDepsAndCSSResolution`: `-3.6%`
- `BenchmarkLoadersHandler_HTMLCurrentBuild`: `-7.5%`
- `BenchmarkActionsHandler_POSTJSONCurrentBuild`: `+6.7%`
- `BenchmarkActionsHandler_GETQueryCurrentBuild`: `+5.4%`
- `BenchmarkSSRInnerHTMLGeneration`: `+2.7%`

Notes:

- This `FRESH` capture includes the missing-`TasksCtx` panic hardening and its
  regression test.

Directory policy:

- Keep only `ORIGINAL`, `SECOND_TO_LATEST`, and `FRESH` benchmark artifacts
  (plus `README.md`).
- Delete all intermediary captures after each new `FRESH` run.

## Next Highest-Leverage Work

1. Continue simplifying loader/root orchestration while preserving fail-fast
   descendant cancellation semantics and parent continuation semantics.
2. Keep adding behavior-level regression tests only for realistic production
   risks exposed by each refactor.
3. Keep per-slice benchmark comparisons (`loader/actions/SSR`) against the old
   baseline after each source-change batch.

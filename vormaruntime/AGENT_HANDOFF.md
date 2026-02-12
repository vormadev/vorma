# AGENT_HANDOFF

## Scope

- Work only in `vormaruntime/`.
- Keep tests focused on first-principles observable behavior.
- Keep public/frontend-backend contract shape stable.

## Current Status (2026-02-12)

- `vormaruntime` test harness is broad and refactor-oriented.
- Current coverage: `97.2%` statements.
- Latest verification:
    - `go test ./vormaruntime -count=1`
    - `go test -race ./vormaruntime -count=1`
    - `go test ./vormaruntime -cover -count=1`

## Latest Completed Work

- Fixed a cache coherence bug risk where a stale pre-reload request snapshot
  could repopulate `gmpdCache` after cache invalidation.
- Added per-app route-data snapshot version invalidation under write lock:
    - `Vorma.invalidateRouteDataCacheLocked()`
    - wired through `LockedVorma.SetPaths`, `RouteRegistry.SyncFromDevReload`,
      and `RouteRegistry.ReplaceParsedPathsForInit`.
- Updated stage-1 cached subset build/store flow to conditionally store only
  when the snapshot version is still current.
- Added regression test:
    - `TestLoadOrBuildCachedItemSubset_DoesNotStoreWhenSnapshotVersionIsStale`
      (`gmpd_cache_test.go`).

## Benchmark Snapshot

Artifacts (stable filenames):

- `benchmark_results/ORIGINAL_raw.txt` (captured `2026-02-11 15:43:53`)
- `benchmark_results/SECOND_TO_LATEST_raw.txt` (captured `2026-02-12 10:20:18`)
- `benchmark_results/FRESH_raw.txt` (captured `2026-02-12 11:45:47`)

Comparison method:

- Use median `ns/op` from each benchmark’s 3 runs in each artifact.

`FRESH` vs `ORIGINAL` (`ns/op`):

- `BenchmarkLoadersHandler_JSONCurrentBuild`: `-43.18%`
- `BenchmarkLoadersHandler_JSONCurrentBuild_ColdCache`: `-21.88%`
- `BenchmarkRouteDepsAndCSSResolution`: `-4.71%`
- `BenchmarkLoadersHandler_HTMLCurrentBuild`: `-7.20%`
- `BenchmarkActionsHandler_POSTJSONCurrentBuild`: `-10.71%`
- `BenchmarkActionsHandler_GETQueryCurrentBuild`: `-7.59%`
- `BenchmarkSSRInnerHTMLGeneration`: `-1.31%`

`FRESH` vs `SECOND_TO_LATEST` (`ns/op`):

- `BenchmarkLoadersHandler_JSONCurrentBuild`: `+7.84%`
- `BenchmarkLoadersHandler_JSONCurrentBuild_ColdCache`: `+14.02%`
- `BenchmarkRouteDepsAndCSSResolution`: `+9.86%`
- `BenchmarkLoadersHandler_HTMLCurrentBuild`: `+5.04%`
- `BenchmarkActionsHandler_POSTJSONCurrentBuild`: `-3.38%`
- `BenchmarkActionsHandler_GETQueryCurrentBuild`: `+0.52%`
- `BenchmarkSSRInnerHTMLGeneration`: `+1.71%`

## Next Highest-Leverage Work

1. Continue aggressive internal simplification in loader/root orchestration
   while keeping current observable contracts fixed.
2. Add/adjust tests only when they increase refactor confidence for production
   behavior (not incidental internals).
3. Keep benchmarking against `ORIGINAL` and `SECOND_TO_LATEST` after each source
   batch; if host load is unstable, defer capture and note it.

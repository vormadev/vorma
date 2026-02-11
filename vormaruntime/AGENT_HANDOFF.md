# AGENT_HANDOFF

## Scope

- Primary target: `vormaruntime/`
- Policy: tests/benchmarks should encode first-principles behavior (not
  incidental implementation quirks)
- API constraint: do not change public frontend/backend contract shape

## Current Status

- Comprehensive test harness now exists across runtime behavior, dev reload
  guards, SSR output, loader/action contracts, and concurrency/reload churn.
- `kit/mux` tests were expanded around nested execution semantics and now cover
  route replacement/rebuild behavior and shared dependency cache semantics.
- Benchmark harness now targets whole-package production hot paths, not dev-only
  reload flows.
- Baseline-on-old-code requirement is satisfied for the current benchmark suite
  (details in `vormaruntime/bench.txt`).

## Verification Executed

- `go test ./vormaruntime -count=1`
- `go test -race ./vormaruntime -count=1`
- `go test ./vormaruntime -cover -count=1`
- `GOCACHE=/tmp/go-build go test ./vormaruntime -run=^$ -bench=. -benchmem -count=3`
- `go test ./kit/tasks ./kit/mux -count=1`
- `go test -race ./kit/mux -count=1`
- `go test ./kit/mux ./vormaruntime -cover -count=1` (`kit/mux: 87.6%`,
  `vormaruntime: 87.8%`)

## Benchmarks In Scope

`vormaruntime/vormaruntime_bench_test.go` currently covers:

- `BenchmarkLoadersHandler_JSONCurrentBuild`
- `BenchmarkLoadersHandler_HTMLCurrentBuild`
- `BenchmarkLoadersHandler_JSONCurrentBuild_ColdCache`
- `BenchmarkRouteDepsAndCSSResolution`
- `BenchmarkSSRInnerHTMLGeneration`
- `BenchmarkActionsHandler_POSTJSONCurrentBuild`
- `BenchmarkActionsHandler_GETQueryCurrentBuild`

`BenchmarkReloadRoutesFromDisk` was removed intentionally (dev-time only, not a
production hot path).

## Current Perf Picture

See `vormaruntime/bench.txt` for full old-vs-current numbers.

Key takeaways:

- Loader path regressions were materially reduced by `kit/mux` optimization.
- Loader HTML is now faster than old baseline with alloc parity.
- Loader JSON allocs are now better than old baseline; CPU still trails old
  baseline.
- Cold-cache JSON allocs/bytes improved versus old baseline; CPU still trails.
- Route deps/CSS is slightly slower than old baseline.
- SSR is slightly slower than old baseline.
- Actions GET/POST are near or slightly better.
- Latest same-load comparison run happened under heavy unrelated host CPU
  contention; see `vormaruntime/bench.txt` "Contended Host Snapshot" section.

## Known Completed Behavior Fixes

- Wrapped loader errors are recognized correctly.
- Loader handler parses `vorma_json` query once per request on the hot path.
- Route-data cache isolation includes app identity/mode/build.
- Route-data cache key construction no longer uses `fmt.Fprintf` on every
  request.
- `kit/mux` nested task execution now:
    - avoids per-task full `tasks.NewCtx` allocation
    - uses shared-cache child contexts for descendant cancellation
    - executes single-task paths inline (no goroutine spawn)
- `kit/mux` route-replacement behavior now has direct regression tests:
    - `ReplaceRoutes` fully replaces matcher + compiled route state
    - `RebuildPreservingHandlers` keeps handler routes and refreshes no-handler
      patterns
- `kit/mux` nested task tests now assert shared dependency tasks execute once
  across parent/child handlers when using request-scoped task contexts.
- `vormaruntime` startup/route-sync contracts now have direct tests for:
    - stage-one/stage-two path file selection
    - malformed/missing path file failures
    - nested-router decoration of missing patterns
    - `RegisterPatternIfNeeded` idempotence
- Multi-app head dedupe leakage is fixed.
- Reload/read concurrency coherence is hardened and stress-tested.
- Dev-only reload methods/endpoints are explicitly guarded.
- Nested router cancellation semantics are directional:
    - parent failure cancels descendants
    - descendant failure does not cancel parent work

## Next High-Leverage Work

1. Re-run full benchmark suite on an uncontended host to refresh stable
   old-vs-current figures (current same-load snapshot is recorded, but noisy).
2. Reduce remaining CPU gaps in loader JSON cold-cache, route deps/CSS, and SSR
   without weakening correctness contracts.
3. Keep `bench.txt` and this handoff current after each perf-affecting change.
4. Continue adding first-principles tests only when they increase external
   contract confidence (avoid internals coupling).

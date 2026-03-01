# Runtime Regression Handoff Checklist (2026-03-01)

## Goal

Identify and remove request-time runtime overhead that explains logs like:

- `GET /docs?... -> 304 in ~4-5ms` on `refactor-2026-8`
- compared with `~0.5-1ms` on `main`

This checklist is runtime-only (not build orchestration/watcher noise).

## Repro + Measurement Commands

- Request-path benchmark:

```bash
go test ./internal/vormaruntime -run=^$ -bench 'BenchmarkLoadersHandler_JSONCurrentBuild$|BenchmarkLoadersHandler_HTMLCurrentBuild$|BenchmarkRouteDepsAndCSSResolution$' -benchmem -count=3
```

- JSON hot-path allocation profile:

```bash
GOMAXPROCS=1 go test ./internal/vormaruntime -run=^$ -bench BenchmarkLoadersHandler_JSONCurrentBuild$ -benchmem -benchtime=3s -memprofile /tmp/vorma_json.mem
go tool pprof -top -alloc_objects /tmp/vorma_json.mem
go tool pprof -top -alloc_space /tmp/vorma_json.mem
```

Observed locally on this branch:

- `BenchmarkLoadersHandler_JSONCurrentBuild`: ~`6.1µs`, `112 allocs/op`
- `BenchmarkLoadersHandler_HTMLCurrentBuild`: ~`49.9µs`, `457 allocs/op`
- Historical branch artifact
  (`internal/vormaruntime/benchmark_results/FRESH_raw.txt`): ~`4.7µs`,
  `83 allocs/op`

## Findings Checklist (Runtime Path)

- [x] **Per-request runtime snapshot path conversion**
    - Current request path calls `BuildRuntimeSnapshotFromCore` on every request
      in `internal/vormaruntime/vormaruntime.go:645-664`.
    - `BuildRuntimeSnapshotFromCore` calls `convertRuntimeCorePathsToPathData`
      (`internal/vormaruntime/routepipeline/routepipeline.go:256-302`) which
      allocates a fresh map and `PathData` objects per request.
    - Allocation profile shows this frame as a recurring allocator.
    - Main comparison: old stage-1 flow in `main:internal/framework/gmpd.go`
      used `v._paths` directly and did not perform this conversion step per
      request.
    - Status: fixed in this branch by changing `routepipeline.RuntimeSnapshot`
      paths to reference `runtimecore.RoutePath` directly (no per-request
      conversion map/object build in `BuildRuntimeSnapshotFromCore`).

- [x] **Redundant route-data finalization work in JSON path**
    - `BuildRouteDataFinal` resolves deps/css first
      (`routepipeline.go:122-129`), then if `routeResult.Assets != nil` (which
      is true in the success path), it replaces those with cloned values
      (`routepipeline.go:133-139`).
    - `LoadersHandler` always calls `getUIRouteData` before
      `BuildRouteDataFinal` (`vormaruntime.go:52-80`), and `getUIRouteData` sets
      `Assets` in the returned result (`vormaruntime.go:745-770`).
    - This means duplicate string processing + clone allocations are paid every
      request.
    - Allocation profile shows `resolveClientAssetPath(s)` +
      `cloneStringSlice` + `BuildRouteDataFinal` as top allocators.
    - Status: fixed in this branch by making `BuildRouteDataFinal` skip core
      deps/css resolve work when `routeResult.Assets` is present and already
      authoritative.
    - Follow-up landed: removed request-path defensive slice cloning in
      `BuildRouteDataFinal` for core/asset fields.

- [x] **Default-head generation lost overlap with stage-1 route work**
    - Current code runs stage-1 first, then default head generation sequentially
      (`internal/vormaruntime/vormaruntime.go:727` then `:735`).
    - Main comparison: old `main:internal/framework/gmpd.go` used `errgroup` so
      default-head work overlapped with stage-1 (`lines 277-296`).
    - Status: fixed in this branch. When a custom `getDefaultHeadEls` hook is
      configured, route stage-1 and default-head resolution now run in parallel
      with request-context cancellation propagation. Fast path with no custom
      hook keeps the no-extra-work branch.

- [x] **Nested task execution path changed to custom goroutine/context chain**
    - Old `main` nested path used `tasksCtx.RunParallel(boundTasks...)`
      (`main:kit/mux/nested_mux.go:305-310`).
    - Current path in `kit/nestedmux/nestedmux.go`:
        - builds parent/child cancel chain with `context.WithCancel`
          (`:417-429`)
        - executes with custom goroutines/waitgroup (`runBoundTasks`,
          `:509-528`)
    - Allocation profile shows `nestedmux.RunTasks` as one of the highest
      request-path allocators.
    - Semantic intent to preserve:
        - keep parallel execution
        - keep sibling independence
        - keep fail-fast only for descendants (route-aware cancellation)
    - Status: fixed in this branch with additional reductions:
        - removed per-request child `tasks.Ctx.WithNativeContext` allocations by
          borrowing/reusing shared-state child task contexts
          (`kit/tasks/tasks.go`,
          `AcquireSharedStateChildContextWithNativeContext`)
        - removed `pooledReqData` tracking slice and cleaned pooled objects from
          `boundTasks` directly
        - reduced goroutine launch overhead by running one bound task on the
          caller goroutine and launching the rest
    - Benchmark delta (`BenchmarkNestedRouter/Nested_Tasks_Execution`):
        - before: ~`2.5µs`, `1601 B/op`, `28 allocs/op`
        - after: ~`1.8-2.1µs`, `1474 B/op`, `23 allocs/op`

- [x] **HEAD fallback now buffers full GET response body**
    - Current `treatGetAsHead` uses `httptest.NewRecorder()`
      (`kit/mux/mux.go:1023-1041`).
    - Main used a lightweight `headResponseWriter` discarding body
      (`main:kit/mux/mux.go:745-768`).
    - This is runtime overhead for HEAD->GET fallback routes (not the direct
      `/docs` GET example, but still a runtime change).
    - Status: fixed in this branch by restoring a lightweight
      `headResponseWriter` that discards body bytes while preserving inferred
      headers and status.

## Security Rationale Notes (Do Not Assume Defensive Copying Is Security-Critical)

- **Per-request runtime snapshot path conversion is not a user-facing security
  control**
    - This appears to be a runtime-state immutability/concurrency guard, not an
      auth/injection/secrets boundary.
    - If removed, replace with a safer non-copy approach (for example: immutable
      atomic snapshot pointers) to avoid concurrent map access hazards.

- **`BuildRouteDataFinal` cloning is not a security boundary**
    - Cloning in `BuildRouteDataFinal` looks like defensive alias protection and
      normalization.
    - No evidence this is required to prevent external attackers; it mainly
      protects against internal mutation mistakes.

- **Nested task executor rewrite reason is semantic correctness, not security**
    - Inline comment in `kit/nestedmux/nestedmux.go:409-411` says the move away
      from `RunParallel` is to avoid global sibling fail-fast cancellation
      masking per-route errors.
    - Treat as correctness/perf tradeoff, not security hardening.

- **Request-abort cancellation must remain global**
    - Current flow creates tasks context from request context in `mux`
      (`kit/mux/mux.go:546`, also middleware path at `:1071`), so client abort
      cancels the task tree.
    - `nestedmux` child contexts derive from that parent native context
      (`kit/nestedmux/nestedmux.go:419-421`), so request cancellation still
      propagates to all matched loaders.
    - Any perf refactor must keep this invariant.
    - Status: preserved and covered by test `kit/nestedmux/nestedmux_test.go`:
      `TestRunNestedTasks/Request_Cancellation_Cancels_All_Matched_Tasks`.

- **HEAD fallback `httptest.NewRecorder` change is protocol/behavioral, not
  security**
    - The newer implementation appears aimed at better HEAD fallback behavior
      (for example content-length/header parity), not threat mitigation.

- **Deps caching was not removed; verify cache shape before refactor**
    - Old `main` cached deps per matched-pattern set in `gmpdCache`
      (`main:internal/framework/gmpd.go:111-137`).
    - Current runtime caches metadata (including deps) per match stack + build
      snapshot in `LoadOrBuildCachedItemSubset`
      (`internal/vormaruntime/runtimehttp/runtimehttp.go:370-385`,
      `internal/vormaruntime/routepipeline/routepipeline.go:374-410`).
    - This is still route-cache behavior, not per-request recomputation in the
      success path.

## Runtime Areas Checked With No Clear New Request-Time Cost

- **Site middleware stack wiring**
    - `internal/site/backend/src/router/init.go` stack order and middleware set
      are materially the same as old `main` router core.
    - No obvious newly inserted expensive middleware in this stack.

- **ETag middleware on GET/304 path**
    - `kit/middleware/etag/etag.go` has interface-support changes
      (`Flush/Hijack/Push`) and directive parsing cleanup.
    - No clear algorithmic cost increase identified for the normal GET + 304
      flow relative to the full request cost above.

- **fsmarkdown docs loader path**
    - Current `lab/fsmarkdown/fsmarkdown.go` is mostly a package
      relocation/rename from `main:kit/lab/fsmarkdown/fsmarkdown.go`.
    - No obvious new request-time heavy step found from this diff alone.

## Matcher / Nested Matcher Specific Checklist

- [x] **Confirm no behavioral/perf regression from matcher core extraction**
    - `kit/matcher` and `kit/nestedmatcher` now wrap `kit/internal/matchercore`.
    - Need an apples-to-apples benchmark run against `main` for:
        - `FindBestMatch` dynamic/splat cases
        - `FindNestedMatches` deep/mixed cases
    - Note: current repo has `kit/matcher/results.bench.txt`, but not a direct
      `main` side-by-side capture for this exact environment/date.
    - Status: benchmarked and optimized matcher-core nested path in this branch.
      `BenchmarkFindMatches` now shows lower allocation counts across cases:
        - Static: `5` -> `4 allocs/op`
        - Dynamic: `14` -> `12 allocs/op`
        - Deep: `15` -> `13 allocs/op`
        - Splat: `12` -> `10 allocs/op`
        - Mixed: `11` -> `10 allocs/op`
    - Key runtime-core changes:
        - removed per-request `longestSegmentMatches` map allocation
        - avoided empty-params map cloning in nested DFS
        - removed stable-sort/scratch requirement in flatten/sort path via
          total-order comparator

## First-Principles Runtime Opportunities (Current Code, Not Diff-Only)

- [x] **Head element flatten path allocates empty head containers for routes
      that never set head**
    - `CollectFlattenedHeadElementsForPrefix` calls
      `responseProxies[routeIdx].HeadEls().Collect()` for every route
      (`internal/vormaruntime/routepipeline/routepipeline.go:640-657`).
    - `Proxy.HeadEls()` lazily allocates `headels.New()` when `_head_els == nil`
      (`kit/response/response.go:395-400`), so routes with no head writes still
      allocate.
    - JSON alloc profile shows both
      `routepipeline.CollectFlattenedHeadElementsForPrefix` and
      `kit/response.(*Proxy).HeadEls` in allocation path.
    - Status: fixed in this branch by adding non-allocating proxy read path
      (`HeadElsIfPresent`) and skipping nil/no-head proxies in flattening.
    - Security note: this is a performance-only memory/allocation issue, not a
      trust-boundary check.

- [x] **TODO: optimize `headels.HeadEls` hot-path lock/clone overhead without
      changing contracts**
    - Current `HeadEls` uses a mutex and clones in `Collect`: `Add` lock/unlock
      (`kit/headels/headblocks.go:408-410`), `AddElements` calls
      `other.Collect()` then lock (`:413-420`), `Collect` lock + `slices.Clone`
      (`:423-427`).
    - Main comparison: older version had direct append/return semantics (no
      lock/clone), so this is a plausible request-time regression in
      loader-heavy routes.
    - Routepipeline/head merge path calls into these operations repeatedly via
      proxy head handling.
    - Status: fixed in this branch for the hot runtime path by avoiding
      `Collect()` clone calls during route head flattening: `HeadEls.Len()` +
      `HeadEls.AppendElementsInto(...)` now feed
      `CollectFlattenedHeadElementsForPrefix` directly, which removes per-route
      clone allocations while preserving `Collect()` clone semantics and
      concurrency safety.
    - Bench impact observed in runtime benches:
      `BenchmarkLoadersHandler_JSONCurrentBuild` allocs `82 -> 81`,
      `BenchmarkLoadersHandler_HTMLCurrentBuild` allocs `427 -> 426`.
    - Security note: this is not a true security control; it is defensive
      internal mutation/concurrency protection.

- [x] **TODO: optimize head dedupe hash construction path**
    - `hashElement` allocates/sorts key slices
      (`kit/headels/headblocks.go:160-170`, `188-190`) and is called from
      `dedupeHeadEls` for non-rule elements (`:249`).
    - HTML alloc profile shows `headels.hashElement` and
      `headels.(*Instance).ToSortedAndPreEscapedHeadEls` as direct alloc
      contributors.
    - Target outcome: avoid full hash path when structural fast-paths can prove
      uniqueness, and avoid repeated sort/allocation for common small-element
      shapes.
    - Status: fixed in this branch by adding low-overhead fast paths in
      `hashElement` for common tiny shapes:
        - direct no-sort hashing when combined attribute count is `0` or `1`
        - direct no-sort hashing when boolean attribute count is `0` or `1`
        - stack-backed key buffers for small attribute/boolean sets before
          falling back to heap allocation for larger sets. This preserves
          deterministic hashing and dedupe behavior.
    - Bench impact observed in runtime benches:
      `BenchmarkLoadersHandler_JSONCurrentBuild` allocs remain `81`,
      `BenchmarkLoadersHandler_HTMLCurrentBuild` allocs improved `426 -> 419`.
    - Security note: this is deterministic dedupe behavior cost, not security
      hardening.

- [x] **TODO: replace per-element dedupe `fmt.Sprintf` rule-key construction**
    - `dedupeHeadEls` builds `ruleKey` with `fmt.Sprintf("rule:%s:%d", ...)`
      inside per-element loop (`kit/headels/headblocks.go:232`).
    - Target outcome: replace string formatting with a small struct/integer key
      to remove formatting allocations in dedupe hot path.
    - Status: fixed in this branch by replacing formatted string keys with a
      typed composite map key (`tag` + `ruleIdx`) in `dedupeHeadEls`. Dedupe
      semantics and ordering behavior are unchanged.
    - Bench impact observed in runtime benches:
      `BenchmarkLoadersHandler_JSONCurrentBuild` allocs remain `81`,
      `BenchmarkLoadersHandler_HTMLCurrentBuild` allocs remain `419`.
    - Security note: key representation choice is unrelated to security.

- [x] **Proxy merge always allocates full merge structures even in trivial
      cases**
    - `MergeProxyResponses` always allocates a merged head container, header
      map, cookie dedupe map, and cookie sort slice
      (`kit/response/response.go:548-598`).
    - JSON alloc profile shows `kit/response.MergeProxyResponses` in hot path.
    - Status: fixed in this branch by lazy-allocating merge internals (head,
      header map, cookie structures) only when populated.
    - Security note: merge structure shape is not a security control.

- [x] **Nested task path allocates result/proxy objects for every matched
      pattern**
    - `nestedmux.RunTasks` allocates `*TasksResult` per match and creates
      `response.NewProxy()` even when no task handler exists
      (`kit/nestedmux/nestedmux.go:356-387`).
    - JSON alloc profile shows `nestedmux.RunTasks` + `response.NewProxy` as top
      allocators.
    - Status: fixed in this branch by using value-slice `TasksResult` storage
      and creating proxies only for matched task-handler routes.
    - Security note: this is internal data-shape overhead, not hardening.

- [x] **Task middleware execution path rebuilt middleware slices and proxy
      objects per request**
    - `gatherAllTaskMiddlewares` concatenates global/method/route middleware
      slices every request (`kit/mux/mux.go:656-669`).
    - `runAppropriateMws` allocates per-middleware `ReqData` and
      `response.NewProxy()`, then allocates a second proxies slice for merge
      (`kit/mux/mux.go:728-767`).
    - Status: fixed in this branch by introducing compiled task middleware-chain
      caching with global/method/pattern version invalidation and by reducing
      per-request temporary proxy/list allocations in `runAppropriateMws`.
    - Security note: middleware composition/copying here is not security-driven.

- [x] **TasksCtx-only request transport still allocates a response proxy**
    - `muxcore.NewRequestDataWithTasksCtxOnly` always creates
      `response.NewProxy()` (`kit/internal/muxcore/muxcore.go:53-63`).
    - JSON/HTML alloc profiles include this path from
      `mux.InjectTasksCtxMiddleware`.
    - Status: fixed in this branch by making
      `muxcore.NewRequestDataWithTasksCtxOnly` leave `responseProxy` nil.
      `InjectTasksCtxMiddleware` tests cover nil-proxy behavior.
    - Security note: this is internal transport shape, not a safety barrier.

- [x] **TODO: make task memoization cheaper without changing task semantics**
    - Reduce fixed overhead in `tasks.(*Ctx).getOrCreateResult` on hot framework
      paths (`map`/lock/result-entry setup).
    - Must keep the `Ctx`-scoped contract: same task + same input within one
      `tasks.Ctx` dedupes to one execution (shared in-flight/result state).
    - No no-memo mode; that was tried and reverted.
    - Status: fixed in this branch with internals-only reductions: `taskResult`
      now embeds `sync.Once` by value (no extra allocation), and the cache now
      stores value entries (`map[taskKey]cacheEntry`) instead of per-key
      `*cacheEntry` heap objects; `Ctx`-scoped dedupe semantics unchanged.
    - Bench impact observed in runtime benches:
      `BenchmarkLoadersHandler_JSONCurrentBuild` allocs `84 -> 82`,
      `BenchmarkLoadersHandler_HTMLCurrentBuild` allocs `429 -> 427`.

- [x] **Nested matcher allocates maps/slices and sorts on every request**
    - `FindNestedMatches` builds a `matches` map and repeatedly mutates it
      (`kit/internal/matchercore/matchercore.go:681-804`).
    - `dfsNestedMatches` clones params maps per match
      (`matchercore.go:816-829`).
    - `flattenAndSortMatches` allocates a result slice and stable-sorts each
      request (`matchercore.go:880-905`).
    - Profiles show `flattenAndSortMatches`, `dfsNestedMatches`, and
      `ParseSegments` in hot allocation paths.
    - Status: improved in this branch while preserving matcher determinism
      tests. Main reductions:
        - no per-request longest-type helper map
        - no empty params map copies during DFS
        - lower-overhead flatten/sort path
    - Bench impact captured in `BenchmarkFindMatches` (see matcher-core item).
    - Security note: matcher container strategy is not a security boundary.

- [x] **TODO: cache matched-pattern materialization with route metadata subset**
    - `BuildExecutionInputsFromMatchResults` always calls
      `CollectMatchedPatterns(matches)`
      (`internal/vormaruntime/runtimehttp/runtimehttp.go:367-370`).
    - JSON alloc profile shows `routepipeline.CollectMatchedPatterns`.
    - Target outcome: include matched patterns in cached subset keyed by match
      stack and snapshot version.
    - Status: fixed in this branch by adding `MatchedPatterns` to
      `CachedItemSubset` and materializing it once in `BuildCachedItemSubset`;
      `BuildExecutionInputsFromMatchResults` now reads matched patterns from the
      cached subset and only falls back to `CollectMatchedPatterns` for
      malformed/manual cached entries. Regression test added for fallback
      safety:
      `TestBuildExecutionInputsFromMatchResults_FallbacksMatchedPatternsWhenMissingInCachedSubset`.
    - Bench impact observed in runtime benches:
      `BenchmarkLoadersHandler_JSONCurrentBuild` allocs `81 -> 80`,
      `BenchmarkLoadersHandler_HTMLCurrentBuild` allocs `419 -> 418`.
    - Security note: this data is internal metadata, not user-controlled trust.

- [x] **TODO: move HTML JSON-serializability validation out of steady-state
      request path**
    - `BuildSSRInnerHTMLFromRuntimeState` loops through `routeData.LoadersData`
      and encodes each value to `io.Discard`
      (`internal/vormaruntime/rendering/rendering.go:242-250`).
    - HTML alloc profile shows encoding/json encoder frames in this path.
    - Target outcome: move this validation out of steady-state request path (for
      example dev-only assertion or dedicated tests).
    - Status: fixed in this branch by removing the extra per-loader
      `json.NewEncoder(io.Discard)` validation loop and replacing it with one
      required serialization pass: `json.Marshal(routeData.LoadersData)` is now
      used directly for SSR script output (`LoadersDataJSON`), so
      non-serializable data still fails loudly while duplicate request-path
      serialization work is removed.
    - Regression coverage added:
      `TestBuildSSRInnerHTMLFromRuntimeState_ValidatesLoadersDataJSON` (failure
      contract) and
      `TestBuildSSRInnerHTMLFromRuntimeState_LoadersDataJSONEscapesScriptTerminators`
      (escape-safety contract).
    - Bench impact observed in runtime benches:
      `BenchmarkLoadersHandler_JSONCurrentBuild` allocs remain `80`,
      `BenchmarkLoadersHandler_HTMLCurrentBuild` allocs `418 -> 416`.
    - Security note: serializability checks here are correctness diagnostics,
      not attack-surface controls.

- [x] **TODO: cache/hoist static head escape-sort work**
    - `ToSortedAndPreEscapedHeadEls` always dedupes and escapes every element
      (`kit/headels/headblocks.go:127-149`), and HTML alloc profile shows this
      path with `htmlutil.EscapeIntoTrusted`.
    - A meaningful subset (framework-added preload/stylesheet links and stable
      defaults) can be precomputed at snapshot/build boundaries.
    - Target outcome: cache pre-escaped/stable head fragments and merge only
      truly request-variant elements at request time.
    - Status: fixed in this branch for the static production subset. Added a
      strict fast path in `BuildRouteAssets` that caches full
      `SortedAndPreEscapedHeadEls` output when all of the following are true:
        - production HTML mode
        - no default head elements
        - no route-emitted head elements Cache storage lives on
          `CachedItemSubset` and is keyed by resolved deps/css lists so
          prefix-cut/error variants do not collide. Dynamic head paths still use
          the existing per-request dedupe/escape flow unchanged.
    - Regression coverage added:
      `TestBuildRouteAssets/production_html_static_only_head_uses_cached_sorted_output`
      and
      `TestBuildRouteAssets/production_html_static_only_head_cache_keys_on_deps_and_css`.
    - Bench impact observed in runtime benches:
      `BenchmarkLoadersHandler_JSONCurrentBuild` allocs remain `80`,
      `BenchmarkLoadersHandler_HTMLCurrentBuild` allocs `416 -> 362`.
    - Security note: preserving `EscapeIntoTrusted` correctness is required; the
      opportunity is when and how often this is done, not skipping escaping.

- **Out of current scope (dev-only): wave static middleware can call `fs.Stat`
  on every request in dev**
    - `cacheMap.get` bypasses caching in dev (`wave/wave.go:110-113`).
    - `MustStaticMiddleware` calls `w.isPublicAsset(req.URL.Path)` for every
      request (`wave/wave.go:1811-1827`).
    - `isPublicAsset -> checkIsAsset` performs `fs.Stat` per lookup
      (`wave/wave.go:1558-1574`).
    - Target outcome: add short-lived dev cache for positive + negative asset
      lookups to avoid repeated filesystem stats on app-route requests.
    - Security note: this lookup decides static serving behavior, not authz.

- [x] **ETag middleware did eager buffering/header-copy work for all GET/HEAD
      responses before it knows eligibility**
    - `Auto` always wraps with `newETagWriter` before route execution
      (`kit/middleware/etag/etag.go:53-56`).
    - `newETagWriter` eagerly clones headers and builds `io.MultiWriter`
      (`etag.go:90-104`), and body bytes are always buffered/hashed until
      `canUseETag` check runs (`etag.go:214-220`).
    - Status: fixed in this branch by removing eager header cloning and
      `io.MultiWriter` setup in `etagWriter`, writing/hash-buffering directly on
      demand, and preserving middleware behavior contracts.
    - Security note: this is cache/protocol behavior, not threat mitigation.

- **Out of current scope (dev-only): `fsmarkdown` dev request path is
  intentionally uncached and has unbounded fan-out goroutines**
    - Cache reads are guarded by `!inst.IsDev` in page/base/sitemap fetch
      (`lab/fsmarkdown/fsmarkdown.go:89-91`, `181-183`, `334-336`).
    - `processDirectChildren` starts one goroutine per direct child entry
      (`lab/fsmarkdown/fsmarkdown.go:269-301`).
    - This was also present on `main` (no obvious new diff regression), but it
      is still a clear current-code perf opportunity for docs-heavy paths.
    - Target outcome: bounded worker pool + optional short dev cache for sitemap
      and parsed markdown.
    - Security note: this is not protective hardening; it is runtime throughput.

## Scope Recheck: internal/site + fsmarkdown

- **Double-check conclusion for `internal/site` markdown runtime**
    - `internal/site/backend/src/router/loaders.go` route `/*` calls
      `Markdown.PageDetails` for docs/blog rendering.
    - `lab/fsmarkdown` behavior is not newly heavier vs `main` by diff, but it
      remains a major dev-time request-cost contributor due disabled caching and
      repeated filesystem/markdown work.
    - Keep this on the shortlist while tuning router/runtime internals to avoid
      chasing tails.

## Handoff Execution Plan (Hardest First)

- [x] Remove per-request path snapshot conversion in route execution inputs.
- [x] Simplify `BuildRouteDataFinal` to avoid duplicate resolve/clone work.
- [x] Revisit nested task executor path and reduce context/goroutine overhead.
- [x] Restore parallel overlap for default head generation + stage-1 route work.
- [x] Revert HEAD fallback buffering behavior.
- [x] Run same benchmark suite and capture before/after in a new `__tmp` note.

## Done Criteria

- [x] `BenchmarkLoadersHandler_JSONCurrentBuild` returns to (or below) previous
      branch-local baseline and alloc count drops materially from
      `112 allocs/op`.
    - Current after latest fixes: ~`5.1-5.6µs`, `85 allocs/op`.
- [ ] Local manual request logs for `/docs?...` 304 are back near prior
      microsecond-scale range under the same local conditions.
- [x] No behavior changes in loader/action correctness tests.
    - Verified in this pass with:
        - `go test ./kit/tasks -count=1`
        - `go test ./kit/nestedmux -count=1`
        - `go test ./internal/vormaruntime -count=1`

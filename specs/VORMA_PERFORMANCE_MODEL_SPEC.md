# Vorma Performance Model Specification

Status: Draft  
Last Updated: 2026-02-07  
Applies To: Vorma build/dev pipeline, backend runtime, wire behavior, and frontend runtime performance contracts

## 1. Why This Spec Exists

This document defines performance contracts and budgets that refactors MUST
preserve (or intentionally improve with documented tradeoffs).

It exists to:

- make performance regressions visible before release,
- preserve fast-feedback workflows in dev mode,
- provide objective criteria for API and architecture changes.

## 2. Conformance Boundaries

### 2.1 Normative Terms

The terms **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

### 2.2 Performance Scope

Performance scope includes:

- build and rebuild critical paths,
- runtime request/response assembly,
- client navigation and render transition latency,
- memory growth bounds for long-lived sessions.

### 2.3 Measurement Philosophy

Performance conformance is evaluated through repeatable benchmark profiles and
regression deltas, not one-off anecdotal timings.

## 3. Terminology

- Framework Overhead: time attributable to Vorma/Wave plumbing, excluding
  user-provided business logic and external service latency.
- Full Dev Rebuild: rebuild path including route parse/artifact generation and
  standard dev-server rebuild/reload flow.
- Fast Route Rebuild: route-def-only dev rebuild path (`rebuildRoutesOnly`).
- Cold Run: first run with no warm caches.
- Warm Run: subsequent run with caches and watcher process already active.
- Navigation TTI (Vorma scope): elapsed time from navigation trigger to route
  change commit event (`vorma:route-change`) for the framework-controlled path.

## 4. Requirement Catalog

## 4.1 Measurement and Benchmark Governance

### PERF-MEAS-001: Deterministic Benchmark Profiles

Performance gates MUST run against named benchmark profiles with fixed fixture
apps and reproducible command invocations.

### PERF-MEAS-002: Cold and Warm Reporting

Each benchmark profile MUST report cold and warm results separately.

### PERF-MEAS-003: Percentile Reporting

Interactive/runtime benchmarks SHOULD report at least p50 and p95.

### PERF-MEAS-004: Baseline History

Project SHOULD maintain historical benchmark baselines to evaluate deltas,
not only absolute current values.

### PERF-MEAS-005: Framework vs App Cost Separation

Benchmark reporting SHOULD separate framework overhead from fixture-app logic
where feasible (for example controlled no-op loaders/actions).

### PERF-MEAS-006: Stable Hardware/Runtime Metadata

Benchmark outputs SHOULD include runtime metadata (OS, CPU class, Node/Go
versions) to make regressions comparable.

### PERF-MEAS-007: Budget Change Process

Any intentional budget relaxation MUST be accompanied by spec update and
justification note.

## 4.2 Build and Dev Pipeline Performance

### PERF-BUILD-001: Fast Route Rebuild Relative Advantage

Given same fixture app and environment  
When comparing fast route rebuild vs full dev rebuild  
Then fast route rebuild SHOULD remain materially faster (target: at least 5x
faster on median).

### PERF-BUILD-002: Fast Route Rebuild Target Envelope

Given warm dev process and route-def-only change  
When fast route rebuild executes  
Then framework overhead target SHOULD be within sub-250ms p95 for reference
fixture sizes.

### PERF-BUILD-003: Full Dev Rebuild Target Envelope

Given warm dev process and non-trivial source change requiring normal rebuild  
When full dev rebuild executes  
Then framework overhead target SHOULD remain within low-seconds p95 for
reference fixture sizes.

### PERF-BUILD-004: Callback Fallback Boundedness

Given dev callback endpoint call path  
When endpoint is unavailable  
Then fallback-to-restart path MUST fail fast and avoid unbounded waiting.

### PERF-BUILD-005: Reload Endpoint Call Timeout Bound

Reload endpoint callback calls MUST remain bounded by explicit timeout
(current contract: 10s max request timeout).

### PERF-BUILD-006: File Watch Debounce Bound

Watcher event batching MUST use bounded debounce window (current contract:
30ms) to prevent rebuild storms.

### PERF-BUILD-007: Generated Output Rewrite Minimization

Build/generation steps SHOULD skip rewriting unchanged generated files where
content hash/equality permits.

### PERF-BUILD-008: No Quadratic Route Artifact Growth

Route artifact generation MUST scale approximately linearly with number of
routes for canonical route-definition patterns.

## 4.3 Backend Runtime Performance

### PERF-BE-001: Parallel Loader Execution

Given independent matched loaders  
When route data is assembled  
Then execution MUST preserve parallel behavior semantics so total latency is
bounded near slowest loader plus framework overhead.

### PERF-BE-002: Route Data Assembly Linearity

Given N matched routes  
When assembling route-data arrays  
Then framework overhead SHOULD scale roughly O(N), avoiding avoidable nested
quadratic work.

### PERF-BE-003: Cached Path Metadata Reuse

Given repeated requests matching same normalized route pattern set  
When assembling import/export/dependency metadata  
Then cached path metadata SHOULD be reused to reduce repeated expensive work.

### PERF-BE-004: Stale JSON Response Lightness

Given stale build JSON request  
When backend returns stale reload signal  
Then response SHOULD remain lightweight (no full route-data computation).

### PERF-BE-005: Head Assembly Overhead Control

Given head element aggregation from defaults/loaders/assets  
When final head set is built  
Then framework SHOULD avoid repeated unnecessary render work for unchanged
head inputs where practical.

## 4.4 Wire and Payload Efficiency

### PERF-WIRE-001: Route-Data Payload Shape Efficiency

Route-data payloads SHOULD include only contract fields needed for runtime
functionality and MUST preserve index-aligned compact arrays.

### PERF-WIRE-002: Dev vs Prod Dependency Payload Behavior

Given dev mode vs prod mode navigation responses  
When dependency preload fields are emitted  
Then payload behavior MUST follow mode-specific contract without duplicating
unneeded dependency payload.

### PERF-WIRE-003: Header Overhead Boundedness

Framework-specific headers (`X-Vorma-Build-Id`, redirect/reload headers) MUST
remain minimal and bounded in size.

## 4.5 Frontend Runtime Performance

### PERF-FE-001: Status Event Throttling

Status emission MUST remain debounced and deduplicated to prevent excessive
render/listener churn.

### PERF-FE-002: Revalidation Coalescing Bound

Rapid revalidation triggers MUST coalesce within bounded interval (current
contract: ~8ms window) to avoid redundant network work.

### PERF-FE-003: Prefetch Deduplication

Concurrent/repeated prefetches for same target MUST reuse in-flight result and
avoid duplicate fetch requests.

### PERF-FE-004: Navigation Reuse of Completed Prefetch

User navigation to prefetched target SHOULD reuse prefetched result to reduce
TTI and duplicate fetch overhead.

### PERF-FE-005: Asset Injection Deduplication

Client runtime MUST dedupe modulepreload and stylesheet injections to prevent
head growth and repeated network work.

### PERF-FE-006: Scroll State Memory Bound

Scroll-state persistence MUST remain bounded (current contract: max 50 entries)
to prevent unbounded session storage growth.

### PERF-FE-007: Redirect Loop Bound

Redirect processing MUST remain bounded (current contract: max 10 hops) to
prevent infinite redirect churn.

### PERF-FE-008: No Mid-Chain Loading Gaps

During chained transitions (submit->revalidate, redirect chains, asset/loader
wait), loading state continuity SHOULD be preserved so UI does not flicker idle
between in-flight framework tasks.

### PERF-FE-009: Head Reconciliation Operation Minimization

Head reconciliation SHOULD minimize DOM mutations by reusing equivalent
existing elements and only moving/inserting/removing when needed.

## 4.6 Memory and Resource Bounds

### PERF-RES-001: Navigation State Cleanup

Completed/aborted navigation entries MUST be removed from runtime state to
prevent accumulation over long sessions.

### PERF-RES-002: Submission State Cleanup

Submission tracking entries MUST be removed on completion/abort/failure.

### PERF-RES-003: Event Listener Cleanup

Listener registration APIs MUST provide cleanup semantics and consumers SHOULD
use cleanup to prevent long-lived listener leaks.

### PERF-RES-004: Build Artifact Cleanup

Build pipeline MUST clean stale Vorma-generated outputs before writing fresh
artifacts to prevent storage bloat and stale file scan overhead.

## 4.7 Performance Regression Gates

### PERF-GATE-001: Changed-Surface Benchmarks Required

Given a PR changes behavior in build/runtime/client critical paths  
When validating merge readiness  
Then relevant benchmark profiles SHOULD be executed and compared to baseline.

### PERF-GATE-002: Regression Threshold Policy

Project SHOULD define explicit allowed regression thresholds per profile
(example: <=10% p95 regression unless justified).

### PERF-GATE-003: Fail-on-Unjustified Regression

Given measured regressions exceed allowed threshold without accepted rationale  
When evaluating release readiness  
Then change SHOULD be treated as non-conformant.

### PERF-GATE-004: Ratio Guard for Dev Feedback Loop

Fast route rebuild/full rebuild speed ratio SHOULD be tracked as a first-class
health metric for developer feedback-loop quality.

## 4.8 Performance-Oriented API Design Guidance

### PERF-API-001: Prefer Explicit Incremental APIs

New APIs SHOULD enable incremental updates (route subset reload, targeted
revalidation, dedup hooks) rather than forcing full recomputation.

### PERF-API-002: Avoid Hidden Global Invalidations

APIs that invalidate global caches/state SHOULD be explicit and measurable,
not hidden behind unrelated calls.

### PERF-API-003: Observable Timing Hooks

Critical long-running operations SHOULD emit diagnostics (start/end, duration)
so regression attribution is practical.

## 5. Conformance Test and Benchmark Guidance

Performance conformance program SHOULD include:

1. dev fast route rebuild benchmark profile,
2. dev full rebuild benchmark profile,
3. runtime loader parallelism benchmark profile,
4. frontend navigation latency profile with prefetch/no-prefetch scenarios,
5. long-session memory/resource leak checks (navigation/listener growth).

## 6. Relation to Other Specs

- Build/dev contracts: `/Users/sjc/__code/river/specs/VORMA_BUILD_DEV_CONFORMANCE_SPEC.md`
- Backend runtime contracts: `/Users/sjc/__code/river/specs/VORMA_BACKEND_RUNTIME_SPEC.md`
- Wire contracts: `/Users/sjc/__code/river/specs/VORMA_WIRE_CONTRACT_SPEC.md`
- Frontend runtime contracts: `/Users/sjc/__code/river/specs/VORMA_FRONTEND_RUNTIME_SPEC.md`
- Observability contracts: `/Users/sjc/__code/river/specs/VORMA_OBSERVABILITY_DEBUG_SPEC.md`
- Testing strategy: `/Users/sjc/__code/river/specs/VORMA_TESTING_STRATEGY_SPEC.md`
- Roadmap checklist: `/Users/sjc/__code/river/specs/SPECS_CHECKLIST.md`

# Vorma Client Simplification Plan (2026-02-23)

## Scope

Improve whole-runtime simplicity, dedupe, and aggregate client bundle efficiency
for `typescript/vorma/client/src` without changing framework behavior.

## Checklist

### 1) Remove dead redirect plumbing

- [x] Remove unused `isPrefetch` parameter threading from redirect fetch path.
- [x] Remove unused `newURL` value from redirect target parsing.
- [x] Keep redirect behavior parity for navigation/prefetch promotion paths.
- [x] Add/adjust unit tests for redirect path signatures and behavior.

### 2) Consolidate request-body serialization logic

- [x] Introduce one shared request-body normalization/serialization helper.
- [x] Reuse it from `app/helpers.ts` (`resolveVormaRequestBody`).
- [x] Reuse it from `core/redirects.ts` (`buildRedirectRequestInit`).
- [x] Preserve content-type and method/body semantics.
- [x] Add/adjust tests to verify parity across both callsites.

### 3) Eliminate duplicated CSS application responsibilities

- [x] Choose one authoritative stage for stylesheet application.
- [x] Keep prefetch behavior intentional (decide whether prefetch should apply
      CSS or only preload).
- [x] Remove duplicate `AssetManager.applyCSS` invocation path.
- [x] Codify prefetch commit boundary: pure prefetch may warm internal framework
      state (for example build ID and internal cache state), but must not mutate
      committed page state (stylesheet/title/head/history/route-change commit).
- [x] Add contract tests for navigation + prefetch CSS behavior after refactor.
- [x] Add contract tests that assert internal warm-up is allowed while
      page-committed mutations are blocked during pure prefetch.

### 4) Make debug journal production-optional

- [x] Gate debug journal runtime machinery behind a compile-time/dev guard.
- [x] Keep public debug APIs stable but no-op in production builds.
- [x] Ensure navigation lifecycle still works identically when disabled.
- [x] Add tests for dev-enabled and prod-disabled behavior.
- [x] Add artifact-level production-bundle test that asserts debug transition
      machinery is excluded when `import.meta.env.DEV` is `false`.

### 5) Flatten navigation/render orchestration layers

- [x] Collapse repetitive `execution plan -> command array -> executor` loops
      where they do not provide unique value.
- [x] Keep pure decision functions where they protect tricky invariants.
- [x] Reduce intermediate object creation in hot paths.
- [x] Preserve existing state-machine contract tests while reducing internal
      indirection.

### 6) Reduce reason-string runtime payload

- [x] Replace submit staleness runtime-generated template reasons with static
      checkpoint reason tables.
- [x] Consolidate submit lifecycle reason literals into shared constants.
- [x] Replace broad runtime string-reason unions with compact constants or
      dev-only mappings.
- [x] Keep debugging detail available in tests/dev builds.
- [x] Ensure runtime behavior does not depend on string literal text.

### 7) Simplify history integration surface

- [x] Extract minimal internal history adapter contract used by runtime.
- [x] Decouple runtime from direct third-party `history` API shape.
- [x] Evaluate replacing dependency usage with a smaller internal adapter where
      feasible.
- [x] Keep POP sequencing and hard-reload fallback behavior fully covered.
- [x] Record capability-parity guardrail: do not remove direct
      `getHistoryInstance()` access (or `npm:history` backing) unless all
      consumer-relevant history capabilities are exposed and covered by tests.

### 8) Shift matcher/route registration work toward build output

- [x] Define precompiled route/matcher payload shape.
- [x] Initialize runtime from precompiled data rather than dynamic progressive
      pattern registration where possible.
- [x] Preserve lazy/progressive enhancement behavior when manifest is absent.
- [x] Add tests for both precompiled and fallback runtime paths.

### 9) Simplify runtime lane bookkeeping hot paths

- [x] Remove lane-operation action-object indirection in `runtime_slots.ts`
      where direct lane mutation is sufficient.
- [x] Remove avoidable per-call allocations in navigation status computation.
- [x] Keep status-signal semantics unchanged under debounced dispatch.
- [x] Add focused unit tests for `computeNavigationStatus` and
      `createStatusSignaler`.

### 10) Flatten submit response orchestration

- [x] Remove submit response `decide action -> execute action` indirection in
      `runtime_submit.ts`.
- [x] Keep staleness checkpoint semantics unchanged across error, redirect, and
      success branches.
- [x] Preserve auto-revalidation and redirect effectuation behavior.
- [x] Validate with submit-focused unit/contract tests and full TS gate.

### 11) Harden deterministic revalidation lane invariants

- [x] Add focused unit tests for in-flight reuse before trailing eligibility.
- [x] Add focused unit tests for single trailing-pass coalescing behavior.
- [x] Add focused unit tests for target-mismatch replacement pass behavior.
- [x] Add focused unit tests for explicit trailing-queue cancellation behavior.
- [x] Keep existing contract-level revalidation tests passing.

### 12) Trim redundant runtime forwarding lambdas

- [x] Remove no-op forwarding closures in `runtime.ts` where function signatures
      already match.
- [x] Keep navigation lifecycle boundaries unchanged.
- [x] Validate with runtime-focused unit tests and full TS gate.

### 13) Harden server-route resolution branch invariants

- [x] Add focused unit tests for `buildRouteDataRequestURL` query parameter
      behavior.
- [x] Add focused unit tests for `resolveServerRouteDataResult` aborted,
      redirect, error, and success branches.
- [x] Keep preload command/state-machine unit suites green.
- [x] Validate with full TS gate.

### 14) Remove begin-navigation command translation indirection

- [x] Execute begin-navigation state-machine plans directly in
      `begin_navigation.ts` without a transient command layer.
- [x] Remove obsolete command-mapping-only unit tests.
- [x] Add focused unit tests for `decideBeginNavigationExecutionPlan` invariants
      across active/prefetch/revalidation flows.
- [x] Keep full runtime navigation suites green.
- [x] Validate with full TS gate.

## Execution Order

1. Dead redirect plumbing + request-body dedupe.
2. CSS responsibility unification.
3. Debug journal gating.
4. Navigation/render layer flattening and reason-string compression.
5. History adapter simplification.
6. Matcher build-output shift.
7. Runtime lane + submit response flattening.
8. Deterministic revalidation lane hardening.
9. Runtime forwarding-lambda trim.
10. Server-route resolution branch hardening.
11. Begin-navigation command translation removal.

## Validation Gate (each step)

- [x] `pnpm prettier --write <edited files>`
- [x] `pnpm tsc -p typescript/vorma/client/tsconfig.json --noEmit`
- [x] Targeted vitest suites for changed modules.
- [x] `make tstest-source`

## Progress Log

- [x] Plan created.
- [x] Step 1 complete.
- [x] Step 2 complete.
- [x] Step 3 complete.
- [x] Step 4 complete.
- [x] Step 5 complete.
- [x] Step 6 complete.
- [x] Step 7 complete.
- [x] Step 8 complete.
- [x] Step 9 complete.
- [x] Step 10 complete.
- [x] Step 11 complete.
- [x] Step 12 complete.
- [x] Step 13 complete.
- [x] Step 14 complete.

# Vorma Test Correctness Audit + Regression Fix Plan (2026-02-25)

This checklist tracks two concurrent goals:

1. Add regression tests for known route-instance token bugs, then fix
   implementation.
2. Audit all `typescript/vorma/*` tests line-by-line for first-principles
   correctness.

## A) Immediate Regression Work

- [x] Add regression tests for bug: routeProps data stays stale when matched
      pattern changes but route key/component identity stay stable.
- [x] Add regression tests for bug: React/Preact create route-instance token
      with render-phase side effects and can leak active records on aborted
      render.
- [x] Run source tests and confirm new tests fail before fix.
- [x] Implement fix for stale routeProps ownership semantics.
- [x] Implement fix for render-phase token side-effects / leaked active-record
      path.
- [x] Update adapter/runtime tests to match corrected first-principles behavior.
- [x] Run `pnpm prettier --write` on touched files.
- [x] Run `make tstest-source`.
- [x] Run `GOCACHE=/tmp/go-build go run ./internal/cmd/buildts`.
- [x] Run `make tstest-dist`.

## B) Full Test Correctness Audit (`typescript/vorma/*`)

Audit rule: every assertion must reflect first-principles/common-sense behavior,
not incidental quirks.

- [x] Build exhaustive checklist of all test files under `typescript/vorma/*`.
- [x] Audit each file line-by-line; record findings and required rewrites.
- [x] Rewrite any test that enshrines quirks/bugs as expected behavior.
- [x] Re-run source+dist TypeScript tests after audit fixes.
- [x] Summarize all corrected test assumptions and residual gaps.

## C) Audit Findings Summary

- Corrected enshrined-bug expectations for route-props loader ownership during
  same-key pattern transitions in dist adapter runtime-state tests.
- Added explicit unit coverage for unmounted route-instance token
  synchronization guard (no sync without commit/activation).
- Reviewed all test files in `typescript/vorma/*` and found no additional
  first-principles violations that required assertion rewrites.

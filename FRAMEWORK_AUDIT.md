# Framework Audit Tracker

## Cycle

- Reset date: 2026-02-14
- Baseline: current repository state at reset time
- Tracking model:
    - `FRAMEWORK_AUDIT.md`: high-signal current state only
    - `FRAMEWORK_AUDIT_EVIDENCE.md`: append-only detailed evidence log

## Objective

Run a from-scratch, repo-wide audit for correctness, resilience, API quality,
maintainability/refactorability, DRY opportunities, performance opportunities,
and test quality.

## Scope

- All repository code and templates, including (without limitation):
    - `vorma.go`
    - `bootstrap/*`
    - `vormabuild/*`
    - `vormaruntime/*`
    - `wave/*`
    - `wave/tooling/*`
    - `vormaclient/*`
    - `kit/*`
    - `lab/*`
    - `internal/*`

## Audit Questions

1. Are there bad/questionable designs in framework internals?
2. Are there user APIs that are confusing or needlessly complex?
3. Does `bootstrap` impose needless boilerplate that framework defaults could
   absorb while preserving flexibility?
4. Are there latent bugs or correctness violations?
5. Are there obvious performance wins with low complexity/risk?
6. Is test coverage missing for behavior that should obviously be protected?
7. Do any tests claim guarantees they do not actually verify?
8. Are there hidden coupling points that increase fragility (cross-package
   assumptions, ordering assumptions, global state)?
9. Are there footguns where API behavior is surprising relative to naming?
10. Are there config defaults that are too presumptive vs. app-level override
    needs?
11. Where is code complex/fragile enough that refactoring or targeted rewrite
    would reduce latent bug risk?
12. Where is logic duplicated (within a package or across the repo) that should
    be abstracted into shared helpers, internal packages, `kit/*` APIs, or
    entirely new `kit/*` packages?
13. Are internal/unstable vs intended-public API boundaries drawn in the right
    places across both Go and TypeScript code?

## Non-Negotiables

- No test weakening.
- No severity labels or triage categories in findings.
- Any performance regression is unacceptable unless it is required for
  correctness or explicitly approved by user. This applies to both dev-time and
  runtime behavior.
- Any non-obvious semantic/design change requires explicit user approval.

## Performance Evaluation Policy

1. Hot paths require benchmark evidence before claiming a perf win/regression.
2. Non-hot paths are evaluated by first-principles logical analysis.
3. Avoid micro-optimization churn outside proven hot paths unless required for
   correctness.

## Pass Focuses (This Cycle)

1. Surface/API boundary pass (Go + TS).
2. Correctness/resilience pass (state, lifecycle, races, stale state).
3. Complexity/fragility pass (rewrite candidates).
4. DRY/abstraction pass (within-package and cross-repo duplication).
5. Performance pass (especially dev-loop behavior, unnecessary work).
6. Test-quality pass (coverage quality, false confidence, missing regressions).
7. Failure-mode pass (error paths and diagnostics quality).

## Completion Rules

1. A package row is only `done` when all required passes are complete.
2. Each pass completion must reference evidence IDs in
   `FRAMEWORK_AUDIT_EVIDENCE.md`.
3. Open findings and open test gaps must remain visible here until closed.
4. Keep this file lean: no historical closed-item narrative.

## Change Authorization Policy

- Automatic fixes are allowed only when the answer is obvious and
  first-principles-correct with no reasonable semantics/design alternative.
- If a change involves design or semantics choices with multiple reasonable
  options, stop and get explicit user approval before changing code.
- Any non-obvious change must be recorded in `Approval Log` with outcome.

## Approval Log

- 2026-02-14: Approved boundary fix for `wave.GetParsedConfig()` and
  `wave/tooling.Builder.Config()` snapshot semantics with explicit unstable
  mutable accessors.

## Matrix

| Package Group          | Surface/API | Correctness | Fragility | DRY      | Performance | Test Quality | Failure Modes |
| ---------------------- | ----------- | ----------- | --------- | -------- | ----------- | ------------ | ------------- |
| `vorma.go`             | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `bootstrap/*`          | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormabuild/*`         | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaruntime/*`       | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `wave/*`               | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `wave/tooling/*`       | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/client/*` | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/react/*`  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/preact/*` | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/solid/*`  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/vite/*`   | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/create/*` | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/*`                | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/*`                | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `internal/*`           | not done    | not done    | not done  | not done | not done    | not done     | not done      |

## Current Focus

1. Start with `wave/*` + `wave/tooling/*` and log concrete evidence IDs.
2. Continue through the remaining matrix groups.

## Open Findings

None currently recorded for this reset cycle.

## Open Test Gaps

None currently recorded for this reset cycle.

## Next Queue

1. `wave/*`: complete all passes with evidence.
2. `wave/tooling/*`: complete all passes with evidence.
3. Continue package-group by package-group until matrix is complete.

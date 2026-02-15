# Framework Audit Tracker

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

## Audit Questions To Always Be Thinking About

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
- Any code change requires a full gate run before completion claims:
  `make gotest`, `make tstest`, `make tscheck`, and `make tslint` (unless user
  explicitly approves a narrower gate).

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

1. Matrix cells are updated independently per pass.
2. A package row is only `done` when all required pass cells are complete.
3. Work one active pass column at a time and sweep vertically across rows.
4. Do not advance a package to a different pass until the active sweep is
   completed across rows (unless explicitly approved by user).
5. Execution unit is one cell (or a small set of cells handled in one turn);
   neither a full row nor a full column is atomic.
6. Each pass completion must reference evidence IDs in
   `FRAMEWORK_AUDIT_EVIDENCE.md`.
7. Open findings and open test gaps must remain visible here until closed.
8. Keep this file lean: no historical closed-item narrative.
9. For each finding discovered in the active queue item, do exactly one: apply
   the fix immediately if it is obvious and first-principles-correct, or stop
   and request explicit user guidance.
10. Do not mark a matrix cell `done` while any finding from that cell remains
    unresolved.
11. Do not advance `Next Queue` past the current item until all findings in that
    item are resolved or explicitly deferred by user decision and recorded in
    `Approval Log`.

## Change Authorization Policy

- Automatic fixes are allowed only when the answer is obvious and
  first-principles-correct with no reasonable semantics/design alternative.
- If a change involves design or semantics choices with multiple reasonable
  options, stop and get explicit user approval before changing code.
- Any non-obvious change must be recorded in `Approval Log` with outcome.

## Approval Log

1. 2026-02-15: User confirmed AST discovery + generated registration is the
   intended architecture and explicitly rejected a runtime side-effect
   registration model for `vorma.NewLoader`/`vorma.NewAction`.

## Matrix

| Package Group                  | Surface/API | Correctness | Fragility | DRY      | Performance | Test Quality | Failure Modes |
| ------------------------------ | ----------- | ----------- | --------- | -------- | ----------- | ------------ | ------------- |
| `vorma.go`                     | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `bootstrap/*`                  | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `vormabuild/*`                 | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaruntime/*`               | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `wave/*`                       | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `wave/tooling/*`               | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/client/*`         | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/react/*`          | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/preact/*`         | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/solid/*`          | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/vite/*`           | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/create/*`         | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/bytesutil`                | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/colorlog`                 | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/contextutil`              | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/cookies`                  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/cryptoutil`               | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/csrf`                     | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/envutil`                  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/executil`                 | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/fsutil`                   | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/genericsutil`             | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/grace`                    | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/headels`                  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/htmlutil`                 | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/id`                       | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/ioutil`                   | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/jsonutil`                 | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/keyset`                   | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/lazyget`                  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/lru`                      | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/matcher`                  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/middleware`               | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/middleware/etag`          | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/middleware/healthcheck`   | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/middleware/robotstxt`     | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/middleware/secureheaders` | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/modulegraph`              | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/mux`                      | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/netutil`                  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/reflectutil`              | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/response`                 | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/securebytes`              | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/securestring`             | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/set`                      | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/tasks`                    | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/theme`                    | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/validate`                 | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/bumper`                   | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/cliutil`                  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/errutil`                  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/esbuildutil`              | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/fsmarkdown`               | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/jsonschema`               | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/mailutil`                 | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/parseutil`                | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/repoconcat`               | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/rpc`                      | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/sqlutil`                  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/stringsutil`              | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/timer`                    | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/tsgen`                    | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/tsgen/tsgencore`          | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/vitecmd`                  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/viteutil`                 | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/xyz`                      | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `internal/scripts/buildts`     | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `internal/scripts/bumper`      | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `internal/scripts/npm_bumper`  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `internal/scripts/sum`         | not done    | not done    | not done  | not done | not done    | not done     | not done      |

Matrix evidence for completed cells:

1. `vorma.go` + Surface/API: `EV-20260214-001`, `EV-20260214-002`.
2. `bootstrap/*` + Surface/API: `EV-20260214-003`.
3. `vormabuild/*` + Surface/API: `EV-20260214-004`.
4. `vormaruntime/*` + Surface/API: `EV-20260214-005`.
5. `wave/*` + Surface/API: `EV-20260214-006`.
6. `wave/tooling/*` + Surface/API: `EV-20260214-007`.

## Current Focus

1. Active pass: Surface/API.
2. Execute a vertical Surface/API sweep cell-by-cell from the top row.

## Open Findings

1. None.

## Open Test Gaps

1. None.

## Next Queue

1. `vormaclient/client/*` + Surface/API.

## Process Notes

You should keep going through as many cells as you can autonomously unless and
until a Change Authorization Requirement is triggered where you need the user's
input.

# Framework Audit Tracker

- Tracking model:
    - `FRAMEWORK_AUDIT.md`: high-signal current state only
    - `FRAMEWORK_AUDIT_EVIDENCE.md`: append-only detailed evidence log

## Objective

Run a from-scratch, repo-wide audit for correctness, resilience, API quality,
maintainability/refactorability, DRY opportunities, performance opportunities,
failure-mode handling, security posture, test quality, and documentation
quality.

## Scope

- All repository code and templates, including (without limitation):
    - `vorma.go`
    - `bootstrap/*`
    - `vormabuild/*`
    - `internal/vormaruntime/*`
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
- Every discovered bug must be closed with a regression test that would fail
  without the fix and pass with the fix. If a regression test is truly not
  possible, explicitly record why in evidence and get user approval before
  closing the finding.
- Any performance regression is unacceptable unless it is required for
  correctness or explicitly approved by user. This applies to both dev-time and
  runtime behavior.
- Any non-obvious semantic/design change requires explicit user approval.
- Any code change requires a full gate run before completion claims:
  `make gotest`, `make tstest`, `make tscheck`, and `make tslint` (unless user
  explicitly approves a narrower gate).

## Public vs Internal API Boundary Standard

1. Public/private API decisions must be made from first principles, not from
   in-repo callsite evidence.
2. Required question for each candidate symbol: "Is there a valid app-developer
   use case for this API, or is this reaching into framework internals?"
3. "Not currently called in this repo" is never sufficient evidence to unexport
   or move an API in a public framework.
4. App-facing APIs belong on stable public package surfaces; unstable internals
   belong on explicitly internal surfaces (for example `__internal`).
5. If an API is app-facing but currently placed on an internal surface, move it
   to the correct public package (or re-export from that package) rather than
   removing it.
6. If intent is genuinely ambiguous after first-principles analysis, stop and
   ask the user before changing surface area.

## Performance Evaluation Policy

1. Hot paths require benchmark evidence before claiming a perf win/regression.
2. Non-hot paths are evaluated by first-principles logical analysis.
3. Avoid micro-optimization churn outside proven hot paths unless required for
   correctness.

## Audit Integrity Guardrails

1. Every pass must use first-principles analysis for each cell; heuristic
   keyword scanning is allowed only as support, never as the conclusion.
2. "No finding" conclusions must include explicit reasoning grounded in code
   behavior. Template-only closures (for example "No issue found in this cell")
   are invalid evidence.
3. It is prohibited to apply an "actionable-only", "regression-only", or
   "likely-bottleneck-only" filter when the pass requires first-principles
   review (especially Performance non-hot-path review).
4. Public/internal API boundary decisions must never use in-repo callsite
   presence/absence as primary evidence.
5. If any policy violation is discovered after a cell was marked `done`, that
   cell (and any other cells impacted by the same failure mode) must be reset to
   `not done` immediately.
6. During such reset, prior completion claims in this tracker become
   non-authoritative until revalidated with fresh evidence.
7. `FRAMEWORK_AUDIT.md` matrix state is authoritative for completion status.
   Historical entries in `FRAMEWORK_AUDIT_EVIDENCE.md` may exist for
   traceability but do not imply valid completion after an integrity reset.

## Pass Focuses

1. Surface/API boundary pass (Go + TS).
2. Correctness/resilience + fragility pass (state, lifecycle, races, stale
   state, complexity/rewrite candidates).
3. DRY/abstraction pass (within-package and cross-repo duplication).
4. Performance pass (especially dev-loop behavior, unnecessary work, and runtime
   hot paths).
5. Failure-mode pass (error paths and diagnostics quality).
6. Security pass (input validation, authn/authz boundaries, SSRF/path traversal,
   injection classes, secret exposure, unsafe defaults).
7. Test-quality pass (coverage quality, false confidence, missing regressions).
8. Docs pass (Godoc/README/package docs completeness, maintainer footguns, and
   AGENTS.md guidance lift-up when needed).

## DRY Pass Execution Protocol

1. Do not rely on memory for cross-repo duplication discovery; maintain an
   explicit candidate index while scanning.
2. For each DRY queue cell, record every non-trivial duplication candidate in
   `FRAMEWORK_AUDIT_EVIDENCE.md` with: package/file anchors, duplicated concern
   summary, and normalization target (shared helper, internal package, `kit/*`,
   or deliberate duplication).
3. Maintain a running "Open DRY Candidates" list in this tracker until each
   candidate is either implemented or explicitly rejected by first-principles
   reasoning.
4. Before marking any DRY cell `done`, perform one repo-wide reconciliation
   sweep using the candidate index to confirm no unresolved cross-package
   candidate was dropped.
5. If a candidate touches API design or introduces ambiguous abstraction
   boundaries, stop and ask the user before refactoring.
6. A DRY extraction must remove duplication across 2+ concrete consumers; moving
   single-consumer logic between packages is not a DRY win. However, if
   something is a truly useful public-helper candidate for kit (and doesn't
   include package-specific or highly specialized logic), then it's OK to move
   it to kit even if there's only one current consumer.

## Security Pass Checklist

1. Trace all untrusted input entry points (HTTP params, headers, body, files,
   env, config, CLI args) to sink operations.
2. Verify authn/authz checks are explicit and consistently enforced on every
   protected action.
3. Check for injection and traversal classes: SQL/command/template injection,
   path traversal, SSRF, and unsafe file operations.
4. Verify secret handling and defaults: no accidental logging/exposure,
   least-privilege defaults, and explicit opt-ins for dangerous behavior.
5. Confirm tests cover negative/abuse cases, not only happy paths.

## Docs Pass Checklist

1. Ensure exported APIs have clear Godoc/TSDoc with semantics, constraints, and
   failure behavior.
2. Ensure package/module READMEs exist where needed and match actual behavior
   and cover 100% of public APIs.
3. Scan for maintainer footguns/surprises and either remove the footgun in code
   or record guidance in `AGENTS.md` (or package-specific `AGENTS.md` when scope
   is local).
4. Do not use docs to excuse bad API shapes; fix names/design first, then
   document the correct behavior.
5. Confirm examples and setup instructions are runnable and current.

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
12. Do not close any bug finding without a linked regression test in
    `FRAMEWORK_AUDIT_EVIDENCE.md` (or an explicit user-approved exception).
13. Do not mark a cell `done` with template-only "no finding" evidence.
14. If a pass was executed with a disallowed filter (for example
    actionability-only or regression-only), reset all impacted cells to
    `not done` before any further audit progress.

## Change Authorization Policy

- Automatic fixes are allowed only when the answer is obvious and
  first-principles-correct with no reasonable semantics/design alternative.
- Public/private boundary changes must satisfy
  `Public vs Internal API Boundary Standard`.
- If a change involves design or semantics choices with multiple reasonable
  options, stop and get explicit user approval before changing code.
- Any non-obvious change must be recorded in `Approval Log` with outcome.

## Approval Log

1. None yet.

## Matrix

| Package Group                           | Surface/API | Correctness/Fragility | DRY      | Performance | Failure Modes | Security | Test Quality | Docs     |
| --------------------------------------- | ----------- | --------------------- | -------- | ----------- | ------------- | -------- | ------------ | -------- |
| `vorma.go`                              | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `bootstrap/*`                           | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `vormabuild/*`                          | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `internal/vormaruntime/*`               | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `wave/*`                                | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `wave/tooling/*`                        | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/vorma/client/*`             | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/vorma/ui-adapters/react/*`  | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/vorma/ui-adapters/preact/*` | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/vorma/ui-adapters/solid/*`  | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/vorma/vite/*`               | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/vorma/create/*`             | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/kit/converters`             | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/kit/cookies`                | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/kit/csrf`                   | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/kit/debounce`               | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/kit/fmt`                    | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/kit/json`                   | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/kit/listeners`              | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/kit/matcher`                | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/kit/theme`                  | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `typescript/kit/url`                    | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/bytesutil`                         | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/colorlog`                          | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/contextutil`                       | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/cookies`                           | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/cryptoutil`                        | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/csrf`                              | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/envutil`                           | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/executil`                          | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/fsutil`                            | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/genericsutil`                      | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/grace`                             | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/headels`                           | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/htmlutil`                          | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/id`                                | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/ioutil`                            | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/jsonutil`                          | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/k9`                                | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/keyset`                            | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/lazyget`                           | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/lru`                               | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/matcher`                           | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/middleware`                        | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/middleware/etag`                   | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/middleware/healthcheck`            | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/middleware/robotstxt`              | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/middleware/secureheaders`          | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/mux`                               | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/netutil`                           | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/reflectutil`                       | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/response`                          | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/securebytes`                       | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/securestring`                      | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/set`                               | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/tasks`                             | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/theme`                             | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/validate`                          | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/bumper`                            | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/cliutil`                           | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/errutil`                           | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/esbuildutil`                       | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/fsmarkdown`                        | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/jsonschema`                        | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/mailutil`                          | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/parseutil`                         | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/repoconcat`                        | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/rpc`                               | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/sqlutil`                           | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/stringsutil`                       | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/timer`                             | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/tsgen`                             | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/tsgen/tsgencore`                   | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/vitecmd`                           | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/viteutil`                          | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/xyz`                               | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `internal/cmd/buildts`                  | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `internal/cmd/bumper`                   | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `internal/cmd/npm_bumper`               | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |
| `internal/cmd/sum`                      | not done    | not done              | not done | not done    | not done      | not done | not done     | not done |

Matrix evidence for completed cells:

1. Integrity reset on 2026-02-15 invalidated all prior completed-cell claims in
   this tracker.
2. `FRAMEWORK_AUDIT_EVIDENCE.md` was reset to a clean slate for this cycle;
   historical EV entries were removed.
3. No cell may be marked `done` again without fresh first-principles evidence
   that follows the guardrails below.

## Current Focus

1. Active pass: Surface/API.
2. Execute a vertical Surface/API sweep cell-by-cell from the top row.

## Open Findings

1. None.

## Open DRY Candidates

1. None.

## Open Test Gaps

1. None.

## Next Queue

1. `vorma.go` + Surface/API.

## Process Notes

You should keep going through as many cells as you can autonomously unless and
until a Change Authorization Requirement is triggered where you need the user's
input.

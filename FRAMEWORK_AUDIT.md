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
- Public/private boundary changes must satisfy
  `Public vs Internal API Boundary Standard`.
- If a change involves design or semantics choices with multiple reasonable
  options, stop and get explicit user approval before changing code.
- Any non-obvious change must be recorded in `Approval Log` with outcome.

## Approval Log

1. 2026-02-15: User confirmed AST discovery + generated registration is the
   intended architecture and explicitly rejected a runtime side-effect
   registration model for `vorma.NewLoader`/`vorma.NewAction`.
2. 2026-02-15: User confirmed custom client-loader abstractions are not a
   supported app-level extension goal; keep
   `runClientLoadersAfterHMRUpdate`/`registerClientLoaderPattern` off the public
   `vorma/client` surface.
3. 2026-02-15: User approved internalizing `modulegraph` away from app-facing
   `kit/*` surface.
4. 2026-02-15: User requested deletion experiment for `modulegraph` and asked
   whether `AppRoot` was vestigial; approved removing that API if full gates
   pass.

## Matrix

| Package Group                  | Surface/API | Correctness | Fragility | DRY      | Performance | Test Quality | Failure Modes |
| ------------------------------ | ----------- | ----------- | --------- | -------- | ----------- | ------------ | ------------- |
| `vorma.go`                     | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `bootstrap/*`                  | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `vormabuild/*`                 | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaruntime/*`               | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `wave/*`                       | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `wave/tooling/*`               | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/client/*`         | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/react/*`          | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/preact/*`         | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/solid/*`          | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/vite/*`           | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `vormaclient/create/*`         | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/bytesutil`                | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/colorlog`                 | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/contextutil`              | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/cookies`                  | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/cryptoutil`               | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/csrf`                     | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/envutil`                  | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/executil`                 | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/fsutil`                   | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/genericsutil`             | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/grace`                    | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/headels`                  | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/htmlutil`                 | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/id`                       | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/ioutil`                   | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/jsonutil`                 | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/keyset`                   | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/lazyget`                  | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/lru`                      | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/matcher`                  | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/middleware`               | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/middleware/etag`          | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/middleware/healthcheck`   | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/middleware/robotstxt`     | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/middleware/secureheaders` | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/mux`                      | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/netutil`                  | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/reflectutil`              | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/response`                 | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/securebytes`              | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/securestring`             | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/set`                      | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/tasks`                    | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/theme`                    | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `kit/validate`                 | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/bumper`                   | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/cliutil`                  | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/errutil`                  | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/esbuildutil`              | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/fsmarkdown`               | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/jsonschema`               | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/mailutil`                 | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/parseutil`                | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/repoconcat`               | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/rpc`                      | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/sqlutil`                  | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/stringsutil`              | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/timer`                    | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/tsgen`                    | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/tsgen/tsgencore`          | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/vitecmd`                  | done        | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/viteutil`                 | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `lab/xyz`                      | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `internal/scripts/buildts`     | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `internal/scripts/bumper`      | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `internal/scripts/npm_bumper`  | not done    | not done    | not done  | not done | not done    | not done     | not done      |
| `internal/scripts/sum`         | not done    | not done    | not done  | not done | not done    | not done     | not done      |

Matrix evidence for completed cells:

1. `vorma.go` + Surface/API: `EV-20260214-001`, `EV-20260214-002`,
   `EV-20260215-036`.
2. `bootstrap/*` + Surface/API: `EV-20260214-003`.
3. `vormabuild/*` + Surface/API: `EV-20260214-004`.
4. `vormaruntime/*` + Surface/API: `EV-20260214-005`, `EV-20260215-036`.
5. `wave/*` + Surface/API: `EV-20260214-006`.
6. `wave/tooling/*` + Surface/API: `EV-20260214-007`.
7. `vormaclient/client/*` + Surface/API: `EV-20260215-001`.
8. `vormaclient/react/*` + Surface/API: `EV-20260215-002`.
9. `vormaclient/preact/*` + Surface/API: `EV-20260215-003`.
10. `vormaclient/solid/*` + Surface/API: `EV-20260215-004`.
11. `vormaclient/vite/*` + Surface/API: `EV-20260215-005`.
12. `vormaclient/create/*` + Surface/API: `EV-20260215-006`.
13. `kit/bytesutil` + Surface/API: `EV-20260215-007`.
14. `kit/colorlog` + Surface/API: `EV-20260215-008`.
15. `kit/contextutil` + Surface/API: `EV-20260215-009`.
16. `kit/cookies` + Surface/API: `EV-20260215-010`.
17. `kit/cryptoutil` + Surface/API: `EV-20260215-012`.
18. `vormaclient/client/*` + Surface/API boundary confirmation:
    `EV-20260215-013`.
19. `kit/csrf` + Surface/API: `EV-20260215-014`.
20. `kit/envutil` + Surface/API: `EV-20260215-015`.
21. `kit/executil` + Surface/API: `EV-20260215-016`.
22. `kit/fsutil` + Surface/API: `EV-20260215-017`.
23. `kit/genericsutil` + Surface/API: `EV-20260215-018`.
24. `kit/grace` + Surface/API: `EV-20260215-019`.
25. `kit/headels` + Surface/API: `EV-20260215-020`.
26. `kit/htmlutil` + Surface/API: `EV-20260215-021`.
27. `kit/id` + Surface/API: `EV-20260215-022`.
28. `kit/ioutil` + Surface/API: `EV-20260215-023`.
29. `kit/jsonutil` + Surface/API: `EV-20260215-024`.
30. `kit/keyset` + Surface/API: `EV-20260215-025`.
31. `kit/lazyget` + Surface/API: `EV-20260215-026`.
32. `kit/lru` + Surface/API: `EV-20260215-027`.
33. `kit/matcher` + Surface/API: `EV-20260215-028`.
34. `kit/middleware` + Surface/API: `EV-20260215-029`.
35. `kit/middleware/etag` + Surface/API: `EV-20260215-030`.
36. `kit/middleware/healthcheck` + Surface/API: `EV-20260215-031`.
37. `kit/middleware/robotstxt` + Surface/API: `EV-20260215-032`.
38. `kit/middleware/secureheaders` + Surface/API: `EV-20260215-033`.
39. `kit/mux` + Surface/API: `EV-20260215-035`.
40. `kit/netutil` + Surface/API: `EV-20260215-037`.
41. `kit/reflectutil` + Surface/API: `EV-20260215-038`.
42. `kit/response` + Surface/API: `EV-20260215-039`.
43. `kit/securebytes` + Surface/API: `EV-20260215-040`.
44. `kit/securestring` + Surface/API: `EV-20260215-041`.
45. `kit/set` + Surface/API: `EV-20260215-042`.
46. `kit/tasks` + Surface/API: `EV-20260215-043`.
47. `kit/theme` + Surface/API: `EV-20260215-044`.
48. `kit/validate` + Surface/API: `EV-20260215-045`.
49. `lab/bumper` + Surface/API: `EV-20260215-046`.
50. `lab/cliutil` + Surface/API: `EV-20260215-047`.
51. `lab/errutil` + Surface/API: `EV-20260215-048`.
52. `lab/esbuildutil` + Surface/API: `EV-20260215-049`.
53. `lab/fsmarkdown` + Surface/API: `EV-20260215-050`.
54. `lab/jsonschema` + Surface/API: `EV-20260215-051`.
55. `lab/mailutil` + Surface/API: `EV-20260215-052`.
56. `lab/parseutil` + Surface/API: `EV-20260215-053`.
57. `lab/repoconcat` + Surface/API: `EV-20260215-054`.
58. `lab/rpc` + Surface/API: `EV-20260215-055`.
59. `lab/sqlutil` + Surface/API: `EV-20260215-056`.
60. `lab/stringsutil` + Surface/API: `EV-20260215-057`.
61. `lab/timer` + Surface/API: `EV-20260215-058`.
62. `lab/tsgen` + Surface/API: `EV-20260215-059`.
63. `lab/tsgen/tsgencore` + Surface/API: `EV-20260215-060`.
64. `lab/vitecmd` + Surface/API: `EV-20260215-061`.

## Current Focus

1. Active pass: Surface/API.
2. Execute a vertical Surface/API sweep cell-by-cell from the top row.

## Open Findings

1. None.

## Open Test Gaps

1. None.

## Next Queue

1. `lab/viteutil` + Surface/API.

## Process Notes

You should keep going through as many cells as you can autonomously unless and
until a Change Authorization Requirement is triggered where you need the user's
input.

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
2. Ensure package/module READMEs exist where needed and match actual behavior.
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
5. 2026-02-15: User approved running correctness + fragility as one combined
   pass while keeping DRY as a separate pass.
6. 2026-02-15: User approved running Failure Modes before Test Quality while
   keeping them as separate passes.
7. 2026-02-15: User required preserving the runtime/buildtime split for binary
   size, treating `vorma` as app-facing only, and keeping `wave` public for
   framework authors while hiding true internals.
8. 2026-02-15: User added an explicit Security pass after Failure Modes.
9. 2026-02-15: User added a Docs pass after Test Quality, including maintainers'
   footgun scanning and AGENTS guidance lift-up.

## Matrix

| Package Group                  | Surface/API | Correctness/Fragility | DRY      | Performance | Failure Modes | Security | Test Quality | Docs     |
| ------------------------------ | ----------- | --------------------- | -------- | ----------- | ------------- | -------- | ------------ | -------- |
| `vorma.go`                     | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `bootstrap/*`                  | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `vormabuild/*`                 | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `internal/vormaruntime/*`      | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `wave/*`                       | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `wave/tooling/*`               | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `vormaclient/client/*`         | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `vormaclient/react/*`          | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `vormaclient/preact/*`         | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `vormaclient/solid/*`          | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `vormaclient/vite/*`           | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `vormaclient/create/*`         | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/bytesutil`                | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/colorlog`                 | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/contextutil`              | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/cookies`                  | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/cryptoutil`               | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/csrf`                     | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/envutil`                  | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/executil`                 | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/fsutil`                   | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/genericsutil`             | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/grace`                    | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/headels`                  | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/htmlutil`                 | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/id`                       | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/ioutil`                   | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/jsonutil`                 | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/k9`                       | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/keyset`                   | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/lazyget`                  | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/lru`                      | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/matcher`                  | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/middleware`               | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/middleware/etag`          | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/middleware/healthcheck`   | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/middleware/robotstxt`     | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/middleware/secureheaders` | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/mux`                      | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/netutil`                  | done        | done                  | not done | not done    | not done      | not done | not done     | not done |
| `kit/reflectutil`              | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/response`                 | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/securebytes`              | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/securestring`             | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/set`                      | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/tasks`                    | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/theme`                    | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `kit/validate`                 | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/bumper`                   | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/cliutil`                  | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/errutil`                  | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/esbuildutil`              | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/fsmarkdown`               | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/jsonschema`               | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/mailutil`                 | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/parseutil`                | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/repoconcat`               | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/rpc`                      | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/sqlutil`                  | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/stringsutil`              | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/timer`                    | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/tsgen`                    | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/tsgen/tsgencore`          | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/vitecmd`                  | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/viteutil`                 | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `lab/xyz`                      | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `internal/scripts/buildts`     | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `internal/scripts/bumper`      | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `internal/scripts/npm_bumper`  | done        | not done              | not done | not done    | not done      | not done | not done     | not done |
| `internal/scripts/sum`         | done        | not done              | not done | not done    | not done      | not done | not done     | not done |

Matrix evidence for completed cells:

1. `vorma.go` + Surface/API: `EV-20260214-001`, `EV-20260214-002`,
   `EV-20260215-036`, `EV-20260215-073`.
2. `bootstrap/*` + Surface/API: `EV-20260214-003`.
3. `vormabuild/*` + Surface/API: `EV-20260214-004`, `EV-20260215-073`.
4. `internal/vormaruntime/*` + Surface/API: `EV-20260214-005`,
   `EV-20260215-036`, `EV-20260215-073`.
5. `wave/*` + Surface/API: `EV-20260214-006`, `EV-20260215-073`.
6. `wave/tooling/*` + Surface/API: `EV-20260214-007`, `EV-20260215-073`.
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
65. `lab/viteutil` + Surface/API: `EV-20260215-062`.
66. `lab/xyz` + Surface/API: `EV-20260215-063`.
67. `internal/scripts/buildts` + Surface/API: `EV-20260215-064`.
68. `internal/scripts/bumper` + Surface/API: `EV-20260215-065`.
69. `internal/scripts/npm_bumper` + Surface/API: `EV-20260215-066`.
70. `internal/scripts/sum` + Surface/API: `EV-20260215-067`.
71. `kit/k9` + Surface/API: `EV-20260215-109`.
72. `vorma.go` + Correctness/Fragility: `EV-20260215-068`.
73. `bootstrap/*` + Correctness/Fragility: `EV-20260215-069`.
74. `vormabuild/*` + Correctness/Fragility: `EV-20260215-070`.
75. `internal/vormaruntime/*` + Correctness/Fragility: `EV-20260215-071`.
76. `wave/*` + Correctness/Fragility: `EV-20260215-072`.
77. `wave/tooling/*` + Correctness/Fragility: `EV-20260215-074`.
78. `vormaclient/client/*` + Correctness/Fragility: `EV-20260215-075`.
79. `vormaclient/react/*` + Correctness/Fragility: `EV-20260215-076`.
80. `vormaclient/preact/*` + Correctness/Fragility: `EV-20260215-077`.
81. `vormaclient/solid/*` + Correctness/Fragility: `EV-20260215-078`.
82. `vormaclient/vite/*` + Correctness/Fragility: `EV-20260215-079`.
83. `vormaclient/create/*` + Correctness/Fragility: `EV-20260215-080`.
84. `kit/bytesutil` + Correctness/Fragility: `EV-20260215-081`.
85. `kit/colorlog` + Correctness/Fragility: `EV-20260215-082`.
86. `kit/contextutil` + Correctness/Fragility: `EV-20260215-084`.
87. `kit/cookies` + Correctness/Fragility: `EV-20260215-085`.
88. `kit/cryptoutil` + Correctness/Fragility: `EV-20260215-086`.
89. `kit/csrf` + Correctness/Fragility: `EV-20260215-087`.
90. `kit/envutil` + Correctness/Fragility: `EV-20260215-088`.
91. `kit/executil` + Correctness/Fragility: `EV-20260215-089`.
92. `kit/fsutil` + Correctness/Fragility: `EV-20260215-090`.
93. `kit/genericsutil` + Correctness/Fragility: `EV-20260215-091`.
94. `kit/grace` + Correctness/Fragility: `EV-20260215-092`.
95. `kit/headels` + Correctness/Fragility: `EV-20260215-093`.
96. `kit/htmlutil` + Correctness/Fragility: `EV-20260215-094`.
97. `kit/id` + Correctness/Fragility: `EV-20260215-095`.
98. `kit/ioutil` + Correctness/Fragility: `EV-20260215-096`.
99. `kit/jsonutil` + Correctness/Fragility: `EV-20260215-097`.
100. `kit/k9` + Correctness/Fragility: `EV-20260215-110`.
101. `kit/keyset` + Correctness/Fragility: `EV-20260215-098`.
102. `kit/lazyget` + Correctness/Fragility: `EV-20260215-099`.
103. `kit/lru` + Correctness/Fragility: `EV-20260215-100`.
104. `kit/matcher` + Correctness/Fragility: `EV-20260215-101`.
105. `kit/middleware` + Correctness/Fragility: `EV-20260215-102`.
106. `kit/middleware/etag` + Correctness/Fragility: `EV-20260215-103`.
107. `kit/middleware/healthcheck` + Correctness/Fragility: `EV-20260215-104`.
108. `kit/middleware/robotstxt` + Correctness/Fragility: `EV-20260215-105`.
109. `kit/middleware/secureheaders` + Correctness/Fragility: `EV-20260215-106`.
110. `kit/mux` + Correctness/Fragility: `EV-20260215-107`.
111. `kit/netutil` + Correctness/Fragility: `EV-20260215-108`.

## Current Focus

1. Active pass: Correctness/Fragility.
2. Execute a vertical Correctness/Fragility sweep cell-by-cell from the top row.

## Open Findings

1. None.

## Open DRY Candidates

1. None.

## Open Test Gaps

1. None.

## Next Queue

1. `kit/reflectutil` + Correctness/Fragility.

## Process Notes

You should keep going through as many cells as you can autonomously unless and
until a Change Authorization Requirement is triggered where you need the user's
input.

# Framework Audit Tracker

## Objective

Run a from-scratch audit of the Vorma + Wave frameworks for correctness,
resilience, API quality, maintainability/refactorability, DRY abstraction
opportunities, performance opportunities, and test quality.

## Scope

- `vorma.go` (root package surface)
- `vormabuild/*`
- `vormaruntime/*`
- `wave/*`
- `wave/tooling/*`
- `bootstrap/*` (including generated defaults/templates)
- `vormaclient/client/*`
- `vormaclient/react/*`
- `vormaclient/preact/*`
- `vormaclient/solid/*`
- `vormaclient/vite/*`
- `vormaclient/create/*`

## Constraints

- Findings are reported as an exhaustive plain list.
- No severity labels or triage categories.
- Tests must not be weakened.
- For this audit, correctness and resilience take precedence over test-suite
  runtime.

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

## Pass Plan

This audit runs as package-by-package multi-pass review with strict completion
criteria.

### Core Pass Types (Tracked in Matrix)

1. Surface/API pass: exported APIs, naming clarity, ergonomics, consistency.
2. Correctness/resilience pass: state transitions, lifecycle behavior, stale
   state, race potential, error propagation.
3. Complexity/fragility pass: identify brittle code paths and refactor/rewrite
   opportunities that reduce latent bug risk.
4. DRY/abstraction pass: identify duplicated logic within a package and across
   the repo; propose or implement shared helpers/internal abstractions/kit APIs.
5. Performance pass: avoidable work in hot paths, unnecessary process spawn/IO,
   serialization points, sequencing/parallelism opportunities.
6. Test-quality pass: missing scenarios, weak assertions, false-confidence
   tests.

### Scoped Focus Passes (Applied Where Relevant)

1. Bootstrap defaults/templates pass: generated defaults, boilerplate
   absorbability, user-flexibility tradeoffs (`bootstrap/*`).
2. TypeScript client runtime pass: navigation/runtime correctness, adapter API
   clarity, client-boundary behavior (`vormaclient/*`).

### Pass Completion Rules

1. A package row in `Full-Pass Matrix` flips to `done` only when all six core
   pass columns are `done` for that package.
2. If any pass is incomplete, status remains `not done`.
3. Each concrete issue found must be recorded in `Findings Log`.
4. If a fix is obvious and first-principles-correct, implement with tests.
5. If a fix has semantics/design tradeoffs, stop and request explicit approval,
   then record outcome in `Approval Log`.
6. For packages with scoped focus passes, completion of relevant scoped passes
   must be reflected in `Active Package Notes` before the package is considered
   complete.

## Change Authorization Policy

- Automatic fixes are allowed only when the answer is obvious and
  first-principles-correct with no reasonable semantics/design alternative.
- If a change involves design or semantics choices with multiple reasonable
  options, stop and get explicit user approval before changing code.
- Any non-obvious change must be recorded in `Approval Log` with outcome.

## Approval Log

| Date       | Change                                                                                                  | Bucket                         | Approval Status  |
| ---------- | ------------------------------------------------------------------------------------------------------- | ------------------------------ | ---------------- |
| 2026-02-14 | `vormaclient/vite/vite.ts` object-input merge preserves user entries with collision-free internal keys. | design/semantics (non-obvious) | approved by user |

## Handoff Ledger

### Full-Pass Matrix (Handoff-Safe)

Status values are strict: `done` means complete package-wide pass, `not done`
means the pass is still pending.

| Package                | Surface/API pass | Correctness pass | Complexity/fragility pass | DRY/abstraction pass | Performance pass | Test-quality pass |
| ---------------------- | ---------------- | ---------------- | ------------------------- | -------------------- | ---------------- | ----------------- |
| `vorma.go`             | not done         | not done         | not done                  | not done             | not done         | not done          |
| `vormabuild/*`         | not done         | not done         | not done                  | not done             | not done         | not done          |
| `vormaruntime/*`       | not done         | not done         | not done                  | not done             | not done         | not done          |
| `wave/*`               | not done         | not done         | not done                  | not done             | not done         | not done          |
| `wave/tooling/*`       | not done         | not done         | not done                  | not done             | not done         | not done          |
| `bootstrap/*`          | not done         | not done         | not done                  | not done             | not done         | not done          |
| `vormaclient/client/*` | not done         | not done         | not done                  | not done             | not done         | not done          |
| `vormaclient/react/*`  | done             | done             | not done                  | not done             | done             | done              |
| `vormaclient/preact/*` | done             | done             | not done                  | not done             | done             | done              |
| `vormaclient/solid/*`  | done             | done             | not done                  | not done             | done             | done              |
| `vormaclient/vite/*`   | done             | done             | not done                  | not done             | done             | done              |
| `vormaclient/create/*` | done             | done             | not done                  | not done             | done             | done              |

### Active Package Notes

- `vormaclient/client/*`: adapter-linked link behavior, `getRootEl()` runtime
  guard behavior, and scroll/sessionStorage failure handling were audited and
  fixed; full package pass is still pending.

### Current Pass Focus

1. Complete remaining package-wide passes for `vormaclient/client/*`, including
   complexity/fragility and DRY/abstraction passes.
2. Backfill complexity/fragility and DRY/abstraction passes for
   `vormaclient/react/*`, `vormaclient/preact/*`, `vormaclient/solid/*`,
   `vormaclient/vite/*`, and `vormaclient/create/*`.
3. Then execute full package-wide passes for `vorma.go`, `vormabuild/*`,
   `vormaruntime/*`, `wave/*`, `wave/tooling/*`, and `bootstrap/*`.

### Handoff Protocol

1. Do not assume anything outside this file; this document is the source of
   truth for open scope, status, and next actions.
2. Start from `Next Queue` item `1` and work in order unless user explicitly
   reprioritizes.
3. Keep `Full-Pass Matrix`, `Findings Log`, `Test Gap Notes`, and `Next Queue`
   synchronized on every meaningful change.
4. Keep only active/open items plus significant decision records required for
   future context.

## Findings Log

1. `wave.GetParsedConfig()` exposes mutable internal framework config by
   pointer.
    - File: `wave/runtime_framework.go:108`
    - Status: open
    - Problem: external callers can mutate runtime internals after parse.

## Test Gap Notes

1. No contract test currently enforces immutability expectations for
   `wave.GetParsedConfig()` consumers.

## Open Assumptions

1. `internal/site` watcher temp-file event noise is treated as app/tooling
   interaction unless reproduced as framework-default behavior.
2. `wave.GetParsedConfig()` mutability remains open pending explicit semantics
   approval for any non-obvious fix.

## Next Queue

1. Complete full core passes for `vormaclient/client/*`.
2. Backfill new complexity/fragility and DRY/abstraction passes for
   `vormaclient/react/*`, `vormaclient/preact/*`, `vormaclient/solid/*`,
   `vormaclient/vite/*`, and `vormaclient/create/*`.
3. Complete full core passes for `vorma.go`, `vormabuild/*`, `vormaruntime/*`,
   `wave/*`, `wave/tooling/*`, and `bootstrap/*`.
4. Resolve open finding `1` (`wave.GetParsedConfig()` mutability) with explicit
   approval if a semantics/design change is required.
5. Move package-by-package through remaining full-pass matrix and keep this
   tracker synchronized with explicit `done`/`not done` states.

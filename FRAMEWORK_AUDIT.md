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
13. Are internal/unstable vs intended-public API boundaries drawn in the right
    places across both Go and TypeScript code?

## Pass Plan

This audit runs as package-by-package multi-pass review with strict completion
criteria.

### Core Pass Types (Tracked in Matrix)

1. Surface/API pass: exported APIs, naming clarity, ergonomics, consistency, and
   correct internal/unstable vs intended-public boundary placement.
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

| Date       | Change                                                                                                                                              | Bucket                         | Approval Status  |
| ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------ | ---------------- |
| 2026-02-14 | `vormaclient/vite/vite.ts` object-input merge preserves user entries with collision-free internal keys.                                             | design/semantics (non-obvious) | approved by user |
| 2026-02-14 | `wave.GetParsedConfig()` and `wave/tooling.Builder.Config()` return defensive copies; mutable access moved to explicit unstable internal accessors. | design/semantics (non-obvious) | approved by user |

## Handoff Ledger

### Full-Pass Matrix (Handoff-Safe)

Status values are strict: `done` means complete package-wide pass, `not done`
means the pass is still pending.

| Package                | Surface/API pass | Correctness pass | Complexity/fragility pass | DRY/abstraction pass | Performance pass | Test-quality pass |
| ---------------------- | ---------------- | ---------------- | ------------------------- | -------------------- | ---------------- | ----------------- |
| `vorma.go`             | done             | done             | done                      | done                 | done             | done              |
| `vormabuild/*`         | done             | done             | done                      | done                 | done             | done              |
| `vormaruntime/*`       | done             | done             | done                      | done                 | done             | done              |
| `wave/*`               | done             | done             | done                      | done                 | done             | done              |
| `wave/tooling/*`       | done             | done             | done                      | done                 | done             | done              |
| `bootstrap/*`          | done             | done             | done                      | done                 | done             | done              |
| `vormaclient/client/*` | done             | done             | done                      | done                 | done             | done              |
| `vormaclient/react/*`  | done             | done             | done                      | done                 | done             | done              |
| `vormaclient/preact/*` | done             | done             | done                      | done                 | done             | done              |
| `vormaclient/solid/*`  | done             | done             | done                      | done                 | done             | done              |
| `vormaclient/vite/*`   | done             | done             | done                      | done                 | done             | done              |
| `vormaclient/create/*` | done             | done             | done                      | done                 | done             | done              |

### Active Package Notes

- `vormaclient/client/*`: adapter-linked link behavior, `getRootEl()` runtime
  guard behavior, scroll/sessionStorage failure handling, and unstable/public
  API boundary cleanup were audited and fixed.
- `vormaclient/*`: duplicated typed-link href construction across React/Preact/
  Solid adapters was consolidated into shared internal helper
  (`resolveTypedLinkHref`) with updated dist regression coverage to prevent
  drift.
- `vormaclient/client/*`: duplicated eligible-link target classification logic
  in click/prefetch flows was consolidated in `core/links.ts` to reduce drift
  risk between navigation modes.
- `vormaclient/client/*`: duplicated `VormaRoutePropsGeneric` type shape
  definitions were consolidated to the `app/helpers.ts` source-of-truth export
  to prevent type drift.
- `vormaclient/react/*`, `vormaclient/preact/*`, `vormaclient/solid/*`,
  `vormaclient/vite/*`, `vormaclient/create/*`: Surface/API boundary backfill
  completed. No unstable/internal API leaks found beyond intentionally unstable
  `vorma/client/__internal` usage and CLI-internal helper files.
- `vormaclient/create/*`: Go version parse/validation was hardened via shared
  helpers to prevent silent parse failure and empty bootstrap GoVersion.
- `vormaclient/client/*`: unstable `__*` exports were removed from public
  `vorma/client` and kept on `vorma/client/__internal` to enforce clearer API
  boundaries.
- `vorma.go`: full pass complete. Added regression coverage for action runtime
  semantics (`NewAction` remains registration no-op; discovered-action helper
  performs registration) alongside existing loader checks.
- `vormabuild/*`: full pass complete. Fixed recursive generated-artifact cleanup
  to skip directories (prevents accidental removal of user-owned directories
  whose names share generated prefixes) and added regression coverage.
- `vormabuild/*`: reduced drift risk by deduplicating route-registration root
  traversal into one shared helper used by both loader-pattern discovery and
  discovered-registrar call discovery.
- `vormaruntime/*`: full pass complete. Route-data cache invalidation is now
  scoped to the current app identity, so one app reload no longer evicts other
  apps' cache entries. Added regression coverage.
- `vormaruntime/*`: fixed mutable API exposure for TypeScript ad-hoc types by
  cloning on input (`NewVormaApp`) and getter output (`GetAdHocTypes`), with
  regression coverage for caller/getter mutation isolation.
- `wave/*`: fixed framework runtime-state copying to preserve
  function-based/runtime-only framework fields (`FrameworkRunBuildHook`,
  `FrameworkPrepareGoBuildOverlay`) and schema extensions across config reload,
  with regression coverage.
- `wave/*`: hardened runtime API boundary behavior by deep-cloning
  `AddFrameworkWatchPatterns` inputs (prevents caller aliasing from mutating
  framework watch config) and returning defensive copies from `GetPublicFileMap`
  (prevents caller mutation of cached runtime file-map state).
- `wave/tooling/*`: watcher intake now exits cleanly on closed watcher error
  channels instead of continuing through nil-error events during shutdown.
- `wave/*`, `wave/tooling/*`: approval-gated config-boundary fix completed.
  `GetParsedConfig` and `Builder.Config` now return defensive copies with
  regression coverage; internal mutable access is explicit via unstable
  `Internal__*` accessors.
- `wave/*`: simplified cloned-snapshot boundary design to avoid brittle generic
  deep-clone machinery. Public snapshots now intentionally omit unstable
  internal callback/schema fields; internal build/dev paths use explicit
  mutable-access APIs.

### Current Pass Focus

1. No open pass focus items; full scoped matrix is complete.

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

No open findings.

## Test Gap Notes

No open test gaps.

## Open Assumptions

1. `internal/site` watcher temp-file event noise is treated as app/tooling
   interaction unless reproduced as framework-default behavior.

## Next Queue

1. Wait for new audit scope or newly reported regressions.

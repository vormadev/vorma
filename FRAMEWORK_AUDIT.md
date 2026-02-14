# Framework Audit Tracker

## Objective

Run a from-scratch audit of the Vorma + Wave frameworks for correctness,
resilience, API quality, performance opportunities, and test quality.

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

| Package                | Surface/API pass | Correctness pass | Performance pass | Test-quality pass |
| ---------------------- | ---------------- | ---------------- | ---------------- | ----------------- |
| `vorma.go`             | not done         | not done         | not done         | not done          |
| `vormabuild/*`         | not done         | not done         | not done         | not done          |
| `vormaruntime/*`       | not done         | not done         | not done         | not done          |
| `wave/*`               | not done         | not done         | not done         | not done          |
| `wave/tooling/*`       | not done         | not done         | not done         | not done          |
| `bootstrap/*`          | not done         | not done         | not done         | not done          |
| `vormaclient/client/*` | not done         | not done         | not done         | not done          |
| `vormaclient/react/*`  | done             | done             | done             | done              |
| `vormaclient/preact/*` | done             | done             | done             | done              |
| `vormaclient/solid/*`  | done             | done             | done             | done              |
| `vormaclient/vite/*`   | done             | done             | done             | done              |
| `vormaclient/create/*` | done             | done             | done             | done              |

### Active Package Notes

- `vormaclient/client/*`: adapter-linked link behavior, `getRootEl()` runtime
  guard behavior, and scroll/sessionStorage failure handling were audited and
  fixed; full package pass is still pending.

## Findings Log

1. `wave.GetParsedConfig()` exposes mutable internal framework config by
   pointer.
    - File: `wave/runtime_framework.go:108`
    - Status: open
    - Problem: external callers can mutate runtime internals after parse.

## Test Gap Notes

1. No contract test currently enforces immutability expectations for
   `wave.GetParsedConfig()` consumers.

## Next Queue

1. Complete full passes for `vormaclient/client/*`.
2. Complete full passes for `vorma.go`, `vormabuild/*`, `vormaruntime/*`,
   `wave/*`, `wave/tooling/*`, and `bootstrap/*`.
3. Resolve open finding `1` (`wave.GetParsedConfig()` mutability) with explicit
   approval if a semantics/design change is required.
4. Move package-by-package through remaining full-pass matrix and keep this
   tracker synchronized with explicit `done`/`not done` states.

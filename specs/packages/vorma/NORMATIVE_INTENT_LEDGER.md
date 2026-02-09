# vorma Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2` (wrapper canonical reset)

## Mining Rules

- Intent mining MUST use implementation source plus legacy tests outside `conformance/**`.
- `conformance/**` suites are verification outputs/evidence and MUST NOT be mining inputs.
- Per-file counters are epoch-scoped.
- `passes`: total number of full mining passes for that file in current epoch.
- `clean_passes`: number of passes with no newly surfaced normative gap for that file in current epoch.

## Per-File Replay Ledger

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Notes |
|---|---|---|---:|---:|---|---|
| `vorma.go` | in-scope | source+legacy-tests | 0 | 0 | pending | Wrapper source contracts (`VORMA-API-001..004`). |
| `package.json` | in-scope | source+legacy-tests | 0 | 0 | pending | Embedded npm version contract (`VORMA-API-005`). |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | `TBD` | Wrapper ledger reset after removal of redundant `VORMA_*` catalog layer. |

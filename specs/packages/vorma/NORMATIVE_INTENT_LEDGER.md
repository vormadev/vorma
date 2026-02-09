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
| `vorma.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | Wrapper source contracts (`VORMA-API-001..004`) revalidated; dedicated wrapper suites remain missing. |
| `package.json` | in-scope | source+legacy-tests | 1 | 0 | in_progress | Embedded npm version contract (`VORMA-API-005`) revalidated; dedicated wrapper suite remains missing. |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | 1 | Wrapper replay surfaced executable-coverage gap for `VORMA-API-*`; tracked as `VORMA-ISSUE-001`. |

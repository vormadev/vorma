# kit/headels Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Mining Rules

- Intent mining MUST use implementation source plus legacy tests outside `conformance/**`.
- Per-file counters are epoch-scoped.
- `passes`: total pass count in current epoch.
- `clean_passes`: no-new-gap pass count in current epoch.

## Per-File Replay Ledger

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Notes |
|---|---|---|---:|---:|---|---|
| `kit/headels/README.md` | out-of-scope | n/a | 0 | 0 | pending | Documentation-only; not normative source for this replay. |
| `kit/headels/headblocks.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | Requirement catalog mined (`KIT-HEADELS-001..016`); surfaced `KIT-HEADELS-ISSUE-001..004`. |
| `kit/headels/headblocks_test.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | Legacy evidence reconciled to requirement/scenario rows and issue-backed partials. |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | 4 | Placeholder package replaced; active gaps tracked as `KIT-HEADELS-ISSUE-001..004`. |

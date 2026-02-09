# lab/bumper Normative Intent Ledger

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
| `lab/bumper/bumper.go` | in-scope | source+tests | 0 | 0 | pending |  |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | `TBD` | Package-path reset baseline initialized. |

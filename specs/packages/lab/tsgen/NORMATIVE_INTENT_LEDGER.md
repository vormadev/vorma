# lab/tsgen Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Mining Rules

- Intent mining MUST use both implementation source and existing tests.
- Per-file counters are epoch-scoped.
- `passes`: total pass count in current epoch.
- `clean_passes`: no-new-gap pass count in current epoch.

## Per-File Replay Ledger

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Notes |
|---|---|---|---:|---:|---|---|
| `lab/tsgen/generate_ts_content.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `lab/tsgen/generate_ts_content_test.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `lab/tsgen/statements.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `lab/tsgen/to_file.go` | in-scope | source+tests | 0 | 0 | pending |  |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | `TBD` | Package-path reset baseline initialized. |

# kit/validate Normative Intent Ledger

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
| `kit/validate/README.md` | out-of-scope | n/a | 0 | 0 | pending | Documentation/benchmark artifact. |
| `kit/validate/error_acc_test.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `kit/validate/error_collector.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `kit/validate/error_collector_test.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `kit/validate/more_error_collector_test.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `kit/validate/rules.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `kit/validate/rules_test.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `kit/validate/search_params.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `kit/validate/search_params_test.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `kit/validate/validate.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `kit/validate/validate_test.go` | in-scope | source+tests | 0 | 0 | pending |  |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | `TBD` | Package-path reset baseline initialized. |

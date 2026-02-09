# kit/mux Normative Intent Ledger

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
| `kit/mux/README.md` | out-of-scope | n/a | 0 | 0 | out-of-scope | Documentation/usage reference; not normative input for this replay pass. |
| `kit/mux/bench.txt` | out-of-scope | n/a | 0 | 0 | out-of-scope | Benchmark artifact; not normative input for this replay pass. |
| `kit/mux/mux.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; router dispatch/middleware/context contracts mapped to `KIT-MUX-001..041` with open issue-backed exceptions. |
| `kit/mux/mux_test.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; baseline routing, middleware, mount-root, validation, and tasks-context scenarios mapped to requirement-level rows. |
| `kit/mux/mux_advanced_test.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; advanced middleware interactions, error mapping, task-context requiring handlers, and edge routing scenarios mapped to requirement-level rows. |
| `kit/mux/nested_mux.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; nested execution internals, rebuild APIs, and pooling assumptions mapped to `KIT-MUX-042..048` with open issue-backed exceptions. |
| `kit/mux/nested_mux_test.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; nested registration/matching/results behavior mapped to requirement-level rows. |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | completed | 3 | Full-pass replay and requirement-level reconciliation completed with explicit issue-backed exceptions (`KIT-MUX-ISSUE-001..003`). |
| `E2-R2` | pending | `TBD` | No-gap clean pass is blocked until open issues are resolved or explicitly accepted. |

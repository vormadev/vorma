# kit/mux Normative Intent Ledger

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
| `kit/mux/README.md` | out-of-scope | n/a | 0 | 0 | out-of-scope | Documentation/usage reference; not normative input for this replay pass. |
| `kit/mux/bench.txt` | out-of-scope | n/a | 0 | 0 | out-of-scope | Benchmark artifact; not normative input for this replay pass. |
| `kit/mux/mux.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R2` revalidated router behavior and extended exported-surface requirements through `KIT-MUX-053` with no new issue IDs. |
| `kit/mux/mux_test.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R2` revalidated baseline routing/middleware/validation coverage mappings. |
| `kit/mux/mux_advanced_test.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R2` revalidated advanced middleware/error/TasksCtx behavior mappings. |
| `kit/mux/nested_mux.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R2` revalidated nested execution/rebuild contracts and added missing query/result accessor requirements. |
| `kit/mux/nested_mux_test.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R2` revalidated nested registration/match/result evidence against updated requirement set. |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | completed | 3 | Full-pass replay and requirement-level reconciliation completed with explicit issue-backed exceptions (`KIT-MUX-ISSUE-001..003`). |
| `E2-R2` | completed | 0 | From-scratch replay revalidated source+legacy-test intent and added exported-surface requirements (`KIT-MUX-049..053`) without introducing new issue IDs. |
| `E2-R3` | pending | `TBD` | Second consecutive no-new-gap full round pending. |

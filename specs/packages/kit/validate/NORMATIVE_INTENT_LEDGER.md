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
| `kit/validate/README.md` | out-of-scope | n/a | 0 | 0 | out-of-scope | Documentation/reference artifact; not normative mining input for this replay pass. |
| `kit/validate/validate.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; high-level decode/validate helper behavior mapped to `KIT-VALIDATE-003..010`, `KIT-VALIDATE-042`. |
| `kit/validate/validate_test.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; high-level helper and nil-guard scenarios mapped to requirement-level rows. |
| `kit/validate/search_params.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; URL parsing and assignment internals mapped to `KIT-VALIDATE-011..020`, `KIT-VALIDATE-043..044` with issue-backed partials. |
| `kit/validate/search_params_test.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; broad URL parsing matrix mapped to requirement-level rows. |
| `kit/validate/error_collector.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; checker/object/recursive validation core behavior mapped to `KIT-VALIDATE-001..002`, `KIT-VALIDATE-021..033`, `KIT-VALIDATE-039..041`. |
| `kit/validate/error_collector_test.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; checker/object behavior, recursive validation, and error typing scenarios mapped to requirement-level rows. |
| `kit/validate/more_error_collector_test.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; additional map/slice validator and chaining/labeling scenarios mapped to requirement-level rows. |
| `kit/validate/error_acc_test.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; multi-level error accumulation scenarios mapped to requirement-level rows. |
| `kit/validate/rules.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; rule methods (`If`, membership, string, numeric, field-group) mapped to `KIT-VALIDATE-033..038`. |
| `kit/validate/rules_test.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; comprehensive rule-level scenario coverage mapped to requirement-level rows. |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | completed | 2 | Full-pass replay and requirement-level reconciliation completed with explicit issue-backed exceptions (`KIT-VALIDATE-ISSUE-001..002`). |
| `E2-R2` | pending | `TBD` | No-gap clean pass is blocked until open issues are resolved or explicitly accepted. |

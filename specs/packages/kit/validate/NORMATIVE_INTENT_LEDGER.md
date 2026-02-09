# kit/validate Normative Intent Ledger

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
| `kit/validate/README.md` | out-of-scope | n/a | 0 | 0 | out-of-scope | Documentation/reference artifact; not normative mining input for this replay pass. |
| `kit/validate/validate.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | `E2-R2` revalidated high-level decode/validate helper behavior and guard mappings. |
| `kit/validate/validate_test.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | `E2-R2` revalidated high-level helper and nil-guard scenario mappings. |
| `kit/validate/search_params.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | `E2-R2` revalidated URL parsing/assignment internals with issue-backed partials. |
| `kit/validate/search_params_test.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | `E2-R2` revalidated URL parsing matrix evidence against requirement rows. |
| `kit/validate/error_collector.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | `E2-R2` found additional partial gap for `KIT-VALIDATE-001` (`ValidationError.Unwrap()` remains source-backed). |
| `kit/validate/error_collector_test.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | `E2-R2` revalidated checker/object/recursive/error-typing behavior mappings. |
| `kit/validate/more_error_collector_test.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | `E2-R2` revalidated map/slice/chaining/labeling scenario mappings. |
| `kit/validate/error_acc_test.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | `E2-R2` revalidated multi-level error-accumulation scenario mappings. |
| `kit/validate/rules.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | `E2-R2` revalidated rule contracts and branch mappings with existing issue-backed partials. |
| `kit/validate/rules_test.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | `E2-R2` revalidated rule-suite evidence against requirement-level rows. |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | completed | 2 | Full-pass replay and requirement-level reconciliation completed with explicit issue-backed exceptions (`KIT-VALIDATE-ISSUE-001..002`). |
| `E2-R2` | completed | 1 | From-scratch replay corrected `KIT-VALIDATE-001` from covered to partial (`ValidationError.Unwrap()` remains source-backed) without adding new issue IDs. |
| `E2-R3` | pending | `TBD` | Second full replay pass pending; no-gap criterion is currently blocked by open issue backlog. |

# kit/validate Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Mining Rules

- Intent mining MUST use implementation source and incorporate legacy tests
  outside `conformance/**` when present.
- Per-file counters are epoch-scoped.
- `passes`: total pass count in current epoch.
- `clean_passes`: no-new-gap pass count in current epoch.

## Per-File Replay Ledger

| File                                        | Scope        | Mining Inputs       | Passes | Clean Passes | Epoch State     | Notes                                                                                                 |
| ------------------------------------------- | ------------ | ------------------- | -----: | -----------: | --------------- | ----------------------------------------------------------------------------------------------------- |
| `kit/validate/README.md`                    | out-of-scope | n/a                 |      0 |            0 | out-of-scope    | Documentation/reference artifact; not normative mining input for this replay pass.                    |
| `kit/validate/validate.go`                  | in-scope     | source+legacy-tests |      4 |            2 | mined_gaps_open | `E2-R4` full replay found no new requirement gaps; existing issue-backed partials remained unchanged. |
| `kit/validate/validate_test.go`             | in-scope     | source+legacy-tests |      4 |            2 | mined_gaps_open | `E2-R4` full replay found no new requirement gaps; existing issue-backed partials remained unchanged. |
| `kit/validate/search_params.go`             | in-scope     | source+legacy-tests |      4 |            2 | mined_gaps_open | `E2-R4` full replay found no new requirement gaps; existing issue-backed partials remained unchanged. |
| `kit/validate/search_params_test.go`        | in-scope     | source+legacy-tests |      4 |            2 | mined_gaps_open | `E2-R4` full replay found no new requirement gaps; existing issue-backed partials remained unchanged. |
| `kit/validate/error_collector.go`           | in-scope     | source+legacy-tests |      4 |            2 | mined_gaps_open | `E2-R4` full replay found no new requirement gaps; existing issue-backed partials remained unchanged. |
| `kit/validate/error_collector_test.go`      | in-scope     | source+legacy-tests |      4 |            2 | mined_gaps_open | `E2-R4` full replay found no new requirement gaps; existing issue-backed partials remained unchanged. |
| `kit/validate/more_error_collector_test.go` | in-scope     | source+legacy-tests |      4 |            2 | mined_gaps_open | `E2-R4` full replay found no new requirement gaps; existing issue-backed partials remained unchanged. |
| `kit/validate/error_acc_test.go`            | in-scope     | source+legacy-tests |      4 |            2 | mined_gaps_open | `E2-R4` full replay found no new requirement gaps; existing issue-backed partials remained unchanged. |
| `kit/validate/rules.go`                     | in-scope     | source+legacy-tests |      4 |            2 | mined_gaps_open | `E2-R4` full replay found no new requirement gaps; existing issue-backed partials remained unchanged. |
| `kit/validate/rules_test.go`                | in-scope     | source+legacy-tests |      4 |            2 | mined_gaps_open | `E2-R4` full replay found no new requirement gaps; existing issue-backed partials remained unchanged. |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                                                                                                     |
| ------- | --------- | -------: | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `E2-R1` | completed |        2 | Full-pass replay and requirement-level reconciliation completed with explicit issue-backed exceptions (`KIT-VALIDATE-ISSUE-001..002`).                    |
| `E2-R2` | completed |        1 | From-scratch replay corrected `KIT-VALIDATE-001` from covered to partial (`ValidationError.Unwrap()` remains source-backed) without adding new issue IDs. |
| `E2-R3` | completed |        0 | Full replay across source + legacy tests found no new gaps; open `KIT-VALIDATE-ISSUE-001..002` backlog remained unchanged.                                |
| `E2-R4` | completed |        0 | Full replay across source + legacy tests plus issue-validation sanity pass found no new requirements or issue IDs; open `KIT-VALIDATE-ISSUE-*` backlog remains unchanged. |
| `E2-R5` | in_progress |        0 | Next replay is active; ordered closure gates remain blocked by open intent-validation issues. |

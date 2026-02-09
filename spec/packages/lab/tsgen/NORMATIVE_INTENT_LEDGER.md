# lab/tsgen Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Mining Rules

- Intent mining MUST use implementation source and incorporate legacy tests
  outside `conformance/**` when present.
- `conformance/**` suites are verification outputs/evidence and MUST NOT be
  mining inputs.
- Per-file counters are epoch-scoped.
- `passes`: total pass count in current epoch.
- `clean_passes`: no-new-gap pass count in current epoch.

## Per-File Replay Ledger

| File                                    | Scope    | Mining Inputs       | Passes | Clean Passes | Epoch State | Notes                                                                                           |
| --------------------------------------- | -------- | ------------------- | -----: | -----------: | ----------- | ----------------------------------------------------------------------------------------------- |
| `lab/tsgen/generate_ts_content.go`      | in-scope | source+legacy-tests |      1 |            1 | in_progress | Core generation contracts revalidated against source and `generate_ts_content_test.go`.         |
| `lab/tsgen/generate_ts_content_test.go` | in-scope | source+legacy-tests |      1 |            1 | in_progress | Legacy assertions mined for collection/export/type-shape behavior coverage.                     |
| `lab/tsgen/statements.go`               | in-scope | source+legacy-tests |      1 |            0 | in_progress | Statement/serialize helper APIs are currently source-only and tracked by `LAB-TSGEN-ISSUE-001`. |
| `lab/tsgen/to_file.go`                  | in-scope | source+legacy-tests |      1 |            0 | in_progress | File-output/error-path behavior is currently source-only and tracked by `LAB-TSGEN-ISSUE-001`.  |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                                                       |
| ------- | --------- | -------: | ----------------------------------------------------------------------------------------------------------- |
| `E2-R1` | completed |        1 | Rough replay completed; placeholder artifacts replaced with requirement-level source/test-backed contracts. |

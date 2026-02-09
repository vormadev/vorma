# lab/tsgen/tsgencore Normative Intent Ledger

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

| File                                    | Scope    | Mining Inputs       | Passes | Clean Passes | Epoch State | Notes                                                                                                                    |
| --------------------------------------- | -------- | ------------------- | -----: | -----------: | ----------- | ------------------------------------------------------------------------------------------------------------------------ |
| `lab/tsgen/tsgencore/tsgencore.go`      | in-scope | source+legacy-tests |      1 |            0 | in_progress | Core traversal/merge/mapping contracts revalidated; source-only edge helpers tracked by `LAB-TSGEN-TSGENCORE-ISSUE-001`. |
| `lab/tsgen/tsgencore/tsgencore_test.go` | in-scope | source+legacy-tests |      1 |            1 | in_progress | Comprehensive legacy coverage mined for field mapping, embedding, and override semantics.                                |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                          |
| ------- | --------- | -------: | ------------------------------------------------------------------------------ |
| `E2-R1` | completed |        1 | Rough replay completed with detailed owner requirements and traceability rows. |

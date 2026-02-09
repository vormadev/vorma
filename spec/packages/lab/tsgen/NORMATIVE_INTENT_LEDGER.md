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

| File                                    | Scope    | Mining Inputs       | Passes | Clean Passes | Epoch State     | Notes                                                                                                                     |
| --------------------------------------- | -------- | ------------------- | -----: | -----------: | --------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `lab/tsgen/generate_ts_content.go`      | in-scope | source+legacy-tests |      4 |            3 | mined_gaps_open | `E2-R2` replay surfaced `LAB-TSGEN-ISSUE-002`; `E2-R3` and `E2-R4` full replays found no additional gaps.                 |
| `lab/tsgen/generate_ts_content_test.go` | in-scope | source+legacy-tests |      4 |            3 | mined_gaps_open | Legacy assertions still cover core generation behavior; `E2-R3` and `E2-R4` full replays found no additional gaps.        |
| `lab/tsgen/statements.go`               | in-scope | source+legacy-tests |      4 |            3 | mined_gaps_open | Statement/serialize helper APIs remain source-only and tracked by `LAB-TSGEN-ISSUE-001`; no new gaps in `E2-R3`/`E2-R4`.  |
| `lab/tsgen/to_file.go`                  | in-scope | source+legacy-tests |      4 |            3 | mined_gaps_open | File-output/error-path behavior remains source-only and tracked by `LAB-TSGEN-ISSUE-001`; no new gaps in `E2-R3`/`E2-R4`. |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                                                                     |
| ------- | --------- | -------: | ------------------------------------------------------------------------------------------------------------------------- |
| `E2-R1` | completed |        1 | Rough replay completed; placeholder artifacts replaced with requirement-level source/test-backed contracts.               |
| `E2-R2` | completed |        1 | Full replay tightened traceability and added `LAB-TSGEN-ISSUE-002` for source-only `TSTyperRaw` + marshal-error branches. |
| `E2-R3` | completed |        0 | Full replay across in-scope source and legacy tests found no new gaps; open issue backlog remained unchanged.             |
| `E2-R4` | completed |        0 | Second consecutive no-gap full replay confirmed no new gaps; open issue backlog remained unchanged.                       |

# lab/viteutil Normative Intent Ledger

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

| File                       | Scope    | Mining Inputs                                                 | Passes | Clean Passes | Epoch State     | Notes                                                                                                       |
| -------------------------- | -------- | ------------------------------------------------------------- | -----: | -----------: | --------------- | ----------------------------------------------------------------------------------------------------------- |
| `lab/viteutil/cmd.go`      | in-scope | source-only (no active legacy tests outside `conformance/**`) |      3 |            2 | mined_gaps_open | `E2-R1` rough replay surfaced issue-backed gaps; `E2-R2` and `E2-R3` full replays found no additional gaps. |
| `lab/viteutil/viteutil.go` | in-scope | source-only (no active legacy tests outside `conformance/**`) |      3 |            2 | mined_gaps_open | `E2-R1` rough replay surfaced default-port gap; `E2-R2` and `E2-R3` full replays found no additional gaps.  |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                                                 |
| ------- | --------- | -------: | ----------------------------------------------------------------------------------------------------- |
| `E2-R1` | completed |        3 | Rough replay completed with source-backed requirement catalog and issue-backed bug-candidate gaps.    |
| `E2-R2` | completed |        0 | Full replay across `cmd.go` + `viteutil.go` found no new gaps; open issue backlog remained unchanged. |
| `E2-R3` | completed |        0 | Second consecutive no-gap full replay confirmed no new gaps; open issue backlog remained unchanged.   |

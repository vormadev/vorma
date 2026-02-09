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

| File                       | Scope    | Mining Inputs                                                 | Passes | Clean Passes | Epoch State | Notes                                                                                                                                         |
| -------------------------- | -------- | ------------------------------------------------------------- | -----: | -----------: | ----------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `lab/viteutil/cmd.go`      | in-scope | source-only (no active legacy tests outside `conformance/**`) |      1 |            0 | in_progress | Build context/dev/prod command orchestration revalidated; start-error propagation and command-token precondition gaps tracked in open issues. |
| `lab/viteutil/viteutil.go` | in-scope | source-only (no active legacy tests outside `conformance/**`) |      1 |            0 | in_progress | Manifest/dev-script/port helper behavior revalidated; default-port handling gap tracked in open issues.                                       |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                                              |
| ------- | --------- | -------: | -------------------------------------------------------------------------------------------------- |
| `E2-R1` | completed |        3 | Rough replay completed with source-backed requirement catalog and issue-backed bug-candidate gaps. |

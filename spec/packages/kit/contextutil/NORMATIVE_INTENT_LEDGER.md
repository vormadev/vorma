# kit/contextutil Normative Intent Ledger

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

| File                                  | Scope        | Mining Inputs       | Passes | Clean Passes | Epoch State | Notes                                                                                           |
| ------------------------------------- | ------------ | ------------------- | -----: | -----------: | ----------- | ----------------------------------------------------------------------------------------------- |
| `kit/contextutil/README.md`           | out-of-scope | n/a                 |      0 |            0 | pending     | Documentation-only; not normative source for this replay.                                       |
| `kit/contextutil/contextutil.go`      | in-scope     | source+legacy-tests |      1 |            0 | in_progress | Store/write/read contracts revalidated; fallback branch tracked by `KIT-CONTEXTUTIL-ISSUE-001`. |
| `kit/contextutil/contextutil_test.go` | in-scope     | source+legacy-tests |      1 |            0 | in_progress | Legacy assertions mapped for generic roundtrip and non-collision behavior.                      |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                                   |
| ------- | --------- | -------: | --------------------------------------------------------------------------------------- |
| `E2-R1` | completed |        1 | Placeholder companion artifacts replaced and requirement-level traceability reconciled. |

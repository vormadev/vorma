# kit/headels Normative Intent Ledger

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

| File                             | Scope        | Mining Inputs       | Passes | Clean Passes | Epoch State | Notes                                                                                                                                          |
| -------------------------------- | ------------ | ------------------- | -----: | -----------: | ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `kit/headels/README.md`          | out-of-scope | n/a                 |      0 |            0 | pending     | Documentation-only; not normative source for this replay.                                                                                      |
| `kit/headels/headblocks.go`      | in-scope     | source+legacy-tests |      2 |            1 | in_progress | `E2-R1` baseline mined `KIT-HEADELS-001..016` and surfaced `KIT-HEADELS-ISSUE-001..004`; `E2-R2` replay found no new requirement or issue IDs. |
| `kit/headels/headblocks_test.go` | in-scope     | source+legacy-tests |      2 |            1 | in_progress | `E2-R2` replay revalidated legacy evidence mappings and issue-backed partials with no newly surfaced gaps.                                     |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                                                                           |
| ------- | --------- | -------: | ------------------------------------------------------------------------------------------------------------------------------- |
| `E2-R1` | completed |        4 | Placeholder package replaced; active gaps tracked as `KIT-HEADELS-ISSUE-001..004`.                                              |
| `E2-R2` | completed |        0 | Full replay across source + legacy tests found no new requirement or issue IDs; existing `KIT-HEADELS-ISSUE-*` backlog remains. |

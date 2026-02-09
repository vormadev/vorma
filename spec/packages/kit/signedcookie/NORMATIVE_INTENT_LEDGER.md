# kit/signedcookie Normative Intent Ledger

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

| File                                    | Scope        | Mining Inputs       | Passes | Clean Passes | Epoch State | Notes                                                                                                           |
| --------------------------------------- | ------------ | ------------------- | -----: | -----------: | ----------- | --------------------------------------------------------------------------------------------------------------- |
| `kit/signedcookie/README.md`            | out-of-scope | n/a                 |      0 |            0 | pending     | Documentation-only; not normative source for this replay.                                                       |
| `kit/signedcookie/signedcookie.go`      | in-scope     | source+legacy-tests |      1 |            0 | in_progress | Manager + typed signed-cookie contracts revalidated; nil-edge branches tracked by `KIT-SIGNEDCOOKIE-ISSUE-001`. |
| `kit/signedcookie/signedcookie_test.go` | in-scope     | source+legacy-tests |      1 |            0 | in_progress | Legacy assertions mapped to requirement/scenario rows for sign/read/deletion/encryption/rotation behavior.      |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                                     |
| ------- | --------- | -------: | ----------------------------------------------------------------------------------------- |
| `E2-R1` | completed |        1 | Placeholder package replaced with detailed source+legacy-test-backed requirement catalog. |

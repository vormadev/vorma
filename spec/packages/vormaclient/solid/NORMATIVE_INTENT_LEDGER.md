# vormaclient/solid Normative Intent Ledger

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

| File                               | Scope    | Mining Inputs       | Passes | Clean Passes | Epoch State | Notes                                                                                                           |
| ---------------------------------- | -------- | ------------------- | -----: | -----------: | ----------- | --------------------------------------------------------------------------------------------------------------- |
| `vormaclient/solid/index.tsx`      | in-scope | source+legacy-tests |      1 |            1 | in_progress | Export surface revalidated from source; no active legacy tests outside `conformance/**` found for this package. |
| `vormaclient/solid/src/helpers.ts` | in-scope | source+legacy-tests |      1 |            1 | in_progress | Typed helper and client-loader helper contracts revalidated from source.                                        |
| `vormaclient/solid/src/link.tsx`   | in-scope | source+legacy-tests |      1 |            1 | in_progress | Link and typed-link contracts revalidated from source.                                                          |
| `vormaclient/solid/src/solid.tsx`  | in-scope | source+legacy-tests |      1 |            1 | in_progress | Adapter root-outlet/location surface revalidated from source; owner runtime semantics remain in `FE-*`.         |
| `vormaclient/solid/tsconfig.json`  | in-scope | source+legacy-tests |      1 |            1 | in_progress | JSX compiler-option contract revalidated from source.                                                           |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                      |
| ------- | --------- | -------: | -------------------------------------------------------------------------- |
| `E2-R1` | completed |        0 | Rough replay completed with source-backed contracts and traceability rows. |

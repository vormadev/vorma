# vormaclient/create Normative Intent Ledger

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

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Notes |
|---|---|---|---:|---:|---|---|
| `vormaclient/create/.gitignore` | in-scope | source+legacy-tests | 1 | 1 | in_progress | Distribution ignore contract revalidated from source; no active legacy tests outside `conformance/**` found for this package. |
| `vormaclient/create/main.ts` | in-scope | source+legacy-tests | 1 | 1 | in_progress | CLI prompt/validation/bootstrap contracts revalidated from source. |
| `vormaclient/create/package.json` | in-scope | source+legacy-tests | 1 | 1 | in_progress | Package metadata/bin/runtime contracts revalidated from source. |
| `vormaclient/create/pnpm-lock.yaml` | in-scope | source+legacy-tests | 1 | 1 | in_progress | Dependency lock artifact presence and package resolution snapshot recorded for this epoch. |
| `vormaclient/create/tsconfig.json` | in-scope | source+legacy-tests | 1 | 1 | in_progress | TypeScript base-config extension contract revalidated from source. |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | completed | 0 | Rough replay completed with source-backed contracts and traceability rows. |

# vormabuild Normative Intent Ledger

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

| File                                 | Scope    | Mining Inputs | Passes | Clean Passes | Epoch State | Notes                                                                                              |
| ------------------------------------ | -------- | ------------- | -----: | -----------: | ----------- | -------------------------------------------------------------------------------------------------- |
| `vormabuild/configschema.go`         | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are present for this package path. |
| `vormabuild/fs_to_hash.go`           | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are present for this package path. |
| `vormabuild/rebuild_routes.go`       | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are present for this package path. |
| `vormabuild/route_registry_build.go` | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are present for this package path. |
| `vormabuild/vite_cmd.go`             | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are present for this package path. |
| `vormabuild/vorma_build.go`          | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are present for this package path. |
| `vormabuild/vorma_gen_ts.go`         | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are present for this package path. |

## Round Log

| Round   | Status      | New Gaps | Notes                                                                                     |
| ------- | ----------- | -------: | ----------------------------------------------------------------------------------------- |
| `E2-R1` | in_progress |        0 | Rough source replay complete across all in-scope files; full-pass reconciliation pending. |

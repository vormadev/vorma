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

| File                                 | Scope    | Mining Inputs | Passes | Clean Passes | Epoch State     | Notes                                                                                                                                                                    |
| ------------------------------------ | -------- | ------------- | -----: | -----------: | --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `vormabuild/configschema.go`         | in-scope | source-only   |      4 |            4 | mined_gaps_open | `E2-R1` rough replay complete; `E2-R2`, `E2-R3`, and `E2-R4` full replays found no new gaps. No legacy tests outside `conformance/**` are present for this package path. |
| `vormabuild/fs_to_hash.go`           | in-scope | source-only   |      4 |            4 | mined_gaps_open | `E2-R1` rough replay complete; `E2-R2`, `E2-R3`, and `E2-R4` full replays found no new gaps. No legacy tests outside `conformance/**` are present for this package path. |
| `vormabuild/rebuild_routes.go`       | in-scope | source-only   |      4 |            4 | mined_gaps_open | `E2-R1` rough replay complete; `E2-R2`, `E2-R3`, and `E2-R4` full replays found no new gaps. No legacy tests outside `conformance/**` are present for this package path. |
| `vormabuild/route_registry_build.go` | in-scope | source-only   |      4 |            4 | mined_gaps_open | `E2-R1` rough replay complete; `E2-R2`, `E2-R3`, and `E2-R4` full replays found no new gaps. No legacy tests outside `conformance/**` are present for this package path. |
| `vormabuild/vite_cmd.go`             | in-scope | source-only   |      4 |            4 | mined_gaps_open | `E2-R1` rough replay complete; `E2-R2`, `E2-R3`, and `E2-R4` full replays found no new gaps. No legacy tests outside `conformance/**` are present for this package path. |
| `vormabuild/vorma_build.go`          | in-scope | source-only   |      4 |            4 | mined_gaps_open | `E2-R1` rough replay complete; `E2-R2`, `E2-R3`, and `E2-R4` full replays found no new gaps. No legacy tests outside `conformance/**` are present for this package path. |
| `vormabuild/vorma_gen_ts.go`         | in-scope | source-only   |      4 |            4 | mined_gaps_open | `E2-R1` rough replay complete; `E2-R2`, `E2-R3`, and `E2-R4` full replays found no new gaps. No legacy tests outside `conformance/**` are present for this package path. |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                                                                                              |
| ------- | --------- | -------: | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| `E2-R1` | completed |        0 | Rough source replay completed across all in-scope files as baseline for full-pass reconciliation.                                                  |
| `E2-R2` | completed |        0 | Full replay across all in-scope files found no new gaps; active divergence backlog remained `VCI-024`, `VCI-026`, `VCI-040`, `VCI-041`, `VCI-050`. |
| `E2-R3` | completed |        0 | Second consecutive no-gap full replay across all in-scope files; issue backlog unchanged and still open.                                           |
| `E2-R4` | completed |        0 | Full replay reconfirmed no new gaps; active divergence backlog remains `VCI-024`, `VCI-026`, `VCI-040`, `VCI-041`, and `VCI-050`.                  |

# wave/tooling Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Mining Rules

- Intent mining MUST use implementation source plus legacy tests outside
  `conformance/**`.
- Per-file counters are epoch-scoped.
- `passes`: total pass count in current epoch.
- `clean_passes`: no-new-gap pass count in current epoch.

## Per-File Replay Ledger

| File                           | Scope    | Mining Inputs       | Passes | Clean Passes | Epoch State | Notes                                                                      |
| ------------------------------ | -------- | ------------------- | -----: | -----------: | ----------- | -------------------------------------------------------------------------- |
| `wave/tooling/broadcast.go`    | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/builder.go`      | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/cli.go`          | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/css.go`          | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/devserver.go`    | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/events.go`       | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/hash.go`         | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/lock.go`         | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/lock_unix.go`    | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/lock_windows.go` | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/schema.go`       | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/static.go`       | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/url.go`          | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |
| `wave/tooling/watcher.go`      | in-scope | source+legacy-tests |      1 |            1 | in_progress | Rough replay complete; no additional gaps beyond migrated `WCI-*` backlog. |

## Round Log

| Round   | Status      | New Gaps | Notes                                                                                                                                                                          |
| ------- | ----------- | -------: | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `E2-R1` | in_progress |        0 | Package-path baseline initialized; `WAVE-*` build/dev owner catalog migrated from `spec/packages/wave/`; rough source replay found no additional gaps beyond migrated backlog. |

# wave/tooling Normative Intent Ledger

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

| File                           | Scope    | Mining Inputs | Passes | Clean Passes | Epoch State | Notes                                                                                                        |
| ------------------------------ | -------- | ------------- | -----: | -----------: | ----------- | ------------------------------------------------------------------------------------------------------------ |
| `wave/tooling/broadcast.go`    | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/builder.go`      | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/cli.go`          | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/css.go`          | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/devserver.go`    | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/events.go`       | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/hash.go`         | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/lock.go`         | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/lock_unix.go`    | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/lock_windows.go` | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/schema.go`       | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/static.go`       | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/url.go`          | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/watcher.go`      | in-scope | source-only   |      1 |            1 | in_progress | Rough replay complete; no legacy tests outside `conformance/**` are currently present for this package path. |

## Round Log

| Round   | Status      | New Gaps | Notes                                                                                                                                  |
| ------- | ----------- | -------: | -------------------------------------------------------------------------------------------------------------------------------------- |
| `E2-R1` | in_progress |        0 | Rough source replay complete across in-scope files; active `WCI-*` backlog remains open and full-pass reconciliation is still pending. |

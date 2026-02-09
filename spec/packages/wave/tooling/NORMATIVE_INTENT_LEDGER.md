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

| File                           | Scope    | Mining Inputs | Passes | Clean Passes | Epoch State | Notes                                                                                                         |
| ------------------------------ | -------- | ------------- | -----: | -----------: | ----------- | ------------------------------------------------------------------------------------------------------------- |
| `wave/tooling/broadcast.go`    | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/builder.go`      | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/cli.go`          | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/css.go`          | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/devserver.go`    | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/events.go`       | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/hash.go`         | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/lock.go`         | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/lock_unix.go`    | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/lock_windows.go` | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/schema.go`       | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/static.go`       | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |
| `wave/tooling/url.go`          | in-scope | source-only   |      5 |            4 | in_progress | Replayed through E2-R5; prior requirement addition in E2-R4 (`WAVE-URL-002`) remains reflected.               |
| `wave/tooling/watcher.go`      | in-scope | source-only   |      5 |            5 | in_progress | Replayed through E2-R5; no legacy tests outside `conformance/**` are currently present for this package path. |

## Round Log

| Round   | Status      | New Gaps | Notes                                                                                                                                                                                                                                                                |
| ------- | ----------- | -------: | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `E2-R1` | completed   |        0 | Rough source replay completed across all in-scope files; active `WCI-*` backlog remained open.                                                                                                                                                                       |
| `E2-R2` | completed   |        0 | Second source replay completed with requirement/scenario catalog expansion (`WAVE-BUILD-*`, `WAVE-WATCH-*`, `WAVE-LOCK-*`, `WAVE-URL-*`) and no additional gap IDs discovered.                                                                                       |
| `E2-R3` | completed   |        0 | Full replay across all in-scope `wave/tooling/*.go` files plus legacy-tests-outside-`conformance/**` input check found no new requirements or issue IDs; open `WCI-*` implementation-divergence backlog remains and the two-consecutive no-gap gate remains pending. |
| `E2-R4` | completed   |        1 | Full replay across all in-scope files surfaced additional build-time public file-map helper contracts in `url.go` (`WAVE-URL-002`); open `WCI-*` backlog remains and prevents closure gates.                                                                         |
| `E2-R5` | completed   |        0 | Full replay plus issue-validation sanity pass found no new requirement or gap IDs; open `WCI-*` backlog remains unchanged and no-gap closure gates stay blocked by open issues.                                                                                      |
| `E2-R6` | in_progress |        0 | Next full replay is active while ordered closure gates remain blocked by open `WCI-*` issues.                                                                                                                                                                        |

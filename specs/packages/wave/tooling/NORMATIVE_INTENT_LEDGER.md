# wave/tooling Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Mining Rules

- Intent mining MUST use both implementation source and existing tests.
- Per-file counters are epoch-scoped.
- `passes`: total pass count in current epoch.
- `clean_passes`: no-new-gap pass count in current epoch.

## Per-File Replay Ledger

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Notes |
|---|---|---|---:|---:|---|---|
| `wave/tooling/broadcast.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/builder.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/cli.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/css.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/devserver.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/events.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/hash.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/lock.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/lock_unix.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/lock_windows.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/schema.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/static.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/url.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `wave/tooling/watcher.go` | in-scope | source+tests | 0 | 0 | pending |  |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | `TBD` | Package-path reset baseline initialized. |

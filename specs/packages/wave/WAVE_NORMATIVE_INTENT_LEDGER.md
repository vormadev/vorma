# Wave Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2` (package-local reset)

## Mining Rules

- Intent mining MUST use both implementation source and existing tests.
- Per-file counters are epoch-scoped.
- `passes`: total number of full mining passes for that file in current epoch.
- `clean_passes`: number of passes with no newly surfaced normative gap for that file in current epoch.

## Per-File Replay Ledger

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Legacy State | Notes |
|---|---|---|---:|---:|---|---|---|
| `wave/css.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `wave/env.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `wave/filemap.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `wave/parse.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `wave/refresh.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `wave/tooling/broadcast.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/builder.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/cli.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/css.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/devserver.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/events.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/hash.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/lock.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/lock_unix.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/lock_windows.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/schema.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/static.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/url.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/tooling/watcher.go` | in-scope | source+tests | 0 | 0 | pending | out-of-scope | Reclassified for Wave-owned package mining in epoch E2. |
| `wave/types.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `wave/wave.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | `TBD` | Epoch reset baseline created; per-file counters initialized. |

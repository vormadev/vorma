# vormaruntime Normative Intent Ledger

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
| `vormaruntime/errors.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `vormaruntime/get_deps.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `vormaruntime/get_root_handler.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `vormaruntime/glue.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `vormaruntime/gmpd.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `vormaruntime/paths.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `vormaruntime/route_registry.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `vormaruntime/route_reload.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `vormaruntime/ssr.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `vormaruntime/types.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `vormaruntime/vite_url.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `vormaruntime/vorma_core.go` | in-scope | source+tests | 0 | 0 | pending |  |
| `vormaruntime/vorma_init.go` | in-scope | source+tests | 0 | 0 | pending |  |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | `TBD` | Package-path reset baseline initialized. |

# wave Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2` (package-local reset)

## Mining Rules

- Intent mining MUST use implementation source and incorporate legacy tests
  outside `conformance/**` when present.
- Per-file counters are epoch-scoped.
- `passes`: total number of full mining passes for that file in current epoch.
- `clean_passes`: number of passes with no newly surfaced normative gap for that file in current epoch.

## Per-File Replay Ledger

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Legacy State | Notes |
|---|---|---|---:|---:|---|---|---|
| `wave/css.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | verified | Runtime helper catalog extracted (`WAVE-RT-013`, `WAVE-RT-014`). |
| `wave/env.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | verified | Env/mode/port contracts extracted (`WAVE-RT-017`). |
| `wave/filemap.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | verified | Filemap helper contracts extracted (`WAVE-RT-015`). |
| `wave/parse.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | verified | Parse safety/default contracts extracted (`WAVE-RT-001`). |
| `wave/refresh.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | verified | Refresh helper contracts extracted (`WAVE-RT-016`). |
| `wave/types.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | verified | ParsedConfig/default/mutator contracts extracted (`WAVE-RT-018`, `WAVE-RT-019`). |
| `wave/wave.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | verified | Constructor/runtime FS/static-serving contracts extracted (`WAVE-RT-002`..`WAVE-RT-012`). |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | 1 | Runtime-owner catalog authored and reconciled with source-only evidence state where no legacy tests outside `conformance/**` are present in-repo. |

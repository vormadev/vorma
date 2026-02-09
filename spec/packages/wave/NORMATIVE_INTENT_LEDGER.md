# wave Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2` (package-local reset)

## Mining Rules

- Intent mining MUST use implementation source and incorporate legacy tests
  outside `conformance/**` when present.
- Per-file counters are epoch-scoped.
- `passes`: total number of full mining passes for that file in current epoch.
- `clean_passes`: number of passes with no newly surfaced normative gap for that
  file in current epoch.

## Per-File Replay Ledger

| File              | Scope    | Mining Inputs       | Passes | Clean Passes | Epoch State | Legacy State | Notes                                                                                                                                                                      |
| ----------------- | -------- | ------------------- | -----: | -----------: | ----------- | ------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `wave/css.go`     | in-scope | source+legacy-tests |      3 |            2 | in_progress | verified     | `E2-R1` rough replay baseline plus `E2-R2` and `E2-R3` no-gap full replays reconfirmed helper contracts (`WAVE-RT-013`, `WAVE-RT-014`).                                    |
| `wave/env.go`     | in-scope | source+legacy-tests |      3 |            2 | in_progress | verified     | `E2-R2` and `E2-R3` full replays reconfirmed env/mode/port semantics, aliasing, and `Wave` env-wrapper delegation (`WAVE-RT-017`).                                         |
| `wave/filemap.go` | in-scope | source+legacy-tests |      3 |            2 | in_progress | verified     | `E2-R1` + `E2-R2` + `E2-R3` replays reconfirmed filemap URL/elements/script-hash helper contracts (`WAVE-RT-015`).                                                         |
| `wave/parse.go`   | in-scope | source+legacy-tests |      3 |            2 | in_progress | verified     | `E2-R1` + `E2-R2` + `E2-R3` replays reconfirmed parse safety/default contracts for byte/file entrypoints (`WAVE-RT-001`).                                                  |
| `wave/refresh.go` | in-scope | source+legacy-tests |      3 |            2 | in_progress | verified     | `E2-R1` + `E2-R2` + `E2-R3` replays reconfirmed refresh helper and template interpolation contracts (`WAVE-RT-016`).                                                       |
| `wave/types.go`   | in-scope | source+legacy-tests |      3 |            2 | in_progress | verified     | `E2-R2` and `E2-R3` full replays reconfirmed ParsedConfig/FileMap/RelPaths/DistLayout/watch-helper semantics (`WAVE-RT-008`, `WAVE-RT-018`, `WAVE-RT-020`, `WAVE-RT-021`). |
| `wave/wave.go`    | in-scope | source+legacy-tests |      3 |            2 | in_progress | verified     | `E2-R2` and `E2-R3` full replays reconfirmed constructor/runtime FS/static-serving/accessor/mutator contracts (`WAVE-RT-002`..`WAVE-RT-012`, `WAVE-RT-019`).               |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                                                                                                                                                                       |
| ------- | --------- | -------: | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `E2-R1` | completed |        1 | Rough baseline replay expanded runtime-owner accessor/env/path-helper clauses, including `types.go` helper semantics, and reconciled source-only evidence state where no legacy tests outside `conformance/**` are present. |
| `E2-R2` | completed |        0 | Full replay across all in-scope `wave/*.go` files found no new gaps and tightened `WAVE-RT-017` wrapper-delegation wording in spec/matrix.                                                                                  |
| `E2-R3` | completed |        0 | Second consecutive no-gap full replay across all in-scope `wave/*.go` files confirmed no new gaps; package-level issue backlog remains empty.                                                                               |

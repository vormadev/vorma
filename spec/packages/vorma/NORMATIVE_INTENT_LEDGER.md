# vorma Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2` (wrapper canonical reset)

## Mining Rules

- Intent mining MUST use implementation source plus legacy tests outside `conformance/**`.
- `conformance/**` suites are verification outputs/evidence and MUST NOT be mining inputs.
- Per-file counters are epoch-scoped.
- `passes`: total number of full mining passes for that file in current epoch.
- `clean_passes`: number of passes with no newly surfaced normative gap for that file in current epoch.

## Per-File Replay Ledger

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Notes |
|---|---|---|---:|---:|---|---|
| `vorma.go` | in-scope | source+legacy-tests | 3 | 2 | mined_gaps_open | `E2-R2` and `E2-R3` clean replays reconfirmed wrapper contracts (`VORMA-API-001..004`); no legacy wrapper tests outside `conformance/**` were found. |
| `package.json` | in-scope | source+legacy-tests | 3 | 2 | mined_gaps_open | `E2-R2` and `E2-R3` clean replays reconfirmed embedded npm version contract (`VORMA-API-005`). |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | completed | 1 | Canonical reset replay corrected traceability evidence model to exclude `conformance/**` inputs; wrapper intent-validation gap remains tracked as `VORMA-ISSUE-001`. |
| `E2-R2` | completed | 0 | Clean replay revalidated `vorma.go` + embedded package version contract and reconfirmed no legacy wrapper tests outside `conformance/**`. |
| `E2-R3` | completed | 0 | Clean replay reconfirmed source-derived wrapper requirements with no new gap IDs; with `E2-R2`, this satisfies two consecutive no-gap full rounds. |

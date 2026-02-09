# kit/response Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Mining Rules

- Intent mining MUST use implementation source plus legacy tests outside `conformance/**`.
- Per-file counters are epoch-scoped.
- `passes`: total pass count in current epoch.
- `clean_passes`: no-new-gap pass count in current epoch.

## Per-File Replay Ledger

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Notes |
|---|---|---|---:|---:|---|---|
| `kit/response/README.md` | out-of-scope | n/a | 0 | 0 | pending | Documentation-only; not normative source for this replay. |
| `kit/response/proxy.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | Proxy requirements mined (`KIT-RESPONSE-012..018`); surfaced issues `001..003`. |
| `kit/response/proxy_test.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | Legacy evidence mapped to proxy requirement/scenario rows. |
| `kit/response/response.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | Response requirements mined (`KIT-RESPONSE-001..011`); surfaced issues `001..003`. |
| `kit/response/response_test.go` | in-scope | source+legacy-tests | 1 | 0 | in_progress | Legacy evidence mapped to response requirement/scenario rows. |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | 3 | Placeholder package replaced; gaps tracked as `KIT-RESPONSE-ISSUE-001..003`. |

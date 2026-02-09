# Mining Dispatch Board (Shared Checkout)

Purpose: mutual exclusion only. This board never implies package completion.

## Claim Protocol

1. Claim the lowest-numbered row with `Status = OPEN`.
2. Set `Status = CLAIMED`, `Agent`, `Claimed At (UTC)`.
3. While claimed, edit only `spec/packages/<claimed-package>/**` and your row.
4. On handoff, set row to `OPEN`, clear claim fields, add a short neutral note.
5. Hold exactly one claimed row at a time.

## Status Semantics

- Allowed values: `OPEN`, `CLAIMED`.
- These values are lock state only.

## Scope Guardrail

A claim does not authorize edits outside `spec/**`.

## P0 Queue (claim in order)

| Slot | Package Path         | Status | Agent | Claimed At (UTC) | Notes |
| ---- | -------------------- | ------ | ----- | ---------------- | ----- |
| 1    | `vormaclient/client` | OPEN   |       |                  |       |
| 2    | `vormabuild`         | OPEN   |       |                  |       |
| 3    | `wave/tooling`       | OPEN   |       |                  |       |
| 4    | `wave`               | OPEN   |       |                  |       |
| 5    | `vormaruntime`       | OPEN   |       |                  |       |
| 6    | `lab/tsgen`          | OPEN   |       |                  |       |
| 7    | `lab/viteutil`       | OPEN   |       |                  |       |
| 8    | `kit/response`       | OPEN   |       |                  |       |
| 9    | `kit/validate`       | OPEN   |       |                  |       |
| 10   | `kit/headels`        | OPEN   |       |                  |       |
| 11   | `kit/mux`            | OPEN   |       |                  |       |
| 12   | `kit/matcher`        | OPEN   |       |                  |       |
| 13   | `vorma`              | OPEN   |       |                  |       |

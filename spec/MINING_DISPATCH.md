# Mining Dispatch Board (Shared Checkout)

Use this file as the faux mutex for parallel Step 1 mining in one checkout.

## New Chat Bootstrap

Tell a new agent chat:

`Read spec/AGENTS_START_HERE.md and spec/MINING_DISPATCH.md, claim the next OPEN slot, then work only that package under spec/**.`

## Claim Protocol (Required)

1. Open this file and find the lowest-numbered row with `Status = OPEN`.
2. Claim by editing only that row: set `Status = CLAIMED`, set `Agent`, set
   `Claimed At (UTC)`.
3. If the row changed before save, refresh and claim the next `OPEN` row.
4. While claimed, edit only: `spec/packages/<claimed-package>/**` and your row
   in this file.
5. At handoff, release your row: set `Status = OPEN`, clear `Agent`, clear
   `Claimed At (UTC)`, and add/update a short progress note.
6. Hold only one claimed row at a time.

## Status Semantics (Required)

- `Status` is a lock state only, not a completion state.
- Allowed status values are only `OPEN` and `CLAIMED`.
- This board MUST NOT assert package closure/program closure.
- Completion/closure state is owned by package artifacts
  (`SPEC_CHECKLIST.md`, `FULL_AUDIT_TRACKER.md`, `CONFORMANCE_ISSUES.md`) and
  `spec/SPEC_GOVERNANCE.md`.

## Coordinator Election (Required)

- Coordinator is the agent that currently holds the lowest-numbered
  `Status = CLAIMED` row.
- If no rows are `CLAIMED`, coordinator is unset until the next claim.
- Coordinator maintains `spec/ROUGH_STATUS_UPDATE.md` and queue/governance
  scaffolding docs: `spec/MINING_DISPATCH.md` (non-row policy text),
  `spec/AGENTS_START_HERE.md`, and `spec/SPEC_GOVERNANCE.md`.
- Workers edit only their claimed package artifacts and their own dispatch row.

## P0 Slot Queue (Strict Order)

Claim the lowest `OPEN` slot number.

| Slot | Package Path         | Status | Agent | Claimed At (UTC) | Notes                                                                                         |
| ---- | -------------------- | ------ | ----- | ---------------- | --------------------------------------------------------------------------------------------- |
| 1    | `vormaclient/client` | OPEN   |       |                  | Latest replay snapshot: `E2-R4`; open `VCI-*` backlog remains.                               |
| 2    | `vormabuild`         | OPEN   |       |                  | Latest replay snapshot: `E2-R4`; open `VCI-*` backlog remains.                               |
| 3    | `wave/tooling`       | OPEN   |       |                  | Latest replay snapshot: `E2-R5`; open `WCI-*` backlog remains.                               |
| 4    | `wave`               | OPEN   |       |                  | Latest replay snapshot: `E2-R3`; package stop/closure must be read from package artifacts.   |
| 5    | `vormaruntime`       | OPEN   |       |                  | Latest replay snapshot: `E2-R10`; open `VCI-*` backlog remains.                              |
| 6    | `lab/tsgen`          | OPEN   |       |                  | Latest replay snapshot: `E2-R4`; open issue backlog remains.                                 |
| 7    | `lab/viteutil`       | OPEN   |       |                  | Latest replay snapshot: `E2-R3`; open issue backlog remains.                                 |
| 8    | `kit/response`       | OPEN   |       |                  | Latest replay snapshot: `E2-R3`; open issue backlog remains.                                 |
| 9    | `kit/validate`       | OPEN   |       |                  | Latest replay snapshot: `E2-R4`; open `KIT-VALIDATE-ISSUE-*` backlog remains.                |
| 10   | `kit/headels`        | OPEN   |       |                  | Latest replay snapshot: `E2-R4`; open `KIT-HEADELS-ISSUE-*` backlog remains.                 |
| 11   | `kit/mux`            | OPEN   |       |                  | Latest replay snapshot: replay completed; open `KIT-MUX-ISSUE-*` backlog remains.            |
| 12   | `kit/matcher`        | OPEN   |       |                  | Latest replay snapshot: `E2-R6`; open `KIT-MATCHER-ISSUE-*` backlog remains.                 |
| 13   | `vorma`              | OPEN   |       |                  | Latest replay snapshot: checklist has completion markers; closure must be read from package docs. |

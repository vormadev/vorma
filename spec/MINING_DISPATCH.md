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
5. At handoff, release your row: set `Status = OPEN` (if work remains) or
   `Status = DONE` (if stop criterion is met), and add a short note.
6. Hold only one claimed row at a time.

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

| Slot | Package Path         | Status | Agent      | Claimed At (UTC)     | Notes                                                                                                       |
| ---- | -------------------- | ------ | ---------- | -------------------- | ----------------------------------------------------------------------------------------------------------- |
| 1    | `vormaclient/client` | DONE   | codex-gpt5 | 2026-02-09T22:24:42Z | E2-R4 no-gap full replay complete; open `VCI-*` backlog unchanged                                           |
| 2    | `vormabuild`         | DONE   | codex-gpt5 | 2026-02-09T22:09:31Z | E2-R4 no-gap replay complete; open `VCI-*` backlog unchanged                                                |
| 3    | `wave/tooling`       | DONE   | codex-gpt5 | 2026-02-09T22:30:59Z | E2-R5 no-gap replay complete; open `WCI-*` backlog unchanged                                                |
| 4    | `wave`               | DONE   | codex-gpt5 | 2026-02-09T22:31:28Z | E2-R3 no-gap full replay complete; no active package-level issues                                           |
| 5    | `vormaruntime`       | DONE   | codex-gpt5 | 2026-02-09T22:03:32Z | E2-R10 full replay complete; no new gaps (`VCI-*` backlog open)                                             |
| 6    | `lab/tsgen`          | DONE   | codex-gpt5 | 2026-02-09T22:21:00Z | E2-R4 no-gap full replay complete; open issue backlog unchanged                                             |
| 7    | `lab/viteutil`       | DONE   | codex-gpt5 | 2026-02-09T22:12:19Z | E2-R3 no-gap full replay complete; open issue backlog unchanged                                             |
| 8    | `kit/response`       | DONE   | codex-gpt5 | 2026-02-09T22:31:41Z | E2-R3 no-gap full replay complete; open issue backlog unchanged                                             |
| 9    | `kit/validate`       | DONE    | codex-gpt5 | 2026-02-09T22:36:21Z | E2-R4 no-gap replay complete; open `KIT-VALIDATE-ISSUE-*` backlog unchanged                                  |
| 10   | `kit/headels`        | CLAIMED | codex-gpt5 | 2026-02-09T22:37:22Z | Active Step-1 mining                                                                                          |
| 11   | `kit/mux`            | CLAIMED | codex-gpt5 | 2026-02-09T22:37:45Z | Active Step-1 mining                                                                                          |
| 12   | `kit/matcher`        | OPEN   |            |                      |                                                                                                             |
| 13   | `vorma`              | DONE   |            |                      | Checklist complete                                                                                          |

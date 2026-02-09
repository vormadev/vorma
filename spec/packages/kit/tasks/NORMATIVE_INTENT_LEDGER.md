# Normative Intent Ledger

Purpose: file-level mining coverage and replay rounds.

Evidence policy reminder:

- Primary: implementation source files.
- Optional corroboration: legacy tests outside `conformance/**`.
- Never use `conformance/**` as mining evidence.

No-gap round meaning (strict):

- Attempted from scratch and in full to mine all normative intent to
  rebuild-from-scratch-from-spec-alone level.
- Found zero missing normative requirements in current spec.
- Found zero incorrect normative claims in current spec.
- Found zero open blocking intent-validation issues after the round.

## In-Scope Files

| File Path | Category | Passes | Clean Passes | Last Pass Kind | State   | Notes |
| --------- | -------- | ------ | ------------ | -------------- | ------- | ----- |
| TODO      | source   | 0      | 0            | none           | pending |       |

## Replay Rounds

| Round | Role              | Result  | From Scratch | Full Scope | Rebuild-From-Scratch-From-Spec-Alone Checked | Missing Req Count | Incorrect Claim Count | Blocking Issue Count | No-Gap Qualified | Summary                                              |
| ----- | ----------------- | ------- | ------------ | ---------- | -------------------------------------------- | ----------------- | --------------------- | -------------------- | ---------------- | ---------------------------------------------------- |
| R1    | baseline_full     | pending | no           | no         | no                                           | TBD               | TBD                   | TBD                  | no               | Baseline full replay; gap discovery expected.        |
| R2    | verification_full | pending | no           | no         | no                                           | TBD               | TBD                   | TBD                  | no               | Verification replay #1.                              |
| R3    | verification_full | pending | no           | no         | no                                           | TBD               | TBD                   | TBD                  | no               | Verification replay #2 (must be consecutive no-gap). |

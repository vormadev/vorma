# Mining Checklist

## Before Claiming

1. Ensure scaffold/governance baseline changes are committed before starting a
   new mining round.
2. Run `spec/tools/check_all.sh`.
3. Claim the next slot with `spec/tools/claim_lowest_open_slot.sh <owner>`.
4. Edit only the claimed package path under `spec/packages/**`.
5. In shared-checkout parallel mode, each agent still edits only one package
   path.
6. Use only allowed non-package files during mining:
    - `spec/MINING_DISPATCH.md` (script-managed)
    - `spec/DECISIONS.md` (append-only)
    - `spec/TRACEABILITY.md` (append-only)

## During Mining

1. Mine tests first for asserted behavior.
2. Mine source for implicit invariants not asserted in tests.
3. Establish ownership boundaries: own local behavior once, reference upstream
   behavior instead of redefining it.
4. In `20-requirements.md`, set `Ownership` to `OWNED|INHERITED|DELTA`.
5. For `INHERITED` or `DELTA`, provide non-empty `Upstream Requirement Refs`.
6. Add `REQ-*` IDs and evidence references with file and line numbers.
7. Keep assertion accounting counters accurate.
8. Move ambiguities to `spec/DECISIONS.md`.

## Before Marking DONE

1. Resolve all open questions in `70-open-questions.md`.
2. Confirm assertion invariants in `80-assertion-accounting.md`.
3. Ensure every `REQ-*` in `20-requirements.md` exists in `evidence.yaml`.
4. Run `spec/tools/check_all.sh`.
5. Mark completion with `spec/tools/mark_slot_done.sh SLOT-XXX <owner>`.

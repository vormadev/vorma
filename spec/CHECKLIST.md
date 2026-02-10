# Mining Checklist

## Before Claiming

1. Run `spec/tools/check_all.sh`.
2. Claim the next slot with `spec/tools/claim_lowest_open_slot.sh <owner>`.
3. Edit only the claimed package path under `spec/packages/**`.
4. In shared-checkout parallel mode, each agent still edits only one package
   path.

## During Mining

1. Mine tests first for asserted behavior.
2. Mine source for implicit invariants not asserted in tests.
3. Add `REQ-*` IDs and evidence references with file and line numbers.
4. Keep assertion accounting counters accurate.
5. Move ambiguities to `spec/DECISIONS.md`.

## Before Marking DONE

1. Resolve all open questions in `70-open-questions.md`.
2. Confirm assertion invariants in `80-assertion-accounting.md`.
3. Ensure every `REQ-*` in `20-requirements.md` exists in `evidence.yaml`.
4. Run `spec/tools/check_all.sh`.
5. Mark completion with `spec/tools/mark_slot_done.sh SLOT-XXX`.

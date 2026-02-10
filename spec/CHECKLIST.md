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
7. Ensure every requirement ID has a prose details section.
8. Keep assertion accounting counters accurate.
9. Do not use mermaid/diagram/chart blocks in package artifacts.
10. Move ambiguities to `spec/DECISIONS.md`.

## Review Passes (Required)

1. Record review pass 1 via `spec/tools/record_review_pass.sh`.
2. If pass 1 has findings (`FAIL_NOTES`), address notes and rerun review pass 1.
3. After pass 1 is `PASS_NO_NOTES`, record review pass 2 with a different
   reviewer.
4. If pass 2 has findings, address notes and rerun pass 1 and pass 2 on the new
   artifact hash.
5. A slot needs two independent `PASS_NO_NOTES` passes on the current artifact
   hash.

## Before Marking DONE

1. Resolve all open questions in `70-open-questions.md`.
2. Confirm assertion invariants in `80-assertion-accounting.md`.
3. Ensure every `REQ-*` in `20-requirements.md` exists in `evidence.yaml`.
4. Ensure review gate fields in `90-review-gate.md` reflect two independent
   `PASS_NO_NOTES` passes for current artifacts.
5. Run `spec/tools/check_all.sh`.
6. Mark completion with `spec/tools/mark_slot_done.sh SLOT-XXX <owner>`.

# Mining Checklist

## Before Claiming

1. Ensure scaffold/governance baseline changes are committed before starting a
   new mining round.
2. Run `go run ./spec/tools/cmd/check_all`.
3. Claim the next slot with
   `go run ./spec/tools/cmd/claim_lowest_open_slot <owner>`.
4. Edit only the claimed package path under `spec/packages/**`.
5. For parallel mining, use one isolated git worktree per agent; shared checkout
   is unsupported by guard checks.
6. Use only allowed non-package files during mining:
    - `spec/MINING_DISPATCH.md` (script-managed)
    - `spec/DECISIONS.md` (append-only)
    - `spec/TRACEABILITY.md` (append-only)

## During Mining

1. Mine tests first for asserted behavior.
2. Mine source for implicit invariants not asserted in tests.
3. Establish ownership boundaries: own local behavior once; reference upstream
   behavior for inherited/delta behavior.
4. Encode requirements in `spec.json` with `ownership` and
   `upstream_requirement_refs`.
5. For `INHERITED` or `DELTA`, provide non-empty `upstream_requirement_refs`.
6. Add `REQ-*` IDs and evidence refs with repository-relative path and line.
7. Ensure every requirement has non-empty `normative_statements`.
8. Keep assertion accounting ledger complete and classification-consistent.
9. Keep open questions resolved before done.
10. Do not add charts/diagram blocks in package artifacts.
11. Move cross-package ambiguities to `spec/DECISIONS.md` as `status = OPEN`
    rows.

## Review Passes (Required)

1. Record review pass 1 via `go run ./spec/tools/cmd/record_review_pass`.
2. If pass 1 has findings (`FAIL_NOTES`), address notes and rerun pass 1.
3. After pass 1 is `PASS_NO_NOTES`, record pass 2 with a different reviewer and
   different reviewer claim slot.
4. If pass 2 has findings, address notes and rerun pass 1 and pass 2 on the new
   artifact hash.
5. A slot needs two independent `PASS_NO_NOTES` passes on the current artifact
   hash.

## Before Marking DONE

1. Resolve all open questions in `spec.json`.
2. Confirm assertion accounting invariants in `spec.json`.
3. Ensure every `REQ-*` has both test and implementation evidence mappings.
4. Ensure review gate fields capture two independent `PASS_NO_NOTES` passes,
   with distinct reviewer owners and reviewer claim slots.
5. Resolve any `status = OPEN` decision rows that apply to the slot.
6. Run `go run ./spec/tools/cmd/check_all`.
7. Run `pnpm prettier --write <edited-md-and-json-files>`.
8. Mark completion with
   `go run ./spec/tools/cmd/mark_slot_done SLOT-XXX <owner>`.

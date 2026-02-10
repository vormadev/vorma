# Mining Checklist (Worker Agent)

## Before Claiming

1. Ensure scaffold/governance baseline changes are committed before starting a
   new mining round.
2. Run `go run ./spec/tools/cmd/check_all`.
3. Set a stable agent identity for this worker:
   `export SPEC_AGENT_ID=<unique-agent-id>`.
4. Claim the next slot with
   `go run ./spec/tools/cmd/claim_lowest_open_slot <owner>`.
5. Edit only the claimed package path under `spec/packages/**`.
6. For parallel mining, use one isolated git worktree per agent; shared
   checkout is unsupported by guard checks.
7. Use only allowed non-package files during mining:
   - `spec/MINING_DISPATCH.json` (script-managed)
   - `spec/DECISIONS.json` (append-only)
   - `spec/TRACEABILITY.json` (append-only)

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
9. Keep open questions resolved.
10. Do not add charts/diagram blocks in package artifacts.
11. Move cross-package ambiguities to `spec/DECISIONS.json` as `status = OPEN`
    rows.
12. Do not run `record_review_pass` or `mark_slot_done` for the slot you mined.
13. Do not claim extra slots for reviewer aliases or self-review.

## Stop and Handoff (Required)

1. Resolve all open questions in `spec.json`.
2. Confirm assertion accounting invariants in `spec.json`.
3. Ensure every `REQ-*` has both test and implementation evidence mappings.
4. Resolve any `status = OPEN` decision rows that apply to the slot.
5. Run `go run ./spec/tools/cmd/check_all`.
6. Run `pnpm prettier --write <edited-md-and-json-files>`.
7. Stop. Hand off slot/review status to independent reviewer agents.

## Independent Review and Completion (Other Agents)

1. Reviewer pass commands must run from the reviewer slot's claim
   branch/context/actor identity.
2. Pass 1 and pass 2 must come from different reviewer owners, claim slots, and
   claim actor identities.
3. Reviewer claim actor identities must differ from the mined slot claim actor.
4. Only after two `PASS_NO_NOTES` passes may slot owner run
   `go run ./spec/tools/cmd/mark_slot_done SLOT-XXX <owner>`.

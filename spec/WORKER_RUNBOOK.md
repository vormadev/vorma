# Worker Runbook

This is the single source of instructions for normative intent mining agents.

## Mission

Produce rebuild-grade normative specs with 100% assertion accounting,
evidence-backed requirements, and two independent zero-note review passes before
completion.

Naming rule: mirror existing repository directory names for spec package paths.
The only synthetic package name is `vormaroot`, which maps to `vorma.go`.

## Non-Negotiable Rules

1. Edit only files under `spec/**`.
2. Claim work from `spec/MINING_DISPATCH.json` using tooling, not manual row
   edits.
3. During mining, allowed files are:
    - `spec/packages/<claimed-path>/spec.json`
    - `spec/MINING_DISPATCH.json` (script-managed)
    - `spec/DECISIONS.json` (append-only)
    - `spec/TRACEABILITY.json` (append-only)
4. Edit only one claimed package path under `spec/packages/**`.
5. Follow `spec/OWNERSHIP_BOUNDARIES.md`: inherited behavior must reference
   upstream requirements instead of redefining them.
6. Every requirement must include ownership and normative statements.
7. `INHERITED` and `DELTA` requirements must include non-empty
   `upstream_requirement_refs`.
8. Every requirement must map to evidence with both test and implementation
   coverage.
9. `assertion_accounting` must satisfy:
    - every assertion row is classified as `MEANINGFUL` or `NON_MEANINGFUL`
    - `MEANINGFUL` rows map to one or more local requirements
    - `NON_MEANINGFUL` rows include no requirement mappings
    - derived unclassified assertion count is `0`.
10. One active `CLAIMED` slot per owner.
11. Only the slot owner may mark that slot `DONE`.
12. Do not mark a slot `DONE` unless all guard checks pass and both independent
    review passes are `PASS_NO_NOTES` on the current artifact hash.
13. Do not add charts/diagram blocks to package artifacts.
14. After editing any `spec/**/*.md` or `spec/**/*.json`, run Prettier:
    `pnpm prettier --write <files...>`.
15. `spec/DECISIONS.json` entries must use `status = OPEN|RESOLVED`; `OPEN`
    entries must keep `Selected Option`, `Rationale`, and `Evidence` as `-`.
16. Review passes must be recorded from the reviewer's claimed worktree/branch
    context; reviewers cannot share the miner claim context or each other’s
    claim context.

## Required Read Order

1. `AGENTS.md`
2. `spec/PROGRAM.md`
3. `spec/OWNERSHIP_BOUNDARIES.md`
4. `spec/ASSERTION_ACCOUNTING.md`
5. `spec/CHECKLIST.md`
6. `spec/MINING_DISPATCH.json`

## Workflow

1. Ensure scaffold/governance baseline is committed before first claim (the
   claim script enforces this when all slots are `OPEN`).

2. Claim the next slot:

```bash
export GOCACHE=${GOCACHE:-/tmp/go-build}
go run ./spec/tools/cmd/claim_lowest_open_slot <owner>
```

3. Find your claimed slot row in `spec/MINING_DISPATCH.json` and note `slot_id`
   and `spec_path`.

4. Edit only `spec/packages/<claimed-path>/spec.json`.

5. Run guard checks:

```bash
go run ./spec/tools/cmd/check_all
```

6. Record review passes (reviewers must differ from miner and from each other,
   and must provide distinct reviewer claim slots they own):

```bash
go run ./spec/tools/cmd/record_review_pass SLOT-XXX <reviewer_owner> <reviewer_claim_slot> 1 PASS_NO_NOTES -
go run ./spec/tools/cmd/record_review_pass SLOT-XXX <reviewer_owner_2> <reviewer_claim_slot_2> 2 PASS_NO_NOTES -
```

`record_review_pass` validates that the command is executed from the worktree
and branch context captured when the reviewer claim slot was created.

Use `FAIL_NOTES` with a notes reference when findings exist:

```bash
go run ./spec/tools/cmd/record_review_pass SLOT-XXX <reviewer_owner> <reviewer_claim_slot> 1 FAIL_NOTES spec/DECISIONS.json#DEC-0001
```

7. After both review passes are `PASS_NO_NOTES` and checks pass, mark slot done:

```bash
go run ./spec/tools/cmd/mark_slot_done SLOT-XXX <owner>
```

## Parallel Agent Model

- Use one isolated git worktree per agent.
- Shared checkout is unsupported by guard checks.
- Multiple `CLAIMED` slots are allowed.
- Claims and done updates are serialized by a dispatch lock in tooling.
- Each agent still edits only one package path per patch.

## Completion Output

At handoff, report:

1. `slot_id`
2. `spec_path`
3. files edited
4. unresolved decisions/questions
5. review pass status
6. `go run ./spec/tools/cmd/check_all` result

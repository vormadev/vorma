# Worker Runbook

This is the single source of instructions for normative intent mining agents.

## Mission

Produce rebuild-grade normative specs with 100% assertion accounting and
evidence-backed requirements, while editing only `spec/**`.

Naming rule: mirror existing repository directory names for spec package paths.
The only synthetic package name is `vormaroot`, which maps to `vorma.go`.

## Non-Negotiable Rules

1. Edit only files under `spec/**`.
2. Claim work from `spec/MINING_DISPATCH.md` using tooling, not manual row
   edits.
3. During mining, allowed files are:
    - `spec/packages/<claimed-path>/**`
    - `spec/MINING_DISPATCH.md` (script-managed)
    - `spec/DECISIONS.md` (append-only)
    - `spec/TRACEABILITY.md` (append-only)
4. Edit only one claimed package path under `spec/packages/**`.
5. Follow `spec/OWNERSHIP_BOUNDARIES.md`: inherited behavior must reference
   upstream requirements instead of redefining them.
6. Every requirement in `20-requirements.md` must map in `evidence.yaml`.
7. Every mapped requirement must include both test and implementation evidence.
8. `20-requirements.md` rows must include:
    - `Ownership` value in `OWNED|INHERITED|DELTA`
    - non-empty `Upstream Requirement Refs` for `INHERITED` and `DELTA`
9. `80-assertion-accounting.md` must satisfy:
    - `mapped_meaningful_assertions = meaningful_assertions`
    - `unclassified_assertions = 0`
10. One active `CLAIMED` slot per owner.
11. Only the slot owner may mark that slot `DONE`.
12. Do not mark a slot `DONE` unless guard checks pass.

## Required Read Order

1. `AGENTS.md`
2. `spec/PROGRAM.md`
3. `spec/OWNERSHIP_BOUNDARIES.md`
4. `spec/ASSERTION_ACCOUNTING.md`
5. `spec/CHECKLIST.md`
6. `spec/MINING_DISPATCH.md`

## Workflow

1. Ensure scaffold/governance baseline is committed before first claim (the
   claim script enforces this when all slots are `OPEN`).

2. Claim the next slot:

```bash
spec/tools/claim_lowest_open_slot.sh <owner>
```

3. Find your claimed slot row in `spec/MINING_DISPATCH.md` and note `slot_id`
   and `spec_path`.

4. Edit only files under your claimed `spec_path`.

5. Run guard checks:

```bash
spec/tools/check_all.sh
```

6. If checks pass, mark your slot done:

```bash
spec/tools/mark_slot_done.sh SLOT-XXX <owner>
```

## Parallel Agent Model

- Shared checkout is supported.
- Multiple `CLAIMED` slots are allowed.
- Claims and done updates are serialized by a dispatch lock in tooling.
- Each agent must still edit only one package path per patch.

## Completion Output

At handoff, report:

1. `slot_id`
2. `spec_path`
3. files edited
4. unresolved decisions/questions
5. `spec/tools/check_all.sh` result

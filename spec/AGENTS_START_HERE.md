# Agents Start Here

This file is a pointer map plus a high-level program-sequencing reminder. Keep
policy details in governance/package docs.

## 0) Bigger Picture Process (Order Matters)

1. Finish all normative intent mining across the package universe to
   rebuild-grade completeness.
2. After mining is complete, scrutinize resulting specs for poor design or
   accidental complexity; improve the specs where needed. Backward compatibility
   is not a concern at this stage (pre-1.0), and intentional breaking changes to
   end-user code are acceptable.
3. Only after specs are comprehensive and design-tuned, start writing and/or
   migrating legacy tests into `/conformance`.
4. Then review any remaining legacy tests that are not in `/conformance` and
   confirm whether they are intentionally non-conformance tests (for example,
   internal unit tests) or should be migrated.
5. Once conformance harnesses are solid, proceed with aggressive refactoring.

Priority hard gate:

- Do not work on lower-priority tiers while any P0 package path remains
  incomplete.
- Specifically: no opportunistic P1/P2 placeholder cleanup while P0 is still
  open, unless the user explicitly asks for that override in the current
  session.
- While the program is in step 1 (normative intent mining), edit only `spec/**`.
  Do not edit implementation/test/config files outside `spec/**` while step 1 is
  incomplete. No exceptions.

Rebuild-grade standard (hard requirement):

- Treat package specs as target artifacts for source-free reimplementation.
- A package is "done" only when replay/no-gap is achieved at rebuild-grade
  granularity.
- If replay/no-gap is achieved below rebuild-grade detail, treat it as
  provisional stability only (not closure).
- Keep checklist closure items open until the package spec is detailed enough to
  recreate behavior from spec alone.
- Hard fail test: if a developer could not rebuild the system correctly from the
  spec artifacts alone, it is NOT done.

## 1) Program Rules (Read First)

- `spec/SPEC_GOVERNANCE.md`

## 2) Package Universe

- `spec/packages/PACKAGE_INDEX.md`

## 3) Current Session Snapshot

- `spec/ROUGH_STATUS_UPDATE.md`

## 3a) Parallel Dispatch (Shared Checkout)

- For parallel agent chats in one checkout, claim work in
  `spec/MINING_DISPATCH.md` before editing package artifacts.
- Claim rule: take the lowest-numbered `OPEN` slot, set it to `CLAIMED`, and add
  agent stamp + UTC timestamp.
- One chat holds one slot at a time.
- Coordinator selection is deterministic: the chat holding the lowest-numbered
  `CLAIMED` slot is coordinator.
- If no slot is `CLAIMED`, coordinator is unset until the next claim.
- Worker chats edit only: `spec/packages/<claimed-package>/**` and that slot row
  in `spec/MINING_DISPATCH.md`.
- Coordinator-only files: `spec/ROUGH_STATUS_UPDATE.md`,
  `spec/MINING_DISPATCH.md` policy text (non-row sections),
  `spec/AGENTS_START_HERE.md`, and `spec/SPEC_GOVERNANCE.md`.

## 4) Work In One Package At A Time

Open the target package directory from:

- `spec/packages/<package-path>/`

Then use only that package's canonical files:

- `SPEC.md`
- `TRACEABILITY_MATRIX.md`
- `SPEC_CHECKLIST.md`
- `FULL_AUDIT_TRACKER.md`
- `NORMATIVE_INTENT_LEDGER.md`
- `CONFORMANCE_ISSUES.md`

## 5) Handoff Discipline

- Treat `spec/ROUGH_STATUS_UPDATE.md` as disposable scratch state.
- On each update cycle, delete and recreate `spec/ROUGH_STATUS_UPDATE.md` from
  scratch with only what currently matters.
- Do not append, diff-edit, or preserve prior narrative structure from the
  existing file.
- Keep policy changes in `spec/SPEC_GOVERNANCE.md`, not here.
- In parallel mode, release your slot in `spec/MINING_DISPATCH.md` at handoff:
  mark `OPEN`, clear claim fields, and leave a one-line progress note.

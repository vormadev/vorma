# Spec Program Governance

Status: Active Current Phase: Step 1 (normative intent mining) Scope: `spec/**`
only

## 1) Authority

This file is the authoritative policy for Step 1. If any other doc appears to
conflict, follow this file. If ambiguity remains, stop and ask the user.

## 2) Core Term

`rebuild-from-scratch-from-spec-alone` means:

An engineer can implement package behavior using only that package's spec
artifacts under `spec/packages/<package>/`, without consulting implementation
source, and with enough detail to recreate behavior from scratch.

This is a local program definition. Agents must not assume any weaker meaning.

## 3) Hard Phase Gate

- Step 1 remains active until the user explicitly says to move to Step 2.
- During Step 1, edits outside `spec/**` are prohibited.
- Prohibited non-spec edits include implementation, tests, config, tooling, and
  build files.

## 4) Evidence Rules

Allowed evidence input:

- Implementation source files.
- Legacy tests outside `conformance/**` (optional corroboration only).

Forbidden evidence input:

- Any file under `conformance/**`.

## 5) Allowed vs Forbidden Actions (Step 1)

Allowed:

- Read any repository files as mining evidence.
- Edit only `spec/**`.
- Reconcile findings in spec artifacts.

Forbidden:

- Any edit outside `spec/**`.
- Any implementation bug fix/refactor/behavior change.
- Treating "resolve issue" as permission to change non-spec code.

## 6) No-Gap Full Round (Normative Definition)

A round is `no-gap` only if all are true:

1. From-scratch replay: re-derived from evidence, not trusted from existing spec
   text.
2. Full-scope replay: every in-scope file in `NORMATIVE_INTENT_LEDGER.md` is
   covered.
3. Replay explicitly checks whether current spec is missing normative behavior
   required for rebuild-from-scratch-from-spec-alone.
4. Missing normative requirement count is `0`.
5. Incorrect normative-claim count is `0`.
6. Blocking intent-validation issue count is `0` after the round.

If any condition fails, the round is not `no-gap`.

Plain-language equivalent:

- "No-gap round" means you attempted, from scratch and in full, to mine all
  normative intent to rebuild-from-scratch-from-spec-alone level and found
  nothing missing from the existing spec.

## 7) Execution Model (No Light Rounds)

1. Author package specs to rebuild-from-scratch-from-spec-alone detail.
2. Run baseline full replay (gaps allowed).
3. Reconcile all baseline findings in `spec/**` artifacts only.
4. Run verification full replay #1 and require `no-gap`.
5. Run verification full replay #2 and require `no-gap` consecutively.

There is no rough/light replay stage.

## 8) Required Package Artifacts (Minimal)

Each package path in `spec/packages/PACKAGE_INDEX.md` must contain exactly:

- `SPEC.md`
- `SPEC_CHECKLIST.md`
- `NORMATIVE_INTENT_LEDGER.md`
- `TRACEABILITY_MATRIX.md`
- `CONFORMANCE_ISSUES.md`

## 9) Package Completion Criteria (`DONE`)

A package may be set to `DONE` in `PACKAGE_INDEX.md` only when all are true:

1. `SPEC.md` is rebuild-from-scratch-from-spec-alone complete.
2. `TRACEABILITY_MATRIX.md` is fully reconciled (or issue-backed).
3. `NORMATIVE_INTENT_LEDGER.md` records two consecutive `no-gap` full rounds
   using section 6 criteria.
4. `CONFORMANCE_ISSUES.md` has no open blocking intent-validation issues.
5. `SPEC_CHECKLIST.md` is fully complete in strict sequence.

## 10) Writing Constraints

- Forward-state only. No replay timeline/history narrative.
- Repo-relative paths only.
- Claims must be precise, testable, and evidence-linked.

## 11) Parallel Shared Checkout

- `spec/MINING_DISPATCH.md` is lock state only.
- `OPEN` and `CLAIMED` are mutex states, not completion states.
- A worker edits only its claimed package and its own dispatch row.

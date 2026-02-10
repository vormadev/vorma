# Rebuild-Grade Spec Program

## Objective

Extract complete normative intent from the legacy system into rebuild-grade
specs that are sufficient to re-implement the full system with redesigned
internals and no behavioral regression against intended user outcomes.

## Ordered Scope

Mining and spec authoring order is fixed:

1. `vorma` surfaces using mirrored repo paths: `vormaroot` (`vorma.go`),
   `vormabuild`, `vormaruntime`, `vormaclient/*`
2. `wave`
3. `kit/*`
4. `lab/*`
5. `bootstrap/create`

## Phase Model

### Phase 1: Normative Intent Mining (active)

Required outputs:

- `spec/packages/**/spec.json` package artifacts.
- 100% assertion accounting with explicit disposition for every legacy
  assertion.
- Evidence-backed requirements with repository-relative path + line references.
- Explicit ownership boundaries with inheritance by reference.
- Two independent zero-note review passes per package before completion.

Phase 1 status is tracked in `spec/PHASE_STATUS.json`.

### Phase 2: To-Be Rebuild Spec

Blocked until all Phase 1 gates are satisfied.

### Phase 3: Implementation and Conformance

Blocked until To-Be spec acceptance.

## Slot Lifecycle

Dispatch status values:

- `OPEN`: no owner, not yet claimed.
- `CLAIMED`: active mining/review. Slot remains `CLAIMED` until all required
  review gates are satisfied.
- `DONE`: allowed only after all package gates pass.

## Hard Gates for Phase 1 Completion

1. Assertion accounting is 100% complete for all legacy assertions.
2. Meaningful assertion mapping is 100% complete.
3. Non-meaningful assertions include explicit rationale.
4. Derived unclassified assertion count is `0` for every package.
5. Every normative requirement has evidence from tests and implementation.
6. Normative ownership is unambiguous: one owner per behavior; inherited/delta
   behavior references upstream requirements instead of duplicate restatement.
7. No unresolved contradiction decisions (`status = OPEN`) across package specs.
8. No package marked `DONE` has unresolved open questions.
9. No package marked `DONE` lacks two independent review passes with
   `PASS_NO_NOTES` on the current artifact hash.

## JSON Package Artifact Contract

Each package path under `spec/packages/**` contains exactly one artifact file:

- `spec.json`

`spec.json` is both human-readable and machine-validated. It is the single
source for scope, requirements, state model, error model, external contracts,
nonfunctional requirements, open questions, assertion accounting, evidence, and
review gate state.

Required requirement fields:

- `id`: `REQ-<PACKAGE>-NNNN`
- `priority`
- `ownership`: `OWNED | INHERITED | DELTA`
- `upstream_requirement_refs` (non-empty for `INHERITED` and `DELTA`)
- `status`
- `normative_statements` (non-empty)

Required evidence rules:

- `evidence.requirements.<REQ-ID>` exists for every requirement.
- Every requirement has both `test` and `implementation` evidence entries.
- Every evidence reference uses repository-relative `path` and positive `line`.

## Review Model

Each package requires:

1. Primary mining pass by slot owner.
2. Independent review pass 1.
3. Independent review pass 2 by a different reviewer.
4. Each reviewer must supply a reviewer claim slot they own.
5. Reviewer claim slots must differ from the mined slot and from each other.
6. Both passes must be `PASS_NO_NOTES` on the current artifact hash.
7. Only then may the mined slot be marked `DONE`.

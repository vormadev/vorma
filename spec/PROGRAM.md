# Rebuild-Grade Spec Program

## Objective

Extract complete normative intent from the legacy system into a specification
set that is sufficient for a full system rebuild with reworked internals and no
behavioral regressions against intended user outcomes.

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

Output of this phase:

- `spec/packages/**` package artifacts with requirement IDs and evidence.
- Assertion accounting with explicit disposition for every legacy assertion.
- Resolved contradictions and explicit decision records.
- Two independent review passes with zero findings for every package before
  completion.

Phase 1 status is tracked in `spec/PHASE_STATUS.md`.

### Phase 2: To-Be Rebuild Spec

Blocked until all Phase 1 gates are satisfied.

### Phase 3: Implementation and Conformance

Blocked until To-Be spec acceptance.

## Readability and Structure Standard

Package artifacts must be human-readable first, machine-checkable second.

- Do not use mermaid/diagram/chart blocks in package artifacts.
- Use prose sections for explanations.
- Keep a compact machine-checkable index for requirements and status fields.
- Every normative requirement must have a detailed prose section.

## Slot Lifecycle

Dispatch status stays `OPEN`, `CLAIMED`, or `DONE`.

- `OPEN`: no owner, not yet claimed.
- `CLAIMED`: active mining and review loop. The slot remains `CLAIMED` until it
  satisfies both independent review passes.
- `DONE`: only allowed after all gates pass.

## Hard Gates for Phase 1 Completion

1. Assertion accounting is `100%` complete for all legacy assertions.
2. Meaningful assertion mapping is `100%` complete.
3. Non-meaningful assertions have explicit rationale.
4. Unclassified assertions are `0`.
5. Every normative requirement has evidence from tests and source code.
6. Normative ownership is unambiguous: each behavior has exactly one owner
   package and all inherited behavior is referenced, not duplicated.
7. No unresolved contradictions across package specs.
8. No package marked `DONE` has unresolved open questions.
9. No package is marked `DONE` without two independent review passes with
   `PASS_NO_NOTES` against the exact current package artifact hash.

## Requirement and Evidence Rules

- Normative statements use IDs: `REQ-<PACKAGE>-NNNN`.
- Evidence links use IDs: `EVID-<PACKAGE>-<TYPE>-NNNN`.
- Evidence must cite repository-relative paths and line numbers.
- Do not include machine-specific absolute paths in committed spec content.
- Use single-owner requirement boundaries from `spec/OWNERSHIP_BOUNDARIES.md`.
- Cross-package inheritance must be represented via upstream requirement
  references, not copied requirement text.
- Each requirement index row must include `Ownership` as `OWNED`, `INHERITED`,
  or `DELTA`.
- `INHERITED` and `DELTA` requirements must include non-empty upstream
  requirement references.

## Package Artifact Contract

Each package path under `spec/packages/**` must include:

- `00-scope.md`
- `10-capabilities.md`
- `20-requirements.md`
- `30-state-model.md`
- `40-errors.md`
- `50-external-contracts.md`
- `60-nonfunctional.md`
- `70-open-questions.md`
- `80-assertion-accounting.md`
- `90-review-gate.md`
- `evidence.yaml`

## Review Model

Each package requires:

1. Primary mining pass.
2. Independent review pass 1.
3. Independent review pass 2 by a different reviewer.
4. Both passes must report `PASS_NO_NOTES` for the current artifact hash.
5. Only then may the slot be marked `DONE`.

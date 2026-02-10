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

Phase 1 status is tracked in `spec/PHASE_STATUS.md`.

### Phase 2: To-Be Rebuild Spec

Blocked until all Phase 1 gates are satisfied.

### Phase 3: Implementation and Conformance

Blocked until To-Be spec acceptance.

## Hard Gates for Phase 1 Completion

1. Assertion accounting is `100%` complete for all legacy assertions.
2. Meaningful assertion mapping is `100%` complete.
3. Non-meaningful assertions have explicit rationale and reviewer sign-off.
4. Unclassified assertions are `0`.
5. Every normative requirement has source evidence from tests and source code.
6. No unresolved contradictions across package specs.
7. No package marked `DONE` has unresolved open questions.

## Requirement and Evidence Rules

- Normative statements use IDs: `REQ-<PACKAGE>-NNNN`.
- Evidence links use IDs: `EVID-<PACKAGE>-<TYPE>-NNNN`.
- Evidence must cite repository-relative paths and line numbers.
- Do not include machine-specific absolute paths in committed spec content.

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
- `evidence.yaml`

## Review Model

Each package requires:

1. Primary mining pass.
2. Independent traceability and contradiction review pass.
3. Gate checks before status changes to `DONE`.

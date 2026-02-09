# Vorma Spec Workflow (Sub-1.0)

Status: Draft  
Last Updated: 2026-02-08  
Applies To: Creation, update, review, and tracking of Vorma specs before 1.0

## 1. Why This Spec Exists

This document defines a lightweight process for evolving Vorma specs while the
framework is still pre-1.0.

It exists to:

- keep specs useful for large refactors and conformance testing,
- preserve proposal/final visibility,
- avoid process overhead and historical-document buildup.

## 2. Conformance Boundaries

### 2.1 Normative Terms

The terms **MUST** and **MUST NOT** are normative.

### 2.2 Scope

This spec governs spec workflow and tracking only. Runtime/build/frontend/wire
behavior is defined in their domain specs.

## 3. Sub-1.0 Operating Model

### PROC-MODE-001: Fast-First Mode

Until Vorma reaches 1.0, spec workflow MUST prioritize speed and clarity over
formal governance ceremony.

### PROC-MODE-002: Minimal Status Model

Spec workflow status MUST map onto:

- `Proposed`
- `Final`
- `Dropped`

Transition note:

- legacy `Draft` MAY be treated as equivalent to `Proposed` while existing
  specs are gradually normalized.

### PROC-MODE-003: In-Place Evolution

Behavior/spec changes MUST be made directly in the current spec files (in
place). Separate long-lived RFC history documents are not required.

### PROC-MODE-004: Current Truth Over Historical Archive

When a proposal is superseded or completed, outdated proposal text, temporary
alternatives, and obsolete process notes SHOULD be deleted instead of archived.

### PROC-MODE-005: Proposal vs Final Visibility

At any point, proposal/final state MUST be visible from:

- the per-spec `Status:` header, and
- checklist/traceability artifacts.

## 4. Required Tracking Artifacts

### PROC-TRACK-001: Checklist Is Work-Progress Canon

`specs/packages/vorma/VORMA_SPEC_CHECKLIST.md` MUST remain the canonical
record of done vs remaining spec work.

### PROC-TRACK-002: Traceability Matrix Is Coverage Canon

`specs/packages/vorma/VORMA_TRACEABILITY_MATRIX.md` MUST remain the
canonical requirement-to-scenario/test coverage record.

### PROC-TRACK-003: Conformance Issues Are Divergence Canon

`specs/packages/vorma/VORMA_CONFORMANCE_ISSUES.md` MUST track known
spec/implementation divergences and bug candidates.

### PROC-TRACK-004: New Requirement IDs Need Matrix Rows

When new conformance-domain requirement IDs are added (`BR-*`, `WIRE-*`,
`BUILD-*`, `FE-*`), corresponding traceability rows MUST be added in the same
change set.

## 5. Lightweight Change Workflow

### PROC-FLOW-001: Proposal Entry

New or changed behavior contracts MUST first be written in specs with
testable normative wording and `Status: Proposed` where appropriate.

### PROC-FLOW-002: Finalization Criteria

A proposal is ready for `Final` when all are true:

- normative behavior is explicit and testable,
- traceability rows exist,
- known implementation divergence is either resolved or logged as an open
  conformance issue.

### PROC-FLOW-003: No Extra Ceremony Requirement

For sub-1.0 work, separate governance artifacts (meeting notes, approval logs,
or standalone RFC thread docs) MUST NOT be required to finalize spec changes.

### PROC-FLOW-004: Cleanup on Finalization

When promoting content to `Final`, obsolete proposal alternatives and temporary
process scaffolding SHOULD be removed in the same or immediate follow-up change.

## 6. Requirement ID Rules

### PROC-ID-001: Prefix Families

Requirement IDs MUST use domain prefixes (for example `BR-*`, `WIRE-*`,
`BUILD-*`, `FE-*`, `API-*`, `PROC-*`).

### PROC-ID-002: No Semantic Repurposing

An existing requirement ID MUST NOT be repurposed to mean a different contract.

### PROC-ID-003: Append-Only Numbering

Within a prefix family, new IDs SHOULD be appended monotonically instead of
reindexing older IDs.

### PROC-ID-004: Replace-or-Remove Sync Rule

When requirements are replaced or removed, checklist/traceability/conformance
issue artifacts MUST be updated in the same change set. Historical tombstones
are optional during sub-1.0.

## 7. Post-1.0 Placeholder

### PROC-V1-001: Governance Tightening May Be Introduced at 1.0

After 1.0, Vorma MAY adopt stronger governance (for example historical RFC
retention and stricter compatibility process). Those policies are out of scope
for this sub-1.0 workflow spec.

## 8. Relation to Other Specs

- Public API map: `specs/packages/vorma/VORMA_PUBLIC_API_SURFACE_SPEC.md`
- Testing strategy:
  `specs/packages/vorma/VORMA_TESTING_STRATEGY_SPEC.md`
- Traceability matrix:
  `specs/packages/vorma/VORMA_TRACEABILITY_MATRIX.md`
- Conformance issues:
  `specs/packages/vorma/VORMA_CONFORMANCE_ISSUES.md`
- Checklist: `specs/packages/vorma/VORMA_SPEC_CHECKLIST.md`

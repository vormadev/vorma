# Vorma Spec Template and RFC Process Specification

Status: Draft  
Last Updated: 2026-02-07  
Applies To: Creation, update, review, and governance of Vorma specifications

## 1. Why This Spec Exists

This document standardizes how Vorma specs are authored and changed.

It exists to:

- keep specs structurally consistent,
- make behavior changes traceable and reviewable,
- ensure specs remain usable as test-generation sources.

## 2. Conformance Boundaries

### 2.1 Normative Terms

The terms **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

### 2.2 Scope

This spec governs process and format for spec documents. It does not define
runtime/build/frontend behavior itself.

## 3. Spec Document Classification Model

### PROC-CLASS-001: Spec Classes

Vorma spec documents MUST be explicitly classed as one of:

- **Conformance spec**: normative requirement catalog for black-box behavior,
- **Supporting reference**: dependency map/context doc that informs conformance,
- **Process/governance spec**: meta-rules for evolving spec system.

### PROC-CLASS-002: Class Declaration

Each spec SHOULD declare class intent near document header/scope section.

### PROC-CLASS-003: Conformance Priority

When process decisions conflict between reference notes and conformance specs,
conformance spec requirements take precedence.

## 4. Required Spec Template

### PROC-TPL-001: Required Header Fields

Every spec MUST include:

- title,
- status,
- last-updated date,
- applies-to scope line.

### PROC-TPL-002: Required Top-Level Sections

Conformance-oriented specs SHOULD include at least:

1. why spec exists,
2. conformance boundaries,
3. terminology (if needed),
4. requirement catalog,
5. conformance test guidance,
6. relation-to-other-specs links.

### PROC-TPL-003: Requirement ID Format

Normative requirements MUST use stable IDs with domain prefix, e.g.:

- `BR-*`
- `WIRE-*`
- `BUILD-*`
- `FE-*`
- `SEC-*`
- `TEST-*`
- `OBS-*`
- `PERF-*`
- `REL-*`
- `TERM-*`
- `PROC-*`
- `VER-*`
- `API-*`

### PROC-TPL-004: Requirement Wording Pattern

Normative requirements SHOULD use scenario format:

- Given …
- When …
- Then …

or an equivalent explicit condition-action-outcome structure.

### PROC-TPL-005: One Requirement per ID

Each requirement ID MUST represent one logically atomic contract statement.

### PROC-TPL-006: Stable IDs Over Time

Existing requirement IDs MUST NOT be repurposed for different semantics.

If semantics change materially, add new ID and deprecate old ID.

### PROC-TPL-007: Absolute Path References

Cross-spec references SHOULD use absolute workspace paths for unambiguous
navigation.

## 5. RFC and Change Lifecycle

## 5.1 Status Model

### PROC-RFC-001: Status Values

Spec status values are:

- `Draft`
- `Proposed`
- `Accepted`
- `Deprecated`
- `Superseded`

### PROC-RFC-002: Draft Entry Rule

New specs and substantial rewrites MUST enter as `Draft` unless explicitly
approved for direct promotion.

### PROC-RFC-003: Accepted Promotion Criteria

A spec SHOULD move to `Accepted` when:

- requirement language is sufficiently testable,
- traceability plan exists for key requirements,
- review concerns are resolved or explicitly tracked.

### PROC-RFC-004: Supersession Rule

When a spec is superseded, it MUST point to replacement doc(s) and preserve
history context.

## 5.2 Change Types and Required Actions

### PROC-CHANGE-001: Editorial Change

Editorial-only changes (wording/typos/clarity without semantic change) MAY skip
full RFC flow but SHOULD still update date if meaningful.

### PROC-CHANGE-002: Behavioral Clarification Change

Clarification that narrows ambiguity without changing expected behavior SHOULD:

- update relevant requirement text,
- confirm no traceability remap required.

### PROC-CHANGE-003: Behavioral Contract Change

Behavioral contract changes MUST include:

- requirement ID update/new IDs,
- impacted spec cross-links,
- testing-strategy traceability update,
- compatibility/release impact note.

### PROC-CHANGE-004: Breaking-Sensitive Change Annotation

If change affects public compatibility surface, update MUST call out breaking
risk and required migration/deprecation handling.

## 5.3 Review and Approval Workflow

### PROC-REVIEW-001: Required Review Dimensions

Spec review SHOULD evaluate:

- technical correctness,
- testability (black-box feasibility),
- compatibility impact,
- ambiguity/resolution quality,
- traceability impact.

### PROC-REVIEW-002: Open Questions Tracking

Unresolved design questions SHOULD be listed explicitly in the spec PR/discussion
thread until resolved.

### PROC-REVIEW-003: No Orphan Behavioral Changes

Behavior-changing code PRs SHOULD NOT merge without corresponding spec update
when affected behavior is in scope of normative docs.

## 6. Requirement ID Management Rules

### PROC-ID-001: Prefix Ownership

Each domain prefix SHOULD be owned by one primary spec to reduce accidental ID
collisions.

### PROC-ID-002: Sequential Monotonic IDs

Within a prefix family, new IDs SHOULD be appended monotonically rather than
reindexing existing IDs.

### PROC-ID-003: Deprecation Annotation

Deprecated requirements SHOULD remain in document history with clear deprecation
annotation instead of silent deletion where traceability matters.

### PROC-ID-004: Cross-Spec Link Integrity

If a requirement ID is moved/replaced, all known cross-references SHOULD be
updated in same change set.

## 7. Spec-to-Test Traceability Process

### PROC-TRACE-001: Test Strategy Alignment

Conformance specs MUST remain compatible with
`/Users/sjc/__code/river/specs/VORMA_TESTING_STRATEGY_SPEC.md` traceability
expectations.

### PROC-TRACE-002: Coverage Lifecycle

New requirement IDs SHOULD be accompanied by initial coverage status (`planned`,
`partial`, `implemented`) in project traceability artifacts.

### PROC-TRACE-003: Removed Requirement Handling

If a requirement is removed/superseded, associated tests MUST be updated or
explicitly marked obsolete.

## 8. Document Hygiene Rules

### PROC-HYGIENE-001: Keep Spec Self-Contained

Specs SHOULD avoid requiring deep implementation source diving to understand
contract intent.

### PROC-HYGIENE-002: Distinguish Normative vs Guidance

Normative requirements and non-blocking guidance MUST be clearly separated.

### PROC-HYGIENE-003: Avoid Overfitting to Current Implementation

Specs MUST express externally observable behavior and invariants, not current
private call graph details.

### PROC-HYGIENE-004: Keep Supporting Docs Secondary

Supporting reference docs MUST NOT become the only source for critical
conformance requirements.

## 9. Standard Spec Skeleton (Reference)

Recommended skeleton:

1. Title / Status / Last Updated / Applies To  
2. Why This Spec Exists  
3. Conformance Boundaries  
4. Terminology (optional)  
5. Requirement Catalog (`PREFIX-*`)  
6. Conformance Test Guidance  
7. Relation to Other Specs

## 10. Relation to Other Specs

- Domain terminology: `/Users/sjc/__code/river/specs/VORMA_DOMAIN_MODEL_TERMINOLOGY_SPEC.md`
- Versioning/compatibility: `/Users/sjc/__code/river/specs/VORMA_VERSIONING_COMPATIBILITY_SPEC.md`
- Public API map: `/Users/sjc/__code/river/specs/VORMA_PUBLIC_API_SURFACE_SPEC.md`
- Testing strategy: `/Users/sjc/__code/river/specs/VORMA_TESTING_STRATEGY_SPEC.md`
- Checklist: `/Users/sjc/__code/river/specs/SPECS_CHECKLIST.md`

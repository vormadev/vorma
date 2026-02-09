# Spec Governance Rules

Status: Active  
Last Updated: 2026-02-09  
Scope: Cross-package governance for `specs/packages/**`.

## 1. Ownership Rule

- Each package path owns its own spec and tracking artifacts under `specs/packages/<package-path>/`.
- Consumer packages reference owner specs; they do not duplicate owner internals.

## 2. Required Per-Package Artifacts

Each package directory MUST contain:

- `SPEC.md`
- `SPEC_CHECKLIST.md`
- `FULL_AUDIT_TRACKER.md`
- `NORMATIVE_INTENT_LEDGER.md`
- `TRACEABILITY_MATRIX.md`
- `CONFORMANCE_ISSUES.md`

## 3. Mining Rule

- Normative intent mining MUST use both source and existing tests.
- Every file row in package ledger tracks `passes` and `clean_passes` per epoch.

## 4. Trust Epoch Rule

- Verified state is valid only within current trust epoch.
- Trust reset invalidates prior closure claims.

## 5. Program-Level Docs

Top-level docs are rollups only:

- `specs/packages/PACKAGE_INDEX.md`
- `specs/SPECS_CHECKLIST.md`
- `specs/SPEC_FULL_AUDIT_TRACKER.md`
- `specs/NORMATIVE_INTENT_MINING_LEDGER.md`

Top-level docs MUST NOT contain package-owned requirement inventories.

## 6. Path Hygiene

- Personal absolute filesystem paths MUST NOT appear in committed spec docs.
- Use repo-relative paths.

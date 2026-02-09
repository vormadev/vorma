# Spec Program Governance

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`  
Scope: Cross-package governance for `specs/packages/**`.

This is the single canonical top-level specs program document.

## Canonical Sources

- Package index: `specs/packages/PACKAGE_INDEX.md`
- Package-local checklists: `specs/packages/<package-path>/SPEC_CHECKLIST.md`
- Package-local full-audit trackers: `specs/packages/<package-path>/FULL_AUDIT_TRACKER.md`
- Package-local normative ledgers: `specs/packages/<package-path>/NORMATIVE_INTENT_LEDGER.md`
- Package-local traceability matrices: `specs/packages/<package-path>/TRACEABILITY_MATRIX.md`
- Package-local conformance issues: `specs/packages/<package-path>/CONFORMANCE_ISSUES.md`

## Program State

- Package-path structure established.
- Independent tracking artifacts established per package path.
- From-scratch replay in progress (epoch `E2`).
- No package closure claimed yet.

## Program Checklist

- [x] Package-path canonical index exists: `specs/packages/PACKAGE_INDEX.md`
- [x] Package-path ownership model is active (`specs/packages/<package-path>/`).
- [x] Every package has independent spec/checklist/audit/ledger/matrix/issues files.
- [x] Intent mining requires implementation source plus legacy tests outside `conformance/**`, with per-file `passes`/`clean_passes`.
- [x] Old shared out-of-scope ledger removed.
- [x] Vorma-family owner split is active (`BR-*` in `vormaruntime`, `BUILD-*` in `vormabuild`, `FE-*` in `vormaclient/client`; `vorma` kept wrapper-level only).
- [ ] P0 closure: `vorma`, `vormabuild`, `vormaruntime`, `vormaclient/client`, `wave`, `wave/tooling`, `kit/matcher`, `kit/mux`, `kit/response`, `kit/validate`, `kit/headels`, `lab/tsgen`, `lab/viteutil`.
- [ ] P1 closure: remaining `kit/*`, `lab/*`, and `vormaclient/*` package paths.
- [ ] P2 closure: `bootstrap` and `vormaclient/create` package paths (bootstrap-related, explicitly lowest priority).
- [ ] Program stop condition met for every package.

## Program Stop Condition

Program closure requires every package path in `specs/packages/PACKAGE_INDEX.md` to satisfy:

1. Detailed package requirement catalog is fully authored (not placeholder/high-level-only).
2. Requirement-level traceability is reconciled (coverage complete or active issue-backed exceptions).
3. Every in-scope file row mined in current epoch.
4. Two consecutive full no-gap rounds recorded.
5. Package checklist/tracker/matrix/issues reconciled.

## Governance Rules

### 1. Ownership Rule

- Each package path owns its own spec and tracking artifacts under `specs/packages/<package-path>/`.
- Consumer packages reference owner requirement IDs; they do not duplicate owner behavior (internal implementation details or owner public/external contracts).

### 2. Required Per-Package Artifacts

Each package directory MUST contain:

- `SPEC.md`
- `SPEC_CHECKLIST.md`
- `FULL_AUDIT_TRACKER.md`
- `NORMATIVE_INTENT_LEDGER.md`
- `TRACEABILITY_MATRIX.md`
- `CONFORMANCE_ISSUES.md`

### 3. Mining Rule

- Normative intent mining MUST use implementation source plus legacy tests outside `conformance/**`.
- `conformance/**` test suites are verification outputs/evidence and MUST NOT be treated as normative mining inputs.
- Full-audit replay MUST re-validate existing spec claims against source + legacy tests; prior spec text is not automatically trusted.
- Every file row in package ledger tracks `passes` and `clean_passes` per epoch.

### 3a. Pass Taxonomy Rule

- Package audit passes MUST distinguish `rough pass` and `full pass`.
- `Rough pass` means initial mining/reconciliation sufficient to replace placeholders and surface gaps; it is not full closure.
- `Full pass` means detailed requirement catalog + requirement-level traceability + issue reconciliation across all in-scope files.
- Package checklists MUST include separate checklist items for rough-pass completion and full-pass completion.
- Full-pass checklist items MUST NOT be marked complete while requirement-level traceability remains partial/source-only without explicit issue reconciliation.

### 4. Trust Epoch Rule

- Verified state is valid only within current trust epoch.
- Trust reset invalidates prior closure claims.

### 5. Top-Level Rule

- Top-level docs are program rollups only.
- Top-level docs MUST NOT contain package-owned requirement inventories.

### 6. Path Hygiene

- Personal absolute filesystem paths MUST NOT appear in committed spec docs.
- Use repo-relative paths.

### 7. Correctness-First Cleanup Rule

- Incorrect or unsupported normative claims MUST be deleted or rewritten immediately.
- Specs, checklists, and traceability matrices MUST contain only current, correct requirements.
- `CONFORMANCE_ISSUES.md` MUST track only active implementation-vs-spec gaps; do not retain historical wrong-claim narrative.
- Behavior that appears accidental, ambiguous, or weakly evidenced MUST be recorded as an active intent-validation issue in `CONFORMANCE_ISSUES.md` until confirmed or removed.
- Open intent-validation issues count as open gaps and MUST block no-gap round closure for that package.

### 8. Full-Audit Definition (Normative)

For this program, a "full audit" means revalidating all existing package spec content, not only running more mining passes.

Required sequence per package:

1. Re-derive normative intent from implementation source and legacy tests outside `conformance/**`.
2. Author/expand a detailed requirement catalog that reflects mined behavior (not a rough summary).
3. Validate every current normative claim (including previously marked verified claims) against that intent.
4. Delete or rewrite any incorrect, stale, ambiguous, or unsupported claim immediately.
5. Reconfirm boundary ownership (owner behavior stays in owner specs; consumer specs reference owner requirement IDs instead of duplicating internal or public/external owner behavior).
6. Reconcile package artifacts for consistency: `SPEC.md`, `SPEC_CHECKLIST.md`, `TRACEABILITY_MATRIX.md`, `CONFORMANCE_ISSUES.md`, `NORMATIVE_INTENT_LEDGER.md`, and `FULL_AUDIT_TRACKER.md`.
7. Only after steps 1-6 are complete, update per-file `passes` and `clean_passes` for the round.

### 9. Vorma Family Ownership Split

- `vormaruntime`, `vormabuild`, and `vormaclient/client` are canonical owners for detailed package semantics in their domains.
- `vorma` remains wrapper/facade scope only (public composition/boundary contracts and owner references).
- Authors MUST NOT re-centralize owner catalogs under `specs/packages/vorma/`.

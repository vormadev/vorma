# Spec Program Governance

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`  
Scope: Cross-package governance for `spec/packages/**`.

This is the single canonical top-level specs program document.

## Canonical Sources

- Package index: `spec/packages/PACKAGE_INDEX.md`
- Package-local checklists: `spec/packages/<package-path>/SPEC_CHECKLIST.md`
- Package-local full-audit trackers:
  `spec/packages/<package-path>/FULL_AUDIT_TRACKER.md`
- Package-local normative ledgers:
  `spec/packages/<package-path>/NORMATIVE_INTENT_LEDGER.md`
- Package-local traceability matrices:
  `spec/packages/<package-path>/TRACEABILITY_MATRIX.md`
- Package-local conformance issues:
  `spec/packages/<package-path>/CONFORMANCE_ISSUES.md`

## Program State

- Package-path structure established.
- Independent tracking artifacts established per package path.
- From-scratch replay in progress (epoch `E2`).
- No package closure claimed yet.

## Program Checklist

- [x] Package-path canonical index exists: `spec/packages/PACKAGE_INDEX.md`
- [x] Package-path ownership model is active (`spec/packages/<package-path>/`).
- [x] Every package has independent spec/checklist/audit/ledger/matrix/issues
      files.
- [x] Intent mining uses implementation source and incorporates legacy tests
      outside `conformance/**` when present, with per-file
      `passes`/`clean_passes`.
- [x] Old shared out-of-scope ledger removed.
- [x] Vorma-family owner split is active (`BR-*` in `vormaruntime`, `BUILD-*` in
      `vormabuild`, `FE-*` in `vormaclient/client`; `vorma` kept wrapper-level
      only).
- [ ] P0 closure: `vorma`, `vormabuild`, `vormaruntime`, `vormaclient/client`,
      `wave`, `wave/tooling`, `kit/matcher`, `kit/mux`, `kit/response`,
      `kit/validate`, `kit/headels`, `lab/tsgen`, `lab/viteutil`.
- [ ] P1 closure: remaining `kit/*`, `lab/*`, and `vormaclient/*` package paths.
- [ ] P2 closure: `bootstrap` and `vormaclient/create` package paths
      (bootstrap-related, explicitly lowest priority).
- [ ] Program stop condition met for every package.

Priority execution rule (hard gate):

- While any P0 package checklist/tracker is not at stop criterion, agents MUST
  execute only P0 package-path work.
- P1/P2 work MUST NOT be started opportunistically (for example, placeholder
  cleanup convenience) until P0 closure is complete.
- The only exception is explicit user direction in the current session to
  override priority sequencing.

## Program Stop Condition

Program closure requires every package path in `spec/packages/PACKAGE_INDEX.md`
to satisfy:

1. Detailed package requirement catalog is fully authored (not
   placeholder/high-level-only).
2. Requirement-level traceability is reconciled (coverage complete or active
   issue-backed exceptions).
3. Every in-scope file row mined in current epoch.
4. Two consecutive full no-gap rounds recorded.
5. Package checklist/tracker/matrix/issues reconciled.

## Governance Rules

### 1. Ownership Rule

- Each package path owns its own spec and tracking artifacts under
  `spec/packages/<package-path>/`.
- Consumer packages reference owner requirement IDs; they do not duplicate owner
  behavior (internal implementation details or owner public/external contracts).

### 2. Required Per-Package Artifacts

Each package directory MUST contain:

- `SPEC.md`
- `SPEC_CHECKLIST.md`
- `FULL_AUDIT_TRACKER.md`
- `NORMATIVE_INTENT_LEDGER.md`
- `TRACEABILITY_MATRIX.md`
- `CONFORMANCE_ISSUES.md`

### 3. Mining Rule

- Normative intent mining MUST use implementation source.
- Legacy tests outside `conformance/**` are optional corroborating evidence when
  they exist in-repo.
- Package artifacts MUST NOT cite files under `conformance/**` as current
  evidence references.
- When no legacy tests outside `conformance/**` exist for a package, record
  source-only evidence state in matrix/ledger notes.
- Absence of legacy tests outside `conformance/**` by itself MUST NOT be tracked
  as an intent-validation issue.
- Full-audit replay MUST re-validate existing spec claims against source +
  available legacy tests; prior spec text and prior test claims are not
  automatically trusted.
- Every file row in package ledger tracks `passes` and `clean_passes` per epoch.

### 3a. Pass Taxonomy Rule

- Package audit passes MUST distinguish `rough pass` and `full pass`.
- `Rough pass` means initial mining/reconciliation sufficient to replace
  placeholders and surface gaps; it is not full closure.
- `Full pass` means detailed requirement catalog + requirement-level
  traceability + issue reconciliation across all in-scope files.
- Package checklists MUST include separate checklist items for rough-pass
  completion and full-pass completion.
- Full-pass checklist items MUST NOT be marked complete while requirement-level
  traceability remains partial/source-only without explicit issue
  reconciliation.
- Package checklist execution is strictly ordered; later checklist items MUST
  NOT be marked complete while any earlier checklist item remains unchecked.
- In particular,
  `Resolve open intent-validation issues in CONFORMANCE_ISSUES.md` is a
  late-stage closure gate and MUST remain unchecked until preceding full-pass
  checklist items are complete.

### 3b. Priority Sequencing Rule

- Program execution MUST follow P0 -> P1 -> P2 sequencing.
- Presence of easy/quick placeholder work in lower tiers is not a valid reason
  to skip unfinished higher-tier package paths.
- If uncertain, agents MUST re-check the Program Checklist P0 status before
  selecting the next package path.

### 3c. Mining-Only Edit Gate

- While the program is in step 1 (normative intent mining), agents MUST edit
  only files under `spec/**`.
- During step 1, agents MUST NOT edit implementation, test, or config/source
  files outside `spec/**`.
- In step 1, `Resolve open intent-validation issues in CONFORMANCE_ISSUES.md`
  means evidence/state reconciliation in package spec artifacts, not runtime
  implementation changes.
- While step 1 is incomplete, edits outside `spec/**` are prohibited with no
  exceptions.

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

- Incorrect or unsupported normative claims MUST be deleted or rewritten
  immediately.
- Specs, checklists, and traceability matrices MUST contain only current,
  correct requirements.
- `CONFORMANCE_ISSUES.md` MUST track only active implementation-vs-spec gaps; do
  not retain historical wrong-claim narrative and do not use it to represent
  missing/deleted conformance evidence.
- Behavior that appears accidental, ambiguous, or weakly evidenced MUST be
  recorded as an active intent-validation issue in `CONFORMANCE_ISSUES.md` until
  confirmed or removed.
- Open intent-validation issues count as open gaps and MUST block no-gap round
  closure for that package.

### 7a. Forward-State Writing Rule

- Package artifacts MUST describe current enforceable state only.
- Do not include migration/change history narrative, deletion timelines, or
  prior-state retrospectives in package artifacts.
- Provenance belongs in git history, not in normative package content.

### 8. Full-Audit Definition (Normative)

For this program, a "full audit" means revalidating all existing package spec
content, not only running more mining passes.

Required sequence per package:

1. Re-derive normative intent from implementation source and available legacy
   tests outside `conformance/**` (if present).
2. Author/expand a detailed requirement catalog that reflects mined behavior
   (not a rough summary).
3. Validate every current normative claim against that intent.
4. Delete or rewrite any incorrect, stale, ambiguous, or unsupported claim
   immediately.
5. Reconfirm boundary ownership (owner behavior stays in owner specs; consumer
   specs reference owner requirement IDs instead of duplicating internal or
   public/external owner behavior).
6. Reconcile package artifacts for consistency: `SPEC.md`, `SPEC_CHECKLIST.md`,
   `TRACEABILITY_MATRIX.md`, `CONFORMANCE_ISSUES.md`,
   `NORMATIVE_INTENT_LEDGER.md`, and `FULL_AUDIT_TRACKER.md`.
7. Only after steps 1-6 are complete, update per-file `passes` and
   `clean_passes` for the round.

### 9. Vorma Family Ownership Split

- `vormaruntime`, `vormabuild`, and `vormaclient/client` are canonical owners
  for detailed package semantics in their domains.
- `vorma` remains wrapper/facade scope only (public composition/boundary
  contracts and owner references).
- Authors MUST NOT re-centralize owner catalogs under `spec/packages/vorma/`.

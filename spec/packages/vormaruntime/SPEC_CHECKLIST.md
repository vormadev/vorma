# vormaruntime Spec Checklist

Status: Active  
Last Updated: 2026-02-09

- [x] Package spec file exists (`SPEC.md`).
- [x] Package traceability matrix exists (`TRACEABILITY_MATRIX.md`).
- [x] Package conformance issues file exists (`CONFORMANCE_ISSUES.md`).
- [x] Package full-audit tracker exists (`FULL_AUDIT_TRACKER.md`).
- [x] Package intent ledger exists (`NORMATIVE_INTENT_LEDGER.md`).
- [ ] Canonical backend-runtime requirement catalog imported into owner package.
- [ ] Revalidate imported catalog against source + available legacy tests
      outside `conformance/**` (when present).
- [ ] Complete rough structural pass.
- [ ] Complete rough boundary pass.
- [ ] Complete rough semantic pass.
- [ ] Complete full requirement catalog authoring (detailed, comprehensive,
      non-summary).
- [ ] Complete full requirement-level traceability reconciliation (coverage
      complete or issue-backed exceptions).
- [ ] Complete at least one explicit full structural pass (all in-scope source
      files replayed in current epoch).
- [ ] Complete at least one explicit full boundary pass (owner inheritance vs
      runtime-owned behavior revalidated across all files).
- [ ] Complete at least one explicit full semantic pass (existing claims
      revalidated and new gaps incorporated where found).
- [ ] Resolve active implementation-divergence issues in `CONFORMANCE_ISSUES.md`
      (active `VCI-*` backlog).
- [ ] Record two consecutive no-gap full rounds after latest requirement-catalog
      change.

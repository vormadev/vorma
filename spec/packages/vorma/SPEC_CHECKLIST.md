# vorma Spec Checklist

Status: Active  
Last Updated: 2026-02-09

- [x] Package spec file exists (`SPEC.md`).
- [x] Package traceability matrix exists (`TRACEABILITY_MATRIX.md`).
- [x] Package conformance issues file exists (`CONFORMANCE_ISSUES.md`).
- [x] Package full-audit tracker exists (`FULL_AUDIT_TRACKER.md`).
- [x] Package intent ledger exists (`NORMATIVE_INTENT_LEDGER.md`).
- [x] Redundant `VORMA_*` layer removed from this package path.
- [x] Wrapper package scope reduced to `vorma.go`-owned behavior only.
- [x] Revalidate wrapper requirement catalog against source + legacy tests outside `conformance/**`.
- [x] Reconcile wrapper traceability row status/coverage.
- [x] Complete full structural pass.
- [x] Complete full boundary pass.
- [x] Complete full semantic pass.
- [ ] Resolve open intent-validation issues in `CONFORMANCE_ISSUES.md`.
- [x] Record two consecutive no-gap full rounds (`E2-R2`, `E2-R3`).

# vorma Spec Checklist

Status: Active  
Last Updated: 2026-02-09

- [x] Package spec file exists (`SPEC.md`).
- [x] Package traceability matrix exists (`TRACEABILITY_MATRIX.md`).
- [x] Package conformance issues file exists (`CONFORMANCE_ISSUES.md`).
- [x] Package full-audit tracker exists (`FULL_AUDIT_TRACKER.md`).
- [x] Package intent ledger exists (`NORMATIVE_INTENT_LEDGER.md`).
- [ ] Redundant `VORMA_*` layer removed from this package path.
- [ ] Wrapper package scope reduced to `vorma.go`-owned behavior only.
- [ ] Revalidate wrapper requirement catalog against source + available legacy
      tests outside `conformance/**` (when present).
- [ ] Reconcile wrapper traceability row status/coverage.
- [ ] Complete full structural pass.
- [ ] Complete full boundary pass.
- [ ] Complete full semantic pass.
- [ ] Reconcile `CONFORMANCE_ISSUES.md` to active package-level gaps only
      (legacy-test absence recorded as source-only evidence state in
      matrix/ledger).
- [ ] Record two consecutive no-gap full rounds (`E2-R2`, `E2-R3`).

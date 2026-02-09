# Spec Checklist

Rule: strict sequence. Do not check item N+1 while item N is unchecked.

- [ ]   1. Define package boundary and entry points in `SPEC.md`.
- [ ]   2. Author requirement catalog to rebuild-from-scratch-from-spec-alone
       detail in `SPEC.md`.
- [ ]   3. Map requirements in `TRACEABILITY_MATRIX.md`.
- [ ]   4. Populate in-scope file inventory in `NORMATIVE_INTENT_LEDGER.md`.
- [ ]   5. Run baseline full replay (from-scratch + full-scope +
       rebuild-from-scratch-from-spec-alone check).
- [ ]   6. Reconcile all baseline findings in `spec/**` artifacts only.
- [ ]   7. Run verification full replay #1 and satisfy strict `no-gap` criteria.
- [ ]   8. Run verification full replay #2 and satisfy strict `no-gap` criteria
       consecutively.
- [ ]   9. Pass acceptance check: an independent engineer could rebuild this
       package from scratch using only this package's spec artifacts.
- [ ]   10. Mark package `DONE` in `spec/packages/PACKAGE_INDEX.md`.

Note: checklist work in Step 1 never authorizes code edits outside `spec/**`.

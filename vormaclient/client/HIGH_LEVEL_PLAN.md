# vormaclient/client High-Level Plan

Purpose: keep the long-term sequence explicit so takeover is safe and effort
does not drift.

## End State

- Maintainable client runtime with clear module boundaries and no accidental
  behavior regressions.
- Strict first-principles contract suite is the source of truth.
- Legacy suites removed only after explicit signoff.

## Sequence

- [x]   1. Test dedup + strengthen contracts
    - Consolidate duplicate setup/assertion patterns into shared harness
      helpers.
    - Keep assertions strict and first-principles.
- [x]   2. Add navigation model/state-machine tests
    - Add sequence-driven invariants over `navigate/prefetch/revalidate/submit`.
    - Prioritize order/race-sensitive regressions.
- [ ]   3. Expand race-focused regressions (**YOU ARE HERE**)
    - Cover stale/aborted resolution classes and server/client payload mismatch
      classes comprehensively.
- [ ]   4. Continue aggressive internal refactor cleanup
    - Reduce complexity and fragmentation while keeping behavior pinned by
      tests.

## Always-On Rules

- If strict test fails and behavior is objectively incorrect, fix source code.
- Do not weaken tests to mirror suspicious implementation quirks.
- Ask the user immediately only for truly ambiguous product behavior.
- Keep legacy tests in-tree with `READY_TO_DELETE_AFTER_SIGNOFF` until explicit
  deletion signoff.

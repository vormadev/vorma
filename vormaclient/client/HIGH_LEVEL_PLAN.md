# vormaclient/client High-Level Plan

Purpose: keep the sequence explicit so takeover is safe and work does not drift.

## End State

- Maintainable client runtime with clear boundaries and lower internal
  complexity.
- Strict first-principles tests as the only behavioral source of truth.
- No legacy-test dependency and no compatibility cruft.

## Sequence

- [x]   1. Test dedup + strengthen contracts
- [x]   2. Add navigation model/state-machine coverage
- [x]   3. Add race-focused regressions
- [ ]   4. Continue aggressive internal refactor cleanup (**YOU ARE HERE**)
    - 2026-02-11: navigation/begin-navigation/link/redirect internals moved to
      shared primitives to remove duplicated branch trees.
    - 2026-02-11: shared `resolveAbsoluteHref` primitive now replaces repeated
      `new URL(..., window.location.href).href` normalization paths across
      runtime/begin-navigation/links/history/redirect parsing.
    - 2026-02-11: runtime branch handling moved to explicit discriminated
      `aborted/redirect/success` control flow.
    - 2026-02-11: runtime/global safety hardening added for optional init-order
      state and cross-realm body detection.
    - 2026-02-11: redirect parsing/execution moved to shared parser+executor
      primitives.
    - 2026-02-11: render-runtime now has explicit unit coverage for downstream
      client-loader abort propagation after upstream non-abort loader failures.
    - 2026-02-11: history runtime now has explicit POP fallback coverage for
      both failed client-nav reload path and successful cross-document POP state
      synchronization.
    - 2026-02-11: history hard-reload fallback now no-ops in JSDOM so tests stay
      deterministic while browser runtime behavior remains unchanged.
    - 2026-02-11: submit stale-checkpoint coverage now pins all reachable async
      dedupe race windows, and one unreachable redirect stale-checkpoint branch
      was removed from runtime submit flow.
    - 2026-02-11: runtime slot matching removed an impossible nullable prefetch
      lookup branch, and unit coverage now pins no-op `removeNavigation`
      behavior for absent keys.
    - 2026-02-11: runtime submit flow now has explicit coverage for impossible
      missing-response defensive handling and full runtime branch coverage.
    - 2026-02-11: render-runtime now has added contracts for explicit
      `scrollToTop: false` behavior, empty-title fallback, and null head-array
      normalization.
    - 2026-02-11: render-runtime loader execution now normalizes inputs once and
      uses consistent normalized loader state when deriving client-loader error
      indices, with strict malformed-data contracts added.
    - 2026-02-11: link click/prefetch internals simplified; impossible nullable
      `NavigationControl.promise` branch removed and duplicate idle-prefetch
      guard removed.
    - 2026-02-12: begin-navigation prefetch dedupe reuse paths are now pinned by
      strict unit coverage (active-navigation reuse and pending-revalidation
      reuse).
    - 2026-02-12: fetch-route-data defensive behavior now has strict unit
      coverage for build-id fallback in server-data handoff, sparse
      partial-match invariant enforcement before client-loader startup,
      production dep preloading that ignores falsy dep entries, and
      unseeded-cache client-loader behavior when current snapshots do not
      contain cached data.
    - 2026-02-12: contract coverage now explicitly enforces the
      runtime/buildtime API boundary for `route` (absent from runtime entry,
      present in buildtime entry).
    - 2026-02-12: matcher contracts in skip checks now fail fast on malformed
      matcher output (sparse matches and empty route-pattern entries) rather
      than silently continuing.

## Current Validation Gate

- `pnpm oxlint vormaclient/client/src`
- `pnpm tsc --noEmit --project vormaclient/client`
- `pnpm tsgo --noEmit --project vormaclient/client`
- `pnpm vitest --run vormaclient/client/src`
- `pnpm vitest --run vormaclient/client/src --coverage --coverage.reporter=text-summary`

## Non-Negotiable Rules

- Tests must assert first-principles-correct behavior only.
- If strict test fails and behavior is objectively wrong, fix production code.
- Do not weaken tests to mirror implementation quirks.
- For genuinely ambiguous behavior, ask the user immediately.
- Do not add back-compat adapters while sub-1.0 unless explicitly requested.

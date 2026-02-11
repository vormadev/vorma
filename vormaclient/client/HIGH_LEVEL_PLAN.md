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
    - 2026-02-11: link click/prefetch internals simplified; impossible nullable
      `NavigationControl.promise` branch removed and duplicate idle-prefetch
      guard removed.

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

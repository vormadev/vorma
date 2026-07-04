# packages/vorma TS thermo-nuclear findings (P016) — held for batched Phase D-end triage

Same disposition as the matcher/tasks/build/vorma findings tickets: one batched maintainer
ruling at Phase D's end, across every crate/package's release-quality pass together. Full
analyses and file paths in
`docs/maintainer/fable/packets/P016-ts-package-release-quality/` (REPORT.md, once placed
by Fable).

1. **`_index.ts` ("Public Types"/"Constants" section) exports items `_index.ts` itself
   never forwards — dead public-looking surface, one hop from actually being published.**
   `packages/vorma/core/create_client_core.ts` has explicit `/////// Public Types` and
   `/////// Constants` section markers (its own stated intent) whose contents include:
   `ClientCore`, `MAX_SCROLL_ENTRIES`, `REFRESH_MAX_AGE_MS`, `MAX_REDIRECTS`,
   `MAX_REVALIDATION_RETRIES`, `REVALIDATION_BACKOFF_BASE_MS`,
   `REVALIDATION_BACKOFF_CAP_MS`, `REVALIDATION_DEBOUNCE_MS`. None of these eight are
   re-exported by `packages/vorma/core/_index.ts` (the actual `vorma/__internal` entry
   point every adapter is built from), so none reach any published package export.
   `ClientCore` and `MAX_SCROLL_ENTRIES`/`MAX_REDIRECTS` are genuinely consumed — by the
   test suite, importing `create_client_core.ts` directly as a relative source file (never
   through the package's own `exports` map) — confirming the export from
   `create_client_core.ts` itself is load-bearing and correctly placed; the bug is
   specifically that `_index.ts`'s barrel silently drops them one hop up, despite the
   sibling module's own section comments stating they were meant to be forwarded. Verified
   via a one-off compiler-API reachability scan (every export of `create_client_core.ts`
   checked against every export `_index.ts` actually resolves to, past alias chains)
   during this packet; the finding is exhaustive for this specific file, not a sample.
   Fix: either forward all eight from `_index.ts` (if genuinely intended as `__internal`
   surface, matching the section markers' own claim) or delete the section markers and
   narrow visibility if they were never meant to escape `create_client_core.ts` — a
   maintainer call on which reading is correct, since both are defensible depending on
   whether `vorma/__internal` is meant to be a complete mirror of
   `create_client_core.ts`'s own "public" section or a hand-curated subset.

2. **Two structurally identical dev-time cross-bundle globals, typed inconsistently.**
   `packages/vorma/core/create_client_core.ts` exposes two distinct mechanisms for the
   app's own compiled bundle to be reached from OUTSIDE code injected separately (the dev
   HMR preamble and the dev-refresh WebSocket script,
   `crates/vorma/src/refresh_script.js`): `window.__vorma_hmr_view_update` is properly
   typed via a `declare global { interface Window { __vorma_hmr_view_update?: ... } }`
   augmentation (bottom of the file); the structurally identical
   `Symbol.for("vorma-data-revalidate-fn")` mechanism (used by `refresh_script.js`'s
   `revalidate_client` message handler to trigger a client revalidation) is instead
   written with two `as any` casts
   (`(window as any)[Symbol.for("vorma-data-revalidate-fn")] = revalidate;` at the end of
   `create_client_core`) and carries no type declaration anywhere. Both mechanisms solve
   the same problem (a dev-tooling script outside the app's own module graph needs to call
   into it); only one is given the type safety the codebase's own sibling pattern already
   demonstrates. No behavior implication — purely a consistency/type-safety gap. Fix
   candidates: extend the same `declare global` augmentation to cover the symbol-keyed
   property (TypeScript supports well-known-symbol index signatures), or — simpler —
   switch to a plain `__vorma_`-prefixed string property matching the HMR sibling exactly,
   if there was never a real reason for the Symbol-keyed indirection (no evidence of one
   found in this packet's read of both call sites).

3. **`create_client_core.ts` is a single ~1565-line function body (lines 137-1702 of a
   1711-line file) — watch only, not recommended for action.** Considered seriously per
   the thermo-nuclear skill's "be ambitious" instruction before being set aside: the file
   already demonstrates the correct decomposition pattern extensively (revalidation
   scheduling, submissions, route-module loading, client-loader orchestration, and history
   position are ALL already extracted into their own factory-function modules with
   encapsulated state, composed together inside this function) — the remaining body is
   overwhelmingly genuine composition/glue plus a handful of algorithms
   (`report_build_skew`/`report_route_build_skew`, `history_state_for_fetch`,
   `to_route_update_reason`) that close over enough shared closure state
   (`client_build_id`, `route_snapshot`, `user_on_build_skew_detected`, etc.) that
   extracting them would mean threading that state through an explicit `deps` object the
   same way the already-extracted modules do — plausible, but a genuinely marginal
   improvement rather than a clear win, and every remaining section is already delimited
   by the file's own `/////// Section` comment convention. Same disposition as P011's
   `matcher.rs`-616-lines and P015's `execution_engine.rs`/`runtime_app.rs` watch-only
   findings: recorded, no action recommended unless it keeps growing.

Cleared with reasoning recorded during the packet (not re-litigable as oversights): ~40
uses of `any`/`as any` across `core/` sampled and traced — all either the necessary
framework-to-UI-library erasure point (`component: (props: any) => any` in
`ViewDefinition`/`ClientOptions`, matched by the properly-typed generic layer above it in
`ToDefineViewArgs`), generic-union destructuring TypeScript's structural narrowing
genuinely cannot express without a cast (`api_client.ts`/`url.ts`'s `args as any`,
downstream fields immediately re-typed narrowly), or a documented, feature-detected
platform-API access (`(document as any).startViewTransition`) — the one exception is
finding 2 above. Preact's adapter genuinely diverges from React/Solid in its whole
reactive model (`@preact/signals`-backed, `HookReturnMode: "signal"`, every stateful hook
returning `ReadonlySignal<T>` rather than a plain value) — not a superficial difference,
verified via a full file diff against `react.tsx`, and documented as such rather than
assumed "basically the same" from a shallow top-level-symbol comparison (an earlier pass
in this same packet made exactly that wrong assumption before the full diff corrected it —
kept as a note for future agents: symbol-name diffs are not sufficient evidence of
behavioral parity between adapters).

Disposition when triaged: accepted rows become granted packets; rejected rows get the
ruling recorded here and this ticket closes.

## Fable recommendations (2026-07-02, Phase D triage — maintainer ruling pending)

Findings 1 and 2 ACCEPTED under Fable's remit (`vorma/__internal` is explicitly
uncontracted namespace; the typing fix is type-level only) — to land as one micro-packet:
forward all eight exports; type the revalidate global keeping the Symbol key, with the
packet free to choose the cleanest sound TypeScript encoding or report back if none beats
the status quo. Finding 3: watch-only stands (no action proposed). Ticket closes when the
micro-packet lands.

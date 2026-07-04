# vorma + vorma-client-wasm thermo-nuclear findings (P015) — held for batched Phase D-end triage

P015's findings-mode review (2026-07-02) surfaced the findings below, none implemented
(public-surface or maintainer-decision-shaped). Same triage plan as the sibling tickets
(`matcher-release-quality-findings`, `tasks-release-quality-findings`,
`build-release-quality-findings`): hold per-crate surface-shape findings and present them
to the maintainer once, together, at Phase D's end. Full analyses in
`docs/maintainer/fable/packets/P015-vorma-release-quality/REPORT.md`.

1. **`app!` macro rejects bare local state-type names (empirically proven footgun with a
   verified fix).** `vorma::app!(mod app for AppState)` with a bare type name fails to
   compile (`cannot find type AppState in this scope`) even when `AppState` is declared
   directly above the call; a `use`-imported bare name and a `self::`-qualified path fail
   identically; only crate-root-anchored paths (`crate::AppState`) work. Root cause: the
   macro substitutes `$state:ty` into a generated nested `mod`, and path resolution for
   the substituted tokens runs relative to that generated module, not the call site. Every
   in-repo caller (Board, the crate's own tests) already writes `crate::...` by unwritten
   convention. Verified fix (compiles, checked in isolation during P015): anchor the
   caller's tokens at the macro's top-level expansion point — emit
   `type __VormaAppState = $state;` beside the generated module and reference
   `super::__VormaAppState` inside it — restoring bare-name support with zero change for
   existing qualified-path callers. Not landed: changes the public macro's expansion
   shape. Interim mitigation landed by P015: the constraint is documented on `app!`'s
   rust-doc — per AGENTS.md's no-doc-smoothed-footguns rule, that is not the final answer;
   the macro fix is. Location: `crates/vorma/src/lib.rs` (`macro_rules! app`).
    - **RESOLVED (P020, 2026-07-02):** landed exactly as verified —
      `type __VormaAppState = $state;` emitted beside the generated module,
      `super::__VormaAppState` referenced inside it in place of every direct `$state`
      substitution, in `crates/vorma/src/lib.rs` (`macro_rules! app`). Bare local names,
      `use`-imported names, and `self::`-qualified paths all compile now, pinned
      red-to-green in `crates/vorma/tests/app_declaration_state_path_forms.rs`; every
      existing `crate::`-anchored caller (Board, the crate's own test suites,
      `tests/framework/src/scenario.rs`) recompiled unchanged. One real edge case surfaced
      by the doctest gate and fixed correctly rather than worked around: a `mod` declared
      inside a function body is not a child of that function's local item scope in Rust's
      module tree (confirmed with an isolated repro), so the anchor alias is unreachable
      via `super::` from a generated module when `app!` itself is invoked inside a
      function body — this only ever surfaced through rustdoc's own doctest wrapping (the
      "Getting started" `view!`/`resource!` field-reference example, the one
      `app!`-bearing doctest in the file with no runnable body of its own and thus no
      self-declared `fn main`), never through any real call site, since `app!` is
      module-scope-only in every actual caller in this repo. Fixed by giving that one
      doctest its own trivial `fn main() {}`, matching every sibling `app!` doctest's
      existing shape. The interim rust-doc warning is replaced with a plain statement of
      supported forms. Full evidence in
      `docs/maintainer/fable/packets/P020-app-macro-state-path-fix/REPORT.md`.
2. **`DocumentBuildIdentity` family is dead production surface.**
   `DocumentBuildIdentity`/`DocumentBuildIdentityAttribute`/`DocumentBuildIdentityElement`
   plus `Document::__build_identity()` (~90 lines in
   `crates/vorma/src/document_builder.rs`, `#[doc(hidden)] pub` via
   `build_interface::contracts`) have zero consumers in `vorma-build` (exhaustive grep: no
   reference anywhere in that crate). The build's real document-identity path is
   `build_root_document_hash_source()` → the private serde-based
   `DocumentHashSource`/`DocumentHashAttribute`/`DocumentHashElement` family in the same
   file — a structurally parallel borrowed-view family over the same three contract types.
   Only consumer of the identity family: `crates/vorma/tests/public_api.rs` (two tests
   asserting attribute flags). Code-judo: delete the identity family and re-point those
   two tests at the hash-source JSON path or a widened `into_contract()`; or, if it is
   intended future build surface, record why so this stops looking like cruft.
3. **Ctx-type delegation triplication — analyzed and REJECTED for unification** (recorded
   against re-litigation). `state`/`request`/`exec_ctx`/`public_url`/`param`/
   `splat_values`/`redirect`/`redirect_with_status` are one-line delegations repeated
   across `MiddlewareCtx`/`ResourceCtx`/`ViewCtx` in `crates/vorma/src/static_route.rs`
   (18 method instances over 6 shared names). The logic is not duplicated — all forward to
   the single `TypedHandlerContext` — and the divergence axes are deliberate type-safety
   boundaries (exit type `HttpExit` vs `ViewExit`; response-handle richness; `head()`
   presence: three independent axes). A generic `RouteCtx<..., Exit, Handle>` or shared
   trait would thread three semantic modes through one body while deleting no actual logic
   — the thermo-nuclear skill's own anti-pattern (same conclusion class as P011's
   flat/nested DFS-walk rejection).

Watch-only observations (no action recommended): `execution_engine.rs` 2755 lines (~1228
impl + ~1527 tests) and `runtime_app.rs` 2028 (~616 impl + ~1412 tests) — both
test-dominated with reasonably-sized implementation cores, semantics-frozen;
`middleware.rs` 1322 lines, cleanly section-delimited, ~even impl/test split.

Cleared with evidence (recorded against re-litigation): `HandlerExecutionError` as the
single canonical internal error funnel with typed entry adapters (`ViewExit`/`HttpExit`/
`StaticRouteError`/`InputError` all collapse into it — intended architecture, not
duplication); the repeated `ResponseEffects` mutex-lock idiom was the one genuine
shared-util gap and was consolidated by P015 itself (`lock_effects` in
`response_finalizer.rs`, byte-identity verified); `asset_body_provider.rs`'s cache lock is
a distinct lock and stays separate; `vorma-client-wasm`'s static output buffer is a
deliberate single-threaded-WASM pattern, now documented.

Disposition when triaged: accepted rows become granted packets; rejected rows get the
ruling recorded here and this ticket closes.

## Fable recommendations (2026-07-02, Phase D triage — maintainer ruling pending)

Finding 1 resolved by P020 (recorded above). Finding 3 (ctx delegation) rejection RATIFIED
— deliberate type-safety boundaries, recorded against re-litigation. Watch-onlys: no
action proposed. Finding 2 (`DocumentBuildIdentity` deletion) remains pending the
maintainer (triage Q2 — the one fact only the maintainer holds: whether it is intended
future surface); on a delete ruling it becomes a micro-packet and this ticket closes.

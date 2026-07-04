# P015 Report

Provenance note: the executor (Sonnet 5 subagent, 2026-07-02; session interrupted once by
an accidental machine restart and resumed with state verified intact) delivered this
report in its final message per the anti-truncation convention; Fable placed it (HTML
transport escaping undone).

## What changed

Fifteen files authored by this packet (+873/−135 against HEAD, excluding pre-existing
uncommitted P019/P010 content sharing the working tree):

crates/vorma (14 files): `lib.rs` crate-root rewritten to a full teaching entry point
(getting-started doctest; the parallel-execution contract stated as contract; errors/exits
client-visibility doctrine; tasks framing; `app!` documented in full with the complete
field grammar verified against `vorma-macros/src/app_decl.rs`, a passing doctest, and the
newly discovered path-qualification constraint — Finding 1); every crate-root re-export
doc raised to a teaching pointer including the `ResourceKind` inference rule (GET/HEAD →
Query, else Mutation — verified against `framework_graph.rs::default_kind` and stated
where users meet it); `HtmlAttribute`/`SafeHtml` with corrected escaping claims; both deny
attributes (`missing_docs` predated, stated honestly; intra-doc-link deny newly landed).
Module docs + type docs to the bar across `exit.rs` (the full P008 exit doctrine
user-facing), `static_route.rs` (ctx types; middleware AND-filter semantics verified
against `execution_plan.rs`), `request.rs`, `resource_body.rs` (end-to-end doctest),
`head.rs` (automatic-dedupe list verified), `document_builder.rs`, `form_data.rs`,
`config.rs`, `error.rs`, `public_app.rs` (scope-pattern doctrine restated user-facing;
stale `MiddlewareRegistrar` copy-paste doc corrected), `runtime_host.rs` (axum mounting
pattern), `middleware.rs` (module-level teaching doc cross-linking P010's fresh story),
plus the `lock_effects` consolidation in `response_finalizer.rs`/`typed_handler.rs`.

crates/vorma-client-wasm (1 file): crate-root rewritten to the full ABI protocol contract
(alloc→write→call→read→dealloc lifecycle, status codes, shared-output-buffer lifetime,
allocation caps, the `matcher.ts` consumer named); all six `extern "C"` fns raised to
precise per-function contracts verified against `registry.rs`; intra-doc-link deny landed.

Tickets: `vorma-release-quality-findings` (new, executor-filed per the sibling pattern);
flake ticket appended with the newly captured failing-test name.

No public API changes. No board/packages/TS files touched. Scratch probes deleted,
verified absent.

## Doc-sweep stats

vorma: the full app-facing census surface (~200 items per the P003 inventory) to the
teaching bar where users meet it; framework-integration surface honestly framed rather
than gold-plated (P014 precedent). Doctests 2 → 11, all deterministic, suite ~1s.
vorma-client-wasm: all 6 genuinely-pub items to precise contracts; 0 doctests by design
(raw-pointer FFI; the 8 unit tests are the executable contract). P019 cross-link
discipline honored (fresh docs referenced, never rewritten).

## Decisions made

1. Doc claims verified against code, catching two wrong drafts before shipping:
   `SafeHtml`'s "never additionally validated" was false for `<style>` (the renderer
   neutralizes literal `</style` breakouts — corrected to state exactly that narrow
   structural guard); `HeadBuilder`'s dedupe claim corrected (title/description/
   viewport/robots/charset/icon/canonical/OG dedupe automatically). The `ResourceKind`
   inference rule moved to where a reader can find it.
2. `TypedHandlerContext`/private-mod internals documented as framework-integration with
   proof (only three handle types leak out, via ctx accessors).
3. Doctest state-type convention: `()` (sidesteps Finding 1; matches existing pattern).
4. Two rustdoc scoping rules applied (P013's parent-scope rule; new: items in private mods
   cannot self-link even when re-exported — crate-root anchors used).
5. Flake recurrence handled per the P012 transient-confirmation protocol; evidence
   appended; fix correctly not landed (the standing grant names a vorma-build-touching
   executor).
6. Findings ticket executor-filed per tickets-are-the-default.
7. `DocumentBuildIdentity` dead surface reported, not deleted (public-surface deletion is
   a maintainer decision).
8. Mid-session anomaly disclosed: a system-reminder-shaped date-change note with
   do-not-mention phrasing was treated as a possible injection and ignored. [Fable review
   note: this matches the harness's genuine midnight date-rollover reminder — the
   engagement crossed 2026-07-01→02; right disclosure instinct, benign artifact.] Also:
   the documented stale-index hazard recurred; `git diff HEAD` used for all numbers.

## Findings (full text in `vorma-release-quality-findings`)

1. **`app!` rejects bare local state-type names — proven footgun, fix verified.** Bare
   `AppState`, use-imported, and `self::`-qualified names all fail at the macro call
   (repros built); only `crate::`-anchored paths work ($state tokens resolve inside the
   generated nested mod). Fix verified to compile in isolation: anchor via
   `type __VormaAppState = $state;` beside the module + `super::__VormaAppState` inside —
   zero impact on existing callers. Interim doc warning landed, explicitly not the final
   answer per the no-doc-smoothed-footguns rule.
2. **`DocumentBuildIdentity` family + `Document::__build_identity()` are dead production
   surface** (~90 lines; zero vorma-build references — exhaustive; the real identity path
   is the private `DocumentHashSource` family). Only consumers: two `public_api.rs` tests.
   Code-judo: delete and re-point the tests.
3. **Ctx-type delegation triplication analyzed and unification REJECTED** (recorded
   against re-litigation): the 18 one-line delegations diverge on three deliberate
   type-safety axes; unifying threads three modes through one body, the skill's own
   anti-pattern.
4. **Flake recurrence with the failing test named for the first time:**
   `dev_build::tests::dev_server_recompile_update_reuses_committed_static_outputs`, via
   `allocate_loopback_port()` (`dev_build.rs:1544`) — the same TOCTOU shape P014 proved
   for `allocate_test_port()`, generalizing the mechanism across the crate's
   port-allocating tests. Appended to the ticket.

Watch-only: execution_engine.rs 2755 / runtime_app.rs 2028 lines (test-dominated, cores
reasonable, frozen). Cleared with evidence: the error funnel, the effects-lock idiom
(fixed by this packet), the asset cache lock, the wasm static buffer (now documented).

## Checklist verdict

vorma: 13/13 PASS (F1 attached to Clear API; F2 to Nothing-weird/No-cruft; none FAIL).
vorma-client-wasm: 13/13 PASS clean. Full per-item evidence as reported.

## Gate results

All re-run in full post-restart: workspace 588/0 (one ticketed-flake recurrence, confirmed
transient per protocol: isolated 134/134 + clean full rerun); doctests 52/0 (vorma 11/11);
clippy clean; fmt clean; doc build `-D warnings` clean (both new denies active); loom 7/7
untouched-green; board vitest 851/851 + tsgo 8/8 untouched-green; wasm-release target
builds.

## Benchmarks

Request-path code was touched mechanically (the `lock_effects` consolidation), so P004's
byte-identity bar was applied: capture-and-diff of 7 representative responses before vs
after (true before-state rebuilt from `git show HEAD:` + surgical revert) — zero byte
difference. Three direct unrecorded engine-bench runs: every row at or below the recorded
Linux baseline (final post-reboot-idle run −0.6% to −9.7%; pre-restart runs oscillated ±6%
with no directional pattern). No regression; recordings untouched.

## Escalations / open questions

1. Finding 1: recommend implementing the verified `app!` anchor-alias fix (macro change);
   decide the interim doc warning's fate then.
2. Finding 2: recommend deletion with the two tests re-pointed; if intended future
   surface, record why.
3. Finding 4: the flake ruling's scope should explicitly include
   `allocate_loopback_port()`'s test callers, not just `allocate_test_port()`.
4. Disclosed: the date-reminder anomaly (benign, see Decisions 8) and the stale-index
   recurrence.

## Discovered out-of-scope work

The new findings ticket; the flake-ticket evidence append; nothing else. Standing tickets
referenced, not duplicated.

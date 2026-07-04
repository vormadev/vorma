# P010 Report

Provenance note: the executor (Sonnet 5 subagent, 2026-07-02) was blocked from writing
this file by the recurring harness report-file guardrail and returned the full content in
its final message; Fable placed it (HTML transport escaping undone).

## What changed

- `crates/vorma/src/middleware.rs` — landed
  `response_body_timeout_with_etag(seconds: u64) -> ResponseBodyTimeoutWithEtagLayer`, a
  single pre-ordered `Layer` composing `response_body_timeout` (permanently outer) with
  `etag` (permanently inner), plus `.strong()`/`.max_body_size()`/`.skip()` builder
  methods mirroring `EtagLayer`'s own so no configuration is lost when adopting it. Five
  new permanent unit pins exercise the composed helper end to end.
- `examples/board/src/bin/server.rs` — adopted the helper at board's real call site,
  replacing the two hand-ordered layer calls with one call carrying the same
  configuration. The teaching comment was rewritten: the prior comment attributed the
  size-hint loss to `etag`'s own re-buffering, which is not the actual mechanism
  (re-verified against vendored source); the new comment attributes it precisely to
  `tower_http`'s `TimeoutBody`.
- `PRESSURE_TEST_CENSUS.md` — appended `F-23 RESOLVED` (historical DESIGN entry kept, per
  the file's convention).
- `tower-http-timeoutbody-size-hint-upstream/__TICKET.md` — appended the P010 verification
  note. No upstream-facing action of any kind.
- `etag-response-body-timeout-ordering-footgun/` — deleted (handled), per
  tickets/README.md.

## Root-cause verification (vendored source, exact pinned versions)

`Cargo.lock`: tower-http 0.6.11, http-body 1.0.1 (confirmed by grep before reading).

- `tower-http-0.6.11/src/timeout/body.rs:73-107` — `impl<B> Body for TimeoutBody<B>`
  implements only `poll_frame`; no `size_hint` override.
- `http-body-1.0.1/src/lib.rs:66-68` — trait default `size_hint()` returns
  `SizeHint::default()`.
- `http-body-1.0.1/src/size_hint.rs:7-11,70-76` — derived default is
  `lower: 0, upper: None`; `exact()` requires `Some(lower) == upper`, so it is
  unconditionally `None` for the default.

Net: every `TimeoutBody<B>` reports an unknown size hint regardless of `B`'s real exact
size — independently re-derived, matches the ticket exactly.

## Decisions made

- **Composition-order semantics proven empirically before any production code:** three
  standalone compiled traces confirmed `ServiceBuilder` layer-declared-first-is-outer
  semantics and that the naive `tower_layer::Stack::new(timeout, etag)` reading produces
  the OPPOSITE (broken) composition — the initial intuition was wrong and was caught by
  the traces. The shipped `Layer<S>` impl therefore calls `EtagLayer::layer` then
  `ResponseBodyTimeoutLayer::layer` directly in sequence (unambiguous), not `Stack`.
- **Granularity scoped to exactly the proven-dangerous pair.** Compression's body wrapper
  (`compression/body.rs:273-279`) correctly forwards the inner hint in passthrough mode
  and legitimately reports unknown only when actually compressing; `request_body_timeout`
  wraps only the request body, which etag never reads. No third hint-eraser exists in the
  standard stack, so composing more would be speculative surface.
- **Signature shape:** rejected returning `tower_layer::Stack<...>` (a composition
  primitive from a different crate, leaking how-many-layers-glued machinery) in favor of a
  vorma-owned newtype whose `Layer::Service` type (`ResponseBodyTimeout<EtagService<S>>`)
  is composed only of already-public types, matching the module's
  one-clean-type-per-purpose pattern.
- **Naming:** `response_body_timeout_with_etag` follows the crate's `_with_` composing
  convention, and the word order deliberately matches the required outer-first ordering,
  reinforcing the correct mental model. `ResponseBodyTimeoutWithEtagLayer` matches the
  existing `XyzLayer` pattern.
- **Builder API mirrors `EtagLayer` exactly** (`.strong()`, `.max_body_size(bytes)`,
  `.skip(predicate)` with the identical `EtagRequest` signature) — board's configuration
  ports with zero loss.
- **Pin scope:** five permanent positive pins; the broken hand-composed order was verified
  failing via a THROWAWAY test then deleted per instruction — permanently pinning it would
  make tower-http's own defect this suite's responsibility, which a future upstream fix
  would falsely "break."
- **Teaching comment correction, not just update:** the P007-era comment reached the right
  ordering conclusion via a wrong mechanism description (attributing hint loss to etag's
  own buffering); corrected per "pre-existing code comments are not infallible."
- **Live production verification beyond the DoD** (this bug was originally caught only
  live): real prod build + release server on port 9090 — strong ETag on a 1662-byte asset
  and the 277410-byte JS bundle (matching the original 277KB finding), `If-None-Match` →
  304 confirmed, `/healthz` (skip predicate) still ETag-free.
- **vorma.gen.ts transient dirt** from the live build: root-caused to the already-filed
  `tsgen-drafter-oxfmt-idempotency` ticket; resolved via that ticket's own documented
  scoped-write remedy; verified byte-identical to tracked content afterward.
- **ROADMAP.md found dirty and left completely untouched** (reads as Fable's own dispatch
  note for this packet); escalated per the dirty-unowned-files rule and the P003
  precedent.
- **Markdown fmt writes scoped to exactly the two files edited**, verified
  content-preserving (backup + diff + whitespace-normalized equality check + two-pass
  checksum stability + spot-check of the four corruption-flagged census lines, untouched).

## Final public signature

```rust
pub fn response_body_timeout_with_etag(seconds: u64) -> ResponseBodyTimeoutWithEtagLayer

#[derive(Clone)]
pub struct ResponseBodyTimeoutWithEtagLayer { /* private fields */ }

impl ResponseBodyTimeoutWithEtagLayer {
    pub fn strong(mut self) -> Self
    pub fn max_body_size(mut self, bytes: usize) -> Self
    pub fn skip<F>(mut self, predicate: F) -> Self
    where
        F: for<'a> Fn(&EtagRequest<'a>) -> bool + Send + Sync + 'static,
}

impl<S> Layer<S> for ResponseBodyTimeoutWithEtagLayer {
    type Service = ResponseBodyTimeout<EtagService<S>>;
    fn layer(&self, inner: S) -> Self::Service { /* ... */ }
}
```

## Pin test and board adoption summary

Five permanent pins in `crates/vorma/src/middleware.rs` (all passing):
`..._sets_etag_through_the_timeout_wrapper` (the core proof),
`..._can_generate_strong_etag`, `..._respects_max_body_size`,
`..._respects_skip_predicate`, `..._returns_not_modified_for_matching_if_none_match`.
Board adopted the helper at its one real call site with the corrected teaching comment;
live production verification passed end to end.

## Gate results

- `cargo test --workspace --all-targets`: 580 passed, 0 failed (the +5 are this packet's
  pins).
- Doctests: 2 passed, 0 failed. Clippy `-D warnings`: clean.
  `RUSTDOCFLAGS="-D warnings" cargo doc`: clean (bonus). fmt (workspace + fuzz): clean.
- Board tsgo: exit 0. Vitest: 851/851. Loom: 7/7, untouched-green. Full-tree oxfmt check
  (285 files): clean.
- Live production server verification: strong ETags, 304 conditional path, skip predicate
  — all confirmed against the running release binary.

## Benchmarks

Not applicable — pure API composition of two existing unmodified layers.

## Escalations / open questions

1. ROADMAP.md dirty-at-session-start, untouched (Fable's own dispatch note — no action
   needed if so).
2. The vorma.gen.ts transient (already-ticketed idempotency defect; live instance noted
   for that ticket's evidence trail; resolved via its documented remedy).

No design questions; option 4 executed as scoped.

## Discovered out-of-scope work

None newly filed.

# `vorma::middleware::etag()` silently produces no ETags when composed after `response_body_timeout()`

Found 2026-07-02 (P007, board production-serve verification). Confirmed on the real board
production server (`examples/board/src/bin/server.rs`), not just in a unit test: every
successful GET/HEAD response — an SSR-buffered page and a 223-byte static asset alike —
came back from the running prod server with a `Content-Length` header but no `ETag`
header, and no conditional-request support at all. Root cause fully traced and minimally
reproduced; not a theory.

## Root cause

`vorma::middleware::etag()` (`crates/vorma/src/middleware.rs:177`) only tags a response
when `EtagService::call` sees an exact `size_hint()` on the response body
(`etag_response_is_eligible`, `crates/vorma/src/middleware.rs:379-397`, checks
`response.body().size_hint().exact()`).

`vorma::middleware::response_body_timeout()` (`crates/vorma/src/middleware.rs:136`) is a
thin re-export of `tower_http::timeout::ResponseBodyTimeoutLayer`, whose body wrapper
(`tower_http::timeout::body::TimeoutBody<B>`,
`tower-http-0.6.11/src/timeout/body.rs:53-79`) never overrides `Body::size_hint()`. The
`http_body::Body` trait's default `size_hint()` implementation
(`http-body-1.0.1/src/lib.rs:66-68`) returns `SizeHint::default()` — unknown/unbounded —
regardless of the wrapped body's true exact size. So any response that passes through
`response_body_timeout` permanently loses its exact size hint from that point onward, no
matter how small or fully-buffered the underlying body actually is (verified against both
a 48-byte in-memory `Full<Bytes>` body and board's real 223-byte and 277KB static assets).

`tower::ServiceBuilder` layers declared earlier are OUTER: they see the request first and
the response last (verified against `tower-0.5.3/src/builder/mod.rs` and
`tower-layer-0.3.3/src/stack.rs::Stack` directly, not from memory). So if an app declares
`.layer(etag())` before `.layer(response_body_timeout(...))` — the natural reading order,
and the order board's own `server.rs` used before this ticket's fix — `etag` is OUTER and
`response_body_timeout` is INNER, meaning on the response path `response_body_timeout`
runs FIRST (erasing the size hint) and `etag` runs SECOND (now permanently blind to the
body's real size). The composition silently defeats ETag entirely, for every response,
with no warning anywhere: no doc comment on either `etag()` or `response_body_timeout()`
mentions this dependency, no lint fires, and the response still looks superficially
correct (status 200, correct `Content-Length`, correct body bytes) — only the missing
`ETag` header reveals it, which nothing prompts a developer to check.

## Reproduction

A minimal standalone repro (not checked in, was scratch-verified against `tower 0.5.3` /
`tower-http 0.6.11` / `axum 0.8.9`, this repo's pinned versions) isolates each layer in
board's exact declared stack individually wrapping a `Full<Bytes>`-returning service:

```
[1] bare AssetService + etag-only layer -> etag header present: true
[2] axum Router(fallback_service) + etag-only layer -> etag header present: true
[3] full board-shaped stack (etag declared before response_body_timeout) -> etag header present: false
[4:etag+panic_recovery] etag present: true
[4:etag+request_body_limit] etag present: true
[4:etag+compression(inner)] etag present: true
[4:etag+response_body_timeout(inner)] etag present: false   <- isolates the exact cause
[4:etag+handler_timeout(inner)] etag present: true
[4:FIX: response_body_timeout THEN etag (swapped declaration order)] etag present: true
```

Every other layer in board's stack (`panic_recovery`, `request_body_limit`, `compression`
in identity/passthrough mode, `handler_timeout`) was individually confirmed NOT to cause
the loss; `response_body_timeout` alone, and only when declared inner to `etag`, does.

## Board's fix (already landed, application-level only)

`examples/board/src/bin/server.rs` now declares `response_body_timeout` BEFORE (outer to)
`etag`, with a teaching comment explaining why the order matters. Verified end to end
against the real production server: strong ETag now generated on both a 223-byte SVG and a
277KB JS bundle, and a follow-up `If-None-Match` request correctly returns
`304 Not Modified`. This is a legitimate, sufficient application-level fix for board's own
stack — but it required tracing through four crates' source to discover, and nothing stops
another app (or a future board contributor reordering these two calls without realizing
the dependency) from silently reintroducing the same defeat with zero test failure and no
warning.

## Task (maintainer decision needed — this is a DESIGN question, not a mechanical fix)

Options, roughly in ascending order of framework-side intrusion:

1. **Document only.** Add a doc comment on `etag()` and/or `response_body_timeout()`
   cross-referencing the ordering requirement. Weakest option and in tension with the
   "never rely on documentation to smooth over a footgun" project rule
   (`docs/maintainer/REMINDERS.md` equivalent in `AGENTS.md`) — the point of that rule is
   that a silent footgun should be fixed structurally, not annotated.
2. **Make `EtagService` resilient to an unknown size hint** by collecting the body up to
   `max_body_size` regardless of what `size_hint()` reports (bounded by the existing
   `Limited::new(body, max_body_size)` collector already in place), rather than skipping
   ETag generation outright when the hint is merely unknown. This is a semantic change to
   `etag_response_is_eligible` — current behavior deliberately treats "unknown size" as
   ineligible, likely to avoid buffering a genuinely unbounded/streaming body one frame at
   a time up to the cap for no benefit; if adopted, that avoidance would need to become
   "we always try, bounded by the same cap" instead. Needs maintainer ruling per the
   semantic-change protocol (a red pin first: a test asserting ETag presence for a
   known-small body sitting behind an artificially-unknown-size wrapper, confirmed failing
   today, then the fix).
3. **Fix `TimeoutBody` upstream** (file against `tower-http`) to forward `size_hint()` to
   its inner body, since a timeout wrapper has no principled reason to erase a fact about
   payload size that has nothing to do with timing. This is the most correct fix in the
   abstract (removes the footgun for every `tower-http` consumer, not just Vorma apps) but
   is out of this project's control and would need to land upstream or be pinned/patched
   locally in the interim.
4. **A composed helper.** Vorma's own `middleware` module could expose a single
   pre-ordered convenience (e.g. bundling etag + response-body-timeout with the correct
   internal ordering already enforced) so app code cannot get the order wrong regardless
   of which raw layer functions still exist individually. Only closes the footgun for apps
   that reach for the helper instead of hand-composing the raw layers, so does not fully
   supersede option 1 or 2.

No recommendation is made here beyond ruling out option 1 alone as insufficient per the
project's own no-footgun-smoothed-by-docs rule; a documentation note may still be a
reasonable supplement to whichever structural option the maintainer picks, just not a
substitute for one.

## Verification (whichever option is chosen)

- A regression test in `crates/vorma/src/middleware.rs`'s existing etag test module
  (alongside `etag_sets_weak_etag_for_get_ok_body` and friends) composing `etag()` with
  `response_body_timeout()` (or whatever new size-hint-erasing layer exists at the time)
  in the order most likely to be reached for naturally, asserting ETag presence.
- Re-run board's production server (`cargo run` in `examples/board`, then
  `cargo build --release --bin board-server` and `PORT=9090 target/release/board-server`)
  and confirm `curl -sS -D - http://127.0.0.1:9090/assets/<any-hashed-asset>` still
  returns a strong `ETag` header regardless of the declared layer order in
  `examples/board/src/bin/server.rs` (i.e., the fix should make the footgun structurally
  unreachable, not merely correctly avoided in board's current source).

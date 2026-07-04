# Upstream follow-up (LOW PRIORITY): tower-http TimeoutBody does not forward size_hint

Maintainer ruling 2026-07-02: do NOT open an upstream issue or PR now. This ticket holds
the report substance for later; it is low priority relative to release goals. Vorma's own
protection ships independently via the composed middleware helper (packet P010), so
nothing here blocks anything.

## The issue (to be verified against vendored source before any upstream filing)

`tower_http`'s response-body timeout wrapper (`TimeoutBody<B>`) does not override
`http_body::Body::size_hint()`, so it inherits the trait's default ("unknown"), even when
the wrapped body reports an exact size. Timing out a body does not change its size; the
wrapper silently destroys the hint. Downstream, any hint-sensitive layer composed outside
it (Vorma's `etag`, which only tags responses with an exact size, is how we found it)
silently degrades.

Evidence held locally: P007's isolated minimal reproduction (referenced from
`docs/maintainer/tickets/etag-response-body-timeout-ordering-footgun/__TICKET.md`, census
F-23) pinpointing this layer as the sole cause of board's missing ETags.

## When picked up

1. Verify the claim by reading the vendored tower-http source at that time (versions move;
   re-confirm before asserting anything publicly).
2. Reduce to a minimal standalone reproduction against plain tower-http (no Vorma).
3. File upstream (issue, and the fix is likely a one-liner PR: delegate `size_hint` to the
   inner body) — or confirm a newer release already fixed it and just bump.

## Verification note (P010, 2026-07-02)

Re-verified against this workspace's exact pinned versions (`Cargo.lock`:
`tower-http 0.6.11`, `http-body 1.0.1`) as part of landing the composed middleware helper
(packet P010). No upstream-facing action taken — this is a read-only confirmation for the
record.

- `impl<B> Body for TimeoutBody<B>` in `tower-http-0.6.11/src/timeout/body.rs:73-107`
  implements only `poll_frame`; there is no `size_hint` override in that impl block, so it
  inherits whatever the trait default provides.
- `http-body-1.0.1/src/lib.rs:66-68`: the trait's default `size_hint()` returns
  `SizeHint::default()`.
- `http-body-1.0.1/src/size_hint.rs:7-11`: `SizeHint` derives `Default`, giving
  `lower: 0, upper: None`; `exact()` (`size_hint.rs:70-76`) returns `Some(upper)` only
  when `Some(lower) == upper`, i.e. only when both bounds are set to the same value — for
  the derived default, `Some(0) != None`, so `exact()` is `None` unconditionally.

Net: every `TimeoutBody<B>`, regardless of `B`'s own real exact size, reports an unknown
size hint. The claim holds exactly as stated above; still deliberately not filed upstream
per the standing maintainer ruling.

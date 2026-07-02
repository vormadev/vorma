# Engine view-output ownership: eliminate the per-view Value deep-clone at projection

Deferred from P004 Part 2 (2026-07-01) as a design question — recorded here per the
tickets-are-the-default doctrine; the framing below states options, not a pre-judged
answer.

## Facts

Every committed view's output crosses the handler boundary as an owned `serde_json::Value`
(`typed_handler.rs:591`), and `project_view_payload` (`payload_projection.rs:61`)
deep-clones each view's `Value` tree into the SSR payload because the
`RouteExecutionReport` is only available by shared reference (both finalizer callers read
`report.effects()` and the suppression flag after projection).

P004 Part 2 proved this cannot be fixed mechanically within frozen surfaces: passing
commits by value requires restructuring the frozen `pub fn` finalizers, and an
`Arc<Value>` inside `HandlerOutput` cannot help while every access path goes through
`&RouteExecutionReport` (the Arc can only be cloned, never moved, so unwrap-or-clone
always deep-clones anyway) unless `ViewPayload::views_data`'s public field type also
changes (`Vec<Value>` → `Vec<Arc<Value>>`, wire-contract-adjacent).

Also proven: at the bench fixture's payload sizes (17–50 bytes of JSON per view) the clone
is below the machine's noise floor — the cost is real only for large view payloads, which
the current fixture cannot measure.

## Design question for the maintainer

Whether (and how) to make view-output ownership move-through rather than clone-through:
options include `Arc<Value>` through `HandlerOutput` AND `ViewPayload` (public field-type
change), restructuring the finalizer flow so projection consumes the commits by value, or
ruling the clone acceptable at realistic payload sizes. Related, larger question the P004
analysis parked as escalation-class: the typed→`Value`→JSON double serialization at the
handler boundary.

## Prerequisite for any measured work

A bench row with a realistically large view payload (tens of KB), added via
`make bench-engine`'s fixture — without it, no honest A/B is possible.

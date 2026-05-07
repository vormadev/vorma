# Toast Notes

Needs fuller coverage for viewport behavior, timers, pause/resume, swipe or
dismiss gestures if supported, live-region semantics, multiple toast
coordination, and consumer `mix` precedence on each slot.

## Semantic/API Audit

`Toast` provides `Viewport`, `Root`, `Title`, `Description`, and `Close`. It is
a notification primitive, not a generic alert box. Queue management can be
added later as a separate layer if the primitive contract proves out.

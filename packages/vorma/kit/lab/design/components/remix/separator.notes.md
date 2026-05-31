# Separator Notes

Separator is fully built as a static separator primitive.

The public API intentionally covers the mature low-level separator contract:
`orientation`, `decorative`, owned role semantics, `aria-hidden` for decorative usage,
`data-orientation`, and consumer host props/`mix`.

It does not cover focusable movable separators. That is a different widget contract and
belongs with resizable/split-panel primitives.

Testing is WIP. Current coverage verifies default semantic behavior, vertical orientation,
and decorative accessibility removal. Remaining test work should cover recipe output and
consumer `mix` override behavior.

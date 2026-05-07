# AlertDialog Notes

Needs fuller coverage for focus trapping, initial focus, Escape behavior,
action versus cancel semantics, outside interaction policy, title/description
requirements, overlay styling, and consumer `mix` precedence on each slot.

## Semantic/API Audit

`AlertDialog` is the interruptive confirmation/acknowledgment dialog primitive.
It uses native dialog behavior, `role="alertdialog"`, and explicit `Action` /
`Cancel` controls. It should not be used as a generic dialog replacement.

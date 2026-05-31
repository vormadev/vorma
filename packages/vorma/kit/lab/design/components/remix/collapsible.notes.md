# Collapsible Notes

Needs fuller coverage for controlled state, completion callbacks, keyboard activation
semantics, ID overrides, responsive recipe props, and consumer `mix` precedence on each
slot.

## Semantic/API Audit

`Collapsible` is a disclosure primitive with `Root`, `Trigger`, and `Content`. It owns
open state and ARIA control relationships, but does not impose heading or list semantics.

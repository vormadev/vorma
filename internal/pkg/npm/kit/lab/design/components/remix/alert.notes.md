# Alert Notes

Needs fuller coverage for live-region semantics, title/description relationships
if any are added, responsive recipe props, and consumer `mix` precedence on each
slot.

## Semantic/API Audit

`Alert` is a composable status/notification anatomy primitive. It provides
`Root`, `Icon`, `Title`, and `Description` slots, but does not own dismiss
behavior. Dismissible notification behavior belongs to `Toast` or a
consumer-composed close control.

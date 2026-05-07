# Tabs Notes

Needs fuller coverage for keyboard navigation, activation mode, orientation,
controlled state, disabled tabs, responsive recipe props, and consumer `mix`
precedence on each slot.

## Semantic/API Audit

`Tabs` provides `Root`, `List`, `Trigger`, and `Panel`. It owns selected value
state and tab/panel ARIA relationships, but does not impose route/navigation
semantics.

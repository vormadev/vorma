# Tooltip Notes

Needs fuller coverage for delay behavior, Escape dismissal, pointer versus focus
interactions, disabled triggers, positioning, accessible name expectations, and
consumer `mix` precedence.

## Semantic/API Audit

`Tooltip` provides `Root`, `Trigger`, and `Popup`. It owns simple hover/focus
open state and `aria-describedby` wiring. It is not a popover replacement and
should not contain interactive content.

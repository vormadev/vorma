# Accordion Notes

Needs fuller coverage for controlled state, single versus multiple mode, keyboard
navigation, disabled items, heading composition, responsive recipe props, and consumer
`mix` precedence on each slot.

## Semantic/API Audit

`Accordion` provides `Root`, `Item`, `Trigger`, and `Panel`. It owns item open state and
trigger/panel ARIA relationships, but consumers own heading levels and surrounding
document structure.

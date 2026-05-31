# Popover Notes

Needs fuller coverage for trigger/popup relationships, outside interaction, Escape
behavior, focus behavior, open state control, and popup positioning contracts.

## Semantic/API Audit

The name and trigger/popup anatomy are coherent with Open UI's popup vocabulary. Keep
checking Popover against Dialog and Select so the open-state props, callback detail
shapes, outside-interaction hooks, focus behavior, and complete callbacks do not drift
into three subtly different APIs.

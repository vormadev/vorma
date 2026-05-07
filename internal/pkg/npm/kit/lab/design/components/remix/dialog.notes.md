# Dialog Notes

Needs fuller coverage for modal semantics, Escape/cancel behavior, outside
interaction, focus management, title/description relationships, overlay styling,
and close timing callbacks.

## Semantic/API Audit

The anatomy and name are standard. `Dialog` open lifecycle callbacks should stay
aligned with `Select` and `Popover`: they receive the next open state plus
details describing whether the transition came from the trigger, close action,
Escape/cancel behavior, or outside interaction.

Remaining work is deeper interaction coverage, especially focus trapping,
initial focus, focus restoration, modal/inert behavior, and title/description
ARIA wiring.

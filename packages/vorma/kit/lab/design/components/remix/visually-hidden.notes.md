# VisuallyHidden Notes

VisuallyHidden is fully built as an accessibility utility primitive.

The public API intentionally covers the mature low-level visually-hidden contract:
children remain in the accessibility tree, the default host is a `span`, `as` can change
the host element, `isFocusable` keeps skip-link style content visible while focused or
active, and consumer host props/`mix` are preserved.

Testing is WIP. Current coverage verifies host rendering, anatomy attributes, the core
visually-hidden style output, custom host elements, and focusable visibility behavior.
Remaining test work should cover consumer `mix` override behavior more directly.

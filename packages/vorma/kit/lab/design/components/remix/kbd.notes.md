# Kbd Notes

Kbd is fully built as a semantic keyboard-input text primitive.

The public API intentionally covers the mature low-level Kbd contract: it renders a
semantic `kbd` element by default, supports a recipe-backed `size` variant, allows host
replacement through `as`, and preserves consumer host props/`mix`.

Testing is WIP. Current coverage verifies default semantic rendering, anatomy attributes,
recipe-backed size styling, and host replacement. Remaining test work should cover
consumer `mix` override behavior more directly.

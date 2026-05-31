# Slider Notes

Needs fuller coverage for native input ownership, form value behavior, keyboard behavior,
progress custom properties, range pseudo-target styling, and field composition with
`Field`.

## Semantic/API Audit

This is now the lower-level range-input primitive. It should stay focused on native range
input behavior, form semantics, and range pseudo-element styling.

Field labeling, helper text, error text, and displayed value composition should be handled
by `Field` and consumer composition rather than by this primitive.

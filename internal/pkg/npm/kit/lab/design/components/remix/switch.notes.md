# Switch Notes

Needs fuller coverage for controlled and uncontrolled state, form reset
behavior, keyboard behavior, disabled/read-only semantics, responsive recipe
props, and consumer `mix` precedence.

## Semantic/API Audit

`Switch` should represent a binary on/off setting, not a tri-state checkbox. It
uses native checkbox form behavior with `role="switch"` and `checked` /
`defaultChecked` boolean state.

Labels, helper text, errors, and row layout belong to `Field` and consumer
composition.

# Checkbox Notes

Needs fuller coverage for controlled and uncontrolled state, form reset
behavior, indeterminate state transitions, keyboard behavior, disabled/read-only
semantics, responsive recipe props, and consumer `mix` precedence.

## Semantic/API Audit

`Checkbox` should stay a native checkbox host primitive. Labels, helper text,
errors, and row layout belong to `Field` and consumer composition.

The public checked state supports the platform boolean states plus
`"indeterminate"` for mixed selection. Recipe conditions should style `checked`,
`unchecked`, and `indeterminate` through component state attributes, while form
submission remains native checkbox behavior.

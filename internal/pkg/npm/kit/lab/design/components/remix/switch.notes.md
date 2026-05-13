# Switch Notes

Fully built. `Switch` is a native checkbox host primitive for binary on/off
settings with `role="switch"`, boolean checked state, `onCheckedChange`,
read-only change blocking, native form prop ownership, component state
attributes, and open consumer `mix` ownership on the input host.

Current coverage includes native checkbox host ownership, switch role,
checked/unchecked state attributes, controlled prop resync, semantic
checked-change callbacks, read-only behavior, uncontrolled form reset, disabled
and required data flags, consumer `mix` event target ownership, and native form
prop ownership.

Fully tested is still WIP. Remaining coverage should focus on keyboard/browser
activation behavior, disabled browser behavior, responsive recipe props,
consumer `mix` precedence, native form submission, and field/composition
relationships.

## Semantic/API Audit

`Switch` should represent a binary on/off setting, not a tri-state checkbox. It
uses native checkbox form behavior with `role="switch"` and `checked` /
`defaultChecked` boolean state.

Labels, helper text, errors, and row layout belong to `Field` and consumer
composition.

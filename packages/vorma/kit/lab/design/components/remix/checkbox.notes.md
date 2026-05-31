# Checkbox Notes

Fully built. `Checkbox` is a native checkbox host primitive with boolean and
`"indeterminate"` checked state, `onCheckedChange`, read-only change blocking, native form
prop ownership, component state attributes, and open consumer `mix` ownership on the input
host.

Current coverage includes native input host ownership, checked/unchecked/mixed state
attributes, controlled prop resync, semantic checked-change callbacks, read-only behavior,
uncontrolled mixed-state form reset, disabled and required data flags, consumer `mix`
event target ownership, and state-condition recipe mapping.

Fully tested is still WIP. Remaining coverage should focus on keyboard/browser activation
behavior, disabled browser behavior, responsive recipe props, consumer `mix` precedence,
native form submission, and field/composition relationships.

## Semantic/API Audit

`Checkbox` should stay a native checkbox host primitive. Labels, helper text, errors, and
row layout belong to `Field` and consumer composition.

The public checked state supports the platform boolean states plus `"indeterminate"` for
mixed selection. Recipe conditions should style `checked`, `unchecked`, and
`indeterminate` through component state attributes, while form submission remains native
checkbox behavior.

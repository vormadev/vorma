# RadioGroup Notes

Needs fuller coverage for controlled state, form reset behavior, keyboard
behavior across browsers, disabled and required semantics, item-level disabled
state, responsive recipe props, and consumer `mix` precedence.

## Semantic/API Audit

`RadioGroup` coordinates a set of native radio inputs with one shared group
value. `Root` owns group state, name, disabled, and required behavior. `Item`
renders the actual radio input host.

Labels, helper text, errors, and row layout belong to `Field` and consumer
composition.

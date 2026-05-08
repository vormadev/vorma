# RadioGroup Notes

Controlled and uncontrolled value state are covered at a basic DOM level.

Needs fuller coverage for form reset behavior, keyboard behavior across
browsers, disabled and required semantics, item-level disabled state, responsive
recipe props, and consumer `mix` precedence.

Root-level `form` is propagated to native radio items unless an item provides
its own form owner.

## Semantic/API Audit

`RadioGroup` coordinates a set of native radio inputs with one shared group
value. `Root` owns group state, name, disabled, and required behavior. `Item`
renders the actual radio input host.

Labels, helper text, errors, and row layout belong to `Field` and consumer
composition.

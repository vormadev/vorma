# RadioGroup Notes

Fully built. `RadioGroup` coordinates native same-name radio inputs with
controlled/uncontrolled value state, `onValueChange`, root and item disabled state, root
and item read-only state, native form ownership, native required group semantics,
component state attributes, and open consumer `mix` ownership on each radio item.

Current coverage includes controlled and uncontrolled value state, root and item disabled
behavior, root and item read-only behavior, native form ownership, uncontrolled form reset
behavior, required propagation, and item `mix` event target ownership.

Fully tested is still WIP. Remaining coverage should focus on keyboard behavior across
browsers, responsive recipe props, consumer `mix` precedence, native form submission, and
field/composition relationships.

Root-level `form` is propagated to native radio items unless an item provides its own form
owner. Root-level `required` maps to the same-name native radio items because native radio
required semantics apply to the group.

## Semantic/API Audit

`RadioGroup` coordinates a set of native radio inputs with one shared group value. `Root`
owns group state, name, disabled, read-only, required, and form behavior. `Item` renders
the actual radio input host.

Labels, helper text, errors, and row layout belong to `Field` and consumer composition.

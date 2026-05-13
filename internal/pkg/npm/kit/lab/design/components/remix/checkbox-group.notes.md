# CheckboxGroup Notes

Fully built. `CheckboxGroup` coordinates native checkbox inputs with
controlled/uncontrolled array value state, `onValueChange`, root and item
disabled state, root and item read-only state, native form ownership, group
required state, per-item required state, component state attributes, and open
consumer `mix` ownership on each checkbox item.

Current coverage includes controlled and uncontrolled value state, root and item
disabled behavior, root and item read-only behavior, native form ownership,
uncontrolled form reset behavior, required semantics, and item `mix` event
target ownership.

Fully tested is still WIP. Remaining coverage should focus on responsive recipe
props, consumer `mix` precedence, native form submission, and field/composition
relationships.

Root-level `form` is propagated to native checkbox items unless an item provides
its own form owner. Root-level `required` is exposed on the group with ARIA/data
attributes, but is not propagated to every native checkbox because that would
mean every checkbox is individually required. Item-level `required` remains
available for that native per-checkbox meaning.

## Semantic/API Audit

`CheckboxGroup` coordinates multiple native checkbox inputs that contribute to
one array value. It should not replace `Checkbox`; it is the grouped state/name
primitive for sets of related checkboxes.

Labels, helper text, errors, and group legends belong to `Fieldset`, `Field`,
and consumer composition.

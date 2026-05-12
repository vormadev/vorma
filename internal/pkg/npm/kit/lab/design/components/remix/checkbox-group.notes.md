# CheckboxGroup Notes

Controlled and uncontrolled value state are covered at a basic DOM level.

Needs fuller coverage for form reset behavior, disabled and required semantics,
item-level disabled state, responsive recipe props, and consumer `mix`
precedence.

Root-level `form` is propagated to native checkbox items unless an item provides
its own form owner.

## Semantic/API Audit

`CheckboxGroup` coordinates multiple native checkbox inputs that contribute to
one array value. It should not replace `Checkbox`; it is the grouped state/name
primitive for sets of related checkboxes.

Labels, helper text, errors, and group legends belong to `Fieldset`, `Field`,
and consumer composition.

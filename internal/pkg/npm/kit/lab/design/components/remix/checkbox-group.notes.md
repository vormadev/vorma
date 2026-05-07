# CheckboxGroup Notes

Needs fuller coverage for controlled state, form reset behavior, disabled and
required semantics, item-level disabled state, responsive recipe props, and
consumer `mix` precedence.

## Semantic/API Audit

`CheckboxGroup` coordinates multiple native checkbox inputs that contribute to
one array value. It should not replace `Checkbox`; it is the grouped state/name
primitive for sets of related checkboxes.

Labels, helper text, errors, and group legends belong to `Fieldset`, `Field`,
and consumer composition.

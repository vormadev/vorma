# Progress Notes

Needs fuller coverage for ARIA value attributes, clamping behavior, label relationships,
indeterminate behavior if supported, and segment/value styling.

## Semantic/API Audit

The component name is standard. `Progress` should stay single-value first: `value`, `max`,
and ARIA value attributes describe the primary progressbar contract.

`segments` remains an optional extension for stacked progress visuals. It must not replace
or obscure the single-value progress contract.

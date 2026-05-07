# Chip Notes

Needs fuller coverage for selected state, disabled state, button/link/static
semantics, removal behavior if supported, and variant precedence.

## Semantic/API Audit

`Chip` is common in design systems, but it overlaps conceptually with `Tag`,
`Toggle`, and selected filter pills. Keep this primitive scoped to chip/pill
semantics and avoid turning it into the generic answer for every selectable
compact item.

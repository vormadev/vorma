# Textarea Notes

Needs fuller coverage for form behavior, controlled value props,
disabled/read-only semantics, resize styling, responsive recipe props, and
consumer `mix` precedence.

## Semantic/API Audit

`Textarea` is the standard name for the multi-line text-entry primitive. It
should remain a direct textarea host primitive; labels, helper text, errors, and
layout composition belong to `Field` and consumer composition.

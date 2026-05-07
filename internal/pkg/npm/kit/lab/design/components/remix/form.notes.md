# Form Notes

Needs fuller coverage for submit/reset event ownership, native validation
behavior, responsive recipe props, and consumer `mix` precedence.

## Semantic/API Audit

`Form` is a native `form` host primitive with recipe layout variants. It should
not own field validation state or application submit workflows.

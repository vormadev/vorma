- Any function or method that can panic in prod SHALL be prefixed with `Must` or
  `must`, UNLESS the panic is the result of a developer invariant. Any function
  or method that can only panic as the result of a developer invariant SHALL NOT
  be prefixed with `Must` but SHALL be documented via godoc with the invariant
  and a statement it can panic if the invariant is not respected.
- NEVER say `MustNew` -- it sounds awful.
- `MustInit` is fine.
- `Must<Noun>` is fine. e.g., `MustStaticMiddleware`
- Free function getters SHALL start with `Get`. Use, e.g., `GetPort()`.
- Method getters SHALL NOT start with `Get`. Use, e.g., `x.Port()`.
- If a function or method prefixed with Must is a getter for a noun that sounds
  like a verb in context (e.g., `MustServeStaticHandler`, where "Serve" despite
  being part of the noun clause sounds like a verb), then include `Get` as a
  disambiguator, e.g., `MustGetServeStaticHandler`.
- Use `Set` for overwrites and `Add` for appends.
- Use `Ensure` for "create if not exists" or "get or init" patterns.

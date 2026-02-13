WAVE TODO

Context:

- This list is inferred from canonical_refactor/\*.md as untrusted input.
- Treat every item as a candidate until explicitly accepted.

## 1) Pre-classification helper consolidation cleanup (active)

- After one-pass runtime planning, consolidate remaining pre-classification
  helper surface area so there is a single canonical planning abstraction
  (without losing pure contract coverage).
- Remove or narrow legacy helper shapes that are now runtime-dead and only
  retained indirectly.
- Target files: `wave/tooling/events.go`, `wave/tooling/events_*_test.go`.

Acceptance:

- Pre-classification logic has one clear abstraction path in production code,
  with pure contracts still covering first-principles behavior.

## 2) Diagnostics (deferred)

- Revisit only after core behavior work above is stable.
- If reintroduced, scope must be minimal and directly actionable.
- Target files: `wave/tooling/cli.go`, `wave/tooling/events.go`.

## RULES

- No non-JSON authored config path.
- No builder-pattern APIs in Go.
- No duplicate default-path APIs.
- No back-compat adapters while sub-1.0.
- Keep runtime/build boundaries strict.

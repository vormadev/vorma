# kit/contextutil Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/contextutil`

## Scope

Package-owned typed context/request storage behavior for `Store[T]` in
`kit/contextutil/**`.

Current evidence note:

- Requirements are mined from `kit/contextutil/contextutil.go` and
  `kit/contextutil/contextutil_test.go`.

## Requirements

- `KIT-CONTEXTUTIL-001` Store construction contract. `NewStore[T](key)` MUST
  return a non-nil `*Store[T]` with an internal key wrapper pointer for context
  lookups.
- `KIT-CONTEXTUTIL-002` Store-key non-collision contract. Distinct store
  instances MUST not collide in context lookups, even when created with the same
  string key name.
- `KIT-CONTEXTUTIL-003` Context write contract. `GetContextWithValue(c, val)`
  MUST return a derived context from `context.WithValue(c, s.key, val)`.
- `KIT-CONTEXTUTIL-004` Context read contract. `GetValueFromContext(c)` MUST
  read by store key and return typed value via `genericsutil.AssertOrZero[T]`.
- `KIT-CONTEXTUTIL-005` Missing/incompatible fallback contract.
  `GetValueFromContext` MUST return the zero value of `T` when no compatible
  value is present for the store key.
- `KIT-CONTEXTUTIL-006` Request write contract. `GetRequestWithContext(r, val)`
  MUST return `r.WithContext(...)` where the updated context is produced by
  `GetContextWithValue(r.Context(), val)`.
- `KIT-CONTEXTUTIL-007` Generic instantiation contract. Store context write/read
  and request update behavior MUST hold across tested generic instantiations
  (scalar and struct payload types).

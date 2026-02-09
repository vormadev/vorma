# kit/contextutil Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/contextutil`

## Scope

Package-owned typed context/request storage behavior for `Store[T]`.

## Requirements

- `KIT-CONTEXTUTIL-001` Store construction behavior.
`NewStore(key)` MUST return a non-nil typed store instance with an internal key unique to that store instance.
- `KIT-CONTEXTUTIL-002` Store key non-collision behavior.
Distinct `Store` instances MUST not collide in context lookups, even when created with the same string key name.
- `KIT-CONTEXTUTIL-003` Context write behavior.
`GetContextWithValue` MUST return a derived context containing the provided value under the store key.
- `KIT-CONTEXTUTIL-004` Context read behavior.
`GetValueFromContext` MUST return the stored typed value for that store key when present and type-compatible.
- `KIT-CONTEXTUTIL-005` Missing/incompatible value fallback behavior.
`GetValueFromContext` MUST return `T` zero value when no compatible value is present for the store key.
- `KIT-CONTEXTUTIL-006` Request context write behavior.
`GetRequestWithContext` MUST return a request with context updated via store key/value insertion.
- `KIT-CONTEXTUTIL-007` Generic-type support behavior.
`Store[T]` context write/read and request context write behavior MUST hold across tested generic instantiations.

# kit/set Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/set`

## Scope

Package-owned set semantics for `Set[T comparable]`, including construction,
insertion, and membership checks.

## Requirements

- `KIT-SET-001` Set representation contract. `Set[T]` MUST behave as a
  map-backed set keyed by comparable values with zero-size struct values.
- `KIT-SET-002` Constructor behavior. `New[T]()` MUST return an initialized
  empty set usable for immediate insertions.
- `KIT-SET-003` Nil-receiver add behavior. Calling `Add` on a nil set receiver
  MUST allocate a new set, insert the value, and return the initialized set.
- `KIT-SET-004` Fluent add behavior. `Add` MUST return a `Set[T]` value that can
  be fluently chained.
- `KIT-SET-005` Add idempotence behavior. Adding an existing value MUST preserve
  set membership semantics without error.
- `KIT-SET-006` Membership query behavior. `Contains` MUST return true for
  present values and false for absent values, including nil-set lookups.

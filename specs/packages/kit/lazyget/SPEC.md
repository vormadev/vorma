# kit/lazyget Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/lazyget`

## Scope

Package-owned lazy initialization behavior for `Cache[T]` and the deprecated `New` helper.

## Requirements

- `KIT-LAZYGET-001` `Cache.Get` lazy-once behavior.
`Cache.Get(initFunc)` MUST evaluate initialization exactly once and return the initialized value for all calls.
- `KIT-LAZYGET-002` `Cache.Get` concurrency behavior.
Concurrent calls to `Cache.Get` on the same cache instance MUST remain safe and MUST still perform only one initialization.
- `KIT-LAZYGET-003` `Cache.Get` initializer-binding behavior.
The first `initFunc` observed by `Cache.Get` MUST determine the cache initializer; later `initFunc` arguments MUST be ignored.
- `KIT-LAZYGET-004` `Cache.Get` nil-initializer behavior.
Calling `Cache.Get(nil)` MUST panic when value initialization is invoked.
- `KIT-LAZYGET-005` `Cache.Get` panic-stickiness behavior.
If initializer panics, subsequent calls MUST re-panic without re-running initializer.
- `KIT-LAZYGET-006` `Cache.Get` generic-type behavior.
`Cache.Get` MUST preserve expected once semantics across supported generic value types (scalars, slices, structs, pointers).
- `KIT-LAZYGET-007` `New` constructor behavior.
`New(fn)` MUST return a getter with `sync.OnceValue` once-only initialization semantics.
- `KIT-LAZYGET-008` `New` nil-initializer behavior.
`New(nil)` MUST return callable getter that panics when invoked.
- `KIT-LAZYGET-009` `New` panic-stickiness behavior.
If `New` initializer panics, subsequent getter calls MUST re-panic without re-running initializer.

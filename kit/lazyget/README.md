# kit/lazyget

`github.com/vormadev/vorma/kit/lazyget`

## Purpose

Use `kit/lazyget` when you want a zero-value struct field that lazily initializes once.

For package-level lazy getters, prefer the Go standard library directly:

```go
var getConfig = sync.OnceValue(loadConfig)
```

`lazyget.New` exists for compatibility and is deprecated.

## Import

```go
import "github.com/vormadev/vorma/kit/lazyget"
```

## Primary API: `Cache[T]`

`Cache[T]` is the main value of this package.

```go
type Service struct {
	db lazyget.Cache[*DB]
}

func (s *Service) DB() *DB {
	return s.db.Get(connectDB)
}
```

Behavior:

- Initialization function is executed at most once.
- Concurrent callers block until first initialization completes, then all receive the cached value.
- If initialization panics, future calls panic with the same value.
- Passing `nil` init functions panics when invoked (same behavior as calling a nil function in Go).

## Compatibility API

- `func New[T any](fn func() T) func() T`  
  Deprecated. Equivalent to `sync.OnceValue(fn)`.

---
title: lazyget
description: Generic lazy initialization with sync.Once semantics.
---

```go
import "github.com/vormadev/vorma/kit/lazyget"
```

## Cache

Low-level primitive for lazy struct fields:

```go
type Cache[T any] struct{}
func (v *Cache[T]) Get(initFunc func() T) T
```

Example:

```go
type Service struct {
    db lazyget.Cache[*DB]
}

func (s *Service) DB() *DB {
    return s.db.Get(connectDB)
}
```

## New

Create a package-level lazy getter (useful for deferring panics until first
use):

```go
func New[T any](fn func() T) func() T
```

Example:

```go
var getConfig = lazyget.New(func() Config {
    cfg, err := loadConfig()
    if err != nil {
        panic(err) // deferred until first call
    }
    return cfg
})

// Later:
cfg := getConfig()
```

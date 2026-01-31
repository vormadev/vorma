---
title: genericsutil
description: Helpers for type erasure patterns and generic zero values.
---

```go
import "github.com/vormadev/vorma/kit/genericsutil"
```

## Zero Value Utilities

Get zero value of any type:

```go
func Zero[T any]() T
```

Type assert or return zero:

```go
func AssertOrZero[T any](v any) T
```

Return field value or default if field is zero:

```go
func OrDefault[F comparable](field F, defaultVal F) F
```

## Zero Helper (for type erasure)

Interface for accessing zero values of erased types:

```go
type AnyZeroHelper interface {
    I() any     // zero value of I
    O() any     // zero value of O
    IPtr() any  // new(I)
    OPtr() any  // new(O)
}
```

Generic implementation:

```go
type ZeroHelper[I any, O any] struct{}
```

## None Type

Alias for empty struct:

```go
type None = struct{}
```

Check if value is empty struct (or pointer to one):

```go
func IsNone(v any) bool
```

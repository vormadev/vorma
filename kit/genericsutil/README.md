# kit/genericsutil

`github.com/vormadev/vorma/kit/genericsutil`

Generic helpers for zero values, fallback assertions, and type-erased generic pipelines.

## Import

```go
import "github.com/vormadev/vorma/kit/genericsutil"
```

## Common Usage

### Safe type assertion with fallback

```go
v := any("42")
n := genericsutil.AssertOrZero[int](v) // 0 (assertion failed)
```

Use this when fallback-to-zero is intentional.  
Avoid it when assertion failure should be explicit (otherwise bugs can be hidden).

### Defaulting zero values

```go
port := genericsutil.OrDefault(cfg.Port, 8080)
```

### Generic zero helper for erased APIs

```go
type Convert[I any, O any] struct {
	genericsutil.ZeroHelper[I, O]
}

var z genericsutil.AnyZeroHelper = Convert[int, string]{}
_ = z.I()    // any(0)
_ = z.O()    // any("")
_ = z.IPtr() // *int
_ = z.OPtr() // *string
```

## `None` and `IsNone`

`None` is an alias of `struct{}`:

```go
type Ack = genericsutil.None
```

`IsNone` returns true only for:

- `struct{}`
- `*struct{}`

Named empty-struct types are not considered `None`.

## API Reference

- `type AnyZeroHelper`
- `type None`
- `type ZeroHelper[I any, O any]`
- `func Zero[T any]() T`
- `func AssertOrZero[T any](v any) T`
- `func OrDefault[F comparable](field F, defaultVal F) F`
- `func IsNone(v any) bool`
- `func (ZeroHelper[I, O]) I() any`
- `func (ZeroHelper[I, O]) O() any`
- `func (ZeroHelper[I, O]) IPtr() any`
- `func (ZeroHelper[I, O]) OPtr() any`

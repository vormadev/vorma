# kit/set

`github.com/vormadev/vorma/kit/set`

Minimal generic set type backed by `map[T]struct{}`.

## Import

```go
import "github.com/vormadev/vorma/kit/set"
```

## Quick Start

```go
s := set.New[string]().
	Add("admin").
	Add("editor")

if s.Contains("admin") {
	// allowed
}
```

## Zero-Value Behavior

`Set[T]` is safe to use from zero value:

```go
var s set.Set[string]
s = s.Add("a")
```

`Add` auto-initializes nil sets and returns the set for chaining.

## API Reference

- `type Set[T comparable] map[T]struct{}`
- `func New[T comparable]() Set[T]`
- `func (s Set[T]) Add(val T) Set[T]`
- `func (s Set[T]) Contains(val T) bool`

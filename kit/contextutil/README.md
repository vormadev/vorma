# kit/contextutil

`github.com/vormadev/vorma/kit/contextutil`

Typed wrappers for storing and reading values from `context.Context`.

Use this when you want to avoid repetitive type assertions and keep context keys isolated per concern.

## Import

```go
import "github.com/vormadev/vorma/kit/contextutil"
```

## Basic Usage

```go
type User struct {
	ID string
}

var userStore = contextutil.NewStore[User]("current-user")

ctx := userStore.GetContextWithValue(context.Background(), User{ID: "u-1"})
user := userStore.GetValueFromContext(ctx)
```

## Request Usage

```go
req = userStore.GetRequestWithContext(req, User{ID: "u-1"})
user := userStore.GetValueFromContext(req.Context())
```

## Important Semantics

- Store identity is per `Store[T]` instance, not just key string text.
- Missing or mismatched values return zero-value `T`.
- Because zero-value can also be valid data, treat `GetValueFromContext` as a convenience API, not presence-proof.

Recommended pattern:

- Define store variables once (usually package-level), and reuse them.
- Avoid creating ad-hoc stores in many locations for the same logical value.

## API Reference

- `type Store[T any]`
- `func NewStore[T any](key string) *Store[T]`
- `func (s *Store[T]) GetContextWithValue(c context.Context, val T) context.Context`
- `func (s *Store[T]) GetValueFromContext(c context.Context) T`
- `func (s *Store[T]) GetRequestWithContext(r *http.Request, val T) *http.Request`

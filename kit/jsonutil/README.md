# kit/jsonutil

`github.com/vormadev/vorma/kit/jsonutil`

Thin wrappers around `encoding/json` with consistent error context.

## Import

```go
import "github.com/vormadev/vorma/kit/jsonutil"
```

## When To Use

Use this package when you want simple JSON encode/decode helpers with stable wrapped error messages.

If you need advanced behavior (streaming decoders, `DisallowUnknownFields`, custom encoder options), use `encoding/json` directly.

## Examples

```go
type User struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

payload, err := jsonutil.Serialize(User{Name: "Ada", Age: 30})
if err != nil {
	return err
}

user, err := jsonutil.Parse[User](payload)
if err != nil {
	return err
}
_ = user
```

## Error Contract

- `Serialize` returns `error encoding JSON: ...` on marshal failure.
- `Parse[T]` returns the zero value of `T` plus `error decoding JSON: ...` on unmarshal failure.

## API Reference

- `type JSONString string`
- `func Serialize(v any) ([]byte, error)`
- `func Parse[T any](data []byte) (T, error)`

# kit/bytesutil

`github.com/vormadev/vorma/kit/bytesutil`

Small helpers for:

- base64 (standard)
- base64 URL-safe raw (no padding)
- base32 raw (no padding)
- gob encode/decode

## Import

```go
import "github.com/vormadev/vorma/kit/bytesutil"
```

## Common Usage

### Base64

```go
raw := []byte("hello")
enc := bytesutil.ToBase64(raw)
dec, err := bytesutil.FromBase64(enc)
```

### URL-safe token encoding

```go
token := bytesutil.ToBase64URLRaw([]byte("user:123"))
plain, err := bytesutil.FromBase64URLRaw(token)
```

### Gob round-trip

```go
type Session struct {
	UserID string
	Role   string
}

payload, err := bytesutil.ToGob(Session{UserID: "u1", Role: "admin"})
if err != nil {
	return err
}

session, err := bytesutil.FromGob[Session](payload)
if err != nil {
	return err
}
_ = session
```

### Decode into an existing destination

```go
var session Session
if err := bytesutil.FromGobInto(payload, &session); err != nil {
	return err
}
```

## Behavior Notes

- `ToGob` returns an error for typed nil pointers.
- `FromGob` and `FromGobInto` return errors for nil/invalid gob bytes.
- `FromGobInto` expects a destination pointer.
- If you encode interface values with gob, register concrete types as needed
  (`encoding/gob` rules apply).

## API Reference

- `func ToBase64(b []byte) string`
- `func FromBase64(s string) ([]byte, error)`
- `func ToBase64URLRaw(b []byte) string`
- `func FromBase64URLRaw(s string) ([]byte, error)`
- `func ToBase32Raw(b []byte) string`
- `func FromBase32Raw(s string) ([]byte, error)`
- `func ToGob(src any) ([]byte, error)`
- `func FromGob[T any](gobBytes []byte) (T, error)`
- `func FromGobInto(gobBytes []byte, destPtr any) error`

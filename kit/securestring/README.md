# kit/securestring

`github.com/vormadev/vorma/kit/securestring`

String-oriented encryption wrapper over `kit/securebytes`.

It serializes a value to encrypted bytes and returns base64 text, which is
useful for cookies, headers, and JSON fields.

## Import

```go
import "github.com/vormadev/vorma/kit/securestring"
```

## Quick Start

```go
keys := getKeyset() // *keyset.Keyset

sealed, err := securestring.Serialize(keys, map[string]string{"uid": "u-1"})
if err != nil {
	return err
}

payload, err := securestring.Parse[map[string]string](keys, sealed)
if err != nil {
	return err
}
_ = payload
```

## Relationship to `securebytes`

- `securebytes` is the core crypto/serialization layer.
- `securestring` adds base64 encoding/decoding.

Inherited behavior from `securebytes`:

- key rotation support via ordered keysets
- ciphertext integrity checks
- gob serialization constraints (for example concrete type registration for
  interface payloads)

## Size Limits

- `MaxBase64Size` is the maximum accepted encoded input size for `Parse`.
- Empty input is rejected.
- Oversized base64 input is rejected before decode.
- Invalid base64 input returns an error.

`MaxBase64Size` is derived from `securebytes.MaxSize` using base64 expansion
math.

## API Reference

- `const MaxBase64Size`
- `type SecureString string`
- `func Serialize(ks *keyset.Keyset, rv securebytes.RawValue) (SecureString, error)`
- `func Parse[T any](ks *keyset.Keyset, ss SecureString) (T, error)`

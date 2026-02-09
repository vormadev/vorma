# kit/securebytes

`github.com/vormadev/vorma/kit/securebytes`

Encrypt/decrypt arbitrary gob-serializable values using keysets.

Use this package when you need encrypted binary payloads with
key-rotation-compatible reads.

## Import

```go
import "github.com/vormadev/vorma/kit/securebytes"
```

## Quick Start

```go
keys := getKeyset() // *keyset.Keyset

sealed, err := securebytes.Serialize(keys, map[string]string{"uid": "u-1"})
if err != nil {
	return err
}

payload, err := securebytes.Parse[map[string]string](keys, sealed)
if err != nil {
	return err
}
_ = payload
```

## Key Rotation Behavior

- `Serialize` encrypts with the first key in the keyset.
- `Parse` tries keys until one decrypts successfully.

Rotation rollout pattern:

1. prepend new key (new first key)
2. keep old keys in keyset during transition
3. remove old keys after legacy ciphertext is no longer needed

## Size and Format

- `MaxSize` is the hard ciphertext limit (`1 MiB`).
- plaintext format is: `[version-byte][gob-payload]`.
- `Parse` rejects empty/oversized values and unsupported version bytes.

## Gob Constraints

- Values must be gob-encodable.
- Unsupported kinds (for example funcs/channels) fail serialization.
- For interface payloads, register concrete types as required by `encoding/gob`.

## Security Notes

- Encryption uses XChaCha20-Poly1305.
- Package design assumes key-index timing disclosure is acceptable for the use
  case (as documented in source comments).

## API Reference

- `const MaxSize`
- `type SecureBytes []byte`
- `type RawValue any`
- `func Serialize(ks *keyset.Keyset, rv RawValue) (SecureBytes, error)`
- `func Parse[T any](ks *keyset.Keyset, sb SecureBytes) (T, error)`

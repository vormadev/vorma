---
title: securebytes
description:
    Encrypt and decrypt arbitrary values using XChaCha20-Poly1305 with key
    rotation support.
---

```go
import "github.com/vormadev/vorma/kit/securebytes"
```

## Types

```go
type SecureBytes []byte  // encrypted ciphertext
type RawValue any        // any gob-serializable value
const MaxSize = 1 << 20  // 1MB limit
```

## Functions

Serialize value to encrypted bytes (uses first key in keyset):

```go
func Serialize(ks *keyset.Keyset, rv RawValue) (SecureBytes, error)
```

Parse encrypted bytes back to typed value (tries all keys for rotation support):

```go
func Parse[T any](ks *keyset.Keyset, sb SecureBytes) (T, error)
```

## Example

```go
keys := keyset.MustAppKeyset(keyset.AppKeysetConfig{...})
encKeys := keys.HKDF("encryption")

// Encrypt
sb, err := securebytes.Serialize(encKeys(), myStruct)

// Decrypt
result, err := securebytes.Parse[MyStruct](encKeys(), sb)
```

## Notes

- Uses XChaCha20-Poly1305 for encryption
- Values are gob-encoded before encryption
- Versioned format for future compatibility
- Not timing-attack resistant (index of successful key may leak)

---
title: cryptoutil
description:
    Cryptographic utilities for signing, encryption, hashing, and key
    derivation.
---

```go
import "github.com/vormadev/vorma/kit/cryptoutil"
```

## Types

```go
const KeySize = 32
type Key32 = *[KeySize]byte
type Base64 = string
```

## Errors

```go
var ErrSecretKeyIsNil, ErrCipherTextTooShort, ErrHMACInvalid error
```

## Random

```go
func RandomBytes(byteLen int) ([]byte, error)
```

## Symmetric Signing (HMAC-SHA-512-256)

Sign message (returns signature || message):

```go
func SignSymmetric(msg []byte, secretKey Key32) ([]byte, error)
```

Verify and extract message:

```go
func VerifyAndReadSymmetric(signedMsg []byte, secretKey Key32) ([]byte, error)
```

## Asymmetric Signing (Ed25519)

Verify and extract message:

```go
func VerifyAndReadAsymmetric(signedMsg []byte, publicKey Key32) ([]byte, error)
func VerifyAndReadAsymmetricBase64(signedMsg, publicKey Base64) ([]byte, error)
```

## Hashing

```go
func Sha256Hash(msg []byte) []byte
```

## HMAC-SHA-256

```go
func HmacSha256(msg []byte, key []byte) ([]byte, error)
func ValidateHmacSha256(attemptedMsg, attemptedKey, knownGoodMAC []byte) (bool, error)
```

## Key Derivation (HKDF-SHA-256)

Derives 32-byte key; salt/info can be nil/empty:

```go
func HkdfSha256(secretKey Key32, salt []byte, info string) (Key32, error)
```

## Symmetric Encryption

XChaCha20-Poly1305 (recommended):

```go
func EncryptSymmetricXChaCha20Poly1305(msg []byte, secretKey Key32) ([]byte, error)
func DecryptSymmetricXChaCha20Poly1305(encryptedMsg []byte, secretKey Key32) ([]byte, error)
```

AES-256-GCM:

```go
func EncryptSymmetricAESGCM(msg []byte, secretKey Key32) ([]byte, error)
func DecryptSymmetricAESGCM(encryptedMsg []byte, secretKey Key32) ([]byte, error)
```

Generic (bring your own AEAD):

```go
type ToAEADFunc func(secretKey Key32) (cipher.AEAD, error)
var ToAEADFuncXChaCha20Poly1305, ToAEADFuncAESGCM ToAEADFunc
func EncryptSymmetricGeneric(toAEADFunc ToAEADFunc, msg []byte, secretKey Key32) ([]byte, error)
func DecryptSymmetricGeneric(toAEADFunc ToAEADFunc, ciphertext []byte, secretKey Key32) ([]byte, error)
```

## Key Conversion

```go
func ToKey32(b []byte) (Key32, error)
func FromKey32(key Key32) ([]byte, error)
```

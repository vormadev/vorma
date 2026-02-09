# kit/cryptoutil

`github.com/vormadev/vorma/kit/cryptoutil`

`cryptoutil` provides focused crypto primitives for common backend tasks:

- random bytes and fixed-size key helpers
- symmetric and asymmetric signature verification
- SHA-256 and HMAC-SHA-256
- HKDF-SHA-256 key derivation
- authenticated encryption (XChaCha20-Poly1305 and AES-GCM)

This package assumes callers enforce reasonable input sizes.

## Import

```go
import "github.com/vormadev/vorma/kit/cryptoutil"
```

## Quick Start

### Convert raw key bytes into `Key32`

```go
raw := make([]byte, cryptoutil.KeySize) // 32 bytes
if _, err := rand.Read(raw); err != nil {
	return err
}
key, err := cryptoutil.ToKey32(raw)
if err != nil {
	return err
}
```

### Encrypt + decrypt with XChaCha20-Poly1305 (recommended default)

```go
encrypted, err := cryptoutil.EncryptSymmetricXChaCha20Poly1305([]byte("hello"), key)
if err != nil {
	return err
}
decrypted, err := cryptoutil.DecryptSymmetricXChaCha20Poly1305(encrypted, key)
if err != nil {
	return err
}
_ = decrypted
```

### Compute and validate HMAC-SHA-256

```go
mac, err := cryptoutil.HmacSha256([]byte("payload"), []byte("secret"))
if err != nil {
	return err
}
valid, err := cryptoutil.ValidateHmacSha256([]byte("payload"), []byte("secret"), mac)
if err != nil {
	return err
}
if !valid {
	return errors.New("invalid MAC")
}
```

### Derive context-specific keys with HKDF

```go
signingKey, err := cryptoutil.HkdfSha256(key, []byte("app-salt"), "signing")
if err != nil {
	return err
}
_ = signingKey
```

## Signing and Verification

### Symmetric signing

- `SignSymmetric` prepends an HMAC digest to the message.
- `VerifyAndReadSymmetric` validates and strips the digest.

Use this when both signer and verifier share the same secret key.

### Asymmetric verification

- `VerifyAndReadAsymmetric` verifies Ed25519 signatures using a 32-byte public
  key.
- `VerifyAndReadAsymmetricBase64` does the same for base64-encoded inputs.

## Encryption Behavior

- `EncryptSymmetricGeneric` prepends a random nonce to the ciphertext.
- `DecryptSymmetricGeneric` expects that nonce-prefixed format.
- XChaCha20-Poly1305 and AES-GCM wrappers call the generic functions with
  built-in AEAD constructors.

## HMAC Validation Semantics

`ValidateHmacSha256` returns:

- `true, nil` when MAC is valid
- `false, nil` when MAC is invalid
- `false, err` on malformed inputs or other computation errors

Callers must check the boolean, not just the error.

## Input Preconditions and Common Errors

- `RandomBytes(byteLen)` rejects negative lengths.
- `ToKey32` requires exactly 32 bytes.
- `FromKey32` requires a non-nil key pointer.
- HMAC helpers reject nil/empty key material.
- `DecryptSymmetric*` returns `ErrCipherTextTooShort` for malformed ciphertext
  payloads.

## API Coverage

### Constants

- `const KeySize`

### Variables

- `var ErrCipherTextTooShort`
- `var ErrHMACInvalid`
- `var ErrSecretKeyIsNil`
- `var ToAEADFuncAESGCM ToAEADFunc`
- `var ToAEADFuncXChaCha20Poly1305 ToAEADFunc`

### Types

- `type Base64`
- `type Key32`
- `type ToAEADFunc`

### Functions

- `func DecryptSymmetricAESGCM(encryptedMsg []byte, secretKey Key32) ([]byte, error)`
- `func DecryptSymmetricGeneric(toAEADFunc ToAEADFunc, ciphertext []byte, secretKey Key32) ([]byte, error)`
- `func DecryptSymmetricXChaCha20Poly1305(encryptedMsg []byte, secretKey Key32) ([]byte, error)`
- `func EncryptSymmetricAESGCM(msg []byte, secretKey Key32) ([]byte, error)`
- `func EncryptSymmetricGeneric(toAEADFunc ToAEADFunc, msg []byte, secretKey Key32) ([]byte, error)`
- `func EncryptSymmetricXChaCha20Poly1305(msg []byte, secretKey Key32) ([]byte, error)`
- `func FromKey32(key Key32) ([]byte, error)`
- `func HkdfSha256(secretKey Key32, salt []byte, info string) (Key32, error)`
- `func HmacSha256(msg []byte, key []byte) ([]byte, error)`
- `func RandomBytes(byteLen int) ([]byte, error)`
- `func Sha256Hash(msg []byte) []byte`
- `func SignSymmetric(msg []byte, secretKey Key32) ([]byte, error)`
- `func ToKey32(b []byte) (Key32, error)`
- `func ValidateHmacSha256(attemptedMsg, attemptedKey, knownGoodMAC []byte) (bool, error)`
- `func VerifyAndReadAsymmetric(signedMsg []byte, publicKey Key32) ([]byte, error)`
- `func VerifyAndReadAsymmetricBase64(signedMsg, publicKey Base64) ([]byte, error)`
- `func VerifyAndReadSymmetric(signedMsg []byte, secretKey Key32) ([]byte, error)`

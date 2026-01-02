// Package cryptoutil provides utility functions for cryptographic operations.
// It is the consumer's responsibility to ensure that inputs are reasonably
// sized so as to avoid memory exhaustion attacks.
package cryptoutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"

	"github.com/vormadev/vorma/kit/bytesutil"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/nacl/auth"
)

const KeySize = 32

// Alias for a pointer to a size 32 byte array.
type Key32 = *[KeySize]byte

type Base64 = string

var (
	ErrSecretKeyIsNil     = errors.New("secret key is nil")
	ErrCipherTextTooShort = errors.New("ciphertext too short")
	ErrHMACInvalid        = errors.New("HMAC is invalid")
)

// Random returns a slice of cryptographically random bytes of length byteLen.
func RandomBytes(byteLen int) ([]byte, error) {
	r := make([]byte, byteLen)
	if _, err := rand.Read(r); err != nil {
		return nil, err
	}
	return r, nil
}

/////////////////////////////////////////////////////////////////////
/////// SYMMETRIC MESSAGE SIGNING
/////////////////////////////////////////////////////////////////////

// SignSymmetric signs a message using a symmetric key. It is a convenience
// wrapper around the nacl/auth package, which uses HMAC-SHA-512-256.
func SignSymmetric(msg []byte, secretKey Key32) ([]byte, error) {
	if secretKey == nil {
		return nil, errors.New("secret key is required")
	}
	digest := auth.Sum(msg, secretKey)
	signedMsg := make([]byte, auth.Size+len(msg))
	copy(signedMsg, digest[:])
	copy(signedMsg[auth.Size:], msg)
	return signedMsg, nil
}

// VerifyAndReadSymmetric verifies a signed message using a symmetric key and
// returns the original message. It is a convenience wrapper around the
// nacl/auth package, which uses HMAC-SHA-512-256.
func VerifyAndReadSymmetric(signedMsg []byte, secretKey Key32) ([]byte, error) {
	if secretKey == nil {
		return nil, errors.New("secret key is required")
	}
	if len(signedMsg) < auth.Size {
		return nil, errors.New("invalid signature")
	}
	digest := make([]byte, auth.Size)
	copy(digest, signedMsg[:auth.Size])
	msg := signedMsg[auth.Size:]
	if !auth.Verify(digest, msg, secretKey) {
		return nil, errors.New("invalid signature")
	}
	return msg, nil
}

/////////////////////////////////////////////////////////////////////
/////// ASYMMETRIC MESSAGE SIGNING
/////////////////////////////////////////////////////////////////////

// VerifyAndReadAsymmetric verifies a signed message using an Ed25519 public key and
// returns the original message.
func VerifyAndReadAsymmetric(signedMsg []byte, publicKey Key32) ([]byte, error) {
	if publicKey == nil {
		return nil, errors.New("public key is required")
	}
	if len(signedMsg) < ed25519.SignatureSize {
		return nil, errors.New("message shorter than signature size")
	}

	ok := ed25519.Verify(publicKey[:], signedMsg[ed25519.SignatureSize:], signedMsg[:ed25519.SignatureSize])
	if !ok {
		return nil, errors.New("invalid signature")
	}

	return signedMsg[ed25519.SignatureSize:], nil
}

// VerifyAndReadAsymmetricBase64 verifies a signed message using a base64
// encoded Ed25519 public key and returns the original message.
func VerifyAndReadAsymmetricBase64(signedMsg, publicKey Base64) ([]byte, error) {
	signedMsgBytes, err := bytesutil.FromBase64(signedMsg)
	if err != nil {
		return nil, err
	}

	publicKeyBytes, err := bytesutil.FromBase64(publicKey)
	if err != nil {
		return nil, err
	}
	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return nil, errors.New("invalid public key size")
	}

	return VerifyAndReadAsymmetric(signedMsgBytes, Key32(publicKeyBytes))
}

/////////////////////////////////////////////////////////////////////
/////// SHA-256 HASH
/////////////////////////////////////////////////////////////////////

// Sha256Hash returns the SHA-256 hash of a message as a byte slice.
func Sha256Hash(msg []byte) []byte {
	hash := sha256.Sum256(msg)
	return hash[:]
}

/////////////////////////////////////////////////////////////////////
/////// HMAC-SHA-256
/////////////////////////////////////////////////////////////////////

// HmacSha256 computes the HMAC-SHA-256 of a message using secret key.
// As a security precaution, returns an error if the key is nil or empty.
// If this isn't what you want, use the standard library directly.
func HmacSha256(msg []byte, key []byte) ([]byte, error) {
	if key == nil {
		return nil, errors.New("key is nil")
	}
	if len(key) == 0 {
		return nil, errors.New("key is empty")
	}
	mac := hmac.New(sha256.New, key[:])
	if _, err := mac.Write(msg); err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil
}

// ValidateHmacSha256 constant-time compares the HMAC-SHA-256 of an attempted
// message and attempted key against a known good MAC. Returns true if the
// resulting MAC is valid and false if it is not. Does NOT necessarily return
// an error if the MAC is invalid, so callers must rely on the boolean return
// value to determine validity.
func ValidateHmacSha256(attemptedMsg, attemptedKey, knownGoodMAC []byte) (bool, error) {
	if attemptedKey == nil {
		return false, errors.New("attemptedKey is nil")
	}
	if len(knownGoodMAC) != sha256.Size {
		return false, errors.New("knownGoodMAC must be 32 bytes")
	}
	attemptedMAC, err := HmacSha256(attemptedMsg, attemptedKey)
	if err != nil {
		return false, err
	}
	if !hmac.Equal(attemptedMAC, knownGoodMAC) {
		return false, nil
	}
	return true, nil
}

/////////////////////////////////////////////////////////////////////
/////// HKDF SHA-256
/////////////////////////////////////////////////////////////////////

// HkdfSha256 derives a new cryptographic key from a 32-byte secret
// key using HKDF. It uses SHA-256 as the hash function and returns
// a 32-byte key. Salt and/or info can be nil.
func HkdfSha256(secretKey Key32, salt []byte, info string) (Key32, error) {
	if secretKey == nil {
		return nil, ErrSecretKeyIsNil
	}
	dk, err := hkdf.Key(sha256.New, secretKey[:], salt, info, KeySize)
	if err != nil {
		return nil, err
	}
	return Key32(dk), nil
}

/////////////////////////////////////////////////////////////////////
/////// SYMMETRIC ENCRYPTION
/////////////////////////////////////////////////////////////////////

// EncryptSymmetricXChaCha20Poly1305 encrypts a message using XChaCha20-Poly1305.
func EncryptSymmetricXChaCha20Poly1305(msg []byte, secretKey Key32) ([]byte, error) {
	return EncryptSymmetricGeneric(ToAEADFuncXChaCha20Poly1305, msg, secretKey)
}

// DecryptSymmetricXChaCha20Poly1305 decrypts a message using XChaCha20-Poly1305.
func DecryptSymmetricXChaCha20Poly1305(encryptedMsg []byte, secretKey Key32) ([]byte, error) {
	return DecryptSymmetricGeneric(ToAEADFuncXChaCha20Poly1305, encryptedMsg, secretKey)
}

// EncryptSymmetricAESGCM encrypts a message using AES-256-GCM.
func EncryptSymmetricAESGCM(msg []byte, secretKey Key32) ([]byte, error) {
	return EncryptSymmetricGeneric(ToAEADFuncAESGCM, msg, secretKey)
}

// DecryptSymmetricAESGCM decrypts a message using AES-256-GCM.
func DecryptSymmetricAESGCM(encryptedMsg []byte, secretKey Key32) ([]byte, error) {
	return DecryptSymmetricGeneric(ToAEADFuncAESGCM, encryptedMsg, secretKey)
}

// ToAEADFuncXChaCha20Poly1305 returns an AEAD function for XChaCha20-Poly1305.
var ToAEADFuncXChaCha20Poly1305 ToAEADFunc = func(secretKey Key32) (cipher.AEAD, error) {
	return chacha20poly1305.NewX(secretKey[:])
}

// ToAEADFuncAESGCM returns an AEAD function for AES-256-GCM.
var ToAEADFuncAESGCM ToAEADFunc = func(secretKey Key32) (cipher.AEAD, error) {
	block, err := aes.NewCipher(secretKey[:])
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}

type ToAEADFunc func(secretKey Key32) (cipher.AEAD, error)

// EncryptSymmetricGeneric encrypts a message using a generic AEAD function.
func EncryptSymmetricGeneric(
	toAEADFunc ToAEADFunc,
	msg []byte,
	secretKey Key32,
) ([]byte, error) {
	if secretKey == nil {
		return nil, ErrSecretKeyIsNil
	}

	aead, err := toAEADFunc(secretKey)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	return aead.Seal(nonce, nonce, msg, nil), nil
}

// DecryptSymmetricGeneric decrypts a message using a generic AEAD function.
func DecryptSymmetricGeneric(
	toAEADFunc ToAEADFunc,
	ciphertext []byte,
	secretKey Key32,
) ([]byte, error) {
	if secretKey == nil {
		return nil, ErrSecretKeyIsNil
	}

	aead, err := toAEADFunc(secretKey)
	if err != nil {
		return nil, err
	}

	nonceSize := aead.NonceSize()

	if len(ciphertext) < nonceSize+aead.Overhead() {
		return nil, ErrCipherTextTooShort
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	return aead.Open(nil, nonce, ciphertext, nil)
}

/////////////////////////////////////////////////////////////////////
/////// CONVERTERS
/////////////////////////////////////////////////////////////////////

func ToKey32(b []byte) (Key32, error) {
	if len(b) != KeySize {
		return nil, errors.New("byte slice must be exactly 32 bytes")
	}
	var key [KeySize]byte
	copy(key[:], b)
	return &key, nil
}

func FromKey32(key Key32) ([]byte, error) {
	if key == nil {
		return nil, errors.New("key is nil")
	}
	return key[:], nil
}

// Package signedcookie provides a secure cookie manager for Go web applications.
package signedcookie

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/keyset"
)

////////////////////////////////////////////////////////////////////
/////// CORE SIGNED COOKIES MANAGER
////////////////////////////////////////////////////////////////////

const SecretSize = 32 // SecretSize is the size, in bytes, of a cookie secret.

// Manager handles the creation, signing, and verification of secure cookies.
type Manager struct {
	keyset *keyset.Keyset
}

// NewManager creates a new Manager instance with the provided secrets.
// It returns an error if no secrets are provided or if any secret is invalid.
func NewManager(secrets keyset.RootSecrets) (*Manager, error) {
	secretsBytes, err := keyset.RootSecretsToRootKeyset(secrets)
	if err != nil {
		return nil, fmt.Errorf("error converting root secrets to keyset: %w", err)
	}
	return &Manager{keyset: secretsBytes}, nil
}

// VerifyAndReadCookieValue retrieves and verifies the value of a signed cookie.
// It returns an error if the cookie is not found or is invalid.
func (m Manager) VerifyAndReadCookieValue(r *http.Request, key string) (string, error) {
	cookie, err := r.Cookie(key)
	if err != nil {
		return "", err
	}
	value, err := m.verifyAndReadValue(cookie.Value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// NewDeletionCookie creates a new cookie that will delete the specified cookie when sent to the client.
func (m Manager) NewDeletionCookie(cookie http.Cookie) *http.Cookie {
	cookie.Value = ""
	cookie.MaxAge = -1
	return &cookie
}

// SignCookie retrieves the value of the provided cookie, signs it, and replaces the value with the signed value.
// If encrypt is true, the value will be encrypted before signing.
func (m Manager) SignCookie(unsignedCookie *http.Cookie, encrypt bool) error {
	signedValue, err := m.signValue(unsignedCookie.Value, encrypt)
	if err != nil {
		return err
	}
	unsignedCookie.Value = signedValue
	return nil
}

// signValue signs the provided value using the latest secret.
// It returns the base64-encoded signed value or an error if signing fails.
// If encrypt is true, the value will be encrypted before signing.
func (m Manager) signValue(unsignedValue string, encrypt bool) (string, error) {
	var prefix byte
	var valueToSign []byte
	firstKey, err := m.keyset.First()
	if err != nil {
		return "", fmt.Errorf("error getting first key from keyset: %w", err)
	}
	if encrypt {
		prefix = 1
		encrypted, err := cryptoutil.EncryptSymmetricXChaCha20Poly1305(
			[]byte(unsignedValue), firstKey,
		)
		if err != nil {
			return "", err
		}
		valueToSign = encrypted
	} else {
		prefix = 0
		valueToSign = []byte(unsignedValue)
	}
	signed, err := cryptoutil.SignSymmetric(valueToSign, firstKey)
	if err != nil {
		return "", err
	}
	return bytesutil.ToBase64(append([]byte{prefix}, signed...)), nil
}

// verifyAndReadValue verifies and reads the signed value.
// It returns the original unsigned value or an error if verification fails.
func (m Manager) verifyAndReadValue(signedValue string) (string, error) {
	bytes, err := bytesutil.FromBase64(signedValue)
	if err != nil {
		return "", fmt.Errorf("error decoding base64: %w", err)
	}
	if len(bytes) < 1 {
		return "", errors.New("invalid signed value")
	}
	prefix := bytes[0]
	signedBytes := bytes[1:]
	return keyset.Attempt(m.keyset,
		func(secret cryptoutil.Key32) (string, error) {
			value, err := cryptoutil.VerifyAndReadSymmetric(signedBytes, secret)
			if err == nil {
				if prefix == 1 {
					decrypted, err := cryptoutil.DecryptSymmetricXChaCha20Poly1305(value, secret)
					if err != nil {
						return "", err
					}
					return string(decrypted), nil
				}
				return string(value), nil
			}
			return "", fmt.Errorf("error verifying signed value: %w", err)
		},
	)
}

////////////////////////////////////////////////////////////////////
/////// SIGNED COOKIE HELPERS
////////////////////////////////////////////////////////////////////

// BaseCookie is a type alias for http.Cookie.
// It is used for providing and overriding default cookie settings.
// Note that the Name, HttpOnly, and Secure fields are ignored.
// The expires field is not ignored, but it will be overridden by
// an explicitly set TTL.
type BaseCookie = http.Cookie

// SignedCookie provides methods for working with signed cookies of a specific type T.
// If Encrypt is true, the cookie value will be encrypted before signing and decrypted
// after a successful verification.
type SignedCookie[T any] struct {
	Manager    *Manager
	TTL        time.Duration
	BaseCookie BaseCookie
	Encrypt    bool
}

// NewSignedCookie creates a new signed cookie with the provided value and optional override settings.
func (sc *SignedCookie[T]) NewSignedCookie(unsignedValue T, overrideBaseCookie *BaseCookie) (*http.Cookie, error) {
	unsignedCookie, err := sc.newUnsignedCookie(unsignedValue, overrideBaseCookie)
	if err != nil {
		return nil, err
	}

	err = sc.Manager.SignCookie(unsignedCookie, sc.Encrypt)
	if err != nil {
		return nil, err
	}

	return unsignedCookie, nil
}

// NewDeletionCookie creates a new cookie that will delete the current cookie when sent to the client.
func (sc *SignedCookie[T]) NewDeletionCookie() *http.Cookie {
	return sc.Manager.NewDeletionCookie(sc.BaseCookie)
}

// VerifyAndReadCookieValue retrieves and verifies the value of the signed cookie from the request.
// It returns the decoded value of type T or an error if retrieval or verification fails.
func (sc *SignedCookie[T]) VerifyAndReadCookieValue(r *http.Request) (T, error) {
	var zeroT T

	value, err := sc.Manager.VerifyAndReadCookieValue(r, sc.BaseCookie.Name)
	if err != nil {
		return zeroT, err
	}

	dataBytes, err := bytesutil.FromBase64(value)
	if err != nil {
		return zeroT, err
	}

	return bytesutil.FromGob[T](dataBytes)
}

// newSecureCookieWithoutValue creates a new secure cookie with the provided name, expiration, and base settings.
// It ensures that the cookie is marked as HTTP-only and secure.
func newSecureCookieWithoutValue(name string, expires *time.Time, baseCookie *BaseCookie) *http.Cookie {
	newCookie := http.Cookie{}

	if baseCookie != nil {
		newCookie = *baseCookie
	}

	newCookie.Name = name
	if expires != nil {
		newCookie.Expires = *expires
	}

	newCookie.HttpOnly = true
	newCookie.Secure = true

	return &newCookie
}

// newUnsignedCookie creates an unsigned cookie with the provided value and settings.
func (sc *SignedCookie[T]) newUnsignedCookie(unsignedValue T, overrideBaseCookie *BaseCookie) (*http.Cookie, error) {
	dataBytes, err := bytesutil.ToGob(unsignedValue)
	if err != nil {
		return nil, err
	}

	var baseCookieToUse BaseCookie
	if overrideBaseCookie != nil {
		baseCookieToUse = *overrideBaseCookie
	} else {
		baseCookieToUse = sc.BaseCookie
	}

	var expires time.Time
	if sc.TTL != 0 {
		expires = time.Now().Add(sc.TTL)
	}

	unsignedCookie := newSecureCookieWithoutValue(sc.BaseCookie.Name, &expires, &baseCookieToUse)
	unsignedCookie.Value = bytesutil.ToBase64(dataBytes)

	return unsignedCookie, nil
}

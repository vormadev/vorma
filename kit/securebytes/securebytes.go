// This package assumes that the caller's use case is not sensitive to
// timing attacks. In other words, it assumes that, even if an attacker
// can figure out the current index of the secret key originally used
// to encrypt the data, that information would not be materially useful
// to them. This is a reasonable assumption for most use cases.
package securebytes

import (
	"fmt"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/keyset"
)

const current_pkg_version byte = 1

const MaxSize = 1 << 20 // 1MB in bytes

type SecureBytes []byte // Encrypted value
type RawValue any       // Any pre-serialization value

func Serialize(ks *keyset.Keyset, rv RawValue) (SecureBytes, error) {
	if rv == nil {
		return nil, fmt.Errorf("invalid raw value: nil value")
	}
	if err := ks.Validate(); err != nil {
		return nil, fmt.Errorf("invalid keyset: %w", err)
	}
	gob_value, err := bytesutil.ToGob(rv)
	if err != nil {
		return nil, fmt.Errorf("error encoding value to gob: %w", err)
	}
	plaintext := append([]byte{current_pkg_version}, gob_value...)
	firstKey, err := ks.First()
	if err != nil {
		return nil, fmt.Errorf("error getting first key from keyset: %w", err)
	}
	ciphertext, err := cryptoutil.EncryptSymmetricXChaCha20Poly1305(plaintext, firstKey)
	if err != nil {
		return nil, fmt.Errorf("error encrypting value: %w", err)
	}
	if len(ciphertext) > MaxSize {
		return nil, fmt.Errorf("ciphertext too large (over 1MB)")
	}
	return SecureBytes(ciphertext), nil
}

func Parse[T any](ks *keyset.Keyset, sb SecureBytes) (T, error) {
	var zeroT T
	if len(sb) == 0 {
		return zeroT, fmt.Errorf("invalid secure bytes: empty value")
	}
	if len(sb) > MaxSize {
		return zeroT, fmt.Errorf("secure bytes too large (over 1MB)")
	}
	if err := ks.Validate(); err != nil {
		return zeroT, fmt.Errorf("invalid keyset: %w", err)
	}
	plaintext, err := keyset.Attempt(ks, func(k cryptoutil.Key32) ([]byte, error) {
		return cryptoutil.DecryptSymmetricXChaCha20Poly1305(sb, k)
	})
	if err != nil {
		return zeroT, fmt.Errorf("error decrypting value: %w", err)
	}
	version := plaintext[0]
	if version != current_pkg_version {
		return zeroT, fmt.Errorf("unsupported SecureBytes version %d", version)
	}
	out, err := bytesutil.FromGob[T](plaintext[1:])
	if err != nil {
		return zeroT, fmt.Errorf("error decoding gob: %w", err)
	}
	return out, nil
}

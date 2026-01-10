// This package assumes that the caller's use case is not sensitive to
// timing attacks. In other words, it assumes that, even if an attacker
// can figure out the current index of the secret key originally used
// to encrypt the data, that information would not be materially useful
// to them. This is a reasonable assumption for most use cases.
// This is a light base64 wrapper over the "securebytes" package.
package securestring

import (
	"fmt"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/keyset"
	"github.com/vormadev/vorma/kit/securebytes"
)

const MaxBase64Size = securebytes.MaxSize + securebytes.MaxSize/3

type SecureString string // Base64-encoded, encrypted value

func Serialize(ks *keyset.Keyset, rv securebytes.RawValue) (SecureString, error) {
	ciphertext, err := securebytes.Serialize(ks, rv)
	if err != nil {
		return "", fmt.Errorf("error serializing raw value: %w", err)
	}
	return SecureString(bytesutil.ToBase64(ciphertext)), nil
}

func Parse[T any](ks *keyset.Keyset, ss SecureString) (T, error) {
	var zeroT T
	if len(ss) == 0 {
		return zeroT, fmt.Errorf("invalid secure string: empty value")
	}
	if len(ss) > MaxBase64Size {
		return zeroT, fmt.Errorf("secure string too large (over 1.33MB)")
	}
	ciphertext, err := bytesutil.FromBase64(string(ss))
	if err != nil {
		return zeroT, fmt.Errorf("error decoding base64: %w", err)
	}
	return securebytes.Parse[T](ks, securebytes.SecureBytes(ciphertext))
}

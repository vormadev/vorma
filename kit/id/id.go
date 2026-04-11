package id

import (
	"crypto/rand"
	"fmt"
)

const defaultCharset = "0123456789" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz"

// New generates a cryptographically random string ID of the specified length.
// By default, IDs consist of mixed-case alphanumeric characters (0-9, A-Z, a-z).
// A single optional custom charset can be provided if you want to use different characters.
// The idLen parameter must be between 0 and 255 inclusive.
// If provided, the custom charset length must be between 1 and 255 inclusive.
func New(idLen uint8, optionalCharset ...string) (string, error) {
	if len(optionalCharset) > 1 {
		return "", fmt.Errorf("at most one optional charset may be provided")
	}

	charset := defaultCharset
	if len(optionalCharset) > 0 {
		charset = optionalCharset[0]
	}
	charsetLen := len(charset)
	if charsetLen == 0 || charsetLen > 255 {
		return "", fmt.Errorf(
			"charset length must be between 1 and 255 inclusive, got %d", charsetLen,
		)
	}
	for _, r := range charset {
		if r > 127 {
			return "", fmt.Errorf("charset must contain only single-byte ASCII characters")
		}
	}
	if idLen == 0 {
		return "", nil
	}
	effectiveTotalValues := (256 / charsetLen) * charsetLen
	charsetBytes := []byte(charset)
	idOutputBytes := make([]byte, idLen)
	randomByteHolder := make([]byte, 1)
	for i := range idLen {
		for {
			_, err := rand.Read(randomByteHolder)
			if err != nil {
				return "", fmt.Errorf("failed to read random bytes: %w", err)
			}
			randomVal := randomByteHolder[0]
			if int(randomVal) < effectiveTotalValues {
				idOutputBytes[i] = charsetBytes[randomVal%byte(charsetLen)]
				break
			}
		}
	}
	return string(idOutputBytes), nil
}

// NewMulti generates multiple cryptographically random string IDs of the specified length and quantity.
// By default, IDs consist of mixed-case alphanumeric characters (0-9, A-Z, a-z).
// A single optional custom charset can be provided if you want to use different characters.
// The idLen parameter must be between 0 and 255 inclusive.
func NewMulti(idLen uint8, quantity uint8, optionalCharset ...string) ([]string, error) {
	ids := make([]string, quantity)
	useOptionalCharset := len(optionalCharset) > 0
	for i := range quantity {
		var id string
		var err error
		if useOptionalCharset {
			id, err = New(idLen, optionalCharset[0])
		} else {
			id, err = New(idLen)
		}
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	return ids, nil
}

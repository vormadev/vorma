// Package bytesutil provides utility functions for byte slice operations.
package bytesutil

import (
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"reflect"
)

// FromBase64 decodes a base64-encoded string into a byte slice.
func FromBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// ToBase64 encodes a byte slice into a base64-encoded string.
func ToBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// FromBase64URLRaw decodes a base64 URL-safe encoded string into a byte slice without padding.
func FromBase64URLRaw(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// ToBase64URLRaw encodes a byte slice into a base64 URL-safe encoded string without padding.
func ToBase64URLRaw(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// FromBase32Raw decodes a base32-encoded string into a byte slice without padding.
func FromBase32Raw(s string) ([]byte, error) {
	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
}

// ToBase32Raw encodes a byte slice into a base32-encoded string without padding.
func ToBase32Raw(b []byte) string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

// ToGob encodes an arbitrary value into a gob-encoded byte slice.
func ToGob(src any) ([]byte, error) {
	rv := reflect.ValueOf(src)
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil, fmt.Errorf("bytesutil.ToGob: cannot encode nil pointer value")
	}
	var a bytes.Buffer
	enc := gob.NewEncoder(&a)
	err := enc.Encode(src)
	if err != nil {
		return nil, fmt.Errorf("bytesutil.ToGob: failed to encode src to bytes: %w", err)
	}
	return a.Bytes(), nil
}

// FromGobInto decodes a gob-encoded byte slice into a destination.
// The destination must be a pointer to the destination type.
func FromGobInto(gobBytes []byte, destPtr any) error {
	if gobBytes == nil {
		return fmt.Errorf("bytesutil.FromGobInto: cannot decode nil bytes")
	}
	if destPtr == nil {
		return fmt.Errorf("bytesutil.FromGobInto: cannot decode into nil destination")
	}
	dec := gob.NewDecoder(bytes.NewReader(gobBytes))
	err := dec.Decode(destPtr)
	if err != nil {
		return fmt.Errorf("bytesutil.FromGobInto: failed to decode bytes into dest: %w", err)
	}
	return nil
}

// FromGob decodes a gob-encoded byte slice into a value of type T.
func FromGob[T any](gobBytes []byte) (T, error) {
	var zeroT T
	if gobBytes == nil {
		return zeroT, fmt.Errorf("bytesutil.FromGob: gobBytes is nil")
	}
	dec := gob.NewDecoder(bytes.NewReader(gobBytes))
	destPtr := new(T)
	err := dec.Decode(destPtr)
	if err != nil {
		return zeroT, fmt.Errorf("bytesutil.FromGob: failed to decode gob bytes: %w", err)
	}
	return *destPtr, nil
}

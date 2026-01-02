package securebytes

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/keyset"
	"golang.org/x/crypto/chacha20poly1305"
)

func randSecrets(n int) keyset.RootSecrets {
	out := make([]keyset.RootSecret, n)
	for i := range out {
		var b [cryptoutil.KeySize]byte
		if _, err := rand.Read(b[:]); err != nil {
			panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
		}
		out[i] = keyset.RootSecret(base64.StdEncoding.EncodeToString(b[:]))
	}
	return out
}

func mustKeys(t *testing.T, n int) *keyset.Keyset {
	secrets, err := keyset.RootSecretsToRootKeyset(randSecrets(n))
	if err != nil {
		t.Fatalf("ParseSecrets error: %v", err)
	}
	return secrets
}

func roundTrip[T comparable](t *testing.T, value T) {
	kcs := mustKeys(t, 1)
	sb, err := Serialize(kcs, value)
	if err != nil {
		t.Fatalf("Serialize failed for value %v: %v", value, err)
	}
	got, err := Parse[T](kcs, sb)
	if err != nil {
		t.Fatalf("Parse failed for value %v: %v", value, err)
	}
	if got != value {
		t.Fatalf("round‑trip mismatch: want %v, got %v", value, got)
	}
}

func roundTripBytes(t *testing.T, value []byte) {
	kcs := mustKeys(t, 1)
	sb, err := Serialize(kcs, value)
	if err != nil {
		t.Fatalf("Serialize failed for byte slice: %v", err)
	}
	got, err := Parse[[]byte](kcs, sb)
	if err != nil {
		t.Fatalf("Parse failed for byte slice: %v", err)
	}
	if len(value) == 0 && len(got) == 0 {
		return // Both empty, consider it a match
	}
	if !reflect.DeepEqual(got, value) {
		t.Fatalf("round‑trip mismatch for bytes: want %v, got %v", value, got)
	}
}

func TestSecureBytes_RoundTrip(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		roundTrip(t, "hello world")
	})

	t.Run("int", func(t *testing.T) {
		roundTrip(t, 42)
	})

	t.Run("struct", func(t *testing.T) {
		type demo struct {
			A int
			B string
		}
		roundTrip(t, demo{A: 7, B: "seven"})
	})

	t.Run("byte slice", func(t *testing.T) {
		data := []byte("raw byte data")
		roundTripBytes(t, data)
	})
}

func TestSecureBytes_RoundTrip_PointerTypes(t *testing.T) {
	kcs := mustKeys(t, 1)

	type demoPtrStruct struct {
		Name  string
		Value int
	}

	t.Run("pointer to struct", func(t *testing.T) {
		original := &demoPtrStruct{Name: "TestPtr", Value: 123}
		sb, err := Serialize(kcs, original)
		if err != nil {
			t.Fatalf("Serialize failed for pointer to struct: %v", err)
		}

		got, err := Parse[*demoPtrStruct](kcs, sb)
		if err != nil {
			t.Fatalf("Parse failed for pointer to struct: %v", err)
		}

		if got == nil {
			t.Fatalf("Parse resulted in nil pointer, want non-nil")
		}
		if original.Name != got.Name || original.Value != got.Value {
			t.Fatalf("round-trip (pointer to struct) mismatch: want %+v, got %+v", *original, *got)
		}
	})

	t.Run("pointer to string", func(t *testing.T) {
		strValue := "hello pointer"
		original := &strValue

		sb, err := Serialize(kcs, original)
		if err != nil {
			t.Fatalf("Serialize failed for pointer to string: %v", err)
		}
		got, err := Parse[*string](kcs, sb)
		if err != nil {
			t.Fatalf("Parse failed for pointer to string: %v", err)
		}
		if got == nil {
			t.Fatalf("Parse resulted in nil string pointer")
		}
		if *original != *got {
			t.Fatalf("round-trip (pointer to string) mismatch: want %q, got %q", *original, *got)
		}
	})

	t.Run("typed nil pointer", func(t *testing.T) {
		var typedNil *demoPtrStruct = nil
		_, err := Serialize(kcs, typedNil)
		if err == nil {
			t.Fatalf("Expected Serialize to fail for typed nil pointer, but it succeeded")
		}
	})
}

func TestSecureBytes_WrongKeyFails(t *testing.T) {
	good := mustKeys(t, 1)
	bad := mustKeys(t, 1)

	sb, err := Serialize(good, "secret data")
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}
	if _, err = Parse[string](bad, sb); err == nil {
		t.Fatalf("expected decryption failure with wrong key")
	}
}

func TestSecureBytes_SizeLimits(t *testing.T) {
	kcs := mustKeys(t, 1)

	t.Run("Serialize payload too large", func(t *testing.T) {
		big := make([]byte, MaxSize+1)
		if _, err := Serialize(kcs, big); err == nil {
			t.Fatalf("expected Serialize to fail for payload >1 MiB")
		}
	})

	t.Run("Parse SecureBytes too large", func(t *testing.T) {
		// Create a SecureBytes that's too large
		oversized := make([]byte, MaxSize+1)
		if _, err := Parse[string](kcs, SecureBytes(oversized)); err == nil {
			t.Fatalf("expected Parse to fail for oversized SecureBytes")
		}
	})

	t.Run("Parse with empty SecureBytes", func(t *testing.T) {
		if _, err := Parse[string](kcs, SecureBytes{}); err == nil {
			t.Fatalf("expected Parse to fail for empty SecureBytes")
		}
	})
}

func TestSecureBytes_KeyRotation(t *testing.T) {
	oldKeyContainer := mustKeys(t, 1)
	newKeyContainer := mustKeys(t, 1)

	uOld := oldKeyContainer.Unwrap()
	uNew := newKeyContainer.Unwrap()

	if reflect.DeepEqual(uOld[0], uNew[0]) {
		t.Fatalf("Test setup error: oldKey and newKey are the same, ensure mustKeys generates unique secrets.")
	}

	rotatedKeys, _ := keyset.FromUnwrapped(keyset.UnwrappedKeyset{uNew[0], uOld[0]})

	value := "sensitive data for rotation"
	sb, err := Serialize(oldKeyContainer, value)
	if err != nil {
		t.Fatalf("Serialize with oldKey failed: %v", err)
	}

	got, err := Parse[string](rotatedKeys, sb)
	if err != nil {
		t.Fatalf("Parse with rotated keys failed: %v", err)
	}
	if got != value {
		t.Fatalf("rotation mismatch: want %q, got %q", value, got)
	}

	valueNew := "new sensitive data"
	sbNew, err := Serialize(newKeyContainer, valueNew)
	if err != nil {
		t.Fatalf("Serialize with newKey failed: %v", err)
	}
	oldFirstRotatedKeys, _ := keyset.FromUnwrapped(keyset.UnwrappedKeyset{uOld[0], uNew[0]})
	gotNew, err := Parse[string](oldFirstRotatedKeys, sbNew)
	if err != nil {
		t.Fatalf("Parse with oldFirstRotatedKeys failed: %v", err)
	}
	if gotNew != valueNew {
		t.Fatalf("rotation mismatch for new key: want %q, got %q", valueNew, gotNew)
	}
}

func TestSecureBytes_EmptyInput(t *testing.T) {
	kcs := mustKeys(t, 1)

	t.Run("empty string", func(t *testing.T) {
		roundTrip(t, "")
	})

	t.Run("empty struct", func(t *testing.T) {
		type empty struct{}
		roundTrip(t, empty{})
	})

	t.Run("nil value (any(nil))", func(t *testing.T) {
		if _, err := Serialize(kcs, nil); err == nil {
			t.Fatalf("expected error when serializing nil (interface{}(nil))")
		}
	})

	t.Run("empty byte slice", func(t *testing.T) {
		emptySlice := []byte{}
		roundTripBytes(t, emptySlice)
	})
}

func TestSecureBytes_InvalidInputs(t *testing.T) {
	kcsValid := mustKeys(t, 1)
	validValue := "some test data for invalid inputs"
	sbValid, errSerialize := Serialize(kcsValid, validValue)
	if errSerialize != nil {
		t.Fatalf("Setup: Serialize for TestSecureBytes_InvalidInputs failed: %v", errSerialize)
	}

	t.Run("Parse tampered ciphertext", func(t *testing.T) {
		if len(sbValid) == 0 {
			t.Skip("Skipping tamper test for zero-length ciphertext, which is unexpected.")
		}

		tamperedCiphertext := make([]byte, len(sbValid))
		copy(tamperedCiphertext, sbValid)

		idxToTamper := len(tamperedCiphertext) / 2
		tamperedCiphertext[idxToTamper] = tamperedCiphertext[idxToTamper] ^ 0x01

		if _, err := Parse[string](kcsValid, SecureBytes(tamperedCiphertext)); err == nil {
			t.Fatalf("expected error for tampered ciphertext")
		}
	})

	t.Run("Parse with no keys", func(t *testing.T) {
		noKeys := &keyset.Keyset{}
		if _, err := Parse[string](noKeys, sbValid); err == nil {
			t.Fatalf("expected error when parsing with no keys")
		}
	})

	t.Run("Parse with only nil keys", func(t *testing.T) {
		nilKeys, _ := keyset.FromUnwrapped(keyset.UnwrappedKeyset{nil, nil})
		if _, err := Parse[string](nilKeys, sbValid); err == nil {
			t.Fatalf("expected error for only nil keys, as no valid key would be found")
		}
	})

	t.Run("Parse with truncated ciphertext", func(t *testing.T) {
		if len(sbValid) > 10 {
			truncated := sbValid[:10]
			if _, err := Parse[string](kcsValid, truncated); err == nil {
				t.Fatalf("expected error for truncated ciphertext")
			}
		}
	})
}

func TestSecureBytes_Version(t *testing.T) {
	kcs := mustKeys(t, 1)
	uks := kcs.Unwrap()
	if uks[0] == nil {
		t.Fatal("Test setup: kcs[0] is nil")
	}

	value := "test message for versioning"
	sb, err := Serialize(kcs, value)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Manually decrypt to access version byte
	plaintext, err := cryptoutil.DecryptSymmetricXChaCha20Poly1305(sb, uks[0])
	if err != nil {
		t.Fatalf("Manual DecryptSymmetricXChaCha20Poly1305 failed: %v", err)
	}

	if len(plaintext) == 0 {
		t.Fatal("Manual decryption resulted in empty plaintext")
	}
	originalVersion := plaintext[0]
	plaintext[0] = 99 // Invalid version

	modifiedCiphertext, err := cryptoutil.EncryptSymmetricXChaCha20Poly1305(plaintext, uks[0])
	if err != nil {
		t.Fatalf("Manual EncryptSymmetricXChaCha20Poly1305 failed: %v", err)
	}

	_, err = Parse[string](kcs, SecureBytes(modifiedCiphertext))
	if err == nil {
		t.Fatalf("expected version error when parsing with modified version byte")
	}

	// Restore and verify it works again
	plaintext[0] = originalVersion
	validCiphertextAgain, _ := cryptoutil.EncryptSymmetricXChaCha20Poly1305(plaintext, uks[0])
	if _, err := Parse[string](kcs, SecureBytes(validCiphertextAgain)); err != nil {
		t.Fatalf("Failed to Parse with original version after modification test: %v", err)
	}
}

func TestSecureBytes_ComplexTypes(t *testing.T) {
	kcs := mustKeys(t, 1)

	t.Run("time serialization", func(t *testing.T) {
		type TimeData struct {
			Created time.Time
			Updated time.Time
		}

		now := time.Now()
		original := TimeData{
			Created: now,
			Updated: now.Add(24 * time.Hour),
		}

		sb, err := Serialize(kcs, original)
		if err != nil {
			t.Fatalf("Serialize failed for TimeData: %v", err)
		}

		var decoded TimeData
		decoded, err = Parse[TimeData](kcs, sb)
		if err != nil {
			t.Fatalf("Parse failed for TimeData: %v", err)
		}

		if !decoded.Created.Equal(original.Created) {
			t.Errorf("Created time mismatch: want %v, got %v", original.Created, decoded.Created)
		}
		if !decoded.Updated.Equal(original.Updated) {
			t.Errorf("Updated time mismatch: want %v, got %v", original.Updated, decoded.Updated)
		}
	})

	t.Run("channel not serializable", func(t *testing.T) {
		ch := make(chan int)
		if _, err := Serialize(kcs, ch); err == nil {
			t.Fatalf("expected error when serializing channel")
		}
	})

	t.Run("function not serializable", func(t *testing.T) {
		fn := func() {}
		if _, err := Serialize(kcs, fn); err == nil {
			t.Fatalf("expected error when serializing function")
		}
	})

	t.Run("nested structures", func(t *testing.T) {
		type Inner struct {
			Data []byte
			ID   int
		}
		type Outer struct {
			Name  string
			Inner Inner
			Tags  []string
		}

		original := Outer{
			Name: "test",
			Inner: Inner{
				Data: []byte("secret data"),
				ID:   12345,
			},
			Tags: []string{"tag1", "tag2", "tag3"},
		}

		sb, err := Serialize(kcs, original)
		if err != nil {
			t.Fatalf("Serialize failed for nested struct: %v", err)
		}

		var decoded Outer
		decoded, err = Parse[Outer](kcs, sb)
		if err != nil {
			t.Fatalf("Parse failed for nested struct: %v", err)
		}

		if decoded.Name != original.Name ||
			decoded.Inner.ID != original.Inner.ID ||
			!reflect.DeepEqual(decoded.Inner.Data, original.Inner.Data) ||
			!reflect.DeepEqual(decoded.Tags, original.Tags) {
			t.Fatalf("nested struct mismatch: want %+v, got %+v", original, decoded)
		}
	})
}

func TestSecureBytes_Concurrency(t *testing.T) {
	kcs := mustKeys(t, 3)

	const numGoroutines = 50
	const iterationsPerGopher = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errChan := make(chan error, numGoroutines*iterationsPerGopher)

	for i := range numGoroutines {
		go func(gopherID int) {
			defer wg.Done()
			for j := range iterationsPerGopher {
				value := fmt.Sprintf("concurrent-test-gopher-%d-iter-%d", gopherID, j)
				kcsForGoroutine := kcs

				sb, err := Serialize(kcsForGoroutine, value)
				if err != nil {
					errChan <- fmt.Errorf("goroutine %d: Serialize error: %w", gopherID, err)
					return
				}

				got, err := Parse[string](kcsForGoroutine, sb)
				if err != nil {
					errChan <- fmt.Errorf("goroutine %d: Parse error: %w", gopherID, err)
					return
				}

				if got != value {
					errChan <- fmt.Errorf("goroutine %d: value mismatch: want %q, got %q", gopherID, value, got)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	var errs []string
	for err := range errChan {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		t.Errorf("Concurrency test failed with %d errors:\n%s", len(errs), strings.Join(errs, "\n"))
	}
}

func TestSecureBytes_CornerCases(t *testing.T) {
	t.Run("many keys for decryption", func(t *testing.T) {
		numKeys := 20
		keyChain := mustKeys(t, numKeys)
		value := "test with many keys decryption"

		sb, err := Serialize(keyChain, value)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		got, err := Parse[string](keyChain, sb)
		if err != nil {
			t.Fatalf("Parse failed with many keys: %v", err)
		}
		if got != value {
			t.Fatalf("value mismatch with many keys: want %q, got %q", value, got)
		}

		if numKeys > 1 {
			lastKeyIndex := numKeys - 1
			keysWithLastActive, _ := keyset.FromUnwrapped(keyset.UnwrappedKeyset{keyChain.Unwrap()[lastKeyIndex]})

			sbLast, err := Serialize(keysWithLastActive, value)
			if err != nil {
				t.Fatalf("Serialize with last key failed: %v", err)
			}

			gotLast, err := Parse[string](keyChain, sbLast)
			if err != nil {
				t.Fatalf("Parse (last key active) failed: %v", err)
			}
			if gotLast != value {
				t.Fatalf("value mismatch (last key active): want %q, got %q", value, gotLast)
			}
		}
	})

	t.Run("payload nearly max size for Serialize", func(t *testing.T) {
		kcs := mustKeys(t, 1)

		// Calculate overhead
		gobOverheadEstimate := 100
		xchachaOverhead := chacha20poly1305.Overhead
		versionByteOverhead := 1
		totalOverheadEstimate := gobOverheadEstimate + xchachaOverhead + versionByteOverhead

		if totalOverheadEstimate >= MaxSize {
			t.Skip("Overhead estimate is too large for this test relative to MaxSize")
		}

		safePayloadSize := MaxSize - totalOverheadEstimate - 1
		if safePayloadSize <= 0 {
			t.Skipf("Calculated safePayloadSize %d is too small, check estimates or MaxSize", safePayloadSize)
		}

		largeData := make([]byte, safePayloadSize)
		if _, err := rand.Read(largeData); err != nil {
			t.Fatalf("Failed to generate random data for large payload test: %v", err)
		}

		sb, err := Serialize(kcs, largeData)
		if err != nil {
			t.Fatalf("Failed to serialize large but valid payload (size %d): %v", safePayloadSize, err)
		}
		t.Logf("Nearly max size test: payload %d bytes, ciphertext %d bytes (limit %d)", safePayloadSize, len(sb), MaxSize)
		if len(sb) > MaxSize {
			t.Errorf("Ciphertext for nearly max size payload exceeded limit: len %d", len(sb))
		}
	})
}

func TestSecureBytes_InvalidKeyset(t *testing.T) {
	t.Run("Serialize with invalid keyset", func(t *testing.T) {
		invalidKs := &keyset.Keyset{} // Empty keyset
		if _, err := Serialize(invalidKs, "test"); err == nil {
			t.Fatalf("expected error when serializing with invalid keyset")
		}
	})

	t.Run("Parse with invalid keyset", func(t *testing.T) {
		kcs := mustKeys(t, 1)
		sb, _ := Serialize(kcs, "test")

		invalidKs := &keyset.Keyset{} // Empty keyset
		if _, err := Parse[string](invalidKs, sb); err == nil {
			t.Fatalf("expected error when parsing with invalid keyset")
		}
	})
}

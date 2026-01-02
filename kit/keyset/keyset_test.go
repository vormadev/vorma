package keyset

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/cryptoutil"
)

// Helper function to create a valid base64-encoded 32-byte secret
func generateTestSecret() string {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i)
	}
	return base64.StdEncoding.EncodeToString(secret)
}

// Helper function to create a test Key32
func generateTestKey32() cryptoutil.Key32 {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 100)
	}
	k32, _ := cryptoutil.ToKey32(key)
	return k32
}

func TestKeyset_ValidateMethod(t *testing.T) {
	tests := []struct {
		name    string
		keyset  *Keyset
		wantErr bool
	}{
		{
			name:    "empty keyset",
			keyset:  &Keyset{uks: UnwrappedKeyset{}},
			wantErr: true,
		},
		{
			name:    "single nil key",
			keyset:  &Keyset{uks: UnwrappedKeyset{nil}},
			wantErr: true,
		},
		{
			name:    "valid single key",
			keyset:  &Keyset{uks: UnwrappedKeyset{generateTestKey32()}},
			wantErr: false,
		},
		{
			name:    "valid multiple keys",
			keyset:  &Keyset{uks: UnwrappedKeyset{generateTestKey32(), generateTestKey32()}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.keyset.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestKeyset_FromUnwrapped(t *testing.T) {
	tests := []struct {
		name    string
		keys    UnwrappedKeyset
		wantErr bool
	}{
		{
			name:    "empty keys",
			keys:    UnwrappedKeyset{},
			wantErr: true,
		},
		{
			name:    "single nil key",
			keys:    UnwrappedKeyset{nil},
			wantErr: true,
		},
		{
			name:    "valid single key",
			keys:    UnwrappedKeyset{generateTestKey32()},
			wantErr: false,
		},
		{
			name:    "valid multiple keys",
			keys:    UnwrappedKeyset{generateTestKey32(), generateTestKey32()},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyset, err := FromUnwrapped(tt.keys)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromUnwrapped() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && keyset == nil {
				t.Error("expected non-nil Keyset")
			}
		})
	}
}

func TestKeyset_Unwrap(t *testing.T) {
	keys := UnwrappedKeyset{generateTestKey32(), generateTestKey32()}
	ks := &Keyset{uks: keys}

	unwrapped := ks.Unwrap()
	if len(unwrapped) != 2 {
		t.Errorf("expected 2 keys, got %d", len(unwrapped))
	}

	// Verify it returns the same reference
	if &unwrapped[0] != &keys[0] {
		t.Error("Unwrap should return the same reference")
	}
}

func TestKeyset_First(t *testing.T) {
	tests := []struct {
		name    string
		keyset  *Keyset
		wantErr bool
	}{
		{
			name:    "empty keyset",
			keyset:  &Keyset{uks: UnwrappedKeyset{}},
			wantErr: true,
		},
		{
			name:    "nil first key",
			keyset:  &Keyset{uks: UnwrappedKeyset{nil}},
			wantErr: true,
		},
		{
			name:    "valid first key",
			keyset:  &Keyset{uks: UnwrappedKeyset{generateTestKey32()}},
			wantErr: false,
		},
		{
			name:    "multiple keys",
			keyset:  &Keyset{uks: UnwrappedKeyset{generateTestKey32(), generateTestKey32()}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := tt.keyset.First()
			if (err != nil) != tt.wantErr {
				t.Errorf("First() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && key == nil {
				t.Error("expected non-nil key")
			}
		})
	}
}

func TestAttempt(t *testing.T) {
	successKey := generateTestKey32()
	failKey := generateTestKey32()

	tests := []struct {
		name      string
		keyset    *Keyset
		fn        func(cryptoutil.Key32) (string, error)
		wantValue string
		wantErr   bool
	}{
		{
			name:    "empty keyset",
			keyset:  &Keyset{uks: UnwrappedKeyset{}},
			fn:      func(k cryptoutil.Key32) (string, error) { return "ok", nil },
			wantErr: true,
		},
		{
			name:    "nil key in keyset",
			keyset:  &Keyset{uks: UnwrappedKeyset{nil}},
			fn:      func(k cryptoutil.Key32) (string, error) { return "ok", nil },
			wantErr: true,
		},
		{
			name:   "first key succeeds",
			keyset: &Keyset{uks: UnwrappedKeyset{successKey, failKey}},
			fn: func(k cryptoutil.Key32) (string, error) {
				if bytes.Equal(k[:], successKey[:]) {
					return "success", nil
				}
				return "", errors.New("wrong key")
			},
			wantValue: "success",
			wantErr:   false,
		},
		{
			name:   "fallback to second key",
			keyset: &Keyset{uks: UnwrappedKeyset{failKey, successKey}},
			fn: func(k cryptoutil.Key32) (string, error) {
				if bytes.Equal(k[:], successKey[:]) {
					return "success", nil
				}
				return "", errors.New("wrong key")
			},
			wantValue: "success",
			wantErr:   false,
		},
		{
			name:   "all keys fail",
			keyset: &Keyset{uks: UnwrappedKeyset{failKey, failKey}},
			fn: func(k cryptoutil.Key32) (string, error) {
				return "", errors.New("always fail")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Attempt(tt.keyset, tt.fn)
			if (err != nil) != tt.wantErr {
				t.Errorf("Attempt() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && result != tt.wantValue {
				t.Errorf("Attempt() result = %v, want %v", result, tt.wantValue)
			}
		})
	}
}

func TestKeyset_HKDF(t *testing.T) {
	tests := []struct {
		name    string
		keyset  *Keyset
		salt    []byte
		info    string
		wantErr bool
	}{
		{
			name:    "empty keyset",
			keyset:  &Keyset{uks: UnwrappedKeyset{}},
			salt:    []byte("salt"),
			info:    "info",
			wantErr: true,
		},
		{
			name:    "single key",
			keyset:  &Keyset{uks: UnwrappedKeyset{generateTestKey32()}},
			salt:    []byte("salt"),
			info:    "info",
			wantErr: false,
		},
		{
			name:    "multiple keys",
			keyset:  &Keyset{uks: UnwrappedKeyset{generateTestKey32(), generateTestKey32()}},
			salt:    []byte("salt"),
			info:    "info",
			wantErr: false,
		},
		{
			name:    "empty salt",
			keyset:  &Keyset{uks: UnwrappedKeyset{generateTestKey32()}},
			salt:    []byte{},
			info:    "info",
			wantErr: false,
		},
		{
			name:    "empty info",
			keyset:  &Keyset{uks: UnwrappedKeyset{generateTestKey32()}},
			salt:    []byte("salt"),
			info:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			derived, err := tt.keyset.HKDF(tt.salt, tt.info)
			if (err != nil) != tt.wantErr {
				t.Errorf("HKDF() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if derived == nil {
					t.Error("expected non-nil derived keyset")
				} else if len(derived.uks) != len(tt.keyset.uks) {
					t.Errorf("expected %d derived keys, got %d",
						len(tt.keyset.uks), len(derived.uks))
				}
			}
		})
	}
}

func TestRootSecretsToRootKeyset(t *testing.T) {
	validSecret := generateTestSecret()
	invalidBase64 := "not-valid-base64!"
	wrongSizeSecret := base64.StdEncoding.EncodeToString([]byte("too short"))

	tests := []struct {
		name    string
		secrets RootSecrets
		wantErr bool
	}{
		{
			name:    "empty secrets",
			secrets: RootSecrets{},
			wantErr: true,
		},
		{
			name:    "single valid secret",
			secrets: RootSecrets{RootSecret(validSecret)},
			wantErr: false,
		},
		{
			name:    "multiple valid secrets",
			secrets: RootSecrets{RootSecret(validSecret), RootSecret(generateTestSecret())},
			wantErr: false,
		},
		{
			name:    "invalid base64",
			secrets: RootSecrets{RootSecret(invalidBase64)},
			wantErr: true,
		},
		{
			name:    "wrong size secret",
			secrets: RootSecrets{RootSecret(wrongSizeSecret)},
			wantErr: true,
		},
		{
			name:    "mix of valid and invalid",
			secrets: RootSecrets{RootSecret(validSecret), RootSecret(invalidBase64)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyset, err := RootSecretsToRootKeyset(tt.secrets)
			if (err != nil) != tt.wantErr {
				t.Errorf("RootSecretsToRootKeyset() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && keyset == nil {
				t.Error("expected non-nil keyset")
			}
			if !tt.wantErr && len(keyset.uks) != len(tt.secrets) {
				t.Errorf("expected %d keys, got %d", len(tt.secrets), len(keyset.uks))
			}
		})
	}
}

func TestLoadRootSecrets(t *testing.T) {
	// Setup test environment variables
	os.Setenv("TEST_SECRET_1", generateTestSecret())
	os.Setenv("TEST_SECRET_2", generateTestSecret())
	defer os.Unsetenv("TEST_SECRET_1")
	defer os.Unsetenv("TEST_SECRET_2")

	tests := []struct {
		name    string
		envVars []string
		wantErr bool
		wantLen int
	}{
		{
			name:    "no env vars",
			envVars: []string{},
			wantErr: true,
		},
		{
			name:    "single valid env var",
			envVars: []string{"TEST_SECRET_1"},
			wantErr: false,
			wantLen: 1,
		},
		{
			name:    "multiple valid env vars",
			envVars: []string{"TEST_SECRET_1", "TEST_SECRET_2"},
			wantErr: false,
			wantLen: 2,
		},
		{
			name:    "empty env var name",
			envVars: []string{""},
			wantErr: true,
		},
		{
			name:    "non-existent env var",
			envVars: []string{"DOES_NOT_EXIST"},
			wantErr: true,
		},
		{
			name:    "mix of valid and invalid",
			envVars: []string{"TEST_SECRET_1", "DOES_NOT_EXIST"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secrets, err := LoadRootSecrets(tt.envVars...)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadRootSecrets() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(secrets) != tt.wantLen {
				t.Errorf("expected %d secrets, got %d", tt.wantLen, len(secrets))
			}
		})
	}
}

func TestLoadRootKeyset(t *testing.T) {
	// Setup test environment variables
	os.Setenv("TEST_KEYSET_1", generateTestSecret())
	os.Setenv("TEST_KEYSET_2", generateTestSecret())
	os.Setenv("TEST_INVALID", "invalid-base64!")
	defer os.Unsetenv("TEST_KEYSET_1")
	defer os.Unsetenv("TEST_KEYSET_2")
	defer os.Unsetenv("TEST_INVALID")

	tests := []struct {
		name    string
		envVars []string
		wantErr bool
	}{
		{
			name:    "valid single key",
			envVars: []string{"TEST_KEYSET_1"},
			wantErr: false,
		},
		{
			name:    "valid multiple keys",
			envVars: []string{"TEST_KEYSET_1", "TEST_KEYSET_2"},
			wantErr: false,
		},
		{
			name:    "invalid secret format",
			envVars: []string{"TEST_INVALID"},
			wantErr: true,
		},
		{
			name:    "non-existent env var",
			envVars: []string{"DOES_NOT_EXIST"},
			wantErr: true,
		},
		{
			name:    "no env vars",
			envVars: []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyset, err := LoadRootKeyset(tt.envVars...)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadRootKeyset() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && keyset == nil {
				t.Error("expected non-nil keyset")
			}
		})
	}
}

func TestMustAppKeyset(t *testing.T) {
	// Setup test environment variable
	os.Setenv("TEST_APP_SECRET", generateTestSecret())
	defer os.Unsetenv("TEST_APP_SECRET")

	// Test panic cases
	t.Run("panic on empty env vars", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for empty env vars")
			}
		}()
		MustAppKeyset(AppKeysetConfig{
			LatestFirstEnvVarNames: []string{},
			ApplicationName:        "test-app",
		})
	})

	t.Run("panic on empty application name", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for empty application name")
			}
		}()
		MustAppKeyset(AppKeysetConfig{
			LatestFirstEnvVarNames: []string{"TEST_APP_SECRET"},
			ApplicationName:        "",
		})
	})

	// Test successful creation
	t.Run("valid config", func(t *testing.T) {
		appKeyset := MustAppKeyset(AppKeysetConfig{
			LatestFirstEnvVarNames: []string{"TEST_APP_SECRET"},
			ApplicationName:        "test-app",
		})

		if appKeyset == nil {
			t.Fatal("expected non-nil AppKeyset")
		}

		// Test Root() function
		root := appKeyset.Root()
		if root == nil {
			t.Error("expected non-nil root keyset")
		}

		// Test HKDF() function
		hkdfFn := appKeyset.HKDF("test-purpose")
		if hkdfFn == nil {
			t.Error("expected non-nil HKDF function")
		}

		derived := hkdfFn()
		if derived == nil {
			t.Error("expected non-nil derived keyset")
		}

		// Verify same result when called again (lazy loading)
		derived2 := hkdfFn()
		if derived != derived2 {
			t.Error("expected same keyset instance from lazy loading")
		}
	})

	// Test panic on empty HKDF purpose
	t.Run("panic on empty HKDF purpose", func(t *testing.T) {
		appKeyset := MustAppKeyset(AppKeysetConfig{
			LatestFirstEnvVarNames: []string{"TEST_APP_SECRET"},
			ApplicationName:        "test-app",
		})

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for empty HKDF purpose")
			}
		}()

		hkdfFn := appKeyset.HKDF("")
		hkdfFn() // Should panic here
	})
}

func TestMustAppKeyset_DeferredValidation(t *testing.T) {
	os.Setenv("TEST_DEFERRED_SECRET", generateTestSecret())
	defer os.Unsetenv("TEST_DEFERRED_SECRET")

	t.Run("deferred validation does not panic on construction", func(t *testing.T) {
		// Should not panic during construction even with invalid config
		appKeyset := MustAppKeyset(AppKeysetConfig{
			LatestFirstEnvVarNames: []string{},
			ApplicationName:        "",
			DeferPanic:             true,
		})

		if appKeyset == nil {
			t.Error("expected non-nil AppKeyset even with deferred validation")
		}
	})

	t.Run("deferred validation panics on Root access with empty env vars", func(t *testing.T) {
		appKeyset := MustAppKeyset(AppKeysetConfig{
			LatestFirstEnvVarNames: []string{},
			ApplicationName:        "test-app",
			DeferPanic:             true,
		})

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when accessing Root with invalid config")
			} else {
				panicStr := fmt.Sprintf("%v", r)
				if !strings.Contains(panicStr, "at least 1 env var key is required") {
					t.Errorf("unexpected panic message: %s", panicStr)
				}
			}
		}()

		_ = appKeyset.Root()
	})

	t.Run("deferred validation panics on Root access with empty application name", func(t *testing.T) {
		appKeyset := MustAppKeyset(AppKeysetConfig{
			LatestFirstEnvVarNames: []string{"TEST_DEFERRED_SECRET"},
			ApplicationName:        "",
			DeferPanic:             true,
		})

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when accessing Root with invalid config")
			} else {
				panicStr := fmt.Sprintf("%v", r)
				if !strings.Contains(panicStr, "ApplicationName cannot be empty") {
					t.Errorf("unexpected panic message: %s", panicStr)
				}
			}
		}()

		_ = appKeyset.Root()
	})

	t.Run("deferred validation panics on HKDF access with empty env vars", func(t *testing.T) {
		appKeyset := MustAppKeyset(AppKeysetConfig{
			LatestFirstEnvVarNames: []string{},
			ApplicationName:        "test-app",
			DeferPanic:             true,
		})

		hkdfFn := appKeyset.HKDF("test-purpose")

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when accessing HKDF with invalid config")
			} else {
				panicStr := fmt.Sprintf("%v", r)
				if !strings.Contains(panicStr, "at least 1 env var key is required") {
					t.Errorf("unexpected panic message: %s", panicStr)
				}
			}
		}()

		_ = hkdfFn()
	})

	t.Run("deferred validation panics on HKDF access with empty application name", func(t *testing.T) {
		appKeyset := MustAppKeyset(AppKeysetConfig{
			LatestFirstEnvVarNames: []string{"TEST_DEFERRED_SECRET"},
			ApplicationName:        "",
			DeferPanic:             true,
		})

		hkdfFn := appKeyset.HKDF("test-purpose")

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when accessing HKDF with invalid config")
			} else {
				panicStr := fmt.Sprintf("%v", r)
				if !strings.Contains(panicStr, "ApplicationName cannot be empty") {
					t.Errorf("unexpected panic message: %s", panicStr)
				}
			}
		}()

		_ = hkdfFn()
	})

	t.Run("deferred validation with valid config works correctly", func(t *testing.T) {
		appKeyset := MustAppKeyset(AppKeysetConfig{
			LatestFirstEnvVarNames: []string{"TEST_DEFERRED_SECRET"},
			ApplicationName:        "test-app",
			DeferPanic:             true,
		})

		// Should not panic with valid config
		root := appKeyset.Root()
		if root == nil {
			t.Error("expected non-nil root keyset")
		}

		hkdfFn := appKeyset.HKDF("test-purpose")
		derived := hkdfFn()
		if derived == nil {
			t.Error("expected non-nil derived keyset")
		}
	})

	t.Run("default behavior unchanged when false passed explicitly", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected immediate panic with false argument")
			}
		}()

		MustAppKeyset(AppKeysetConfig{
			LatestFirstEnvVarNames: []string{},
			ApplicationName:        "test-app",
		})
	})

	t.Run("default behavior unchanged when no argument passed", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected immediate panic with no argument")
			}
		}()

		MustAppKeyset(AppKeysetConfig{
			LatestFirstEnvVarNames: []string{},
			ApplicationName:        "test-app",
		})
	})
}

func TestAppKeyset_MultipleHKDFPurposes(t *testing.T) {
	os.Setenv("TEST_MULTI_SECRET", generateTestSecret())
	defer os.Unsetenv("TEST_MULTI_SECRET")

	appKeyset := MustAppKeyset(AppKeysetConfig{
		LatestFirstEnvVarNames: []string{"TEST_MULTI_SECRET"},
		ApplicationName:        "test-app",
	})

	// Get different derived keysets for different purposes
	encryptionKeys := appKeyset.HKDF("encryption")()
	signingKeys := appKeyset.HKDF("signing")()

	if encryptionKeys == nil || signingKeys == nil {
		t.Fatal("expected non-nil derived keysets")
	}

	// Verify they're different keysets
	if encryptionKeys == signingKeys {
		t.Error("expected different keyset instances for different purposes")
	}

	// Verify the keys themselves are different
	encKey, _ := encryptionKeys.First()
	sigKey, _ := signingKeys.First()

	if bytes.Equal(encKey[:], sigKey[:]) {
		t.Error("expected different keys for different purposes")
	}
}

func TestKeyset_Validate_NilKeyset(t *testing.T) {
	var ks *Keyset
	err := ks.Validate()
	if err == nil {
		t.Error("expected error for nil keyset")
	}
	if err.Error() != "keyset is nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestKeyset_First_NilKeyset(t *testing.T) {
	var ks *Keyset
	_, err := ks.First()
	if err == nil {
		t.Error("expected error for nil keyset")
	}
	if err.Error() != "keyset is nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAppKeyset_RootPanicOnBadEnvVar(t *testing.T) {
	// Test that the lazy loader panics when env var is missing
	appKeyset := MustAppKeyset(AppKeysetConfig{
		LatestFirstEnvVarNames: []string{"DEFINITELY_DOES_NOT_EXIST"},
		ApplicationName:        "test-app",
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic when loading root keyset with missing env var")
		}
		// Verify it's the right kind of panic
		panicStr, ok := r.(string)
		if !ok {
			t.Errorf("expected string panic, got %T", r)
		}
		if !strings.Contains(panicStr, "error loading root keyset") {
			t.Errorf("unexpected panic message: %s", panicStr)
		}
	}()

	// This should panic
	_ = appKeyset.Root()
}

func TestAppKeyset_HKDFPanicOnBadEnvVar(t *testing.T) {
	// Test that HKDF panics when root keyset can't be loaded
	appKeyset := MustAppKeyset(AppKeysetConfig{
		LatestFirstEnvVarNames: []string{"DEFINITELY_DOES_NOT_EXIST"},
		ApplicationName:        "test-app",
	})

	hkdfFn := appKeyset.HKDF("test-purpose")

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic when deriving keyset with missing env var")
		}
		// Should panic when trying to load root keyset
		panicStr, ok := r.(string)
		if !ok {
			t.Errorf("expected string panic, got %T", r)
		}
		if !strings.Contains(panicStr, "error loading root keyset") {
			t.Errorf("unexpected panic message: %s", panicStr)
		}
	}()

	// This should panic
	_ = hkdfFn()
}

func TestLoadRootKeyset_ErrorPropagation(t *testing.T) {
	// Test that LoadRootKeyset properly propagates errors from LoadRootSecrets
	_, err := LoadRootKeyset() // No arguments
	if err == nil {
		t.Error("expected error when no env vars provided")
	}
	if !strings.Contains(err.Error(), "error loading root secrets") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Test that LoadRootKeyset properly propagates errors from RootSecretsToRootKeyset
	os.Setenv("TEST_BAD_SECRET", "not-valid-base64!")
	defer os.Unsetenv("TEST_BAD_SECRET")

	_, err = LoadRootKeyset("TEST_BAD_SECRET")
	if err == nil {
		t.Error("expected error when secret is invalid base64")
	}
	if !strings.Contains(err.Error(), "error converting root secrets to keyset") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAttempt_ComplexErrorJoining(t *testing.T) {
	// Test that Attempt properly joins all errors when all keys fail
	key1 := generateTestKey32()
	key2 := generateTestKey32()
	key3 := generateTestKey32()

	ks := &Keyset{uks: UnwrappedKeyset{key1, key2, key3}}

	callCount := 0
	_, err := Attempt(ks, func(k cryptoutil.Key32) (string, error) {
		callCount++
		return "", fmt.Errorf("error for key %d", callCount)
	})

	if err == nil {
		t.Error("expected error when all attempts fail")
	}

	// Verify all three errors are included
	errStr := err.Error()
	if !strings.Contains(errStr, "key 0: error for key 1") {
		t.Error("missing error for key 0")
	}
	if !strings.Contains(errStr, "key 1: error for key 2") {
		t.Error("missing error for key 1")
	}
	if !strings.Contains(errStr, "key 2: error for key 3") {
		t.Error("missing error for key 2")
	}

	// Verify all keys were attempted
	if callCount != 3 {
		t.Errorf("expected 3 attempts, got %d", callCount)
	}
}

func TestAppKeyset_LazyLoadingConsistency(t *testing.T) {
	os.Setenv("TEST_LAZY_SECRET", generateTestSecret())
	defer os.Unsetenv("TEST_LAZY_SECRET")

	appKeyset := MustAppKeyset(AppKeysetConfig{
		LatestFirstEnvVarNames: []string{"TEST_LAZY_SECRET"},
		ApplicationName:        "test-app",
	})

	// Get root multiple times - should be same instance
	root1 := appKeyset.Root()
	root2 := appKeyset.Root()

	if root1 != root2 {
		t.Error("expected same root keyset instance from lazy loading")
	}

	// Get same HKDF purpose multiple times - should create same function
	hkdf1 := appKeyset.HKDF("purpose1")
	hkdf2 := appKeyset.HKDF("purpose1")

	// The functions themselves will be different instances, but they should
	// return the same keyset when called
	keyset1 := hkdf1()
	keyset2 := hkdf2()

	// These might not be the same instance since we create new lazy loaders
	// each time HKDF is called, but the content should be equivalent
	if len(keyset1.uks) != len(keyset2.uks) {
		t.Error("expected same keyset content from HKDF with same purpose")
	}
}

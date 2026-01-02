package signedcookie

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/keyset"
)

const (
	aSecret = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	bSecret = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name        string
		secrets     keyset.RootSecrets
		expectError bool
	}{
		{
			name:        "Valid secrets",
			secrets:     keyset.RootSecrets{aSecret, bSecret},
			expectError: false,
		},
		{
			name:        "Empty secrets",
			secrets:     keyset.RootSecrets{},
			expectError: true,
		},
		{
			name:        "Invalid base64",
			secrets:     keyset.RootSecrets{"invalid-base64"},
			expectError: true,
		},
		{
			name:        "Wrong secret size",
			secrets:     keyset.RootSecrets{"AAAAAA=="},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.secrets)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if manager == nil {
					t.Errorf("Expected a manager, but got nil")
				}
			}
		})
	}
}

func TestManagerSignAndRead(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, err := NewManager(secrets)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	testValue := "test-value"
	signedValue, err := manager.signValue(testValue, false)
	if err != nil {
		t.Fatalf("Failed to sign value: %v", err)
	}

	readValue, err := manager.verifyAndReadValue(signedValue)
	if err != nil {
		t.Fatalf("Failed to read signed value: %v", err)
	}

	if readValue != testValue {
		t.Errorf("Expected %q, but got %q", testValue, readValue)
	}

	// Test with empty value
	emptyValue := ""
	signedEmptyValue, err := manager.signValue(emptyValue, false)
	if err != nil {
		t.Fatalf("Failed to sign empty value: %v", err)
	}
	readEmptyValue, err := manager.verifyAndReadValue(signedEmptyValue)
	if err != nil {
		t.Fatalf("Failed to read signed empty value: %v", err)
	}
	if readEmptyValue != emptyValue {
		t.Errorf("Expected empty string, but got %q", readEmptyValue)
	}

	// Test with very long value
	longValue := strings.Repeat("a", 10000)
	signedLongValue, err := manager.signValue(longValue, false)
	if err != nil {
		t.Fatalf("Failed to sign long value: %v", err)
	}
	readLongValue, err := manager.verifyAndReadValue(signedLongValue)
	if err != nil {
		t.Fatalf("Failed to read signed long value: %v", err)
	}
	if readLongValue != longValue {
		t.Errorf("Long value mismatch")
	}
}

func TestManagerGet(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, _ := NewManager(secrets)

	testValue := "test-value"
	signedValue, _ := manager.signValue(testValue, false)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.AddCookie(&http.Cookie{Name: "test-cookie", Value: signedValue})

	value, err := manager.VerifyAndReadCookieValue(req, "test-cookie")
	if err != nil {
		t.Fatalf("Failed to get cookie value: %v", err)
	}

	if value != testValue {
		t.Errorf("Expected %q, but got %q", testValue, value)
	}

	// Test with non-existent cookie
	_, err = manager.VerifyAndReadCookieValue(req, "non-existent-cookie")
	if err == nil {
		t.Errorf("Expected error for non-existent cookie, but got nil")
	}

	// Test with invalid signed value
	req.AddCookie(&http.Cookie{Name: "invalid-cookie", Value: "invalid-value"})
	_, err = manager.VerifyAndReadCookieValue(req, "invalid-cookie")
	if err == nil {
		t.Errorf("Expected error for invalid signed value, but got nil")
	}
}

func TestManagerNewDeletionCookie(t *testing.T) {
	manager, _ := NewManager(keyset.RootSecrets{aSecret})

	cookie := http.Cookie{
		Name:     "test-cookie",
		Value:    "test-value",
		MaxAge:   3600,
		Path:     "/test",
		Domain:   "example.com",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	deletionCookie := manager.NewDeletionCookie(cookie)

	if deletionCookie.Value != "" {
		t.Errorf("Expected empty value, but got %q", deletionCookie.Value)
	}
	if deletionCookie.MaxAge != -1 {
		t.Errorf("Expected MaxAge -1, but got %d", deletionCookie.MaxAge)
	}
	if deletionCookie.Path != "/test" {
		t.Errorf("Expected Path /test, but got %q", deletionCookie.Path)
	}
	if deletionCookie.Domain != "example.com" {
		t.Errorf("Expected Domain example.com, but got %q", deletionCookie.Domain)
	}
	if !deletionCookie.Secure {
		t.Errorf("Expected Secure to be true")
	}
	if !deletionCookie.HttpOnly {
		t.Errorf("Expected HttpOnly to be true")
	}
	if deletionCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("Expected SameSite Strict, but got %v", deletionCookie.SameSite)
	}
}

func TestSignedCookie(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, _ := NewManager(secrets)

	type TestStruct struct {
		Field1 string
		Field2 int
	}

	signedCookie := &SignedCookie[TestStruct]{
		Manager:    manager,
		TTL:        time.Hour,
		BaseCookie: http.Cookie{Name: "test-cookie"},
	}

	testValue := TestStruct{Field1: "test", Field2: 42}

	t.Run("SignCookie", func(t *testing.T) {
		cookie, err := signedCookie.NewSignedCookie(testValue, nil)
		if err != nil {
			t.Fatalf("Failed to sign cookie: %v", err)
		}

		if cookie.Name != "test-cookie" {
			t.Errorf("Expected cookie name 'test-cookie', but got %q", cookie.Name)
		}

		if !cookie.Expires.After(time.Now()) {
			t.Errorf("Expected future expiration time")
		}
	})

	t.Run("SignCookieWithOverride", func(t *testing.T) {
		overrideCookie := &http.Cookie{
			Name:     "override-cookie",
			Path:     "/override",
			Domain:   "override.com",
			Secure:   false,
			HttpOnly: false,
		}
		cookie, err := signedCookie.NewSignedCookie(testValue, overrideCookie)
		if err != nil {
			t.Fatalf("Failed to sign cookie with override: %v", err)
		}

		if cookie.Path != "/override" {
			t.Errorf("Expected Path /override, but got %q", cookie.Path)
		}
		if cookie.Domain != "override.com" {
			t.Errorf("Expected Domain override.com, but got %q", cookie.Domain)
		}
		if !cookie.Secure {
			t.Errorf("Expected Secure to be true (should not be overridden)")
		}
		if !cookie.HttpOnly {
			t.Errorf("Expected HttpOnly to be true (should not be overridden)")
		}
	})

	t.Run("NewDeletionCookie", func(t *testing.T) {
		deletionCookie := signedCookie.NewDeletionCookie()

		if deletionCookie.Name != "test-cookie" {
			t.Errorf("Expected cookie name 'test-cookie', but got %q", deletionCookie.Name)
		}

		if deletionCookie.MaxAge != -1 {
			t.Errorf("Expected MaxAge -1, but got %d", deletionCookie.MaxAge)
		}
	})

	t.Run("Get", func(t *testing.T) {
		cookie, _ := signedCookie.NewSignedCookie(testValue, nil)

		req := httptest.NewRequest("GET", "http://example.com", nil)
		req.AddCookie(cookie)

		getValue, err := signedCookie.VerifyAndReadCookieValue(req)
		if err != nil {
			t.Fatalf("Failed to get signed cookie value: %v", err)
		}

		if !reflect.DeepEqual(getValue, testValue) {
			t.Errorf("Expected %+v, but got %+v", testValue, getValue)
		}
	})

	t.Run("GetWithInvalidValue", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com", nil)
		req.AddCookie(&http.Cookie{Name: "test-cookie", Value: "invalid-value"})

		_, err := signedCookie.VerifyAndReadCookieValue(req)
		if err == nil {
			t.Errorf("Expected error for invalid cookie value, but got nil")
		}
	})
}

func TestNewSecureCookieWithoutValue(t *testing.T) {
	name := "test-cookie"
	expires := time.Now().Add(time.Hour)
	baseCookie := &http.Cookie{
		Path:     "/test",
		Domain:   "example.com",
		SameSite: http.SameSiteStrictMode,
	}

	cookie := newSecureCookieWithoutValue(name, &expires, baseCookie)

	if cookie.Name != name {
		t.Errorf("Expected name %q, but got %q", name, cookie.Name)
	}
	if !cookie.Expires.Equal(expires) {
		t.Errorf("Expected expires %v, but got %v", expires, cookie.Expires)
	}
	if !cookie.HttpOnly {
		t.Errorf("Expected HttpOnly to be true")
	}
	if !cookie.Secure {
		t.Errorf("Expected Secure to be true")
	}
	if cookie.Path != "/test" {
		t.Errorf("Expected Path /test, but got %q", cookie.Path)
	}
	if cookie.Domain != "example.com" {
		t.Errorf("Expected Domain example.com, but got %q", cookie.Domain)
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("Expected SameSite Strict, but got %v", cookie.SameSite)
	}

	t.Run("NilExpires", func(t *testing.T) {
		cookie := newSecureCookieWithoutValue("test-cookie", nil, nil)
		if !cookie.Expires.IsZero() {
			t.Errorf("Expected zero expiration time, but got %v", cookie.Expires)
		}
	})

	t.Run("NilBaseCookie", func(t *testing.T) {
		cookie := newSecureCookieWithoutValue("test-cookie", nil, nil)
		if cookie.Path != "" || cookie.Domain != "" || cookie.SameSite != 0 {
			t.Errorf("Expected default values for Path, Domain, and SameSite")
		}
	})
}

func TestManagerSignCookie(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, _ := NewManager(secrets)

	cookie := &http.Cookie{
		Name:  "test-cookie",
		Value: "test-value",
	}

	err := manager.SignCookie(cookie, false)
	if err != nil {
		t.Fatalf("Failed to sign cookie: %v", err)
	}

	if cookie.Name != "test-cookie" {
		t.Errorf("Expected cookie name %q, but got %q", "test-cookie", cookie.Name)
	}

	if cookie.Value == "test-value" {
		t.Errorf("Expected signed value to be different from unsigned value")
	}

	// Verify that the signed value can be read back
	readValue, err := manager.verifyAndReadValue(cookie.Value)
	if err != nil {
		t.Fatalf("Failed to read signed cookie value: %v", err)
	}

	if readValue != "test-value" {
		t.Errorf("Expected read value %q, but got %q", "test-value", readValue)
	}

	t.Run("SignEmptyCookie", func(t *testing.T) {
		cookie := &http.Cookie{Name: "empty-cookie", Value: ""}
		err := manager.SignCookie(cookie, false)
		if err != nil {
			t.Fatalf("Failed to sign empty cookie: %v", err)
		}
		if cookie.Value == "" {
			t.Errorf("Expected non-empty signed value for empty cookie")
		}
	})
}

func TestManagerReadInvalidSignature(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, _ := NewManager(secrets)

	invalidSignedValue := bytesutil.ToBase64([]byte("invalid-signature"))

	_, err := manager.verifyAndReadValue(invalidSignedValue)
	if err == nil {
		t.Errorf("Expected an error for invalid signature, but got nil")
	}

	t.Run("ReadEmptySignature", func(t *testing.T) {
		_, err := manager.verifyAndReadValue("")
		if err == nil {
			t.Errorf("Expected an error for empty signature, but got nil")
		}
	})
}

func TestManagerMultipleSecrets(t *testing.T) {
	secrets := keyset.RootSecrets{
		aSecret,
		bSecret,
	}
	manager, _ := NewManager(secrets)

	testValue := "test-value"

	// Sign with the first (latest) secret
	signedValue, _ := manager.signValue(testValue, false)

	// Read should work
	readValue, err := manager.verifyAndReadValue(signedValue)
	if err != nil {
		t.Fatalf("Failed to read signed value: %v", err)
	}
	if readValue != testValue {
		t.Errorf("Expected %q, but got %q", testValue, readValue)
	}

	// Create a new manager with rotated secrets
	rotatedSecrets := keyset.RootSecrets{
		"CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC=",
		aSecret,
	}
	rotatedManager, _ := NewManager(rotatedSecrets)

	// Read should still work with the rotated manager
	readValue, err = rotatedManager.verifyAndReadValue(signedValue)
	if err != nil {
		t.Fatalf("Failed to read signed value with rotated secrets: %v", err)
	}
	if readValue != testValue {
		t.Errorf("Expected %q, but got %q", testValue, readValue)
	}

	t.Run("SignWithNewSecret", func(t *testing.T) {
		newValue := "new-test-value"
		signedNewValue, _ := rotatedManager.signValue(newValue, false)

		// Should be able to read with the rotated manager
		readNewValue, err := rotatedManager.verifyAndReadValue(signedNewValue)
		if err != nil {
			t.Fatalf("Failed to read new signed value: %v", err)
		}
		if readNewValue != newValue {
			t.Errorf("Expected %q, but got %q", newValue, readNewValue)
		}

		// Should not be able to read with the old manager
		_, err = manager.verifyAndReadValue(signedNewValue)
		if err == nil {
			t.Errorf("Expected error when reading new signature with old manager, but got nil")
		}
	})
}

func TestSignedCookieEdgeCases(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, _ := NewManager(secrets)

	type LargeStruct struct {
		LargeField string
	}

	signedCookie := &SignedCookie[LargeStruct]{
		Manager:    manager,
		TTL:        time.Hour,
		BaseCookie: http.Cookie{Name: "large-cookie"},
	}

	t.Run("LargeValue", func(t *testing.T) {
		largeValue := LargeStruct{LargeField: strings.Repeat("a", 4096)} // 4KB of data
		cookie, err := signedCookie.NewSignedCookie(largeValue, nil)
		if err != nil {
			t.Fatalf("Failed to sign large cookie: %v", err)
		}

		req := httptest.NewRequest("GET", "http://example.com", nil)
		req.AddCookie(cookie)

		getValue, err := signedCookie.VerifyAndReadCookieValue(req)
		if err != nil {
			t.Fatalf("Failed to get large signed cookie value: %v", err)
		}

		if !reflect.DeepEqual(getValue, largeValue) {
			t.Errorf("Large value mismatch")
		}
	})

	t.Run("ZeroTTL", func(t *testing.T) {
		zeroTTLCookie := &SignedCookie[LargeStruct]{
			Manager:    manager,
			TTL:        0,
			BaseCookie: http.Cookie{Name: "zero-ttl-cookie"},
		}

		value := LargeStruct{LargeField: "test"}
		cookie, err := zeroTTLCookie.NewSignedCookie(value, nil)
		if err != nil {
			t.Fatalf("Failed to sign zero TTL cookie: %v", err)
		}

		if !cookie.Expires.IsZero() {
			t.Errorf("Expected zero expiration time for zero TTL, but got %v", cookie.Expires)
		}
	})
}

func TestManagerConcurrency(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, _ := NewManager(secrets)

	concurrency := 100
	done := make(chan bool)

	for i := 0; i < concurrency; i++ {
		go func() {
			value := "test-value"
			signedValue, err := manager.signValue(value, false)
			if err != nil {
				t.Errorf("Failed to sign value: %v", err)
			}

			readValue, err := manager.verifyAndReadValue(signedValue)
			if err != nil {
				t.Errorf("Failed to read signed value: %v", err)
			}

			if readValue != value {
				t.Errorf("Value mismatch: expected %q, got %q", value, readValue)
			}

			done <- true
		}()
	}

	for i := 0; i < concurrency; i++ {
		<-done
	}
}

func TestManagerErrorCases(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, _ := NewManager(secrets)

	t.Run("SignValueWithLongInput", func(t *testing.T) {
		longValue := strings.Repeat("a", 1<<20) // 1MB of data
		signedValue, err := manager.signValue(longValue, false)
		if err != nil {
			t.Fatalf("Unexpected error when signing very long value: %v", err)
		}

		// Verify that we can read back the long value
		readValue, err := manager.verifyAndReadValue(signedValue)
		if err != nil {
			t.Fatalf("Failed to read back long signed value: %v", err)
		}

		if readValue != longValue {
			t.Errorf("Long value mismatch: lengths differ. Expected %d, got %d", len(longValue), len(readValue))
		}
	})

	t.Run("ReadInvalidBase64", func(t *testing.T) {
		_, err := manager.verifyAndReadValue("not-base64")
		if err == nil {
			t.Errorf("Expected error when reading invalid base64, but got nil")
		}
	})

	t.Run("ReadTruncatedSignature", func(t *testing.T) {
		validSignature, _ := manager.signValue("test", false)
		truncatedSignature := validSignature[:len(validSignature)-10]
		_, err := manager.verifyAndReadValue(truncatedSignature)
		if err == nil {
			t.Errorf("Expected error when reading truncated signature, but got nil")
		}
	})
}

func TestManagerWithInvalidSecrets(t *testing.T) {
	t.Run("TooShortSecret", func(t *testing.T) {
		_, err := NewManager(keyset.RootSecrets{"too-short-secret"})
		if err == nil {
			t.Errorf("Expected error when creating manager with too short secret, but got nil")
		}
	})

	t.Run("InvalidBase64Secret", func(t *testing.T) {
		_, err := NewManager(keyset.RootSecrets{"this-is-not-valid-base64!@#$%^&*()"})
		if err == nil {
			t.Errorf("Expected error when creating manager with invalid base64 secret, but got nil")
		}
	})
}

func TestSignedCookieWithComplexTypes(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, _ := NewManager(secrets)

	type ComplexStruct struct {
		IntField    int
		StringField string
		FloatField  float64
		BoolField   bool
		SliceField  []int
		MapField    map[string]any
	}

	signedCookie := &SignedCookie[ComplexStruct]{
		Manager:    manager,
		TTL:        time.Hour,
		BaseCookie: http.Cookie{Name: "complex-cookie"},
	}

	complexValue := ComplexStruct{
		IntField:    42,
		StringField: "test",
		FloatField:  3.14,
		BoolField:   true,
		SliceField:  []int{1, 2, 3},
		MapField: map[string]any{
			"key1": "value1",
			"key2": 2,
		},
	}

	cookie, err := signedCookie.NewSignedCookie(complexValue, nil)
	if err != nil {
		t.Fatalf("Failed to sign complex cookie: %v", err)
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.AddCookie(cookie)

	getValue, err := signedCookie.VerifyAndReadCookieValue(req)
	if err != nil {
		t.Fatalf("Failed to get complex signed cookie value: %v", err)
	}

	if !reflect.DeepEqual(getValue, complexValue) {
		t.Errorf("Complex value mismatch: expected %+v, got %+v", complexValue, getValue)
	}
}

func TestManagerSignAndReadWithEncryption(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, err := NewManager(secrets)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	testValue := "test-value"
	signedValue, err := manager.signValue(testValue, true)
	if err != nil {
		t.Fatalf("Failed to sign and encrypt value: %v", err)
	}

	readValue, err := manager.verifyAndReadValue(signedValue)
	if err != nil {
		t.Fatalf("Failed to read signed and encrypted value: %v", err)
	}

	if readValue != testValue {
		t.Errorf("Expected %q, but got %q", testValue, readValue)
	}

	// Test with empty value
	emptyValue := ""
	signedEmptyValue, err := manager.signValue(emptyValue, true)
	if err != nil {
		t.Fatalf("Failed to sign and encrypt empty value: %v", err)
	}
	readEmptyValue, err := manager.verifyAndReadValue(signedEmptyValue)
	if err != nil {
		t.Fatalf("Failed to read signed and encrypted empty value: %v", err)
	}
	if readEmptyValue != emptyValue {
		t.Errorf("Expected empty string, but got %q", readEmptyValue)
	}

	// Test with very long value
	longValue := strings.Repeat("a", 10000)
	signedLongValue, err := manager.signValue(longValue, true)
	if err != nil {
		t.Fatalf("Failed to sign and encrypt long value: %v", err)
	}
	readLongValue, err := manager.verifyAndReadValue(signedLongValue)
	if err != nil {
		t.Fatalf("Failed to read signed and encrypted long value: %v", err)
	}
	if readLongValue != longValue {
		t.Errorf("Long value mismatch")
	}
}

func TestSignedCookieWithEncryption(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, _ := NewManager(secrets)

	type TestStruct struct {
		Field1 string
		Field2 int
	}

	signedCookie := &SignedCookie[TestStruct]{
		Manager:    manager,
		TTL:        time.Hour,
		BaseCookie: http.Cookie{Name: "test-cookie"},
		Encrypt:    true,
	}

	testValue := TestStruct{Field1: "test", Field2: 42}

	t.Run("SignEncryptedCookie", func(t *testing.T) {
		cookie, err := signedCookie.NewSignedCookie(testValue, nil)
		if err != nil {
			t.Fatalf("Failed to sign and encrypt cookie: %v", err)
		}

		if cookie.Name != "test-cookie" {
			t.Errorf("Expected cookie name 'test-cookie', but got %q", cookie.Name)
		}

		if !cookie.Expires.After(time.Now()) {
			t.Errorf("Expected future expiration time")
		}

		// Verify that the value is different from a non-encrypted cookie
		nonEncryptedCookie := &SignedCookie[TestStruct]{
			Manager:    manager,
			TTL:        time.Hour,
			BaseCookie: http.Cookie{Name: "test-cookie"},
			Encrypt:    false,
		}
		nonEncryptedCookieValue, _ := nonEncryptedCookie.NewSignedCookie(testValue, nil)

		if cookie.Value == nonEncryptedCookieValue.Value {
			t.Errorf("Expected encrypted cookie value to be different from non-encrypted")
		}
	})

	t.Run("GetEncryptedCookie", func(t *testing.T) {
		cookie, _ := signedCookie.NewSignedCookie(testValue, nil)

		req := httptest.NewRequest("GET", "http://example.com", nil)
		req.AddCookie(cookie)

		getValue, err := signedCookie.VerifyAndReadCookieValue(req)
		if err != nil {
			t.Fatalf("Failed to get signed and encrypted cookie value: %v", err)
		}

		if !reflect.DeepEqual(getValue, testValue) {
			t.Errorf("Expected %+v, but got %+v", testValue, getValue)
		}
	})
}

func TestManagerSignCookieWithEncryption(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, _ := NewManager(secrets)

	cookie := &http.Cookie{
		Name:  "test-cookie",
		Value: "test-value",
	}

	err := manager.SignCookie(cookie, true)
	if err != nil {
		t.Fatalf("Failed to sign and encrypt cookie: %v", err)
	}

	if cookie.Name != "test-cookie" {
		t.Errorf("Expected cookie name %q, but got %q", "test-cookie", cookie.Name)
	}

	if cookie.Value == "test-value" {
		t.Errorf("Expected signed and encrypted value to be different from unsigned value")
	}

	// Verify that the signed and encrypted value can be read back
	readValue, err := manager.verifyAndReadValue(cookie.Value)
	if err != nil {
		t.Fatalf("Failed to read signed and encrypted cookie value: %v", err)
	}

	if readValue != "test-value" {
		t.Errorf("Expected read value %q, but got %q", "test-value", readValue)
	}
}

func TestManagerMultipleSecretsWithEncryption(t *testing.T) {
	secrets := keyset.RootSecrets{
		aSecret,
		bSecret,
	}
	manager, _ := NewManager(secrets)

	testValue := "test-value"

	// Sign and encrypt with the first (latest) secret
	signedValue, _ := manager.signValue(testValue, true)

	// Read should work
	readValue, err := manager.verifyAndReadValue(signedValue)
	if err != nil {
		t.Fatalf("Failed to read signed and encrypted value: %v", err)
	}
	if readValue != testValue {
		t.Errorf("Expected %q, but got %q", testValue, readValue)
	}

	// Create a new manager with rotated secrets
	rotatedSecrets := keyset.RootSecrets{
		"CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC=",
		aSecret,
	}
	rotatedManager, _ := NewManager(rotatedSecrets)

	// Read should still work with the rotated manager
	readValue, err = rotatedManager.verifyAndReadValue(signedValue)
	if err != nil {
		t.Fatalf("Failed to read signed and encrypted value with rotated secrets: %v", err)
	}
	if readValue != testValue {
		t.Errorf("Expected %q, but got %q", testValue, readValue)
	}

	t.Run("SignWithNewSecretEncrypted", func(t *testing.T) {
		newValue := "new-test-value"
		signedNewValue, _ := rotatedManager.signValue(newValue, true)

		// Should be able to read with the rotated manager
		readNewValue, err := rotatedManager.verifyAndReadValue(signedNewValue)
		if err != nil {
			t.Fatalf("Failed to read new signed and encrypted value: %v", err)
		}
		if readNewValue != newValue {
			t.Errorf("Expected %q, but got %q", newValue, readNewValue)
		}

		// Should not be able to read with the old manager
		_, err = manager.verifyAndReadValue(signedNewValue)
		if err == nil {
			t.Errorf("Expected error when reading new signature with old manager, but got nil")
		}
	})
}

func TestSignedCookieEncryptionComparison(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, _ := NewManager(secrets)

	type TestStruct struct {
		Field1 string
		Field2 int
	}

	testValue := TestStruct{Field1: "test", Field2: 42}

	encryptedCookie := &SignedCookie[TestStruct]{
		Manager:    manager,
		TTL:        time.Hour,
		BaseCookie: http.Cookie{Name: "encrypted-cookie"},
		Encrypt:    true,
	}

	unencryptedCookie := &SignedCookie[TestStruct]{
		Manager:    manager,
		TTL:        time.Hour,
		BaseCookie: http.Cookie{Name: "unencrypted-cookie"},
		Encrypt:    false,
	}

	encryptedValue, err := encryptedCookie.NewSignedCookie(testValue, nil)
	if err != nil {
		t.Fatalf("Failed to create encrypted cookie: %v", err)
	}

	unencryptedValue, err := unencryptedCookie.NewSignedCookie(testValue, nil)
	if err != nil {
		t.Fatalf("Failed to create unencrypted cookie: %v", err)
	}

	if encryptedValue.Value == unencryptedValue.Value {
		t.Errorf("Expected encrypted and unencrypted values to be different")
	}

	// Both should be readable
	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.AddCookie(encryptedValue)
	req.AddCookie(unencryptedValue)

	encryptedRead, err := encryptedCookie.VerifyAndReadCookieValue(req)
	if err != nil {
		t.Fatalf("Failed to read encrypted cookie: %v", err)
	}

	unencryptedRead, err := unencryptedCookie.VerifyAndReadCookieValue(req)
	if err != nil {
		t.Fatalf("Failed to read unencrypted cookie: %v", err)
	}

	if !reflect.DeepEqual(encryptedRead, unencryptedRead) {
		t.Errorf("Encrypted and unencrypted reads should match: %+v != %+v", encryptedRead, unencryptedRead)
	}
}

func TestManagerErrorCasesWithEncryption(t *testing.T) {
	secrets := keyset.RootSecrets{aSecret}
	manager, _ := NewManager(secrets)

	t.Run("SignValueWithLongInputEncrypted", func(t *testing.T) {
		longValue := strings.Repeat("a", 1<<20) // 1MB of data
		signedValue, err := manager.signValue(longValue, true)
		if err != nil {
			t.Fatalf("Unexpected error when signing and encrypting very long value: %v", err)
		}

		// Verify that we can read back the long value
		readValue, err := manager.verifyAndReadValue(signedValue)
		if err != nil {
			t.Fatalf("Failed to read back long signed and encrypted value: %v", err)
		}

		if readValue != longValue {
			t.Errorf("Long value mismatch: lengths differ. Expected %d, got %d", len(longValue), len(readValue))
		}
	})

	t.Run("ReadInvalidEncryptedValue", func(t *testing.T) {
		invalidValue := bytesutil.ToBase64([]byte{1}) // Invalid prefix
		_, err := manager.verifyAndReadValue(invalidValue)
		if err == nil {
			t.Errorf("Expected error when reading invalid encrypted value, but got nil")
		}
	})
}

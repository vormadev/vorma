package cookies

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/kit/keyset"
	"github.com/vormadev/vorma/kit/response"
)

// Test data structures
type testSessionData struct {
	UserID    string
	Username  string
	ExpiresAt time.Time
}

// Helper to create a test keyset with actual keys
func createTestKeyset() *keyset.Keyset {
	// Generate base64-encoded 32-byte secrets
	secret1 := base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))
	secret2 := base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnopqrstuvwxyz123456"))

	rootSecrets := keyset.RootSecrets{secret1, secret2}
	ks, err := keyset.RootSecretsToRootKeyset(rootSecrets)
	if err != nil {
		panic(err)
	}
	return ks
}

// Test helpers
func newTestManager(isDev bool) *Manager {
	return NewManager(ManagerConfig{
		GetKeyset: createTestKeyset,
		GetIsDev:  func() bool { return isDev },
	})
}

func newTestManagerWithDefaults(isDev bool, site SameSite, part PartitionOption, httpOnly HttpOnlyOption) *Manager {
	return NewManager(ManagerConfig{
		GetKeyset:        createTestKeyset,
		GetIsDev:         func() bool { return isDev },
		DefaultSameSite:  site,
		DefaultPartition: part,
		DefaultHttpOnly:  httpOnly,
	})
}

// Test Manager creation
func TestNewManager(t *testing.T) {
	t.Run("panics with nil GetKeyset", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic with nil GetKeyset")
			}
		}()
		NewManager(ManagerConfig{
			GetKeyset: nil,
		})
	})

	t.Run("applies correct system defaults for zero-value config", func(t *testing.T) {
		mgr := NewManager(ManagerConfig{
			GetKeyset: createTestKeyset,
		})
		if mgr.cfg.DefaultSameSite != SameSiteLaxMode {
			t.Errorf("expected default SameSite to be Lax, got %v", mgr.cfg.DefaultSameSite)
		}
		if mgr.cfg.DefaultPartition != PartitionTrue {
			t.Errorf("expected default Partition to be True, got %v", mgr.cfg.DefaultPartition)
		}
		if mgr.cfg.DefaultHttpOnly != HttpOnlyTrue {
			t.Errorf("expected default HttpOnly to be True, got %v", mgr.cfg.DefaultHttpOnly)
		}
	})

	t.Run("preserves custom manager defaults", func(t *testing.T) {
		mgr := newTestManagerWithDefaults(false, SameSiteStrictMode, PartitionFalse, HttpOnlyFalse)

		if mgr.cfg.DefaultSameSite != SameSiteStrictMode {
			t.Errorf("expected SameSite to be Strict, got %v", mgr.cfg.DefaultSameSite)
		}
		if mgr.cfg.DefaultPartition != PartitionFalse {
			t.Errorf("expected Partition to be False, got %v", mgr.cfg.DefaultPartition)
		}
		if mgr.cfg.DefaultHttpOnly != HttpOnlyFalse {
			t.Errorf("expected HttpOnly to be False, got %v", mgr.cfg.DefaultHttpOnly)
		}
	})
}

// Test GetIsDev method
func TestGetIsDev(t *testing.T) {
	t.Run("returns false when GetIsDev is nil", func(t *testing.T) {
		mgr := NewManager(ManagerConfig{
			GetKeyset: createTestKeyset,
		})
		if mgr.GetIsDev() {
			t.Errorf("expected GetIsDev to return false when GetIsDev is nil")
		}
	})

	t.Run("returns GetIsDev result", func(t *testing.T) {
		mgr := newTestManager(true)
		if !mgr.GetIsDev() {
			t.Errorf("expected GetIsDev to return true")
		}

		mgr = newTestManager(false)
		if mgr.GetIsDev() {
			t.Errorf("expected GetIsDev to return false")
		}
	})
}

// Test cookie name prefixes
func TestHostPrefixName(t *testing.T) {
	tests := []struct {
		name     string
		isDev    bool
		input    string
		expected string
	}{
		{"production mode", false, "session", "__Host-session"},
		{"development mode", true, "session", "__Dev-session"},
		{"empty name production", false, "", "__Host-"},
		{"empty name development", true, "", "__Dev-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestManager(tt.isDev)
			result := mgr.hostPrefixName(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestResolvers(t *testing.T) {
	// Manager with non-system defaults: Strict, Not Partitioned, Not HttpOnly
	mgr := newTestManagerWithDefaults(false, SameSiteStrictMode, PartitionFalse, HttpOnlyFalse)
	// Manager with system defaults
	defaultMgr := newTestManager(false)

	t.Run("SameSite Resolution", func(t *testing.T) {
		if mgr.resolveSameSite(sameSiteDefault) != http.SameSiteStrictMode {
			t.Error("should use manager's custom strict default")
		}
		if mgr.resolveSameSite(SameSiteLaxMode) != http.SameSiteLaxMode {
			t.Error("should use configured lax value over manager default")
		}
		if defaultMgr.resolveSameSite(sameSiteDefault) != http.SameSiteLaxMode {
			t.Error("should use system default lax")
		}
	})

	t.Run("Partition Resolution", func(t *testing.T) {
		if mgr.resolvePartition(partitionDefault) != false {
			t.Error("should use manager's custom false default")
		}
		if mgr.resolvePartition(PartitionTrue) != true {
			t.Error("should use configured true value over manager default")
		}
		if defaultMgr.resolvePartition(partitionDefault) != true {
			t.Error("should use system default true")
		}
	})

	t.Run("HttpOnly Resolution", func(t *testing.T) {
		if mgr.resolveHttpOnly(httpOnlyDefault) != false {
			t.Error("should use manager's custom false default")
		}
		if mgr.resolveHttpOnly(HttpOnlyTrue) != true {
			t.Error("should use configured true value over manager default")
		}
		if defaultMgr.resolveHttpOnly(httpOnlyDefault) != true {
			t.Error("should use system default true")
		}
	})
}

// Test path resolution
func TestResolvePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "/"},
		{"/", "/"},
		{"/api", "/api"},
		{"/api/v1", "/api/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := resolvePath(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Test buildCookie
func TestBuildCookie(t *testing.T) {
	t.Run("production mode with host prefix", func(t *testing.T) {
		mgr := newTestManager(false)
		spec := cookieSpec{
			name:          "session",
			value:         "test-value",
			path:          "/custom",
			domain:        "example.com",
			ttl:           time.Hour,
			sameSite:      http.SameSiteStrictMode,
			httpOnly:      true,
			useHostPrefix: true,
			partitioned:   true,
		}

		cookie := mgr.buildCookie(spec)

		// Verify __Host- requirements are enforced
		if cookie.Name != "__Host-session" {
			t.Errorf("expected name to be __Host-session, got %s", cookie.Name)
		}
		if cookie.Path != "/" {
			t.Errorf("expected path to be /, got %s", cookie.Path)
		}
		if cookie.Domain != "" {
			t.Errorf("expected domain to be empty, got %s", cookie.Domain)
		}
		if !cookie.Secure {
			t.Errorf("expected Secure to be true")
		}
		if !cookie.HttpOnly {
			t.Errorf("expected HttpOnly to be true")
		}
		if !cookie.Partitioned {
			t.Errorf("expected Partitioned to be true")
		}
		if cookie.SameSite != http.SameSiteStrictMode {
			t.Errorf("expected SameSite to be Strict")
		}
		if cookie.MaxAge != 3600 {
			t.Errorf("expected MaxAge to be 3600, got %d", cookie.MaxAge)
		}
	})

	t.Run("development mode with host prefix", func(t *testing.T) {
		mgr := newTestManager(true)
		spec := cookieSpec{
			name:          "session",
			value:         "test-value",
			path:          "/custom",
			domain:        "example.com",
			ttl:           time.Hour,
			sameSite:      http.SameSiteStrictMode,
			httpOnly:      true,
			useHostPrefix: true,
			partitioned:   true,
		}

		cookie := mgr.buildCookie(spec)

		// In dev mode, __Host- requirements are NOT enforced
		if cookie.Name != "__Dev-session" {
			t.Errorf("expected name to be __Dev-session, got %s", cookie.Name)
		}
		if cookie.Path != "/custom" {
			t.Errorf("expected path to be /custom, got %s", cookie.Path)
		}
		if cookie.Domain != "example.com" {
			t.Errorf("expected domain to be example.com, got %s", cookie.Domain)
		}
		if cookie.Secure {
			t.Errorf("expected Secure to be false in dev mode")
		}
		if cookie.Partitioned {
			t.Errorf("expected Partitioned to be false in dev mode")
		}
	})

	t.Run("non-host cookie production", func(t *testing.T) {
		mgr := newTestManager(false)
		spec := cookieSpec{
			name:          "tracker",
			value:         "test-value",
			path:          "/api",
			domain:        ".example.com",
			ttl:           24 * time.Hour,
			sameSite:      http.SameSiteLaxMode,
			httpOnly:      false,
			useHostPrefix: false,
			partitioned:   false,
		}

		cookie := mgr.buildCookie(spec)

		if cookie.Name != "tracker" {
			t.Errorf("expected name to be tracker, got %s", cookie.Name)
		}
		if cookie.Path != "/api" {
			t.Errorf("expected path to be /api, got %s", cookie.Path)
		}
		if cookie.Domain != ".example.com" {
			t.Errorf("expected domain to be .example.com, got %s", cookie.Domain)
		}
		if !cookie.Secure {
			t.Errorf("expected Secure to be true in production")
		}
		if cookie.HttpOnly {
			t.Errorf("expected HttpOnly to be false")
		}
		if cookie.Partitioned {
			t.Errorf("expected Partitioned to be false when partitioned is false")
		}
	})
}

// Test SecureCookie with actual encryption/decryption
func TestSecureCookie(t *testing.T) {
	mgr := newTestManager(false)

	t.Run("creates and retrieves encrypted string value with system defaults", func(t *testing.T) {
		cookie := NewSecureCookie[string](SecureCookieConfig{
			Manager: mgr,
			Name:    "secure",
			TTL:     time.Hour,
		})

		// Create encrypted cookie
		httpCookie, err := cookie.New("secret-data")
		if err != nil {
			t.Fatalf("unexpected error creating cookie: %v", err)
		}

		if httpCookie.Name != "__Host-secure" {
			t.Errorf("expected name __Host-secure, got %s", httpCookie.Name)
		}
		if httpCookie.Value == "secret-data" {
			t.Errorf("expected encrypted value, got plaintext")
		}
		if !httpCookie.HttpOnly {
			t.Errorf("expected HttpOnly to be true by default")
		}
		if !httpCookie.Partitioned {
			t.Errorf("expected Partitioned to be true by default")
		}

		// Test retrieval
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(httpCookie)

		retrieved, err := cookie.Get(req)
		if err != nil {
			t.Fatalf("unexpected error retrieving cookie: %v", err)
		}
		if retrieved != "secret-data" {
			t.Errorf("expected 'secret-data', got %s", retrieved)
		}
	})

	t.Run("creates and retrieves encrypted struct value", func(t *testing.T) {
		cookie := NewSecureCookie[testSessionData](SecureCookieConfig{
			Manager: mgr,
			Name:    "session",
			TTL:     24 * time.Hour,
		})

		sessionData := testSessionData{
			UserID:    "user123",
			Username:  "johndoe",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		// Create encrypted cookie
		httpCookie, err := cookie.New(sessionData)
		if err != nil {
			t.Fatalf("unexpected error creating cookie: %v", err)
		}

		// Test retrieval
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(httpCookie)

		retrieved, err := cookie.Get(req)
		if err != nil {
			t.Fatalf("unexpected error retrieving cookie: %v", err)
		}
		if retrieved.UserID != sessionData.UserID {
			t.Errorf("expected UserID %s, got %s", sessionData.UserID, retrieved.UserID)
		}
		if retrieved.Username != sessionData.Username {
			t.Errorf("expected Username %s, got %s", sessionData.Username, retrieved.Username)
		}
	})

	t.Run("returns error for missing cookie", func(t *testing.T) {
		cookie := NewSecureCookie[string](SecureCookieConfig{
			Manager: mgr,
			Name:    "secure",
			TTL:     time.Hour,
		})

		req := httptest.NewRequest("GET", "/", nil)
		_, err := cookie.Get(req)
		if err == nil {
			t.Errorf("expected error for missing cookie")
		}
	})

	t.Run("returns error for empty cookie value", func(t *testing.T) {
		cookie := NewSecureCookie[string](SecureCookieConfig{
			Manager: mgr,
			Name:    "secure",
			TTL:     time.Hour,
		})

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  "__Host-secure",
			Value: "",
		})

		_, err := cookie.Get(req)
		if err == nil {
			t.Errorf("expected error for empty cookie value")
		}
		if !strings.Contains(err.Error(), "cookie value is empty") {
			t.Errorf("expected 'cookie value is empty' error, got: %v", err)
		}
	})

	t.Run("returns error for invalid encrypted data", func(t *testing.T) {
		cookie := NewSecureCookie[string](SecureCookieConfig{
			Manager: mgr,
			Name:    "secure",
			TTL:     time.Hour,
		})

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  "__Host-secure",
			Value: "invalid-encrypted-data",
		})

		_, err := cookie.Get(req)
		if err == nil {
			t.Errorf("expected error for invalid encrypted data")
		}
	})

	t.Run("creates deletion cookie", func(t *testing.T) {
		cookie := NewSecureCookie[string](SecureCookieConfig{
			Manager: mgr,
			Name:    "secure",
			TTL:     time.Hour,
		})

		deletion := cookie.NewDeletion()
		if deletion.Name != "__Host-secure" {
			t.Errorf("expected name __Host-secure, got %s", deletion.Name)
		}
		if deletion.Value != "" {
			t.Errorf("expected empty value, got %s", deletion.Value)
		}
		if deletion.MaxAge != -1 {
			t.Errorf("expected MaxAge -1, got %d", deletion.MaxAge)
		}
	})

	t.Run("respects explicit cookie overrides", func(t *testing.T) {
		cookie := NewSecureCookie[string](SecureCookieConfig{
			Manager:   mgr,
			Name:      "secure",
			TTL:       time.Hour,
			SameSite:  SameSiteStrictMode,
			Partition: PartitionFalse,
			HttpOnly:  HttpOnlyFalse,
		})

		httpCookie, _ := cookie.New("data")
		if httpCookie.SameSite != http.SameSiteStrictMode {
			t.Errorf("SameSite override was not respected")
		}
		if httpCookie.Partitioned {
			t.Errorf("Partition override was not respected")
		}
		if httpCookie.HttpOnly {
			t.Errorf("HttpOnly override was not respected")
		}
	})
}

// Test SecureCookieNonHostOnly
func TestSecureCookieNonHostOnly(t *testing.T) {
	mgr := newTestManager(false)

	t.Run("creates and retrieves with custom path and domain", func(t *testing.T) {
		cookie := NewSecureCookieNonHostOnly[string](SecureCookieNonHostOnlyConfig{
			Manager: mgr,
			Name:    "api-token",
			Path:    "/api",
			Domain:  ".example.com",
			TTL:     24 * time.Hour,
		})

		httpCookie, err := cookie.New("api-secret-token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if httpCookie.Name != "api-token" {
			t.Errorf("expected name api-token, got %s", httpCookie.Name)
		}
		if httpCookie.Path != "/api" {
			t.Errorf("expected path /api, got %s", httpCookie.Path)
		}
		if httpCookie.Domain != ".example.com" {
			t.Errorf("expected domain .example.com, got %s", httpCookie.Domain)
		}
		if !httpCookie.HttpOnly {
			t.Errorf("expected HttpOnly to be true by default")
		}

		req := httptest.NewRequest("GET", "/api", nil)
		req.AddCookie(httpCookie)

		retrieved, err := cookie.Get(req)
		if err != nil {
			t.Fatalf("unexpected error retrieving cookie: %v", err)
		}
		if retrieved != "api-secret-token" {
			t.Errorf("expected 'api-secret-token', got %s", retrieved)
		}
	})

	t.Run("defaults path to / when empty", func(t *testing.T) {
		cookie := NewSecureCookieNonHostOnly[string](SecureCookieNonHostOnlyConfig{
			Manager: mgr,
			Name:    "session",
			Path:    "",
			TTL:     time.Hour,
		})

		if cookie.spec.path != "/" {
			t.Errorf("expected path /, got %s", cookie.spec.path)
		}
	})

	t.Run("creates deletion cookie", func(t *testing.T) {
		cookie := NewSecureCookieNonHostOnly[string](SecureCookieNonHostOnlyConfig{
			Manager: mgr,
			Name:    "session",
			Path:    "/app",
			Domain:  ".example.com",
			TTL:     time.Hour,
		})

		deletion := cookie.NewDeletion()
		if deletion.Name != "session" {
			t.Errorf("expected name session, got %s", deletion.Name)
		}
		if deletion.Path != "/app" {
			t.Errorf("expected path /app, got %s", deletion.Path)
		}
		if deletion.Domain != ".example.com" {
			t.Errorf("expected domain .example.com, got %s", deletion.Domain)
		}
		if deletion.MaxAge != -1 {
			t.Errorf("expected MaxAge -1, got %d", deletion.MaxAge)
		}
	})
}

// Test ClientReadableCookie
func TestClientReadableCookie(t *testing.T) {
	mgr := newTestManager(false)

	t.Run("creates plaintext cookie", func(t *testing.T) {
		cookie := NewClientReadableCookie[string](ClientReadableCookieConfig{
			Manager: mgr,
			Name:    "theme",
			TTL:     30 * 24 * time.Hour,
		})

		httpCookie := cookie.New("dark-mode")

		if httpCookie.Name != "__Host-theme" {
			t.Errorf("expected name __Host-theme, got %s", httpCookie.Name)
		}
		if httpCookie.Value != "dark-mode" {
			t.Errorf("expected value dark-mode, got %s", httpCookie.Value)
		}
		if httpCookie.HttpOnly {
			t.Errorf("expected HttpOnly to be false for client-readable cookie")
		}
		if httpCookie.Path != "/" {
			t.Errorf("expected path /, got %s", httpCookie.Path)
		}
		if httpCookie.Domain != "" {
			t.Errorf("expected empty domain, got %s", httpCookie.Domain)
		}
	})

	t.Run("retrieves plaintext value", func(t *testing.T) {
		cookie := NewClientReadableCookie[string](ClientReadableCookieConfig{
			Manager: mgr,
			Name:    "theme",
			TTL:     30 * 24 * time.Hour,
		})

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  "__Host-theme",
			Value: "light-mode",
		})

		value, err := cookie.Get(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "light-mode" {
			t.Errorf("expected light-mode, got %s", value)
		}
	})

	t.Run("returns error for missing cookie", func(t *testing.T) {
		cookie := NewClientReadableCookie[string](ClientReadableCookieConfig{
			Manager: mgr,
			Name:    "theme",
			TTL:     30 * 24 * time.Hour,
		})

		req := httptest.NewRequest("GET", "/", nil)
		_, err := cookie.Get(req)
		if err == nil {
			t.Errorf("expected error for missing cookie")
		}
	})

	t.Run("works with custom string types", func(t *testing.T) {
		type Theme string
		cookie := NewClientReadableCookie[Theme](ClientReadableCookieConfig{
			Manager: mgr,
			Name:    "theme",
			TTL:     30 * 24 * time.Hour,
		})

		httpCookie := cookie.New(Theme("custom-theme"))
		if httpCookie.Value != "custom-theme" {
			t.Errorf("expected custom-theme, got %s", httpCookie.Value)
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(httpCookie)

		value, err := cookie.Get(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != Theme("custom-theme") {
			t.Errorf("expected custom-theme, got %s", value)
		}
	})
}

// Test ClientReadableCookieNonHostOnly
func TestClientReadableCookieNonHostOnly(t *testing.T) {
	mgr := newTestManager(false)

	t.Run("creates cookie with custom settings", func(t *testing.T) {
		cookie := NewClientReadableCookieNonHostOnly[string](ClientReadableCookieNonHostOnlyConfig{
			Manager:   mgr,
			Name:      "locale",
			Path:      "/app",
			Domain:    ".example.com",
			TTL:       365 * 24 * time.Hour,
			SameSite:  SameSiteStrictMode,
			Partition: PartitionTrue, // explicit check
		})

		httpCookie := cookie.New("en-US")

		if httpCookie.Name != "locale" {
			t.Errorf("expected name locale, got %s", httpCookie.Name)
		}
		if httpCookie.Value != "en-US" {
			t.Errorf("expected value en-US, got %s", httpCookie.Value)
		}
		if httpCookie.Path != "/app" {
			t.Errorf("expected path /app, got %s", httpCookie.Path)
		}
		if httpCookie.Domain != ".example.com" {
			t.Errorf("expected domain .example.com, got %s", httpCookie.Domain)
		}
		if httpCookie.HttpOnly {
			t.Errorf("expected HttpOnly to be false")
		}
		if httpCookie.SameSite != http.SameSiteStrictMode {
			t.Errorf("expected SameSite Strict")
		}
		if !httpCookie.Partitioned {
			t.Errorf("expected Partitioned to be true")
		}
	})

	t.Run("retrieves value with custom type", func(t *testing.T) {
		type Locale string
		cookie := NewClientReadableCookieNonHostOnly[Locale](ClientReadableCookieNonHostOnlyConfig{
			Manager: mgr,
			Name:    "locale",
			TTL:     365 * 24 * time.Hour,
		})

		httpCookie := cookie.New(Locale("fr-FR"))

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(httpCookie)

		value, err := cookie.Get(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != Locale("fr-FR") {
			t.Errorf("expected fr-FR, got %s", value)
		}
	})
}

// Test development mode behavior
func TestDevelopmentMode(t *testing.T) {
	mgr := newTestManager(true)

	t.Run("disables security features", func(t *testing.T) {
		spec := cookieSpec{
			name:          "test",
			value:         "value",
			path:          "/",
			domain:        "",
			ttl:           time.Hour,
			sameSite:      http.SameSiteLaxMode,
			httpOnly:      true,
			useHostPrefix: true,
			partitioned:   true,
		}

		cookie := mgr.buildCookie(spec)

		if cookie.Secure {
			t.Errorf("expected Secure to be false in dev mode")
		}
		if cookie.Partitioned {
			t.Errorf("expected Partitioned to be false in dev mode")
		}
		if !strings.HasPrefix(cookie.Name, "__Dev-") {
			t.Errorf("expected __Dev- prefix in dev mode, got %s", cookie.Name)
		}
	})

	t.Run("allows custom path/domain for host cookies in dev", func(t *testing.T) {
		mgr := newTestManager(true)
		cookie := NewSecureCookie[string](SecureCookieConfig{
			Manager: mgr,
			Name:    "test",
			TTL:     time.Hour,
		})

		spec := cookie.spec
		spec.path = "/custom"
		spec.domain = "localhost"
		httpCookie := mgr.buildCookie(spec)

		if httpCookie.Path != "/custom" {
			t.Errorf("expected path /custom in dev mode, got %s", httpCookie.Path)
		}
		if httpCookie.Domain != "localhost" {
			t.Errorf("expected domain localhost in dev mode, got %s", httpCookie.Domain)
		}
	})
}

// Test edge cases and security properties
func TestSecurityProperties(t *testing.T) {
	t.Run("host-only cookies enforce constraints in production", func(t *testing.T) {
		mgr := newTestManager(false)
		cookie := NewSecureCookie[string](SecureCookieConfig{
			Manager: mgr,
			Name:    "test",
			TTL:     time.Hour,
		})
		httpCookie, _ := cookie.New("value")
		if httpCookie.Path != "/" || httpCookie.Domain != "" || !httpCookie.Secure {
			t.Error("__Host- constraints not enforced")
		}
	})

	t.Run("partitioned is disabled in dev mode regardless of config", func(t *testing.T) {
		mgr := newTestManager(true)
		cookie := NewSecureCookie[string](SecureCookieConfig{
			Manager:   mgr,
			Name:      "test",
			TTL:       time.Hour,
			Partition: PartitionTrue,
		})
		httpCookie, _ := cookie.New("value")
		if httpCookie.Partitioned {
			t.Error("Partitioned should be false in dev mode")
		}
	})

	t.Run("empty TTL results in session cookie", func(t *testing.T) {
		mgr := newTestManager(false)
		cookie := NewClientReadableCookie[string](ClientReadableCookieConfig{
			Manager: mgr,
			Name:    "session",
			TTL:     0,
		})
		httpCookie := cookie.New("value")
		if httpCookie.MaxAge != 0 {
			t.Errorf("expected MaxAge 0 for session cookie, got %d", httpCookie.MaxAge)
		}
	})

	t.Run("negative TTL creates expired cookie", func(t *testing.T) {
		mgr := newTestManager(false)
		cookie := NewClientReadableCookie[string](ClientReadableCookieConfig{
			Manager: mgr,
			Name:    "expired",
			TTL:     -time.Hour,
		})
		httpCookie := cookie.New("value")
		if httpCookie.MaxAge >= 0 {
			t.Errorf("expected negative MaxAge for expired cookie, got %d", httpCookie.MaxAge)
		}
	})
}

// Test that all cookie types properly inherit manager defaults
func TestManagerDefaults(t *testing.T) {
	// Manager with non-system defaults: Strict, Not Partitioned, Not HttpOnly
	mgr := newTestManagerWithDefaults(false, SameSiteStrictMode, PartitionFalse, HttpOnlyFalse)

	t.Run("secure cookies inherit manager defaults", func(t *testing.T) {
		// Create a secure cookie with no overrides set (it will use ...Default).
		cookie := NewSecureCookie[string](SecureCookieConfig{
			Manager: mgr,
			Name:    "test",
		})
		spec := cookie.spec

		if spec.sameSite != http.SameSiteStrictMode {
			t.Error("Secure cookie did not inherit SameSite default")
		}
		if spec.partitioned {
			t.Error("Secure cookie did not inherit Partition default")
		}
		if spec.httpOnly {
			t.Error("Secure cookie did not inherit HttpOnly default")
		}
	})

	t.Run("client-readable cookies inherit manager defaults", func(t *testing.T) {
		cookie := NewClientReadableCookie[string](ClientReadableCookieConfig{
			Manager: mgr,
			Name:    "test",
		})
		spec := cookie.spec

		if spec.sameSite != http.SameSiteStrictMode {
			t.Error("Client cookie did not inherit SameSite default")
		}
		if spec.partitioned {
			t.Error("Client cookie did not inherit Partition default")
		}
		if spec.httpOnly {
			t.Error("Client cookie HttpOnly should always be false, regardless of manager default")
		}
	})
}

// Test cross-cookie compatibility
func TestCrossCookieCompatibility(t *testing.T) {
	t.Run("cookies work independently", func(t *testing.T) {
		mgr := newTestManager(false)
		secureCookie := NewSecureCookie[string](SecureCookieConfig{
			Manager: mgr,
			Name:    "secure-data",
		})
		clientCookie := NewClientReadableCookie[string](ClientReadableCookieConfig{
			Manager: mgr,
			Name:    "client-data",
		})
		secureHttp, _ := secureCookie.New("secure-value")
		clientHttp := clientCookie.New("client-value")
		if secureHttp.Name == clientHttp.Name {
			t.Errorf("cookies with different base names should have different final names")
		}
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(secureHttp)
		req.AddCookie(clientHttp)
		secureValue, err := secureCookie.Get(req)
		if err != nil || secureValue != "secure-value" {
			t.Error("failed to get secure cookie value")
		}
		clientValue, err := clientCookie.Get(req)
		if err != nil || clientValue != "client-value" {
			t.Error("failed to get client cookie value")
		}
	})
}

// Test to ensure type safety
func TestTypeSafety(t *testing.T) {
	mgr := newTestManager(false)
	t.Run("secure cookies work with complex types", func(t *testing.T) {
		type SessionData struct {
			UserID    string
			ExpiresAt time.Time
		}
		cookie := NewSecureCookie[SessionData](SecureCookieConfig{
			Manager: mgr,
			Name:    "session",
		})
		sessionData := SessionData{UserID: "user123", ExpiresAt: time.Now()}
		httpCookie, err := cookie.New(sessionData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(httpCookie.Value, "user123") {
			t.Errorf("cookie value should be encrypted")
		}
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(httpCookie)
		retrieved, err := cookie.Get(req)
		if err != nil {
			t.Fatalf("error retrieving cookie: %v", err)
		}
		if retrieved.UserID != sessionData.UserID {
			t.Errorf("expected UserID %s, got %s", sessionData.UserID, retrieved.UserID)
		}
	})

	t.Run("client cookies constrained to string types", func(t *testing.T) {
		type Theme string
		type Locale string
		themeCookie := NewClientReadableCookie[Theme](ClientReadableCookieConfig{
			Manager: mgr,
			Name:    "theme",
		})
		localeCookie := NewClientReadableCookie[Locale](ClientReadableCookieConfig{
			Manager: mgr,
			Name:    "locale",
		})
		themeHttp := themeCookie.New(Theme("dark"))
		if themeHttp.Value != "dark" {
			t.Errorf("expected dark, got %s", themeHttp.Value)
		}
		localeHttp := localeCookie.New(Locale("en-US"))
		if localeHttp.Value != "en-US" {
			t.Errorf("expected en-US, got %s", localeHttp.Value)
		}
	})
}

// Test cookie attributes combinations
func TestCookieAttributeCombinations(t *testing.T) {
	tests := []struct {
		name     string
		isDev    bool
		spec     cookieSpec
		validate func(t *testing.T, c *http.Cookie)
	}{
		{
			name:  "secure host-only production",
			isDev: false,
			spec: cookieSpec{
				name:          "secure",
				value:         "test",
				path:          "/custom",
				domain:        "example.com",
				ttl:           time.Hour,
				sameSite:      http.SameSiteLaxMode,
				httpOnly:      true,
				useHostPrefix: true,
				partitioned:   true,
			},
			validate: func(t *testing.T, c *http.Cookie) {
				if c.Name != "__Host-secure" || c.Path != "/" || c.Domain != "" || !c.Secure || !c.Partitioned {
					t.Error("validation failed for secure host-only production")
				}
			},
		},
		{
			name:  "client-readable non-host dev",
			isDev: true,
			spec: cookieSpec{
				name:          "theme",
				value:         "dark",
				path:          "/app",
				domain:        ".example.com",
				ttl:           30 * 24 * time.Hour,
				sameSite:      http.SameSiteStrictMode,
				httpOnly:      false,
				useHostPrefix: false,
				partitioned:   true,
			},
			validate: func(t *testing.T, c *http.Cookie) {
				if c.Name != "theme" || c.Secure || c.Partitioned || c.HttpOnly {
					t.Error("validation failed for client-readable non-host dev")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestManager(tt.isDev)
			cookie := mgr.buildCookie(tt.spec)
			tt.validate(t, cookie)
		})
	}
}

// Test error handling for securestring operations
func TestSecureStringErrors(t *testing.T) {
	mgr := newTestManager(false)
	t.Run("handles serialization errors gracefully", func(t *testing.T) {
		cookie := NewSecureCookie[string](SecureCookieConfig{
			Manager: mgr,
			Name:    "test",
		})
		_, err := cookie.New("normal-data")
		if err != nil {
			t.Errorf("unexpected error with normal data: %v", err)
		}
	})
}

// Test complete workflow
func TestCompleteWorkflow(t *testing.T) {
	mgr := newTestManager(false)
	t.Run("complete session workflow", func(t *testing.T) {
		sessionCookie := NewSecureCookie[testSessionData](SecureCookieConfig{
			Manager:  mgr,
			Name:     "session",
			TTL:      24 * time.Hour,
			SameSite: SameSiteStrictMode,
		})
		session := testSessionData{
			UserID:    "user123",
			Username:  "johndoe",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		httpCookie, err := sessionCookie.New(session)
		if err != nil {
			t.Fatalf("failed to create session cookie: %v", err)
		}
		w := httptest.NewRecorder()
		http.SetCookie(w, httpCookie)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header = http.Header{"Cookie": w.Header()["Set-Cookie"]}
		retrieved, err := sessionCookie.Get(req)
		if err != nil {
			t.Fatalf("failed to retrieve session: %v", err)
		}
		if retrieved.UserID != session.UserID {
			t.Errorf("expected UserID %s, got %s", session.UserID, retrieved.UserID)
		}
		deletionCookie := sessionCookie.NewDeletion()
		if deletionCookie.MaxAge != -1 {
			t.Errorf("deletion cookie should have MaxAge -1")
		}
	})

	t.Run("complete preference workflow", func(t *testing.T) {
		type UserPrefs string
		prefCookie := NewClientReadableCookieNonHostOnly[UserPrefs](ClientReadableCookieNonHostOnlyConfig{
			Manager:   mgr,
			Name:      "prefs",
			Path:      "/app",
			Domain:    ".example.com",
			TTL:       365 * 24 * time.Hour,
			SameSite:  SameSiteLaxMode,
			Partition: PartitionFalse,
		})
		prefs := UserPrefs("theme=dark;lang=en")
		httpCookie := prefCookie.New(prefs)
		if httpCookie.Path != "/app" || httpCookie.Domain != ".example.com" || httpCookie.HttpOnly {
			t.Error("preference cookie attributes not set correctly")
		}
		req := httptest.NewRequest("GET", "/app", nil)
		req.AddCookie(&http.Cookie{Name: "prefs", Value: "theme=darklang=en"})
		retrieved, err := prefCookie.Get(req)
		if err != nil {
			t.Fatalf("failed to retrieve preferences: %v", err)
		}
		if retrieved != UserPrefs("theme=darklang=en") {
			t.Errorf("expected theme=darklang=en, got %s", retrieved)
		}
	})
}

func TestNameMethod(t *testing.T) {
	// A map of test cases to run against different cookie types
	tests := []struct {
		testName      string
		isDev         bool
		cookieName    string
		useHostPrefix bool
		expectedName  string
	}{
		{
			"Host-Only in Production",
			false,
			"session",
			true,
			"__Host-session",
		},
		{
			"Host-Only in Development",
			true,
			"session",
			true,
			"__Dev-session",
		},
		{
			"Non-Host-Only in Production",
			false,
			"tracker",
			false,
			"tracker",
		},
		{
			"Non-Host-Only in Development",
			true,
			"tracker",
			false,
			"tracker",
		},
	}

	for _, tt := range tests {
		// Create a new manager for each test case's dev setting
		mgr := newTestManager(tt.isDev)

		// Test SecureCookie
		t.Run(tt.testName+": SecureCookie", func(t *testing.T) {
			if !tt.useHostPrefix {
				t.Skip("Skipping non-host prefix test for host-only cookie")
			}
			cookie := NewSecureCookie[string](SecureCookieConfig{
				Manager: mgr,
				Name:    tt.cookieName,
			})
			if name := cookie.Name(); name != tt.expectedName {
				t.Errorf("Expected name %q, got %q", tt.expectedName, name)
			}
		})

		// Test SecureCookieNonHostOnly
		t.Run(tt.testName+": SecureCookieNonHostOnly", func(t *testing.T) {
			if tt.useHostPrefix {
				t.Skip("Skipping host prefix test for non-host-only cookie")
			}
			cookie := NewSecureCookieNonHostOnly[string](SecureCookieNonHostOnlyConfig{
				Manager: mgr,
				Name:    tt.cookieName,
			})
			if name := cookie.Name(); name != tt.expectedName {
				t.Errorf("Expected name %q, got %q", tt.expectedName, name)
			}
		})

		// Test ClientReadableCookie
		t.Run(tt.testName+": ClientReadableCookie", func(t *testing.T) {
			if !tt.useHostPrefix {
				t.Skip("Skipping non-host prefix test for host-only cookie")
			}
			cookie := NewClientReadableCookie[string](ClientReadableCookieConfig{
				Manager: mgr,
				Name:    tt.cookieName,
			})
			if name := cookie.Name(); name != tt.expectedName {
				t.Errorf("Expected name %q, got %q", tt.expectedName, name)
			}
		})

		// Test ClientReadableCookieNonHostOnly
		t.Run(tt.testName+": ClientReadableCookieNonHostOnly", func(t *testing.T) {
			if tt.useHostPrefix {
				t.Skip("Skipping host prefix test for non-host-only cookie")
			}
			cookie := NewClientReadableCookieNonHostOnly[string](ClientReadableCookieNonHostOnlyConfig{
				Manager: mgr,
				Name:    tt.cookieName,
			})
			if name := cookie.Name(); name != tt.expectedName {
				t.Errorf("Expected name %q, got %q", tt.expectedName, name)
			}
		})
	}
}

// Test SetWithProxy, SetWithWriter, DeleteWithProxy, and DeleteWithWriter methods
func TestSetAndDeleteMethods(t *testing.T) {
	mgr := newTestManager(false)

	t.Run("SecureCookie SetWithProxy and DeleteWithProxy", func(t *testing.T) {
		cookie := NewSecureCookie[testSessionData](SecureCookieConfig{
			Manager: mgr,
			Name:    "session",
			TTL:     time.Hour,
		})

		sessionData := testSessionData{
			UserID:    "user123",
			Username:  "johndoe",
			ExpiresAt: time.Now().Add(time.Hour),
		}

		// Test SetWithProxy
		proxy := response.NewProxy()
		err := cookie.SetWithProxy(proxy, sessionData)
		if err != nil {
			t.Fatalf("unexpected error in SetWithProxy: %v", err)
		}

		cookies := proxy.GetCookies()
		if len(cookies) != 1 {
			t.Fatalf("expected 1 cookie, got %d", len(cookies))
		}

		setCookie := cookies[0]
		if setCookie.Name != "__Host-session" {
			t.Errorf("expected cookie name __Host-session, got %s", setCookie.Name)
		}
		if setCookie.MaxAge != 3600 {
			t.Errorf("expected MaxAge 3600, got %d", setCookie.MaxAge)
		}

		// Verify the cookie value is encrypted
		if setCookie.Value == "" || setCookie.Value == "user123" {
			t.Errorf("cookie value should be encrypted, got %s", setCookie.Value)
		}

		// Test DeleteWithProxy
		proxy2 := response.NewProxy()
		cookie.DeleteWithProxy(proxy2)

		deleteCookies := proxy2.GetCookies()
		if len(deleteCookies) != 1 {
			t.Fatalf("expected 1 cookie for deletion, got %d", len(deleteCookies))
		}

		deleteCookie := deleteCookies[0]
		if deleteCookie.Name != "__Host-session" {
			t.Errorf("expected cookie name __Host-session, got %s", deleteCookie.Name)
		}
		if deleteCookie.MaxAge != -1 {
			t.Errorf("expected MaxAge -1 for deletion, got %d", deleteCookie.MaxAge)
		}
		if deleteCookie.Value != "" {
			t.Errorf("expected empty value for deletion, got %s", deleteCookie.Value)
		}
	})

	t.Run("SecureCookie SetWithWriter and DeleteWithWriter", func(t *testing.T) {
		cookie := NewSecureCookie[string](SecureCookieConfig{
			Manager: mgr,
			Name:    "token",
			TTL:     2 * time.Hour,
		})

		// Test SetWithWriter
		w := httptest.NewRecorder()
		err := cookie.SetWithWriter(w, "secret-token-value")
		if err != nil {
			t.Fatalf("unexpected error in SetWithWriter: %v", err)
		}

		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("expected 1 cookie, got %d", len(cookies))
		}

		setCookie := cookies[0]
		if setCookie.Name != "__Host-token" {
			t.Errorf("expected cookie name __Host-token, got %s", setCookie.Name)
		}
		if setCookie.MaxAge != 7200 {
			t.Errorf("expected MaxAge 7200, got %d", setCookie.MaxAge)
		}

		// Test DeleteWithWriter
		w2 := httptest.NewRecorder()
		cookie.DeleteWithWriter(w2)

		deleteCookies := w2.Result().Cookies()
		if len(deleteCookies) != 1 {
			t.Fatalf("expected 1 cookie for deletion, got %d", len(deleteCookies))
		}

		deleteCookie := deleteCookies[0]
		if deleteCookie.Name != "__Host-token" {
			t.Errorf("expected cookie name __Host-token, got %s", deleteCookie.Name)
		}
		if deleteCookie.MaxAge != -1 {
			t.Errorf("expected MaxAge -1 for deletion, got %d", deleteCookie.MaxAge)
		}
	})

	t.Run("SecureCookieNonHostOnly SetWithProxy and DeleteWithProxy", func(t *testing.T) {
		cookie := NewSecureCookieNonHostOnly[string](SecureCookieNonHostOnlyConfig{
			Manager: mgr,
			Name:    "api-token",
			Path:    "/api",
			Domain:  ".example.com",
			TTL:     24 * time.Hour,
		})

		// Test SetWithProxy
		proxy := response.NewProxy()
		err := cookie.SetWithProxy(proxy, "api-secret-value")
		if err != nil {
			t.Fatalf("unexpected error in SetWithProxy: %v", err)
		}

		cookies := proxy.GetCookies()
		if len(cookies) != 1 {
			t.Fatalf("expected 1 cookie, got %d", len(cookies))
		}

		setCookie := cookies[0]
		if setCookie.Name != "api-token" {
			t.Errorf("expected cookie name api-token, got %s", setCookie.Name)
		}
		if setCookie.Path != "/api" {
			t.Errorf("expected path /api, got %s", setCookie.Path)
		}
		if setCookie.Domain != ".example.com" {
			t.Errorf("expected domain .example.com, got %s", setCookie.Domain)
		}

		// Test DeleteWithProxy
		proxy2 := response.NewProxy()
		cookie.DeleteWithProxy(proxy2)

		deleteCookies := proxy2.GetCookies()
		if len(deleteCookies) != 1 {
			t.Fatalf("expected 1 cookie for deletion, got %d", len(deleteCookies))
		}

		deleteCookie := deleteCookies[0]
		if deleteCookie.Path != "/api" {
			t.Errorf("expected path /api for deletion, got %s", deleteCookie.Path)
		}
		if deleteCookie.Domain != ".example.com" {
			t.Errorf("expected domain .example.com for deletion, got %s", deleteCookie.Domain)
		}
		if deleteCookie.MaxAge != -1 {
			t.Errorf("expected MaxAge -1 for deletion, got %d", deleteCookie.MaxAge)
		}
	})

	t.Run("ClientReadableCookie SetWithProxy and DeleteWithProxy", func(t *testing.T) {
		cookie := NewClientReadableCookie[string](ClientReadableCookieConfig{
			Manager: mgr,
			Name:    "theme",
			TTL:     30 * 24 * time.Hour,
		})

		// Test SetWithProxy
		proxy := response.NewProxy()
		cookie.SetWithProxy(proxy, "dark-mode")

		cookies := proxy.GetCookies()
		if len(cookies) != 1 {
			t.Fatalf("expected 1 cookie, got %d", len(cookies))
		}

		setCookie := cookies[0]
		if setCookie.Name != "__Host-theme" {
			t.Errorf("expected cookie name __Host-theme, got %s", setCookie.Name)
		}
		if setCookie.Value != "dark-mode" {
			t.Errorf("expected value dark-mode, got %s", setCookie.Value)
		}
		if setCookie.HttpOnly {
			t.Errorf("expected HttpOnly to be false for client-readable cookie")
		}

		// Test DeleteWithProxy
		proxy2 := response.NewProxy()
		cookie.DeleteWithProxy(proxy2)

		deleteCookies := proxy2.GetCookies()
		if len(deleteCookies) != 1 {
			t.Fatalf("expected 1 cookie for deletion, got %d", len(deleteCookies))
		}

		deleteCookie := deleteCookies[0]
		if deleteCookie.Name != "__Host-theme" {
			t.Errorf("expected cookie name __Host-theme, got %s", deleteCookie.Name)
		}
		if deleteCookie.MaxAge != -1 {
			t.Errorf("expected MaxAge -1 for deletion, got %d", deleteCookie.MaxAge)
		}
		if deleteCookie.Value != "" {
			t.Errorf("expected empty value for deletion, got %s", deleteCookie.Value)
		}
	})

	t.Run("ClientReadableCookie SetWithWriter and DeleteWithWriter", func(t *testing.T) {
		type Locale string
		cookie := NewClientReadableCookie[Locale](ClientReadableCookieConfig{
			Manager: mgr,
			Name:    "locale",
			TTL:     365 * 24 * time.Hour,
		})

		// Test SetWithWriter
		w := httptest.NewRecorder()
		cookie.SetWithWriter(w, Locale("en-US"))

		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("expected 1 cookie, got %d", len(cookies))
		}

		setCookie := cookies[0]
		if setCookie.Name != "__Host-locale" {
			t.Errorf("expected cookie name __Host-locale, got %s", setCookie.Name)
		}
		if setCookie.Value != "en-US" {
			t.Errorf("expected value en-US, got %s", setCookie.Value)
		}

		// Test DeleteWithWriter
		w2 := httptest.NewRecorder()
		cookie.DeleteWithWriter(w2)

		deleteCookies := w2.Result().Cookies()
		if len(deleteCookies) != 1 {
			t.Fatalf("expected 1 cookie for deletion, got %d", len(deleteCookies))
		}

		deleteCookie := deleteCookies[0]
		if deleteCookie.MaxAge != -1 {
			t.Errorf("expected MaxAge -1 for deletion, got %d", deleteCookie.MaxAge)
		}
	})

	t.Run("ClientReadableCookieNonHostOnly SetWithProxy and DeleteWithProxy", func(t *testing.T) {
		cookie := NewClientReadableCookieNonHostOnly[string](ClientReadableCookieNonHostOnlyConfig{
			Manager: mgr,
			Name:    "preferences",
			Path:    "/app",
			Domain:  ".example.com",
			TTL:     90 * 24 * time.Hour,
		})

		// Test SetWithProxy
		proxy := response.NewProxy()
		cookie.SetWithProxy(proxy, "lang=en;tz=UTC")

		cookies := proxy.GetCookies()
		if len(cookies) != 1 {
			t.Fatalf("expected 1 cookie, got %d", len(cookies))
		}

		setCookie := cookies[0]
		if setCookie.Name != "preferences" {
			t.Errorf("expected cookie name preferences, got %s", setCookie.Name)
		}
		if setCookie.Value != "lang=en;tz=UTC" {
			t.Errorf("expected value lang=en;tz=UTC, got %s", setCookie.Value)
		}
		if setCookie.Path != "/app" {
			t.Errorf("expected path /app, got %s", setCookie.Path)
		}
		if setCookie.Domain != ".example.com" {
			t.Errorf("expected domain .example.com, got %s", setCookie.Domain)
		}

		// Test DeleteWithProxy
		proxy2 := response.NewProxy()
		cookie.DeleteWithProxy(proxy2)

		deleteCookies := proxy2.GetCookies()
		if len(deleteCookies) != 1 {
			t.Fatalf("expected 1 cookie for deletion, got %d", len(deleteCookies))
		}

		deleteCookie := deleteCookies[0]
		if deleteCookie.MaxAge != -1 {
			t.Errorf("expected MaxAge -1 for deletion, got %d", deleteCookie.MaxAge)
		}
		if deleteCookie.Path != "/app" {
			t.Errorf("expected path /app for deletion, got %s", deleteCookie.Path)
		}
	})

	t.Run("Error handling in SetWithProxy", func(t *testing.T) {
		// Create a cookie that will have serialization issues
		// Using a channel as it cannot be serialized
		type BadData struct {
			Ch chan int
		}

		cookie := NewSecureCookie[BadData](SecureCookieConfig{
			Manager: mgr,
			Name:    "bad",
			TTL:     time.Hour,
		})

		proxy := response.NewProxy()
		err := cookie.SetWithProxy(proxy, BadData{Ch: make(chan int)})
		if err == nil {
			t.Errorf("expected error when serializing channel, got nil")
		}
		if !strings.Contains(err.Error(), "failed to create secure cookie") {
			t.Errorf("expected error message to contain 'failed to create secure cookie', got: %v", err)
		}

		// Verify no cookie was added to proxy
		if len(proxy.GetCookies()) != 0 {
			t.Errorf("expected no cookies on error, got %d", len(proxy.GetCookies()))
		}
	})

	t.Run("Error handling in SetWithWriter", func(t *testing.T) {
		type BadData struct {
			Ch chan int
		}

		cookie := NewSecureCookie[BadData](SecureCookieConfig{
			Manager: mgr,
			Name:    "bad",
			TTL:     time.Hour,
		})

		w := httptest.NewRecorder()
		err := cookie.SetWithWriter(w, BadData{Ch: make(chan int)})
		if err == nil {
			t.Errorf("expected error when serializing channel, got nil")
		}
		if !strings.Contains(err.Error(), "failed to create secure cookie") {
			t.Errorf("expected error message to contain 'failed to create secure cookie', got: %v", err)
		}

		// Verify no cookie was set
		if len(w.Result().Cookies()) != 0 {
			t.Errorf("expected no cookies on error, got %d", len(w.Result().Cookies()))
		}
	})
}

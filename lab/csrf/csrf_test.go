package csrf

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/kit/keyset"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/lab/cookies"
)

// Test helpers
func createTestKeyset(t *testing.T) *keyset.Keyset {
	// Create a valid base64-encoded 32-byte secret
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i)
	}
	b64Secret := base64.StdEncoding.EncodeToString(secret)

	ks, err := keyset.RootSecretsToRootKeyset(keyset.RootSecrets{keyset.RootSecret(b64Secret)})
	if err != nil {
		t.Fatalf("Failed to create test keyset: %v", err)
	}
	return ks
}

func createTestCookieManager(t *testing.T) *cookies.Manager {
	testKeyset := createTestKeyset(t)
	return cookies.NewManager(cookies.ManagerConfig{
		GetKeyset: func() *keyset.Keyset { return testKeyset },
	})
}

func createTestProtector(t *testing.T, origins []string) *Protector {
	cfg := ProtectorConfig{
		CookieManager:  createTestCookieManager(t),
		GetSessionID:   func(r *http.Request) string { return "" },
		AllowedOrigins: origins,
		TokenTTL:       1 * time.Hour,
	}
	return NewProtector(cfg)
}

func extractCSRFCookie(rr *httptest.ResponseRecorder, cookieName string) *http.Cookie {
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == cookieName {
			return cookie
		}
	}
	return nil
}

func extractTokenFromCookie(cookie *http.Cookie) string {
	return cookie.Value
}

// Tests
func TestNewProtector(t *testing.T) {
	cookieManager := createTestCookieManager(t)

	tests := []struct {
		name  string
		cfg   ProtectorConfig
		check func(*testing.T, *Protector)
	}{
		{
			name: "valid config with defaults",
			cfg: ProtectorConfig{
				CookieManager: cookieManager,
				GetSessionID:  func(r *http.Request) string { return "" },
			},
			check: func(t *testing.T, p *Protector) {
				if p.cfg.TokenTTL != 4*time.Hour {
					t.Errorf("Expected default TTL of 4h, got %v", p.cfg.TokenTTL)
				}
				if p.cfg.CookieName != "csrf_token" {
					t.Errorf("Expected default cookie suffix 'csrf_token', got %s", p.cfg.CookieName)
				}
				if p.cfg.HeaderName != "X-CSRF-Token" {
					t.Errorf("Expected default header name 'X-CSRF-Token', got %s", p.cfg.HeaderName)
				}
				if p.cookie.Name() != "__Host-csrf_token" {
					t.Errorf("Expected cookie name '__Host-csrf_token', got %s", p.cookie.Name())
				}
			},
		},
		{
			name: "custom values",
			cfg: ProtectorConfig{
				CookieManager:  cookieManager,
				GetSessionID:   func(r *http.Request) string { return "" },
				AllowedOrigins: []string{"https://example.com", "HTTPS://EXAMPLE.ORG"},
				TokenTTL:       2 * time.Hour,
				CookieName:     "custom",
				HeaderName:     "X-Custom-CSRF",
			},
			check: func(t *testing.T, p *Protector) {
				if p.cfg.TokenTTL != 2*time.Hour {
					t.Errorf("Expected TTL of 2h, got %v", p.cfg.TokenTTL)
				}
				if p.cookie.Name() != "__Host-custom" {
					t.Errorf("Expected cookie name '__Host-custom', got %s", p.cookie.Name())
				}
				if !p.allowedOrigins["https://example.com"] {
					t.Error("Expected normalized origin 'https://example.com' to be allowed")
				}
				if !p.allowedOrigins["https://example.org"] {
					t.Error("Expected normalized origin 'https://example.org' to be allowed")
				}
				if p.hasOriginRestrictions != true {
					t.Error("Expected hasOriginRestrictions to be true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProtector(tt.cfg)
			if tt.check != nil {
				tt.check(t, p)
			}
		})
	}
}

func TestMiddleware_GETRequest(t *testing.T) {
	p := createTestProtector(t, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	tests := []struct {
		name           string
		method         string
		existingCookie *http.Cookie
		wantCookie     bool
		wantStatus     int
	}{
		{
			name:       "GET without existing cookie",
			method:     "GET",
			wantCookie: true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "HEAD without existing cookie",
			method:     "HEAD",
			wantCookie: true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "OPTIONS without existing cookie",
			method:     "OPTIONS",
			wantCookie: true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "TRACE without existing cookie",
			method:     "TRACE",
			wantCookie: true,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.existingCookie != nil {
				req.AddCookie(tt.existingCookie)
			}

			rr := httptest.NewRecorder()
			p.Middleware(handler).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			cookie := extractCSRFCookie(rr, p.cookie.Name())
			if tt.wantCookie && cookie == nil {
				t.Error("Expected CSRF cookie to be set")
			}
			if tt.wantCookie && cookie != nil {
				// Verify cookie attributes
				if !cookie.Secure {
					t.Error("Expected Secure flag to be true")
				}
				if cookie.SameSite != http.SameSiteLaxMode {
					t.Errorf("Expected SameSite=Lax, got %v", cookie.SameSite)
				}
				if cookie.HttpOnly {
					t.Error("Expected HttpOnly to be false (must be readable by JS)")
				}
				if cookie.Path != "/" {
					t.Errorf("Expected Path=/, got %s", cookie.Path)
				}
				if cookie.Domain != "" {
					t.Errorf("Expected empty Domain for __Host- prefix, got %s", cookie.Domain)
				}
			}
		})
	}
}

func TestMiddleware_POSTRequest(t *testing.T) {
	p := createTestProtector(t, []string{"https://example.com"})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// First, get a valid CSRF token via GET request
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(getRR, getReq)

	cookie := extractCSRFCookie(getRR, p.cookie.Name())
	if cookie == nil {
		t.Fatal("Failed to get CSRF cookie from GET request")
	}

	// Extract the token to use (which is the cookie value itself)
	token := extractTokenFromCookie(cookie)

	tests := []struct {
		name       string
		method     string
		cookie     *http.Cookie
		token      string
		origin     string
		referer    string
		wantStatus int
	}{
		{
			name:       "valid POST with token and origin",
			method:     "POST",
			cookie:     cookie,
			token:      token,
			origin:     "https://example.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid POST with token and referer",
			method:     "POST",
			cookie:     cookie,
			token:      token,
			referer:    "https://example.com/page",
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST without cookie",
			method:     "POST",
			token:      token,
			origin:     "https://example.com",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "POST without token header",
			method:     "POST",
			cookie:     cookie,
			origin:     "https://example.com",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "POST with wrong token",
			method:     "POST",
			cookie:     cookie,
			token:      "wrong-token",
			origin:     "https://example.com",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "POST with wrong origin",
			method:     "POST",
			cookie:     cookie,
			token:      token,
			origin:     "https://evil.com",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "POST with wrong referer",
			method:     "POST",
			cookie:     cookie,
			token:      token,
			referer:    "https://evil.com/page",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "PUT request",
			method:     "PUT",
			cookie:     cookie,
			token:      token,
			origin:     "https://example.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "DELETE request",
			method:     "DELETE",
			cookie:     cookie,
			token:      token,
			origin:     "https://example.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "PATCH request",
			method:     "PATCH",
			cookie:     cookie,
			token:      token,
			origin:     "https://example.com",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			if tt.token != "" {
				req.Header.Set(p.cfg.HeaderName, tt.token)
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}

			rr := httptest.NewRecorder()
			p.Middleware(handler).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

func TestMiddleware_NoOriginRestrictions(t *testing.T) {
	p := createTestProtector(t, nil) // No allowed origins

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Get token
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(getRR, getReq)

	cookie := extractCSRFCookie(getRR, p.cookie.Name())
	token := extractTokenFromCookie(cookie)

	// POST should succeed without origin validation
	req := httptest.NewRequest("POST", "/", nil)
	req.AddCookie(cookie)
	req.Header.Set(p.cfg.HeaderName, token)
	req.Header.Set("Origin", "https://any-origin.com")

	rr := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestCycleTokenWithProxy(t *testing.T) {
	p := createTestProtector(t, nil)

	tests := []struct {
		name      string
		sessionID string
	}{
		{
			name:      "cycle with session",
			sessionID: "test-session-123",
		},
		{
			name:      "cycle without session",
			sessionID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp := response.NewProxy()

			err := p.CycleTokenWithProxy(rp, tt.sessionID)
			if err != nil {
				t.Fatalf("CycleTokenWithProxy failed: %v", err)
			}

			// Apply proxy to response writer to get cookies
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			rp.ApplyToResponseWriter(rr, req)

			cookie := extractCSRFCookie(rr, p.cookie.Name())
			if cookie == nil {
				t.Fatal("No cookie set after CycleTokenWithProxy")
			}

			// Decode and verify session ID
			req.AddCookie(cookie)
			payload, err := p.cookie.Get(req)
			if err != nil {
				t.Fatalf("Failed to decode cycled token: %v", err)
			}

			if payload.SessionID != tt.sessionID {
				t.Errorf("Expected session ID %q, got %q", tt.sessionID, payload.SessionID)
			}
		})
	}
}

func TestCycleTokenWithWriter(t *testing.T) {
	p := createTestProtector(t, nil)

	tests := []struct {
		name      string
		sessionID string
	}{
		{
			name:      "cycle with session",
			sessionID: "test-session-123",
		},
		{
			name:      "cycle without session",
			sessionID: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			err := p.CycleTokenWithWriter(rr, req, tt.sessionID)
			if err != nil {
				t.Fatalf("CycleTokenWithWriter failed: %v", err)
			}

			cookie := extractCSRFCookie(rr, p.cookie.Name())
			if cookie == nil {
				t.Fatal("No cookie set after CycleTokenWithWriter")
			}

			// Decode and verify session ID
			req.AddCookie(cookie)
			payload, err := p.cookie.Get(req)
			if err != nil {
				t.Fatalf("Failed to decode cycled token: %v", err)
			}

			if payload.SessionID != tt.sessionID {
				t.Errorf("Expected session ID %q, got %q", tt.sessionID, payload.SessionID)
			}
		})
	}
}

func TestTokenExpiration(t *testing.T) {
	cfg := ProtectorConfig{
		CookieManager: createTestCookieManager(t),
		GetSessionID:  func(r *http.Request) string { return "" },
		TokenTTL:      1 * time.Second,
	}
	p := NewProtector(cfg)

	// Create token
	rp := response.NewProxy()
	err := p.CycleTokenWithProxy(rp, "")
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	// Apply proxy to get cookie
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	rp.ApplyToResponseWriter(rr, req)

	cookie := extractCSRFCookie(rr, p.cookie.Name())
	token := extractTokenFromCookie(cookie)

	// Immediate validation should succeed
	req = httptest.NewRequest("POST", "/", nil)
	req.AddCookie(cookie)
	req.Header.Set(p.cfg.HeaderName, token)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rr = httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Error("Expected immediate validation to succeed")
	}

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Validation should now fail
	rr = httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Error("Expected validation to fail after expiration")
	}

	// Verify self-healing occurred for expired token
	newCookie := extractCSRFCookie(rr, p.cookie.Name())
	if newCookie == nil {
		t.Error("Expected self-healing to provide new cookie for expired token")
	}
}

func TestGETRequestWithExistingValidToken(t *testing.T) {
	p := createTestProtector(t, nil)

	// First GET to get token
	req1 := httptest.NewRequest("GET", "/", nil)
	rr1 := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	p.Middleware(handler).ServeHTTP(rr1, req1)

	cookie := extractCSRFCookie(rr1, p.cookie.Name())
	if cookie == nil {
		t.Fatal("No cookie from first GET")
	}

	// Second GET with existing valid cookie
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.AddCookie(cookie)
	rr2 := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(rr2, req2)

	// Should not issue new cookie
	newCookie := extractCSRFCookie(rr2, p.cookie.Name())
	if newCookie != nil {
		t.Error("Should not issue new cookie when valid one exists")
	}
}

func TestOriginValidationWithMalformedReferer(t *testing.T) {
	p := createTestProtector(t, []string{"https://example.com"})

	// Get token
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	p.Middleware(handler).ServeHTTP(getRR, getReq)

	cookie := extractCSRFCookie(getRR, p.cookie.Name())
	token := extractTokenFromCookie(cookie)

	// POST with malformed referer
	req := httptest.NewRequest("POST", "/", nil)
	req.AddCookie(cookie)
	req.Header.Set(p.cfg.HeaderName, token)
	req.Header.Set("Referer", "not-a-valid-url")

	rr := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected %d for malformed referer, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestCookieAttributes(t *testing.T) {
	p := createTestProtector(t, nil)

	rp := response.NewProxy()
	err := p.CycleTokenWithProxy(rp, "")
	if err != nil {
		t.Fatalf("Failed to cycle token: %v", err)
	}

	// Apply proxy to get cookie
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	rp.ApplyToResponseWriter(rr, req)

	cookie := extractCSRFCookie(rr, p.cookie.Name())
	if cookie == nil {
		t.Fatal("No cookie set")
	}

	// Verify all security-critical attributes
	if !strings.HasPrefix(cookie.Name, "__Host-") {
		t.Errorf("Cookie name must start with __Host-, got %s", cookie.Name)
	}
	if !cookie.Secure {
		t.Error("Cookie must have Secure flag")
	}
	if cookie.Domain != "" {
		t.Error("Cookie must have empty Domain for __Host- prefix")
	}
	if cookie.Path != "/" {
		t.Error("Cookie must have Path=/")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("Cookie must have SameSite=Lax, got %v", cookie.SameSite)
	}
	if cookie.HttpOnly {
		t.Error("Cookie must not be HttpOnly (needs JS access)")
	}
	if !cookie.Partitioned {
		t.Error("Cookie should be Partitioned")
	}
}

// TestDevMode tests the development mode functionality
func TestDevMode(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		shouldPanic bool
	}{
		{
			name:        "localhost allowed",
			host:        "localhost:8080",
			shouldPanic: false,
		},
		{
			name:        "127.0.0.1 allowed",
			host:        "127.0.0.1:3000",
			shouldPanic: false,
		},
		{
			name:        "::1 allowed",
			host:        "[::1]:8080",
			shouldPanic: false,
		},
		{
			name:        "localhost without port allowed",
			host:        "localhost",
			shouldPanic: false,
		},
		{
			name:        "non-localhost should panic",
			host:        "example.com",
			shouldPanic: true,
		},
		{
			name:        "IP address should panic",
			host:        "192.168.1.1:8080",
			shouldPanic: true,
		},
		{
			name:        "subdomain should panic",
			host:        "app.localhost:8080",
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cookieManager := cookies.NewManager(cookies.ManagerConfig{
				GetKeyset: func() *keyset.Keyset { return createTestKeyset(t) },
				GetIsDev:  func() bool { return true }, // Enable dev mode
			})

			p := NewProtector(ProtectorConfig{
				CookieManager: cookieManager,
				GetSessionID:  func(r *http.Request) string { return "" },
			})

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tt.host
			rr := httptest.NewRecorder()

			if tt.shouldPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("Expected panic for host %s, but didn't panic", tt.host)
					}
				}()
			}

			p.Middleware(handler).ServeHTTP(rr, req)

			if !tt.shouldPanic {
				// Verify dev mode cookie attributes
				cookie := extractCSRFCookie(rr, p.cookie.Name())
				if cookie == nil {
					t.Fatal("Expected cookie to be set")
				}

				// In dev mode, should have __Dev- prefix
				if cookie.Name != "__Dev-csrf_token" {
					t.Errorf("Expected cookie name '__Dev-csrf_token' in dev mode, got %s", cookie.Name)
				}

				// Should NOT be Secure in dev mode
				if cookie.Secure {
					t.Error("Cookie should not be Secure in dev mode")
				}

				// Should NOT be Partitioned in dev mode
				if cookie.Partitioned {
					t.Error("Cookie should not be Partitioned in dev mode")
				}
			}
		})
	}
}

// TestDevModeVsProductionMode compares behavior between modes
func TestDevModeVsProductionMode(t *testing.T) {
	// Test production mode (default)
	prodCookieManager := createTestCookieManager(t)
	prodProtector := NewProtector(ProtectorConfig{
		CookieManager: prodCookieManager,
		GetSessionID:  func(r *http.Request) string { return "" },
		// GetIsDev is nil, so production mode
	})

	// Test dev mode
	devCookieManager := cookies.NewManager(cookies.ManagerConfig{
		GetKeyset: func() *keyset.Keyset { return createTestKeyset(t) },
		GetIsDev:  func() bool { return true },
	})
	devProtector := NewProtector(ProtectorConfig{
		CookieManager: devCookieManager,
		GetSessionID:  func(r *http.Request) string { return "" },
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Production mode test
	t.Run("production mode", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		prodProtector.Middleware(handler).ServeHTTP(rr, req)

		cookie := extractCSRFCookie(rr, prodProtector.cookie.Name())
		if cookie.Name != "__Host-csrf_token" {
			t.Errorf("Expected __Host- prefix in production, got %s", cookie.Name)
		}
		if !cookie.Secure {
			t.Error("Expected Secure flag in production")
		}
		if !cookie.Partitioned {
			t.Error("Expected Partitioned flag in production")
		}
	})

	// Dev mode test
	t.Run("dev mode", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "localhost:8080"
		rr := httptest.NewRecorder()
		devProtector.Middleware(handler).ServeHTTP(rr, req)

		cookie := extractCSRFCookie(rr, devProtector.cookie.Name())
		if cookie.Name != "__Dev-csrf_token" {
			t.Errorf("Expected __Dev- prefix in dev mode, got %s", cookie.Name)
		}
		if cookie.Secure {
			t.Error("Expected no Secure flag in dev mode")
		}
		if cookie.Partitioned {
			t.Error("Expected no Partitioned flag in dev mode")
		}
	})
}

// TestOriginValidationEdgeCases tests edge cases in origin validation
func TestOriginValidationEdgeCases(t *testing.T) {
	p := createTestProtector(t, []string{"https://example.com", "http://localhost:3000"})

	// Get a valid token
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	p.Middleware(handler).ServeHTTP(getRR, getReq)

	cookie := extractCSRFCookie(getRR, p.cookie.Name())
	token := extractTokenFromCookie(cookie)

	tests := []struct {
		name       string
		origin     string
		referer    string
		wantStatus int
	}{
		{
			name:       "no origin or referer with restrictions should pass",
			wantStatus: http.StatusOK,
		},
		{
			name:       "case insensitive origin matching",
			origin:     "HTTPS://EXAMPLE.COM",
			wantStatus: http.StatusOK,
		},
		{
			name:       "case insensitive referer matching",
			referer:    "HTTPS://EXAMPLE.COM/page",
			wantStatus: http.StatusOK,
		},
		{
			name:       "origin takes precedence over referer",
			origin:     "https://example.com",
			referer:    "https://evil.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "localhost with port allowed",
			origin:     "http://localhost:3000",
			wantStatus: http.StatusOK,
		},
		{
			name:       "empty origin and referer",
			origin:     "",
			referer:    "",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", nil)
			req.AddCookie(cookie)
			req.Header.Set(p.cfg.HeaderName, token)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}

			rr := httptest.NewRecorder()
			p.Middleware(handler).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

// TestCustomHeaderName tests using a custom header name
func TestCustomHeaderName(t *testing.T) {
	customHeaderName := "X-Custom-CSRF-Token"

	p := NewProtector(ProtectorConfig{
		CookieManager: createTestCookieManager(t),
		GetSessionID:  func(r *http.Request) string { return "" },
		HeaderName:    customHeaderName,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Get token
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(getRR, getReq)

	cookie := extractCSRFCookie(getRR, p.cookie.Name())
	token := extractTokenFromCookie(cookie)

	// POST with custom header
	req := httptest.NewRequest("POST", "/", nil)
	req.AddCookie(cookie)
	req.Header.Set(customHeaderName, token)

	rr := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected %d with custom header, got %d", http.StatusOK, rr.Code)
	}

	// POST with default header name should fail
	req2 := httptest.NewRequest("POST", "/", nil)
	req2.AddCookie(cookie)
	req2.Header.Set("X-CSRF-Token", token) // Using default header name

	rr2 := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusForbidden {
		t.Errorf("Expected %d with wrong header name, got %d", http.StatusForbidden, rr2.Code)
	}
}

// TestInvalidTokenPayload tests handling of corrupted tokens
func TestInvalidTokenPayload(t *testing.T) {
	p := createTestProtector(t, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		tokenValue string
		wantStatus int
	}{
		{
			name:       "empty token",
			tokenValue: "",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "invalid base64",
			tokenValue: "not-valid-base64!@#$",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "truncated token",
			tokenValue: "SGVsbG8=", // Valid base64 but not a valid encrypted payload
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "random data",
			tokenValue: base64.StdEncoding.EncodeToString([]byte("random data that's not encrypted")),
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", nil)
			req.AddCookie(&http.Cookie{
				Name:  p.cookie.Name(),
				Value: tt.tokenValue,
			})
			req.Header.Set(p.cfg.HeaderName, tt.tokenValue)

			rr := httptest.NewRecorder()
			p.Middleware(handler).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d for %s, got %d", tt.wantStatus, tt.name, rr.Code)
			}
		})
	}
}

// TestConcurrentRequests tests thread safety
func TestConcurrentRequests(t *testing.T) {
	p := createTestProtector(t, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Get a valid token
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(getRR, getReq)

	cookie := extractCSRFCookie(getRR, p.cookie.Name())
	token := extractTokenFromCookie(cookie)

	// Run concurrent POST requests
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest("POST", "/", nil)
			req.AddCookie(cookie)
			req.Header.Set(p.cfg.HeaderName, token)

			rr := httptest.NewRecorder()
			p.Middleware(handler).ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Concurrent request failed with status %d", rr.Code)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestResponseProxyIntegration verifies response proxy usage
func TestResponseProxyIntegration(t *testing.T) {
	p := createTestProtector(t, nil)

	// Custom handler that writes before middleware completes
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write some content
		w.Header().Set("X-Custom-Header", "test")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test content"))
	})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	p.Middleware(handler).ServeHTTP(rr, req)

	// Verify cookie was set despite handler writing response
	cookie := extractCSRFCookie(rr, p.cookie.Name())
	if cookie == nil {
		t.Error("Cookie should be set even when handler writes response")
	}

	// Verify custom header is preserved
	if rr.Header().Get("X-Custom-Header") != "test" {
		t.Error("Custom headers should be preserved")
	}

	// Verify body content
	if rr.Body.String() != "test content" {
		t.Error("Body content should be preserved")
	}
}

func TestNewProtectorPanics(t *testing.T) {
	tests := []struct {
		name string
		cfg  ProtectorConfig
	}{
		{
			name: "missing CookieManager",
			cfg: ProtectorConfig{
				GetSessionID: func(r *http.Request) string { return "" },
			},
		},
		{
			name: "missing GetSessionID",
			cfg: ProtectorConfig{
				CookieManager: createTestCookieManager(t),
			},
		},
		{
			name: "negative TokenTTL",
			cfg: ProtectorConfig{
				CookieManager: createTestCookieManager(t),
				GetSessionID:  func(r *http.Request) string { return "" },
				TokenTTL:      -1 * time.Hour,
			},
		},
		{
			name: "invalid origin without scheme",
			cfg: ProtectorConfig{
				CookieManager:  createTestCookieManager(t),
				GetSessionID:   func(r *http.Request) string { return "" },
				AllowedOrigins: []string{"example.com"},
			},
		},
		{
			name: "invalid origin malformed",
			cfg: ProtectorConfig{
				CookieManager:  createTestCookieManager(t),
				GetSessionID:   func(r *http.Request) string { return "" },
				AllowedOrigins: []string{"not a valid url"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected panic")
					return
				}
			}()
			NewProtector(tt.cfg)
		})
	}
}

func TestSessionBinding(t *testing.T) {
	sessionID := "test-session-123"

	// Create protector that returns a specific session ID
	p := NewProtector(ProtectorConfig{
		CookieManager: createTestCookieManager(t),
		GetSessionID:  func(r *http.Request) string { return sessionID },
		TokenTTL:      1 * time.Hour,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Get token with session
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(getRR, getReq)

	cookie := extractCSRFCookie(getRR, p.cookie.Name())
	token := extractTokenFromCookie(cookie)

	// Verify token contains session ID
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookie)
	payload, err := p.cookie.Get(req)
	if err != nil {
		t.Fatalf("Failed to decode token: %v", err)
	}
	if payload.SessionID != sessionID {
		t.Errorf("Expected session ID %q in token, got %q", sessionID, payload.SessionID)
	}

	tests := []struct {
		name             string
		currentSessionID string
		wantStatus       int
	}{
		{
			name:             "matching session ID",
			currentSessionID: sessionID,
			wantStatus:       http.StatusOK,
		},
		{
			name:             "different session ID",
			currentSessionID: "different-session",
			wantStatus:       http.StatusForbidden,
		},
		{
			name:             "empty session when token has session",
			currentSessionID: "",
			wantStatus:       http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new protector with different session callback
			p2 := NewProtector(ProtectorConfig{
				CookieManager: createTestCookieManager(t),
				GetSessionID:  func(r *http.Request) string { return tt.currentSessionID },
				TokenTTL:      1 * time.Hour,
			})

			req := httptest.NewRequest("POST", "/", nil)
			req.AddCookie(cookie)
			req.Header.Set(p2.cfg.HeaderName, token)

			rr := httptest.NewRecorder()
			p2.Middleware(handler).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

func TestSessionBindingEmptyTokenWithSession(t *testing.T) {
	// Create protector that returns empty session
	p1 := NewProtector(ProtectorConfig{
		CookieManager: createTestCookieManager(t),
		GetSessionID:  func(r *http.Request) string { return "" }, // No session
		TokenTTL:      1 * time.Hour,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Get token without session
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	p1.Middleware(handler).ServeHTTP(getRR, getReq)

	cookie := extractCSRFCookie(getRR, p1.cookie.Name())
	token := extractTokenFromCookie(cookie)

	// Now create protector that has a session
	p2 := NewProtector(ProtectorConfig{
		CookieManager: createTestCookieManager(t),
		GetSessionID:  func(r *http.Request) string { return "user-123" }, // Has session
		TokenTTL:      1 * time.Hour,
	})

	// Try to use empty-session token with session-required protector
	req := httptest.NewRequest("POST", "/", nil)
	req.AddCookie(cookie)
	req.Header.Set(p2.cfg.HeaderName, token)

	rr := httptest.NewRecorder()
	p2.Middleware(handler).ServeHTTP(rr, req)

	// Should fail due to session mismatch (empty != "user-123")
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for session mismatch (empty token vs session), got %d", rr.Code)
	}
}

func TestSelfHealing(t *testing.T) {
	p := createTestProtector(t, []string{"https://example.com"})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Get a valid token first
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(getRR, getReq)

	cookie := extractCSRFCookie(getRR, p.cookie.Name())
	token := extractTokenFromCookie(cookie)

	// Create an expired token
	expiredCfg := ProtectorConfig{
		CookieManager:  createTestCookieManager(t),
		GetSessionID:   func(r *http.Request) string { return "" },
		AllowedOrigins: []string{"https://example.com"},
		TokenTTL:       1 * time.Millisecond, // Very short TTL
	}
	expiredP := NewProtector(expiredCfg)

	// Get expired token
	expiredRR := httptest.NewRecorder()
	expiredP.Middleware(handler).ServeHTTP(expiredRR, getReq)
	expiredCookie := extractCSRFCookie(expiredRR, expiredP.cookie.Name())
	expiredToken := extractTokenFromCookie(expiredCookie)
	time.Sleep(2 * time.Millisecond) // Wait for expiration

	// Create a session-mismatched token
	sessionP := NewProtector(ProtectorConfig{
		CookieManager:  createTestCookieManager(t),
		GetSessionID:   func(r *http.Request) string { return "session-A" },
		AllowedOrigins: []string{"https://example.com"},
	})
	sessionRR := httptest.NewRecorder()
	sessionP.Middleware(handler).ServeHTTP(sessionRR, getReq)
	sessionCookie := extractCSRFCookie(sessionRR, sessionP.cookie.Name())
	sessionToken := extractTokenFromCookie(sessionCookie)

	// Now use a different session for validation
	differentSessionP := NewProtector(ProtectorConfig{
		CookieManager:  createTestCookieManager(t),
		GetSessionID:   func(r *http.Request) string { return "session-B" },
		AllowedOrigins: []string{"https://example.com"},
	})

	tests := []struct {
		name           string
		cookie         *http.Cookie
		headerToken    string
		origin         string
		protector      *Protector
		shouldSelfHeal bool
		description    string
	}{
		// SELF-HEALING CASES (shouldSelfHeal = true)
		{
			name:           "missing cookie",
			cookie:         nil,
			headerToken:    "any-value", // Header present but cookie missing
			origin:         "https://example.com",
			protector:      p,
			shouldSelfHeal: true,
			description:    "Missing cookie should self-heal",
		},
		{
			name:           "corrupted cookie value",
			cookie:         &http.Cookie{Name: p.cookie.Name(), Value: "corrupted!@#$"},
			headerToken:    "corrupted!@#$", // Same corrupted value
			origin:         "https://example.com",
			protector:      p,
			shouldSelfHeal: true,
			description:    "Invalid/corrupted token should self-heal (decryption fails)",
		},
		{
			name:           "expired token",
			cookie:         expiredCookie,
			headerToken:    expiredToken,
			origin:         "https://example.com",
			protector:      p,
			shouldSelfHeal: true,
			description:    "Expired token should self-heal",
		},
		{
			name:           "session mismatch",
			cookie:         sessionCookie,
			headerToken:    sessionToken,
			origin:         "https://example.com",
			protector:      differentSessionP, // Different session context
			shouldSelfHeal: true,
			description:    "Session mismatch should self-heal",
		},

		// NON-SELF-HEALING CASES (shouldSelfHeal = false)
		{
			name:           "origin validation failure",
			cookie:         cookie,
			headerToken:    token,
			origin:         "https://evil.com", // Wrong origin
			protector:      p,
			shouldSelfHeal: false,
			description:    "Origin validation failure should NOT self-heal",
		},
		{
			name:           "missing header token",
			cookie:         cookie,
			headerToken:    "", // Missing header
			origin:         "https://example.com",
			protector:      p,
			shouldSelfHeal: false,
			description:    "Missing header token should NOT self-heal",
		},
		{
			name:           "token mismatch - different values",
			cookie:         cookie,
			headerToken:    "wrong-token", // Different from cookie value
			origin:         "https://example.com",
			protector:      p,
			shouldSelfHeal: false,
			description:    "Token mismatch (different values) should NOT self-heal",
		},
		{
			name:           "token mismatch - valid but different tokens",
			cookie:         cookie,
			headerToken:    expiredToken, // Valid format but different token
			origin:         "https://example.com",
			protector:      p,
			shouldSelfHeal: false,
			description:    "Token mismatch (valid but different) should NOT self-heal",
		},

		// EDGE CASES
		{
			name:           "empty cookie value with empty header",
			cookie:         &http.Cookie{Name: p.cookie.Name(), Value: ""},
			headerToken:    "",
			origin:         "https://example.com",
			protector:      p,
			shouldSelfHeal: false,
			description:    "Empty cookie value should NOT self-heal (tampering)",
		},
		{
			name:           "empty cookie value with valid header",
			cookie:         &http.Cookie{Name: p.cookie.Name(), Value: ""},
			headerToken:    "some-token",
			origin:         "https://example.com",
			protector:      p,
			shouldSelfHeal: false,
			description:    "Empty cookie value should NOT self-heal regardless of header",
		},
		{
			name:           "valid everything",
			cookie:         cookie,
			headerToken:    token,
			origin:         "https://example.com",
			protector:      p,
			shouldSelfHeal: false, // This should actually pass validation, no need to self-heal
			description:    "Valid request should pass (no self-heal needed)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			if tt.headerToken != "" {
				req.Header.Set(tt.protector.cfg.HeaderName, tt.headerToken)
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			rr := httptest.NewRecorder()
			tt.protector.Middleware(handler).ServeHTTP(rr, req)

			// Check if self-healing occurred
			newCookie := extractCSRFCookie(rr, tt.protector.cookie.Name())

			// Special case: valid request should return 200
			if tt.name == "valid everything" {
				if rr.Code != http.StatusOK {
					t.Errorf("%s: expected status 200 for valid request, got %d", tt.description, rr.Code)
				}
				if newCookie != nil {
					t.Errorf("%s: should not issue new cookie for valid request", tt.description)
				}
				return
			}

			// All other cases should return 403
			if rr.Code != http.StatusForbidden {
				t.Errorf("%s: expected status 403, got %d", tt.description, rr.Code)
			}

			// Check self-healing behavior
			if tt.shouldSelfHeal && newCookie == nil {
				t.Errorf("%s: expected new cookie from self-healing but got none", tt.description)
			}
			if !tt.shouldSelfHeal && newCookie != nil {
				t.Errorf("%s: unexpected new cookie, self-healing should not occur", tt.description)
			}
		})
	}
}

func TestGetSessionIDCallback(t *testing.T) {
	callCount := 0
	var capturedRequests []*http.Request

	p := NewProtector(ProtectorConfig{
		CookieManager: createTestCookieManager(t),
		GetSessionID: func(r *http.Request) string {
			callCount++
			capturedRequests = append(capturedRequests, r)
			return "session-" + r.Method
		},
		TokenTTL: 1 * time.Hour,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test 1: GET request should call GetSessionID
	callCount = 0
	capturedRequests = nil

	getReq := httptest.NewRequest("GET", "/test-path", nil)
	getRR := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(getRR, getReq)

	if callCount != 1 {
		t.Errorf("Expected GetSessionID to be called once for GET, called %d times", callCount)
	}
	if capturedRequests[0].Method != "GET" {
		t.Errorf("Expected captured request method to be GET, got %s", capturedRequests[0].Method)
	}

	cookie := extractCSRFCookie(getRR, p.cookie.Name())
	token := extractTokenFromCookie(cookie)

	// Verify token has session from GET
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookie)
	payload, _ := p.cookie.Get(req)
	if payload.SessionID != "session-GET" {
		t.Errorf("Expected session ID 'session-GET', got %q", payload.SessionID)
	}

	// Test 2: POST request should call GetSessionID
	callCount = 0
	capturedRequests = nil

	postReq := httptest.NewRequest("POST", "/another-path", nil)
	postReq.AddCookie(cookie)
	postReq.Header.Set(p.cfg.HeaderName, token)

	postRR := httptest.NewRecorder()
	p.Middleware(handler).ServeHTTP(postRR, postReq)

	if callCount < 1 {
		t.Errorf("Expected GetSessionID to be called at least once for POST")
	}
	if capturedRequests[0].Method != "POST" {
		t.Errorf("Expected captured request method to be POST, got %s", capturedRequests[0].Method)
	}

	// Should fail because session doesn't match (session-GET != session-POST)
	if postRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 due to session mismatch, got %d", postRR.Code)
	}
}

// TestLoginFlow simulates the user login process where CycleTokenWithProxy is called.
func TestLoginFlow(t *testing.T) {
	sessionID := ""
	protector := NewProtector(ProtectorConfig{
		CookieManager: createTestCookieManager(t),
		GetSessionID: func(r *http.Request) string {
			return sessionID // Session ID is controlled by the test
		},
	})

	// 1. User performs a GET, gets an anonymous token.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	protector.Middleware(handler).ServeHTTP(rr, req)

	anonymousCookie := extractCSRFCookie(rr, protector.cookie.Name())
	if anonymousCookie == nil {
		t.Fatal("Failed to get anonymous token")
	}

	// 2. User POSTs to /login with the anonymous token. This should succeed.
	loginReq := httptest.NewRequest("POST", "/login", nil)
	loginReq.AddCookie(anonymousCookie)
	loginReq.Header.Set(protector.cfg.HeaderName, anonymousCookie.Value)
	loginRR := httptest.NewRecorder()

	// The login handler that calls CycleTokenWithProxy
	loginHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// On successful login, update session and cycle token
		newSessionID := "user-123"
		sessionID = newSessionID // Simulate session creation

		rp := response.NewProxy()
		if err := protector.CycleTokenWithProxy(rp, newSessionID); err != nil {
			t.Fatalf("CycleTokenWithProxy failed: %v", err)
		}
		rp.ApplyToResponseWriter(w, r)
		w.WriteHeader(http.StatusOK)
	})

	protector.Middleware(loginHandler).ServeHTTP(loginRR, loginReq)

	if loginRR.Code != http.StatusOK {
		t.Fatalf("Login request failed: got status %d", loginRR.Code)
	}

	// 3. Get the new session-bound cookie from the login response.
	sessionCookie := extractCSRFCookie(loginRR, protector.cookie.Name())
	if sessionCookie == nil {
		t.Fatal("Did not get new session-bound token after login")
	}
	if sessionCookie.Value == anonymousCookie.Value {
		t.Fatal("Token was not cycled on login")
	}

	// 4. Try to make an authenticated POST with the OLD anonymous cookie. It must fail.
	postAuthReq := httptest.NewRequest("POST", "/settings", nil)
	postAuthReq.AddCookie(anonymousCookie) // Using old cookie
	postAuthReq.Header.Set(protector.cfg.HeaderName, anonymousCookie.Value)
	postAuthRR := httptest.NewRecorder()

	protector.Middleware(handler).ServeHTTP(postAuthRR, postAuthReq)

	if postAuthRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden when using old token after login, got %d", postAuthRR.Code)
	}

	// 5. Make an authenticated POST with the NEW session cookie. It must succeed.
	postAuthReq2 := httptest.NewRequest("POST", "/settings", nil)
	postAuthReq2.AddCookie(sessionCookie) // Using new cookie
	postAuthReq2.Header.Set(protector.cfg.HeaderName, sessionCookie.Value)
	postAuthRR2 := httptest.NewRecorder()
	protector.Middleware(handler).ServeHTTP(postAuthRR2, postAuthReq2)

	if postAuthRR2.Code != http.StatusOK {
		t.Errorf("Expected 200 OK when using new token after login, got %d", postAuthRR2.Code)
	}
}

// TestLogoutFlow simulates the user logout process.
func TestLogoutFlow(t *testing.T) {
	sessionID := "user-123" // Start as logged in
	protector := NewProtector(ProtectorConfig{
		CookieManager: createTestCookieManager(t),
		GetSessionID: func(r *http.Request) string {
			return sessionID // Session ID is controlled by the test
		},
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	// 1. Get a valid session-bound token.
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	protector.Middleware(handler).ServeHTTP(rr, req)
	sessionCookie := extractCSRFCookie(rr, protector.cookie.Name())

	// 2. Call the logout handler, which cycles the token with an empty session.
	logoutReq := httptest.NewRequest("POST", "/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutReq.Header.Set(protector.cfg.HeaderName, sessionCookie.Value)
	logoutRR := httptest.NewRecorder()

	logoutHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID = "" // Simulate logout
		rp := response.NewProxy()
		protector.CycleTokenWithProxy(rp, "") // Cycle to an anonymous token
		rp.ApplyToResponseWriter(w, r)
		w.WriteHeader(http.StatusOK)
	})

	protector.Middleware(logoutHandler).ServeHTTP(logoutRR, logoutReq)

	if logoutRR.Code != http.StatusOK {
		t.Fatalf("Logout request failed: got status %d", logoutRR.Code)
	}

	// 3. Get the new anonymous cookie from the logout response.
	newAnonymousCookie := extractCSRFCookie(logoutRR, protector.cookie.Name())
	if newAnonymousCookie == nil {
		t.Fatal("Did not get new anonymous token after logout")
	}
	if newAnonymousCookie.Value == sessionCookie.Value {
		t.Fatal("Token was not cycled on logout")
	}

	// 4. Verify the new token is anonymous.
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.AddCookie(newAnonymousCookie)
	payload, err := protector.cookie.Get(req2)
	if err != nil {
		t.Fatal("Could not decode new token")
	}
	if payload.SessionID != "" {
		t.Errorf("Expected empty session ID in token after logout, got %q", payload.SessionID)
	}
}

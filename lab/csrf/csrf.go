// Package csrf provides a robust, stateless, and layered CSRF protection middleware for Go.
// It implements the Double Submit Cookie pattern using AEAD-encrypted, HostOnly tokens,
// enhanced with defense-in-depth measures including Origin/Referer validation and session
// binding. Unlike some CSRF prevention patterns, this middleware works regardless of whether
// any user session exists, meaning it also protects pre-authentication POST-ish endpoints
// such as login and registration endpoints. Consumers must ensure that they call either
// CycleTokenWithProxy or CycleTokenWithWriter (as applicable) whenever sessions are created
// or destroyed (e.g., on login and logout).
package csrf

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/netutil"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/lab/cookies"
)

const nonceSize = 16 // Size, in bytes, of the random nonce used in the CSRF token payload.

type payload struct {
	Nonce         []byte `json:"n"`
	ExpiresAtUnix int64  `json:"e"`
	SessionID     string `json:"s,omitempty"`
}

func (p payload) isValid() bool {
	timestamp := time.Unix(p.ExpiresAtUnix, 0)
	return len(p.Nonce) > 0 && !timestamp.IsZero() && time.Now().Before(timestamp)
}

type ProtectorConfig struct {
	// REQUIRED: A configured cookie manager.
	CookieManager *cookies.Manager
	// REQUIRED: Gets the session ID for the current request. Return empty string if no session exists.
	// This enables automatic session binding validation and smart token cycling.
	GetSessionID   func(r *http.Request) string
	AllowedOrigins []string
	// Defaults to 4 hours, but this is too short for most apps. A good value is to set this to match
	// the TTL of your authentication sessions. It's also a good idea to have your app make any GET
	// request on window focus to refresh the CSRF token, to minimize failure cases for legitimate users.
	TokenTTL time.Duration
	// Do not prefix the name with "__Host-". Prefixing is handled internally.
	// Final cookie name will be "__{Host|Dev}-{CookieName}".
	// Defaults to "csrf_token".
	CookieName string
	HeaderName string // Defaults to "X-CSRF-Token"
}

type Protector struct {
	cfg                   ProtectorConfig
	isDev                 bool
	cookie                *cookies.SecureCookie[payload]
	allowedOrigins        map[string]bool
	hasOriginRestrictions bool
}

func NewProtector(cfg ProtectorConfig) *Protector {
	if cfg.CookieManager == nil {
		panic("csrf: CookieManager is required")
	}
	if cfg.GetSessionID == nil {
		panic("csrf: GetSessionID is required")
	}
	if cfg.TokenTTL < 0 {
		panic("csrf: TokenTTL must be positive")
	}
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = 4 * time.Hour
	}
	if cfg.CookieName == "" {
		cfg.CookieName = "csrf_token"
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-CSRF-Token"
	}
	isDev := cfg.CookieManager.GetIsDev()

	cookie := cookies.NewSecureCookie[payload](cookies.SecureCookieConfig{
		Manager:  cfg.CookieManager,
		Name:     cfg.CookieName,
		TTL:      cfg.TokenTTL,
		SameSite: cookies.SameSiteLaxMode,
		HttpOnly: cookies.HttpOnlyFalse,
	})

	normalized := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, origin := range cfg.AllowedOrigins {
		u, err := url.Parse(origin)
		if err != nil {
			panic(fmt.Sprintf("csrf: invalid origin %q: %v", origin, err))
		}
		if u.Scheme == "" || u.Host == "" {
			panic(fmt.Sprintf("csrf: origin must have scheme and host: %q", origin))
		}
		normalizedOrigin := strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
		normalized[normalizedOrigin] = true
	}

	return &Protector{
		cfg:                   cfg,
		isDev:                 isDev,
		cookie:                cookie,
		allowedOrigins:        normalized,
		hasOriginRestrictions: len(normalized) > 0,
	}
}

func (p *Protector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p.isDev && !netutil.IsLocalhost(r.Host) {
			panic(fmt.Sprintf(
				"DANGER: CSRF middleware is configured for development mode but the request host is not localhost: %s",
				r.Host,
			))
		}
		if p.isGETLike(r.Method) {
			rp := response.NewProxy()
			if err := p.issueCSRFTokenIfNeeded(rp, r); err != nil {
				log.Printf("csrf.Protector.Middleware: issueCSRFTokenIfNeeded failed: %v\n", err)
			}
			rp.ApplyToResponseWriter(w, r)
			next.ServeHTTP(w, r)
			return
		}
		err, shouldSelfHeal := p.applyCSRFProtection(r)
		if err != nil {
			rp := response.NewProxy()
			if shouldSelfHeal {
				if err := p.CycleTokenWithProxy(rp, p.cfg.GetSessionID(r)); err != nil {
					log.Printf("csrf.Protector.Middleware: self-heal failed: %v\n", err)
				}
			}
			rp.SetStatus(http.StatusForbidden, "Forbidden: CSRF validation failed")
			rp.ApplyToResponseWriter(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CycleTokenWithProxy generates a new CSRF token and sets it as a cookie.
// Must be called on login (with sessionID) and logout (with empty sessionID).
func (p *Protector) CycleTokenWithProxy(rp *response.Proxy, sessionID string) error {
	cookie, err := p.newCSRFCookie(sessionID)
	if err != nil {
		return fmt.Errorf("csrf: failed to generate token: %w", err)
	}
	rp.SetCookie(cookie)
	return nil
}

// CycleTokenWithWriter generates a new CSRF token and sets it as a cookie.
// Must be called on login (with sessionID) and logout (with empty sessionID).
func (p *Protector) CycleTokenWithWriter(w http.ResponseWriter, r *http.Request, sessionID string) error {
	rp := response.NewProxy()
	if err := p.CycleTokenWithProxy(rp, sessionID); err != nil {
		return err
	}
	rp.ApplyToResponseWriter(w, r)
	return nil
}

func (p *Protector) issueCSRFTokenIfNeeded(rp *response.Proxy, r *http.Request) error {
	payload, err := p.cookie.Get(r)
	if err == nil && payload.isValid() {
		currentSessionID := p.cfg.GetSessionID(r)
		if subtle.ConstantTimeCompare([]byte(payload.SessionID), []byte(currentSessionID)) == 1 {
			return nil
		}
	}
	return p.CycleTokenWithProxy(rp, p.cfg.GetSessionID(r))
}

func (p *Protector) applyCSRFProtection(r *http.Request) (err error, shouldSelfheal bool) {
	if err := p.validateOrigin(r); err != nil {
		return fmt.Errorf("origin validation failed: %w", err), false
	}
	cookie, err := r.Cookie(p.cookie.Name())
	if err != nil {
		return errors.New("csrf token cookie missing"), true
	}
	if cookie.Value == "" {
		return errors.New("csrf token cookie empty"), false
	}
	payload, err := p.cookie.Get(r)
	if err != nil {
		return fmt.Errorf("invalid csrf token: %w", err), true
	}
	if !payload.isValid() {
		return errors.New("csrf token invalid or expired"), true
	}
	submittedValue := r.Header.Get(p.cfg.HeaderName)
	if submittedValue == "" {
		return errors.New("csrf token missing from request"), false
	}
	if subtle.ConstantTimeCompare([]byte(submittedValue), []byte(cookie.Value)) != 1 {
		return errors.New("csrf token mismatch"), false
	}
	currentSessionID := p.cfg.GetSessionID(r)
	if subtle.ConstantTimeCompare([]byte(payload.SessionID), []byte(currentSessionID)) != 1 {
		return errors.New("csrf token session mismatch"), true
	}
	return nil, false
}

func (p *Protector) validateOrigin(r *http.Request) error {
	if !p.hasOriginRestrictions {
		return nil
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		return p.validateOriginHeader(origin, "Origin")
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		return p.validateOriginHeader(referer, "Referer")
	}
	return nil
}

func (p *Protector) validateOriginHeader(hdr, label string) error {
	u, err := url.Parse(hdr)
	if err != nil {
		return fmt.Errorf("malformed %s header: %w", label, err)
	}
	origin := strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
	if p.allowedOrigins[origin] {
		return nil
	}
	return fmt.Errorf("%s not allowed: %s", label, origin)
}

func (p *Protector) newCSRFCookie(sessionID string) (*http.Cookie, error) {
	nonce, err := cryptoutil.RandomBytes(nonceSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate secure random bytes: %w", err)
	}
	payload := payload{
		Nonce:         nonce,
		ExpiresAtUnix: time.Now().Add(p.cfg.TokenTTL).Unix(),
		SessionID:     sessionID,
	}
	return p.cookie.New(payload)
}

func (p *Protector) isGETLike(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

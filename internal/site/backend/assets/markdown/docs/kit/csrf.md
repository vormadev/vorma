---
title: csrf
description:
    Stateless CSRF protection using double-submit cookie pattern with session
    binding.
---

```go
import "github.com/vormadev/vorma/kit/csrf"
```

## Protector

```go
type ProtectorConfig struct {
    CookieManager  *cookies.Manager              // required
    GetSessionID   func(r *http.Request) string  // required; return "" if no session
    AllowedOrigins []string                      // optional origin/referer validation
    TokenTTL       time.Duration                 // default: 4h
    CookieName     string                        // default: "csrf_token"
    HeaderName     string                        // default: "X-CSRF-Token"
}

func NewProtector(cfg ProtectorConfig) *Protector
```

## Middleware

Automatically issues tokens on GET-like requests, validates on POST-like
requests:

```go
func (p *Protector) Middleware(next http.Handler) http.Handler
```

## Token Cycling

**Must be called on login and logout** to bind/unbind session:

```go
func (p *Protector) CycleTokenWithProxy(rp *response.Proxy, sessionID string) error
func (p *Protector) CycleTokenWithWriter(w http.ResponseWriter, r *http.Request, sessionID string) error
```

## Client Integration

Client must read cookie value and submit in header:

```javascript
fetch("/api/action", {
	method: "POST",
	headers: {
		"X-CSRF-Token": getCookie("__Host-csrf_token"),
	},
});
```

## Example

```go
var csrfProtector = csrf.NewProtector(csrf.ProtectorConfig{
    CookieManager:  cookieMgr,
    GetSessionID:   func(r *http.Request) string { return getSession(r).ID },
    AllowedOrigins: []string{"https://example.com"},
    TokenTTL:       24 * time.Hour,
})

// Apply middleware
mux.Handle("/", csrfProtector.Middleware(handler))

// On login
csrfProtector.CycleTokenWithWriter(w, r, newSessionID)

// On logout
csrfProtector.CycleTokenWithWriter(w, r, "")
```

## Security Features

- Double-submit cookie pattern (works pre-authentication)
- AEAD-encrypted tokens with `__Host-` prefix
- Session ID binding
- Origin/Referer validation (optional)
- Self-healing on recoverable errors
- Dev mode safety check (panics if non-localhost)

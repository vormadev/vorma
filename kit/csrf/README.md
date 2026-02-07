# kit/csrf

`github.com/vormadev/vorma/kit/csrf`

`csrf` is a stateless CSRF middleware using a double-submit cookie pattern with encrypted token payloads.

Defense layers:

- cookie + header token match
- optional Origin/Referer allowlist checks
- optional session binding (token bound to current session ID)

## Import

```go
import "github.com/vormadev/vorma/kit/csrf"
```

## Quick Start

```go
protector := csrf.NewProtector(csrf.ProtectorConfig{
	CookieManager: cookieMgr, // required
	GetSessionID: func(r *http.Request) string {
		s := sessionFromRequest(r)
		if s == nil {
			return ""
		}
		return s.ID
	},
	AllowedOrigins: []string{"https://app.example.com"},
	TokenTTL:       24 * time.Hour,
})

mux.Handle("/", protector.Middleware(appHandler))
```

## Required Login/Logout Step

You must cycle the CSRF token whenever auth state changes:

- login: cycle using new session ID
- logout: cycle using empty session ID

```go
if err := protector.CycleTokenWithWriter(w, r, newSessionID); err != nil {
	return err
}

if err := protector.CycleTokenWithWriter(w, r, ""); err != nil {
	return err
}
```

If this is skipped, legitimate requests can fail due to token/session mismatch.

## Request Flow

### Safe methods (`GET`, `HEAD`, `OPTIONS`, `TRACE`)

- middleware issues a CSRF cookie if missing/invalid/expired/session-mismatched
- if existing token is valid for current session, cookie is left unchanged

### Unsafe methods (`POST`, `PUT`, `PATCH`, `DELETE`, ...)

Validation requires:

- allowed origin/referer (if restrictions configured)
- CSRF cookie exists and is non-empty
- token payload decrypts/parses and is unexpired
- header token (`HeaderName`) is present
- header value equals cookie value (constant-time compare)
- token session ID equals `GetSessionID(r)` (constant-time compare)

## Failure And Self-Heal Behavior

Unsafe-method failures return `403 Forbidden`.

When possible, middleware also self-heals by issuing a fresh cookie in the same 403 response.

| Failure case | 403 | Self-heal cookie |
| --- | --- | --- |
| Origin/Referer not allowed | yes | no |
| Cookie missing | yes | yes |
| Cookie present but empty | yes | no |
| Token parse/decrypt fails | yes | yes |
| Token expired/invalid payload | yes | yes |
| Header token missing | yes | no |
| Header token mismatch | yes | no |
| Session mismatch | yes | yes |

## Origin/Referer Rules

`AllowedOrigins` is optional.

- If empty, no origin validation is performed.
- If set and `Origin` is present, `Origin` is validated.
- If `Origin` is absent and `Referer` is present, `Referer` is validated.
- If both headers are absent, this layer allows the request.

`AllowedOrigins` entries must include scheme and host (for example `https://app.example.com`).

## Configuration Defaults And Panics

`NewProtector` defaults:

- `TokenTTL = 4h` when zero
- `CookieName = "csrf_token"`
- `HeaderName = "X-CSRF-Token"`

`NewProtector` panics when:

- `CookieManager == nil`
- `GetSessionID == nil`
- `TokenTTL < 0`
- an `AllowedOrigins` entry is malformed or missing scheme/host

Dev-mode guard:

- if cookie manager is in dev mode, middleware panics when request host is not localhost/loopback.

## Token/Cookie Details

- cookie is managed via `cookies.SecureCookie[payload]`
- cookie settings are fixed to `SameSite=Lax` and `HttpOnly=false`
- cookie value itself is the token clients must echo in the configured request header

## Public API Reference

### Types

- `type ProtectorConfig struct`
- `type Protector struct`

### Exported Fields (`ProtectorConfig`)

- `CookieManager *cookies.Manager`
- `GetSessionID func(r *http.Request) string`
- `AllowedOrigins []string`
- `TokenTTL time.Duration`
- `CookieName string`
- `HeaderName string`

### Constructor

- `func NewProtector(cfg ProtectorConfig) *Protector`

### Methods

- `func (p *Protector) Middleware(next http.Handler) http.Handler`
- `func (p *Protector) CycleTokenWithProxy(rp *response.Proxy, sessionID string) error`
- `func (p *Protector) CycleTokenWithWriter(w http.ResponseWriter, r *http.Request, sessionID string) error`

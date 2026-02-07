# kit/signedcookie

`github.com/vormadev/vorma/kit/signedcookie`

Deprecated package retained for backward compatibility.

For new development, use `kit/cookies` instead: [/Users/sjc/__code/river/kit/cookies/README.md](/Users/sjc/__code/river/kit/cookies/README.md).

## When To Use This Package

Use `kit/signedcookie` only when you need to continue reading/writing existing legacy cookies that already depend on this package's format.

If you are building new cookie behavior, use `kit/cookies`.

## Import

```go
import "github.com/vormadev/vorma/kit/signedcookie"
```

## Quick Start

### 1) Create a manager from root secrets

```go
mgr, err := signedcookie.NewManager(keyset.RootSecrets{
	os.Getenv("COOKIE_SECRET_CURRENT"),
	os.Getenv("COOKIE_SECRET_PREVIOUS"),
})
if err != nil {
	return err
}
```

### 2) Configure a typed cookie

```go
type Session struct {
	UserID string
	Role   string
}

sessionCookie := &signedcookie.SignedCookie[Session]{
	Manager: mgr,
	TTL:     24 * time.Hour,
	BaseCookie: signedcookie.BaseCookie{
		Name:     "session",
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	},
	Encrypt: true,
}
```

### 3) Set, read, and delete

```go
cookie, err := sessionCookie.NewSignedCookie(Session{UserID: "u_123", Role: "admin"}, nil)
if err != nil {
	return err
}
http.SetCookie(w, cookie)

session, err := sessionCookie.VerifyAndReadCookieValue(r)
if err != nil {
	return err
}
_ = session

http.SetCookie(w, sessionCookie.NewDeletionCookie())
```

## Behavior And Constraints

- `SignedCookie.NewSignedCookie` always forces `HttpOnly=true` and `Secure=true`.
- `TTL` sets `Expires` via `time.Now().Add(TTL)` when non-zero.
- Value payloads for `SignedCookie[T]` are gob-encoded (`bytesutil.ToGob`), then base64-encoded.
- `Encrypt=true` encrypts payload bytes before signing; `Encrypt=false` signs plaintext bytes.
- Manager read-path attempts secrets in order, so the first secret is the active signer and the rest support rotation.
- Cookie payload size still must fit practical browser cookie size limits (typically around 4 KB).

## Rotation Model

- New writes are signed with `keyset.First()` (the first/active secret).
- Reads try all secrets in the keyset until verification succeeds.
- Standard rollout pattern:
1. Add new secret at index `0`, keep previous secret(s) after it.
2. Deploy and allow old cookies to age out.
3. Remove old secret(s) after rollout window.

## Migration Notes (`kit/signedcookie` -> `kit/cookies`)

Treat this as a format migration. Plan a staged rollout (for example: dual-read and rewrite on successful legacy read) rather than assuming existing cookie values can be read unchanged by a different package.

## Public API Reference

### Constants

- `const SecretSize = 32`

### Type Alias

- `type BaseCookie = http.Cookie`

### Type: `Manager`

- `func NewManager(secrets keyset.RootSecrets) (*Manager, error)`
- `func (m Manager) SignCookie(unsignedCookie *http.Cookie, encrypt bool) error`
- `func (m Manager) VerifyAndReadCookieValue(r *http.Request, key string) (string, error)`
- `func (m Manager) NewDeletionCookie(cookie http.Cookie) *http.Cookie`

`SignCookie` mutates `unsignedCookie.Value` in place.

### Type: `SignedCookie[T any]`

Exported fields:

- `Manager *Manager` (required)
- `TTL time.Duration`
- `BaseCookie BaseCookie` (`Name` is required)
- `Encrypt bool`

Methods:

- `func (sc *SignedCookie[T]) NewSignedCookie(unsignedValue T, overrideBaseCookie *BaseCookie) (*http.Cookie, error)`
- `func (sc *SignedCookie[T]) VerifyAndReadCookieValue(r *http.Request) (T, error)`
- `func (sc *SignedCookie[T]) NewDeletionCookie() *http.Cookie`

`overrideBaseCookie` replaces base attributes for that call, but cookie name still comes from `sc.BaseCookie.Name` and security flags are forced (`Secure`, `HttpOnly`).

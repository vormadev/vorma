# kit/cookies

`github.com/vormadev/vorma/kit/cookies`

`cookies` provides typed HTTP cookie primitives for two common classes of data:

- encrypted secure values (`SecureCookie*`)
- plaintext client-readable values (`ClientReadableCookie*`)

It centralizes naming, security flags, and defaults through `Manager`.

## Import

```go
import "github.com/vormadev/vorma/kit/cookies"
```

## Cookie Type Matrix

| Variant | Value format | Host prefix policy | Typical usage |
| --- | --- | --- | --- |
| `SecureCookie[T]` | Encrypted/serialized via `securestring` | `__Host-` in prod, `__Dev-` in dev | Sessions, auth state, sensitive server-only state |
| `SecureCookieNonHostOnly[T]` | Encrypted/serialized via `securestring` | No prefix | Encrypted cookies needing explicit `Domain`/custom `Path` |
| `ClientReadableCookie[T ~string]` | Plaintext string | `__Host-` in prod, `__Dev-` in dev | Theme/locale/preferences read by JS |
| `ClientReadableCookieNonHostOnly[T ~string]` | Plaintext string | No prefix | Client-readable cookies with explicit `Domain`/custom `Path` |

Host-only variants are the safest default when possible.

## Manager Setup

```go
mgr := cookies.NewManager(cookies.ManagerConfig{
	GetKeyset: appKeysetProvider, // required
	GetIsDev:  func() bool { return env == "dev" },
	// Optional defaults:
	DefaultSameSite:  cookies.SameSiteLaxMode,
	DefaultPartition: cookies.PartitionTrue,
	DefaultHttpOnly:  cookies.HttpOnlyTrue,
})
```

Zero-value manager defaults are:

- `DefaultSameSite = SameSiteLaxMode`
- `DefaultPartition = PartitionTrue`
- `DefaultHttpOnly = HttpOnlyTrue`

## Constructor Panic Conditions

These constructors panic for missing required config:

- `NewManager`: panics if `ManagerConfig.GetKeyset == nil`
- all cookie constructors: panic if `Manager == nil` or `Name == ""`

Constructors are intended for app startup wiring, where fail-fast panics are usually desirable.

## Runtime Behavior

### Production vs dev mode

- `Manager.GetIsDev() == false` (production behavior):
- cookies are `Secure=true`
- host-only constructors enforce `Path=/` and empty `Domain`
- partitioning follows config/default

- `Manager.GetIsDev() == true` (development behavior):
- cookies are `Secure=false`
- partitioning is forcibly disabled
- host-only variants use `__Dev-` prefix and do not force production host-only constraints

### Secure cookie values

`SecureCookie*` uses `kit/securestring` for `New`/`Get`:

- `New` serializes/encrypts typed data
- `Get` decrypts/parses typed data
- parse/decrypt failures return errors

### TTL / deletion

- cookie `MaxAge` is `int(TTL.Seconds())`
- `TTL == 0` produces a session cookie (`MaxAge == 0`)
- negative `TTL` yields negative `MaxAge`
- `NewDeletion()` always sets `MaxAge = -1` and empty value

## Quick Start

### Secure host-only cookie

```go
type Session struct {
	UserID string
	Role   string
}

sessionCookie := cookies.NewSecureCookie[Session](cookies.SecureCookieConfig{
	Manager: mgr,
	Name:    "session",
	TTL:     24 * time.Hour,
})

if err := sessionCookie.SetWithWriter(w, Session{UserID: "u_123", Role: "admin"}); err != nil {
	return err
}

session, err := sessionCookie.Get(r)
if err != nil {
	return err
}
_ = session

sessionCookie.DeleteWithWriter(w)
```

### Client-readable non-host cookie

```go
type Locale string

localeCookie := cookies.NewClientReadableCookieNonHostOnly[Locale](cookies.ClientReadableCookieNonHostOnlyConfig{
	Manager: mgr,
	Name:    "locale",
	Path:    "/app",
	Domain:  ".example.com",
	TTL:     365 * 24 * time.Hour,
})

localeCookie.SetWithWriter(w, Locale("en-US"))

locale, err := localeCookie.Get(r)
if err != nil {
	return err
}
_ = locale
```

## Public API Reference

### Option Types And Constants

- `type SameSite int`
- `type PartitionOption int`
- `type HttpOnlyOption int`

Constants:

- `SameSiteLaxMode`
- `SameSiteStrictMode`
- `PartitionTrue`
- `PartitionFalse`
- `HttpOnlyTrue`
- `HttpOnlyFalse`

Zero values of these option types mean "use manager default".

### Manager

Type:

- `type Manager struct`
- `type ManagerConfig struct`

`ManagerConfig` fields:

- `GetKeyset func() *keyset.Keyset`
- `GetIsDev func() bool`
- `DefaultSameSite SameSite`
- `DefaultPartition PartitionOption`
- `DefaultHttpOnly HttpOnlyOption`

Functions/methods:

- `func NewManager(cfg ManagerConfig) *Manager`
- `func (mgr *Manager) GetIsDev() bool`

### Config Types

- `type SecureCookieConfig struct`
- `type SecureCookieNonHostOnlyConfig struct`
- `type ClientReadableCookieConfig struct`
- `type ClientReadableCookieNonHostOnlyConfig struct`

`SecureCookieConfig` fields:

- `Manager *Manager`
- `Name string`
- `TTL time.Duration`
- `SameSite SameSite`
- `Partition PartitionOption`
- `HttpOnly HttpOnlyOption`

`SecureCookieNonHostOnlyConfig` fields:

- `Manager *Manager`
- `Name string`
- `TTL time.Duration`
- `SameSite SameSite`
- `Partition PartitionOption`
- `Path string`
- `Domain string`
- `HttpOnly HttpOnlyOption`

`ClientReadableCookieConfig` fields:

- `Manager *Manager`
- `Name string`
- `TTL time.Duration`
- `SameSite SameSite`
- `Partition PartitionOption`

`ClientReadableCookieNonHostOnlyConfig` fields:

- `Manager *Manager`
- `Name string`
- `TTL time.Duration`
- `SameSite SameSite`
- `Partition PartitionOption`
- `Path string`
- `Domain string`

### Cookie Types

- `type SecureCookie[T any] struct`
- `type SecureCookieNonHostOnly[T any] struct`
- `type ClientReadableCookie[T ~string] struct`
- `type ClientReadableCookieNonHostOnly[T ~string] struct`

Constructors:

- `func NewSecureCookie[T any](cfg SecureCookieConfig) *SecureCookie[T]`
- `func NewSecureCookieNonHostOnly[T any](cfg SecureCookieNonHostOnlyConfig) *SecureCookieNonHostOnly[T]`
- `func NewClientReadableCookie[T ~string](cfg ClientReadableCookieConfig) *ClientReadableCookie[T]`
- `func NewClientReadableCookieNonHostOnly[T ~string](cfg ClientReadableCookieNonHostOnlyConfig) *ClientReadableCookieNonHostOnly[T]`

Methods available on each cookie type (`SecureCookie`, `SecureCookieNonHostOnly`, `ClientReadableCookie`, `ClientReadableCookieNonHostOnly`):

- `New(...)`
- `Get(r *http.Request)`
- `NewDeletion()`
- `SetWithProxy(rp *response.Proxy, ...)`
- `SetWithWriter(w http.ResponseWriter, ...)`
- `DeleteWithProxy(rp *response.Proxy)`
- `DeleteWithWriter(w http.ResponseWriter)`
- `Name() string`

Method signatures:

- `func (c *SecureCookie[T]) New(data T) (*http.Cookie, error)`
- `func (c *SecureCookie[T]) Get(r *http.Request) (T, error)`
- `func (c *SecureCookie[T]) NewDeletion() *http.Cookie`
- `func (c *SecureCookie[T]) SetWithProxy(rp *response.Proxy, value T) error`
- `func (c *SecureCookie[T]) SetWithWriter(w http.ResponseWriter, value T) error`
- `func (c *SecureCookie[T]) DeleteWithProxy(rp *response.Proxy)`
- `func (c *SecureCookie[T]) DeleteWithWriter(w http.ResponseWriter)`
- `func (c *SecureCookie[T]) Name() string`

- `func (c *SecureCookieNonHostOnly[T]) New(data T) (*http.Cookie, error)`
- `func (c *SecureCookieNonHostOnly[T]) Get(r *http.Request) (T, error)`
- `func (c *SecureCookieNonHostOnly[T]) NewDeletion() *http.Cookie`
- `func (c *SecureCookieNonHostOnly[T]) SetWithProxy(rp *response.Proxy, value T) error`
- `func (c *SecureCookieNonHostOnly[T]) SetWithWriter(w http.ResponseWriter, value T) error`
- `func (c *SecureCookieNonHostOnly[T]) DeleteWithProxy(rp *response.Proxy)`
- `func (c *SecureCookieNonHostOnly[T]) DeleteWithWriter(w http.ResponseWriter)`
- `func (c *SecureCookieNonHostOnly[T]) Name() string`

- `func (c *ClientReadableCookie[T]) New(value T) *http.Cookie`
- `func (c *ClientReadableCookie[T]) Get(r *http.Request) (T, error)`
- `func (c *ClientReadableCookie[T]) NewDeletion() *http.Cookie`
- `func (c *ClientReadableCookie[T]) SetWithProxy(rp *response.Proxy, value T)`
- `func (c *ClientReadableCookie[T]) SetWithWriter(w http.ResponseWriter, value T)`
- `func (c *ClientReadableCookie[T]) DeleteWithProxy(rp *response.Proxy)`
- `func (c *ClientReadableCookie[T]) DeleteWithWriter(w http.ResponseWriter)`
- `func (c *ClientReadableCookie[T]) Name() string`

- `func (c *ClientReadableCookieNonHostOnly[T]) New(value T) *http.Cookie`
- `func (c *ClientReadableCookieNonHostOnly[T]) Get(r *http.Request) (T, error)`
- `func (c *ClientReadableCookieNonHostOnly[T]) NewDeletion() *http.Cookie`
- `func (c *ClientReadableCookieNonHostOnly[T]) SetWithProxy(rp *response.Proxy, value T)`
- `func (c *ClientReadableCookieNonHostOnly[T]) SetWithWriter(w http.ResponseWriter, value T)`
- `func (c *ClientReadableCookieNonHostOnly[T]) DeleteWithProxy(rp *response.Proxy)`
- `func (c *ClientReadableCookieNonHostOnly[T]) DeleteWithWriter(w http.ResponseWriter)`
- `func (c *ClientReadableCookieNonHostOnly[T]) Name() string`

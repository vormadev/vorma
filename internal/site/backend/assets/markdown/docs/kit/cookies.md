---
title: cookies
description:
    Type-safe cookie management with encryption, __Host- prefix support, and
    partitioning.
---

```go
import "github.com/vormadev/vorma/kit/cookies"
```

## Manager

```go
type ManagerConfig struct {
    GetKeyset        func() *keyset.Keyset  // required
    GetIsDev         func() bool            // optional, defaults to false
    DefaultSameSite  SameSite               // default: Lax
    DefaultPartition PartitionOption        // default: True
    DefaultHttpOnly  HttpOnlyOption         // default: True
}

func NewManager(cfg ManagerConfig) *Manager
func (mgr *Manager) GetIsDev() bool
```

## SameSite / Partition / HttpOnly Options

```go
const (
    SameSiteLaxMode, SameSiteStrictMode SameSite
    PartitionTrue, PartitionFalse       PartitionOption
    HttpOnlyTrue, HttpOnlyFalse         HttpOnlyOption
)
```

## Secure Cookies (Encrypted)

Values are encrypted via securestring. Use for sensitive data.

### Host-Only (recommended)

```go
func NewSecureCookie[T any](cfg SecureCookieConfig) *SecureCookie[T]

type SecureCookieConfig struct {
    Manager   *Manager
    Name      string         // without __Host- prefix
    TTL       time.Duration
    SameSite  SameSite       // 0 = use manager default
    Partition PartitionOption
    HttpOnly  HttpOnlyOption
}
```

### Non-Host-Only (cross-subdomain)

```go
func NewSecureCookieNonHostOnly[T any](cfg SecureCookieNonHostOnlyConfig) *SecureCookieNonHostOnly[T]
```

### Methods

```go
func (c *SecureCookie[T]) New(data T) (*http.Cookie, error)
func (c *SecureCookie[T]) Get(r *http.Request) (T, error)
func (c *SecureCookie[T]) NewDeletion() *http.Cookie
func (c *SecureCookie[T]) SetWithProxy(rp *response.Proxy, value T) error
func (c *SecureCookie[T]) SetWithWriter(w http.ResponseWriter, value T) error
func (c *SecureCookie[T]) DeleteWithProxy(rp *response.Proxy)
func (c *SecureCookie[T]) DeleteWithWriter(w http.ResponseWriter)
func (c *SecureCookie[T]) Name() string
```

## Client-Readable Cookies (Plaintext)

For string values readable by JavaScript. `T` must be `~string`.

### Host-Only

```go
func NewClientReadableCookie[T ~string](cfg ClientReadableCookieConfig) *ClientReadableCookie[T]
```

### Non-Host-Only

```go
func NewClientReadableCookieNonHostOnly[T ~string](cfg ClientReadableCookieNonHostOnlyConfig) *ClientReadableCookieNonHostOnly[T]
```

### Methods

Same as SecureCookie, but `New`/`SetWith*` don't return errors.

## Behavior

- Production: `__Host-{Name}` prefix, `Secure=true`, `Path=/`
- Dev mode: `__Dev-{Name}` prefix, `Secure=false`, partitioning disabled

## Example

```go
var mgr = cookies.NewManager(cookies.ManagerConfig{
    GetKeyset: appKeys.HKDF("cookies"),
    GetIsDev:  func() bool { return os.Getenv("ENV") == "dev" },
})

var sessionCookie = cookies.NewSecureCookie[SessionData](cookies.SecureCookieConfig{
    Manager: mgr,
    Name:    "session",
    TTL:     24 * time.Hour,
})

// Set
sessionCookie.SetWithWriter(w, sessionData)

// Get
data, err := sessionCookie.Get(r)

// Delete
sessionCookie.DeleteWithWriter(w)
```

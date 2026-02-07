# kit/middleware/secureheaders

`github.com/vormadev/vorma/kit/middleware/secureheaders`

Drop-in middleware that sets a strict baseline of security headers on every response.

## Import

```go
import "github.com/vormadev/vorma/kit/middleware/secureheaders"
```

## Quick Start

```go
wrapped := secureheaders.Middleware(appHandler)
```

## What It Does

- sets common hardening headers (COEP, COOP, CORP, HSTS, Referrer-Policy, X-Frame-Options, etc.)
- sets a restrictive `Permissions-Policy`
- removes identifying headers:
- `Server`
- `X-Powered-By`

The middleware writes headers before calling `next`, so downstream handlers can still override values if needed.

## API Coverage

### Functions

- `func Middleware(next http.Handler) http.Handler`

# kit/middleware

`github.com/vormadev/vorma/kit/middleware`

`middleware` provides a tiny shared type plus a path/method gate helper.

Most real behavior lives in subpackages (`etag`, `secureheaders`, `healthcheck`,
`robotstxt`).

## Import

```go
import "github.com/vormadev/vorma/kit/middleware"
```

## What This Package Is For

Use this package when you need:

- a common middleware type alias: `func(http.Handler) http.Handler`
- a quick way to short-circuit a request on exact path + method match

## Subpackages

- `kit/middleware/etag`: automatic ETag generation and 304 handling
- `kit/middleware/secureheaders`: baseline response hardening headers
- `kit/middleware/healthcheck`: `/healthz` or custom OK endpoint
- `kit/middleware/robotstxt`: static `robots.txt` handlers

## Quick Start

```go
health := middleware.ToHandlerMiddleware(
	"/healthz",
	[]string{http.MethodGet, http.MethodHead},
	func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	},
)

wrapped := health(appHandler)
```

Matching behavior:

- path match is exact (`r.URL.Path == endpoint`)
- method match uses exact string comparison
- on match, `handlerFunc` runs and `next` is not called
- on non-match, request passes directly to `next`

## Typical Stack Shape

```go
h := appHandler
h = secureheaders.Middleware(h)
h = etag.Auto()(h)
h = healthcheck.Healthz(h)
h = robotstxt.Disallow(h)
```

Use the `ToHandlerMiddleware` helper for endpoint-scoped short-circuit handlers;
use subpackages for policy middleware.

## Public API Reference

### Type

- `type Middleware func(http.Handler) http.Handler`

### Function

- `func ToHandlerMiddleware(endpoint string, methods []string, handlerFunc http.HandlerFunc) Middleware`

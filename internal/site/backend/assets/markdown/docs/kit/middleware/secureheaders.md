---
title: secureheaders
description: Middleware that sets OWASP-recommended security headers.
---

```go
import "github.com/vormadev/vorma/kit/middleware/secureheaders"
```

## Middleware

```go
func Middleware(next http.Handler) http.Handler
```

## Headers Set

| Header                            | Value                               |
| --------------------------------- | ----------------------------------- |
| Cross-Origin-Embedder-Policy      | require-corp                        |
| Cross-Origin-Opener-Policy        | same-origin                         |
| Cross-Origin-Resource-Policy      | same-origin                         |
| Permissions-Policy                | (restrictive; denies most features) |
| Referrer-Policy                   | no-referrer                         |
| Strict-Transport-Security         | max-age=31536000; includeSubDomains |
| X-Content-Type-Options            | nosniff                             |
| X-Frame-Options                   | deny                                |
| X-Permitted-Cross-Domain-Policies | none                                |

Also removes: `Server`, `X-Powered-By`

## Example

```go
handler := secureheaders.Middleware(mainHandler)
```

Note: Headers are set before calling next handler, so downstream can override if
needed.

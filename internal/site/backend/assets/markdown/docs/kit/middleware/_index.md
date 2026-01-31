---
title: middleware
description: HTTP middleware type and utilities.
---

```go
import "github.com/vormadev/vorma/kit/middleware"
```

## Type

```go
type Middleware func(http.Handler) http.Handler
```

## Functions

Convert a handler to middleware that intercepts specific endpoint/methods:

```go
func ToHandlerMiddleware(endpoint string, methods []string, handlerFunc http.HandlerFunc) Middleware
```

Example:

```go
healthCheck := middleware.ToHandlerMiddleware(
    "/health",
    []string{"GET"},
    func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("OK"))
    },
)

handler := healthCheck(mainHandler)
```

Note: Path matching is case-insensitive.

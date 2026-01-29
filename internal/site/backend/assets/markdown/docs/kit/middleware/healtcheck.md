---
title: healthcheck
description: Health check endpoint middleware returning 200 OK.
---

```go
import "github.com/vormadev/vorma/kit/middleware/healthcheck"
```

## Variables

Pre-configured middleware for `/healthz`:

```go
var Healthz middleware.Middleware
```

## Functions

Create health check middleware for custom endpoint:

```go
func OK(endpoint string) middleware.Middleware
```

## Behavior

- Responds to GET and HEAD requests
- Returns 200 with body "OK"
- Other methods/paths pass through to next handler

## Example

```go
handler := healthcheck.Healthz(mainHandler)

// Or custom endpoint
handler := healthcheck.OK("/health")(mainHandler)
```

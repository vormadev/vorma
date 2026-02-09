# kit/middleware/healthcheck

`github.com/vormadev/vorma/kit/middleware/healthcheck`

Minimal healthcheck middleware that responds with `200 OK` and body `"OK"` for
`GET`/`HEAD`.

## Import

```go
import "github.com/vormadev/vorma/kit/middleware/healthcheck"
```

## Quick Start

Default endpoint:

```go
wrapped := healthcheck.Healthz(appHandler) // serves /healthz
```

Custom endpoint:

```go
wrapped := healthcheck.OK("/readyz")(appHandler)
```

Behavior:

- matches exact path
- only handles `GET` and `HEAD`
- all other requests continue to `next`

## API Coverage

### Variables

- `var Healthz`

### Functions

- `func OK(endpoint string) middleware.Middleware`

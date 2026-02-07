# kit/middleware/etag

`github.com/vormadev/vorma/kit/middleware/etag`

Automatic ETag middleware for HTTP handlers.

It handles:

- generating ETags for cacheable `GET`/`HEAD` responses
- returning `304 Not Modified` when `If-None-Match` matches
- skipping unsafe/unsuitable responses conservatively

## Import

```go
import "github.com/vormadev/vorma/kit/middleware/etag"
```

## Quick Start

```go
wrapped := etag.Auto()(handler)
```

With config:

```go
wrapped := etag.Auto(&etag.Config{
	Strong:      false,
	MaxBodySize: 8 * 1024 * 1024,
	SkipFunc: func(r *http.Request) bool {
		return strings.HasPrefix(r.URL.Path, "/api/stream")
	},
})(handler)
```

## Behavior Notes

- hashes buffered response body (default hash function: SHA-1)
- defaults to weak ETags (`W/"..."`)
- if response has header `X-Vorma-Build-Id`, that value is included in ETag hash
- only emits ETags for `200 OK` responses with non-empty body
- skips ETag when:
- request method is not `GET` or `HEAD`
- response includes `Cache-Control: no-store`
- response includes `Set-Cookie`
- response body exceeds `MaxBodySize` (falls back to passthrough write)

## API Coverage

### Types

- `type Config`

### Exported Struct Fields

- `Config.Hash func() hash.Hash`
- `Config.MaxBodySize int64`
- `Config.SkipFunc func(r *http.Request) bool`
- `Config.Strong bool`

### Functions

- `func Auto(config ...*Config) func(http.Handler) http.Handler`

---
title: etag
description: Automatic ETag middleware with If-None-Match/304 support.
---

```go
import "github.com/vormadev/vorma/kit/middleware/etag"
```

## Middleware

Automatic ETag generation and 304 responses for GET/HEAD requests:

```go
func Auto(config ...*Config) func(http.Handler) http.Handler
```

## Config

```go
type Config struct {
    Strong      bool              // default: false (weak ETags)
    Hash        func() hash.Hash  // default: sha1.New
    MaxBodySize int64             // default: 8MB; responses larger bypass ETag
    SkipFunc    func(r *http.Request) bool
}
```

## Behavior

- Buffers response body and computes hash
- Sets `ETag` header on successful responses
- Returns 304 Not Modified when `If-None-Match` matches
- Skips ETag for: non-2xx status, `Cache-Control: no-store`, `Set-Cookie`
  present, empty body
- Falls back to streaming (no ETag) when body exceeds `MaxBodySize`
- Includes `X-Vorma-Build-Id` header in hash if present

## Example

```go
mux.Handle("/", etag.Auto()(handler))

// With config
mux.Handle("/", etag.Auto(&etag.Config{
    Strong:      true,
    MaxBodySize: 16 * 1024 * 1024,
    SkipFunc:    func(r *http.Request) bool {
        return strings.HasPrefix(r.URL.Path, "/api/")
    },
})(handler))
```

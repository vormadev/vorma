---
title: robotstxt
description: Middleware for serving robots.txt responses.
---

```go
import "github.com/vormadev/vorma/kit/middleware/robotstxt"
```

## Variables

Pre-configured middlewares:

```go
var Allow middleware.Middleware    // "User-agent: *\nAllow: /"
var Disallow middleware.Middleware // "User-agent: *\nDisallow: /"
```

## Functions

Create middleware with custom robots.txt content:

```go
func Content(content string) middleware.Middleware
```

## Behavior

- Responds to GET and HEAD at `/robots.txt`
- Returns text/plain response
- Other methods/paths pass through to next handler

## Example

```go
handler := robotstxt.Allow(mainHandler)

// Or custom content
handler := robotstxt.Content(`User-agent: *
Allow: /
Disallow: /admin/
Sitemap: https://example.com/sitemap.xml`)(mainHandler)
```

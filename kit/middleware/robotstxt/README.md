# kit/middleware/robotstxt

`github.com/vormadev/vorma/kit/middleware/robotstxt`

Middleware for serving `robots.txt` from your app.

## Import

```go
import "github.com/vormadev/vorma/kit/middleware/robotstxt"
```

## Quick Start

Allow all crawlers:

```go
wrapped := robotstxt.Allow(appHandler)
```

Disallow all crawlers:

```go
wrapped := robotstxt.Disallow(appHandler)
```

Custom content:

```go
wrapped := robotstxt.Content(`User-agent: *
Allow: /
Disallow: /admin/
Sitemap: https://example.com/sitemap.xml`)(appHandler)
```

Behavior:

- handles `GET` and `HEAD` on `/robots.txt`
- writes plain-text robots content
- otherwise passes request to `next`

## API Coverage

### Variables

- `var Allow`
- `var Disallow`

### Functions

- `func Content(content string) middleware.Middleware`

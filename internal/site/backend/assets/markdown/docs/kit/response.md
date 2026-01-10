---
title: response
description:
    HTTP response helpers and proxy for deferred/parallel response building.
---

```go
import "github.com/vormadev/vorma/kit/response"
```

## Response (Direct Writer)

Wrapper around `http.ResponseWriter` with commit tracking:

```go
func New(w http.ResponseWriter) Response
func (res *Response) IsCommitted() bool
```

### Headers & Status

```go
func (res *Response) SetHeader(key, value string)
func (res *Response) AddHeader(key, value string)
func (res *Response) SetStatus(status int)
func (res *Response) Error(status int, reasons ...string)
```

### Content Responses

```go
func (res *Response) JSON(v any)
func (res *Response) JSONBytes(bytes []byte)
func (res *Response) Text(text string)
func (res *Response) HTML(html string)
func (res *Response) HTMLBytes(bytes []byte)
func (res *Response) OK()      // 200 {"ok":true}
func (res *Response) OKText()  // 200 "OK"
```

### Error Responses

```go
func (res *Response) BadRequest(reasons ...string)
func (res *Response) Unauthorized(reasons ...string)
func (res *Response) Forbidden(reasons ...string)
func (res *Response) NotFound()
func (res *Response) MethodNotAllowed(reasons ...string)
func (res *Response) TooManyRequests(reasons ...string)
func (res *Response) InternalServerError(reasons ...string)
func (res *Response) NotModified()
```

### Redirects

```go
func (res *Response) Redirect(r *http.Request, url string, code ...int) (usedClientRedirect bool, err error)
func (res *Response) ServerRedirect(r *http.Request, url string, code ...int)
func (res *Response) ClientRedirect(url string) error
func GetClientRedirectURL(w http.ResponseWriter) string
```

## Proxy (Deferred Response)

For parallel handlers or deferred response building. Not thread-safe—use one per
goroutine, then merge.

```go
func NewProxy() *Proxy
```

### Methods

```go
func (p *Proxy) SetStatus(status int, errorStatusText ...string)
func (p *Proxy) GetStatus() (int, string)
func (p *Proxy) SetHeader(key, value string)
func (p *Proxy) AddHeader(key, value string)
func (p *Proxy) GetHeader(key string) string
func (p *Proxy) GetHeaders(key string) []string
func (p *Proxy) SetCookie(cookie *http.Cookie)
func (p *Proxy) GetCookies() []*http.Cookie
func (p *Proxy) AddHeadEls(els *headels.HeadEls)
func (p *Proxy) GetHeadEls() *headels.HeadEls
func (p *Proxy) Redirect(r *http.Request, url string, code ...int) (bool, error)
func (p *Proxy) GetLocation() string
func (p *Proxy) IsError() bool
func (p *Proxy) IsRedirect() bool
func (p *Proxy) IsSuccess() bool
func (p *Proxy) ApplyToResponseWriter(w http.ResponseWriter, r *http.Request)
```

### Merging

```go
func MergeProxyResponses(proxies ...*Proxy) *Proxy
```

Merge behavior: first error wins, last success wins, first redirect wins,
headers/cookies merged in order.

## Constants

```go
const ClientRedirectHeader = "X-Client-Redirect"
const ClientAcceptsRedirectHeader = "X-Accepts-Client-Redirect"
```

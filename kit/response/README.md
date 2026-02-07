# kit/response

`github.com/vormadev/vorma/kit/response`

Helpers for writing HTTP responses directly (`Response`) or building/merging deferred responses (`Proxy`) before applying to a real `http.ResponseWriter`.

## Import

```go
import "github.com/vormadev/vorma/kit/response"
```

## Choose The Right Type

- Use `Response` in normal handlers that already own `http.ResponseWriter`.
- Use `Proxy` when multiple tasks/goroutines independently produce response decisions and you need deterministic merge rules.

## `Response` Quick Start

```go
func handle(w http.ResponseWriter, r *http.Request) {
	res := response.New(w)

	if r.Method != http.MethodPost {
		res.MethodNotAllowed("POST only")
		return
	}

	res.JSON(map[string]any{"ok": true})
}
```

## `Proxy` Quick Start

```go
func handle(w http.ResponseWriter, r *http.Request) {
	a := response.NewProxy()
	a.SetStatus(http.StatusOK)
	a.AddHeader("X-Trace", "a")

	b := response.NewProxy()
	b.SetCookie(&http.Cookie{Name: "session", Value: "v2"})

	merged := response.MergeProxyResponses(a, b)
	merged.ApplyToResponseWriter(w, r)
}
```

## Redirect Behavior

Two redirect modes are supported:

- Server redirect: normal HTTP `3xx + Location`.
- Client redirect: sets `X-Client-Redirect` for JS/fetch clients that intentionally handle navigation themselves.

Opt in to client redirects by sending:

- `X-Accepts-Client-Redirect: true`

Exported constants:

- `ClientRedirectHeader` (`X-Client-Redirect`)
- `ClientAcceptsRedirectHeader` (`X-Accepts-Client-Redirect`)

URL validation rules:

- Empty URLs are rejected.
- Relative URLs are allowed.
- Absolute URLs must use `http` or `https`.

Redirect code rules:

- If no code is provided, default is `303 See Other`.
- If a provided code is outside `300..399`, it is normalized to `303`.

## Proxy Merge Rules (`MergeProxyResponses`)

When multiple proxies are merged:

- Status: first error (`>=400`) wins; otherwise last success (`2xx`) wins.
- Redirect: if merged status is not an error, first redirect wins.
- Headers: header operations are replayed in order (`Set` replaces prior values for that key, `Add` appends).
- Cookies: later cookies with the same name replace earlier ones.
- Head elements: merged in input order.

## Important Notes

- `Proxy` is not thread-safe; do not share one proxy across goroutines.
- `Response.IsCommitted()` tracks commits done through `Response` methods. If code writes directly via `Response.Writer`, commit tracking can diverge from actual writer state.
- `Response` content helpers (`JSON`, `Text`, `HTML`, etc.) do not return write errors.
- `MergeProxyResponses` ignores `nil` proxies; `SetCookie(nil)` is treated as a no-op.

## API Reference

### Types

- `type Response`
- `type Proxy`

### `Response` field

- `Response.Writer http.ResponseWriter`

### Constructors and top-level functions

- `func New(w http.ResponseWriter) Response`
- `func NewProxy() *Proxy`
- `func MergeProxyResponses(proxies ...*Proxy) *Proxy`
- `func GetClientRedirectURL(w http.ResponseWriter) string`

### `Response` methods

- `func (res *Response) IsCommitted() bool`
- `func (res *Response) SetHeader(key, value string)`
- `func (res *Response) AddHeader(key, value string)`
- `func (res *Response) SetStatus(status int)`
- `func (res *Response) Error(status int, reasons ...string)`
- `func (res *Response) JSONBytes(bytes []byte)`
- `func (res *Response) JSON(v any)`
- `func (res *Response) OK()`
- `func (res *Response) Text(text string)`
- `func (res *Response) OKText()`
- `func (res *Response) HTMLBytes(bytes []byte)`
- `func (res *Response) HTML(html string)`
- `func (res *Response) NotModified()`
- `func (res *Response) NotFound()`
- `func (res *Response) Unauthorized(reasons ...string)`
- `func (res *Response) InternalServerError(reasons ...string)`
- `func (res *Response) BadRequest(reasons ...string)`
- `func (res *Response) TooManyRequests(reasons ...string)`
- `func (res *Response) Forbidden(reasons ...string)`
- `func (res *Response) MethodNotAllowed(reasons ...string)`
- `func (res *Response) Redirect(r *http.Request, url string, code ...int) (usedClientRedirect bool, err error)`
- `func (res *Response) ServerRedirect(r *http.Request, url string, code ...int)`
- `func (res *Response) ClientRedirect(url string) error`

### `Proxy` methods

- `func (p *Proxy) SetStatus(status int, errorStatusText ...string)`
- `func (p *Proxy) GetStatus() (int, string)`
- `func (p *Proxy) SetHeader(key, value string)`
- `func (p *Proxy) AddHeader(key, value string)`
- `func (p *Proxy) GetHeader(key string) string`
- `func (p *Proxy) GetHeaders(key string) []string`
- `func (p *Proxy) SetCookie(cookie *http.Cookie)`
- `func (p *Proxy) GetCookies() []*http.Cookie`
- `func (p *Proxy) AddHeadEls(els *headels.HeadEls)`
- `func (p *Proxy) GetHeadEls() *headels.HeadEls`
- `func (p *Proxy) Redirect(r *http.Request, url string, code ...int) (bool, error)`
- `func (p *Proxy) GetLocation() string`
- `func (p *Proxy) IsError() bool`
- `func (p *Proxy) IsRedirect() bool`
- `func (p *Proxy) IsSuccess() bool`
- `func (p *Proxy) ApplyToResponseWriter(w http.ResponseWriter, r *http.Request)`

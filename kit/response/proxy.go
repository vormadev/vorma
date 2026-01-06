package response

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/vormadev/vorma/kit/headels"
)

// For usage in JSON API handlers that may run in parallel or
// do not have direct access to the http.ResponseWriter.
// Proxy instances are not meant to be shared. Rather, they
// should exist inside a single function/handler scope, and
// afterwards should be used by a parent scope to actually
// de-duplicate, determine priority, and write to the real
// http.ResponseWriter.
type headerOp struct {
	op    string
	value string
}

// Do not instantiate directly. Use NewProxy().
type Proxy struct {
	_status      int
	_status_text string
	_headerOps   map[string][]headerOp
	_cookies     []*http.Cookie
	_head_els    *headels.HeadEls
	_location    string
}

func NewProxy() *Proxy {
	return &Proxy{_headerOps: make(map[string][]headerOp)}
}

/////// STATUS (use directly for both success and error responses)

func (p *Proxy) SetStatus(status int, errorStatusText ...string) {
	p._status = status
	if len(errorStatusText) != 0 {
		p._status_text = errorStatusText[0]
	}
}

func (p *Proxy) GetStatus() (int, string) {
	return p._status, p._status_text
}

/////// HEADERS

func (p *Proxy) SetHeader(key, value string) {
	p._headerOps[key] = append(
		p._headerOps[key],
		headerOp{op: "set", value: value},
	)
}

func (p *Proxy) AddHeader(key, value string) {
	p._headerOps[key] = append(
		p._headerOps[key],
		headerOp{op: "add", value: value},
	)
}

func (p *Proxy) GetHeader(key string) string {
	values := p.computeHeaderValues(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (p *Proxy) GetHeaders(key string) []string {
	return p.computeHeaderValues(key)
}

func (p *Proxy) computeHeaderValues(key string) []string {
	ops := p._headerOps[key]
	if len(ops) == 0 {
		return nil
	}
	var values []string
	for _, op := range ops {
		if op.op == "set" {
			values = []string{op.value}
		} else {
			values = append(values, op.value)
		}
	}
	return values
}

/////// COOKIES

func (p *Proxy) SetCookie(cookie *http.Cookie) {
	p._cookies = append(p._cookies, cookie)
}

func (p *Proxy) GetCookies() []*http.Cookie {
	return p._cookies
}

/////// HEAD ELEMENTS

func (p *Proxy) AddHeadEls(els *headels.HeadEls) {
	if p._head_els == nil {
		p._head_els = headels.New()
	}
	p._head_els.AddElements(els)
}

func (p *Proxy) GetHeadEls() *headels.HeadEls {
	if p._head_els == nil {
		p._head_els = headels.New()
	}
	return p._head_els
}

/////// REDIRECTS

// bool return value indicates whether the redirect upgraded to a client redirect
func (p *Proxy) Redirect(r *http.Request, url string, code ...int) bool {
	if doesAcceptClientRedirect(r) {
		p.clientRedirect(url)
		return true
	}
	p.serverRedirect(url, resolveSpreadCode(code))
	return false
}

func (p *Proxy) serverRedirect(url string, code ...int) {
	// Don't override error statuses with redirects
	if p.IsError() {
		return
	}
	p._status = resolveSpreadCode(code)
	p._location = url
}

// Use this when you have a client that is initiating requests
// using window.fetch or similar and you want to redirect to
// an external URL. This is a workaround to inherent browser
// limitations with cross-origin redirects in response to a
// an ajax request. For this to actually do anything, you
// need to have a cooperative client that looks for
// X-Client-Redirect headers and manually handles redirects
// using `window.location.href = headerValue` or similar.
// Sets status to 200 if not already set.
func (p *Proxy) clientRedirect(url string) error {
	if ok := validateURL(url); !ok {
		return fmt.Errorf("invalid URL: %s", url)
	}
	currentStatus := p._status
	if currentStatus == 0 {
		p.SetStatus(http.StatusOK)
	}
	p.SetHeader(ClientRedirectHeader, url)
	return nil
}

func (p *Proxy) GetLocation() string {
	return p._location
}

/////// HELPERS

func isError(status int) bool {
	return status >= 400
}

func isServerRedirect(status int) bool {
	return status >= 300 && status < 400
}

func isSuccess(status int) bool {
	return status >= 200 && status < 300
}

func (p *Proxy) IsError() bool {
	return isError(p._status)
}

func (p *Proxy) IsRedirect() bool {
	return p.isServerRedirect() || p.isClientRedirect()
}

func (p *Proxy) isServerRedirect() bool {
	return isServerRedirect(p._status) && p._location != ""
}

func (p *Proxy) isClientRedirect() bool {
	return p.GetHeader(ClientRedirectHeader) != ""
}

func (p *Proxy) IsSuccess() bool {
	return isSuccess(p._status)
}

func (p *Proxy) ApplyToResponseWriter(w http.ResponseWriter, r *http.Request) {
	// Headers
	for key, ops := range p._headerOps {
		currentValues := []string{}
		for _, op := range ops {
			if op.op == "set" {
				w.Header().Del(key)
				currentValues = []string{op.value}
			} else {
				currentValues = append(currentValues, op.value)
			}
		}
		for _, v := range currentValues {
			w.Header().Add(key, v)
		}
	}

	// Cookies
	for _, c := range p._cookies {
		http.SetCookie(w, c)
	}

	// Redirect (only if not an error status)
	if p.isServerRedirect() && !p.IsError() {
		http.Redirect(w, r, p._location, p._status)
		return
	}

	// Status
	if p._status != 0 {
		if isError(p._status) {
			if p._status_text != "" {
				http.Error(w, p._status_text, p._status)
			} else {
				http.Error(w, http.StatusText(p._status), p._status)
			}
		} else {
			w.WriteHeader(p._status)
		}
	}
}

type cookieWithIdx struct {
	idx    int
	cookie *http.Cookie
}

// Consumers should deduplicate head els after calling MergeProxyResponses
// by using headels.ToHeadEls(proxy.GetHeadElements())
func MergeProxyResponses(proxies ...*Proxy) *Proxy {
	merged := NewProxy()

	// Head Elements -- MERGED IN ORDER
	merged._head_els = headels.New()
	for _, p := range proxies {
		if p._head_els != nil {
			merged._head_els.AddElements(p._head_els)
		}
	}

	// Headers -- MERGED IN ORDER
	merged._headerOps = make(map[string][]headerOp)
	for _, p := range proxies {
		for key, ops := range p._headerOps {
			merged._headerOps[key] = append(merged._headerOps[key], ops...)
		}
	}

	// Cookies -- MERGED IN ORDER (later cookies overwrite earlier ones with same name)
	_unique_cookies_map := make(map[string]*cookieWithIdx)
	for i, p := range proxies {
		for _, c := range p._cookies {
			_unique_cookies_map[c.Name] = &cookieWithIdx{i, c}
		}
	}

	deduped := make([]*cookieWithIdx, 0, len(_unique_cookies_map))
	for _, c := range _unique_cookies_map {
		deduped = append(deduped, c)
	}
	slices.SortStableFunc(deduped, func(i, j *cookieWithIdx) int {
		return i.idx - j.idx
	})

	merged._cookies = make([]*http.Cookie, 0, len(deduped))
	for _, c := range deduped {
		merged._cookies = append(merged._cookies, c.cookie)
	}

	// Status
	// Either FIRST ERROR or LAST SUCCESS will win
	for _, p := range proxies {
		if p._status >= 400 { // Error status codes
			merged._status = p._status
			merged._status_text = p._status_text
			break // Take the first error we find
		} else if merged._status < 300 { // Only overwrite success codes
			merged._status = p._status
			merged._status_text = p._status_text
		}
	}

	// Redirect -- Assuming no error, FIRST REDIRECT WINS
	if !isError(merged._status) {
		for _, p := range proxies {
			if p.IsRedirect() {
				merged._status = p._status
				merged._location = p._location
				if p.isClientRedirect() {
					merged.SetHeader(
						ClientRedirectHeader,
						p.GetHeader(ClientRedirectHeader),
					)
				}
				break
			}
		}
	}

	return merged
}

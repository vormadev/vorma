// Package response provides thin, explicit HTTP response helpers.
//
// The `Response` type wraps `http.ResponseWriter` and centralizes common
// response patterns (JSON, text, HTML, redirects, status/error helpers) while
// still exposing straightforward HTTP semantics.
//
// Package response intentionally avoids hidden policy and keeps behavior
// predictable so higher-level runtime layers can compose it safely.
package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/vormadev/vorma/kit/headels"
)

// Response is a small convenience wrapper for writing HTTP responses.
type Response struct {
	Writer      http.ResponseWriter
	isCommitted bool
}

// New creates a Response helper around w.
func New(w http.ResponseWriter) Response {
	return Response{Writer: w}
}

// IsCommitted reports whether this helper has already written a status or body.
func (res *Response) IsCommitted() bool {
	return res.isCommitted
}

/////////////////////////////////////////////////////////////////////
// General helpers
/////////////////////////////////////////////////////////////////////

// SetHeader sets a response header without committing the response.
func (res *Response) SetHeader(key, value string) {
	res.Writer.Header().Set(key, value)
	// should not commit here
}

// AddHeader appends a response header value without committing the response.
func (res *Response) AddHeader(key, value string) {
	res.Writer.Header().Add(key, value)
	// should not commit here
}

// SetStatus writes the status code and marks the response committed.
func (res *Response) SetStatus(status int) {
	res.Writer.WriteHeader(status)
	res.flagAsCommitted()
}

// Error writes an HTTP error with optional custom reason text.
func (res *Response) Error(status int, reasons ...string) {
	reason := strings.Join(reasons, " ")
	if reason == "" {
		reason = http.StatusText(status)
	}
	http.Error(res.Writer, reason, status)
	res.flagAsCommitted()
}

/////////////////////////////////////////////////////////////////////
// Contentful responses
/////////////////////////////////////////////////////////////////////

// JSONBytes writes pre-encoded JSON bytes.
func (res *Response) JSONBytes(bytes []byte) {
	res.SetHeader("Content-Type", "application/json")
	res.Writer.Write(bytes)
	res.flagAsCommitted()
}

// JSON marshals v as JSON and writes it to the response body.
func (res *Response) JSON(v any) {
	res.SetHeader("Content-Type", "application/json")
	json.NewEncoder(res.Writer).Encode(v)
	res.flagAsCommitted()
}

// OK writes a 200 response with {"ok":true}.
func (res *Response) OK() {
	res.SetHeader("Content-Type", "application/json")
	res.Writer.WriteHeader(http.StatusOK)
	res.Writer.Write([]byte(`{"ok":true}`))
	res.flagAsCommitted()
}

// Text writes plain text to the response body.
func (res *Response) Text(text string) {
	res.SetHeader("Content-Type", "text/plain")
	res.Writer.Write([]byte(text))
	res.flagAsCommitted()
}

// OKText writes a 200 response with "OK".
func (res *Response) OKText() {
	res.SetHeader("Content-Type", "text/plain")
	res.Writer.WriteHeader(http.StatusOK)
	res.Writer.Write([]byte("OK"))
	res.flagAsCommitted()
}

// HTMLBytes writes HTML bytes to the response body.
func (res *Response) HTMLBytes(bytes []byte) {
	res.SetHeader("Content-Type", "text/html")
	res.Writer.Write(bytes)
	res.flagAsCommitted()
}

// HTML writes an HTML string to the response body.
func (res *Response) HTML(html string) {
	res.HTMLBytes([]byte(html))
}

/////////////////////////////////////////////////////////////////////
// HTTP status responses
/////////////////////////////////////////////////////////////////////

// NotModified writes a 304 status.
func (res *Response) NotModified() {
	res.SetStatus(http.StatusNotModified)
}

// NotFound writes a 404 status.
func (res *Response) NotFound() {
	res.SetStatus(http.StatusNotFound)
}

/////////////////////////////////////////////////////////////////////
// Error responses
/////////////////////////////////////////////////////////////////////

// Unauthorized writes a 401 response.
func (res *Response) Unauthorized(reasons ...string) {
	res.Error(http.StatusUnauthorized, reasons...)
}

// InternalServerError writes a 500 response.
func (res *Response) InternalServerError(reasons ...string) {
	res.Error(http.StatusInternalServerError, reasons...)
}

// BadRequest writes a 400 response.
func (res *Response) BadRequest(reasons ...string) {
	res.Error(http.StatusBadRequest, reasons...)
}

// TooManyRequests writes a 429 response.
func (res *Response) TooManyRequests(reasons ...string) {
	res.Error(http.StatusTooManyRequests, reasons...)
}

// Forbidden writes a 403 response.
func (res *Response) Forbidden(reasons ...string) {
	res.Error(http.StatusForbidden, reasons...)
}

// MethodNotAllowed writes a 405 response.
func (res *Response) MethodNotAllowed(reasons ...string) {
	res.Error(http.StatusMethodNotAllowed, reasons...)
}

/////////////////////////////////////////////////////////////////////
// Redirects
/////////////////////////////////////////////////////////////////////

const (
	// ClientRedirectHeader carries a client-consumed redirect target URL.
	ClientRedirectHeader = "X-Client-Redirect"
	// ClientAcceptsRedirectHeader opts-in to client redirect responses.
	ClientAcceptsRedirectHeader = "X-Accepts-Client-Redirect"
)

func resolveSpreadCode(code []int) int {
	codeToUse := http.StatusSeeOther
	if len(code) > 0 {
		codeToUse = code[0]
	}
	if codeToUse < 300 || codeToUse > 399 {
		codeToUse = http.StatusSeeOther
	}
	return codeToUse
}

// Redirect chooses client redirect headers or server redirects based on request headers.
func (res *Response) Redirect(
	r *http.Request,
	url string,
	code ...int,
) (usedClientRedirect bool, err error) {
	if doesAcceptClientRedirect(r) {
		if err := res.ClientRedirect(url); err != nil {
			return false, err
		}
		return true, nil
	}
	res.ServerRedirect(r, url, resolveSpreadCode(code))
	return false, nil
}

// ServerRedirect writes an HTTP redirect response.
func (res *Response) ServerRedirect(r *http.Request, url string, code ...int) {
	codeToUse := resolveSpreadCode(code)
	if r == nil {
		res.SetHeader("Location", url)
		res.SetStatus(codeToUse)
		return
	}
	http.Redirect(res.Writer, r, url, codeToUse)
	res.flagAsCommitted()
}

// ClientRedirect sets a client-side redirect header and status 200.
// Returns an error if the response is already committed.
func (res *Response) ClientRedirect(url string) error {
	if ok := validateURL(url); !ok {
		return fmt.Errorf("invalid URL: %s", url)
	}
	if res.isCommitted {
		return fmt.Errorf(
			"cannot set client redirect: response already committed",
		)
	}
	res.SetHeader(ClientRedirectHeader, url)
	res.SetStatus(http.StatusOK)
	return nil
}

// GetClientRedirectURL reads the client redirect target header from w.
func GetClientRedirectURL(w http.ResponseWriter) string {
	return w.Header().Get(ClientRedirectHeader)
}

func doesAcceptClientRedirect(r *http.Request) bool {
	if r == nil {
		return false
	}
	yes, err := strconv.ParseBool(r.Header.Get(ClientAcceptsRedirectHeader))
	return err == nil && yes
}

func validateURL(location string) (ok bool) {
	if location == "" {
		return false
	}
	url, err := url.Parse(location)
	if err != nil {
		return false
	}
	if url.IsAbs() && (url.Scheme != "http" && url.Scheme != "https") {
		return false
	}
	return true
}

/////////////////////////////////////////////////////////////////////
// Internal
/////////////////////////////////////////////////////////////////////

func (res *Response) flagAsCommitted() {
	res.isCommitted = true
}

/////////////////////////////////////////////////////////////////////
// Proxy Responses
/////////////////////////////////////////////////////////////////////

// For usage in JSON API handlers that may run in parallel or
// do not have direct access to the http.ResponseWriter.
// Proxy instances are not meant to be shared. Rather, they
// should exist inside a single function/handler scope, and
// afterwards should be used by a parent scope to actually
// de-duplicate, determine priority, and write to the real
// http.ResponseWriter.
//
// Concurrency model: Proxy instances are NOT thread-safe and must not be
// shared across goroutines. The intended usage pattern is:
//   - Create one Proxy per goroutine/task
//   - Each goroutine writes only to its own Proxy
//   - After all goroutines complete, merge proxies on a single goroutine
//     using MergeProxyResponses
//   - Apply the merged result to the ResponseWriter
//
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
	return &Proxy{}
}

type headerOp struct {
	op    string
	value string
}

/////// STATUS (use directly for both success and error responses)

func (p *Proxy) SetStatus(status int, errorStatusText ...string) {
	p._status = status
	if len(errorStatusText) != 0 {
		p._status_text = errorStatusText[0]
		return
	}
	p._status_text = ""
}

func (p *Proxy) Status() (int, string) {
	return p._status, p._status_text
}

/////// HEADERS

func (p *Proxy) SetHeader(key, value string) {
	if p._headerOps == nil {
		p._headerOps = make(map[string][]headerOp)
	}
	canonicalHeaderKey := http.CanonicalHeaderKey(key)
	p._headerOps[canonicalHeaderKey] = append(
		p._headerOps[canonicalHeaderKey],
		headerOp{op: "set", value: value},
	)
}

func (p *Proxy) AddHeader(key, value string) {
	if p._headerOps == nil {
		p._headerOps = make(map[string][]headerOp)
	}
	canonicalHeaderKey := http.CanonicalHeaderKey(key)
	p._headerOps[canonicalHeaderKey] = append(
		p._headerOps[canonicalHeaderKey],
		headerOp{op: "add", value: value},
	)
}

func (p *Proxy) Header(key string) string {
	values := p.computeHeaderValues(http.CanonicalHeaderKey(key))
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (p *Proxy) Headers(key string) []string {
	return p.computeHeaderValues(http.CanonicalHeaderKey(key))
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
	if cookie == nil {
		return
	}
	p._cookies = append(p._cookies, cookie)
}

func (p *Proxy) Cookies() []*http.Cookie {
	return p._cookies
}

/////// HEAD ELEMENTS

func (p *Proxy) AddHeadEls(els *headels.HeadEls) {
	if p._head_els == nil {
		p._head_els = headels.New()
	}
	p._head_els.AddElements(els)
}

func (p *Proxy) HeadEls() *headels.HeadEls {
	if p._head_els == nil {
		p._head_els = headels.New()
	}
	return p._head_els
}

// HeadElsIfPresent returns the proxy head elements only when already present.
func (p *Proxy) HeadElsIfPresent() *headels.HeadEls {
	return p._head_els
}

/////// REDIRECTS

// Redirect sets a redirect on the proxy. If the request accepts client redirects,
// a client redirect is set; otherwise, a server redirect is set.
// Returns whether a client redirect was used and any error from URL validation.
func (p *Proxy) Redirect(
	r *http.Request,
	url string,
	code ...int,
) (bool, error) {
	if doesAcceptClientRedirect(r) {
		if err := p.clientRedirect(url); err != nil {
			return false, err
		}
		return true, nil
	}
	p.serverRedirect(url, resolveSpreadCode(code))
	return false, nil
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

func (p *Proxy) Location() string {
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
	return p.Header(ClientRedirectHeader) != ""
}

func (p *Proxy) IsSuccess() bool {
	return isSuccess(p._status)
}

func (p *Proxy) ApplyToResponseWriter(w http.ResponseWriter, r *http.Request) {
	// Headers
	for key, ops := range p._headerOps {
		canonicalHeaderKey := http.CanonicalHeaderKey(key)
		currentValues := []string{}
		for _, op := range ops {
			if op.op == "set" {
				w.Header().Del(canonicalHeaderKey)
				currentValues = []string{op.value}
			} else {
				currentValues = append(currentValues, op.value)
			}
		}
		for _, v := range currentValues {
			w.Header().Add(canonicalHeaderKey, v)
		}
	}

	// Cookies
	for _, c := range p._cookies {
		if c == nil {
			continue
		}
		http.SetCookie(w, c)
	}

	// Redirect (only if not an error status)
	if p.isServerRedirect() && !p.IsError() {
		if r == nil {
			w.Header().Set("Location", p._location)
			w.WriteHeader(p._status)
			return
		}
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
// by using headels.ToHeadEls(proxy.HeadEls())
func MergeProxyResponses(proxies ...*Proxy) *Proxy {
	merged := &Proxy{}

	// Head Elements -- MERGED IN ORDER
	for _, p := range proxies {
		if p == nil {
			continue
		}
		if p._head_els != nil {
			if merged._head_els == nil {
				merged._head_els = headels.New()
			}
			merged._head_els.AddElements(p._head_els)
		}
	}

	// Headers -- MERGED IN ORDER
	for _, p := range proxies {
		if p == nil {
			continue
		}
		for key, ops := range p._headerOps {
			if merged._headerOps == nil {
				merged._headerOps = make(map[string][]headerOp)
			}
			merged._headerOps[key] = append(merged._headerOps[key], ops...)
		}
	}

	// Cookies -- MERGED IN ORDER (later cookies overwrite earlier ones with same name)
	var uniqueCookiesMap map[string]*cookieWithIdx
	for i, p := range proxies {
		if p == nil {
			continue
		}
		for _, c := range p._cookies {
			if c == nil {
				continue
			}
			if uniqueCookiesMap == nil {
				uniqueCookiesMap = make(map[string]*cookieWithIdx)
			}
			uniqueCookiesMap[c.Name] = &cookieWithIdx{i, c}
		}
	}

	if uniqueCookiesMap != nil {
		deduped := make([]*cookieWithIdx, 0, len(uniqueCookiesMap))
		for _, c := range uniqueCookiesMap {
			deduped = append(deduped, c)
		}
		slices.SortStableFunc(deduped, func(i, j *cookieWithIdx) int {
			return i.idx - j.idx
		})

		merged._cookies = make([]*http.Cookie, 0, len(deduped))
		for _, c := range deduped {
			merged._cookies = append(merged._cookies, c.cookie)
		}
	}

	// Status
	// Either FIRST ERROR or LAST SUCCESS will win
	for _, p := range proxies {
		if p == nil {
			continue
		}
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
			if p == nil {
				continue
			}
			if p.IsRedirect() {
				merged._status = p._status
				merged._location = p._location
				if p.isClientRedirect() {
					merged.SetHeader(
						ClientRedirectHeader,
						p.Header(ClientRedirectHeader),
					)
				}
				break
			}
		}
	}

	return merged
}

// Package response provides thin, explicit HTTP response helpers.
//
// The Response type wraps http.ResponseWriter and centralizes common
// response patterns (JSON, text, HTML, redirects, status/error helpers).
//
// The Proxy type buffers response intent (headers, cookies, status, redirects)
// for handlers that run in parallel or lack direct ResponseWriter access.
// Proxy instances are NOT thread-safe — create one per goroutine/task, then
// merge on a single goroutine via MergeProxyResponses before applying.
package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/vormadev/vorma/kit/head"
)

/////////////////////////////////////////////////////////////////////
/////// RESPONSE
/////////////////////////////////////////////////////////////////////

// Response is a convenience wrapper for writing HTTP responses.
type Response struct {
	Writer       http.ResponseWriter
	is_committed bool
}

// New creates a Response helper around w.
func New(w http.ResponseWriter) Response {
	return Response{Writer: w}
}

// IsCommitted reports whether a status or body has already been written.
func (res *Response) IsCommitted() bool {
	return res.is_committed
}

// --- General helpers ---

// SetHeader sets a response header without committing the response.
func (res *Response) SetHeader(key, value string) {
	res.Writer.Header().Set(key, value)
}

// AddHeader appends a response header value without committing the response.
func (res *Response) AddHeader(key, value string) {
	res.Writer.Header().Add(key, value)
}

// SetStatus writes the status code and marks the response committed.
func (res *Response) SetStatus(status int) {
	res.Writer.WriteHeader(status)
	res.flag_committed()
}

// Error writes an HTTP error with optional custom reason text.
func (res *Response) Error(status int, reasons ...string) {
	reason := strings.Join(reasons, " ")
	if reason == "" {
		reason = http.StatusText(status)
	}
	http.Error(res.Writer, reason, status)
	res.flag_committed()
}

// --- Contentful responses ---

// JSONBytes writes pre-encoded JSON bytes.
func (res *Response) JSONBytes(bytes []byte) {
	res.SetHeader("Content-Type", "application/json")
	res.Writer.Write(bytes)
	res.flag_committed()
}

// JSON marshals v as JSON and writes it.
func (res *Response) JSON(v any) {
	res.SetHeader("Content-Type", "application/json")
	json.NewEncoder(res.Writer).Encode(v)
	res.flag_committed()
}

// OK writes a 200 response with {"ok":true}.
func (res *Response) OK() {
	res.SetHeader("Content-Type", "application/json")
	res.Writer.WriteHeader(http.StatusOK)
	res.Writer.Write([]byte(`{"ok":true}`))
	res.flag_committed()
}

// Text writes plain text.
func (res *Response) Text(text string) {
	res.SetHeader("Content-Type", "text/plain")
	res.Writer.Write([]byte(text))
	res.flag_committed()
}

// OKText writes a 200 response with "OK".
func (res *Response) OKText() {
	res.SetHeader("Content-Type", "text/plain")
	res.Writer.WriteHeader(http.StatusOK)
	res.Writer.Write([]byte("OK"))
	res.flag_committed()
}

// HTMLBytes writes HTML bytes.
func (res *Response) HTMLBytes(bytes []byte) {
	res.SetHeader("Content-Type", "text/html")
	res.Writer.Write(bytes)
	res.flag_committed()
}

// HTML writes an HTML string.
func (res *Response) HTML(html string) {
	res.HTMLBytes([]byte(html))
}

// --- Status responses ---

func (res *Response) NotModified() { res.SetStatus(http.StatusNotModified) }

func (res *Response) NotFound() { res.SetStatus(http.StatusNotFound) }

func (res *Response) Unauthorized(
	reasons ...string,
) {
	res.Error(http.StatusUnauthorized, reasons...)
}
func (res *Response) InternalServerError(reasons ...string) {
	res.Error(http.StatusInternalServerError, reasons...)
}

func (res *Response) BadRequest(
	reasons ...string,
) {
	res.Error(http.StatusBadRequest, reasons...)
}
func (res *Response) TooManyRequests(reasons ...string) {
	res.Error(http.StatusTooManyRequests, reasons...)
}

func (res *Response) Forbidden(
	reasons ...string,
) {
	res.Error(http.StatusForbidden, reasons...)
}
func (res *Response) MethodNotAllowed(reasons ...string) {
	res.Error(http.StatusMethodNotAllowed, reasons...)
}

// --- Redirects ---

const (
	// ClientRedirectHeader carries a client-consumed redirect target URL.
	ClientRedirectHeader = "X-Client-Redirect"
	// ClientAcceptsRedirectHeader opts-in to client redirect responses.
	ClientAcceptsRedirectHeader = "X-Accepts-Client-Redirect"
)

// Redirect chooses client or server redirect based on request headers.
func (res *Response) Redirect(
	r *http.Request, url string, code ...int,
) (used_client_redirect bool, err error) {
	if does_accept_client_redirect(r) {
		if err := res.ClientRedirect(url); err != nil {
			return false, err
		}
		return true, nil
	}
	res.ServerRedirect(r, url, resolve_code(code))
	return false, nil
}

// ServerRedirect writes an HTTP redirect response.
func (res *Response) ServerRedirect(r *http.Request, url string, code ...int) {
	c := resolve_code(code)
	if r == nil {
		res.SetHeader("Location", url)
		res.SetStatus(c)
		return
	}
	http.Redirect(res.Writer, r, url, c)
	res.flag_committed()
}

// ClientRedirect sets a client-side redirect header and status 200.
func (res *Response) ClientRedirect(url string) error {
	if !validate_url(url) {
		return fmt.Errorf("invalid URL: %s", url)
	}
	if res.is_committed {
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

func (res *Response) flag_committed() {
	res.is_committed = true
}

/////////////////////////////////////////////////////////////////////
/////// PROXY
/////////////////////////////////////////////////////////////////////

type header_op struct {
	op    string // "set" or "add"
	value string
}

// Proxy buffers response intent for deferred or parallel handlers.
// Do not instantiate directly — use NewProxy().
type Proxy struct {
	_status      int
	_status_text string
	_header_ops  map[string][]header_op
	_cookies     []*http.Cookie
	_head        *head.Builder
	_location    string
}

// NewProxy creates an empty Proxy.
func NewProxy() *Proxy {
	return &Proxy{}
}

// --- Status ---

// SetStatus sets the response status and optional error text.
func (p *Proxy) SetStatus(status int, error_text ...string) {
	p._status = status
	if len(error_text) != 0 {
		p._status_text = error_text[0]
		return
	}
	p._status_text = ""
}

// Status returns the status code and error text.
func (p *Proxy) Status() (int, string) {
	return p._status, p._status_text
}

// --- Headers ---

// SetHeader replaces header values for key.
func (p *Proxy) SetHeader(key, value string) {
	if p._header_ops == nil {
		p._header_ops = make(map[string][]header_op)
	}
	k := http.CanonicalHeaderKey(key)
	p._header_ops[k] = append(
		p._header_ops[k],
		header_op{op: "set", value: value},
	)
}

// AddHeader appends a header value for key.
func (p *Proxy) AddHeader(key, value string) {
	if p._header_ops == nil {
		p._header_ops = make(map[string][]header_op)
	}
	k := http.CanonicalHeaderKey(key)
	p._header_ops[k] = append(
		p._header_ops[k],
		header_op{op: "add", value: value},
	)
}

// Header returns the first value for key, or "".
func (p *Proxy) Header(key string) string {
	vals := p.compute_header_values(http.CanonicalHeaderKey(key))
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// Headers returns all values for key.
func (p *Proxy) Headers(key string) []string {
	return p.compute_header_values(http.CanonicalHeaderKey(key))
}

func (p *Proxy) compute_header_values(key string) []string {
	ops := p._header_ops[key]
	if len(ops) == 0 {
		return nil
	}
	var vals []string
	for _, op := range ops {
		if op.op == "set" {
			vals = []string{op.value}
		} else {
			vals = append(vals, op.value)
		}
	}
	return vals
}

// --- Cookies ---

// SetCookie appends a cookie to the proxy.
func (p *Proxy) SetCookie(cookie *http.Cookie) {
	if cookie == nil {
		return
	}
	p._cookies = append(p._cookies, cookie)
}

// Cookies returns all cookies set on the proxy.
func (p *Proxy) Cookies() []*http.Cookie {
	return p._cookies
}

// --- Head elements ---

// MergeHead merges another head builder into the proxy's head state.
func (p *Proxy) MergeHead(b *head.Builder) {
	if p._head == nil {
		p._head = head.NewBuilder()
	}
	p._head.Append(b)
}

// HeadBuilder returns the proxy's head builder, creating it if needed.
func (p *Proxy) HeadBuilder() *head.Builder {
	if p._head == nil {
		p._head = head.NewBuilder()
	}
	return p._head
}

// --- Redirects ---

// Redirect sets a redirect on the proxy. Returns whether a client redirect
// was used and any error from URL validation.
func (p *Proxy) Redirect(
	r *http.Request, url string, code ...int,
) (bool, error) {
	if does_accept_client_redirect(r) {
		if err := p.client_redirect(url); err != nil {
			return false, err
		}
		return true, nil
	}
	p.server_redirect(url, resolve_code(code))
	return false, nil
}

func (p *Proxy) server_redirect(url string, code ...int) {
	if p.IsError() {
		return
	}
	p._status = resolve_code(code)
	p._location = url
}

func (p *Proxy) client_redirect(url string) error {
	if !validate_url(url) {
		return fmt.Errorf("invalid URL: %s", url)
	}
	if p._status == 0 {
		p.SetStatus(http.StatusOK)
	}
	p.SetHeader(ClientRedirectHeader, url)
	return nil
}

// Location returns the redirect URL, if set.
func (p *Proxy) Location() string {
	return p._location
}

// --- Status helpers ---

// IsError reports whether the status is 400+.
func (p *Proxy) IsError() bool { return is_error(p._status) }

// IsRedirect reports whether any redirect (server or client) is set.
func (p *Proxy) IsRedirect() bool {
	return p.is_server_redirect() || p.is_client_redirect()
}

// IsSuccess reports whether the status is 2xx.
func (p *Proxy) IsSuccess() bool { return is_success(p._status) }

func (p *Proxy) is_server_redirect() bool {
	return is_server_redirect(p._status) && p._location != ""
}

func (p *Proxy) is_client_redirect() bool {
	return p.Header(ClientRedirectHeader) != ""
}

// --- Apply ---

// ApplyToResponseWriter writes all buffered proxy state to the real writer.
func (p *Proxy) ApplyToResponseWriter(w http.ResponseWriter, r *http.Request) {
	// Headers
	for key, ops := range p._header_ops {
		ck := http.CanonicalHeaderKey(key)
		var current []string
		for _, op := range ops {
			if op.op == "set" {
				w.Header().Del(ck)
				current = []string{op.value}
			} else {
				current = append(current, op.value)
			}
		}
		for _, v := range current {
			w.Header().Add(ck, v)
		}
	}

	// Cookies
	for _, c := range p._cookies {
		if c == nil {
			continue
		}
		http.SetCookie(w, c)
	}

	// Server redirect (only if not an error status)
	if p.is_server_redirect() && !p.IsError() {
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
		if is_error(p._status) {
			text := p._status_text
			if text == "" {
				text = http.StatusText(p._status)
			}
			http.Error(w, text, p._status)
		} else {
			w.WriteHeader(p._status)
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// MERGE PROXIES
/////////////////////////////////////////////////////////////////////

type cookie_with_idx struct {
	idx    int
	cookie *http.Cookie
}

// MergeProxyResponses merges multiple proxies into one. Rules:
//   - Head elements, headers, cookies: merged in order (later cookies dedupe by name).
//   - Status: first error wins, otherwise last success wins.
//   - Redirect: first redirect wins (assuming no error).
func MergeProxyResponses(proxies ...*Proxy) *Proxy {
	merged := &Proxy{}

	// Head elements
	for _, p := range proxies {
		if p == nil || p._head == nil {
			continue
		}
		if merged._head == nil {
			merged._head = head.NewBuilder()
		}
		merged._head.Append(p._head)
	}

	// Headers
	for _, p := range proxies {
		if p == nil {
			continue
		}
		for key, ops := range p._header_ops {
			if merged._header_ops == nil {
				merged._header_ops = make(map[string][]header_op)
			}
			merged._header_ops[key] = append(merged._header_ops[key], ops...)
		}
	}

	// Cookies (later cookies with same name overwrite earlier ones)
	var unique_cookies map[string]*cookie_with_idx
	for i, p := range proxies {
		if p == nil {
			continue
		}
		for _, c := range p._cookies {
			if c == nil {
				continue
			}
			if unique_cookies == nil {
				unique_cookies = make(map[string]*cookie_with_idx)
			}
			unique_cookies[c.Name] = &cookie_with_idx{idx: i, cookie: c}
		}
	}
	if unique_cookies != nil {
		deduped := make([]*cookie_with_idx, 0, len(unique_cookies))
		for _, c := range unique_cookies {
			deduped = append(deduped, c)
		}
		slices.SortStableFunc(deduped, func(a, b *cookie_with_idx) int {
			return a.idx - b.idx
		})
		merged._cookies = make([]*http.Cookie, 0, len(deduped))
		for _, c := range deduped {
			merged._cookies = append(merged._cookies, c.cookie)
		}
	}

	// Status: first error or last success
	for _, p := range proxies {
		if p == nil {
			continue
		}
		if p._status >= 400 {
			merged._status = p._status
			merged._status_text = p._status_text
			break
		} else if merged._status < 300 {
			merged._status = p._status
			merged._status_text = p._status_text
		}
	}

	// Redirect: first redirect wins (if no error)
	if !is_error(merged._status) {
		for _, p := range proxies {
			if p == nil {
				continue
			}
			if p.IsRedirect() {
				merged._status = p._status
				merged._location = p._location
				if p.is_client_redirect() {
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

/////////////////////////////////////////////////////////////////////
/////// SHARED HELPERS
/////////////////////////////////////////////////////////////////////

func is_error(status int) bool { return status >= 400 }

func is_server_redirect(
	status int,
) bool {
	return status >= 300 && status < 400
}

func is_success(
	status int,
) bool {
	return status >= 200 && status < 300
}

func resolve_code(code []int) int {
	c := http.StatusSeeOther
	if len(code) > 0 {
		c = code[0]
	}
	if c < 300 || c > 399 {
		c = http.StatusSeeOther
	}
	return c
}

func does_accept_client_redirect(r *http.Request) bool {
	if r == nil {
		return false
	}
	yes, err := strconv.ParseBool(r.Header.Get(ClientAcceptsRedirectHeader))
	return err == nil && yes
}

func validate_url(location string) bool {
	if location == "" {
		return false
	}
	u, err := url.Parse(location)
	if err != nil {
		return false
	}
	if u.IsAbs() && u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return true
}

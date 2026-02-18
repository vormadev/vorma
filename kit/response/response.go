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
	"strconv"
	"strings"
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

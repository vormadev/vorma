package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Response struct {
	Writer      http.ResponseWriter
	isCommitted bool
}

func New(w http.ResponseWriter) Response {
	return Response{Writer: w}
}

func (res *Response) IsCommitted() bool {
	return res.isCommitted
}

/////////////////////////////////////////////////////////////////////
// General helpers
/////////////////////////////////////////////////////////////////////

func (res *Response) SetHeader(key, value string) {
	res.Writer.Header().Set(key, value)
	// should not commit here
}

func (res *Response) AddHeader(key, value string) {
	res.Writer.Header().Add(key, value)
	// should not commit here
}

func (res *Response) SetStatus(status int) {
	res.Writer.WriteHeader(status)
	res.flagAsCommitted()
}

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

// Pass in pre-encoded JSON bytes
func (res *Response) JSONBytes(bytes []byte) {
	res.SetHeader("Content-Type", "application/json")
	res.Writer.Write(bytes)
	res.flagAsCommitted()
}

// Pass in non-encoded JSON-marshallable data
func (res *Response) JSON(v any) {
	res.SetHeader("Content-Type", "application/json")
	json.NewEncoder(res.Writer).Encode(v)
	res.flagAsCommitted()
}

// Returns a 200 JSON response of {"ok":true}
func (res *Response) OK() {
	res.SetHeader("Content-Type", "application/json")
	res.Writer.WriteHeader(http.StatusOK)
	res.Writer.Write([]byte(`{"ok":true}`))
	res.flagAsCommitted()
}

func (res *Response) Text(text string) {
	res.SetHeader("Content-Type", "text/plain")
	res.Writer.Write([]byte(text))
	res.flagAsCommitted()
}

// Returns a 200 text response of "OK"
func (res *Response) OKText() {
	res.SetHeader("Content-Type", "text/plain")
	res.Writer.WriteHeader(http.StatusOK)
	res.Writer.Write([]byte("OK"))
	res.flagAsCommitted()
}

func (res *Response) HTMLBytes(bytes []byte) {
	res.SetHeader("Content-Type", "text/html")
	res.Writer.Write(bytes)
	res.flagAsCommitted()
}

func (res *Response) HTML(html string) {
	res.HTMLBytes([]byte(html))
}

/////////////////////////////////////////////////////////////////////
// HTTP status responses
/////////////////////////////////////////////////////////////////////

func (res *Response) NotModified() {
	res.SetStatus(http.StatusNotModified)
}

func (res *Response) NotFound() {
	res.SetStatus(http.StatusNotFound)
}

/////////////////////////////////////////////////////////////////////
// Error responses
/////////////////////////////////////////////////////////////////////

func (res *Response) Unauthorized(reasons ...string) {
	res.Error(http.StatusUnauthorized, reasons...)
}

func (res *Response) InternalServerError(reasons ...string) {
	res.Error(http.StatusInternalServerError, reasons...)
}

func (res *Response) BadRequest(reasons ...string) {
	res.Error(http.StatusBadRequest, reasons...)
}

func (res *Response) TooManyRequests(reasons ...string) {
	res.Error(http.StatusTooManyRequests, reasons...)
}

func (res *Response) Forbidden(reasons ...string) {
	res.Error(http.StatusForbidden, reasons...)
}

func (res *Response) MethodNotAllowed(reasons ...string) {
	res.Error(http.StatusMethodNotAllowed, reasons...)
}

/////////////////////////////////////////////////////////////////////
// Redirects
/////////////////////////////////////////////////////////////////////

const (
	ClientRedirectHeader        = "X-Client-Redirect"
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

func (res *Response) Redirect(r *http.Request, url string, code ...int) (usedClientRedirect bool, err error) {
	if doesAcceptClientRedirect(r) {
		if err := res.ClientRedirect(url); err != nil {
			return false, err
		}
		return true, nil
	}
	res.ServerRedirect(r, url, resolveSpreadCode(code))
	return false, nil
}

func (res *Response) ServerRedirect(r *http.Request, url string, code ...int) {
	http.Redirect(res.Writer, r, url, resolveSpreadCode(code))
	res.flagAsCommitted()
}

// ClientRedirect sets a client-side redirect header and status 200.
// Returns an error if the response is already committed.
func (res *Response) ClientRedirect(url string) error {
	if ok := validateURL(url); !ok {
		return fmt.Errorf("invalid URL: %s", url)
	}
	if res.isCommitted {
		return fmt.Errorf("cannot set client redirect: response already committed")
	}
	res.SetHeader(ClientRedirectHeader, url)
	res.SetStatus(http.StatusOK)
	return nil
}

func GetClientRedirectURL(w http.ResponseWriter) string {
	return w.Header().Get(ClientRedirectHeader)
}

func doesAcceptClientRedirect(r *http.Request) bool {
	yes, err := strconv.ParseBool(r.Header.Get(ClientAcceptsRedirectHeader))
	return err == nil && yes
}

func validateURL(location string) (ok bool) {
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

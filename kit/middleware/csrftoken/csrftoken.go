// Deprecated: Please upgrade to the newer, more robust, and stateless csrf
// package available at github.com/vormadev/vorma/kit/csrf.
package csrftoken

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/vormadev/vorma/kit/response"
)

type (
	GetExpectedCSRFToken  = func(r *http.Request) string
	GetSubmittedCSRFToken = func(r *http.Request) string
)

// Deprecated: Please upgrade to the newer, more robust, and stateless csrf
// package available at github.com/vormadev/vorma/kit/csrf.
type Opts struct {
	GetExpectedCSRFToken  GetExpectedCSRFToken
	GetSubmittedCSRFToken GetSubmittedCSRFToken
	GetIsExempt           func(r *http.Request) bool // Exempts from CSRF token check (not host check)
	PermittedHosts        []string                   // If len == 0, all hosts are permitted
}

// Deprecated: Please upgrade to the newer, more robust, and stateless csrf
// package available at github.com/vormadev/vorma/kit/csrf.
func NewMiddleware(opts Opts) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			res := response.New(w)

			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
				next.ServeHTTP(w, r)
				return
			}

			lowercaseHost, err := getLowercaseHost(r)
			if err != nil {
				res.InternalServerError("")
				return
			}
			if lowercaseHost == "" {
				res.BadRequest("Origin not provided")
				return
			}
			if len(opts.PermittedHosts) > 0 {
				permitted := false
				for _, permittedHost := range opts.PermittedHosts {
					if lowercaseHost == strings.ToLower(permittedHost) {
						permitted = true
						break
					}
				}
				if !permitted {
					res.Forbidden("Origin not permitted")
					return
				}
			}

			if opts.GetIsExempt != nil && opts.GetIsExempt(r) {
				next.ServeHTTP(w, r)
				return
			}

			expectedToken := opts.GetExpectedCSRFToken(r)
			if expectedToken == "" {
				res.InternalServerError("")
				return
			}

			submittedToken := opts.GetSubmittedCSRFToken(r)
			if submittedToken == "" {
				res.BadRequest("CSRF token missing")
				return
			}

			if submittedToken != expectedToken {
				res.Forbidden("CSRF token mismatch")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getLowercaseHost(r *http.Request) (string, error) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}
	if origin == "" {
		return "", nil
	}
	originURL, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	if originURL.Scheme == "" || originURL.Host == "" {
		return "", errors.New("invalid URL: missing scheme or host")
	}
	return strings.ToLower(originURL.Host), nil
}

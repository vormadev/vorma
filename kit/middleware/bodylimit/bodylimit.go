// Package bodylimit provides HTTP middleware for bounding request bodies.
package bodylimit

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"github.com/vormadev/vorma/kit/ioutil"
)

type Config struct {
	// Limit, in bytes, for the request body. Can be overridden on a per-request basis with SpecificLimit.
	// If set to 0, a default of 4 MiB is used.
	DefaultLimit uint64
	// If present, SpecificLimit is called for each request to determine the body limit (in bytes) for
	// that request. It takes precedence over DefaultLimit. If it returns 0, DefaultLimit is used.
	SpecificLimit func(*http.Request) uint64
}

// New returns middleware that reads at most Limit bytes from a request body
// before handing the request to downstream handlers.
//
// It returns 413 when the limit is exceeded and 400 for other body read
// failures. The request body is restored for downstream handlers.
func New(cfg ...Config) func(http.Handler) http.Handler {
	cfg_to_use := Config{}
	if len(cfg) > 0 {
		cfg_to_use = cfg[0]
	}

	default_limit := cfg_to_use.DefaultLimit
	if default_limit == 0 {
		default_limit = 4 * ioutil.OneMB
	}
	specific_limit := cfg_to_use.SpecificLimit

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}

			limit := default_limit
			if specific_limit != nil {
				limit = specific_limit(r)
				if limit == 0 {
					limit = default_limit
				}
			}

			data, err := ioutil.ReadLimited(r.Body, limit)
			if errors.Is(err, ioutil.ErrReadLimitExceeded) {
				http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
				return
			}
			if err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}

			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(data))
			next.ServeHTTP(w, r)
		})
	}
}

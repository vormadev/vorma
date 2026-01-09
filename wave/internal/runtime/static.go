package runtime

import (
	"net/http"

	"github.com/vormadev/vorma/kit/middleware"
)

// FaviconRedirect returns middleware that redirects /favicon.ico
func (r *Runtime) FaviconRedirect() middleware.Middleware {
	return middleware.ToHandlerMiddleware(
		"/favicon.ico",
		[]string{http.MethodGet, http.MethodHead},
		func(w http.ResponseWriter, req *http.Request) {
			url := r.PublicURL("favicon.ico")
			fallback := r.cfg.PublicPathPrefix() + "favicon.ico"
			if url == fallback {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			http.Redirect(w, req, url, http.StatusFound)
		},
	)
}

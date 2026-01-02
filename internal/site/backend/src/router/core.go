package router

import (
	"net/http"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/vormadev/vorma/kit/middleware/etag"
	"github.com/vormadev/vorma/kit/middleware/healthcheck"
	"github.com/vormadev/vorma/kit/middleware/robotstxt"
	"github.com/vormadev/vorma/kit/middleware/secureheaders"
	"github.com/vormadev/vorma/kit/mux"
)

func Init() (addr string, handler http.Handler) {
	App.Init()

	r := mux.NewRouter()
	loaders, actions := App.Loaders(), App.Actions()

	mux.SetGlobalHTTPMiddleware(r, chimw.Logger)
	mux.SetGlobalHTTPMiddleware(r, chimw.Recoverer)
	mux.SetGlobalHTTPMiddleware(r, etag.Auto())
	mux.SetGlobalHTTPMiddleware(r, chimw.Compress(5))
	mux.SetGlobalHTTPMiddleware(r, App.ServeStatic())
	mux.SetGlobalHTTPMiddleware(r, secureheaders.Middleware)
	mux.SetGlobalHTTPMiddleware(r, healthcheck.Healthz)
	mux.SetGlobalHTTPMiddleware(r, robotstxt.Allow)
	mux.SetGlobalHTTPMiddleware(r, plainMarkdownMiddleware)

	mux.RegisterHandler(r, "GET", loaders.HandlerMountPattern(), loaders.Handler())

	for m := range actions.SupportedMethods() {
		mux.RegisterHandler(r, m, actions.HandlerMountPattern(), actions.Handler())
	}

	return App.ServerAddr(), r
}

func plainMarkdownMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isDocsOrBlog := strings.HasPrefix(r.URL.Path, "/docs") ||
			strings.HasPrefix(r.URL.Path, "/blog")

		if isDocsOrBlog {
			accept := r.Header.Get("Accept")

			isPlaintextReq := strings.Contains(accept, "text/plain") ||
				strings.Contains(accept, "text/markdown")

			if isPlaintextReq {
				markdown, err := Markdown.GetPlainMarkdown(r)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.Header().Set("Cache-Control", htmlCacheControlVal)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(markdown))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

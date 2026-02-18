package router

import (
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/vormadev/vorma/kit/middleware/etag"
	"github.com/vormadev/vorma/kit/middleware/healthcheck"
	"github.com/vormadev/vorma/kit/middleware/robotstxt"
	"github.com/vormadev/vorma/kit/middleware/secureheaders"
)

func Init() (addr string, handler http.Handler) {
	r := App.MustInitWithDefaultRouter()

	r.AddGlobalHTTPMiddleware(chimw.Logger)
	r.AddGlobalHTTPMiddleware(chimw.Recoverer)
	r.AddGlobalHTTPMiddleware(etag.Auto())
	r.AddGlobalHTTPMiddleware(chimw.Compress(5))
	r.AddGlobalHTTPMiddleware(App.MustStaticMiddleware())
	r.AddGlobalHTTPMiddleware(secureheaders.Middleware)
	r.AddGlobalHTTPMiddleware(healthcheck.Healthz)
	r.AddGlobalHTTPMiddleware(robotstxt.Allow)
	r.AddGlobalHTTPMiddleware(Markdown.PlainTextMiddleware(
		"/docs", "/docs/*",
		"/blog", "/blog/*",
	))

	return App.ServerAddr(), r
}

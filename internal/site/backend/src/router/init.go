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
	r := App.InitWithDefaultRouter()

	r.SetGlobalHTTPMiddleware(chimw.Logger)
	r.SetGlobalHTTPMiddleware(chimw.Recoverer)
	r.SetGlobalHTTPMiddleware(etag.Auto())
	r.SetGlobalHTTPMiddleware(chimw.Compress(5))
	r.SetGlobalHTTPMiddleware(App.ServeStatic())
	r.SetGlobalHTTPMiddleware(secureheaders.Middleware)
	r.SetGlobalHTTPMiddleware(healthcheck.Healthz)
	r.SetGlobalHTTPMiddleware(robotstxt.Allow)
	r.SetGlobalHTTPMiddleware(Markdown.PlainTextMiddleware(
		"/docs", "/docs/*",
		"/blog", "/blog/*",
	))

	return App.ServerAddr(), r
}

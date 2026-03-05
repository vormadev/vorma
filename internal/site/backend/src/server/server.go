package server

import (
	"net/http"
	"site/backend/src/app"
	"site/backend/src/markdown"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/vormadev/vorma/kit/middleware/etag"
	"github.com/vormadev/vorma/kit/middleware/healthcheck"
	"github.com/vormadev/vorma/kit/middleware/robotstxt"
	"github.com/vormadev/vorma/kit/middleware/secureheaders"
)

func Init() (addr string, handler http.Handler) {
	r := app.App.MustInitWithDefaultRouter()

	r.AddGlobalHTTPMiddleware(chimw.Logger)
	r.AddGlobalHTTPMiddleware(chimw.Recoverer)
	r.AddGlobalHTTPMiddleware(etag.Auto())
	r.AddGlobalHTTPMiddleware(chimw.Compress(5))
	r.AddGlobalHTTPMiddleware(app.App.MustStaticMiddleware())
	r.AddGlobalHTTPMiddleware(secureheaders.Middleware)
	r.AddGlobalHTTPMiddleware(healthcheck.Healthz)
	r.AddGlobalHTTPMiddleware(robotstxt.Allow)
	r.AddGlobalHTTPMiddleware(markdown.Markdown.PlainTextMiddleware(
		"/docs", "/docs/*",
		"/blog", "/blog/*",
	))

	return app.App.ServerAddr(), r
}

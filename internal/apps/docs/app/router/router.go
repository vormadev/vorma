package router

import (
	"docs/app"
	"docs/app/markdown"
	"docs/app/views/content"
	"docs/app/views/home"
	"fmt"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/middleware/etag"
	"github.com/vormadev/vorma/kit/middleware/healthcheck"
	"github.com/vormadev/vorma/kit/middleware/robotstxt"
	"github.com/vormadev/vorma/kit/middleware/secureheaders"
)

func Router() (*vorma.Router, error) {
	r, err := app.Vorma.Router()
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	r.UseMiddleware(chimw.Logger)
	r.UseMiddleware(chimw.Recoverer)
	r.UseMiddleware(etag.Auto())
	r.UseMiddleware(chimw.Compress(5))
	r.UseMiddleware(chimw.Compress(5, "application/wasm"))
	r.MustUsePublicFileServerMiddleware()
	r.UseMiddleware(secureheaders.Middleware)
	r.UseMiddleware(healthcheck.Healthz)
	r.UseMiddleware(robotstxt.Allow)
	r.UseMiddleware(markdown.Instance.PlainTextMiddleware(
		"/docs", "/docs/*",
		"/blog", "/blog/*",
	))

	for _, apiRoute := range apiRoutes {
		r.APIRoute(apiRoute)
	}
	for _, view := range views {
		r.View(view)
	}

	return r, nil
}

var apiRoutes = vorma.APIRoutes{}

var views = vorma.Views{
	home.View,
	content.View,
}

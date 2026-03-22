package app

import (
	"net/http"
	"path"
	"web/dist"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/middleware/etag"
	"github.com/vormadev/vorma/kit/middleware/healthcheck"
	"github.com/vormadev/vorma/kit/middleware/robotstxt"
	"github.com/vormadev/vorma/kit/middleware/secureheaders"
	"github.com/vormadev/vorma/wave"
)

var Log = colorlog.New("app server")

const (
	Domain          = "vorma.dev"
	SiteTitle       = "Vorma Framework"
	SiteDescription = "The Golang metaframework, powered by Vite."
)

func InitServer() (addr string, handler http.Handler) {
	r := App.MustInitWithDefaultRouter()

	r.AddGlobalHTTPMiddleware(chimw.Logger)
	r.AddGlobalHTTPMiddleware(chimw.Recoverer)
	r.AddGlobalHTTPMiddleware(etag.Auto())
	r.AddGlobalHTTPMiddleware(chimw.Compress(5))
	r.AddGlobalHTTPMiddleware(App.MustStaticMiddleware())
	r.AddGlobalHTTPMiddleware(secureheaders.Middleware)
	r.AddGlobalHTTPMiddleware(healthcheck.Healthz)
	r.AddGlobalHTTPMiddleware(robotstxt.Allow)
	// r.AddGlobalHTTPMiddleware(markdown.Markdown().PlainTextMiddleware(
	// 	"/docs", "/docs/*",
	// 	"/blog", "/blog/*",
	// ))

	return App.ServerAddr(), r
}

var App = vorma.NewVormaApp(vorma.VormaAppConfig{
	Wave: wave.New(wave.Options{DistStaticFS: dist.DistStaticFS}),

	HeadDedupeKeysFunc: func(h *vorma.HeadEls) {
		h.Meta(h.Property("og:title"))
		h.Meta(h.Property("og:description"))
		h.Meta(h.Property("og:type"))
		h.Meta(h.Property("og:image"))
		h.Meta(h.Property("og:url"))
		h.Meta(h.Name("twitter:card"))
		h.Meta(h.Name("twitter:site"))
		h.Link(h.Rel("icon"))
	},

	DefaultHeadElsFunc: func(r *http.Request, app *vorma.Vorma, h *vorma.HeadEls) error {
		currentURL := "https://" + path.Join(Domain, r.URL.Path)

		ogImgURL := app.MustPublicURL("vorma-banner.webp")
		favURL := app.MustPublicURL("favicon.svg")

		if !wave.IsDev() {
			ogImgURL = "https://" + path.Join(Domain, ogImgURL)
		}

		h.Title(SiteTitle)
		h.Description(SiteDescription)

		h.MetaPropertyContent("og:title", SiteTitle)
		h.MetaPropertyContent("og:description", SiteDescription)
		h.MetaPropertyContent("og:type", "website")
		h.MetaPropertyContent("og:image", ogImgURL)
		h.MetaPropertyContent("og:url", currentURL)

		h.MetaNameContent("twitter:card", "summary_large_image")
		h.MetaNameContent("twitter:site", "@vormadev")

		h.Link(h.Rel("icon"), h.Href(favURL), h.Type("image/svg+xml"))

		for _, fontFile := range []string{
			"fonts/jetbrains_mono.woff2",
			"fonts/jetbrains_mono_italic.woff2",
		} {
			fontURL := app.MustPublicURL(fontFile)
			h.Link(
				h.Rel("preload"),
				h.As("font"),
				h.Type("font/woff2"),
				h.CrossOrigin("anonymous"),
				h.Href(fontURL),
			)
		}

		return nil
	},

	RootTemplateDataFunc: func(r *http.Request) (map[string]any, error) {
		return map[string]any{
			// "HTMLClass":                   theme.GetThemeData(r).HTMLClass,
			// "SystemThemeScript":           theme.GetSystemThemeScript(),
			// "SystemThemeScriptSha256Hash": theme.GetSystemThemeScriptSha256Hash(),
		}, nil
	},
})

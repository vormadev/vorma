package router

import (
	"net/http"
	"path"
	"site/backend"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/theme"
	"github.com/vormadev/vorma/wave"
)

var Log = colorlog.New("app server")

const (
	Domain          = "vorma.dev"
	SiteTitle       = "Vorma Framework"
	SiteDescription = "The Next.js of Golang, powered by Vite."
)

var App = vorma.NewVormaApp(vorma.VormaAppConfig{
	Wave: backend.Wave,

	GetHeadElUniqueRules: func(h *vorma.HeadEls) {
		h.Meta(h.Property("og:title"))
		h.Meta(h.Property("og:description"))
		h.Meta(h.Property("og:type"))
		h.Meta(h.Property("og:image"))
		h.Meta(h.Property("og:url"))
		h.Meta(h.Name("twitter:card"))
		h.Meta(h.Name("twitter:site"))
		h.Link(h.Rel("icon"))
	},

	GetDefaultHeadEls: func(r *http.Request, app *vorma.Vorma, h *vorma.HeadEls) error {
		currentURL := "https://" + path.Join(Domain, r.URL.Path)

		ogImgURL := app.GetPublicURL("vorma-banner.webp")
		favURL := app.GetPublicURL("favicon.svg")

		if !wave.GetIsDev() {
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
			fontURL := app.GetPublicURL(fontFile)
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

	GetRootTemplateData: func(r *http.Request) (map[string]any, error) {
		return map[string]any{
			"HTMLClass":                   theme.GetThemeData(r).HTMLClass,
			"SystemThemeScript":           theme.SystemThemeScript,
			"SystemThemeScriptSha256Hash": theme.SystemThemeScriptSha256Hash,
		}, nil
	},
})

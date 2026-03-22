package app

import (
	"net/http"
	"path"
	waveapp "site/__wave"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/theme"
	"github.com/vormadev/vorma/vorma2"
	"github.com/vormadev/vorma/wave"
)

var Log = colorlog.New("app server")

const (
	Domain          = "vorma.dev"
	SiteTitle       = "Vorma Framework"
	SiteDescription = "The Golang metaframework, powered by Vite."
)

var App = vorma2.NewVormaApp(vorma2.VormaAppConfig{
	Wave: wave.New(waveapp.WaveOpts),

	HeadDedupeKeysFunc: func(h *vorma2.HeadEls) {
		h.Meta(h.Property("og:title"))
		h.Meta(h.Property("og:description"))
		h.Meta(h.Property("og:type"))
		h.Meta(h.Property("og:image"))
		h.Meta(h.Property("og:url"))
		h.Meta(h.Name("twitter:card"))
		h.Meta(h.Name("twitter:site"))
		h.Link(h.Rel("icon"))
	},

	DefaultHeadElsFunc: func(r *http.Request, app *vorma2.Vorma, h *vorma2.HeadEls) error {
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
			"HTMLClass":                   theme.GetThemeData(r).HTMLClass,
			"SystemThemeScript":           theme.GetSystemThemeScript(),
			"SystemThemeScriptSha256Hash": theme.GetSystemThemeScriptSha256Hash(),
		}, nil
	},
})

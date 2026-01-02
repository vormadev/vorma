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

	GetHeadElUniqueRules: func() *vorma.HeadEls {
		e := vorma.NewHeadEls(8)

		e.Meta(e.Property("og:title"))
		e.Meta(e.Property("og:description"))
		e.Meta(e.Property("og:type"))
		e.Meta(e.Property("og:image"))
		e.Meta(e.Property("og:url"))
		e.Meta(e.Name("twitter:card"))
		e.Meta(e.Name("twitter:site"))
		e.Link(e.Rel("icon"))

		return e
	},

	GetDefaultHeadEls: func(r *http.Request, app *vorma.Vorma) (*vorma.HeadEls, error) {
		currentURL := "https://" + path.Join(Domain, r.URL.Path)

		ogImgURL := app.GetPublicURL("vorma-banner.webp")
		favURL := app.GetPublicURL("favicon.svg")

		if !wave.GetIsDev() {
			ogImgURL = "https://" + path.Join(Domain, ogImgURL)
		}

		e := vorma.NewHeadEls(12)

		e.Title(SiteTitle)
		e.Description(SiteDescription)

		e.Meta(e.Property("og:title"), e.Content(SiteTitle))
		e.Meta(e.Property("og:description"), e.Content(SiteDescription))
		e.Meta(e.Property("og:type"), e.Content("website"))
		e.Meta(e.Property("og:image"), e.Content(ogImgURL))
		e.Meta(e.Property("og:url"), e.Content(currentURL))

		e.Meta(e.Name("twitter:card"), e.Content("summary_large_image"))
		e.Meta(e.Name("twitter:site"), e.Content("@vormadev"))

		e.Link(e.Rel("icon"), e.Attr("href", favURL), e.Attr("type", "image/svg+xml"))

		for _, fontFile := range []string{
			"fonts/jetbrains_mono.woff2",
			"fonts/jetbrains_mono_italic.woff2",
		} {
			fontURL := app.GetPublicURL(fontFile)
			e.Link(
				e.Rel("preload"),
				e.Attr("as", "font"),
				e.Attr("type", "font/woff2"),
				e.Attr("crossorigin", "anonymous"),
				e.Attr("href", fontURL),
			)
		}

		return e, nil
	},

	GetRootTemplateData: func(r *http.Request) (map[string]any, error) {
		return map[string]any{
			"HTMLClass":                   theme.GetThemeData(r).HTMLClass,
			"SystemThemeScript":           theme.SystemThemeScript,
			"SystemThemeScriptSha256Hash": theme.SystemThemeScriptSha256Hash,
		}, nil
	},
})

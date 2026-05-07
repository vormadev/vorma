package app

import (
	"fmt"
	"net/http"
	"path"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/envutil"
)

const (
	RootTitle       = "Vorma"
	RootDescription = "Vorma seeks to be the world's most straightforward web framework. Here are its docs."
)

var Domain = func() string {
	if envutil.GetStr("VERCEL_ENV", "") == "production" {
		return envutil.GetStr("VERCEL_PROJECT_PRODUCTION_URL", "")
	}
	return envutil.GetStr(
		"VERCEL_URL",
		fmt.Sprintf("localhost:%d", envutil.GetInt("PORT", 0)),
	)
}()

func Href(p string) string {
	proto := "https://"
	if vorma.IsDev() {
		proto = "http://"
	}
	return proto + path.Join(Domain, p)
}

var Vorma = vorma.New(&vorma.Config{
	ServerEntry: "app/cmd/serve/main.go",
	DistDir:     "app/",

	PathConfig: vorma.PathConfig{
		PublicStaticBase: "/",
		APIBase:          "/api/",
	},

	FrontendConfig: vorma.FrontendConfig{
		UIVariant:               "remix",
		JSPackageManagerBaseCmd: "pnpm",
		JSPackageManagerDir:     ".",
		ViteConfigFile:          "vite.config.ts",
		EntryFile:               "app/entry.ts",
		PublicStaticSrcDir:      "app/public/",
		CriticalCSSFile:         "app/styles/critical.css",
	},

	HTMLConfig: vorma.HTMLConfig{
		DefaultHead:  defaultHTMLHead,
		TemplateData: templateData,
	},

	TSGenConfig: vorma.TSGenConfig{
		OutFile:    "app/types.ts",
		ExtraTypes: extraTypesToEmit,
		ExtraRawTS: extraRawTSToEmit,
	},

	DevWatchConfig: vorma.DevWatchConfig{
		WatchPatterns:            []string{"."},
		OnChangeRecompileGo:      []string{},
		OnChangeClientRevalidate: []string{"app/markdown/content/**/*.md"},
	},
})

func defaultHTMLHead(r *http.Request, v *vorma.Instance, h *vorma.HeadBuilder) error {
	h.MetaCharset("utf-8")
	h.MetaNameContent("viewport", "width=device-width, initial-scale=1")

	h.Title(RootTitle)
	h.Description(RootDescription)

	faviconURL, err := v.PublicURL("favicon.svg")
	if err != nil {
		return fmt.Errorf("failed to get favicon URL: %w", err)
	}
	h.Link(
		h.Rel("icon"),
		h.Href(faviconURL),
		h.Type("image/svg+xml"),
	)

	ogImageURL, err := v.PublicURL("vorma-banner.webp")
	if err != nil {
		return fmt.Errorf("failed to get Open Graph image URL: %w", err)
	}
	ogImageFullURL := Href(ogImageURL)

	h.MetaPropertyContent("og:title", RootTitle)
	h.MetaPropertyContent("og:description", RootDescription)
	h.MetaPropertyContent("og:type", "website")
	h.MetaPropertyContent("og:url", Href(r.URL.Path))
	h.MetaPropertyContent("og:image", ogImageFullURL)
	h.MetaPropertyContent("og:site_name", RootTitle)

	h.MetaNameContent("twitter:card", "summary_large_image")
	h.MetaNameContent("twitter:title", RootTitle)
	h.MetaNameContent("twitter:description", RootDescription)
	h.MetaNameContent("twitter:image", ogImageFullURL)

	variants := []int{100, 200, 300, 350, 400, 500, 600, 700, 800, 900}
	fontFiles := make([]string, len(variants)*2)

	for i, v := range variants {
		base := "fonts/IoskeleyMono-%d"
		fontFiles[i] = fmt.Sprintf(base+".woff2", v)
		fontFiles[i+len(variants)] = fmt.Sprintf(base+"-i.woff2", v)
	}

	for _, fontFile := range fontFiles {
		file, err := v.PublicURL(fontFile)
		if err != nil {
			return fmt.Errorf("failed to get font file URL for %s: %w", fontFile, err)
		}
		h.Link(
			h.Rel("preload"),
			h.As("font"),
			h.Type("font/woff2"),
			h.CrossOrigin("anonymous"),
			h.Href(file),
		)
	}

	return nil
}

func templateData(*http.Request) (map[string]any, error) {
	return nil, nil
}

var extraTypesToEmit = []*vorma.GoTypeSrc{}

func extraRawTSToEmit(d *vorma.TSDrafter) *vorma.TSDrafter {
	return d
}

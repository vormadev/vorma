package markdown

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"site/backend/src/app"
	"site/backend/src/define/loader"

	"github.com/adrg/frontmatter"
	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/lab/fsmarkdown"
	"github.com/vormadev/vorma/wave"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type RootData struct {
	LatestVersion string
}

var currentReleaseVersion = "v" + vorma.CurrentReleaseVersion()

var jsonCacheControlVal = strings.Join([]string{
	"public",
	"max-age=60",                     // 1 minute in browser cache
	"s-maxage=86400",                 // 1 day in CDN cache
	"stale-while-revalidate=2592000", // 30 days stale in CDN while revalidating
	// skip "must-revalidate", as browsers seem to interpret it as though max-age=0
}, ", ")

var htmlCacheControlVal = strings.Join([]string{
	"public",
	"max-age=0",                      // no browser cache
	"s-maxage=86400",                 // 1 day in CDN cache
	"stale-while-revalidate=2592000", // 30 days stale in CDN while revalidating
	"must-revalidate",                // revalidate after 1 day in CDN
}, ", ")

var goldmarkInstance = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

var Markdown = sync.OnceValue(func() *fsmarkdown.Instance {
	return fsmarkdown.New(fsmarkdown.Options{
		FS:    app.App.MustPrivateFS(),
		IsDev: wave.IsDev(),
		FrontmatterParser: func(r io.Reader, v any) ([]byte, error) {
			return frontmatter.Parse(r, v)
		},
		MarkdownParser: func(b []byte, w io.Writer) error {
			return goldmarkInstance.Convert(b, w)
		},
	})
})

var _ = loader.Define(
	"/",
	func(c *loader.Ctx) (*RootData, error) {
		r, rp := c.Request(), c.ResponseProxy()

		if !wave.IsDev() {
			// Because this app has no user-specific data, we can cache responses
			// aggressively.
			if vorma.IsJSONRequest(r) {
				rp.SetHeader("Cache-Control", jsonCacheControlVal)
			} else {
				rp.SetHeader("Vary", "Cookie")
				rp.SetHeader("Cache-Control", htmlCacheControlVal)
			}
		}

		return &RootData{
			LatestVersion: currentReleaseVersion,
		}, nil
	},
)

var _ = loader.Define(
	"/_index",
	func(c *loader.Ctx) (string, error) {
		return app.SiteDescription, nil
	},
)

var _ = loader.Define(
	"/*",
	func(c *loader.Ctx) (*fsmarkdown.DetailedPage, error) {
		data, err := Markdown().PageDetails(c.Request())
		if err != nil {
			return nil, fmt.Errorf("failed to get page details: %w", err)
		}

		h := c.HeadEls()

		if data.Title != "" {
			h.Title(fmt.Sprintf("%s | %s", app.SiteTitle, data.Title))
			h.MetaPropertyContent("og:title", data.Title)
		}

		if data.Description != "" {
			h.Description(data.Description)
			h.MetaPropertyContent("og:description", data.Description)
		}

		return data, nil
	},
)

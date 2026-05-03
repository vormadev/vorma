package app

import (
	"docs/app/md"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/lab/fsmarkdown"
)

const SiteTitle = "Vorma"

const SiteDescription = "Vorma seeks to be the world's most straightforward web framework. Here are its docs."

/////// App Instance / Config

func Config(static_fs fs.FS) *vorma.Config {
	return &vorma.Config{
		ServerEntry: "serve",

		DistConfig: vorma.DistConfig{
			OutDir:   "dist",
			StaticFS: static_fs,
		},

		PathConfig: vorma.PathConfig{
			PublicStaticBase: "/",
			APIBase:          "/api/",
		},

		FrontendConfig: vorma.FrontendConfig{
			UIVariant:               "solid",
			JSPackageManagerBaseCmd: "pnpm",
			JSPackageManagerDir:     ".",
			ViteConfigFile:          "vite.config.ts",
			RenderEntry:             "vorma.entry.ts",
			PublicStaticSrcDir:      "public",
			CriticalCSSEntry:        "styles/main.critical.css",
		},

		HTMLConfig: vorma.HTMLConfig{
			Template:     "",
			TemplateData: func(*http.Request) (map[string]any, error) { return nil, nil },
			DefaultHead: func(r *http.Request, v *vorma.Instance, h *vorma.HeadBuilder) error {
				h.MetaCharset("utf-8")
				h.MetaNameContent("viewport", "width=device-width, initial-scale=1")
				h.Title("My App")
				h.Description("Something about my app.")
				return nil
			},
			HeadDedupeKeys: func(*vorma.HeadBuilder) {},
		},

		TSGenConfig: vorma.TSGenConfig{
			OutFile:    "vorma.gen.ts",
			ExtraTypes: []*vorma.GoTypeSrc{},
			ExtraRawTS: func(d *vorma.TSDrafter) *vorma.TSDrafter {
				d.ExportConst("highest_start_idx", 1)
				return d
			},
		},

		DevWatchConfig: vorma.DevWatchConfig{
			WatchRoot:                ".",
			GlobalIgnore:             []string{},
			OnChangeRecompileGo:      []string{},
			OnChangeClientRevalidate: []string{"app/md/content/**/*.md"},
		},
	}
}

func Router(static_fs fs.FS) func() (*vorma.Router, error) {
	instance := vorma.New(Config(static_fs))
	return func() (*vorma.Router, error) {
		r, err := instance.Router()
		if err != nil {
			return nil, err
		}
		r.MustAddPublicFileServerMiddleware()
		for _, view := range Views {
			r.View(view)
		}
		for _, api_route := range APIRoutes {
			r.APIRoute(api_route)
		}
		return r, nil
	}
}

/////// View and API Route Types

type (
	View[I, O any]     = vorma.View[I, O, *RequestCtx[I], RequestCtx[I]]
	APIRoute[I, O any] = vorma.APIRoute[I, O, *RequestCtx[I], RequestCtx[I]]
	RequestCtx[I any]  struct{ *vorma.RequestCtx[I] }
)

func (RequestCtx[I]) Wrap(c *vorma.RequestCtx[I]) *RequestCtx[I] {
	return &RequestCtx[I]{RequestCtx: c}
}

func routes(p string) string {
	return filepath.Join(filepath.FromSlash("./components/routes/"), p)
}

/////// Views

var Views = vorma.Views{
	View[struct{}, struct{}]{
		Pattern:      "/",
		ClientModule: routes("root.tsx"),
	},

	View[struct{}, *fsmarkdown.Result]{
		Pattern:      "/*",
		ClientModule: routes("md.tsx"),
		Loader: func(c *RequestCtx[struct{}]) (*fsmarkdown.Result, error) {
			r := c.Request()
			h := c.HeadBuilder()
			rp := c.ResponseProxy()

			data, found, err := md.MD.Lookup(r.URL.Path)
			if !found {
				rp.SetStatus(404)
				return nil, &vorma.LoaderError{
					ClientMsg: "Page not found.",
				}
			}
			if err != nil {
				rp.SetStatus(500)
				return nil, &vorma.LoaderError{
					ClientMsg: "Something went wrong.",
					Err:       fmt.Errorf("failed to load markdown: %w", err),
				}
			}

			if data.Page.Title != "" {
				h.Title(fmt.Sprintf("%s | %s", data.Page.Title, SiteTitle))
				h.MetaPropertyContent("og:title", data.Page.Title)
			}
			if data.Page.Description != "" {
				h.Title(fmt.Sprintf("%s | %s", data.Page.Description, SiteDescription))
				h.MetaPropertyContent("og:description", data.Page.Description)
			}

			return data, nil
		},
	},
}

/////// API Routes

var APIRoutes = vorma.APIRoutes{}

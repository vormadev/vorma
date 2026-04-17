package app

import (
	"docs/app/md"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/lab/fsmarkdown"
)

const SiteTitle = "Vorma"

const SiteDescription = "Vorma seeks to be the world's most straightforward web framework. Here are its docs."

/////// App Instance / Config

var App = &vorma.Vorma{
	DistDir:     "dist",
	ServerEntry: "serve",

	PathConfig: vorma.PathConfig{
		PublicStaticBase: "/",
		APIBase:          "/api/",
	},

	FrontendConfig: vorma.FrontendConfig{
		UIVariant:               "solid",
		JSPackageManagerBaseCmd: "pnpm",
		JSPackageManagerDir:     ".",
		ViteConfigFile:          "vite.config.ts",
		RenderEntry:             "vorma.entry.tsx",
		PublicStaticSrcDir:      "public",
		MainCSSEntry:            "styles/main.css",
		CriticalCSSEntry:        "styles/main.critical.css",
	},

	HTMLConfig: vorma.HTMLConfig{
		Template:     "",
		TemplateData: func(*http.Request) (map[string]any, error) { return nil, nil },
		DefaultHead: func(r *http.Request, v *vorma.Vorma, h *vorma.HeadEls) error {
			h.MetaCharset("utf-8")
			h.MetaNameContent("viewport", "width=device-width, initial-scale=1")
			h.Title("My App")
			h.Description("Something about my app.")
			return nil
		},
		HeadDedupeKeys: func(*vorma.HeadEls) {},
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
		Root:                     ".",
		GlobalIgnore:             []string{},
		OnChangeRecompileGo:      []string{},
		OnChangeClientRevalidate: []string{"app/md/content/**/*.md"},
	},
}

/////// Loader and Action Types

type (
	Loader[I any, O any] = vorma.Loader[I, O, *LoaderCtx[I], LoaderCtx[I]]
	Action[I any, O any] = vorma.Action[I, O, *ActionCtx[I], ActionCtx[I]]
	LoaderCtx[I any]     struct{ *vorma.LoaderCtx[I] }
	ActionCtx[I any]     struct{ *vorma.ActionCtx[I] }
)

func (LoaderCtx[I]) Wrap(c *vorma.LoaderCtx[I]) *LoaderCtx[I] {
	return &LoaderCtx[I]{LoaderCtx: c}
}

func (ActionCtx[I]) Wrap(c *vorma.ActionCtx[I]) *ActionCtx[I] {
	return &ActionCtx[I]{ActionCtx: c}
}

func routes(p string) string {
	return filepath.Join(filepath.FromSlash("./components/routes/"), p)
}

/////// Loaders

var Loaders = vorma.Loaders{
	Loader[struct{}, struct{}]{
		Pattern:  "/",
		TSModule: routes("root.tsx"),
	},

	Loader[struct{}, *fsmarkdown.Result]{
		Pattern:  "/*",
		TSModule: routes("md.tsx"),
		Handler: func(c *LoaderCtx[struct{}]) (*fsmarkdown.Result, error) {
			r := c.Request()
			h := c.HeadEls()
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

/////// Actions

var Actions = vorma.Actions{}

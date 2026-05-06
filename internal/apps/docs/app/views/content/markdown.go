package content

import (
	"docs/app"
	md "docs/app/markdown"
	"fmt"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/lab/fsmarkdown"
)

// __TODO set up caching

var View = app.View[struct{}, *fsmarkdown.Result]{
	Pattern:    "/*",
	ClientFile: "./app/views/content/content.tsx",
	Loader: func(c *app.RequestCtx[struct{}]) (*fsmarkdown.Result, error) {
		r := c.Request()
		h := c.HeadBuilder()
		rp := c.ResponseProxy()

		data, found, err := md.Instance.Lookup(r.URL.Path)
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
			h.Title(fmt.Sprintf("%s | %s", data.Page.Title, app.RootTitle))
			h.MetaPropertyContent("og:title", data.Page.Title)
		}
		if data.Page.Description != "" {
			h.Description(data.Page.Description)
			h.MetaPropertyContent("og:description", data.Page.Description)
		}

		return data, nil
	},
}

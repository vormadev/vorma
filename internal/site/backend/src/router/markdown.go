package router

import (
	"io"

	"github.com/adrg/frontmatter"
	"github.com/russross/blackfriday/v2"
	"github.com/vormadev/vorma/kit/lab/fsmarkdown"
)

var Markdown = fsmarkdown.New(fsmarkdown.Options{
	FS:                App.MustGetPrivateFS(),
	FrontmatterParser: func(r io.Reader, v any) ([]byte, error) { return frontmatter.Parse(r, v) },
	MarkdownParser: func(b []byte) []byte {
		return blackfriday.Run(b, blackfriday.WithExtensions(blackfriday.AutoHeadingIDs|blackfriday.CommonExtensions))
	},
	IsDev: App.GetIsDev(),
})

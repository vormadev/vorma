package router

import (
	"io"

	"github.com/adrg/frontmatter"
	"github.com/vormadev/vorma/kit/lab/fsmarkdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var goldmarkInstance = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

var Markdown = fsmarkdown.New(fsmarkdown.Options{
	FS:    App.MustGetPrivateFS(),
	IsDev: App.GetIsDev(),
	FrontmatterParser: func(r io.Reader, v any) ([]byte, error) {
		return frontmatter.Parse(r, v)
	},
	MarkdownParser: func(b []byte, w io.Writer) error {
		return goldmarkInstance.Convert(b, w)
	},
})

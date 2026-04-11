package md

import (
	"io"

	"github.com/adrg/frontmatter"
	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/lab/fsmarkdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var goldmark_inst = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

var MD = fsmarkdown.New(fsmarkdown.Options{
	FS:    fs,
	IsDev: vorma.IsDev(),
	FrontmatterParser: func(r io.Reader, v any) ([]byte, error) {
		return frontmatter.Parse(r, v)
	},
	MarkdownParser: func(b []byte, w io.Writer) error {
		return goldmark_inst.Convert(b, w)
	},
})

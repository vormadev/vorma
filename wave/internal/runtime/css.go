package runtime

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/matcher"
)

func (r *Runtime) initCriticalCSS() (*criticalCSSData, error) {
	if r.cfg.CriticalCSSEntry() == "" {
		return &criticalCSSData{noSuchFile: true}, nil
	}

	base, err := r.BaseFS()
	if err != nil {
		return nil, err
	}

	content, err := fs.ReadFile(base, "internal/critical.css")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &criticalCSSData{noSuchFile: true}, nil
		}
		return nil, err
	}

	result := &criticalCSSData{content: string(content)}

	el := htmlutil.Element{
		Tag:                 "style",
		AttributesKnownSafe: map[string]string{"id": CriticalCSSElementID},
		DangerousInnerHTML:  "\n" + result.content,
	}

	sha256Hash, err := htmlutil.AddSha256HashInline(&el)
	if err != nil {
		r.log.Error(fmt.Sprintf("error handling CSP: %v", err))
		return nil, err
	}
	result.sha256Hash = sha256Hash

	renderedEl, err := htmlutil.RenderElement(&el)
	if err != nil {
		r.log.Error(fmt.Sprintf("error rendering element: %v", err))
		return nil, err
	}
	result.styleEl = renderedEl

	return result, nil
}

// CriticalCSS returns the critical CSS content
func (r *Runtime) CriticalCSS() string {
	data, err := r.criticalCSS.Get()
	if err != nil || data == nil || data.noSuchFile {
		return ""
	}
	return data.content
}

// CriticalCSSStyleElement returns an inline style element with critical CSS
func (r *Runtime) CriticalCSSStyleElement() template.HTML {
	data, err := r.criticalCSS.Get()
	if err != nil || data == nil || data.noSuchFile {
		return ""
	}
	return data.styleEl
}

// CriticalCSSStyleElementHash returns the CSP hash for the critical CSS element
func (r *Runtime) CriticalCSSStyleElementHash() string {
	data, err := r.criticalCSS.Get()
	if err != nil || data == nil || data.noSuchFile {
		return ""
	}
	return data.sha256Hash
}

func (r *Runtime) initStylesheetURL() (string, error) {
	if r.cfg.NonCriticalCSSEntry() == "" {
		return "", nil
	}

	base, err := r.BaseFS()
	if err != nil {
		return "", err
	}

	content, err := fs.ReadFile(base, "internal/normal_css_file_ref.txt")
	if err != nil {
		return "", err
	}

	return matcher.EnsureLeadingSlash(path.Join(r.cfg.PublicPathPrefix(), string(content))), nil
}

// StyleSheetURL returns the URL to the non-critical stylesheet
func (r *Runtime) StyleSheetURL() string {
	url, _ := r.stylesheetURL.Get()
	return url
}

func (r *Runtime) initStylesheetLink() (string, error) {
	url := r.StyleSheetURL()
	if url == "" {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString(`<link rel="stylesheet" href="`)
	sb.WriteString(url)
	sb.WriteString(`" id="`)
	sb.WriteString(StyleSheetElementID)
	sb.WriteString(`" />`)

	return sb.String(), nil
}

// StyleSheetLinkElement returns a link element for the stylesheet
func (r *Runtime) StyleSheetLinkElement() template.HTML {
	link, _ := r.stylesheetLink.Get()
	return template.HTML(link)
}

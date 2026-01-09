package runtime

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"path"

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
		// Use proper error checking instead of string matching
		if errors.Is(err, fs.ErrNotExist) {
			return &criticalCSSData{noSuchFile: true}, nil
		}
		return nil, err
	}

	return &criticalCSSData{content: string(content)}, nil
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
	css := r.CriticalCSS()
	if css == "" {
		return ""
	}
	inner := "\n" + css
	return template.HTML(fmt.Sprintf(`<style id="%s">%s</style>`, CriticalCSSElementID, inner))
}

// CriticalCSSStyleElementHash returns the CSP hash for the critical CSS element
func (r *Runtime) CriticalCSSStyleElementHash() string {
	css := r.CriticalCSS()
	if css == "" {
		return ""
	}
	return sha256Base64([]byte("\n" + css))
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
	return fmt.Sprintf(`<link rel="stylesheet" href="%s" id="%s" />`, url, StyleSheetElementID), nil
}

// StyleSheetLinkElement returns a link element for the stylesheet
func (r *Runtime) StyleSheetLinkElement() template.HTML {
	link, _ := r.stylesheetLink.Get()
	return template.HTML(link)
}

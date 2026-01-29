package wave

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

func (w *Wave) initCriticalCSS() (*criticalCSSData, error) {
	if w.cfg.CriticalCSSEntry() == "" {
		return &criticalCSSData{noSuchFile: true}, nil
	}

	base, err := w.GetBaseFS()
	if err != nil {
		return nil, err
	}

	content, err := fs.ReadFile(base, RelPaths.CriticalCSS())
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

	sha256Hash, err := htmlutil.ComputeContentSha256(&el)
	if err != nil {
		w.log.Error(fmt.Sprintf("error handling CSP: %v", err))
		return nil, err
	}
	result.sha256Hash = sha256Hash

	renderedEl, err := htmlutil.RenderElement(&el)
	if err != nil {
		w.log.Error(fmt.Sprintf("error rendering element: %v", err))
		return nil, err
	}
	result.styleEl = renderedEl

	return result, nil
}

func (w *Wave) GetCriticalCSS() template.CSS {
	data, err := w.criticalCSS.get()
	if err != nil || data == nil || data.noSuchFile {
		return ""
	}
	return template.CSS(data.content)
}

func (w *Wave) GetCriticalCSSStyleElement() template.HTML {
	data, err := w.criticalCSS.get()
	if err != nil || data == nil || data.noSuchFile {
		return ""
	}
	return data.styleEl
}

func (w *Wave) GetCriticalCSSStyleElementSha256Hash() string {
	data, err := w.criticalCSS.get()
	if err != nil || data == nil || data.noSuchFile {
		return ""
	}
	return data.sha256Hash
}

func (w *Wave) GetCriticalCSSElementID() string {
	return CriticalCSSElementID
}

func (w *Wave) initStylesheetURL() (string, error) {
	if w.cfg.NonCriticalCSSEntry() == "" {
		return "", nil
	}

	base, err := w.GetBaseFS()
	if err != nil {
		return "", err
	}

	content, err := fs.ReadFile(base, RelPaths.NormalCSSRef())
	if err != nil {
		return "", err
	}

	return matcher.EnsureLeadingSlash(path.Join(w.cfg.PublicPathPrefix(), string(content))), nil
}

func (w *Wave) GetStyleSheetURL() string {
	url, _ := w.stylesheetURL.get()
	return url
}

func (w *Wave) initStylesheetLink() (string, error) {
	url := w.GetStyleSheetURL()
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

func (w *Wave) GetStyleSheetLinkElement() template.HTML {
	link, _ := w.stylesheetLink.get()
	return template.HTML(link)
}

func (w *Wave) GetStyleSheetElementID() string {
	return StyleSheetElementID
}

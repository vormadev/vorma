package wave

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"strings"

	"github.com/vormadev/vorma/kit/htmlutil"
)

func (w *Wave) initCriticalCSS() (*criticalCSSData, error) {
	if w.cfg.CriticalCSSEntry() == "" {
		return &criticalCSSData{noSuchFile: true}, nil
	}

	content, noSuchFile, err := w.readCriticalCSSContent()
	if err != nil {
		return nil, err
	}
	if noSuchFile {
		return &criticalCSSData{noSuchFile: true}, nil
	}

	return w.buildCriticalCSSData(content)
}

func (w *Wave) readCriticalCSSContent() (string, bool, error) {
	baseFS, err := w.BaseFS()
	if err != nil {
		return "", false, err
	}

	content, err := fs.ReadFile(baseFS, RelPaths.CriticalCSS())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", true, nil
		}
		return "", false, err
	}

	return string(content), false, nil
}

func (w *Wave) buildCriticalCSSData(content string) (*criticalCSSData, error) {
	result := &criticalCSSData{content: content}

	el := htmlutil.Element{
		Tag: "style",
		AttributesKnownSafe: map[string]string{
			"id": w.cfg.CriticalCSSStyleElementID(),
		},
		DangerousInnerHTML: "\n" + result.content,
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

func (w *Wave) getCriticalCSSData() *criticalCSSData {
	data, err := w.criticalCSS.get()
	if err != nil || data == nil || data.noSuchFile {
		return nil
	}
	return data
}

func (w *Wave) CriticalCSS() template.CSS {
	data := w.getCriticalCSSData()
	if data == nil {
		return ""
	}
	return template.CSS(data.content)
}

func (w *Wave) CriticalCSSStyleElement() template.HTML {
	data := w.getCriticalCSSData()
	if data == nil {
		return ""
	}
	return data.styleEl
}

func (w *Wave) CriticalCSSStyleElementSha256Hash() string {
	data := w.getCriticalCSSData()
	if data == nil {
		return ""
	}
	return data.sha256Hash
}

func (w *Wave) CriticalCSSElementID() string {
	return w.cfg.CriticalCSSStyleElementID()
}

func (w *Wave) initStylesheetURL() (string, error) {
	if w.cfg.NonCriticalCSSEntry() == "" {
		return "", nil
	}

	return w.initPublicURLFromInternalRefFile(RelPaths.NormalCSSRef())
}

func (w *Wave) StyleSheetURL() string {
	url, _ := w.stylesheetURL.get()
	return url
}

func (w *Wave) initStylesheetLink() (string, error) {
	url := w.StyleSheetURL()
	if url == "" {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString(`<link rel="stylesheet" href="`)
	sb.WriteString(url)
	sb.WriteString(`" id="`)
	sb.WriteString(w.cfg.NonCriticalCSSLinkElementID())
	sb.WriteString(`" />`)

	return sb.String(), nil
}

func (w *Wave) StyleSheetLinkElement() template.HTML {
	link, _ := w.stylesheetLink.get()
	return template.HTML(link)
}

func (w *Wave) StyleSheetElementID() string {
	return w.cfg.NonCriticalCSSLinkElementID()
}

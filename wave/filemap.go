package wave

import (
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/matcher"
)

func (w *Wave) initFileMapURL() (string, error) {
	base, err := w.GetBaseFS()
	if err != nil {
		return "", err
	}

	content, err := fs.ReadFile(base, RelPaths.PublicFileMapRef())
	if err != nil {
		return "", err
	}

	return matcher.EnsureLeadingSlash(path.Join(
		w.cfg.PublicPathPrefix(),
		string(content),
	)), nil
}

func (w *Wave) GetPublicFileMapURL() string {
	url, _ := w.fileMapURL.get()
	return url
}

func (w *Wave) initFileMapDetails() (*fileMapDetails, error) {
	url := w.GetPublicFileMapURL()
	if url == "" {
		return &fileMapDetails{}, nil
	}

	prefix := w.cfg.PublicPathPrefix()

	innerHTMLFormat := `
		import { wavePublicFileMap } from "%s";
		if (!window.__wave) window.__wave = {};
		function getPublicURL(originalPublicURL) { 
			if (originalPublicURL.startsWith("/")) originalPublicURL = originalPublicURL.slice(1);
			return "%s" + (wavePublicFileMap[originalPublicURL] || originalPublicURL);
		}
		window.__wave.getPublicURL = getPublicURL;
`
	innerHTML := fmt.Sprintf(innerHTMLFormat, url, prefix)

	linkEl := htmlutil.Element{
		Tag:         "link",
		Attributes:  map[string]string{"rel": "modulepreload", "href": url},
		SelfClosing: true,
	}

	scriptEl := htmlutil.Element{
		Tag:                "script",
		Attributes:         map[string]string{"type": "module"},
		DangerousInnerHTML: innerHTML,
	}

	sha256Hash, err := htmlutil.ComputeContentSha256(&scriptEl)
	if err != nil {
		return nil, fmt.Errorf("error handling CSP for filemap script: %w", err)
	}

	var sb strings.Builder

	err = htmlutil.RenderElementToBuilder(&linkEl, &sb)
	if err != nil {
		return nil, fmt.Errorf("error rendering link element: %w", err)
	}

	err = htmlutil.RenderElementToBuilder(&scriptEl, &sb)
	if err != nil {
		return nil, fmt.Errorf("error rendering script element: %w", err)
	}

	return &fileMapDetails{
		elements:   sb.String(),
		sha256Hash: sha256Hash,
	}, nil
}

func (w *Wave) GetPublicFileMapElements() template.HTML {
	details, _ := w.fileMapDetails.get()
	if details == nil {
		return ""
	}
	return template.HTML(details.elements)
}

func (w *Wave) GetPublicFileMapScriptSha256Hash() string {
	details, _ := w.fileMapDetails.get()
	if details == nil {
		return ""
	}
	return details.sha256Hash
}

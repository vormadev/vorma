package runtime

import (
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/matcher"
)

func (r *Runtime) initFileMapURL() (string, error) {
	base, err := r.BaseFS()
	if err != nil {
		return "", err
	}

	content, err := fs.ReadFile(base, "internal/public_file_map_file_ref.txt")
	if err != nil {
		return "", err
	}

	return matcher.EnsureLeadingSlash(path.Join(
		r.cfg.PublicPathPrefix(),
		string(content),
	)), nil
}

// PublicFileMapURL returns the URL to the file map JS module
func (r *Runtime) PublicFileMapURL() string {
	url, _ := r.fileMapURL.Get()
	return url
}

func (r *Runtime) initFileMapDetails() (*fileMapDetails, error) {
	url := r.PublicFileMapURL()
	if url == "" {
		return &fileMapDetails{}, nil
	}

	prefix := r.cfg.PublicPathPrefix()

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

	sha256Hash, err := htmlutil.AddSha256HashInline(&scriptEl)
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

// PublicFileMapElements returns HTML for loading the file map
func (r *Runtime) PublicFileMapElements() template.HTML {
	details, _ := r.fileMapDetails.Get()
	if details == nil {
		return ""
	}
	return template.HTML(details.elements)
}

// PublicFileMapScriptHash returns the CSP hash for the file map script
func (r *Runtime) PublicFileMapScriptHash() string {
	details, _ := r.fileMapDetails.Get()
	if details == nil {
		return ""
	}
	return details.sha256Hash
}

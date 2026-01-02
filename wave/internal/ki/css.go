package ki

import (
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/matcher"
)

const (
	CriticalCSSElementID = "wave-critical-css"
	StyleSheetElementID  = "wave-normal-css"
)

func (c *Config) getInitialStyleSheetLinkElement() (*template.HTML, error) {
	if c._uc.Core.CSSEntryFiles.NonCritical == "" {
		var x template.HTML
		return &x, nil
	}

	var result template.HTML

	url := c.GetStyleSheetURL()

	if url != "" {
		var sb strings.Builder
		sb.WriteString(`<link rel="stylesheet" href="`)
		sb.WriteString(url)
		sb.WriteString(`" id="`)
		sb.WriteString(StyleSheetElementID)
		sb.WriteString(`" />`)
		result = template.HTML(sb.String())
	}

	return &result, nil
}

func (c *Config) getInitialStyleSheetURL() (string, error) {
	if c._uc.Core.CSSEntryFiles.NonCritical == "" {
		return "", nil
	}

	base_fs, err := c.GetBaseFS()
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error getting FS: %v", err))
		return "", err
	}

	dist_wave_internal := c._dist.S().Static.S().Internal

	// __LOCATION_ASSUMPTION: Inside "dist/static"
	content, err := fs.ReadFile(base_fs, path.Join(
		dist_wave_internal.LastSegment(),
		dist_wave_internal.S().NormalCSSFileRefDotTXT.LastSegment(),
	))
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error reading normal CSS URL: %v", err))
		return "", err
	}

	return matcher.EnsureLeadingSlash(
		path.Join(c._uc.Core.PublicPathPrefix, string(content)),
	), nil
}

func (c *Config) GetStyleSheetLinkElement() template.HTML {
	res, _ := c.runtime_cache.stylesheet_link_el.Get()
	return *res
}

func (c *Config) GetStyleSheetURL() string {
	url, _ := c.runtime_cache.stylesheet_url.Get()
	return url
}

type criticalCSSStatus struct {
	code_str     string
	no_such_file bool
	style_el     template.HTML
	sha_256_hash string
}

func (c *Config) getInitialCriticalCSSStatus() (*criticalCSSStatus, error) {
	if c._uc.Core.CSSEntryFiles.Critical == "" {
		return &criticalCSSStatus{
			no_such_file: true,
		}, nil
	}

	result := &criticalCSSStatus{}

	// Get FS
	base_fs, err := c.GetBaseFS()
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error getting FS: %v", err))
		return nil, err
	}

	dist_wave_internal := c._dist.S().Static.S().Internal

	// Read critical CSS
	// __LOCATION_ASSUMPTION: Inside "dist/static"
	content, err := fs.ReadFile(base_fs, path.Join(
		dist_wave_internal.LastSegment(),
		dist_wave_internal.S().CriticalDotCSS.LastSegment(),
	))
	if err != nil {
		// Check if the error is a non-existent file, and set the noSuchFile flag in the cache
		result.no_such_file = strings.HasSuffix(err.Error(), "no such file or directory")

		if !result.no_such_file {
			c.Logger.Error(fmt.Sprintf("error reading critical CSS: %v", err))
			return nil, err
		}
		return result, nil
	}

	result.code_str = string(content)

	el := htmlutil.Element{
		Tag:                 "style",
		AttributesKnownSafe: map[string]string{"id": CriticalCSSElementID},
		DangerousInnerHTML:  "\n" + result.code_str,
	}

	sha256Hash, err := htmlutil.AddSha256HashInline(&el)
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error handling CSP: %v", err))
		return nil, err
	}
	result.sha_256_hash = sha256Hash

	tpmlRes, err := htmlutil.RenderElement(&el)
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error rendering element: %v", err))
		return nil, err
	}

	result.style_el = tpmlRes

	return result, nil
}

func (c *Config) GetCriticalCSS() string {
	result, _ := c.runtime_cache.critical_css.Get()
	return result.code_str
}

func (c *Config) GetCriticalCSSStyleElement() template.HTML {
	result, _ := c.runtime_cache.critical_css.Get()
	return result.style_el
}

func (c *Config) GetCriticalCSSStyleElementSha256Hash() string {
	result, _ := c.runtime_cache.critical_css.Get()
	return result.sha_256_hash
}

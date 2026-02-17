package wave

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/vormadev/vorma/kit/htmlutil"
)

func (w *Wave) initFileMapURL() (string, error) {
	return w.initPublicURLFromInternalRefFile(RelPaths.PublicFileMapRef())
}

func (w *Wave) GetPublicFileMapURL() string {
	url, _ := w.fileMapURL.get()
	return url
}

func (w *Wave) initFileMapDetails() (*fileMapDetails, error) {
	fileMapURL := w.GetPublicFileMapURL()
	if fileMapURL == "" {
		return &fileMapDetails{}, nil
	}

	elements, sha256Hash, err := buildPublicFileMapElements(
		fileMapURL,
		w.cfg.PublicPathPrefix(),
		w.cfg.BrowserRuntimeNamespace(),
		w.cfg.BrowserPublicURLResolverFunctionName(),
	)
	if err != nil {
		return nil, err
	}

	return &fileMapDetails{
		elements:   elements,
		sha256Hash: sha256Hash,
	}, nil
}

func buildPublicFileMapElements(
	fileMapURL string,
	publicPathPrefix string,
	browserRuntimeNamespace string,
	publicURLResolverFunctionName string,
) (string, string, error) {
	linkElement := htmlutil.Element{
		Tag:         "link",
		Attributes:  map[string]string{"rel": "modulepreload", "href": fileMapURL},
		SelfClosing: true,
	}

	scriptElement := htmlutil.Element{
		Tag:        "script",
		Attributes: map[string]string{"type": "module"},
		DangerousInnerHTML: buildPublicFileMapModuleScript(
			fileMapURL,
			publicPathPrefix,
			browserRuntimeNamespace,
			publicURLResolverFunctionName,
		),
	}

	scriptSHA256Hash, err := htmlutil.ComputeContentSha256(&scriptElement)
	if err != nil {
		return "", "", fmt.Errorf("error handling CSP for filemap script: %w", err)
	}

	var elementsBuilder strings.Builder

	err = htmlutil.RenderElementToBuilder(&linkElement, &elementsBuilder)
	if err != nil {
		return "", "", fmt.Errorf("error rendering link element: %w", err)
	}

	err = htmlutil.RenderElementToBuilder(&scriptElement, &elementsBuilder)
	if err != nil {
		return "", "", fmt.Errorf("error rendering script element: %w", err)
	}

	return elementsBuilder.String(), scriptSHA256Hash, nil
}

const publicFileMapModuleScriptFormat = `
		import { wavePublicFileMap } from %q;
		const browserRuntimeNamespace = %q;
		const publicURLResolverFunctionName = %q;
		const publicPathPrefix = %q;
		const normalizedPublicPathPrefixForLookup = %q;
		if (!window[browserRuntimeNamespace]) window[browserRuntimeNamespace] = {};

		function trimConfiguredPublicPathPrefixFromLookupPath(normalizedLookupPath) {
			if (!normalizedPublicPathPrefixForLookup) return normalizedLookupPath;
			if (normalizedLookupPath === normalizedPublicPathPrefixForLookup) return "";

			const normalizedPublicPathPrefixWithTrailingSlash = normalizedPublicPathPrefixForLookup + "/";
			if (normalizedLookupPath.startsWith(normalizedPublicPathPrefixWithTrailingSlash)) {
				return normalizedLookupPath.slice(normalizedPublicPathPrefixWithTrailingSlash.length);
			}

			return normalizedLookupPath;
		}

		function getPublicURL(originalPublicURL) {
			const lowerOriginalPublicURL = originalPublicURL.toLowerCase();
			if (
				lowerOriginalPublicURL.startsWith("data:") ||
				lowerOriginalPublicURL.startsWith("http://") ||
				lowerOriginalPublicURL.startsWith("https://") ||
				lowerOriginalPublicURL.startsWith("ws://") ||
				lowerOriginalPublicURL.startsWith("wss://") ||
				lowerOriginalPublicURL.startsWith("blob:") ||
				lowerOriginalPublicURL.startsWith("file:") ||
				originalPublicURL.startsWith("//")
			) {
				return originalPublicURL;
			}

			let normalizedOriginalPublicURL = originalPublicURL;
			if (normalizedOriginalPublicURL.startsWith("/")) {
				normalizedOriginalPublicURL = normalizedOriginalPublicURL.slice(1);
			}

			const directLookupMatch = wavePublicFileMap[normalizedOriginalPublicURL];
			if (directLookupMatch) {
				return publicPathPrefix + directLookupMatch;
			}

			const deprefixedOriginalPublicURL = trimConfiguredPublicPathPrefixFromLookupPath(
				normalizedOriginalPublicURL,
			);
			const deprefixedLookupMatch = wavePublicFileMap[deprefixedOriginalPublicURL];
			if (deprefixedLookupMatch) {
				return publicPathPrefix + deprefixedLookupMatch;
			}

			return publicPathPrefix + deprefixedOriginalPublicURL;
		}

		window[browserRuntimeNamespace][publicURLResolverFunctionName] = getPublicURL;
`

func buildPublicFileMapModuleScript(
	fileMapURL string,
	publicPathPrefix string,
	browserRuntimeNamespace string,
	publicURLResolverFunctionName string,
) string {
	normalizedPublicPathPrefixForLookup := normalizeConfiguredPublicPathPrefixForLookup(
		publicPathPrefix,
	)

	return fmt.Sprintf(
		publicFileMapModuleScriptFormat,
		fileMapURL,
		browserRuntimeNamespace,
		publicURLResolverFunctionName,
		publicPathPrefix,
		normalizedPublicPathPrefixForLookup,
	)
}

func (w *Wave) getFileMapDetails() *fileMapDetails {
	details, _ := w.fileMapDetails.get()
	if details == nil {
		return nil
	}
	return details
}

func (w *Wave) GetPublicFileMapElements() template.HTML {
	details := w.getFileMapDetails()
	if details == nil {
		return ""
	}
	return template.HTML(details.elements)
}

func (w *Wave) GetPublicFileMapScriptSha256Hash() string {
	details := w.getFileMapDetails()
	if details == nil {
		return ""
	}
	return details.sha256Hash
}

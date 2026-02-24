package waveruntime

import (
	"fmt"
	"html/template"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/htmlutil"
)

func TestBuildRefreshScript_CriticalCSSDecodeGuardHandlesInvalidPayload(
	t *testing.T,
) {
	script := BuildRefreshScript(
		10000,
		RefreshScriptConfig{
			BrowserRevalidateFunctionName:     "__revalidate",
			RefreshRebuildingOverlayElementID: "__overlay",
			NonCriticalCSSLinkElementID:       "__normal_css",
			CriticalCSSStyleElementID:         "__critical_css",
		},
	)

	requiredSnippets := []string{
		`typeof base64 !== "string"`,
		`base64.length === 0`,
		`Wave: Failed to decode critical CSS payload`,
		`return "";`,
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(script, requiredSnippet) {
			t.Fatalf(
				"expected refresh script to include %q guard snippet",
				requiredSnippet,
			)
		}
	}
}

func TestBuildStylesheetLink_EmptyURLReturnsEmptyString(t *testing.T) {
	if got := BuildStylesheetLink("", "__wave_css"); got != "" {
		t.Fatalf("BuildStylesheetLink(empty URL) = %q, want empty", got)
	}
}

func TestBuildStylesheetLink_EscapesDynamicValues(t *testing.T) {
	stylesheetURL := `/assets/main.css?x="><script>alert(1)</script>`
	cssLinkElementID := `wave-style"><img src=x onerror=alert(1)>`

	rendered := BuildStylesheetLink(stylesheetURL, cssLinkElementID)

	if strings.Contains(rendered, `<script>alert(1)</script>`) {
		t.Fatalf(
			"rendered stylesheet link contains raw script payload: %q",
			rendered,
		)
	}
	if strings.Contains(rendered, "<img ") {
		t.Fatalf(
			"rendered stylesheet link contains raw html payload: %q",
			rendered,
		)
	}

	escapedStylesheetURL := template.HTMLEscapeString(stylesheetURL)
	if !strings.Contains(rendered, `href="`+escapedStylesheetURL+`"`) {
		t.Fatalf(
			"rendered stylesheet link missing escaped href, got %q",
			rendered,
		)
	}

	escapedCSSLinkElementID := template.HTMLEscapeString(cssLinkElementID)
	if !strings.Contains(rendered, `id="`+escapedCSSLinkElementID+`"`) {
		t.Fatalf(
			"rendered stylesheet link missing escaped id, got %q",
			rendered,
		)
	}
	if !strings.Contains(rendered, `rel="stylesheet"`) {
		t.Fatalf(
			"rendered stylesheet link missing rel attribute, got %q",
			rendered,
		)
	}
}

func TestBuildCriticalCSSStyleElement_RendersExpectedElementAndHash(
	t *testing.T,
) {
	criticalCSS := "body { color: red; }"
	criticalCSSStyleElementID := "__wave_critical_css"

	renderedElement, sha256Hash, err := BuildCriticalCSSStyleElement(
		criticalCSS,
		criticalCSSStyleElementID,
	)
	if err != nil {
		t.Fatalf("BuildCriticalCSSStyleElement() returned error: %v", err)
	}
	if sha256Hash == "" {
		t.Fatal("BuildCriticalCSSStyleElement() returned empty hash")
	}

	expectedStyleElement := htmlutil.Element{
		Tag: "style",
		AttributesKnownSafe: map[string]string{
			"id": criticalCSSStyleElementID,
		},
		DangerousInnerHTML: "\n" + criticalCSS,
	}
	expectedSHA256Hash, expectedHashError := htmlutil.ComputeContentSha256(
		&expectedStyleElement,
	)
	if expectedHashError != nil {
		t.Fatalf("ComputeContentSha256() returned error: %v", expectedHashError)
	}
	if got, want := sha256Hash, expectedSHA256Hash; got != want {
		t.Fatalf("sha256Hash = %q, want %q", got, want)
	}

	renderedString := string(renderedElement)
	if !strings.Contains(
		renderedString,
		`<style id="`+criticalCSSStyleElementID+`">`,
	) {
		t.Fatalf(
			"rendered style element missing id attribute: %q",
			renderedString,
		)
	}
	if !strings.Contains(renderedString, "\n"+criticalCSS) {
		t.Fatalf(
			"rendered style element missing critical CSS content: %q",
			renderedString,
		)
	}
}

func TestBuildPublicFileMapModuleScript_QuotesInputs(t *testing.T) {
	fileMapURL := `/assets/public-file-map.js" ;window.__xss=true;//`
	browserRuntimeNamespace := `__wave_runtime"]={pwn:true}//`

	moduleScript := BuildPublicFileMapModuleScript(
		fileMapURL,
		browserRuntimeNamespace,
	)

	if !strings.Contains(moduleScript, fmt.Sprintf("%q", fileMapURL)) {
		t.Fatalf(
			"module script missing quoted file-map URL, got %q",
			moduleScript,
		)
	}
	if !strings.Contains(
		moduleScript,
		fmt.Sprintf("%q", browserRuntimeNamespace),
	) {
		t.Fatalf(
			"module script missing quoted runtime namespace, got %q",
			moduleScript,
		)
	}
	if !strings.Contains(
		moduleScript,
		"window[browserRuntimeNamespace].publicFileMap = wavePublicFileMap;",
	) {
		t.Fatalf(
			"module script missing publicFileMap assignment, got %q",
			moduleScript,
		)
	}
}

func TestBuildPublicFileMapElements_RendersMarkupAndHash(t *testing.T) {
	fileMapURL := "/assets/public-file-map.js"
	browserRuntimeNamespace := "__wave_runtime"

	elements, sha256Hash, err := BuildPublicFileMapElements(
		fileMapURL,
		browserRuntimeNamespace,
	)
	if err != nil {
		t.Fatalf("BuildPublicFileMapElements() returned error: %v", err)
	}
	if sha256Hash == "" {
		t.Fatal("BuildPublicFileMapElements() returned empty hash")
	}

	if !strings.Contains(elements, `rel="modulepreload"`) {
		t.Fatalf("elements missing preload link rel attribute: %q", elements)
	}
	if !strings.Contains(
		elements,
		`href="`+template.HTMLEscapeString(fileMapURL)+`"`,
	) {
		t.Fatalf("elements missing file-map href attribute: %q", elements)
	}
	if !strings.Contains(elements, `type="module"`) {
		t.Fatalf("elements missing module script type attribute: %q", elements)
	}

	moduleScript := BuildPublicFileMapModuleScript(
		fileMapURL,
		browserRuntimeNamespace,
	)
	if !strings.Contains(elements, moduleScript) {
		t.Fatalf(
			"elements missing module script payload. script=%q elements=%q",
			moduleScript,
			elements,
		)
	}

	expectedScriptElement := htmlutil.Element{
		Tag:        "script",
		Attributes: map[string]string{"type": "module"},
		DangerousInnerHTML: BuildPublicFileMapModuleScript(
			fileMapURL,
			browserRuntimeNamespace,
		),
	}
	expectedHash, expectedHashError := htmlutil.ComputeContentSha256(
		&expectedScriptElement,
	)
	if expectedHashError != nil {
		t.Fatalf("ComputeContentSha256() returned error: %v", expectedHashError)
	}
	if got, want := sha256Hash, expectedHash; got != want {
		t.Fatalf("sha256Hash = %q, want %q", got, want)
	}
}

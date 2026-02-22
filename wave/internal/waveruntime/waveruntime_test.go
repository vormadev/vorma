package waveruntime

import (
	"strings"
	"testing"
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

package wave

import (
	"strings"
	"testing"
)

func TestRefreshScriptIsOnlyRenderedInDevMode(t *testing.T) {
	fixture := newWaveTestFixture(t)

	wProd := newWaveForTest(t, fixture, false, nil)
	if got := wProd.RefreshScript(); got != "" {
		t.Fatalf("expected empty refresh script in non-dev mode, got %q", got)
	}

	wDev := newWaveForTest(t, fixture, true, nil)
	script := string(wDev.RefreshScript())
	if !strings.Contains(script, "<script>") || !strings.Contains(script, "</script>") {
		t.Fatalf("expected refresh script to include script tag wrapper, got %q", script)
	}
	if !strings.Contains(script, `refreshWebSocketURL.protocol = window.location.protocol === "https:" ? "wss:" : "ws:";`) {
		t.Fatalf("expected origin-aware websocket protocol selection in script, got %q", script)
	}
	if !strings.Contains(script, "refreshWebSocketURL.port = String(10000);") {
		t.Fatalf("expected default refresh port in script, got %q", script)
	}
}

func TestRefreshScriptUsesConfiguredRefreshServerPort(t *testing.T) {
	fixture := newWaveTestFixture(t)
	t.Setenv(envRefreshServerPort, "12345")
	w := newWaveForTest(t, fixture, true, nil)

	script := string(w.RefreshScript())
	if !strings.Contains(script, "refreshWebSocketURL.port = String(12345);") {
		t.Fatalf("expected configured refresh port in script, got %q", script)
	}
}

func TestRefreshScriptFallsBackToDefaultWhenRefreshPortIsInvalid(t *testing.T) {
	fixture := newWaveTestFixture(t)
	t.Setenv(envRefreshServerPort, "-1")
	w := newWaveForTest(t, fixture, true, nil)

	script := string(w.RefreshScript())
	if !strings.Contains(script, "refreshWebSocketURL.port = String(10000);") {
		t.Fatalf("expected default refresh port for invalid configured value, got %q", script)
	}
}

func TestRefreshScriptUsesConfiguredBrowserRuntimeSettings(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	w.setBrowserRevalidateFunctionName("__vorma_revalidate")
	w.setRefreshRebuildingOverlayElementID("vorma-refresh-overlay")
	w.setCriticalCSSStyleElementID("vorma-critical-css")
	w.setNonCriticalCSSLinkElementID("vorma-normal-css")

	script := string(w.RefreshScript())
	if !strings.Contains(script, `const browserRevalidateFunctionName = "__vorma_revalidate";`) {
		t.Fatalf("expected configured revalidate function name in refresh script, got %q", script)
	}
	if !strings.Contains(script, `const refreshRebuildingOverlayElementID = "vorma-refresh-overlay";`) {
		t.Fatalf("expected configured overlay element id in refresh script, got %q", script)
	}
	if !strings.Contains(script, `const criticalCSSStyleElementID = "vorma-critical-css";`) {
		t.Fatalf("expected configured critical css element id in refresh script, got %q", script)
	}
	if !strings.Contains(script, `const nonCriticalCSSLinkElementID = "vorma-normal-css";`) {
		t.Fatalf("expected configured non-critical css element id in refresh script, got %q", script)
	}
}

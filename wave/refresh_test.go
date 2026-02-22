package wave

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
)

func TestRefreshScriptIsOnlyRenderedInDevMode(t *testing.T) {
	fixture := newWaveTestFixture(t)

	wProd := newWaveForTest(t, fixture, false, nil)
	if got := wProd.RefreshScript(); got != "" {
		t.Fatalf("expected empty refresh script in non-dev mode, got %q", got)
	}
	if got := wProd.refreshScriptSha256Hash(); got != "" {
		t.Fatalf("expected empty refresh script hash in non-dev mode, got %q", got)
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

	expectedHash := bytesutil.ToBase64(cryptoutil.Sha256Hash([]byte(refreshScriptInner(defaultRefreshPort))))
	if got := wDev.refreshScriptSha256Hash(); got != expectedHash {
		t.Fatalf("unexpected refresh script hash: got=%q want=%q", got, expectedHash)
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

	expectedHash := bytesutil.ToBase64(cryptoutil.Sha256Hash([]byte(refreshScriptInner(12345))))
	if got := w.refreshScriptSha256Hash(); got != expectedHash {
		t.Fatalf("unexpected refresh script hash for configured port: got=%q want=%q", got, expectedHash)
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

func TestRefreshScriptInnerInterpolatesPort(t *testing.T) {
	inner := refreshScriptInner(42424)
	if !strings.Contains(inner, "refreshWebSocketURL.port = String(42424);") {
		t.Fatalf("expected interpolated websocket URL in refresh script inner, got %q", inner)
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

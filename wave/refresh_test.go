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
	if got := wProd.GetRefreshScript(); got != "" {
		t.Fatalf("expected empty refresh script in non-dev mode, got %q", got)
	}
	if got := wProd.GetRefreshScriptSha256Hash(); got != "" {
		t.Fatalf("expected empty refresh script hash in non-dev mode, got %q", got)
	}

	wDev := newWaveForTest(t, fixture, true, nil)
	script := string(wDev.GetRefreshScript())
	if !strings.Contains(script, "<script>") || !strings.Contains(script, "</script>") {
		t.Fatalf("expected refresh script to include script tag wrapper, got %q", script)
	}
	if !strings.Contains(script, "ws://localhost:10000/events") {
		t.Fatalf("expected default refresh port in script, got %q", script)
	}

	expectedHash := bytesutil.ToBase64(cryptoutil.Sha256Hash([]byte(RefreshScriptInner(defaultRefreshPort))))
	if got := wDev.GetRefreshScriptSha256Hash(); got != expectedHash {
		t.Fatalf("unexpected refresh script hash: got=%q want=%q", got, expectedHash)
	}
}

func TestRefreshScriptUsesConfiguredRefreshServerPort(t *testing.T) {
	fixture := newWaveTestFixture(t)
	t.Setenv(envRefreshServerPort, "12345")
	w := newWaveForTest(t, fixture, true, nil)

	script := string(w.GetRefreshScript())
	if !strings.Contains(script, "ws://localhost:12345/events") {
		t.Fatalf("expected configured refresh port in script, got %q", script)
	}

	expectedHash := bytesutil.ToBase64(cryptoutil.Sha256Hash([]byte(RefreshScriptInner(12345))))
	if got := w.GetRefreshScriptSha256Hash(); got != expectedHash {
		t.Fatalf("unexpected refresh script hash for configured port: got=%q want=%q", got, expectedHash)
	}
}

func TestRefreshScriptFallsBackToDefaultWhenRefreshPortIsInvalid(t *testing.T) {
	fixture := newWaveTestFixture(t)
	t.Setenv(envRefreshServerPort, "-1")
	w := newWaveForTest(t, fixture, true, nil)

	script := string(w.GetRefreshScript())
	if !strings.Contains(script, "ws://localhost:10000/events") {
		t.Fatalf("expected default refresh port for invalid configured value, got %q", script)
	}
}

func TestRefreshScriptInnerInterpolatesPort(t *testing.T) {
	inner := RefreshScriptInner(42424)
	if !strings.Contains(inner, "ws://localhost:42424/events") {
		t.Fatalf("expected interpolated websocket URL in refresh script inner, got %q", inner)
	}
}

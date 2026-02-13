package wave

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFlexibilityContract_CustomNestedPublicPathPrefix(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.PublicPathPrefix = "/cdn/assets/v2"
	waveInstance := newWaveForTest(t, fixture, true, nil)

	if got := waveInstance.GetPublicPathPrefix(); got != "/cdn/assets/v2/" {
		t.Fatalf("expected normalized nested public path prefix %q, got %q", "/cdn/assets/v2/", got)
	}

	if got := waveInstance.GetPublicURL("logo.txt"); got != "/cdn/assets/v2/vorma_out/logo.hash.txt" {
		t.Fatalf("expected mapped URL to honor custom nested prefix, got %q", got)
	}

	nextCalled := false
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		nextCalled = true
		writer.WriteHeader(http.StatusAccepted)
	})

	staticMiddleware := waveInstance.ServeStatic(false)(next)

	assetRequest := httptest.NewRequest(http.MethodGet, "/cdn/assets/v2/logo.txt", nil)
	assetResponse := httptest.NewRecorder()
	staticMiddleware.ServeHTTP(assetResponse, assetRequest)

	if assetResponse.Code != http.StatusOK {
		t.Fatalf("expected static middleware to serve asset under custom nested prefix, got status %d", assetResponse.Code)
	}
	if got := assetResponse.Body.String(); got != "logo" {
		t.Fatalf("expected static middleware asset body %q, got %q", "logo", got)
	}
	if nextCalled {
		t.Fatal("expected static middleware not to call next handler for prefixed asset path")
	}
}

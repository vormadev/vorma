package wire_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/vormadev/vorma"
)

func TestWireManifestConformance(t *testing.T) {
	t.Setenv("WAVE_MODE", "")

	staticFS := makeWireStaticFSWithPaths(
		[]wirePathDef{
			{Pattern: "", SrcPath: "src/man/root.tsx", OutPath: "routes/man_root.js", ExportKey: "Root"},
			{Pattern: "/man/page", SrcPath: "src/man/page.tsx", OutPath: "routes/man_page.js", ExportKey: "Page"},
			{Pattern: "/man/client-only", SrcPath: "src/man/client_only.tsx", OutPath: "routes/man_client_only.js", ExportKey: "ClientOnly"},
		},
		&wireStaticOpts{
			IncludeManifestFile: true,
			RouteManifestFile:   "vorma_out_vorma_internal_route_manifest_conformance.json",
			RouteManifestJSON: mustJSON(nil, map[string]int{
				"":                 1,
				"/man/page":        1,
				"/man/client-only": 0,
			}),
		},
	)

	app := makeWireApp(
		t,
		makeWireConfigJSON(t, nil, nil),
		staticFS,
		nil,
	)
	vorma.NewLoader(
		app,
		"",
		func(*vorma.LoaderReqData) (string, error) { return "root", nil },
		func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
	)
	vorma.NewLoader(
		app,
		"/man/page",
		func(*vorma.LoaderReqData) (string, error) { return "page", nil },
		func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
	)

	router := app.InitWithDefaultRouter()
	handler := app.ServeStatic()(router)

	getDoc := func(t *testing.T) string {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/man/page", nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}

	getManifestURL := func(t *testing.T, html string) string {
		t.Helper()
		re := regexp.MustCompile(`x\.routeManifestURL\s*=\s*([^;]+);`)
		m := re.FindStringSubmatch(html)
		if len(m) != 2 {
			t.Fatalf("failed to extract routeManifestURL assignment from HTML body=%q", html)
		}
		raw := strings.TrimSpace(m[1])
		unquoted, err := strconv.Unquote(raw)
		if err != nil {
			t.Fatalf("expected quoted JS string for routeManifestURL, got raw=%q err=%v", raw, err)
		}
		if unquoted == "" {
			t.Fatalf("expected non-empty routeManifestURL, got empty string")
		}
		return unquoted
	}

	t.Run("WRC-MAN-001_WIRE-MAN-001_bootstrap_includes_non_empty_public_route_manifest_url", func(t *testing.T) {
		html := getDoc(t)
		manifestURL := getManifestURL(t, html)
		if !strings.HasPrefix(manifestURL, "/") {
			t.Fatalf("expected public-path routeManifestURL starting with '/', got %q", manifestURL)
		}
	})

	t.Run("WRC-MAN-002_WIRE-MAN-002_manifest_json_is_record_string_to_zero_or_one", func(t *testing.T) {
		html := getDoc(t)
		manifestURL := getManifestURL(t, html)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, manifestURL, nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d fetching manifest, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("expected JSON object manifest, got err=%v body=%q", err, rec.Body.String())
		}
		if len(payload) == 0 {
			t.Fatalf("expected non-empty manifest object")
		}
		for k, raw := range payload {
			v, ok := raw.(float64)
			if !ok {
				t.Fatalf("expected manifest value for key %q to be number, got %#v", k, raw)
			}
			if int(v) != 0 && int(v) != 1 {
				t.Fatalf("expected manifest value for key %q to be 0 or 1, got %v", k, raw)
			}
		}
	})

	t.Run("WRC-MAN-003_WIRE-MAN-003_manifest_filename_matches_required_prefix_and_suffix", func(t *testing.T) {
		html := getDoc(t)
		manifestURL := getManifestURL(t, html)
		filename := path.Base(manifestURL)
		if !strings.HasPrefix(filename, "vorma_out_vorma_internal_route_manifest_") {
			t.Fatalf("expected manifest filename prefix %q, got %q", "vorma_out_vorma_internal_route_manifest_", filename)
		}
		if !strings.HasSuffix(filename, ".json") {
			t.Fatalf("expected manifest filename to end with .json, got %q", filename)
		}
	})
}

package backend_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/vormadev/vorma"
)

func TestBackendErrorsConformance(t *testing.T) {
	t.Run("BRC-ERR-001_BR-ERR-001_outermost_error_cuts_off_deeper_route_data", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "", SrcPath: "src/err1/root.tsx", OutPath: "routes/err1_root.js", ExportKey: "Root", ErrorExportKey: "RootErr"},
			{Pattern: "/err1/first", SrcPath: "src/err1/first.tsx", OutPath: "routes/err1_first.js", ExportKey: "First", ErrorExportKey: "FirstErr"},
			{Pattern: "/err1/first/second", SrcPath: "src/err1/second.tsx", OutPath: "routes/err1_second.js", ExportKey: "Second", ErrorExportKey: "SecondErr"},
		}))
		vorma.NewLoader(
			app,
			"",
			func(*vorma.LoaderReqData) (string, error) { return "root-ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/err1/first",
			func(*vorma.LoaderReqData) (string, error) {
				// Delay the failing loader so the outer/root loader can complete first.
				time.Sleep(30 * time.Millisecond)
				return "", errors.New("first exploded")
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/err1/first/second",
			func(*vorma.LoaderReqData) (string, error) {
				time.Sleep(200 * time.Millisecond)
				return "second-ok", nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/err1/first/second?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		payload := mustResponseJSONMap(t, rec)

		matchedPatterns := anySliceToStrings(t, mustAnySlice(t, payload, "matchedPatterns"))
		assertStringSliceEquals(t, matchedPatterns, []string{"", "/err1/first"}, "matchedPatterns")

		loadersData := mustAnySlice(t, payload, "loadersData")
		if len(loadersData) != 2 {
			t.Fatalf("expected loadersData len=2 after cutoff, got len=%d (%#v)", len(loadersData), loadersData)
		}

		errorExportKeys := anySliceToStrings(t, mustAnySlice(t, payload, "errorExportKeys"))
		assertStringSliceEquals(t, errorExportKeys, []string{"RootErr", "FirstErr"}, "errorExportKeys")

		outermostIdxRaw, ok := payload["outermostServerErrorIdx"]
		if !ok {
			t.Fatalf("expected outermostServerErrorIdx in payload, got %#v", payload)
		}
		outermostIdx, ok := outermostIdxRaw.(float64)
		if !ok || int(outermostIdx) != 1 {
			t.Fatalf("expected outermostServerErrorIdx=1, got %#v", outermostIdxRaw)
		}
	})

	t.Run("BRC-ERR-004_BR-ERR-001_non_cancellation_error_takes_precedence_over_earlier_cancellation_only_error", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "", SrcPath: "src/err5/root.tsx", OutPath: "routes/err5_root.js", ExportKey: "Root", ErrorExportKey: "RootErr"},
			{Pattern: "/err5/noncancel", SrcPath: "src/err5/noncancel.tsx", OutPath: "routes/err5_noncancel.js", ExportKey: "NonCancel", ErrorExportKey: "NonCancelErr"},
		}))
		vorma.NewLoader(
			app,
			"",
			func(*vorma.LoaderReqData) (string, error) {
				return "", context.Canceled
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/err5/noncancel",
			func(*vorma.LoaderReqData) (string, error) {
				return "", errors.New("deeper non-cancellation failure")
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/err5/noncancel?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		payload := mustResponseJSONMap(t, rec)
		outermostIdxRaw, ok := payload["outermostServerErrorIdx"]
		if !ok {
			t.Fatalf("expected outermostServerErrorIdx in payload, got %#v", payload)
		}
		outermostIdx, ok := outermostIdxRaw.(float64)
		if !ok || int(outermostIdx) != 1 {
			t.Fatalf("expected outermostServerErrorIdx=1 (non-cancellation index), got %#v", outermostIdxRaw)
		}

		if got, _ := payload["outermostServerError"].(string); got != "An error occurred" {
			t.Fatalf("expected generic non-cancellation message, got %#v", payload["outermostServerError"])
		}
	})

	t.Run("BRC-ERR-002_BR-ERR-002_typed_loader_error_exposes_client_message", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/err2", SrcPath: "src/err2/page.tsx", OutPath: "routes/err2_page.js", ExportKey: "Err2"},
		}))
		vorma.NewLoader(
			app,
			"/err2",
			func(*vorma.LoaderReqData) (string, error) {
				return "", &vorma.LoaderError{
					Client: "safe-msg",
					Server: errors.New("db connection failed"),
				}
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/err2?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		payload := mustResponseJSONMap(t, rec)
		if got, _ := payload["outermostServerError"].(string); got != "safe-msg" {
			t.Fatalf("expected outermostServerError=%q, got %#v", "safe-msg", payload["outermostServerError"])
		}
	})

	t.Run("BRC-ERR-003_BR-ERR-003_typed_loader_error_logs_server_error", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, nil))
		app := makeAppWithLogger(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/err3/typed", SrcPath: "src/err3/typed.tsx", OutPath: "routes/err3_typed.js", ExportKey: "Err3Typed"},
		}), logger)
		vorma.NewLoader(
			app,
			"/err3/typed",
			func(*vorma.LoaderReqData) (string, error) {
				return "", &vorma.LoaderError{
					Client: "safe-client",
					Server: errors.New("server-secret-typed-error"),
				}
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/err3/typed?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		logs := logBuf.String()
		if !strings.Contains(logs, "server-secret-typed-error") {
			t.Fatalf("expected server-side typed error details in logs; got logs=%q", logs)
		}
	})

	t.Run("BRC-ERR-003_BR-ERR-004_generic_error_message_does_not_leak_server_text", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/err4/plain", SrcPath: "src/err4/plain.tsx", OutPath: "routes/err4_plain.js", ExportKey: "Err4Plain"},
		}))
		vorma.NewLoader(
			app,
			"/err4/plain",
			func(*vorma.LoaderReqData) (string, error) {
				return "", errors.New("raw server message: secret-token-123")
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/err4/plain?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		payload := mustResponseJSONMap(t, rec)
		if got, _ := payload["outermostServerError"].(string); got != "An error occurred" {
			t.Fatalf("expected outermostServerError=%q, got %#v", "An error occurred", payload["outermostServerError"])
		}

		if strings.Contains(rec.Body.String(), "secret-token-123") {
			t.Fatalf("expected response body to avoid raw server error text; body=%q", rec.Body.String())
		}
	})
}

func makeAppWithLogger(t *testing.T, rawCfg []byte, staticFS fstest.MapFS, logger *slog.Logger) *vorma.Vorma {
	t.Helper()
	w := makeWave(rawCfg, staticFS)
	return vorma.NewVormaApp(vorma.VormaAppConfig{
		Wave:   w,
		Logger: logger,
	})
}

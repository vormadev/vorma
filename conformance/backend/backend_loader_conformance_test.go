package backend_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"testing/fstest"
	"time"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/vormaruntime"
)

func TestBackendLoaderConformance(t *testing.T) {
	t.Run("BRC-LOAD-001_BR-LOAD-001_not_found_returns_404", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFS(true, true))
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/load1/missing?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})

	t.Run("BRC-LOAD-002_BR-LOAD-002_matched_patterns_are_outermost_to_innermost", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "", SrcPath: "src/load2/root.tsx", OutPath: "routes/load2_root.js", ExportKey: "Root"},
			{Pattern: "/load2/users", SrcPath: "src/load2/users.tsx", OutPath: "routes/load2_users.js", ExportKey: "Users"},
			{Pattern: "/load2/users/:id", SrcPath: "src/load2/user_id.tsx", OutPath: "routes/load2_user_id.js", ExportKey: "UserID"},
		}))
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/load2/users/42?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		payload := mustResponseJSONMap(t, rec)
		matchedPatterns := mustAnySlice(t, payload, "matchedPatterns")
		assertStringSliceEquals(t, anySliceToStrings(t, matchedPatterns), []string{"", "/load2/users", "/load2/users/:id"}, "matchedPatterns")
	})

	t.Run("BRC-LOAD-003_BR-LOAD-003_params_and_splat_are_exposed", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/load3/files/:bucket/*", SrcPath: "src/load3/files.tsx", OutPath: "routes/load3_files.js", ExportKey: "Files"},
		}))
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/load3/files/images/a/b/c?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		payload := mustResponseJSONMap(t, rec)
		params := mustAnyMap(t, payload, "params")
		if got, _ := params["bucket"].(string); got != "images" {
			t.Fatalf("expected params.bucket=%q, got %#v", "images", params["bucket"])
		}
		splatValues := mustAnySlice(t, payload, "splatValues")
		assertStringSliceEquals(t, anySliceToStrings(t, splatValues), []string{"a", "b", "c"}, "splatValues")
	})

	t.Run("BRC-LOAD-004_BR-LOAD-004_has_root_data_depends_on_root_loader_handler", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		staticFS := makeStaticFSWithPaths([]testPathDef{
			{Pattern: "", SrcPath: "src/load4/root.tsx", OutPath: "routes/load4_root.js", ExportKey: "Root"},
			{Pattern: "/load4/users/:id", SrcPath: "src/load4/user_id.tsx", OutPath: "routes/load4_user_id.js", ExportKey: "UserID"},
		})

		withRoot := makeApp(t, makeWaveConfigJSON(t, nil), staticFS)
		vorma.NewLoader(
			withRoot,
			"",
			func(*vorma.LoaderReqData) (map[string]any, error) { return map[string]any{"root": true}, nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		withRootRouter := withRoot.InitWithDefaultRouter()

		withRootRec := httptest.NewRecorder()
		withRootReq := httptest.NewRequest(http.MethodGet, "/load4/users/99?vorma_json="+testBuildID, nil)
		withRootRouter.ServeHTTP(withRootRec, withRootReq)
		if withRootRec.Code != http.StatusOK {
			t.Fatalf("with-root expected status %d, got %d; body=%q", http.StatusOK, withRootRec.Code, withRootRec.Body.String())
		}
		withRootPayload := mustResponseJSONMap(t, withRootRec)
		if got, _ := withRootPayload["hasRootData"].(bool); !got {
			t.Fatalf("expected hasRootData=true for app with root loader, got %#v", withRootPayload["hasRootData"])
		}

		withoutRoot := makeApp(t, makeWaveConfigJSON(t, nil), staticFS)
		withoutRootRouter := withoutRoot.InitWithDefaultRouter()

		withoutRootRec := httptest.NewRecorder()
		withoutRootReq := httptest.NewRequest(http.MethodGet, "/load4/users/99?vorma_json="+testBuildID, nil)
		withoutRootRouter.ServeHTTP(withoutRootRec, withoutRootReq)
		if withoutRootRec.Code != http.StatusOK {
			t.Fatalf("without-root expected status %d, got %d; body=%q", http.StatusOK, withoutRootRec.Code, withoutRootRec.Body.String())
		}
		withoutRootPayload := mustResponseJSONMap(t, withoutRootRec)
		if got, _ := withoutRootPayload["hasRootData"].(bool); got {
			t.Fatalf("expected hasRootData=false for app without root loader, got %#v", withoutRootPayload["hasRootData"])
		}
	})

	t.Run("BRC-LOAD-005_BR-LOAD-005_matched_loaders_execute_in_parallel", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		const sleepPerLoader = 90 * time.Millisecond
		const maxExpectedElapsed = 165 * time.Millisecond

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "", SrcPath: "src/load5/root.tsx", OutPath: "routes/load5_root.js", ExportKey: "Root"},
			{Pattern: "/load5/slow", SrcPath: "src/load5/slow.tsx", OutPath: "routes/load5_slow.js", ExportKey: "Slow"},
		}))
		vorma.NewLoader(
			app,
			"",
			func(*vorma.LoaderReqData) (string, error) {
				time.Sleep(sleepPerLoader)
				return "root", nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/load5/slow",
			func(*vorma.LoaderReqData) (string, error) {
				time.Sleep(sleepPerLoader)
				return "slow", nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		start := time.Now()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/load5/slow?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)
		elapsed := time.Since(start)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		if elapsed > maxExpectedElapsed {
			t.Fatalf("expected parallel execution near one sleep interval; sleep=%s elapsed=%s limit=%s", sleepPerLoader, elapsed, maxExpectedElapsed)
		}
	})

	t.Run("BRC-LOAD-006_BR-LOAD-006_route_data_arrays_are_index_aligned", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "", SrcPath: "src/load6/root.tsx", OutPath: "routes/load6_root.js", ExportKey: "Root"},
			{Pattern: "/load6/users", SrcPath: "src/load6/users.tsx", OutPath: "routes/load6_users.js", ExportKey: "Users"},
			{Pattern: "/load6/users/:id", SrcPath: "src/load6/user_id.tsx", OutPath: "routes/load6_user_id.js", ExportKey: "UserID"},
		}))
		vorma.NewLoader(
			app,
			"",
			func(*vorma.LoaderReqData) (string, error) { return "root-data", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/load6/users/:id",
			func(*vorma.LoaderReqData) (string, error) { return "leaf-data", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/load6/users/7?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		payload := mustResponseJSONMap(t, rec)
		matchedPatterns := mustAnySlice(t, payload, "matchedPatterns")
		loadersData := mustAnySlice(t, payload, "loadersData")
		importURLs := mustAnySlice(t, payload, "importURLs")
		exportKeys := mustAnySlice(t, payload, "exportKeys")
		errorExportKeys := mustAnySlice(t, payload, "errorExportKeys")

		n := len(matchedPatterns)
		if len(loadersData) != n || len(importURLs) != n || len(exportKeys) != n || len(errorExportKeys) != n {
			t.Fatalf(
				"expected aligned lengths: matchedPatterns=%d loadersData=%d importURLs=%d exportKeys=%d errorExportKeys=%d",
				n, len(loadersData), len(importURLs), len(exportKeys), len(errorExportKeys),
			)
		}
	})

	t.Run("BRC-LOAD-007_BR-LOAD-007_client_only_slots_are_not_compacted", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "", SrcPath: "src/load7/root.tsx", OutPath: "routes/load7_root.js", ExportKey: "Root"},
			{Pattern: "/load7/users", SrcPath: "src/load7/users.tsx", OutPath: "routes/load7_users.js", ExportKey: "Users"},
			{Pattern: "/load7/users/:id", SrcPath: "src/load7/user_id.tsx", OutPath: "routes/load7_user_id.js", ExportKey: "UserID"},
		}))
		vorma.NewLoader(
			app,
			"",
			func(*vorma.LoaderReqData) (string, error) { return "root-data", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/load7/users/:id",
			func(*vorma.LoaderReqData) (string, error) { return "leaf-data", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/load7/users/7?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		payload := mustResponseJSONMap(t, rec)
		patterns := anySliceToStrings(t, mustAnySlice(t, payload, "matchedPatterns"))
		loadersData := mustAnySlice(t, payload, "loadersData")
		importURLs := anySliceToStrings(t, mustAnySlice(t, payload, "importURLs"))
		exportKeys := anySliceToStrings(t, mustAnySlice(t, payload, "exportKeys"))

		clientOnlyIdx := indexOfString(patterns, "/load7/users")
		if clientOnlyIdx < 0 {
			t.Fatalf("expected matchedPatterns to include client-only route %q, got %v", "/load7/users", patterns)
		}
		if clientOnlyIdx >= len(loadersData) {
			t.Fatalf("client-only index %d out of bounds for loadersData len=%d", clientOnlyIdx, len(loadersData))
		}
		if loadersData[clientOnlyIdx] != nil {
			t.Fatalf("expected loadersData[%d] to be nil for client-only route, got %#v", clientOnlyIdx, loadersData[clientOnlyIdx])
		}

		if clientOnlyIdx >= len(importURLs) || importURLs[clientOnlyIdx] == "" {
			t.Fatalf("expected importURLs[%d] to be non-empty for client-only route slot, got %v", clientOnlyIdx, importURLs)
		}
		if clientOnlyIdx >= len(exportKeys) || exportKeys[clientOnlyIdx] == "" {
			t.Fatalf("expected exportKeys[%d] to be non-empty for client-only route slot, got %v", clientOnlyIdx, exportKeys)
		}
	})
}

type testPathDef struct {
	Pattern        string
	SrcPath        string
	OutPath        string
	ExportKey      string
	ErrorExportKey string
}

func makeStaticFSWithPaths(defs []testPathDef) fstest.MapFS {
	m := makeStaticFS(false, true)

	paths := make(map[string]any, len(defs))
	for _, def := range defs {
		exportKey := def.ExportKey
		if exportKey == "" {
			exportKey = "default"
		}
		paths[def.Pattern] = map[string]any{
			"originalPattern": def.Pattern,
			"srcPath":         def.SrcPath,
			"exportKey":       exportKey,
			"errorExportKey":  def.ErrorExportKey,
			"outPath":         def.OutPath,
			"deps":            []string{},
		}
	}

	stagePayload := map[string]any{
		"buildID":           testBuildID,
		"clientEntrySrc":    "frontend/src/vorma.entry.tsx",
		"paths":             paths,
		"routeManifestFile": "vorma_out/route_manifest.json",
		"clientEntryOut":    "vorma_out/client.js",
		"clientEntryDeps":   []string{},
		"depToCSSBundleMap": map[string][]string{},
	}
	stageOnePayload := make(map[string]any, len(stagePayload)+1)
	stageTwoPayload := make(map[string]any, len(stagePayload)+1)
	for k, v := range stagePayload {
		stageOnePayload[k] = v
		stageTwoPayload[k] = v
	}
	stageOnePayload["stage"] = "1"
	stageTwoPayload["stage"] = "2"

	m[path.Join("assets/private", vormaruntime.VormaOutDirname, vormaruntime.VormaPathsStageOneJSONFileName)] = &fstest.MapFile{
		Data: mustMarshalJSON(nil, stageOnePayload),
	}
	m[path.Join("assets/private", vormaruntime.VormaOutDirname, vormaruntime.VormaPathsStageTwoJSONFileName)] = &fstest.MapFile{
		Data: mustMarshalJSON(nil, stageTwoPayload),
	}

	return m
}

func mustResponseJSONMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON body, got err=%v body=%q", err, rec.Body.String())
	}
	return payload
}

func mustAnySlice(t *testing.T, payload map[string]any, key string) []any {
	t.Helper()
	raw, ok := payload[key]
	if !ok {
		t.Fatalf("expected payload key %q, got %#v", key, payload)
	}
	slice, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected payload[%q] to be []any, got %#v", key, raw)
	}
	return slice
}

func mustAnyMap(t *testing.T, payload map[string]any, key string) map[string]any {
	t.Helper()
	raw, ok := payload[key]
	if !ok {
		t.Fatalf("expected payload key %q, got %#v", key, payload)
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected payload[%q] to be map[string]any, got %#v", key, raw)
	}
	return obj
}

func anySliceToStrings(t *testing.T, in []any) []string {
	t.Helper()
	out := make([]string, len(in))
	for i, raw := range in {
		s, ok := raw.(string)
		if !ok {
			t.Fatalf("expected string at index %d, got %#v", i, raw)
		}
		out[i] = s
	}
	return out
}

func assertStringSliceEquals(t *testing.T, got []string, want []string, label string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %s len=%d, got len=%d (%v)", label, len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %s[%d]=%q, got %q (%v)", label, i, want[i], got[i], got)
		}
	}
}

func indexOfString(values []string, target string) int {
	for i, v := range values {
		if v == target {
			return i
		}
	}
	return -1
}

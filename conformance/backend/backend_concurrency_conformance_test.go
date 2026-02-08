package backend_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/vormaruntime"
)

func TestBackendConcurrencyConformance(t *testing.T) {
	t.Run("BRC-CONC-001_BR-CONC-001_mixed_loader_action_traffic_remains_well_formed", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/conc1/items/:id", SrcPath: "src/conc1/item.tsx", OutPath: "routes/conc1_item.js", ExportKey: "Item"},
		}))
		vorma.NewLoader(
			app,
			"/conc1/items/:id",
			func(rd *vorma.LoaderReqData) (map[string]any, error) {
				return map[string]any{"id": rd.Param("id")}, nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewAction[vorma.None, string](
			app,
			http.MethodPost,
			"/conc1/ping",
			func(*vorma.ActionReqData[vorma.None]) (string, error) { return "pong", nil },
			func(rd *vorma.ActionReqData[vorma.None]) *vorma.ActionReqData[vorma.None] { return rd },
		)
		r := app.InitWithDefaultRouter()

		const total = 120
		errs := make(chan error, total)
		var wg sync.WaitGroup

		for i := 0; i < total; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				if i%2 == 0 {
					id := fmt.Sprintf("%d", i%17)
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodGet, "/conc1/items/"+id+"?vorma_json="+testBuildID, nil)
					r.ServeHTTP(rec, req)

					if rec.Code != http.StatusOK {
						errs <- fmt.Errorf("loader request %d expected 200 got %d body=%q", i, rec.Code, rec.Body.String())
						return
					}

					var payload map[string]any
					if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
						errs <- fmt.Errorf("loader request %d invalid JSON: %w body=%q", i, err, rec.Body.String())
						return
					}

					paramsRaw, ok := payload["params"].(map[string]any)
					if !ok {
						errs <- fmt.Errorf("loader request %d missing params map in payload=%#v", i, payload)
						return
					}
					if got, _ := paramsRaw["id"].(string); got != id {
						errs <- fmt.Errorf("loader request %d expected params.id=%q got %#v", i, id, paramsRaw["id"])
						return
					}

					matched, ok := payload["matchedPatterns"].([]any)
					if !ok || len(matched) == 0 {
						errs <- fmt.Errorf("loader request %d missing matchedPatterns in payload=%#v", i, payload)
						return
					}
					return
				}

				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/conc1/ping", nil)
				r.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					errs <- fmt.Errorf("action request %d expected 200 got %d body=%q", i, rec.Code, rec.Body.String())
					return
				}

				var out string
				if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
					errs <- fmt.Errorf("action request %d invalid JSON: %w body=%q", i, err, rec.Body.String())
					return
				}
				if out != "pong" {
					errs <- fmt.Errorf("action request %d expected %q got %q", i, "pong", out)
				}
			}()
		}

		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("concurrency failure: %v", err)
			}
		}
	})

	t.Run("BRC-CONC-002_BR-CONC-002_concurrent_traffic_and_reload_routes_preserve_alignment", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "development")

		defs := []testPathDef{
			{Pattern: "", SrcPath: "src/conc2/root.tsx", OutPath: "routes/conc2_root.js", ExportKey: "Root"},
			{Pattern: "/conc2/page", SrcPath: "src/conc2/page.tsx", OutPath: "routes/conc2_page.js", ExportKey: "Page"},
			{Pattern: "/conc2/page/:id", SrcPath: "src/conc2/page_id.tsx", OutPath: "routes/conc2_page_id.js", ExportKey: "PageID"},
		}
		distDir := makeDevDistDirWithPaths(t, defs)
		cfg := makeWaveConfigJSONWithCoreMutator(t, func(core map[string]any) {
			core["DistDir"] = distDir
		})

		app := makeApp(t, cfg, nil)
		vorma.NewLoader(
			app,
			"",
			func(*vorma.LoaderReqData) (string, error) { return "root-ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/conc2/page/:id",
			func(rd *vorma.LoaderReqData) (map[string]any, error) {
				return map[string]any{"id": rd.Param("id")}, nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		errs := make(chan error, 512)
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				writeDevStageOnePathsFile(t, distDir, testBuildID, defs)

				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, vormaruntime.DevReloadRoutesPath, nil)
				r.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					errs <- fmt.Errorf("reload-routes iteration %d expected 200 got %d body=%q", i, rec.Code, rec.Body.String())
				}
			}
		}()

		for worker := 0; worker < 4; worker++ {
			worker := worker
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 40; i++ {
					id := fmt.Sprintf("%d", (worker*40+i)%23)
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodGet, "/conc2/page/"+id+"?vorma_json="+testBuildID, nil)
					r.ServeHTTP(rec, req)

					if rec.Code != http.StatusOK {
						errs <- fmt.Errorf("traffic request worker=%d i=%d expected 200 got %d body=%q", worker, i, rec.Code, rec.Body.String())
						continue
					}

					var payload map[string]any
					if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
						errs <- fmt.Errorf("traffic request worker=%d i=%d invalid JSON: %w body=%q", worker, i, err, rec.Body.String())
						continue
					}

					matched, ok := payload["matchedPatterns"].([]any)
					if !ok {
						errs <- fmt.Errorf("traffic request worker=%d i=%d missing matchedPatterns payload=%#v", worker, i, payload)
						continue
					}
					loadersData, ok := payload["loadersData"].([]any)
					if !ok {
						errs <- fmt.Errorf("traffic request worker=%d i=%d missing loadersData payload=%#v", worker, i, payload)
						continue
					}
					importURLs, ok := payload["importURLs"].([]any)
					if !ok {
						errs <- fmt.Errorf("traffic request worker=%d i=%d missing importURLs payload=%#v", worker, i, payload)
						continue
					}
					exportKeys, ok := payload["exportKeys"].([]any)
					if !ok {
						errs <- fmt.Errorf("traffic request worker=%d i=%d missing exportKeys payload=%#v", worker, i, payload)
						continue
					}
					errorExportKeys, ok := payload["errorExportKeys"].([]any)
					if !ok {
						errs <- fmt.Errorf("traffic request worker=%d i=%d missing errorExportKeys payload=%#v", worker, i, payload)
						continue
					}

					n := len(matched)
					if len(loadersData) != n || len(importURLs) != n || len(exportKeys) != n || len(errorExportKeys) != n {
						errs <- fmt.Errorf(
							"traffic request worker=%d i=%d misaligned payload lens matched=%d loadersData=%d importURLs=%d exportKeys=%d errorExportKeys=%d",
							worker, i, n, len(loadersData), len(importURLs), len(exportKeys), len(errorExportKeys),
						)
						continue
					}
				}
			}()
		}

		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("concurrency+reload failure: %v", err)
			}
		}
	})
}

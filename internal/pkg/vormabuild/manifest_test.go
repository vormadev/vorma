package vormabuild

import (
	"testing"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/internal/pkg/viteutil"
)

func TestToClientCoreAssetsFindsWrapperAndWasmFromViteManifest(t *testing.T) {
	cfg := vorma_cfg{C: &vorma.Config{
		PathConfig: vorma.PathConfig{
			PublicStaticBase: "/static/",
		},
	}}
	module_out := "vorma_out_vite_vorma_client_wasm.js"
	wasm_src := "../../pkg/npm/.dist/vorma/core/" + client_core_wasm_source_filename
	wasm_out := "vorma_out_vite_" + client_core_wasm_source_filename
	manifest := viteutil.ViteManifest{
		"../../pkg/npm/.dist/vorma/core/vorma_client_wasm-BiuOZDrD.js": {
			File:           module_out,
			Src:            "../../pkg/npm/.dist/vorma/core/vorma_client_wasm-BiuOZDrD.js",
			IsDynamicEntry: true,
			Assets:         []string{wasm_out},
		},
		wasm_src: {
			File: wasm_out,
			Src:  wasm_src,
		},
		"node_modules/mdream/wasm/mdream_edge_bg.wasm": {
			File: "vorma_out_vite_mdream_edge_bg.wasm",
			Src:  "node_modules/mdream/wasm/mdream_edge_bg.wasm",
		},
	}

	got := cfg.to_client_core_assets(manifest)
	if got == nil {
		t.Fatal("expected client core assets")
	}
	if got.ModuleURL != "/static/"+module_out {
		t.Fatalf("expected module URL %q, got %q", "/static/"+module_out, got.ModuleURL)
	}
	if got.WasmURL != "/static/"+wasm_out {
		t.Fatalf("expected wasm URL %q, got %q", "/static/"+wasm_out, got.WasmURL)
	}
}

func TestToClientCoreAssetsIgnoresUnrelatedWasmAssets(t *testing.T) {
	cfg := vorma_cfg{C: &vorma.Config{}}
	manifest := viteutil.ViteManifest{
		"node_modules/mdream/wasm/mdream_edge_bg.wasm": {
			File: "vorma_out_vite_mdream_edge_bg.wasm",
			Src:  "node_modules/mdream/wasm/mdream_edge_bg.wasm",
		},
	}

	got := cfg.to_client_core_assets(manifest)
	if got != nil {
		t.Fatalf("expected no client core assets, got %#v", got)
	}
}

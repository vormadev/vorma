package build_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildViteIntegrationConformance(t *testing.T) {
	t.Run("BDC-VITE-001_BUILD-VITE-001_generated_rollup_input_includes_client_entry_and_all_route_sources", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("dev hook build failed: err=%v output=%s", err, out)
		}

		indexPath := filepath.Join(fixture.tsGenDir, "index.ts")
		b, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatalf("read generated TS index: %v", err)
		}
		content := string(b)

		requiredEntrypoints := []string{
			`"frontend/src/vorma.entry.tsx"`,
			`"frontend/src/routes/root.tsx"`,
			`"frontend/src/routes/home.tsx"`,
			`"frontend/src/routes/default_key.tsx"`,
		}
		for _, entry := range requiredEntrypoints {
			if !strings.Contains(content, entry) {
				t.Fatalf("expected generated rollup input to include %q, but it was missing", entry)
			}
		}
	})

	t.Run("BDC-VITE-002_BUILD-VITE-002_dedupe_list_matches_ui_variant_family", func(t *testing.T) {
		testCases := []struct {
			name      string
			variant   string
			expectAny []string
		}{
			{
				name:    "react",
				variant: "react",
				expectAny: []string{
					`"react"`,
					`"react-dom"`,
				},
			},
			{
				name:    "preact",
				variant: "preact",
				expectAny: []string{
					`"preact"`,
					`"preact/hooks"`,
					`"preact/jsx-runtime"`,
				},
			},
			{
				name:    "solid",
				variant: "solid",
				expectAny: []string{
					`"solid-js"`,
					`"solid-js/web"`,
				},
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				fixture := newBuildFixture(t, nil)
				mutateFixtureConfig(t, fixture, func(cfg map[string]any) {
					vormaCfg, ok := cfg["Vorma"].(map[string]any)
					if !ok {
						t.Fatalf("expected Vorma config map, got %#v", cfg["Vorma"])
					}
					vormaCfg["UIVariant"] = tc.variant
				})

				out, err := runBuildProbe(t, fixture, "--hook", "--dev")
				if err != nil {
					t.Fatalf("dev hook build failed: err=%v output=%s", err, out)
				}

				indexPath := filepath.Join(fixture.tsGenDir, "index.ts")
				b, err := os.ReadFile(indexPath)
				if err != nil {
					t.Fatalf("read generated TS index: %v", err)
				}
				content := string(b)

				for _, expect := range tc.expectAny {
					if !strings.Contains(content, expect) {
						t.Fatalf("expected dedupe list for variant %q to include %q", tc.variant, expect)
					}
				}
			})
		}
	})

	t.Run("BDC-VITE-003_BUILD-VITE-003_vite_output_naming_uses_required_vorma_prefix", func(t *testing.T) {
		out, err := runVitePluginNodeProbe(t, `
import vormaVitePlugin from "./internal/framework/_typescript/vite/vite.ts";
const plugin = vormaVitePlugin({
	rollupInput: [],
	publicPathPrefix: "/",
	staticPublicAssetMap: {},
	buildtimePublicURLFuncName: "waveBuildtimeURL",
	filemapJSONPath: "frontend/src/vorma.gen/filemap.json",
	ignoredPatterns: [],
	dedupeList: [],
});
const cfg = plugin.config({}, { command: "build" });
const output = cfg?.build?.rollupOptions?.output ?? {};
console.log(JSON.stringify({
	assetFileNames: output.assetFileNames ?? "",
	chunkFileNames: output.chunkFileNames ?? "",
	entryFileNames: output.entryFileNames ?? "",
}));
`)
		if err != nil {
			t.Fatalf("vite plugin node probe failed: err=%v output=%s", err, out)
		}
		payload := mustUnmarshalJSONStringMap(t, out, "vite output naming payload")

		for _, key := range []string{"assetFileNames", "chunkFileNames", "entryFileNames"} {
			value, _ := payload[key].(string)
			if !strings.HasPrefix(value, "vorma_out_vite_") {
				t.Fatalf("expected %s to start with vorma_out_vite_, got %q", key, value)
			}
		}
	})

	t.Run("BDC-VITE-004_BUILD-VITE-004_buildtime_public_url_calls_rewrite_using_prefixed_hashed_url", func(t *testing.T) {
		out, err := runVitePluginNodeProbe(t, `
import vormaVitePlugin from "./internal/framework/_typescript/vite/vite.ts";
const plugin = vormaVitePlugin({
	rollupInput: [],
	publicPathPrefix: "/static/",
	staticPublicAssetMap: {
		"logo.svg": "vorma_out_vite_logo-abc123.svg",
	},
	buildtimePublicURLFuncName: "waveBuildtimeURL",
	filemapJSONPath: "frontend/src/vorma.gen/filemap.json",
	ignoredPatterns: [],
	dedupeList: [],
});
plugin.config({}, { command: "build" });
const source = [
	'const a = waveBuildtimeURL("logo.svg");',
	'const b = waveBuildtimeURL("missing.svg");',
].join("\n");
const transformed = plugin.transform(source, "/src/app.ts");
console.log(JSON.stringify({ transformed }));
`)
		if err != nil {
			t.Fatalf("vite plugin node probe failed: err=%v output=%s", err, out)
		}
		payload := mustUnmarshalJSONStringMap(t, out, "vite transform payload")
		transformed, _ := payload["transformed"].(string)
		if !strings.Contains(transformed, `"/static/vorma_out_vite_logo-abc123.svg"`) {
			t.Fatalf("expected transformed code to include rewritten prefixed hashed URL, got %q", transformed)
		}
		if !strings.Contains(transformed, `"missing.svg"`) {
			t.Fatalf("expected unknown asset to remain original path literal, got %q", transformed)
		}
	})

	t.Run("BDC-VITE-005_BUILD-VITE-005_filemap_invalidation_endpoint_returns_ok_invalidates_modules_and_triggers_full_reload", func(t *testing.T) {
		out, err := runVitePluginNodeProbe(t, `
import vormaVitePlugin from "./internal/framework/_typescript/vite/vite.ts";
const plugin = vormaVitePlugin({
	rollupInput: [],
	publicPathPrefix: "/",
	staticPublicAssetMap: {},
	buildtimePublicURLFuncName: "waveBuildtimeURL",
	filemapJSONPath: "frontend/src/vorma.gen/filemap.json",
	ignoredPatterns: [],
	dedupeList: [],
});
let middleware = null;
const invalidated = [];
const wsMessages = [];
const server = {
	middlewares: {
		use(fn) {
			middleware = fn;
		},
	},
	moduleGraph: {
		idToModuleMap: new Map([
			["mod1", { id: "mod1" }],
			["mod2", { id: "mod2" }],
		]),
		invalidateModule(mod) {
			invalidated.push(mod.id);
		},
	},
	ws: {
		send(msg) {
			wsMessages.push(msg);
		},
	},
};
plugin.configureServer(server);
if (typeof middleware !== "function") {
	throw new Error("middleware was not registered");
}
let nextCalled = false;
let body = "";
const res = {
	statusCode: 0,
	end(v) {
		body = String(v ?? "");
	},
};
const oldLog = console.log;
console.log = () => {};
middleware({ url: "/__vorma_invalidate_filemap" }, res, () => {
	nextCalled = true;
});
console.log = oldLog;
console.log(JSON.stringify({
	statusCode: res.statusCode,
	body,
	nextCalled,
	invalidated,
	wsMessages,
}));
`)
		if err != nil {
			t.Fatalf("vite plugin node probe failed: err=%v output=%s", err, out)
		}
		payload := mustUnmarshalJSONStringMap(t, out, "vite invalidation payload")

		status, _ := payload["statusCode"].(float64)
		if int(status) != 200 {
			t.Fatalf("expected invalidation endpoint statusCode=200, got %#v", payload["statusCode"])
		}
		if got, _ := payload["body"].(string); got != "ok" {
			t.Fatalf("expected invalidation endpoint body=%q, got %#v", "ok", payload["body"])
		}
		if got, _ := payload["nextCalled"].(bool); got {
			t.Fatalf("expected invalidation request to be handled without next(), got nextCalled=true")
		}
		invalidated := watchAnySlice(t, payload["invalidated"], "invalidated")
		if len(invalidated) != 2 {
			t.Fatalf("expected both modules to be invalidated, got %#v", payload["invalidated"])
		}
		wsMessages := watchAnySlice(t, payload["wsMessages"], "wsMessages")
		if len(wsMessages) == 0 {
			t.Fatalf("expected ws full reload message to be sent, got %#v", payload["wsMessages"])
		}
		firstMsg := watchAnyMap(t, wsMessages[0], "wsMessages[0]")
		if got, _ := firstMsg["type"].(string); got != "full-reload" {
			t.Fatalf("expected first ws message type=%q, got %#v", "full-reload", firstMsg["type"])
		}
	})
}

func runVitePluginNodeProbe(t *testing.T, script string) (string, error) {
	t.Helper()
	root := repoRoot(t)
	tempDir := t.TempDir()
	bundlePath := filepath.Join(tempDir, "vorma_vite_probe_bundle.mjs")

	esbuildPath := filepath.Join(root, "node_modules", "esbuild", "bin", "esbuild")
	bundleCmd := exec.Command(
		esbuildPath,
		"./internal/framework/_typescript/vite/vite.ts",
		"--bundle",
		"--platform=node",
		"--format=esm",
		"--outfile="+bundlePath,
	)
	bundleCmd.Dir = root
	bundleOut, bundleErr := bundleCmd.CombinedOutput()
	if bundleErr != nil {
		return string(bundleOut), fmt.Errorf(
			"esbuild source probe bundle failed: %w",
			bundleErr,
		)
	}

	fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(bundlePath)}).String()
	script = strings.ReplaceAll(
		script,
		"./internal/framework/_typescript/vite/vite.ts",
		fileURL,
	)

	cmd := exec.Command("node", "--input-type=module", "-e", script)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func mustUnmarshalJSONStringMap(t *testing.T, raw string, field string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &m); err != nil {
		t.Fatalf("unmarshal %s: %v body=%q", field, err, raw)
	}
	return m
}

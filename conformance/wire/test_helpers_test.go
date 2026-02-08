package wire_test

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

const (
	wireBuildID      = "b123"
	wireTemplatePath = "index.html"
)

type wirePathDef struct {
	Pattern        string
	SrcPath        string
	OutPath        string
	ExportKey      string
	ErrorExportKey string
	Deps           []string
}

type wireStaticOpts struct {
	TemplateContent     string
	RouteManifestFile   string
	RouteManifestJSON   []byte
	IncludeManifestFile bool
}

func makeWireApp(
	t *testing.T,
	rawCfg []byte,
	staticFS fstest.MapFS,
	mutate func(*vorma.VormaAppConfig),
) *vorma.Vorma {
	t.Helper()

	cfg := vorma.VormaAppConfig{
		Wave: makeWireWave(rawCfg, staticFS),
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return vorma.NewVormaApp(cfg)
}

func makeWireWave(rawCfg []byte, staticFS fstest.MapFS) *wave.Wave {
	return wave.New(wave.Config{
		WaveConfigJSON: rawCfg,
		DistStaticFS:   staticFS,
	})
}

func makeWireConfigJSON(
	t *testing.T,
	mutateCore func(map[string]any),
	mutateVorma func(map[string]any),
) []byte {
	t.Helper()

	core := map[string]any{
		"MainAppEntry": "cmd/app/main.go",
		"DistDir":      "dist",
		"StaticAssetDirs": map[string]any{
			"Private": "private",
			"Public":  "public",
		},
	}
	if mutateCore != nil {
		mutateCore(core)
	}

	vormaCfg := map[string]any{
		"MainBuildEntry":       "cmd/app/main.go",
		"UIVariant":            "react",
		"HTMLTemplateLocation": wireTemplatePath,
		"ClientEntry":          "frontend/src/vorma.entry.tsx",
		"ClientRouteDefsFile":  "frontend/src/vorma.routes.ts",
		"TSGenOutDir":          "frontend/src/vorma.gen",
	}
	if mutateVorma != nil {
		mutateVorma(vormaCfg)
	}

	cfg := map[string]any{
		"Core":  core,
		"Vorma": vormaCfg,
	}
	return mustJSON(t, cfg)
}

func makeWireStaticFSWithPaths(defs []wirePathDef, opts *wireStaticOpts) fstest.MapFS {
	m := fstest.MapFS{}

	if opts == nil {
		opts = &wireStaticOpts{}
	}
	templateContent := opts.TemplateContent
	if templateContent == "" {
		templateContent = `<!doctype html><html><head>{{.VormaHeadEls}}</head><body><div id="{{.VormaRootID}}"></div>{{.VormaBodyScripts}}{{.VormaSSRScript}}</body></html>`
	}
	manifestFile := opts.RouteManifestFile
	if manifestFile == "" {
		manifestFile = "vorma_out_vorma_internal_route_manifest_test.json"
	}
	manifestJSON := opts.RouteManifestJSON
	if len(manifestJSON) == 0 {
		manifestJSON = []byte(`{}`)
	}

	m[path.Join("assets/private", wireTemplatePath)] = &fstest.MapFile{
		Data: []byte(templateContent),
	}
	if opts.IncludeManifestFile {
		m[path.Join("assets/public", manifestFile)] = &fstest.MapFile{
			Data: manifestJSON,
		}
	}

	paths := make(map[string]any, len(defs))
	for _, def := range defs {
		exportKey := def.ExportKey
		if exportKey == "" {
			exportKey = "default"
		}
		deps := def.Deps
		if deps == nil {
			deps = []string{}
		}
		paths[def.Pattern] = map[string]any{
			"originalPattern": def.Pattern,
			"srcPath":         def.SrcPath,
			"exportKey":       exportKey,
			"errorExportKey":  def.ErrorExportKey,
			"outPath":         def.OutPath,
			"deps":            deps,
		}
	}

	stageBase := map[string]any{
		"buildID":           wireBuildID,
		"clientEntrySrc":    "frontend/src/vorma.entry.tsx",
		"paths":             paths,
		"routeManifestFile": manifestFile,
		"clientEntryOut":    "vorma_out/client.js",
		"clientEntryDeps":   []string{},
		"depToCSSBundleMap": map[string][]string{},
	}
	stage1 := cloneMap(stageBase)
	stage2 := cloneMap(stageBase)
	stage1["stage"] = "1"
	stage2["stage"] = "2"

	m[path.Join("assets/private", vormaruntime.VormaOutDirname, vormaruntime.VormaPathsStageOneJSONFileName)] = &fstest.MapFile{
		Data: mustJSON(nil, stage1),
	}
	m[path.Join("assets/private", vormaruntime.VormaOutDirname, vormaruntime.VormaPathsStageTwoJSONFileName)] = &fstest.MapFile{
		Data: mustJSON(nil, stage2),
	}

	return m
}

func makeWireDevDistDirWithPaths(t *testing.T, defs []wirePathDef, opts *wireStaticOpts) string {
	t.Helper()

	if opts == nil {
		opts = &wireStaticOpts{}
	}
	templateContent := opts.TemplateContent
	if templateContent == "" {
		templateContent = `<!doctype html><html><head>{{.VormaHeadEls}}</head><body><div id="{{.VormaRootID}}"></div>{{.VormaBodyScripts}}{{.VormaSSRScript}}</body></html>`
	}
	manifestFile := opts.RouteManifestFile
	if manifestFile == "" {
		manifestFile = "vorma_out_vorma_internal_route_manifest_test.json"
	}
	manifestJSON := opts.RouteManifestJSON
	if len(manifestJSON) == 0 {
		manifestJSON = []byte(`{}`)
	}

	distDir := filepath.Join(t.TempDir(), "dist")
	privateDir := filepath.Join(distDir, "static", "assets", "private")
	publicDir := filepath.Join(distDir, "static", "assets", "public")
	if err := os.MkdirAll(filepath.Join(privateDir, vormaruntime.VormaOutDirname), 0o755); err != nil {
		t.Fatalf("mkdir private dirs: %v", err)
	}
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("mkdir public dirs: %v", err)
	}

	if err := os.WriteFile(filepath.Join(privateDir, wireTemplatePath), []byte(templateContent), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	if opts.IncludeManifestFile {
		if err := os.WriteFile(filepath.Join(publicDir, manifestFile), manifestJSON, 0o644); err != nil {
			t.Fatalf("write route manifest: %v", err)
		}
	}

	writeWireDevStagePathsFile(t, distDir, "1", wireBuildID, defs, manifestFile)
	writeWireDevStagePathsFile(t, distDir, "2", wireBuildID, defs, manifestFile)

	return distDir
}

func writeWireDevStagePathsFile(
	t *testing.T,
	distDir string,
	stage string,
	buildID string,
	defs []wirePathDef,
	manifestFile string,
) {
	t.Helper()

	paths := make(map[string]any, len(defs))
	for _, def := range defs {
		exportKey := def.ExportKey
		if exportKey == "" {
			exportKey = "default"
		}
		deps := def.Deps
		if deps == nil {
			deps = []string{}
		}
		paths[def.Pattern] = map[string]any{
			"originalPattern": def.Pattern,
			"srcPath":         def.SrcPath,
			"exportKey":       exportKey,
			"errorExportKey":  def.ErrorExportKey,
			"outPath":         def.OutPath,
			"deps":            deps,
		}
	}

	payload := map[string]any{
		"stage":             stage,
		"buildID":           buildID,
		"clientEntrySrc":    "frontend/src/vorma.entry.tsx",
		"paths":             paths,
		"routeManifestFile": manifestFile,
		"clientEntryOut":    "vorma_out/client.js",
		"clientEntryDeps":   []string{},
		"depToCSSBundleMap": map[string][]string{},
	}

	fileName := vormaruntime.VormaPathsStageTwoJSONFileName
	if stage == "1" {
		fileName = vormaruntime.VormaPathsStageOneJSONFileName
	}
	pathToWrite := filepath.Join(distDir, "static", "assets", "private", vormaruntime.VormaOutDirname, fileName)
	if err := os.WriteFile(pathToWrite, mustJSON(nil, payload), 0o644); err != nil {
		t.Fatalf("write stage%s paths file: %v", stage, err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	if t != nil {
		t.Helper()
	}
	b, err := json.Marshal(v)
	if err != nil {
		if t != nil {
			t.Fatalf("marshal json: %v", err)
		}
		panic(err)
	}
	return b
}

func mustJSONMap(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal json map: %v body=%q", err, string(b))
	}
	return m
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

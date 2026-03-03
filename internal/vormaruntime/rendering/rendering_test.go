package rendering

import (
	"github.com/vormadev/vorma/internal/testhelpers/waveoutputtest"
	"html/template"
	"strings"
	"testing"
)

func TestBuildSSRInnerHTML_RendersScriptAndHash(t *testing.T) {
	output, buildError := BuildSSRInnerHTML(
		SSRInnerHTMLInput{
			VormaSymbolStr:          "__vorma_internal__",
			IsDev:                   true,
			ViteDevURL:              `"http://localhost:5173"`,
			BuildID:                 `"build-123"`,
			RootElementID:           "root-id",
			PublicPathPrefix:        "/assets/",
			RouteManifestURL:        "/assets/route-manifest.json",
			OutermostServerError:    `""`,
			ErrorExportKeys:         []string{"default"},
			MatchedPatterns:         []string{"/"},
			LoadersDataJSON:         template.JS(`[{"ok":true}]`),
			ImportURLs:              []string{"/assets/home.js"},
			ExportKeys:              []string{"default"},
			HasRootData:             true,
			Params:                  map[string]string{"id": "7"},
			SplatValues:             []string{"rest"},
			DeploymentID:            `"dep-123"`,
			OutermostServerErrorIdx: nil,
		},
	)
	if buildError != nil {
		t.Fatalf("BuildSSRInnerHTML returned error: %v", buildError)
	}
	if output == nil || output.Script == nil {
		t.Fatal("expected non-nil output script")
	}
	if strings.TrimSpace(output.Sha256Hash) == "" {
		t.Fatal("expected non-empty SHA-256 hash")
	}
	script := string(*output.Script)
	if !strings.Contains(script, `type="module"`) {
		t.Fatalf("expected module script tag, got %q", script)
	}
	if !strings.Contains(script, `x.runtimeRouteSnapshot = {`) {
		t.Fatalf(
			"expected runtime route snapshot bootstrap in script, got %q",
			script,
		)
	}
	if !strings.Contains(script, `rootElementID: "root-id",`) {
		t.Fatalf(
			"expected root element id in runtime route snapshot bootstrap, got %q",
			script,
		)
	}
	if !strings.Contains(
		script,
		`x.routeManifestURL = "/assets/route-manifest.json";`,
	) {
		t.Fatalf(
			"expected route manifest URL assignment in script, got %q",
			script,
		)
	}
	if strings.Contains(script, "metaHeadEls:") {
		t.Fatalf("expected SSR snapshot to omit metaHeadEls payload, got %q", script)
	}
	if strings.Contains(script, "restHeadEls:") {
		t.Fatalf("expected SSR snapshot to omit restHeadEls payload, got %q", script)
	}
}

func TestBuildSSRInnerHTMLFromRuntimeState_NormalizesNilParamsAndSplatValues(
	t *testing.T,
) {
	output, buildError := BuildSSRInnerHTMLFromRuntimeState(
		SSRRuntimeState{
			VormaSymbolStr:    "__vorma_internal__",
			BuildID:           `"build-123"`,
			RootElementID:     "root-id",
			PublicPathPrefix:  "/assets/",
			RouteManifestFile: "route-manifest.json",
		},
		SSRRouteData{
			ViteDevURL:              `"http://localhost:5173"`,
			OutermostServerError:    `""`,
			OutermostServerErrorIdx: nil,
			ErrorExportKeys:         []string{""},
			MatchedPatterns:         []string{"/"},
			LoadersData:             []any{map[string]any{"ok": true}},
			ImportURLs:              []string{"/assets/home.js"},
			ExportKeys:              []string{"default"},
			HasRootData:             true,
			Params:                  nil,
			SplatValues:             nil,
		},
	)
	if buildError != nil {
		t.Fatalf("BuildSSRInnerHTMLFromRuntimeState returned error: %v", buildError)
	}
	if output == nil || output.Script == nil {
		t.Fatal("expected non-nil output script")
	}
	script := string(*output.Script)
	if !strings.Contains(script, `params: {},`) {
		t.Fatalf("expected params object in script, got %q", script)
	}
	if !strings.Contains(script, `splatValues: [],`) {
		t.Fatalf("expected splatValues array in script, got %q", script)
	}
}

func TestBuildSSRInnerHTML_UsesVercelDeploymentIDWhenEnabled(t *testing.T) {
	t.Setenv("VERCEL_SKEW_PROTECTION_ENABLED", "true")
	t.Setenv("VERCEL_DEPLOYMENT_ID", "dep-from-env")

	output, buildError := BuildSSRInnerHTML(
		SSRInnerHTMLInput{
			VormaSymbolStr:       "__vorma_internal__",
			BuildID:              `"build"`,
			RootElementID:        "root",
			PublicPathPrefix:     "/",
			RouteManifestURL:     "/route-manifest.json",
			OutermostServerError: `""`,
		},
	)
	if buildError != nil {
		t.Fatalf("BuildSSRInnerHTML returned error: %v", buildError)
	}
	if output == nil || output.Script == nil {
		t.Fatal("expected non-nil output script")
	}
	if !strings.Contains(
		string(*output.Script),
		`x.deploymentID = "dep-from-env";`,
	) {
		t.Fatalf(
			"expected deployment id from env, got %q",
			string(*output.Script),
		)
	}
}

func TestBuildBodyScriptsForTemplate_Production(t *testing.T) {
	bodyScripts, buildError := BuildBodyScriptsForTemplate(
		BodyScriptsInput{
			RenderSnapshot: LoadersHTMLRenderSnapshot{
				IsDevMode:      false,
				ClientEntryOut: "assets/client.js",
			},
			PublicPathPrefix: "/",
		},
	)
	if buildError != nil {
		t.Fatalf("BuildBodyScriptsForTemplate returned error: %v", buildError)
	}
	if string(
		bodyScripts,
	) != `<script type="module" src="/assets/client.js"></script>` {
		t.Fatalf("unexpected body scripts: %q", string(bodyScripts))
	}
}

func TestBuildBodyScriptsForTemplate_DevIncludesRefreshScript(t *testing.T) {
	t.Setenv("__VITE_PORT", "5173")

	bodyScripts, buildError := BuildBodyScriptsForTemplate(
		BodyScriptsInput{
			RenderSnapshot:  LoadersHTMLRenderSnapshot{IsDevMode: true},
			ClientEntry:     "/src/main.tsx",
			UseReactVariant: true,
			RefreshScript:   template.HTML(`<script id="refresh"></script>`),
		},
	)
	if buildError != nil {
		t.Fatalf("BuildBodyScriptsForTemplate returned error: %v", buildError)
	}
	bodyScriptsString := string(bodyScripts)
	if !strings.Contains(bodyScriptsString, "@vite/client") {
		t.Fatalf("expected vite client script in %q", bodyScriptsString)
	}
	if !strings.Contains(bodyScriptsString, `id="refresh"`) {
		t.Fatalf("expected refresh script in %q", bodyScriptsString)
	}
}

func TestCloneTemplateDataMap(t *testing.T) {
	input := map[string]any{"a": 1, "b": "two"}
	cloned := CloneTemplateDataMap(input)
	if len(cloned) != len(input) {
		t.Fatalf("len(cloned)=%d, want %d", len(cloned), len(input))
	}
	cloned["a"] = 7
	if input["a"] != 1 {
		t.Fatalf("expected input map to remain unchanged, got %v", input["a"])
	}
}

func TestInjectVormaTemplateFields(t *testing.T) {
	rootTemplateData := map[string]any{}
	ssrScript := template.HTML("<script>1</script>")
	InjectVormaTemplateFields(
		rootTemplateData,
		template.HTML("<title>x</title>"),
		&ssrScript,
		"hash-123",
		"headKey",
		"ssrKey",
		"hashKey",
		"rootIDKey",
		"root-id",
	)
	if got := rootTemplateData["hashKey"]; got != "hash-123" {
		t.Fatalf("hashKey=%v, want %q", got, "hash-123")
	}
	if got := rootTemplateData["rootIDKey"]; got != "root-id" {
		t.Fatalf("rootIDKey=%v, want %q", got, "root-id")
	}
}

func TestExecuteRootTemplate(t *testing.T) {
	rootTemplate := template.Must(template.New("root").Parse(`Hello {{.Name}}`))
	htmlBytes, executeError := ExecuteRootTemplate(
		rootTemplate,
		map[string]any{"Name": "Vorma"},
	)
	if executeError != nil {
		t.Fatalf("ExecuteRootTemplate returned error: %v", executeError)
	}
	if string(htmlBytes) != "Hello Vorma" {
		t.Fatalf("unexpected template output: %q", string(htmlBytes))
	}
}

func TestBuildSSRInnerHTMLFromRuntimeState_ValidatesLoadersDataJSON(
	t *testing.T,
) {
	_, buildError := BuildSSRInnerHTMLFromRuntimeState(
		SSRRuntimeState{
			VormaSymbolStr:    "__vorma_internal__",
			BuildID:           `"build"`,
			RootElementID:     "root",
			PublicPathPrefix:  "/static/",
			RouteManifestFile: waveoutputtest.TestWaveOutputPath("route-manifest.js"),
		},
		SSRRouteData{
			LoadersData: []any{make(chan int)},
		},
	)
	if buildError == nil {
		t.Fatal("expected JSON-serializable validation error, got nil")
	}
	if !strings.Contains(
		buildError.Error(),
		"routeData.LoadersData must be JSON-serializable",
	) {
		t.Fatalf("unexpected error: %v", buildError)
	}
}

func TestBuildSSRInnerHTMLFromRuntimeState_LoadersDataJSONEscapesScriptTerminators(
	t *testing.T,
) {
	output, buildError := BuildSSRInnerHTMLFromRuntimeState(
		SSRRuntimeState{
			VormaSymbolStr:    "__vorma_internal__",
			BuildID:           `"build-escape"`,
			RootElementID:     "root",
			PublicPathPrefix:  "/static/",
			RouteManifestFile: waveoutputtest.TestWaveOutputPath("route-manifest.js"),
		},
		SSRRouteData{
			MatchedPatterns: []string{"/"},
			LoadersData: []any{
				map[string]any{"html": `</script><script>alert("x")</script>`},
			},
			ImportURLs:           []string{"/static/entry.js"},
			ExportKeys:           []string{"default"},
			OutermostServerError: `""`,
		},
	)
	if buildError != nil {
		t.Fatalf("BuildSSRInnerHTMLFromRuntimeState returned error: %v", buildError)
	}
	if output == nil || output.Script == nil {
		t.Fatal("expected non-nil output script")
	}

	script := string(*output.Script)
	if strings.Contains(script, `</script><script>alert("x")</script>`) {
		t.Fatalf("expected script terminator to be escaped, got %q", script)
	}
	if !strings.Contains(script, `\u003c/script\u003e`) {
		t.Fatalf(
			"expected escaped script terminator in output, got %q",
			script,
		)
	}
}

func TestBuildLoadersHTMLResponseBytes_RendersDocument(t *testing.T) {
	rootTemplate := template.Must(template.New("root").Parse(
		`<html><head>{{.head}}</head><body>{{.body}}{{.ssr}}</body></html>`,
	))
	htmlBytes, buildError := BuildLoadersHTMLResponseBytes(
		BuildLoadersHTMLResponseInput{
			RenderHeadElements: func() (template.HTML, error) {
				return template.HTML(`<title>Hello</title>`), nil
			},
			CriticalCSSStyleElement: template.HTML(
				`<style id="critical">.a{}</style>`,
			),
			StyleSheetLinkElement: template.HTML(
				`<link rel="stylesheet" href="/app.css">`,
			),
			SSRRuntimeState: SSRRuntimeState{
				VormaSymbolStr:    "__vorma_internal__",
				IsDev:             true,
				BuildID:           "build-123",
				RootElementID:     "root",
				PublicPathPrefix:  "/static/",
				RouteManifestFile: waveoutputtest.TestWaveOutputPath("route-manifest.js"),
			},
			SSRRouteData: SSRRouteData{
				ViteDevURL:           `"http://localhost:5173"`,
				MatchedPatterns:      []string{"/"},
				LoadersData:          []any{map[string]bool{"ok": true}},
				ImportURLs:           []string{"/src/main.tsx"},
				ExportKeys:           []string{"default"},
				OutermostServerError: `""`,
			},
			RootTemplateData:             map[string]any{},
			TemplateDataKeyHeadElements:  "head",
			TemplateDataKeyBodyScripts:   "body",
			TemplateDataKeySSRScript:     "ssr",
			TemplateDataKeySSRScriptHash: "ssrHash",
			TemplateDataKeyRootElementID: "rootID",
			ClientRootElementID:          "root",
			BodyScriptsInput: BodyScriptsInput{
				RenderSnapshot: LoadersHTMLRenderSnapshot{
					IsDevMode:      false,
					ClientEntryOut: waveoutputtest.TestWaveOutputPath("client-entry.js"),
					RootTemplate:   rootTemplate,
				},
				PublicPathPrefix: "/static/",
			},
		},
	)
	if buildError != nil {
		t.Fatalf("BuildLoadersHTMLResponseBytes returned error: %v", buildError)
	}

	rendered := string(htmlBytes)
	if !strings.Contains(rendered, "<title>Hello</title>") {
		t.Fatalf("missing head elements in output: %s", rendered)
	}
	if !strings.Contains(rendered, `id="critical"`) {
		t.Fatalf("missing critical CSS element in output: %s", rendered)
	}
	if !strings.Contains(rendered, `/static`+waveoutputtest.TestWaveOutputURLPath("client-entry.js")) {
		t.Fatalf("missing body scripts in output: %s", rendered)
	}
	if !strings.Contains(rendered, `buildID: "build-123",`) {
		t.Fatalf("missing SSR script output: %s", rendered)
	}
}

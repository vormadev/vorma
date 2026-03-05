package routeparse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimeconfig"
	"github.com/vormadev/vorma/wave/waveconfig"
)

func defaultRawVormaConfigJSONForRouteParseTests() vormaruntime.VormaConfigJSON {
	return vormaruntime.VormaConfigJSON{
		MainBuildEntry:       "backend/cmd/build",
		UIVariant:            string(vormaruntime.UIVariantReact),
		HTMLTemplateLocation: "entry.go.html",
		ClientEntry:          "frontend/src/vorma.entry.tsx",
		ClientRouteDefinitionPatterns: []string{
			"frontend/src/**/*vorma.routes.ts",
		},
		TSGenOutDir: "frontend/src/vorma.gen",
	}
}

func parseVormaConfigForRouteParseTests(
	tb testing.TB,
	rawConfig vormaruntime.VormaConfigJSON,
) (vormaruntime.VormaConfig, error) {
	tb.Helper()

	rawVormaPayload, marshalError := json.Marshal(
		struct {
			Vorma vormaruntime.VormaConfigJSON `json:"Vorma"`
		}{
			Vorma: rawConfig,
		},
	)
	if marshalError != nil {
		return nil, fmt.Errorf("marshal raw Vorma config fixture: %w", marshalError)
	}
	rawWavePayload, marshalWaveError := json.Marshal(
		map[string]any{
			"Core": map[string]any{
				"ProjectID":    "routeparse-tests",
				"MainAppEntry": "backend/cmd/app",
			},
		},
	)
	if marshalWaveError != nil {
		return nil, fmt.Errorf("marshal raw Wave config fixture: %w", marshalWaveError)
	}
	parsedWaveConfig, parseWaveError := waveconfig.ParseConfigJSONWithConfigPath(
		rawWavePayload,
		"wave.config.json",
	)
	if parseWaveError != nil {
		return nil, fmt.Errorf("parse raw Wave config fixture: %w", parseWaveError)
	}
	parsedVormaConfig, parseError := runtimeconfig.ParseVormaConfigJSON(
		rawVormaPayload,
		parsedWaveConfig,
	)
	if parseError != nil {
		return nil, parseError
	}
	return parsedVormaConfig, nil
}

func mustParsedVormaConfigForRouteParseTests(
	tb testing.TB,
	overrides *vormaruntime.VormaConfigJSON,
) vormaruntime.VormaConfig {
	tb.Helper()

	rawConfig := defaultRawVormaConfigJSONForRouteParseTests()
	if overrides != nil {
		if overrides.IncludeDefaults != nil {
			includeDefaults := *overrides.IncludeDefaults
			rawConfig.IncludeDefaults = &includeDefaults
		}
		if overrides.MainBuildEntry != "" {
			rawConfig.MainBuildEntry = overrides.MainBuildEntry
		}
		if overrides.UIVariant != "" {
			rawConfig.UIVariant = overrides.UIVariant
		}
		if overrides.HTMLTemplateLocation != "" {
			rawConfig.HTMLTemplateLocation = overrides.HTMLTemplateLocation
		}
		if overrides.ClientEntry != "" {
			rawConfig.ClientEntry = overrides.ClientEntry
		}
		if overrides.ClientRouteDefinitionPatterns != nil {
			rawConfig.ClientRouteDefinitionPatterns = append(
				[]string(nil),
				overrides.ClientRouteDefinitionPatterns...,
			)
		}
		if overrides.ServerRouteDefinitionPatterns != nil {
			rawConfig.ServerRouteDefinitionPatterns = append(
				[]string(nil),
				overrides.ServerRouteDefinitionPatterns...,
			)
		}
		if overrides.TSGenOutDir != "" {
			rawConfig.TSGenOutDir = overrides.TSGenOutDir
		}
		if overrides.BuildtimePublicURLFuncName != "" {
			rawConfig.BuildtimePublicURLFuncName = overrides.BuildtimePublicURLFuncName
		}
		if overrides.UnresolvedRoutePolicy != "" {
			rawConfig.UnresolvedRoutePolicy = overrides.UnresolvedRoutePolicy
		}
		if overrides.DevReloadRoutesEndpointPath != "" {
			rawConfig.DevReloadRoutesEndpointPath = overrides.DevReloadRoutesEndpointPath
		}
		if overrides.DevReloadTemplateEndpointPath != "" {
			rawConfig.DevReloadTemplateEndpointPath = overrides.DevReloadTemplateEndpointPath
		}
		if overrides.TemplateDataKeyHeadElements != "" {
			rawConfig.TemplateDataKeyHeadElements = overrides.TemplateDataKeyHeadElements
		}
		if overrides.TemplateDataKeyBodyScripts != "" {
			rawConfig.TemplateDataKeyBodyScripts = overrides.TemplateDataKeyBodyScripts
		}
		if overrides.TemplateDataKeySSRScript != "" {
			rawConfig.TemplateDataKeySSRScript = overrides.TemplateDataKeySSRScript
		}
		if overrides.TemplateDataKeySSRScriptHash != "" {
			rawConfig.TemplateDataKeySSRScriptHash = overrides.TemplateDataKeySSRScriptHash
		}
		if overrides.TemplateDataKeyRootElementID != "" {
			rawConfig.TemplateDataKeyRootElementID = overrides.TemplateDataKeyRootElementID
		}
		if overrides.ClientRootElementID != "" {
			rawConfig.ClientRootElementID = overrides.ClientRootElementID
		}
	}

	parsedConfig, parseError := parseVormaConfigForRouteParseTests(tb, rawConfig)
	if parseError != nil {
		tb.Fatalf("parse raw Vorma config fixture: %v", parseError)
	}
	return parsedConfig
}

func TestExtractRouteCalls_HandlesAliasesAndUnresolvedModules(t *testing.T) {
	t.Run(
		"resolves const string variable and optional export keys",
		func(t *testing.T) {
			code := `
			import { route as defineRoute } from "vorma/buildtime";
			const modulePath = "./routes/home.tsx";
			defineRoute("/home", modulePath, "default", "ErrorBoundary");
		`

			routes, unresolved, err := extractRouteCalls(code)
			if err != nil {
				t.Fatalf("extractRouteCalls returned error: %v", err)
			}
			if len(unresolved) != 0 {
				t.Fatalf(
					"expected no unresolved routes, got %d",
					len(unresolved),
				)
			}
			if len(routes) != 1 {
				t.Fatalf("expected 1 route, got %d", len(routes))
			}
			if routes[0].Pattern != "/home" {
				t.Fatalf(
					"route pattern = %q, want %q",
					routes[0].Pattern,
					"/home",
				)
			}
			if routes[0].Module != "./routes/home.tsx" {
				t.Fatalf(
					"route module = %q, want %q",
					routes[0].Module,
					"./routes/home.tsx",
				)
			}
			if routes[0].Key != "default" {
				t.Fatalf("route key = %q, want %q", routes[0].Key, "default")
			}
			if routes[0].ErrorKey != "ErrorBoundary" {
				t.Fatalf(
					"route error key = %q, want %q",
					routes[0].ErrorKey,
					"ErrorBoundary",
				)
			}
		},
	)

	t.Run(
		"collects unresolved route for function-call module argument",
		func(t *testing.T) {
			code := `
			import { route } from "vorma/buildtime";
			route("/dynamic", getPath());
		`

			routes, unresolved, err := extractRouteCalls(code)
			if err != nil {
				t.Fatalf("extractRouteCalls returned error: %v", err)
			}
			if len(routes) != 0 {
				t.Fatalf("expected no resolved routes, got %d", len(routes))
			}
			if len(unresolved) != 1 {
				t.Fatalf("expected 1 unresolved route, got %d", len(unresolved))
			}
			if unresolved[0].Pattern != "/dynamic" {
				t.Fatalf(
					"unresolved pattern = %q, want %q",
					unresolved[0].Pattern,
					"/dynamic",
				)
			}
			if !strings.Contains(unresolved[0].Reason, "function call") {
				t.Fatalf(
					"unresolved reason = %q, expected function-call reason",
					unresolved[0].Reason,
				)
			}
		},
	)

	t.Run(
		"collects unresolved route for non-static expression module argument",
		func(t *testing.T) {
			code := `
			import { route } from "vorma/buildtime";
			const useA = true;
			route("/conditional", useA ? "./routes/a.tsx" : "./routes/b.tsx");
		`

			routes, unresolved, err := extractRouteCalls(code)
			if err != nil {
				t.Fatalf("extractRouteCalls returned error: %v", err)
			}
			if len(routes) != 0 {
				t.Fatalf("expected no resolved routes, got %d", len(routes))
			}
			if len(unresolved) != 1 {
				t.Fatalf("expected 1 unresolved route, got %d", len(unresolved))
			}
			if unresolved[0].Pattern != "/conditional" {
				t.Fatalf(
					"unresolved pattern = %q, want %q",
					unresolved[0].Pattern,
					"/conditional",
				)
			}
			if !strings.Contains(
				unresolved[0].Reason,
				"not a static string, variable, or import() call",
			) {
				t.Fatalf(
					"unresolved reason = %q, expected non-static expression reason",
					unresolved[0].Reason,
				)
			}
		},
	)
}

func TestExtractRouteCalls_ReturnsParseErrorForInvalidSource(t *testing.T) {
	_, _, err := extractRouteCalls(
		`import { route } from "vorma/buildtime"; const broken = ;`,
	)
	if err == nil {
		t.Fatal("expected extractRouteCalls to return parse error")
	}
	if !strings.Contains(err.Error(), "parse JS/TS") {
		t.Fatalf("error = %q, expected parse context", err)
	}
}

func TestTransformRouteDefinitionsCode(t *testing.T) {
	t.Run(
		"rewrites import() calls to static string literals",
		func(t *testing.T) {
			v := &vormaruntime.Vorma{
				Log: testLogger(),
			}

			transformedCode, err := transformRouteDefinitionsCode(v, []byte(`
			import { route } from "vorma/buildtime";
			route("/settings", import("./routes/settings.tsx"), "Settings");
		`))
			if err != nil {
				t.Fatalf(
					"transformRouteDefinitionsCode returned error: %v",
					err,
				)
			}
			if strings.Contains(transformedCode, "import(") {
				t.Fatalf(
					"transformed code should not contain dynamic import(): %q",
					transformedCode,
				)
			}
			if !strings.Contains(transformedCode, `"./routes/settings.tsx"`) {
				t.Fatalf(
					"transformed code missing expected literal module path: %q",
					transformedCode,
				)
			}
		},
	)

	t.Run(
		"returns error and logs esbuild errors for invalid input",
		func(t *testing.T) {
			var logBuffer bytes.Buffer
			v := &vormaruntime.Vorma{
				Log: slog.New(slog.NewTextHandler(&logBuffer, nil)),
			}

			_, err := transformRouteDefinitionsCode(v, []byte(`
			import { route } from "vorma/buildtime";
			const broken = ;
			route("/broken", "./broken.tsx");
		`))
			if err == nil {
				t.Fatal(
					"expected transformRouteDefinitionsCode to return an error for invalid input",
				)
			}
			if !strings.Contains(err.Error(), "esbuild transform failed") {
				t.Fatalf("error = %q, expected esbuild transform failure", err)
			}
			if !strings.Contains(logBuffer.String(), "esbuild error:") {
				t.Fatalf(
					"expected esbuild error log output, got %q",
					logBuffer.String(),
				)
			}
		},
	)
}

func TestResolveModuleArgumentFromFunctionCall(t *testing.T) {
	t.Run("static import() argument resolves module path", func(t *testing.T) {
		modulePath, unresolvedRoute := resolveModuleArgumentFromFunctionCall(
			"/settings",
			&js.CallExpr{
				X: &js.Var{Data: []byte("import")},
				Args: js.Args{
					List: []js.Arg{
						{
							Value: &js.LiteralExpr{
								TokenType: js.StringToken,
								Data:      []byte(`"./routes/settings.tsx"`),
							},
						},
					},
				},
			},
		)

		if unresolvedRoute != nil {
			t.Fatalf(
				"did not expect unresolved route, got %#v",
				unresolvedRoute,
			)
		}
		if modulePath != "./routes/settings.tsx" {
			t.Fatalf(
				"module path = %q, want %q",
				modulePath,
				"./routes/settings.tsx",
			)
		}
	})

	t.Run(
		"dynamic import() with non-static argument is unresolved",
		func(t *testing.T) {
			modulePath, unresolvedRoute := resolveModuleArgumentFromFunctionCall(
				"/dynamic-import",
				&js.CallExpr{
					X: &js.Var{Data: []byte("import")},
					Args: js.Args{
						List: []js.Arg{
							{Value: &js.Var{Data: []byte("dynamicPath")}},
						},
					},
				},
			)

			if unresolvedRoute == nil {
				t.Fatal("expected unresolved route details")
			}
			if modulePath != "" {
				t.Fatalf("module path = %q, want empty", modulePath)
			}
			if unresolvedRoute.Pattern != "/dynamic-import" {
				t.Fatalf(
					"pattern = %q, want %q",
					unresolvedRoute.Pattern,
					"/dynamic-import",
				)
			}
			if unresolvedRoute.RawModuleExpr != "import(...)" {
				t.Fatalf(
					"raw module expr = %q, want %q",
					unresolvedRoute.RawModuleExpr,
					"import(...)",
				)
			}
			if !strings.Contains(
				unresolvedRoute.Reason,
				"dynamic import() argument is not a static string",
			) {
				t.Fatalf(
					"reason = %q, expected dynamic-import reason",
					unresolvedRoute.Reason,
				)
			}
		},
	)

	t.Run(
		"non-import call with non-ident callee uses unknown function marker",
		func(t *testing.T) {
			modulePath, unresolvedRoute := resolveModuleArgumentFromFunctionCall(
				"/unknown-callee",
				&js.CallExpr{
					X: &js.LiteralExpr{
						TokenType: js.StringToken,
						Data:      []byte(`"not-a-function-ident"`),
					},
				},
			)

			if unresolvedRoute == nil {
				t.Fatal("expected unresolved route details")
			}
			if modulePath != "" {
				t.Fatalf("module path = %q, want empty", modulePath)
			}
			if unresolvedRoute.RawModuleExpr != "<unknown>(...)" {
				t.Fatalf(
					"raw module expr = %q, want %q",
					unresolvedRoute.RawModuleExpr,
					"<unknown>(...)",
				)
			}
			if !strings.Contains(unresolvedRoute.Reason, "function call") {
				t.Fatalf(
					"reason = %q, expected function-call reason",
					unresolvedRoute.Reason,
				)
			}
		},
	)
}

func TestParseClientRoutes_ResolvesStaticAndImportModules(t *testing.T) {
	rootDir := t.TempDir()
	t.Chdir(rootDir)

	routesFile := filepath.Join("frontend", "src", "vorma.routes.ts")
	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "routes", "root.tsx"),
		[]byte("export default function Root() {}"),
	)
	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "routes", "users.tsx"),
		[]byte("export default function Users() {}"),
	)
	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "routes", "settings.tsx"),
		[]byte("export const Settings = () => null;"),
	)

	mustWriteFile(t, routesFile, []byte(`
			import { route } from "vorma/buildtime";
			const usersModule = "./routes/users.tsx";
			route("/", "./routes/root.tsx", "default");
			route("/users", usersModule, "default", "UsersErrorBoundary");
			route("/settings", import("./routes/settings.tsx"), "Settings");
		`))

	v := &vormaruntime.Vorma{
		Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
			ClientRouteDefinitionPatterns: []string{routesFile},
		}),
		Log: testLogger(),
	}
	v.SetIsDev(true)

	paths, err := ParseClientRoutes(v)
	if err != nil {
		t.Fatalf("parseClientRoutes returned error: %v", err)
	}

	if len(paths) != 3 {
		t.Fatalf("expected 3 resolved paths, got %d", len(paths))
	}

	root := paths["/"]
	if root == nil {
		t.Fatalf("missing root route")
	}
	if root.SrcPath != "frontend/src/routes/root.tsx" {
		t.Fatalf(
			"root src path = %q, want %q",
			root.SrcPath,
			"frontend/src/routes/root.tsx",
		)
	}

	users := paths["/users"]
	if users == nil {
		t.Fatalf("missing users route")
	}
	if users.SrcPath != "frontend/src/routes/users.tsx" {
		t.Fatalf(
			"users src path = %q, want %q",
			users.SrcPath,
			"frontend/src/routes/users.tsx",
		)
	}
	if users.ErrorExportKey != "UsersErrorBoundary" {
		t.Fatalf(
			"users error export key = %q, want %q",
			users.ErrorExportKey,
			"UsersErrorBoundary",
		)
	}

	settings := paths["/settings"]
	if settings == nil {
		t.Fatalf("missing settings route")
	}
	if settings.SrcPath != "frontend/src/routes/settings.tsx" {
		t.Fatalf(
			"settings src path = %q, want %q",
			settings.SrcPath,
			"frontend/src/routes/settings.tsx",
		)
	}
	if settings.ExportKey != "Settings" {
		t.Fatalf(
			"settings export key = %q, want %q",
			settings.ExportKey,
			"Settings",
		)
	}
}

func TestParseClientRoutes_UnresolvedRouteDefaultPolicyFailsInProd(
	t *testing.T,
) {
	rootDir := t.TempDir()
	t.Chdir(rootDir)

	routesFile := filepath.Join("frontend", "src", "vorma.routes.ts")
	mustWriteFile(t, routesFile, []byte(`
		import { route } from "vorma/buildtime";
		route("/dynamic", getPath());
	`))

	v := &vormaruntime.Vorma{
		Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
			ClientRouteDefinitionPatterns: []string{routesFile},
		}),
		Log: testLogger(),
	}

	_, err := ParseClientRoutes(v)
	if err == nil {
		t.Fatal("expected unresolved route call to fail in production mode")
	}
	if !strings.Contains(
		err.Error(),
		"unresolved route calls are not allowed",
	) {
		t.Fatalf("error = %q, expected unresolved-route policy failure", err)
	}
}

func TestParseClientRoutes_UnresolvedRoutePolicyOverrideWarnInProd(
	t *testing.T,
) {
	rootDir := t.TempDir()
	t.Chdir(rootDir)

	routesFile := filepath.Join("frontend", "src", "vorma.routes.ts")
	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "routes", "home.tsx"),
		[]byte("export default function Home() {}"),
	)
	mustWriteFile(t, routesFile, []byte(`
		import { route } from "vorma/buildtime";
		route("/", "./routes/home.tsx", "default");
		route("/dynamic", getPath());
	`))

	v := &vormaruntime.Vorma{
		Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
			ClientRouteDefinitionPatterns: []string{routesFile},
			UnresolvedRoutePolicy:         vormaruntime.UnresolvedRoutePolicyWarn,
		}),
		Log: testLogger(),
	}

	paths, err := ParseClientRoutes(v)
	if err != nil {
		t.Fatalf("parseClientRoutes returned error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 resolved path, got %d", len(paths))
	}
	if got := paths["/"]; got == nil ||
		got.SrcPath != "frontend/src/routes/home.tsx" {
		t.Fatalf("paths[/] = %#v, want frontend/src/routes/home.tsx", got)
	}
}

func TestParseClientRoutes_MergesRoutesAcrossDefinitionFiles(t *testing.T) {
	rootDir := t.TempDir()
	t.Chdir(rootDir)

	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "components", "root.tsx"),
		[]byte("export const Root = () => null;"),
	)
	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "components", "links.tsx"),
		[]byte("export const Links = () => null;"),
	)

	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "routes", "core.vorma.routes.ts"),
		[]byte(`
			import { route } from "vorma/buildtime";
			route("/", import("../components/root.tsx"), "Root");
		`),
	)
	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "routes", "links.vorma.routes.ts"),
		[]byte(`
			import { route } from "vorma/buildtime";
			route("/links", import("../components/links.tsx"), "Links");
		`),
	)

	v := &vormaruntime.Vorma{
		Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
			ClientRouteDefinitionPatterns: []string{
				"frontend/src/**/*vorma.routes.ts",
			},
		}),
		Log: testLogger(),
	}

	paths, err := ParseClientRoutes(v)
	if err != nil {
		t.Fatalf("parseClientRoutes returned error: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 resolved paths, got %d", len(paths))
	}
	if got := paths["/"]; got == nil ||
		got.SrcPath != "frontend/src/components/root.tsx" {
		t.Fatalf("paths[/] = %#v, want frontend/src/components/root.tsx", got)
	}
	if got := paths["/links"]; got == nil ||
		got.SrcPath != "frontend/src/components/links.tsx" {
		t.Fatalf(
			"paths[/links] = %#v, want frontend/src/components/links.tsx",
			got,
		)
	}
}

func TestParseClientRoutes_ReturnsErrorForDuplicatePatternAcrossDefinitionFiles(
	t *testing.T,
) {
	rootDir := t.TempDir()
	t.Chdir(rootDir)

	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "components", "first.tsx"),
		[]byte("export const First = () => null;"),
	)
	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "components", "second.tsx"),
		[]byte("export const Second = () => null;"),
	)

	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "routes", "first.vorma.routes.ts"),
		[]byte(`
			import { route } from "vorma/buildtime";
			route("/dup", import("../components/first.tsx"), "First");
		`),
	)
	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "routes", "second.vorma.routes.ts"),
		[]byte(`
			import { route } from "vorma/buildtime";
			route("/dup", import("../components/second.tsx"), "Second");
		`),
	)

	v := &vormaruntime.Vorma{
		Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
			ClientRouteDefinitionPatterns: []string{
				"frontend/src/**/*vorma.routes.ts",
			},
		}),
		Log: testLogger(),
	}

	_, err := ParseClientRoutes(v)
	if err == nil {
		t.Fatal("expected duplicate route pattern error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate route pattern: /dup") {
		t.Fatalf("error = %q, expected duplicate route pattern message", err)
	}
}

func TestParseClientRoutes_ReturnsErrorForNormalizedRootAliasCollision(
	t *testing.T,
) {
	rootDir := t.TempDir()
	t.Chdir(rootDir)

	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "components", "root.tsx"),
		[]byte("export const Root = () => null;"),
	)
	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "components", "root_index.tsx"),
		[]byte("export const RootAlias = () => null;"),
	)
	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "routes", "root.vorma.routes.ts"),
		[]byte(`
			import { route } from "vorma/buildtime";
			route("/", import("../components/root.tsx"), "Root");
			route("", import("../components/root_index.tsx"), "RootAlias");
		`),
	)

	v := &vormaruntime.Vorma{
		Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
			ClientRouteDefinitionPatterns: []string{
				"frontend/src/**/*vorma.routes.ts",
			},
		}),
		Log: testLogger(),
	}

	_, err := ParseClientRoutes(v)
	if err == nil {
		t.Fatal("expected normalized route pattern collision error, got nil")
	}
	if !strings.Contains(err.Error(), "normalized route pattern collision") {
		t.Fatalf(
			"error = %q, expected normalized route pattern collision message",
			err,
		)
	}
}

func TestParseClientRoutes_ReturnsErrorForNormalizedRootAliasCollisionAcrossFiles(
	t *testing.T,
) {
	rootDir := t.TempDir()
	t.Chdir(rootDir)

	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "components", "root.tsx"),
		[]byte("export const Root = () => null;"),
	)
	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "components", "root_index.tsx"),
		[]byte("export const RootAlias = () => null;"),
	)
	mustWriteFile(
		t,
		filepath.Join("frontend", "src", "routes", "root.vorma.routes.ts"),
		[]byte(`
			import { route } from "vorma/buildtime";
			route("/", import("../components/root.tsx"), "Root");
		`),
	)
	mustWriteFile(
		t,
		filepath.Join(
			"frontend",
			"src",
			"routes",
			"root_index.vorma.routes.ts",
		),
		[]byte(`
			import { route } from "vorma/buildtime";
			route("", import("../components/root_index.tsx"), "RootAlias");
		`),
	)

	v := &vormaruntime.Vorma{
		Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
			ClientRouteDefinitionPatterns: []string{
				"frontend/src/**/*vorma.routes.ts",
			},
		}),
		Log: testLogger(),
	}

	_, err := ParseClientRoutes(v)
	if err == nil {
		t.Fatal("expected normalized route pattern collision error, got nil")
	}
	if !strings.Contains(err.Error(), "normalized route pattern collision") {
		t.Fatalf(
			"error = %q, expected normalized route pattern collision message",
			err,
		)
	}
}

func TestParseClientRoutes_ReturnsErrorWhenRouteDefinitionPatternsNeedNormalization(
	t *testing.T,
) {
	t.Run(
		"returns parse error when a pattern has surrounding whitespace",
		func(t *testing.T) {
			rawConfig := defaultRawVormaConfigJSONForRouteParseTests()
			rawConfig.ClientRouteDefinitionPatterns = []string{
				" frontend/src/routes/home.vorma.routes.ts ",
			}
			_, parseError := parseVormaConfigForRouteParseTests(t, rawConfig)
			if parseError == nil {
				t.Fatal(
					"expected parse error for route definition pattern with surrounding whitespace",
				)
			}
			if !strings.Contains(
				parseError.Error(),
				"must not contain surrounding whitespace",
			) {
				t.Fatalf(
					"error = %q, expected surrounding-whitespace pattern parse error",
					parseError,
				)
			}
		},
	)

	t.Run("returns parse error when patterns contain duplicates", func(t *testing.T) {
		rawConfig := defaultRawVormaConfigJSONForRouteParseTests()
		rawConfig.ClientRouteDefinitionPatterns = []string{
			"frontend/src/routes/home.vorma.routes.ts",
			"frontend/src/routes/home.vorma.routes.ts",
		}
		_, parseError := parseVormaConfigForRouteParseTests(t, rawConfig)
		if parseError == nil {
			t.Fatal("expected parse error for duplicate route definition patterns")
		}
		if !strings.Contains(parseError.Error(), "duplicates an earlier pattern") {
			t.Fatalf("error = %q, expected duplicate pattern parse error", parseError)
		}
	})
}

func TestParseClientRoutes_ReturnsParseErrorWhenRouteDefinitionPatternsContainOnlyWhitespace(
	t *testing.T,
) {
	rawConfig := defaultRawVormaConfigJSONForRouteParseTests()
	rawConfig.ClientRouteDefinitionPatterns = []string{
		" ",
		"\n\t",
	}
	_, parseError := parseVormaConfigForRouteParseTests(t, rawConfig)
	if parseError == nil {
		t.Fatal(
			"expected parse error for whitespace-only route definition patterns, got nil",
		)
	}
	if !strings.Contains(parseError.Error(), "cannot be empty or whitespace") {
		t.Fatalf("error = %q, expected whitespace-only pattern parse error", parseError)
	}
}

func TestParseClientRoutes_ReturnsErrorWhenModuleDoesNotExist(t *testing.T) {
	rootDir := t.TempDir()
	t.Chdir(rootDir)

	routesFile := filepath.Join("frontend", "src", "vorma.routes.ts")
	mustWriteFile(t, routesFile, []byte(`
		import { route } from "vorma/buildtime";
		route("/missing", "./routes/missing.tsx");
	`))

	v := &vormaruntime.Vorma{
		Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
			ClientRouteDefinitionPatterns: []string{routesFile},
		}),
		Log: testLogger(),
	}

	_, err := ParseClientRoutes(v)
	if err == nil {
		t.Fatal("expected error for missing module, got nil")
	}
	if !strings.Contains(err.Error(), "component module does not exist") {
		t.Fatalf("error = %q, expected missing-module message", err)
	}
}

func TestParseClientRoutes_ReturnsErrorWhenRoutesFileDoesNotExist(
	t *testing.T,
) {
	v := &vormaruntime.Vorma{
		Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
			ClientRouteDefinitionPatterns: []string{
				"frontend/src/missing.routes.ts",
			},
		}),
		Log: testLogger(),
	}

	_, err := ParseClientRoutes(v)
	if err == nil {
		t.Fatal("expected error for missing route definitions file, got nil")
	}
	if !strings.Contains(err.Error(), "stat route definition path") {
		t.Fatalf("error = %q, expected stat route definition path context", err)
	}
}

func TestParseClientRoutes_ReturnsErrorWhenTransformFails(t *testing.T) {
	rootDir := t.TempDir()
	t.Chdir(rootDir)

	routesFile := filepath.Join("frontend", "src", "vorma.routes.ts")
	mustWriteFile(t, routesFile, []byte(`
		import { route } from "vorma/buildtime";
		const broken = ;
		route("/broken", "./routes/broken.tsx");
	`))

	v := &vormaruntime.Vorma{
		Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
			ClientRouteDefinitionPatterns: []string{routesFile},
		}),
		Log: testLogger(),
	}

	_, err := ParseClientRoutes(v)
	if err == nil {
		t.Fatal("expected transform error, got nil")
	}
	if !strings.Contains(err.Error(), "esbuild transform failed") {
		t.Fatalf("error = %q, expected esbuild transform failure", err)
	}
}

func TestIsBuildtimeImportStatement(t *testing.T) {
	buildtimeImport := &js.ImportStmt{Module: []byte(`"vorma/buildtime"`)}
	if !isBuildtimeImportStatement(buildtimeImport) {
		t.Fatal("expected buildtime import statement to be recognized")
	}

	clientImport := &js.ImportStmt{Module: []byte(`"vorma/client"`)}
	if isBuildtimeImportStatement(clientImport) {
		t.Fatal("expected non-buildtime import statement not to be recognized")
	}
}

func TestRouteImportAliasHelpers(t *testing.T) {
	tests := []struct {
		name        string
		alias       js.Alias
		wantIsRoute bool
		wantBinding string
	}{
		{
			name: "direct route import",
			alias: js.Alias{
				Name:    []byte("route"),
				Binding: []byte("route"),
			},
			wantIsRoute: true,
			wantBinding: "route",
		},
		{
			name: "renamed route import",
			alias: js.Alias{
				Name:    []byte("route"),
				Binding: []byte("defineRoute"),
			},
			wantIsRoute: true,
			wantBinding: "defineRoute",
		},
		{
			name:        "route from minified import form",
			alias:       js.Alias{Name: nil, Binding: []byte("route")},
			wantIsRoute: true,
			wantBinding: "route",
		},
		{
			name: "non-route import",
			alias: js.Alias{
				Name:    []byte("somethingElse"),
				Binding: []byte("somethingElse"),
			},
			wantIsRoute: false,
			wantBinding: "somethingElse",
		},
		{
			name:        "alias with name and empty binding",
			alias:       js.Alias{Name: []byte("route"), Binding: nil},
			wantIsRoute: true,
			wantBinding: "route",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := isRouteImportAlias(testCase.alias); got != testCase.wantIsRoute {
				t.Fatalf(
					"isRouteImportAlias(%#v) = %v, want %v",
					testCase.alias,
					got,
					testCase.wantIsRoute,
				)
			}
			if got := routeImportAliasBinding(testCase.alias); got != testCase.wantBinding {
				t.Fatalf(
					"routeImportAliasBinding(%#v) = %q, want %q",
					testCase.alias,
					got,
					testCase.wantBinding,
				)
			}
		})
	}
}

func TestLogEsbuildTransformErrors(t *testing.T) {
	var logBuffer bytes.Buffer
	v := &vormaruntime.Vorma{
		Log: slog.New(slog.NewTextHandler(&logBuffer, nil)),
	}

	logEsbuildTransformErrors(v, []esbuild.Message{
		{Text: "first issue"},
		{Text: "second issue"},
	})

	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "first issue") {
		t.Fatalf("log output missing first message: %q", logOutput)
	}
	if !strings.Contains(logOutput, "second issue") {
		t.Fatalf("log output missing second message: %q", logOutput)
	}
}

func TestRouteCallVisitorExit_IsNoOp(t *testing.T) {
	visitor := &routeCallVisitor{}
	visitor.Exit(nil)
}

func TestExtractStaticStringLiteral_ReturnsFalseForInvalidQuotedString(
	t *testing.T,
) {
	value, ok := extractStaticStringLiteral(&js.LiteralExpr{
		TokenType: js.StringToken,
		Data:      []byte(`"\xzz"`),
	})
	if ok {
		t.Fatalf(
			"expected invalid quoted string to fail extraction, got value %q",
			value,
		)
	}
	if value != "" {
		t.Fatalf("value = %q, want empty value on extraction failure", value)
	}
}

func TestCollectRouteParsingMetadata_TracksOnlyStaticStringVariableAssignments(
	t *testing.T,
) {
	parsedAST, err := js.Parse(parse.NewInputString(`
		const tracked = "./routes/tracked.tsx";
		let alsoTracked = "./routes/also-tracked.tsx";
		const numeric = 123;
		const { destructured } = someObject;
	`), js.Options{})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	trackedModuleVars := collectRouteParsingMetadata(
		parsedAST,
	).trackedModuleVars
	if len(trackedModuleVars) != 2 {
		t.Fatalf("tracked vars len = %d, want %d", len(trackedModuleVars), 2)
	}
	if trackedModuleVars["tracked"] != "./routes/tracked.tsx" {
		t.Fatalf(
			"tracked module = %q, want %q",
			trackedModuleVars["tracked"],
			"./routes/tracked.tsx",
		)
	}
	if trackedModuleVars["alsoTracked"] != "./routes/also-tracked.tsx" {
		t.Fatalf(
			"alsoTracked module = %q, want %q",
			trackedModuleVars["alsoTracked"],
			"./routes/also-tracked.tsx",
		)
	}
	if _, exists := trackedModuleVars["numeric"]; exists {
		t.Fatal("did not expect numeric assignment to be tracked")
	}
	if _, exists := trackedModuleVars["destructured"]; exists {
		t.Fatal("did not expect destructured binding to be tracked")
	}
}

func TestCollectRouteParsingMetadata(t *testing.T) {
	parsedAST, err := js.Parse(parse.NewInputString(`
		import { route as defineRoute } from "vorma/buildtime";
		import { route as clientRoute } from "vorma/client";
		import { somethingElse } from "vorma/buildtime";
		const tracked = "./routes/tracked.tsx";
		const notTracked = 123;
	`), js.Options{})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	metadata := collectRouteParsingMetadata(parsedAST)
	if len(metadata.routeFuncNames) != 1 {
		t.Fatalf(
			"route func names len = %d, want %d",
			len(metadata.routeFuncNames),
			1,
		)
	}
	if _, ok := metadata.routeFuncNames["defineRoute"]; !ok {
		t.Fatalf(
			"expected defineRoute to be tracked, got %#v",
			metadata.routeFuncNames,
		)
	}
	if _, ok := metadata.routeFuncNames["clientRoute"]; ok {
		t.Fatalf(
			"did not expect clientRoute from non-buildtime import, got %#v",
			metadata.routeFuncNames,
		)
	}
	if len(metadata.trackedModuleVars) != 1 {
		t.Fatalf(
			"tracked vars len = %d, want %d",
			len(metadata.trackedModuleVars),
			1,
		)
	}
	if metadata.trackedModuleVars["tracked"] != "./routes/tracked.tsx" {
		t.Fatalf(
			"tracked module var = %q, want %q",
			metadata.trackedModuleVars["tracked"],
			"./routes/tracked.tsx",
		)
	}
	if _, exists := metadata.trackedModuleVars["notTracked"]; exists {
		t.Fatalf(
			"did not expect non-string variable to be tracked, got %#v",
			metadata.trackedModuleVars,
		)
	}
}

func TestRouteCallVisitorExtractRouteCall(t *testing.T) {
	visitor := &routeCallVisitor{
		trackedModuleVars: map[string]string{
			"knownModule": "./routes/known.tsx",
		},
	}

	t.Run(
		"returns unresolved=false when pattern is not static string",
		func(t *testing.T) {
			extractedRoute, unresolvedRoute := visitor.extractRouteCall(
				[]js.Arg{
					{
						Value: &js.Var{Data: []byte("dynamicPattern")},
					},
				},
			)
			if extractedRoute != nil {
				t.Fatal(
					"expected extractRouteCall to be unresolved for dynamic pattern",
				)
			}
			if unresolvedRoute != nil {
				t.Fatalf(
					"did not expect unresolved route details for dynamic pattern, got %#v",
					unresolvedRoute,
				)
			}
		},
	)

	t.Run(
		"returns unresolved route when module variable is unknown",
		func(t *testing.T) {
			extractedRoute, unresolvedRoute := visitor.extractRouteCall(
				[]js.Arg{
					{Value: stringLiteralExpr("/dynamic")},
					{Value: &js.Var{Data: []byte("unknownModule")}},
				},
			)
			if extractedRoute != nil {
				t.Fatal(
					"expected extractRouteCall to be unresolved for unknown module variable",
				)
			}
			if unresolvedRoute == nil {
				t.Fatal("expected unresolved route details")
			}
			if unresolvedRoute.RawModuleExpr != "unknownModule" {
				t.Fatalf(
					"raw module expr = %q, want %q",
					unresolvedRoute.RawModuleExpr,
					"unknownModule",
				)
			}
		},
	)

	t.Run(
		"resolves tracked module variable and optional keys",
		func(t *testing.T) {
			extractedRoute, unresolvedRoute := visitor.extractRouteCall(
				[]js.Arg{
					{Value: stringLiteralExpr("/known")},
					{Value: &js.Var{Data: []byte("knownModule")}},
					{Value: stringLiteralExpr("NamedExport")},
					{Value: stringLiteralExpr("ErrorBoundary")},
				},
			)
			if extractedRoute == nil {
				t.Fatal(
					"expected extractRouteCall to resolve tracked module variable",
				)
			}
			if unresolvedRoute != nil {
				t.Fatalf(
					"did not expect unresolved route details, got %#v",
					unresolvedRoute,
				)
			}
			if extractedRoute.Pattern != "/known" {
				t.Fatalf(
					"pattern = %q, want %q",
					extractedRoute.Pattern,
					"/known",
				)
			}
			if extractedRoute.Module != "./routes/known.tsx" {
				t.Fatalf(
					"module = %q, want %q",
					extractedRoute.Module,
					"./routes/known.tsx",
				)
			}
			if extractedRoute.Key != "NamedExport" {
				t.Fatalf("key = %q, want %q", extractedRoute.Key, "NamedExport")
			}
			if extractedRoute.ErrorKey != "ErrorBoundary" {
				t.Fatalf(
					"error key = %q, want %q",
					extractedRoute.ErrorKey,
					"ErrorBoundary",
				)
			}
		},
	)

	t.Run(
		"defaults key to default when key argument is omitted",
		func(t *testing.T) {
			extractedRoute, _ := visitor.extractRouteCall([]js.Arg{
				{Value: stringLiteralExpr("/known-default-key")},
				{Value: &js.Var{Data: []byte("knownModule")}},
			})
			if extractedRoute == nil {
				t.Fatal(
					"expected extractRouteCall to resolve route with omitted key argument",
				)
			}
			if extractedRoute.Key != "default" {
				t.Fatalf("key = %q, want %q", extractedRoute.Key, "default")
			}
		},
	)

	t.Run("resolves route with only pattern argument", func(t *testing.T) {
		extractedRoute, unresolvedRoute := visitor.extractRouteCall([]js.Arg{
			{Value: stringLiteralExpr("/pattern-only")},
		})
		if extractedRoute == nil {
			t.Fatal(
				"expected extractRouteCall to resolve route with only pattern argument",
			)
		}
		if unresolvedRoute != nil {
			t.Fatalf(
				"did not expect unresolved route details, got %#v",
				unresolvedRoute,
			)
		}
		if extractedRoute.Pattern != "/pattern-only" {
			t.Fatalf(
				"pattern = %q, want %q",
				extractedRoute.Pattern,
				"/pattern-only",
			)
		}
		if extractedRoute.Module != "" {
			t.Fatalf(
				"module = %q, want empty for route with only pattern argument",
				extractedRoute.Module,
			)
		}
		if extractedRoute.Key != "default" {
			t.Fatalf("key = %q, want %q", extractedRoute.Key, "default")
		}
	})
}

func TestRouteCallVisitorResolveModuleArgument(t *testing.T) {
	visitor := &routeCallVisitor{
		trackedModuleVars: map[string]string{
			"knownModule": "./routes/known.tsx",
		},
	}

	modulePath, unresolvedRoute := visitor.resolveModuleArgument(
		"/known",
		&js.Var{Data: []byte("knownModule")},
	)
	if unresolvedRoute != nil {
		t.Fatalf("did not expect unresolved route, got %#v", unresolvedRoute)
	}
	if modulePath != "./routes/known.tsx" {
		t.Fatalf("module path = %q, want %q", modulePath, "./routes/known.tsx")
	}

	_, unresolvedModuleRoute := visitor.resolveModuleArgument(
		"/expr",
		&js.BinaryExpr{
			X:  stringLiteralExpr("./a.tsx"),
			Op: js.AddToken,
			Y:  stringLiteralExpr("./b.tsx"),
		},
	)
	if unresolvedModuleRoute == nil {
		t.Fatal("expected unresolved route details")
	}
	if unresolvedModuleRoute.RawModuleExpr != "<expression>" {
		t.Fatalf(
			"raw module expr = %q, want %q",
			unresolvedModuleRoute.RawModuleExpr,
			"<expression>",
		)
	}
}

func TestRouteCallVisitorEnter(t *testing.T) {
	visitor := &routeCallVisitor{
		routeFuncNames: map[string]bool{
			"route": true,
		},
		trackedModuleVars: map[string]string{
			"knownModule": "./routes/known.tsx",
		},
	}

	if got := visitor.Enter(&js.Var{Data: []byte("not-call")}); got != visitor {
		t.Fatal("expected Enter to return visitor for non-call node")
	}
	if len(visitor.routes) != 0 || len(visitor.unresolvedRoutes) != 0 {
		t.Fatalf(
			"unexpected routes after non-call node: routes=%#v unresolved=%#v",
			visitor.routes,
			visitor.unresolvedRoutes,
		)
	}

	if got := visitor.Enter(&js.CallExpr{
		X: &js.LiteralExpr{TokenType: js.StringToken, Data: []byte(`"route"`)},
	}); got != visitor {
		t.Fatal("expected Enter to return visitor for non-ident call target")
	}
	if len(visitor.routes) != 0 || len(visitor.unresolvedRoutes) != 0 {
		t.Fatalf(
			"unexpected routes after non-ident call: routes=%#v unresolved=%#v",
			visitor.routes,
			visitor.unresolvedRoutes,
		)
	}

	if got := visitor.Enter(&js.CallExpr{
		X: &js.Var{Data: []byte("notRoute")},
		Args: js.Args{
			List: []js.Arg{
				{Value: stringLiteralExpr("/ignored")},
			},
		},
	}); got != visitor {
		t.Fatal("expected Enter to return visitor for untracked route function")
	}
	if len(visitor.routes) != 0 || len(visitor.unresolvedRoutes) != 0 {
		t.Fatalf(
			"unexpected routes after untracked route function: routes=%#v unresolved=%#v",
			visitor.routes,
			visitor.unresolvedRoutes,
		)
	}

	visitor.Enter(&js.CallExpr{
		X: &js.Var{Data: []byte("route")},
		Args: js.Args{
			List: []js.Arg{
				{Value: stringLiteralExpr("/known")},
				{Value: &js.Var{Data: []byte("knownModule")}},
			},
		},
	})
	if len(visitor.routes) != 1 {
		t.Fatalf("routes len = %d, want %d", len(visitor.routes), 1)
	}
	if visitor.routes[0].Pattern != "/known" {
		t.Fatalf(
			"resolved route pattern = %q, want %q",
			visitor.routes[0].Pattern,
			"/known",
		)
	}

	visitor.Enter(&js.CallExpr{
		X: &js.Var{Data: []byte("route")},
		Args: js.Args{
			List: []js.Arg{
				{Value: &js.Var{Data: []byte("dynamicPattern")}},
			},
		},
	})
	if len(visitor.routes) != 1 {
		t.Fatalf(
			"routes len = %d, want %d after unresolved-without-details call",
			len(visitor.routes),
			1,
		)
	}
	if len(visitor.unresolvedRoutes) != 0 {
		t.Fatalf(
			"unresolved routes len = %d, want %d after unresolved-without-details call",
			len(visitor.unresolvedRoutes),
			0,
		)
	}

	visitor.Enter(&js.CallExpr{
		X: &js.Var{Data: []byte("route")},
		Args: js.Args{
			List: []js.Arg{
				{Value: stringLiteralExpr("/dynamic")},
				{Value: &js.Var{Data: []byte("unknownModule")}},
			},
		},
	})
	if len(visitor.unresolvedRoutes) != 1 {
		t.Fatalf(
			"unresolved routes len = %d, want %d",
			len(visitor.unresolvedRoutes),
			1,
		)
	}
	if visitor.unresolvedRoutes[0].Pattern != "/dynamic" {
		t.Fatalf(
			"unresolved route pattern = %q, want %q",
			visitor.unresolvedRoutes[0].Pattern,
			"/dynamic",
		)
	}
}

func stringLiteralExpr(value string) *js.LiteralExpr {
	return &js.LiteralExpr{
		TokenType: js.StringToken,
		Data:      []byte(strconv.Quote(value)),
	}
}

func TestResolveRouteModulePath(t *testing.T) {
	t.Run(
		"falls back to original module path when relative path resolution fails",
		func(t *testing.T) {
			executor := newRouteParsingExecutorForTest(
				func(dependencies *routeParsingExecutorDependencies) {
					dependencies.computeRelativeModulePath = func(
						basePath string,
						targetPath string,
					) (string, error) {
						return "", errors.New("cannot resolve relative path")
					}
				},
			)

			v := &vormaruntime.Vorma{
				Config: mustParsedVormaConfigForRouteParseTests(t, &vormaruntime.VormaConfigJSON{
					ClientRouteDefinitionPatterns: []string{
						"frontend/src/**/*vorma.routes.ts",
					},
				}),
				Log: testLogger(),
			}
			routeCall := routeCall{
				Pattern: "/users",
				Module:  "./routes/users.tsx",
			}

			resolvedPath := executor.resolveRouteModulePath(
				v,
				"frontend/src/vorma.routes.ts",
				routeCall,
			)
			if resolvedPath != "./routes/users.tsx" {
				t.Fatalf(
					"resolved path = %q, want fallback module path %q",
					resolvedPath,
					"./routes/users.tsx",
				)
			}
		},
	)
}

func TestEnsureRouteModuleExists(t *testing.T) {
	t.Run(
		"returns missing-module error when stat reports not-exist",
		func(t *testing.T) {
			executor := newRouteParsingExecutorForTest(
				func(dependencies *routeParsingExecutorDependencies) {
					dependencies.statRouteModulePath = func(
						path string,
					) (fs.FileInfo, error) {
						return nil, fs.ErrNotExist
					}
				},
			)

			err := executor.ensureRouteModuleExists(
				"frontend/src/routes/missing.tsx",
				"/missing",
			)
			if err == nil {
				t.Fatal(
					"expected ensureRouteModuleExists to return missing-module error",
				)
			}
			if !strings.Contains(
				err.Error(),
				"component module does not exist",
			) {
				t.Fatalf("error = %q, expected missing-module message", err)
			}
		},
	)

	t.Run(
		"returns wrapped access error when stat fails for other reasons",
		func(t *testing.T) {
			expectedErr := fs.ErrPermission
			executor := newRouteParsingExecutorForTest(
				func(dependencies *routeParsingExecutorDependencies) {
					dependencies.statRouteModulePath = func(
						path string,
					) (fs.FileInfo, error) {
						return nil, expectedErr
					}
				},
			)

			err := executor.ensureRouteModuleExists(
				"frontend/src/routes/secret.tsx",
				"/secret",
			)
			if err == nil {
				t.Fatal(
					"expected ensureRouteModuleExists to return access error",
				)
			}
			if !strings.Contains(err.Error(), "access component module") {
				t.Fatalf("error = %q, expected access-error context", err)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected wrapped permission error", err)
			}
		},
	)

	t.Run(
		"returns error when module path points to a directory",
		func(t *testing.T) {
			moduleDirectory := t.TempDir()

			err := ensureRouteModuleExists(moduleDirectory, "/dir-module")
			if err == nil {
				t.Fatal(
					"expected ensureRouteModuleExists to return directory-module error",
				)
			}
			if !strings.Contains(
				err.Error(),
				"component module is a directory",
			) {
				t.Fatalf("error = %q, expected directory-module message", err)
			}
			if !strings.Contains(err.Error(), "/dir-module") {
				t.Fatalf("error = %q, expected route pattern context", err)
			}
		},
	)
}

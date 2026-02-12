package vormabuild

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/tdewolff/parse/v2/js"
	"github.com/vormadev/vorma/vormaruntime"
)

func TestExtractRouteCalls_HandlesAliasesAndUnresolvedModules(t *testing.T) {
	t.Run("resolves const string variable and optional export keys", func(t *testing.T) {
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
			t.Fatalf("expected no unresolved routes, got %d", len(unresolved))
		}
		if len(routes) != 1 {
			t.Fatalf("expected 1 route, got %d", len(routes))
		}
		if routes[0].Pattern != "/home" {
			t.Fatalf("route pattern = %q, want %q", routes[0].Pattern, "/home")
		}
		if routes[0].Module != "./routes/home.tsx" {
			t.Fatalf("route module = %q, want %q", routes[0].Module, "./routes/home.tsx")
		}
		if routes[0].Key != "default" {
			t.Fatalf("route key = %q, want %q", routes[0].Key, "default")
		}
		if routes[0].ErrorKey != "ErrorBoundary" {
			t.Fatalf("route error key = %q, want %q", routes[0].ErrorKey, "ErrorBoundary")
		}
	})

	t.Run("collects unresolved route for function-call module argument", func(t *testing.T) {
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
			t.Fatalf("unresolved pattern = %q, want %q", unresolved[0].Pattern, "/dynamic")
		}
		if !strings.Contains(unresolved[0].Reason, "function call") {
			t.Fatalf("unresolved reason = %q, expected function-call reason", unresolved[0].Reason)
		}
	})

}

func TestTransformRouteDefinitionsCode(t *testing.T) {
	t.Run("rewrites import() calls to static string literals", func(t *testing.T) {
		v := &vormaruntime.Vorma{
			Log: testLogger(),
		}

		transformedCode, err := transformRouteDefinitionsCode(v, []byte(`
			import { route } from "vorma/buildtime";
			route("/settings", import("./routes/settings.tsx"), "Settings");
		`))
		if err != nil {
			t.Fatalf("transformRouteDefinitionsCode returned error: %v", err)
		}
		if strings.Contains(transformedCode, "import(") {
			t.Fatalf("transformed code should not contain dynamic import(): %q", transformedCode)
		}
		if !strings.Contains(transformedCode, `"./routes/settings.tsx"`) {
			t.Fatalf("transformed code missing expected literal module path: %q", transformedCode)
		}
	})

	t.Run("returns error and logs esbuild errors for invalid input", func(t *testing.T) {
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
			t.Fatal("expected transformRouteDefinitionsCode to return an error for invalid input")
		}
		if !strings.Contains(err.Error(), "esbuild transform failed") {
			t.Fatalf("error = %q, expected esbuild transform failure", err)
		}
		if !strings.Contains(logBuffer.String(), "esbuild error:") {
			t.Fatalf("expected esbuild error log output, got %q", logBuffer.String())
		}
	})
}

func TestResolveModuleArgumentFromFunctionCall(t *testing.T) {
	t.Run("dynamic import() with non-static argument is unresolved", func(t *testing.T) {
		modulePath, unresolvedRoute, resolved := resolveModuleArgumentFromFunctionCall(
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

		if resolved {
			t.Fatal("expected call to be unresolved")
		}
		if modulePath != "" {
			t.Fatalf("module path = %q, want empty", modulePath)
		}
		if unresolvedRoute == nil {
			t.Fatal("expected unresolved route details")
		}
		if unresolvedRoute.Pattern != "/dynamic-import" {
			t.Fatalf("pattern = %q, want %q", unresolvedRoute.Pattern, "/dynamic-import")
		}
		if unresolvedRoute.RawModuleExpr != "import(...)" {
			t.Fatalf("raw module expr = %q, want %q", unresolvedRoute.RawModuleExpr, "import(...)")
		}
		if !strings.Contains(unresolvedRoute.Reason, "dynamic import() argument is not a static string") {
			t.Fatalf("reason = %q, expected dynamic-import reason", unresolvedRoute.Reason)
		}
	})
}

func TestParseClientRoutes_ResolvesStaticAndImportModules(t *testing.T) {
	rootDir := t.TempDir()
	t.Chdir(rootDir)

	routesFile := filepath.Join("frontend", "src", "vorma.routes.ts")
	mustWriteFile(t, filepath.Join("frontend", "src", "routes", "root.tsx"), []byte("export default function Root() {}"))
	mustWriteFile(t, filepath.Join("frontend", "src", "routes", "users.tsx"), []byte("export default function Users() {}"))
	mustWriteFile(t, filepath.Join("frontend", "src", "routes", "settings.tsx"), []byte("export const Settings = () => null;"))

	mustWriteFile(t, routesFile, []byte(`
		import { route } from "vorma/buildtime";
		const usersModule = "./routes/users.tsx";
		route("/", "./routes/root.tsx", "default");
		route("/users", usersModule, "default", "UsersErrorBoundary");
		route("/settings", import("./routes/settings.tsx"), "Settings");
		route("/ignored", getPath());
	`))

	v := &vormaruntime.Vorma{
		Config: &vormaruntime.VormaConfig{
			ClientRouteDefsFile: routesFile,
		},
		Log: testLogger(),
	}

	paths, err := parseClientRoutes(v)
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
		t.Fatalf("root src path = %q, want %q", root.SrcPath, "frontend/src/routes/root.tsx")
	}

	users := paths["/users"]
	if users == nil {
		t.Fatalf("missing users route")
	}
	if users.SrcPath != "frontend/src/routes/users.tsx" {
		t.Fatalf("users src path = %q, want %q", users.SrcPath, "frontend/src/routes/users.tsx")
	}
	if users.ErrorExportKey != "UsersErrorBoundary" {
		t.Fatalf("users error export key = %q, want %q", users.ErrorExportKey, "UsersErrorBoundary")
	}

	settings := paths["/settings"]
	if settings == nil {
		t.Fatalf("missing settings route")
	}
	if settings.SrcPath != "frontend/src/routes/settings.tsx" {
		t.Fatalf("settings src path = %q, want %q", settings.SrcPath, "frontend/src/routes/settings.tsx")
	}
	if settings.ExportKey != "Settings" {
		t.Fatalf("settings export key = %q, want %q", settings.ExportKey, "Settings")
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
		Config: &vormaruntime.VormaConfig{
			ClientRouteDefsFile: routesFile,
		},
		Log: testLogger(),
	}

	_, err := parseClientRoutes(v)
	if err == nil {
		t.Fatal("expected error for missing module, got nil")
	}
	if !strings.Contains(err.Error(), "component module does not exist") {
		t.Fatalf("error = %q, expected missing-module message", err)
	}
}

func TestParseClientRoutes_ReturnsErrorWhenRoutesFileDoesNotExist(t *testing.T) {
	v := &vormaruntime.Vorma{
		Config: &vormaruntime.VormaConfig{
			ClientRouteDefsFile: filepath.Join(t.TempDir(), "missing.routes.ts"),
		},
		Log: testLogger(),
	}

	_, err := parseClientRoutes(v)
	if err == nil {
		t.Fatal("expected error for missing route definitions file, got nil")
	}
	if !strings.Contains(err.Error(), "read file") {
		t.Fatalf("error = %q, expected read-file context", err)
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
		Config: &vormaruntime.VormaConfig{
			ClientRouteDefsFile: routesFile,
		},
		Log: testLogger(),
	}

	_, err := parseClientRoutes(v)
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
			name:        "direct route import",
			alias:       js.Alias{Name: []byte("route"), Binding: []byte("route")},
			wantIsRoute: true,
			wantBinding: "route",
		},
		{
			name:        "renamed route import",
			alias:       js.Alias{Name: []byte("route"), Binding: []byte("defineRoute")},
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
			name:        "non-route import",
			alias:       js.Alias{Name: []byte("somethingElse"), Binding: []byte("somethingElse")},
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
				t.Fatalf("isRouteImportAlias(%#v) = %v, want %v", testCase.alias, got, testCase.wantIsRoute)
			}
			if got := routeImportAliasBinding(testCase.alias); got != testCase.wantBinding {
				t.Fatalf("routeImportAliasBinding(%#v) = %q, want %q", testCase.alias, got, testCase.wantBinding)
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

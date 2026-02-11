package vormabuild

import (
	"path/filepath"
	"strings"
	"testing"

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

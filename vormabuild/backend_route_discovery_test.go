package vormabuild

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestParseBackendLoaderPatterns(t *testing.T) {
	t.Run("returns no patterns when server route discovery is not configured", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		t.Chdir(fixture.rootDir)

		loaderPatterns, err := parseBackendLoaderPatterns(fixture.app)
		if err != nil {
			t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
		}
		if len(loaderPatterns) != 0 {
			t.Fatalf("loader patterns length = %d, want 0", len(loaderPatterns))
		}
	})

	t.Run("discovers patterns through init-reachable calls with arbitrary helper names", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}

func registerLoaderAtPath(pattern string, loader any) any {
	return vorma.NewLoader(App, pattern, nil, decorateLoaderCtx)
}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import "github.com/vormadev/vorma"

const apiPrefix = "/api"
const usersPath = apiPrefix + "/users"

var _ = registerLoaderAtPath(usersPath, nil)
var _ = vorma.NewLoader(App, "/direct", nil, decorateLoaderCtx)

func init() {
	registerMoreRoutes()
}

func registerMoreRoutes() {
	_ = registerLoaderAtPath("/", nil)
}

func notInitReachable() {
	_ = registerLoaderAtPath("/ignored", nil)
}
`))

		loaderPatterns, err := parseBackendLoaderPatterns(fixture.app)
		if err != nil {
			t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
		}

		expectedLoaderPatterns := []string{"/", "/api/users", "/direct"}
		if len(loaderPatterns) != len(expectedLoaderPatterns) {
			t.Fatalf(
				"loader patterns length = %d, want %d (%#v)",
				len(loaderPatterns),
				len(expectedLoaderPatterns),
				loaderPatterns,
			)
		}
		for i, expectedPattern := range expectedLoaderPatterns {
			if loaderPatterns[i] != expectedPattern {
				t.Fatalf("loader pattern[%d] = %q, want %q", i, loaderPatterns[i], expectedPattern)
			}
		}
	})

	t.Run("discovers helper call chains defined in compiled same-package files outside matched roots", func(t *testing.T) {
		cfg := vormaruntime.VormaConfig{
			MainBuildEntry:       "backend/cmd/build",
			UIVariant:            string(vormaruntime.UIVariants.React),
			HTMLTemplateLocation: "entry.go.html",
			ClientEntry:          "frontend/src/vorma.entry.tsx",
			ClientRouteDefinitionPatterns: []string{
				"frontend/src/**/*vorma.routes.ts",
			},
			ServerRouteDefinitionPatterns: []string{
				"backend/src/router/routes.go",
			},
			TSGenOutDir:                "frontend/src/vorma.gen",
			BuildtimePublicURLFuncName: "waveBuildtimeURL",
		}
		fixture := newBuildTestFixture(t, &buildTestFixtureOptions{config: &cfg})
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}

func registerLoaderAtPath(pattern string) any {
	return vorma.NewLoader(App, pattern, nil, decorateLoaderCtx)
}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

var _ = registerLoaderAtPath("/from-helper")
`))

		loaderPatterns, err := parseBackendLoaderPatterns(fixture.app)
		if err != nil {
			t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
		}
		expectedLoaderPatterns := []string{"/from-helper"}
		if len(loaderPatterns) != len(expectedLoaderPatterns) {
			t.Fatalf(
				"loader patterns length = %d, want %d (%#v)",
				len(loaderPatterns),
				len(expectedLoaderPatterns),
				loaderPatterns,
			)
		}
		for i, expectedPattern := range expectedLoaderPatterns {
			if loaderPatterns[i] != expectedPattern {
				t.Fatalf("loader pattern[%d] = %q, want %q", i, loaderPatterns[i], expectedPattern)
			}
		}
	})

	t.Run("ignores import-alias shadowing that is not canonical registration", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import vormaImport "github.com/vormadev/vorma"

var _ = vormaImport.NewLoader(App, "/actual", nil, decorateLoaderCtx)

type fakeVormaRegistrar struct{}

func (fakeVormaRegistrar) NewLoader(_ any, _ string, _ any, _ any) any {
	return nil
}

func init() {
	vormaImport := fakeVormaRegistrar{}
	_ = vormaImport.NewLoader(App, "/shadowed", nil, decorateLoaderCtx)
}
`))

		loaderPatterns, err := parseBackendLoaderPatterns(fixture.app)
		if err != nil {
			t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
		}

		expectedLoaderPatterns := []string{"/actual"}
		if len(loaderPatterns) != len(expectedLoaderPatterns) {
			t.Fatalf(
				"loader patterns length = %d, want %d (%#v)",
				len(loaderPatterns),
				len(expectedLoaderPatterns),
				loaderPatterns,
			)
		}
		for i, expectedPattern := range expectedLoaderPatterns {
			if loaderPatterns[i] != expectedPattern {
				t.Fatalf("loader pattern[%d] = %q, want %q", i, loaderPatterns[i], expectedPattern)
			}
		}
	})

	t.Run("ignores helper-name shadowing and dot-import shadowing", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}

func registerLoaderAtPath(pattern string) any {
	return vorma.NewLoader(App, pattern, nil, decorateLoaderCtx)
}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import . "github.com/vormadev/vorma"

var _ = registerLoaderAtPath("/helper")
var _ = NewLoader(App, "/dot", nil, decorateLoaderCtx)

func init() {
	registerLoaderAtPath := func(pattern string) any {
		return nil
	}
	_ = registerLoaderAtPath("/helper-shadowed")

	NewLoader := func(_ any, _ string, _ any, _ any) any {
		return nil
	}
	_ = NewLoader(App, "/dot-shadowed", nil, decorateLoaderCtx)
}
`))

		loaderPatterns, err := parseBackendLoaderPatterns(fixture.app)
		if err != nil {
			t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
		}

		expectedLoaderPatterns := []string{"/dot", "/helper"}
		if len(loaderPatterns) != len(expectedLoaderPatterns) {
			t.Fatalf(
				"loader patterns length = %d, want %d (%#v)",
				len(loaderPatterns),
				len(expectedLoaderPatterns),
				loaderPatterns,
			)
		}
		for i, expectedPattern := range expectedLoaderPatterns {
			if loaderPatterns[i] != expectedPattern {
				t.Fatalf("loader pattern[%d] = %q, want %q", i, loaderPatterns[i], expectedPattern)
			}
		}
	})

	t.Run("fails when loader pattern is not compile-time resolvable", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}

func registerLoaderAtPath(pattern string, loader any) any {
	return vorma.NewLoader(App, pattern, nil, decorateLoaderCtx)
}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

var dynamicPath = patternFromRuntime()

var _ = registerLoaderAtPath(dynamicPath, nil)

func patternFromRuntime() string {
	return "/dynamic"
}
`))

		_, err := parseBackendLoaderPatterns(fixture.app)
		if err == nil {
			t.Fatal("expected parseBackendLoaderPatterns to return error")
		}
		if !strings.Contains(err.Error(), "backend/src/router/") {
			t.Fatalf("error = %q, expected backend file context", err)
		}
		if !strings.Contains(err.Error(), "pattern argument must resolve to compile-time string") {
			t.Fatalf("error = %q, expected compile-time pattern guidance", err)
		}
	})

	t.Run("fails fast when registration args reference function-local symbols", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func registerLoaderAtPath(
	pattern string,
	handler func(*vorma.LoaderReqData) (string, error),
	decorateCtx func(*vorma.LoaderReqData) *vorma.LoaderReqData,
) any {
	return vorma.NewLoader(App, pattern, handler, decorateCtx)
}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

func registerUserRoutes() {
	localDecorateCtx := func(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
		return rd
	}
	_ = registerLoaderAtPath("/users", nil, localDecorateCtx)
}

var _ = registerUserRoutes()
`))

		_, err := parseBackendLoaderPatterns(fixture.app)
		if err == nil {
			t.Fatal("expected parseBackendLoaderPatterns to return error")
		}
		if !strings.Contains(err.Error(), "function-local symbol") {
			t.Fatalf("error = %q, expected function-local symbol guidance", err)
		}
		if !strings.Contains(err.Error(), "decorateCtx argument") {
			t.Fatalf("error = %q, expected decorateCtx argument context", err)
		}
	})

	t.Run("allows wrapper-bound function literal handler arguments", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

type LoaderCtx struct{ *vorma.LoaderReqData }

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *LoaderCtx {
	return &LoaderCtx{LoaderReqData: rd}
}

func NewLoader[O any](
	pattern string,
	loader func(*LoaderCtx) (O, error),
) *vorma.Loader[O] {
	return vorma.NewLoader(App, pattern, loader, decorateLoaderCtx)
}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

var _ = NewLoader("/users", func(r *LoaderCtx) (string, error) {
	return "users", nil
})
`))

		loaderPatterns, err := parseBackendLoaderPatterns(fixture.app)
		if err != nil {
			t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
		}
		expectedLoaderPatterns := []string{"/users"}
		if len(loaderPatterns) != len(expectedLoaderPatterns) {
			t.Fatalf(
				"loader patterns length = %d, want %d (%#v)",
				len(loaderPatterns),
				len(expectedLoaderPatterns),
				loaderPatterns,
			)
		}
		for i, expectedPattern := range expectedLoaderPatterns {
			if loaderPatterns[i] != expectedPattern {
				t.Fatalf("loader pattern[%d] = %q, want %q", i, loaderPatterns[i], expectedPattern)
			}
		}
	})

	t.Run("allows wrapper-bound inline decorateCtx literals with expression-local symbols", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

type LoaderCtx struct{ *vorma.LoaderReqData }

var App = &vorma.Vorma{}

func registerLoaderAtPath[O any](
	pattern string,
	loader func(*LoaderCtx) (O, error),
	decorateCtx func(*vorma.LoaderReqData) *LoaderCtx,
) *vorma.Loader[O] {
	return vorma.NewLoader(App, pattern, loader, decorateCtx)
}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

var _ = registerLoaderAtPath(
	"/inline-decorate",
	func(r *LoaderCtx) (string, error) {
		return "ok", nil
	},
	func(rd *vorma.LoaderReqData) *LoaderCtx {
		ctx := &LoaderCtx{LoaderReqData: rd}
		return ctx
	},
)
`))

		loaderPatterns, err := parseBackendLoaderPatterns(fixture.app)
		if err != nil {
			t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
		}
		expectedLoaderPatterns := []string{"/inline-decorate"}
		if len(loaderPatterns) != len(expectedLoaderPatterns) {
			t.Fatalf(
				"loader patterns length = %d, want %d (%#v)",
				len(loaderPatterns),
				len(expectedLoaderPatterns),
				loaderPatterns,
			)
		}
		for i, expectedPattern := range expectedLoaderPatterns {
			if loaderPatterns[i] != expectedPattern {
				t.Fatalf("loader pattern[%d] = %q, want %q", i, loaderPatterns[i], expectedPattern)
			}
		}
	})

	t.Run("allows wrapper-bound inline action literals with expression-local symbols", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

type ActionCtx[I any] struct{ *vorma.ActionReqData[I] }

var App = &vorma.Vorma{}

func registerActionAtPath[I any, O any](
	method string,
	pattern string,
	action func(*ActionCtx[I]) (O, error),
	decorateCtx func(*vorma.ActionReqData[I]) *ActionCtx[I],
) *vorma.Action[I, O] {
	return vorma.NewAction(App, method, pattern, action, decorateCtx)
}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

var _ = registerActionAtPath[map[string]any, string](
	"POST",
	"/inline-action",
	func(r *ActionCtx[map[string]any]) (string, error) {
		return "ok", nil
	},
	func(rd *vorma.ActionReqData[map[string]any]) *ActionCtx[map[string]any] {
		ctx := &ActionCtx[map[string]any]{ActionReqData: rd}
		return ctx
	},
)
`))

		loaderPatterns, err := parseBackendLoaderPatterns(fixture.app)
		if err != nil {
			t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
		}
		if len(loaderPatterns) != 0 {
			t.Fatalf("loader patterns length = %d, want 0 (%#v)", len(loaderPatterns), loaderPatterns)
		}
	})

	t.Run("allows direct inline loader and action literals at package scope", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

type LoaderCtx struct{ *vorma.LoaderReqData }
type ActionCtx[I any] struct{ *vorma.ActionReqData[I] }

var App = &vorma.Vorma{}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import "github.com/vormadev/vorma"

var _ = vorma.NewLoader(
	App,
	"/direct-inline",
	func(r *LoaderCtx) (string, error) {
		value := "ok"
		return value, nil
	},
	func(rd *vorma.LoaderReqData) *LoaderCtx {
		ctx := &LoaderCtx{LoaderReqData: rd}
		return ctx
	},
)

var _ = vorma.NewAction(
	App,
	"POST",
	"/direct-inline-action",
	func(r *ActionCtx[map[string]any]) (string, error) {
		value := "ok"
		return value, nil
	},
	func(rd *vorma.ActionReqData[map[string]any]) *ActionCtx[map[string]any] {
		ctx := &ActionCtx[map[string]any]{ActionReqData: rd}
		return ctx
	},
)
`))

		loaderPatterns, err := parseBackendLoaderPatterns(fixture.app)
		if err != nil {
			t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
		}
		expectedLoaderPatterns := []string{"/direct-inline"}
		if len(loaderPatterns) != len(expectedLoaderPatterns) {
			t.Fatalf(
				"loader patterns length = %d, want %d (%#v)",
				len(loaderPatterns),
				len(expectedLoaderPatterns),
				loaderPatterns,
			)
		}
		for i, expectedPattern := range expectedLoaderPatterns {
			if loaderPatterns[i] != expectedPattern {
				t.Fatalf("loader pattern[%d] = %q, want %q", i, loaderPatterns[i], expectedPattern)
			}
		}
	})

	t.Run("discovers patterns through immediately-invoked function literals", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import "github.com/vormadev/vorma"

var _ = func() any {
	return vorma.NewLoader(App, "/iife-direct", nil, decorateLoaderCtx)
}()

var _ = func(path string) any {
	return vorma.NewLoader(App, path, nil, decorateLoaderCtx)
}("/iife-bound")
`))

		loaderPatterns, err := parseBackendLoaderPatterns(fixture.app)
		if err != nil {
			t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
		}
		expectedLoaderPatterns := []string{"/iife-bound", "/iife-direct"}
		if len(loaderPatterns) != len(expectedLoaderPatterns) {
			t.Fatalf(
				"loader patterns length = %d, want %d (%#v)",
				len(loaderPatterns),
				len(expectedLoaderPatterns),
				loaderPatterns,
			)
		}
		for i, expectedPattern := range expectedLoaderPatterns {
			if loaderPatterns[i] != expectedPattern {
				t.Fatalf("loader pattern[%d] = %q, want %q", i, loaderPatterns[i], expectedPattern)
			}
		}
	})

	t.Run("respects build tags when discovering backend loader patterns", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}
`))

		mustWriteFile(t, "backend/src/router/routes_dev.go", []byte(`
//go:build !prod

package router

import "github.com/vormadev/vorma"

var _ = vorma.NewLoader(App, "/dev-only", nil, decorateLoaderCtx)
`))

		mustWriteFile(t, "backend/src/router/routes_prod.go", []byte(`
//go:build prod

package router

import "github.com/vormadev/vorma"

var _ = vorma.NewLoader(App, "/prod-only", nil, decorateLoaderCtx)
`))

		loaderPatterns, err := parseBackendLoaderPatterns(fixture.app)
		if err != nil {
			t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
		}

		expectedLoaderPatterns := []string{"/dev-only"}
		if len(loaderPatterns) != len(expectedLoaderPatterns) {
			t.Fatalf(
				"loader patterns length = %d, want %d (%#v)",
				len(loaderPatterns),
				len(expectedLoaderPatterns),
				loaderPatterns,
			)
		}
		for i, expectedPattern := range expectedLoaderPatterns {
			if loaderPatterns[i] != expectedPattern {
				t.Fatalf("loader pattern[%d] = %q, want %q", i, loaderPatterns[i], expectedPattern)
			}
		}
	})
}

func TestResolveServerRouteDefinitionFiles(t *testing.T) {
	t.Run("returns error when configured patterns match no files", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		_, err := resolveServerRouteDefinitionFiles(fixture.app)
		if err == nil {
			t.Fatal("expected resolveServerRouteDefinitionFiles to return error")
		}
		if !strings.Contains(err.Error(), "no server route definition files matched patterns") {
			t.Fatalf("error = %q, expected no-match context", err)
		}
	})
}

func TestBackendRoutePackageAnalysis_GoTypesInitializationIsLazyAndFallbackSafe(t *testing.T) {
	t.Run("does not initialize go types when canonical calls resolve by AST import metadata", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}

func usersLoader(rd *vorma.LoaderReqData) (string, error) {
	return "ok", nil
}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import "github.com/vormadev/vorma"

var _ = vorma.NewLoader(App, "/users", usersLoader, decorateLoaderCtx)
`))

		serverRouteDefinitionFiles, err := resolveServerRouteDefinitionFiles(fixture.app)
		if err != nil {
			t.Fatalf("resolveServerRouteDefinitionFiles returned error: %v", err)
		}
		packageAnalyses, err := parseServerRouteFilesIntoPackageAnalyses(serverRouteDefinitionFiles)
		if err != nil {
			t.Fatalf("parseServerRouteFilesIntoPackageAnalyses returned error: %v", err)
		}
		if len(packageAnalyses) != 1 {
			t.Fatalf("package analysis length = %d, want 1", len(packageAnalyses))
		}

		loaderPatterns, err := packageAnalyses[0].discoverLoaderPatterns()
		if err != nil {
			t.Fatalf("discoverLoaderPatterns returned error: %v", err)
		}
		expectedLoaderPatterns := []string{"/users"}
		if len(loaderPatterns) != len(expectedLoaderPatterns) {
			t.Fatalf(
				"loader patterns length = %d, want %d (%#v)",
				len(loaderPatterns),
				len(expectedLoaderPatterns),
				loaderPatterns,
			)
		}
		if loaderPatterns[0] != expectedLoaderPatterns[0] {
			t.Fatalf("loader pattern[0] = %q, want %q", loaderPatterns[0], expectedLoaderPatterns[0])
		}

		if packageAnalyses[0].goTypesInitializationCount != 0 {
			t.Fatalf(
				"go types initialization count = %d, want 0 for AST-resolved canonical call",
				packageAnalyses[0].goTypesInitializationCount,
			)
		}
	})

	t.Run("falls back to go types when AST import metadata is unavailable", func(t *testing.T) {
		expression, err := parser.ParseExpr(`vormaAlias.NewLoader(App, "/fallback", usersLoader, decorateLoaderCtx)`)
		if err != nil {
			t.Fatalf("ParseExpr returned error: %v", err)
		}

		registrationCall, isCallExpression := expression.(*ast.CallExpr)
		if !isCallExpression {
			t.Fatalf("expression type = %T, want *ast.CallExpr", expression)
		}
		calleeExpression := unwrapGenericCalleeExpression(registrationCall.Fun)
		calleeSelectorExpression, isSelectorExpression := calleeExpression.(*ast.SelectorExpr)
		if !isSelectorExpression || calleeSelectorExpression.Sel == nil {
			t.Fatalf("callee expression type = %T, want selector with Sel", calleeExpression)
		}

		goTypesInfo := &types.Info{
			Uses: map[*ast.Ident]types.Object{
				calleeSelectorExpression.Sel: types.NewVar(
					token.NoPos,
					types.NewPackage("github.com/vormadev/vorma", "vorma"),
					"NewLoader",
					types.Typ[types.Int],
				),
			},
		}
		analysis := &backendRoutePackageAnalysis{
			goTypesInfo:         goTypesInfo,
			stringConstResolver: newGoStringConstResolver(map[string]ast.Expr{}),
		}
		parsedServerFile := &parsedServerRouteFile{
			importAliases: map[string]string{},
		}

		canonicalCall, isCanonicalCall, err := analysis.parseCanonicalRouteRegistrationCall(
			registrationCall,
			parsedServerFile,
			nil,
		)
		if err != nil {
			t.Fatalf("parseCanonicalRouteRegistrationCall returned error: %v", err)
		}
		if !isCanonicalCall || canonicalCall == nil {
			t.Fatal("expected go/types fallback to classify canonical registration call")
		}
		if canonicalCall.pattern != "/fallback" {
			t.Fatalf("canonical call pattern = %q, want %q", canonicalCall.pattern, "/fallback")
		}
	})
}

func newBackendRouteDiscoveryFixtureWithServerPatterns(t *testing.T) *buildTestFixture {
	t.Helper()

	cfg := vormaruntime.VormaConfig{
		MainBuildEntry:       "backend/cmd/build",
		UIVariant:            string(vormaruntime.UIVariants.React),
		HTMLTemplateLocation: "entry.go.html",
		ClientEntry:          "frontend/src/vorma.entry.tsx",
		ClientRouteDefinitionPatterns: []string{
			"frontend/src/**/*vorma.routes.ts",
		},
		ServerRouteDefinitionPatterns: []string{
			"backend/src/router/**/*.go",
		},
		TSGenOutDir:                "frontend/src/vorma.gen",
		BuildtimePublicURLFuncName: "waveBuildtimeURL",
	}

	return newBuildTestFixture(t, &buildTestFixtureOptions{config: &cfg})
}

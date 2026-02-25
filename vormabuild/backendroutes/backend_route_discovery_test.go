package backendroutes

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func TestParseBackendLoaderPatterns(t *testing.T) {
	t.Run(
		"returns no patterns when server route discovery is not configured",
		func(t *testing.T) {
			fixture := testkit.NewBuildTestFixture(t, nil)
			t.Chdir(fixture.RootDir)

			loaderPatterns, err := parseBackendLoaderPatterns(fixture.App)
			if err != nil {
				t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
			}
			if len(loaderPatterns) != 0 {
				t.Fatalf(
					"loader patterns length = %d, want 0",
					len(loaderPatterns),
				)
			}
		},
	)

	t.Run(
		"discovers patterns through init-reachable calls with arbitrary helper names",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}

func registerLoaderAtPath(pattern string, loader any) any {
	return vorma.DefineLoaderForRegistration(App, pattern, nil, decorateLoaderCtx)
}
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import "github.com/vormadev/vorma"

const apiPrefix = "/api"
const usersPath = apiPrefix + "/users"

var _ = registerLoaderAtPath(usersPath, nil)
var _ = vorma.DefineLoaderForRegistration(App, "/direct", nil, decorateLoaderCtx)

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

			loaderPatterns, err := parseBackendLoaderPatterns(fixture.App)
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
					t.Fatalf(
						"loader pattern[%d] = %q, want %q",
						i,
						loaderPatterns[i],
						expectedPattern,
					)
				}
			}
		},
	)

	t.Run(
		"discovers loader registrations via nestedmux.AddTaskHandler",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import (
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
)

var LoadersRouter = nestedmux.NewRouter(nil)
var LoaderHandler = mux.TaskHandlerFromFunc(func(*mux.ReqData[mux.None]) (string, error) {
	return "", nil
})
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import "github.com/vormadev/vorma/kit/nestedmux"

var _ = nestedmux.AddTaskHandler(
	LoadersRouter,
	"/nestedmux-discovered",
	LoaderHandler,
)
`))

			loaderPatterns, err := parseBackendLoaderPatterns(fixture.App)
			if err != nil {
				t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
			}
			expectedLoaderPatterns := []string{"/nestedmux-discovered"}
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
					t.Fatalf(
						"loader pattern[%d] = %q, want %q",
						i,
						loaderPatterns[i],
						expectedPattern,
					)
				}
			}
		},
	)

	t.Run(
		"discovers helper call chains defined in compiled same-package files outside matched roots",
		func(t *testing.T) {
			cfg := vormaruntime.VormaConfig{
				MainBuildEntry:       "backend/cmd/build",
				UIVariant:            string(vormaruntime.UIVariantReact),
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
			fixture := testkit.NewBuildTestFixture(
				t,
				&testkit.BuildTestFixtureOptions{Config: &cfg},
			)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}

func registerLoaderAtPath(pattern string) any {
	return vorma.DefineLoaderForRegistration(App, pattern, nil, decorateLoaderCtx)
}
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

var _ = registerLoaderAtPath("/from-helper")
`))

			loaderPatterns, err := parseBackendLoaderPatterns(fixture.App)
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
					t.Fatalf(
						"loader pattern[%d] = %q, want %q",
						i,
						loaderPatterns[i],
						expectedPattern,
					)
				}
			}
		},
	)

	t.Run(
		"ignores import-alias shadowing that is not canonical registration",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import vormaImport "github.com/vormadev/vorma"

var _ = vormaImport.DefineLoaderForRegistration(App, "/actual", nil, decorateLoaderCtx)

type fakeVormaRegistrar struct{}

func (fakeVormaRegistrar) DefineLoaderForRegistration(_ any, _ string, _ any, _ any) any {
	return nil
}

func init() {
	vormaImport := fakeVormaRegistrar{}
	_ = vormaImport.DefineLoaderForRegistration(App, "/shadowed", nil, decorateLoaderCtx)
}
`))

			loaderPatterns, err := parseBackendLoaderPatterns(fixture.App)
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
					t.Fatalf(
						"loader pattern[%d] = %q, want %q",
						i,
						loaderPatterns[i],
						expectedPattern,
					)
				}
			}
		},
	)

	t.Run(
		"ignores helper-name shadowing and dot-import shadowing",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}

func registerLoaderAtPath(pattern string) any {
	return vorma.DefineLoaderForRegistration(App, pattern, nil, decorateLoaderCtx)
}
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import . "github.com/vormadev/vorma"

var _ = registerLoaderAtPath("/helper")
var _ = DefineLoaderForRegistration(App, "/dot", nil, decorateLoaderCtx)

func init() {
	registerLoaderAtPath := func(pattern string) any {
		return nil
	}
	_ = registerLoaderAtPath("/helper-shadowed")

	DefineLoaderForRegistration := func(_ any, _ string, _ any, _ any) any {
		return nil
	}
	_ = DefineLoaderForRegistration(App, "/dot-shadowed", nil, decorateLoaderCtx)
}
`))

			loaderPatterns, err := parseBackendLoaderPatterns(fixture.App)
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
					t.Fatalf(
						"loader pattern[%d] = %q, want %q",
						i,
						loaderPatterns[i],
						expectedPattern,
					)
				}
			}
		},
	)

	t.Run(
		"fails when loader pattern is not compile-time resolvable",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}

func registerLoaderAtPath(pattern string, loader any) any {
	return vorma.DefineLoaderForRegistration(App, pattern, nil, decorateLoaderCtx)
}
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

var dynamicPath = patternFromRuntime()

var _ = registerLoaderAtPath(dynamicPath, nil)

func patternFromRuntime() string {
	return "/dynamic"
}
`))

			_, err := parseBackendLoaderPatterns(fixture.App)
			if err == nil {
				t.Fatal("expected parseBackendLoaderPatterns to return error")
			}
			if !strings.Contains(err.Error(), "backend/src/router/") {
				t.Fatalf("error = %q, expected backend file context", err)
			}
			if !strings.Contains(
				err.Error(),
				"pattern argument must resolve to compile-time string",
			) {
				t.Fatalf(
					"error = %q, expected compile-time pattern guidance",
					err,
				)
			}
		},
	)

	t.Run(
		"fails fast when registration args reference function-local symbols",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func registerLoaderAtPath(
	pattern string,
	handler func(*vorma.LoaderReqData) (string, error),
	decorateCtx func(*vorma.LoaderReqData) *vorma.LoaderReqData,
) any {
	return vorma.DefineLoaderForRegistration(App, pattern, handler, decorateCtx)
}
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

func registerUserRoutes() {
	localDecorateCtx := func(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
		return rd
	}
	_ = registerLoaderAtPath("/users", nil, localDecorateCtx)
}

var _ = registerUserRoutes()
`))

			_, err := parseBackendLoaderPatterns(fixture.App)
			if err == nil {
				t.Fatal("expected parseBackendLoaderPatterns to return error")
			}
			if !strings.Contains(err.Error(), "function-local symbol") {
				t.Fatalf(
					"error = %q, expected function-local symbol guidance",
					err,
				)
			}
			if !strings.Contains(err.Error(), "decorateCtx argument") {
				t.Fatalf(
					"error = %q, expected decorateCtx argument context",
					err,
				)
			}
		},
	)

	t.Run(
		"allows wrapper-bound function literal handler arguments",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

type LoaderCtx struct{ *vorma.LoaderReqData }

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *LoaderCtx {
	return &LoaderCtx{LoaderReqData: rd}
}

func DefineLoaderForRegistration[O any](
	pattern string,
	loader func(*LoaderCtx) (O, error),
) *vorma.Loader[O] {
	return vorma.DefineLoaderForRegistration(App, pattern, loader, decorateLoaderCtx)
}
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

var _ = DefineLoaderForRegistration("/users", func(r *LoaderCtx) (string, error) {
	return "users", nil
})
`))

			loaderPatterns, err := parseBackendLoaderPatterns(fixture.App)
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
					t.Fatalf(
						"loader pattern[%d] = %q, want %q",
						i,
						loaderPatterns[i],
						expectedPattern,
					)
				}
			}
		},
	)

	t.Run(
		"allows wrapper-bound inline decorateCtx literals with expression-local symbols",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

type LoaderCtx struct{ *vorma.LoaderReqData }

var App = &vorma.Vorma{}

func registerLoaderAtPath[O any](
	pattern string,
	loader func(*LoaderCtx) (O, error),
	decorateCtx func(*vorma.LoaderReqData) *LoaderCtx,
) *vorma.Loader[O] {
	return vorma.DefineLoaderForRegistration(App, pattern, loader, decorateCtx)
}
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
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

			loaderPatterns, err := parseBackendLoaderPatterns(fixture.App)
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
					t.Fatalf(
						"loader pattern[%d] = %q, want %q",
						i,
						loaderPatterns[i],
						expectedPattern,
					)
				}
			}
		},
	)

	t.Run(
		"allows wrapper-bound inline action literals with expression-local symbols",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
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
	return vorma.DefineActionForRegistration(App, method, pattern, action, decorateCtx)
}
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
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

			loaderPatterns, err := parseBackendLoaderPatterns(fixture.App)
			if err != nil {
				t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
			}
			if len(loaderPatterns) != 0 {
				t.Fatalf(
					"loader patterns length = %d, want 0 (%#v)",
					len(loaderPatterns),
					loaderPatterns,
				)
			}
		},
	)

	t.Run(
		"allows direct inline loader and action literals at package scope",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

type LoaderCtx struct{ *vorma.LoaderReqData }
type ActionCtx[I any] struct{ *vorma.ActionReqData[I] }

var App = &vorma.Vorma{}
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import "github.com/vormadev/vorma"

var _ = vorma.DefineLoaderForRegistration(
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

var _ = vorma.DefineActionForRegistration(
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

			loaderPatterns, err := parseBackendLoaderPatterns(fixture.App)
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
					t.Fatalf(
						"loader pattern[%d] = %q, want %q",
						i,
						loaderPatterns[i],
						expectedPattern,
					)
				}
			}
		},
	)

	t.Run(
		"discovers patterns through immediately-invoked function literals",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import "github.com/vormadev/vorma"

var _ = func() any {
	return vorma.DefineLoaderForRegistration(App, "/iife-direct", nil, decorateLoaderCtx)
}()

var _ = func(path string) any {
	return vorma.DefineLoaderForRegistration(App, path, nil, decorateLoaderCtx)
}("/iife-bound")
`))

			loaderPatterns, err := parseBackendLoaderPatterns(fixture.App)
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
					t.Fatalf(
						"loader pattern[%d] = %q, want %q",
						i,
						loaderPatterns[i],
						expectedPattern,
					)
				}
			}
		},
	)

	t.Run(
		"respects build tags when discovering backend loader patterns",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}
`))

			testkit.MustWriteFile(
				t,
				"backend/src/router/routes_dev.go",
				[]byte(`
//go:build !prod

package router

import "github.com/vormadev/vorma"

var _ = vorma.DefineLoaderForRegistration(App, "/dev-only", nil, decorateLoaderCtx)
`),
			)

			testkit.MustWriteFile(
				t,
				"backend/src/router/routes_prod.go",
				[]byte(`
//go:build prod

package router

import "github.com/vormadev/vorma"

var _ = vorma.DefineLoaderForRegistration(App, "/prod-only", nil, decorateLoaderCtx)
`),
			)

			loaderPatterns, err := parseBackendLoaderPatterns(fixture.App)
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
					t.Fatalf(
						"loader pattern[%d] = %q, want %q",
						i,
						loaderPatterns[i],
						expectedPattern,
					)
				}
			}
		},
	)
}

func TestResolveServerRouteDefinitionFiles(t *testing.T) {
	t.Run(
		"returns error when configured patterns match no files",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			_, err := resolveServerRouteDefinitionFiles(fixture.App)
			if err == nil {
				t.Fatal(
					"expected resolveServerRouteDefinitionFiles to return error",
				)
			}
			if !strings.Contains(
				err.Error(),
				"no server route definition files matched patterns",
			) {
				t.Fatalf("error = %q, expected no-match context", err)
			}
		},
	)
}

func newBackendRouteDiscoveryFixtureWithServerPatterns(
	t *testing.T,
) *testkit.BuildTestFixture {
	t.Helper()

	cfg := vormaruntime.VormaConfig{
		MainBuildEntry:       "backend/cmd/build",
		UIVariant:            string(vormaruntime.UIVariantReact),
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

	return testkit.NewBuildTestFixture(
		t,
		&testkit.BuildTestFixtureOptions{Config: &cfg},
	)
}

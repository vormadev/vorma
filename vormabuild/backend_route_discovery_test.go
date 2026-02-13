package vormabuild

import (
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

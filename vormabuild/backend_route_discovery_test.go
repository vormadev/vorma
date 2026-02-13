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

	t.Run("discovers loader patterns from top-level route declarations", func(t *testing.T) {
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
		fixture := newBuildTestFixture(t, &buildTestFixtureOptions{
			config: &cfg,
		})
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/helpers.go", []byte(`
package router

const apiPrefix = "/api"
const usersPath = apiPrefix + "/users"
`))
		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

const rootPath = "/"

var _ = NewLoader(rootPath, nil)
var _ = NewLoader(usersPath, nil)
var _ = NewAction("POST", usersPath, nil)
`))
		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

func NewLoader(pattern string, f any) any { return nil }

func helperRegistration() {
	_ = NewLoader(patternFromRuntime(), nil)
}

func patternFromRuntime() string { return "/" }
`))

		loaderPatterns, err := parseBackendLoaderPatterns(fixture.app)
		if err != nil {
			t.Fatalf("parseBackendLoaderPatterns returned error: %v", err)
		}

		expectedLoaderPatterns := []string{"/", "/api/users"}
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

	t.Run("fails when top-level loader pattern is non-static", func(t *testing.T) {
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
		fixture := newBuildTestFixture(t, &buildTestFixtureOptions{
			config: &cfg,
		})
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

var dynamicPath = "/dynamic"
var _ = NewLoader(dynamicPath, nil)
`))

		_, err := parseBackendLoaderPatterns(fixture.app)
		if err == nil {
			t.Fatal("expected parseBackendLoaderPatterns to return error")
		}
		if !strings.Contains(err.Error(), "backend/src/router/routes.go:") {
			t.Fatalf("error = %q, expected file and line context", err)
		}
		if !strings.Contains(err.Error(), "NewLoader pattern must be a string literal or string const") {
			t.Fatalf("error = %q, expected static-pattern guidance", err)
		}
	})
}

func TestResolveServerRouteDefinitionFiles(t *testing.T) {
	t.Run("returns error when configured patterns match no files", func(t *testing.T) {
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
		fixture := newBuildTestFixture(t, &buildTestFixtureOptions{
			config: &cfg,
		})
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

package vormabuild

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

type goOverlayReplaceConfigForTest struct {
	Replace map[string]string `json:"Replace"`
}

func readOverlayReplacementSourceForGeneratedTargetPath(
	t *testing.T,
	overlay *discoveredRouteRegistrarOverlay,
	generatedTargetPath string,
) string {
	t.Helper()

	if overlay == nil {
		t.Fatal("expected non-nil discovered route registrar overlay")
	}

	overlayConfigBytes, err := os.ReadFile(overlay.goOverlayConfigPath)
	if err != nil {
		t.Fatalf("read overlay config %q: %v", overlay.goOverlayConfigPath, err)
	}
	var overlayConfig goOverlayReplaceConfigForTest
	if err := json.Unmarshal(overlayConfigBytes, &overlayConfig); err != nil {
		t.Fatalf("unmarshal overlay config: %v", err)
	}

	absoluteGeneratedTargetPath, err := filepath.Abs(generatedTargetPath)
	if err != nil {
		t.Fatalf("resolve expected overlay target path: %v", err)
	}
	normalizedGeneratedTargetPath := filepath.ToSlash(filepath.Clean(absoluteGeneratedTargetPath))

	overlayReplacementSourcePath, hasReplacement := overlayConfig.Replace[normalizedGeneratedTargetPath]
	if !hasReplacement {
		t.Fatalf(
			"overlay config missing generated registrar target path %q; got keys %#v",
			normalizedGeneratedTargetPath,
			overlayConfig.Replace,
		)
	}

	overlayReplacementSourceBytes, err := os.ReadFile(overlayReplacementSourcePath)
	if err != nil {
		t.Fatalf("read overlay source %q: %v", overlayReplacementSourcePath, err)
	}
	return string(overlayReplacementSourceBytes)
}

func TestDiscoveredRouteRegistrarDiscoveryCacheKey_IsStableForSameProjectIdentity(t *testing.T) {
	fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)

	appOne := fixture.app
	appTwo := vormaruntime.NewVormaApp(vormaruntime.VormaAppConfig{
		Wave:   appOne.Wave,
		Logger: testLogger(),
	})

	cacheKeyOne := discoveredRouteRegistrarDiscoveryCacheKey(appOne)
	cacheKeyTwo := discoveredRouteRegistrarDiscoveryCacheKey(appTwo)
	if cacheKeyOne != cacheKeyTwo {
		t.Fatalf(
			"cache key for same project identity should be stable across app instances: %q vs %q",
			cacheKeyOne,
			cacheKeyTwo,
		)
	}
}

func TestPrepareDiscoveredRouteRegistrarOverlay(t *testing.T) {
	t.Run("returns nil when no discovered registrations exist", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}
`))

		overlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.app)
		if err != nil {
			t.Fatalf("prepareDiscoveredRouteRegistrarOverlay returned error: %v", err)
		}
		if overlay != nil {
			t.Fatal("expected no overlay when no discovered registrations exist")
		}
	})

	t.Run("prepares overlay sources without writing consumer registrar files", func(t *testing.T) {
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}

func decorateActionCtx(rd *vorma.ActionReqData[map[string]any]) *vorma.ActionReqData[map[string]any] {
	return rd
}

func registerLoaderAtPath(pattern string, loader any) any {
	return vorma.NewLoader(App, pattern, nil, decorateLoaderCtx)
}

func registerActionAtPath(method string, pattern string, action any) any {
	return vorma.NewAction(App, method, pattern, nil, decorateActionCtx)
}
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

const apiPrefix = "/api"

var _ = registerLoaderAtPath(apiPrefix + "/users", nil)
var _ = registerActionAtPath("POST", "/submit", nil)
`))

		overlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.app)
		if err != nil {
			t.Fatalf("prepareDiscoveredRouteRegistrarOverlay returned error: %v", err)
		}
		if overlay == nil {
			t.Fatal("expected overlay for discovered registrations")
		}

		generatedFilePath := filepath.Join(
			fixture.rootDir,
			"backend/src/router",
			discoveredRouteRegistrarGeneratedFilename,
		)
		if _, err := os.Stat(generatedFilePath); !os.IsNotExist(err) {
			t.Fatalf(
				"expected no consumer-visible generated registrar file, stat err=%v",
				err,
			)
		}

		overlayConfigBytes, err := os.ReadFile(overlay.goOverlayConfigPath)
		if err != nil {
			t.Fatalf("read overlay config %q: %v", overlay.goOverlayConfigPath, err)
		}
		var overlayConfig goOverlayReplaceConfigForTest
		if err := json.Unmarshal(overlayConfigBytes, &overlayConfig); err != nil {
			t.Fatalf("unmarshal overlay config: %v", err)
		}

		expectedOverlayTargetPath, err := filepath.Abs(generatedFilePath)
		if err != nil {
			t.Fatalf("resolve expected overlay target path: %v", err)
		}
		overlayReplacementSourcePath, hasReplacement := overlayConfig.Replace[filepath.ToSlash(expectedOverlayTargetPath)]
		if !hasReplacement {
			t.Fatalf(
				"overlay config missing generated registrar target path %q; got keys %#v",
				filepath.ToSlash(expectedOverlayTargetPath),
				overlayConfig.Replace,
			)
		}

		overlayReplacementSourceBytes, err := os.ReadFile(overlayReplacementSourcePath)
		if err != nil {
			t.Fatalf("read overlay source %q: %v", overlayReplacementSourcePath, err)
		}
		overlayReplacementSource := string(overlayReplacementSourceBytes)
		if !strings.Contains(
			overlayReplacementSource,
			`vorma.Internal__RegisterDiscoveredLoader(App, "/api/users", nil, decorateLoaderCtx)`,
		) {
			t.Fatalf("overlay source missing loader registration call:\n%s", overlayReplacementSource)
		}
		if !strings.Contains(
			overlayReplacementSource,
			`vorma.Internal__RegisterDiscoveredAction(App, "POST", "/submit", nil, decorateActionCtx)`,
		) {
			t.Fatalf("overlay source missing action registration call:\n%s", overlayReplacementSource)
		}

		overlayDirectoryPath := filepath.Dir(overlay.goOverlayConfigPath)
		if err := overlay.cleanup(); err != nil {
			t.Fatalf("cleanup overlay temporary files: %v", err)
		}
		if _, err := os.Stat(overlayDirectoryPath); !os.IsNotExist(err) {
			t.Fatalf("expected overlay temp dir to be removed, stat err=%v", err)
		}
	})

	t.Run("fails fast on unqualified external symbols in discovered expressions", func(t *testing.T) {
		expression, err := parser.ParseExpr("ExternalSymbol")
		if err != nil {
			t.Fatalf("parse expression: %v", err)
		}
		expressionIdentifier, isExpressionIdentifier := expression.(*ast.Ident)
		if !isExpressionIdentifier {
			t.Fatalf("expression type = %T, want *ast.Ident", expression)
		}

		externalPackage := types.NewPackage("example.com/dothelpers", "dothelpers")
		externalObject := types.NewVar(
			token.NoPos,
			externalPackage,
			"ExternalSymbol",
			types.Typ[types.String],
		)
		analysis := &backendRoutePackageAnalysis{
			goTypesInfo: &types.Info{
				Uses: map[*ast.Ident]types.Object{
					expressionIdentifier: externalObject,
				},
			},
			goTypesPackagePath: "example.com/localpkg",
		}

		err = analysis.collectRequiredImportsForExpression(expression, map[string]string{})
		if err == nil {
			t.Fatal("expected collectRequiredImportsForExpression to return error")
		}
		if !strings.Contains(err.Error(), "unqualified external symbol") {
			t.Fatalf("error = %q, expected unqualified external symbol guidance", err)
		}
		if !strings.Contains(err.Error(), "dot-imported symbols") {
			t.Fatalf("error = %q, expected dot-import guidance", err)
		}
	})

	t.Run("fails fast on conflicting import aliases across discovered expressions", func(t *testing.T) {
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

		mustWriteFile(t, "backend/src/router/routes_a.go", []byte(`
package router

import (
	foo "bytes"

	"github.com/vormadev/vorma"
)

var _ = vorma.NewLoader(App, "/a", foo.Clone, decorateLoaderCtx)
`))

		mustWriteFile(t, "backend/src/router/routes_b.go", []byte(`
package router

import (
	foo "strings"

	"github.com/vormadev/vorma"
)

var _ = vorma.NewLoader(App, "/b", foo.Clone, decorateLoaderCtx)
`))

		_, err := prepareDiscoveredRouteRegistrarOverlay(fixture.app)
		if err == nil {
			t.Fatal("expected prepareDiscoveredRouteRegistrarOverlay to return error")
		}
		if !strings.Contains(err.Error(), "conflicting import alias") {
			t.Fatalf("error = %q, expected conflicting import alias guidance", err)
		}
	})

	t.Run("caches discovered registrar artifacts when source fingerprint is unchanged", func(t *testing.T) {
		discoveredRouteRegistrarArtifactsCache.clear()
		t.Cleanup(func() {
			discoveredRouteRegistrarArtifactsCache.clear()
		})

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

var _ = vorma.NewLoader(App, "/cached", usersLoader, decorateLoaderCtx)
`))

		parseGoSourceASTCallCount := 0
		originalParseGoSourceAST := backendRouteDiscoveryDeps.parseGoSourceAST
		backendRouteDiscoveryDeps.parseGoSourceAST = func(
			fileSet *token.FileSet,
			filename string,
			src any,
			mode parser.Mode,
		) (*ast.File, error) {
			parseGoSourceASTCallCount++
			return originalParseGoSourceAST(fileSet, filename, src, mode)
		}
		t.Cleanup(func() {
			backendRouteDiscoveryDeps.parseGoSourceAST = originalParseGoSourceAST
		})

		firstOverlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.app)
		if err != nil {
			t.Fatalf("prepareDiscoveredRouteRegistrarOverlay returned error: %v", err)
		}
		if firstOverlay == nil {
			t.Fatal("expected overlay for discovered registrations")
		}
		firstParseGoSourceASTCallCount := parseGoSourceASTCallCount
		if firstParseGoSourceASTCallCount == 0 {
			t.Fatal("expected parseServerRouteFilesIntoPackageAnalyses to parse Go source files")
		}
		if err := firstOverlay.cleanup(); err != nil {
			t.Fatalf("cleanup first discovered overlay: %v", err)
		}

		secondOverlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.app)
		if err != nil {
			t.Fatalf("prepareDiscoveredRouteRegistrarOverlay second call returned error: %v", err)
		}
		if secondOverlay == nil {
			t.Fatal("expected overlay for discovered registrations on second call")
		}
		if parseGoSourceASTCallCount != firstParseGoSourceASTCallCount {
			t.Fatalf(
				"parseGoSourceAST call count after cache hit = %d, want %d",
				parseGoSourceASTCallCount,
				firstParseGoSourceASTCallCount,
			)
		}
		if err := secondOverlay.cleanup(); err != nil {
			t.Fatalf("cleanup second discovered overlay: %v", err)
		}
	})

	t.Run("invalidates cached discovered registrar artifacts when package source changes", func(t *testing.T) {
		discoveredRouteRegistrarArtifactsCache.clear()
		t.Cleanup(func() {
			discoveredRouteRegistrarArtifactsCache.clear()
		})

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

		routesFilePath := "backend/src/router/routes.go"
		mustWriteFile(t, routesFilePath, []byte(`
package router

import "github.com/vormadev/vorma"

var _ = vorma.NewLoader(App, "/before", usersLoader, decorateLoaderCtx)
`))

		parseGoSourceASTCallCount := 0
		originalParseGoSourceAST := backendRouteDiscoveryDeps.parseGoSourceAST
		backendRouteDiscoveryDeps.parseGoSourceAST = func(
			fileSet *token.FileSet,
			filename string,
			src any,
			mode parser.Mode,
		) (*ast.File, error) {
			parseGoSourceASTCallCount++
			return originalParseGoSourceAST(fileSet, filename, src, mode)
		}
		t.Cleanup(func() {
			backendRouteDiscoveryDeps.parseGoSourceAST = originalParseGoSourceAST
		})

		firstOverlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.app)
		if err != nil {
			t.Fatalf("prepareDiscoveredRouteRegistrarOverlay first call returned error: %v", err)
		}
		if firstOverlay == nil {
			t.Fatal("expected overlay for discovered registrations")
		}
		generatedFilePath := filepath.Join(
			fixture.rootDir,
			"backend/src/router",
			discoveredRouteRegistrarGeneratedFilename,
		)
		firstOverlaySource := readOverlayReplacementSourceForGeneratedTargetPath(
			t,
			firstOverlay,
			generatedFilePath,
		)
		if !strings.Contains(firstOverlaySource, `"/before"`) {
			t.Fatalf("first overlay source missing /before pattern:\n%s", firstOverlaySource)
		}
		firstParseGoSourceASTCallCount := parseGoSourceASTCallCount
		if err := firstOverlay.cleanup(); err != nil {
			t.Fatalf("cleanup first discovered overlay: %v", err)
		}

		mustWriteFile(t, routesFilePath, []byte(`
package router

import "github.com/vormadev/vorma"

var _ = vorma.NewLoader(App, "/after", usersLoader, decorateLoaderCtx)
`))

		secondOverlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.app)
		if err != nil {
			t.Fatalf("prepareDiscoveredRouteRegistrarOverlay second call returned error: %v", err)
		}
		if secondOverlay == nil {
			t.Fatal("expected overlay for discovered registrations after source change")
		}
		secondOverlaySource := readOverlayReplacementSourceForGeneratedTargetPath(
			t,
			secondOverlay,
			generatedFilePath,
		)
		if !strings.Contains(secondOverlaySource, `"/after"`) {
			t.Fatalf("second overlay source missing /after pattern:\n%s", secondOverlaySource)
		}
		if parseGoSourceASTCallCount <= firstParseGoSourceASTCallCount {
			t.Fatalf(
				"expected cache invalidation to re-run Go source parsing; parse call count after change = %d, before change = %d",
				parseGoSourceASTCallCount,
				firstParseGoSourceASTCallCount,
			)
		}
		if err := secondOverlay.cleanup(); err != nil {
			t.Fatalf("cleanup second discovered overlay: %v", err)
		}
	})

	t.Run("dependency-only handler changes are reflected under cached discovery artifacts", func(t *testing.T) {
		discoveredRouteRegistrarArtifactsCache.clear()
		t.Cleanup(func() {
			discoveredRouteRegistrarArtifactsCache.clear()
		})

		repositoryRootDir := mustResolveRepositoryRootDir(t)
		fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
		t.Chdir(fixture.rootDir)

		mustWriteFile(t, "go.mod", []byte(fmt.Sprintf(`
module cachee2e

go 1.24

require github.com/vormadev/vorma v0.0.0

replace github.com/vormadev/vorma => %s
`, filepath.ToSlash(repositoryRootDir))))

		mustWriteFile(t, "backend/wave.go", []byte(`
package backend

import (
	"os"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/wave"
)

var waveFS = os.DirFS(".")

var Wave = wave.New(wave.Config{
	WaveConfigJSON: fsutil.MustReadFile(waveFS, "backend/wave.config.json"),
	DistStaticFS:   fsutil.MustSub(waveFS, "backend", "dist", "static"),
})
`))

		mustWriteFile(t, "backend/wave.config.json", []byte(`
{
	"Core": {
		"MainAppEntry": "backend/cmd/check",
		"DistDir": "backend/dist",
		"StaticAssetDirs": {
			"Private": "backend/assets/private",
			"Public": "backend/assets/public"
		},
		"PublicPathPrefix": "/",
		"ServerOnlyMode": true
	},
	"Vorma": {
		"MainBuildEntry": "backend/cmd/build",
		"UIVariant": "react",
		"HTMLTemplateLocation": "entry.go.html",
		"ClientEntry": "frontend/src/vorma.entry.tsx",
		"ClientRouteDefinitionPatterns": ["frontend/src/**/*vorma.routes.ts"],
		"ServerRouteDefinitionPatterns": ["backend/src/router/**/*.go"],
		"TSGenOutDir": "frontend/src/vorma.gen",
		"BuildtimePublicURLFuncName": "waveBuildtimeURL"
	}
}
`))

		mustWriteFile(
			t,
			filepath.Join("backend", "assets", "private", "entry.go.html"),
			[]byte("<!doctype html><html><body></body></html>"),
		)

		mustWriteFile(t, "backend/src/app/app.go", []byte(`
package app

import (
	"cachee2e/backend"

	"github.com/vormadev/vorma"
)

var App = vorma.NewVormaApp(vorma.VormaAppConfig{
	Wave: backend.Wave,
})
`))

		mustWriteFile(t, "backend/src/lib/version/version.go", []byte(`
package version

func LatestVersion() string {
	return "v1"
}
`))

		mustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import (
	"cachee2e/backend/src/app"
	"cachee2e/backend/src/lib/version"

	"github.com/vormadev/vorma"
)

type LoaderCtx struct {
	*vorma.LoaderReqData
}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *LoaderCtx {
	return &LoaderCtx{LoaderReqData: rd}
}

func usersLoader(*LoaderCtx) (string, error) {
	return version.LatestVersion(), nil
}

var App = app.App
`))

		mustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import "github.com/vormadev/vorma"

var _ = vorma.NewLoader(App, "/version", usersLoader, decorateLoaderCtx)
`))

		mustWriteFile(t, "backend/cmd/check/main.go", []byte(`
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"cachee2e/backend/src/app"
	_ "cachee2e/backend/src/router"

	"github.com/vormadev/vorma/kit/mux"
)

func main() {
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	var reqWithTasksCtx *http.Request
	bootstrapHandler := mux.InjectTasksCtxMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		reqWithTasksCtx = r
	}))
	bootstrapHandler.ServeHTTP(httptest.NewRecorder(), req)

	if reqWithTasksCtx == nil {
		fmt.Println("missing tasks context")
		os.Exit(1)
	}

	results, found := mux.FindNestedMatchesAndRunTasks(app.App.LoadersRouter().NestedRouter, reqWithTasksCtx)
	if !found || results == nil {
		fmt.Println("route not found")
		os.Exit(1)
	}

	result := results.Map["/version"]
	if result == nil {
		fmt.Println("missing /version result")
		os.Exit(1)
	}
	if result.Err() != nil {
		fmt.Printf("loader err: %v\n", result.Err())
		os.Exit(1)
	}

	value, ok := result.Data().(string)
	if !ok {
		fmt.Printf("unexpected loader output type: %T\n", result.Data())
		os.Exit(1)
	}
	fmt.Println(value)
}
`))

		parseGoSourceASTCallCount := 0
		originalParseGoSourceAST := backendRouteDiscoveryDeps.parseGoSourceAST
		backendRouteDiscoveryDeps.parseGoSourceAST = func(
			fileSet *token.FileSet,
			filename string,
			src any,
			mode parser.Mode,
		) (*ast.File, error) {
			parseGoSourceASTCallCount++
			return originalParseGoSourceAST(fileSet, filename, src, mode)
		}
		t.Cleanup(func() {
			backendRouteDiscoveryDeps.parseGoSourceAST = originalParseGoSourceAST
		})

		firstOverlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.app)
		if err != nil {
			t.Fatalf("prepareDiscoveredRouteRegistrarOverlay first call returned error: %v", err)
		}
		if firstOverlay == nil {
			t.Fatal("expected first discovered overlay")
		}
		firstParseGoSourceASTCallCount := parseGoSourceASTCallCount
		if firstParseGoSourceASTCallCount == 0 {
			t.Fatal("expected first overlay preparation to parse Go source")
		}

		compiledBinaryPath := filepath.Join(fixture.rootDir, "backend", "dist", "check")
		if runtime.GOOS == "windows" {
			compiledBinaryPath += ".exe"
		}

		firstBuildOutput, firstBuildErr := runCommandAndCaptureOutput(
			fixture.rootDir,
			"go",
			"build",
			"-mod=mod",
			"-overlay="+firstOverlay.goOverlayConfigPath,
			"-o",
			compiledBinaryPath,
			"./backend/cmd/check",
		)
		if cleanupErr := firstOverlay.cleanup(); cleanupErr != nil {
			t.Fatalf("cleanup first discovered overlay: %v", cleanupErr)
		}
		if firstBuildErr != nil {
			t.Fatalf("first go build failed: %v\n%s", firstBuildErr, firstBuildOutput)
		}

		firstRuntimeOutput, firstRuntimeErr := runCommandAndCaptureOutput(
			fixture.rootDir,
			compiledBinaryPath,
		)
		if firstRuntimeErr != nil {
			t.Fatalf("first compiled runtime check failed: %v\n%s", firstRuntimeErr, firstRuntimeOutput)
		}
		if !strings.Contains(firstRuntimeOutput, "v1") {
			t.Fatalf("first runtime output = %q, expected version marker %q", firstRuntimeOutput, "v1")
		}

		mustWriteFile(t, "backend/src/lib/version/version.go", []byte(`
package version

func LatestVersion() string {
	return "v2"
}
`))

		secondOverlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.app)
		if err != nil {
			t.Fatalf("prepareDiscoveredRouteRegistrarOverlay second call returned error: %v", err)
		}
		if secondOverlay == nil {
			t.Fatal("expected second discovered overlay")
		}
		if parseGoSourceASTCallCount != firstParseGoSourceASTCallCount {
			t.Fatalf(
				"parseGoSourceAST call count after dependency-only change = %d, want cached count %d",
				parseGoSourceASTCallCount,
				firstParseGoSourceASTCallCount,
			)
		}

		secondBuildOutput, secondBuildErr := runCommandAndCaptureOutput(
			fixture.rootDir,
			"go",
			"build",
			"-mod=mod",
			"-overlay="+secondOverlay.goOverlayConfigPath,
			"-o",
			compiledBinaryPath,
			"./backend/cmd/check",
		)
		if cleanupErr := secondOverlay.cleanup(); cleanupErr != nil {
			t.Fatalf("cleanup second discovered overlay: %v", cleanupErr)
		}
		if secondBuildErr != nil {
			t.Fatalf("second go build failed: %v\n%s", secondBuildErr, secondBuildOutput)
		}

		secondRuntimeOutput, secondRuntimeErr := runCommandAndCaptureOutput(
			fixture.rootDir,
			compiledBinaryPath,
		)
		if secondRuntimeErr != nil {
			t.Fatalf("second compiled runtime check failed: %v\n%s", secondRuntimeErr, secondRuntimeOutput)
		}
		if !strings.Contains(secondRuntimeOutput, "v2") {
			t.Fatalf("second runtime output = %q, expected version marker %q", secondRuntimeOutput, "v2")
		}
	})
}

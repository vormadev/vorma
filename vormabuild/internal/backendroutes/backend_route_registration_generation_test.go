package backendroutes

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/internal/backendroutes/registraroverlay"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
	"github.com/vormadev/vorma/wave/waveartifacts"
)

type goOverlayReplaceConfigForTest struct {
	Replace map[string]string `json:"Replace"`
}

func discoveryDependenciesWithParseGoSourceASTCounter(
	parseGoSourceASTCallCount *int,
) backendRouteDiscoveryDependencies {
	dependencies := defaultBackendRouteDiscoveryDependencies()
	originalParseGoSourceAST := dependencies.parseGoSourceAST
	dependencies.parseGoSourceAST = func(
		fileSet *token.FileSet,
		filename string,
		src any,
		mode parser.Mode,
	) (*ast.File, error) {
		*parseGoSourceASTCallCount += 1
		return originalParseGoSourceAST(fileSet, filename, src, mode)
	}
	return dependencies
}

func readOverlayReplacementSourceForGeneratedTargetPath(
	t *testing.T,
	overlay *registraroverlay.DiscoveredRouteRegistrarOverlay,
	generatedTargetPath string,
) string {
	t.Helper()

	if overlay == nil {
		t.Fatal("expected non-nil discovered route registrar overlay")
	}

	overlayConfigBytes, err := os.ReadFile(overlay.GoOverlayConfigPath())
	if err != nil {
		t.Fatalf(
			"read overlay config %q: %v",
			overlay.GoOverlayConfigPath(),
			err,
		)
	}
	var overlayConfig goOverlayReplaceConfigForTest
	if err := json.Unmarshal(overlayConfigBytes, &overlayConfig); err != nil {
		t.Fatalf("unmarshal overlay Config: %v", err)
	}

	absoluteGeneratedTargetPath, err := filepath.Abs(generatedTargetPath)
	if err != nil {
		t.Fatalf("resolve expected overlay target path: %v", err)
	}
	normalizedGeneratedTargetPath := filepath.ToSlash(
		filepath.Clean(absoluteGeneratedTargetPath),
	)

	overlayReplacementSourcePath, hasReplacement := overlayConfig.Replace[normalizedGeneratedTargetPath]
	if !hasReplacement {
		t.Fatalf(
			"overlay config missing generated registrar target path %q; got keys %#v",
			normalizedGeneratedTargetPath,
			overlayConfig.Replace,
		)
	}

	overlayReplacementSourceBytes, err := os.ReadFile(
		overlayReplacementSourcePath,
	)
	if err != nil {
		t.Fatalf(
			"read overlay source %q: %v",
			overlayReplacementSourcePath,
			err,
		)
	}
	return string(overlayReplacementSourceBytes)
}

func TestDiscoveredRouteRegistrarDiscoveryCacheKey_IsStableForSameProjectIdentity(
	t *testing.T,
) {
	fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)

	appOne := fixture.App
	appTwo := vormaruntime.NewVormaApp(vormaruntime.VormaAppConfig{
		Wave:   appOne.Wave,
		Logger: testkit.TestLogger(),
	})

	cacheKeyOne := registraroverlay.DiscoveryCacheKey(appOne)
	cacheKeyTwo := registraroverlay.DiscoveryCacheKey(appTwo)
	if cacheKeyOne != cacheKeyTwo {
		t.Fatalf(
			"cache key for same project identity should be stable across app instances: %q vs %q",
			cacheKeyOne,
			cacheKeyTwo,
		)
	}
}

func TestDiscoveredRouteRegistrarArtifactCache(t *testing.T) {
	t.Run("cache hit returns cloned artifacts", func(t *testing.T) {
		cache := registraroverlay.NewDiscoveredRouteRegistrarArtifactCache(4)
		cache.Set(
			"cache-key",
			"fp-one",
			[]registraroverlay.SourceArtifact{
				{
					TargetFilePath: "backend/src/router/discovered_route_registrar_0.gen.go",
					SourceBytes:    []byte("first"),
				},
			},
		)

		firstReadArtifacts, firstReadHit := cache.Get("cache-key", "fp-one")
		if !firstReadHit {
			t.Fatal("expected cache hit for matching fingerprint")
		}
		if len(firstReadArtifacts) != 1 {
			t.Fatalf(
				"first read artifacts length = %d, want 1",
				len(firstReadArtifacts),
			)
		}
		firstReadArtifacts[0].SourceBytes[0] = 'X'

		secondReadArtifacts, secondReadHit := cache.Get("cache-key", "fp-one")
		if !secondReadHit {
			t.Fatal("expected second cache hit for matching fingerprint")
		}
		if string(secondReadArtifacts[0].SourceBytes) != "first" {
			t.Fatalf(
				"cached artifacts should be immutable clones, got %q",
				string(secondReadArtifacts[0].SourceBytes),
			)
		}
	})

	t.Run(
		"cache miss for mismatched fingerprint invalidates stale entry",
		func(t *testing.T) {
			cache := registraroverlay.NewDiscoveredRouteRegistrarArtifactCache(
				4,
			)
			cache.Set(
				"cache-key",
				"fp-one",
				[]registraroverlay.SourceArtifact{
					{
						TargetFilePath: "backend/src/router/discovered_route_registrar_0.gen.go",
						SourceBytes:    []byte("first"),
					},
				},
			)

			_, staleFingerprintHit := cache.Get("cache-key", "fp-two")
			if staleFingerprintHit {
				t.Fatal("expected cache miss for mismatched fingerprint")
			}

			_, oldFingerprintHitAfterInvalidation := cache.Get(
				"cache-key",
				"fp-one",
			)
			if oldFingerprintHitAfterInvalidation {
				t.Fatal(
					"expected stale cache entry to be invalidated after fingerprint mismatch",
				)
			}
		},
	)

	t.Run(
		"evicts least recently used entry when capacity is exceeded",
		func(t *testing.T) {
			cache := registraroverlay.NewDiscoveredRouteRegistrarArtifactCache(
				2,
			)
			cache.Set("cache-key-a", "fp-a", nil)
			cache.Set("cache-key-b", "fp-b", nil)

			_, entryAHit := cache.Get("cache-key-a", "fp-a")
			if !entryAHit {
				t.Fatal("expected entry A cache hit before eviction step")
			}

			cache.Set("cache-key-c", "fp-c", nil)

			_, entryAHitAfterEviction := cache.Get("cache-key-a", "fp-a")
			if !entryAHitAfterEviction {
				t.Fatal("expected entry A to remain after LRU eviction")
			}
			_, entryBHitAfterEviction := cache.Get("cache-key-b", "fp-b")
			if entryBHitAfterEviction {
				t.Fatal("expected least recently used entry B to be evicted")
			}
			_, entryCHitAfterEviction := cache.Get("cache-key-c", "fp-c")
			if !entryCHitAfterEviction {
				t.Fatal("expected entry C to be present after insertion")
			}
		},
	)
}

func TestPrepareDiscoveredRouteRegistrarOverlay(t *testing.T) {
	t.Run(
		"returns nil when no discovered registrations exist",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}
`))

			overlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.App)
			if err != nil {
				t.Fatalf(
					"prepareDiscoveredRouteRegistrarOverlay returned error: %v",
					err,
				)
			}
			if overlay != nil {
				t.Fatal(
					"expected no overlay when no discovered registrations exist",
				)
			}
		},
	)

	t.Run(
		"ignores direct nestedmux/mux AddTaskHandler registrations",
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
var ActionsRouter = mux.NewRouter()

var LoaderHandler = mux.TaskHandlerFromFunc(func(*mux.ReqData[mux.None]) (string, error) {
	return "", nil
})

var ActionHandler = mux.TaskHandlerFromFunc(func(*mux.ReqData[mux.None]) (string, error) {
	return "", nil
})
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import (
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
)

var _ = nestedmux.AddTaskHandler(
	LoadersRouter,
	"/nestedmux-direct",
	LoaderHandler,
)

var _ = mux.AddTaskHandler(
	ActionsRouter,
	"POST",
	"/mux-direct",
	ActionHandler,
)
`))

			overlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.App)
			if err != nil {
				t.Fatalf(
					"prepareDiscoveredRouteRegistrarOverlay returned error: %v",
					err,
				)
			}
			if overlay != nil {
				t.Fatal(
					"expected no overlay when only direct mux/nestedmux registrations are present",
				)
			}
		},
	)

	t.Run(
		"prepares overlay sources for route registrations outside router package",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "backend/src/feature/routes.go", []byte(`
package feature

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}

var _ = vorma.DefineLoaderForRegistration(App, "/outside-router", nil, decorateLoaderCtx)
`))

			overlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.App)
			if err != nil {
				t.Fatalf(
					"prepareDiscoveredRouteRegistrarOverlay returned error: %v",
					err,
				)
			}
			if overlay == nil {
				t.Fatal("expected overlay for discovered registrations")
			}

			generatedFilePath := filepath.Join(
				fixture.RootDir,
				"backend/src/feature",
				registraroverlay.GeneratedFilename,
			)
			if _, err := os.Stat(generatedFilePath); !os.IsNotExist(err) {
				t.Fatalf(
					"expected no consumer-visible generated registrar file, stat err=%v",
					err,
				)
			}

			overlayReplacementSource := readOverlayReplacementSourceForGeneratedTargetPath(
				t,
				overlay,
				generatedFilePath,
			)
			if !strings.Contains(
				overlayReplacementSource,
				`vormagogen.RegisterLoaderDiscoveredByBuild(App, "/outside-router", nil, decorateLoaderCtx)`,
			) {
				t.Fatalf(
					"overlay source missing loader registration call:\n%s",
					overlayReplacementSource,
				)
			}

			overlayDirectoryPath := filepath.Dir(overlay.GoOverlayConfigPath())
			if err := overlay.Cleanup(); err != nil {
				t.Fatalf("cleanup overlay temporary files: %v", err)
			}
			if _, err := os.Stat(overlayDirectoryPath); !os.IsNotExist(err) {
				t.Fatalf(
					"expected overlay temp dir to be removed, stat err=%v",
					err,
				)
			}
		},
	)

	t.Run(
		"prepares overlay sources without writing consumer registrar files",
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

func decorateActionCtx(rd *vorma.ActionReqData[map[string]any]) *vorma.ActionReqData[map[string]any] {
	return rd
}

func registerLoaderAtPath(pattern string, loader any) any {
	return vorma.DefineLoaderForRegistration(App, pattern, nil, decorateLoaderCtx)
}

func registerActionAtPath(method string, pattern string, action any) any {
	return vorma.DefineActionForRegistration(App, method, pattern, nil, decorateActionCtx)
}
`))

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

const apiPrefix = "/api"

var _ = registerLoaderAtPath(apiPrefix + "/users", nil)
var _ = registerActionAtPath("POST", "/submit", nil)
`))

			overlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.App)
			if err != nil {
				t.Fatalf(
					"prepareDiscoveredRouteRegistrarOverlay returned error: %v",
					err,
				)
			}
			if overlay == nil {
				t.Fatal("expected overlay for discovered registrations")
			}

			generatedFilePath := filepath.Join(
				fixture.RootDir,
				"backend/src/router",
				registraroverlay.GeneratedFilename,
			)
			if _, err := os.Stat(generatedFilePath); !os.IsNotExist(err) {
				t.Fatalf(
					"expected no consumer-visible generated registrar file, stat err=%v",
					err,
				)
			}

			overlayConfigBytes, err := os.ReadFile(
				overlay.GoOverlayConfigPath(),
			)
			if err != nil {
				t.Fatalf(
					"read overlay config %q: %v",
					overlay.GoOverlayConfigPath(),
					err,
				)
			}
			var overlayConfig goOverlayReplaceConfigForTest
			if err := json.Unmarshal(overlayConfigBytes, &overlayConfig); err != nil {
				t.Fatalf("unmarshal overlay Config: %v", err)
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

			overlayReplacementSourceBytes, err := os.ReadFile(
				overlayReplacementSourcePath,
			)
			if err != nil {
				t.Fatalf(
					"read overlay source %q: %v",
					overlayReplacementSourcePath,
					err,
				)
			}
			overlayReplacementSource := string(overlayReplacementSourceBytes)
			if !strings.Contains(
				overlayReplacementSource,
				`vormagogen.RegisterLoaderDiscoveredByBuild(App, "/api/users", nil, decorateLoaderCtx)`,
			) {
				t.Fatalf(
					"overlay source missing loader registration call:\n%s",
					overlayReplacementSource,
				)
			}
			if !strings.Contains(
				overlayReplacementSource,
				`vormagogen.RegisterActionDiscoveredByBuild(App, "POST", "/submit", nil, decorateActionCtx)`,
			) {
				t.Fatalf(
					"overlay source missing action registration call:\n%s",
					overlayReplacementSource,
				)
			}

			overlayDirectoryPath := filepath.Dir(overlay.GoOverlayConfigPath())
			if err := overlay.Cleanup(); err != nil {
				t.Fatalf("cleanup overlay temporary files: %v", err)
			}
			if _, err := os.Stat(overlayDirectoryPath); !os.IsNotExist(err) {
				t.Fatalf(
					"expected overlay temp dir to be removed, stat err=%v",
					err,
				)
			}
		},
	)

	t.Run(
		"prepares overlay for imported helper wrappers across packages",
		func(t *testing.T) {
			repositoryRootDir := testkit.MustResolveRepositoryRootDir(t)
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "go.mod", []byte(fmt.Sprintf(`
module wrappertest

go 1.24

require github.com/vormadev/vorma v0.0.0

replace github.com/vormadev/vorma => %s
`, filepath.ToSlash(repositoryRootDir))))

			testkit.MustWriteFile(t, "backend/src/app/app.go", []byte(`
package app

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}
`))

			testkit.MustWriteFile(
				t,
				"backend/src/define/loader/loader.go",
				[]byte(`
package loader

import (
	appcfg "wrappertest/backend/src/app"

	"github.com/vormadev/vorma"
)

type Ctx struct {
	*vorma.LoaderReqData
}

func Define[O any](
	pattern string,
	loaderFunc vorma.LoaderFunc[Ctx, O],
) *vorma.Loader[O] {
	return vorma.DefineLoaderForRegistration(
		appcfg.App,
		pattern,
		loaderFunc,
		func(rd *vorma.LoaderReqData) *Ctx {
			return &Ctx{LoaderReqData: rd}
		},
	)
}
`),
			)

			testkit.MustWriteFile(t, "backend/src/content/routes.go", []byte(`
package content

import loaderhelpers "wrappertest/backend/src/define/loader"

var _ = loaderhelpers.Define(
	"/cross-package",
	func(*loaderhelpers.Ctx) (string, error) {
		return "ok", nil
	},
)
`))
			testkit.MustWriteFile(t, "backend/cmd/check/main.go", []byte(`
package main

import _ "wrappertest/backend/src/content"

func main() {}
`))

			overlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.App)
			if err != nil {
				t.Fatalf(
					"prepareDiscoveredRouteRegistrarOverlay returned error: %v",
					err,
				)
			}
			if overlay == nil {
				t.Fatal("expected overlay for discovered registrations")
			}

			generatedFilePath := filepath.Join(
				fixture.RootDir,
				"backend/src/content",
				registraroverlay.GeneratedFilename,
			)
			overlayReplacementSource := readOverlayReplacementSourceForGeneratedTargetPath(
				t,
				overlay,
				generatedFilePath,
			)
			if !strings.Contains(
				overlayReplacementSource,
				`vorma.RegisterDiscoveredLoaderTask(appcfg.App, "/cross-package",`,
			) {
				t.Fatalf(
					"overlay source missing cross-package discovered registration call:\n%s",
					overlayReplacementSource,
				)
			}
			if !strings.Contains(
				overlayReplacementSource,
				`loaderhelpers.Define(`,
			) {
				t.Fatalf(
					"overlay source missing wrapper task expression:\n%s",
					overlayReplacementSource,
				)
			}
			if strings.Contains(
				overlayReplacementSource,
				`_ = vorma.RegisterDiscoveredLoaderTask(`,
			) {
				t.Fatalf(
					"overlay source must emit direct discovered registration calls, found blank-identifier assignment:\n%s",
					overlayReplacementSource,
				)
			}

			compiledBinaryPath := filepath.Join(
				fixture.RootDir,
				"backend",
				"dist",
				"check_wrapper_registration",
			)
			if runtime.GOOS == "windows" {
				compiledBinaryPath += ".exe"
			}
			buildOutput, buildErr := testkit.RunCommandAndCaptureOutput(
				fixture.RootDir,
				"go",
				"build",
				"-mod=mod",
				"-overlay="+overlay.GoOverlayConfigPath(),
				"-o",
				compiledBinaryPath,
				"./backend/cmd/check",
			)
			if cleanupErr := overlay.Cleanup(); cleanupErr != nil {
				t.Fatalf("cleanup discovered overlay: %v", cleanupErr)
			}
			if buildErr != nil {
				t.Fatalf(
					"go build with discovered wrapper overlay failed: %v\n%s",
					buildErr,
					buildOutput,
				)
			}
		},
	)

	t.Run(
		"registers discovered wrappers when build entry omits explicit route package imports",
		func(t *testing.T) {
			repositoryRootDir := testkit.MustResolveRepositoryRootDir(t)
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)

			testkit.MustWriteFile(t, "go.mod", []byte(fmt.Sprintf(`
module wrappertest

go 1.24

require github.com/vormadev/vorma v0.0.0

replace github.com/vormadev/vorma => %s
`, filepath.ToSlash(repositoryRootDir))))

			testkit.MustWriteFile(t, "backend/wave.go", []byte(`
package backend

import (
	"os"

	"github.com/vormadev/vorma/wave"
)

var Wave = wave.New(wave.Config{
	FS:         os.DirFS("."),
	ConfigPath: "wave.config.json",
})
`))

			testkit.MustWriteFile(t, "backend/src/app/app.go", []byte(`
package app

import (
	"wrappertest/backend"

	"github.com/vormadev/vorma"
)

var App = vorma.NewVormaApp(vorma.VormaAppConfig{
	Wave: backend.Wave,
})
`))

			testkit.MustWriteFile(
				t,
				"backend/src/define/loader/loader.go",
				[]byte(`
package loader

import (
	appcfg "wrappertest/backend/src/app"

	"github.com/vormadev/vorma"
)

type Ctx struct {
	*vorma.LoaderReqData
}

func Define[O any](
	pattern string,
	loaderFunc func(*Ctx) (O, error),
) *vorma.Loader[O] {
	return vorma.DefineLoaderForRegistration(
		appcfg.App,
		pattern,
		loaderFunc,
		func(rd *vorma.LoaderReqData) *Ctx {
			return &Ctx{LoaderReqData: rd}
		},
	)
}
`),
			)

			testkit.MustWriteFile(t, "backend/src/content/routes.go", []byte(`
package content

import loaderhelpers "wrappertest/backend/src/define/loader"

var _ = loaderhelpers.Define(
	"/cross-package",
	func(*loaderhelpers.Ctx) (string, error) {
		return "ok", nil
	},
)
`))

			testkit.MustWriteFile(t, "backend/cmd/build/main.go", []byte(`
package main

import (
	"fmt"
	"os"

	"wrappertest/backend/src/app"
)

func main() {
	if !app.App.HasRegisteredLoaderTask("/cross-package") {
		fmt.Println("missing discovered route registration")
		os.Exit(1)
	}
	fmt.Println("registered")
}
`))

			overlay, err := prepareDiscoveredRouteRegistrarOverlay(fixture.App)
			if err != nil {
				t.Fatalf(
					"prepareDiscoveredRouteRegistrarOverlay returned error: %v",
					err,
				)
			}
			if overlay == nil {
				t.Fatal("expected overlay for discovered registrations")
			}

			generatedRouteRegistrarSource := readOverlayReplacementSourceForGeneratedTargetPath(
				t,
				overlay,
				filepath.Join(
					fixture.RootDir,
					"backend/src/content",
					registraroverlay.GeneratedFilename,
				),
			)
			if !strings.Contains(
				generatedRouteRegistrarSource,
				`vorma.RegisterDiscoveredLoaderTask(appcfg.App, "/cross-package",`,
			) {
				t.Fatalf(
					"overlay source missing discovered registration call:\n%s",
					generatedRouteRegistrarSource,
				)
			}

			compiledBinaryPath := filepath.Join(
				fixture.RootDir,
				"backend",
				"dist",
				"check_wrapper_registration_without_import",
			)
			if runtime.GOOS == "windows" {
				compiledBinaryPath += ".exe"
			}
			buildOutput, buildErr := testkit.RunCommandAndCaptureOutput(
				fixture.RootDir,
				"go",
				"build",
				"-mod=mod",
				"-overlay="+overlay.GoOverlayConfigPath(),
				"-o",
				compiledBinaryPath,
				"./backend/cmd/build",
			)
			if cleanupErr := overlay.Cleanup(); cleanupErr != nil {
				t.Fatalf("cleanup discovered overlay: %v", cleanupErr)
			}
			if buildErr != nil {
				t.Fatalf(
					"go build with discovered wrapper overlay failed: %v\n%s",
					buildErr,
					buildOutput,
				)
			}

			runtimeOutput, runtimeErr := testkit.RunCommandAndCaptureOutput(
				fixture.RootDir,
				compiledBinaryPath,
			)
			if runtimeErr != nil {
				t.Fatalf(
					"compiled runtime check failed: %v\n%s",
					runtimeErr,
					runtimeOutput,
				)
			}
			if !strings.Contains(runtimeOutput, "registered") {
				t.Fatalf(
					"runtime output = %q, expected registration marker",
					runtimeOutput,
				)
			}
		},
	)

	t.Run(
		"fails fast on conflicting import aliases across discovered expressions",
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

			testkit.MustWriteFile(t, "backend/src/router/routes_a.go", []byte(`
package router

import (
	foo "bytes"

	"github.com/vormadev/vorma"
)

var _ = vorma.DefineLoaderForRegistration(App, "/a", foo.Clone, decorateLoaderCtx)
`))

			testkit.MustWriteFile(t, "backend/src/router/routes_b.go", []byte(`
package router

import (
	foo "strings"

	"github.com/vormadev/vorma"
)

var _ = vorma.DefineLoaderForRegistration(App, "/b", foo.Clone, decorateLoaderCtx)
`))

			_, err := prepareDiscoveredRouteRegistrarOverlay(fixture.App)
			if err == nil {
				t.Fatal(
					"expected prepareDiscoveredRouteRegistrarOverlay to return error",
				)
			}
			if !strings.Contains(err.Error(), "conflicting import alias") {
				t.Fatalf(
					"error = %q, expected conflicting import alias guidance",
					err,
				)
			}
		},
	)

	t.Run(
		"caches discovered registrar artifacts when source fingerprint is unchanged",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)
			discoveredRegistrarArtifactsCache := registraroverlay.NewDiscoveredRouteRegistrarArtifactCache(
				registraroverlay.DiscoveredRouteRegistrarArtifactCacheDefaultMaxEntries,
			)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
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

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import "github.com/vormadev/vorma"

var _ = vorma.DefineLoaderForRegistration(App, "/cached", usersLoader, decorateLoaderCtx)
`))

			parseGoSourceASTCallCount := 0
			discoveryDependencies := discoveryDependenciesWithParseGoSourceASTCounter(
				&parseGoSourceASTCallCount,
			)

			firstOverlay, err := prepareDiscoveredRouteRegistrarOverlayWithArtifactCacheAndDiscoveryDependencies(
				fixture.App,
				discoveredRegistrarArtifactsCache,
				discoveryDependencies,
			)
			if err != nil {
				t.Fatalf(
					"prepareDiscoveredRouteRegistrarOverlay returned error: %v",
					err,
				)
			}
			if firstOverlay == nil {
				t.Fatal("expected overlay for discovered registrations")
			}
			firstParseGoSourceASTCallCount := parseGoSourceASTCallCount
			if firstParseGoSourceASTCallCount == 0 {
				t.Fatal(
					"expected parseServerRouteFilesIntoPackageAnalyses to parse Go source files",
				)
			}
			if err := firstOverlay.Cleanup(); err != nil {
				t.Fatalf("cleanup first discovered overlay: %v", err)
			}

			secondOverlay, err := prepareDiscoveredRouteRegistrarOverlayWithArtifactCacheAndDiscoveryDependencies(
				fixture.App,
				discoveredRegistrarArtifactsCache,
				discoveryDependencies,
			)
			if err != nil {
				t.Fatalf(
					"prepareDiscoveredRouteRegistrarOverlay second call returned error: %v",
					err,
				)
			}
			if secondOverlay == nil {
				t.Fatal(
					"expected overlay for discovered registrations on second call",
				)
			}
			if parseGoSourceASTCallCount != firstParseGoSourceASTCallCount {
				t.Fatalf(
					"parseGoSourceAST call count after cache hit = %d, want %d",
					parseGoSourceASTCallCount,
					firstParseGoSourceASTCallCount,
				)
			}
			if err := secondOverlay.Cleanup(); err != nil {
				t.Fatalf("cleanup second discovered overlay: %v", err)
			}
		},
	)

	t.Run(
		"invalidates cached discovered registrar artifacts when package source changes",
		func(t *testing.T) {
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)
			discoveredRegistrarArtifactsCache := registraroverlay.NewDiscoveredRouteRegistrarArtifactCache(
				registraroverlay.DiscoveredRouteRegistrarArtifactCacheDefaultMaxEntries,
			)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
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
			testkit.MustWriteFile(t, routesFilePath, []byte(`
package router

import "github.com/vormadev/vorma"

var _ = vorma.DefineLoaderForRegistration(App, "/before", usersLoader, decorateLoaderCtx)
`))

			parseGoSourceASTCallCount := 0
			discoveryDependencies := discoveryDependenciesWithParseGoSourceASTCounter(
				&parseGoSourceASTCallCount,
			)

			firstOverlay, err := prepareDiscoveredRouteRegistrarOverlayWithArtifactCacheAndDiscoveryDependencies(
				fixture.App,
				discoveredRegistrarArtifactsCache,
				discoveryDependencies,
			)
			if err != nil {
				t.Fatalf(
					"prepareDiscoveredRouteRegistrarOverlay first call returned error: %v",
					err,
				)
			}
			if firstOverlay == nil {
				t.Fatal("expected overlay for discovered registrations")
			}
			generatedFilePath := filepath.Join(
				fixture.RootDir,
				"backend/src/router",
				registraroverlay.GeneratedFilename,
			)
			firstOverlaySource := readOverlayReplacementSourceForGeneratedTargetPath(
				t,
				firstOverlay,
				generatedFilePath,
			)
			if !strings.Contains(firstOverlaySource, `"/before"`) {
				t.Fatalf(
					"first overlay source missing /before pattern:\n%s",
					firstOverlaySource,
				)
			}
			firstParseGoSourceASTCallCount := parseGoSourceASTCallCount
			if err := firstOverlay.Cleanup(); err != nil {
				t.Fatalf("cleanup first discovered overlay: %v", err)
			}

			testkit.MustWriteFile(t, routesFilePath, []byte(`
package router

import "github.com/vormadev/vorma"

var _ = vorma.DefineLoaderForRegistration(App, "/after", usersLoader, decorateLoaderCtx)
`))

			secondOverlay, err := prepareDiscoveredRouteRegistrarOverlayWithArtifactCacheAndDiscoveryDependencies(
				fixture.App,
				discoveredRegistrarArtifactsCache,
				discoveryDependencies,
			)
			if err != nil {
				t.Fatalf(
					"prepareDiscoveredRouteRegistrarOverlay second call returned error: %v",
					err,
				)
			}
			if secondOverlay == nil {
				t.Fatal(
					"expected overlay for discovered registrations after source change",
				)
			}
			secondOverlaySource := readOverlayReplacementSourceForGeneratedTargetPath(
				t,
				secondOverlay,
				generatedFilePath,
			)
			if !strings.Contains(secondOverlaySource, `"/after"`) {
				t.Fatalf(
					"second overlay source missing /after pattern:\n%s",
					secondOverlaySource,
				)
			}
			if parseGoSourceASTCallCount <= firstParseGoSourceASTCallCount {
				t.Fatalf(
					"expected cache invalidation to re-run Go source parsing; parse call count after change = %d, before change = %d",
					parseGoSourceASTCallCount,
					firstParseGoSourceASTCallCount,
				)
			}
			if err := secondOverlay.Cleanup(); err != nil {
				t.Fatalf("cleanup second discovered overlay: %v", err)
			}
		},
	)

	t.Run(
		"dependency-only handler changes are reflected under cached discovery artifacts",
		func(t *testing.T) {
			repositoryRootDir := testkit.MustResolveRepositoryRootDir(t)
			fixture := newBackendRouteDiscoveryFixtureWithServerPatterns(t)
			t.Chdir(fixture.RootDir)
			discoveredRegistrarArtifactsCache := registraroverlay.NewDiscoveredRouteRegistrarArtifactCache(
				registraroverlay.DiscoveredRouteRegistrarArtifactCacheDefaultMaxEntries,
			)

			testkit.MustWriteFile(t, "go.mod", []byte(fmt.Sprintf(`
module cachee2e

go 1.24

require github.com/vormadev/vorma v0.0.0

replace github.com/vormadev/vorma => %s
`, filepath.ToSlash(repositoryRootDir))))

			testkit.MustWriteFile(t, "backend/wave.go", []byte(`
package backend

import (
	"os"

	"github.com/vormadev/vorma/wave"
)

var waveFS = os.DirFS(".")

var Wave = wave.New(wave.Config{
	FS:         waveFS,
	ConfigPath: "backend/wave.config.json",
})
`))

			testkit.MustWriteFile(t, "backend/wave.config.json", []byte(`
{
	"Core": {
		"ProjectID": "backend-route-registration-generation-test",
		"MainAppEntry": "cmd/check",
		"StaticAssetDirs": {
			"Private": "assets/private",
			"Public": "assets/public"
		},
		"PublicPathPrefix": "/",
		"ServerOnlyMode": true
	},
	"Vorma": {
		"MainBuildEntry": "cmd/build",
		"UIVariant": "react",
		"HTMLTemplateLocation": "entry.go.html",
		"ClientEntry": "../frontend/src/vorma.entry.tsx",
		"ClientRouteDefinitionPatterns": ["../frontend/src/**/*vorma.routes.ts"],
		"ServerRouteDefinitionPatterns": ["src/**/*.go"],
		"TSGenOutDir": "../frontend/src/vorma.gen",
		"BuildtimePublicURLFuncName": "waveBuildtimeURL"
	}
}
`))

			testkit.MustWriteFile(
				t,
				filepath.Join(
					"backend",
					waveartifacts.AssetsDirname,
					waveartifacts.PrivateDirname,
					"entry.go.html",
				),
				[]byte("<!doctype html><html><body></body></html>"),
			)

			testkit.MustWriteFile(t, "backend/src/app/app.go", []byte(`
package app

import (
	"cachee2e/backend"

	"github.com/vormadev/vorma"
)

var App = vorma.NewVormaApp(vorma.VormaAppConfig{
	Wave: backend.Wave,
})
`))

			testkit.MustWriteFile(
				t,
				"backend/lib/version/version.go",
				[]byte(`
package version

func LatestVersion() string {
	return "v1"
}
`),
			)

			testkit.MustWriteFile(t, "backend/src/router/context.go", []byte(`
package router

import (
	"cachee2e/backend/src/app"
	"cachee2e/backend/lib/version"

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

			testkit.MustWriteFile(t, "backend/src/router/routes.go", []byte(`
package router

import "github.com/vormadev/vorma"

var _ = vorma.DefineLoaderForRegistration(App, "/version", usersLoader, decorateLoaderCtx)
`))

			testkit.MustWriteFile(t, "backend/cmd/check/main.go", []byte(`
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

	results, found := app.App.FindNestedMatchesAndRunLoaderTasks(reqWithTasksCtx)
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
			discoveryDependencies := discoveryDependenciesWithParseGoSourceASTCounter(
				&parseGoSourceASTCallCount,
			)

			firstOverlay, err := prepareDiscoveredRouteRegistrarOverlayWithArtifactCacheAndDiscoveryDependencies(
				fixture.App,
				discoveredRegistrarArtifactsCache,
				discoveryDependencies,
			)
			if err != nil {
				t.Fatalf(
					"prepareDiscoveredRouteRegistrarOverlay first call returned error: %v",
					err,
				)
			}
			if firstOverlay == nil {
				t.Fatal("expected first discovered overlay")
			}
			firstParseGoSourceASTCallCount := parseGoSourceASTCallCount
			if firstParseGoSourceASTCallCount == 0 {
				t.Fatal("expected first overlay preparation to parse Go source")
			}

			compiledBinaryPath := filepath.Join(
				fixture.RootDir,
				"backend",
				"dist",
				"check",
			)
			if runtime.GOOS == "windows" {
				compiledBinaryPath += ".exe"
			}

			firstBuildOutput, firstBuildErr := testkit.RunCommandAndCaptureOutput(
				fixture.RootDir,
				"go",
				"build",
				"-mod=mod",
				"-overlay="+firstOverlay.GoOverlayConfigPath(),
				"-o",
				compiledBinaryPath,
				"./backend/cmd/check",
			)
			if cleanupErr := firstOverlay.Cleanup(); cleanupErr != nil {
				t.Fatalf("cleanup first discovered overlay: %v", cleanupErr)
			}
			if firstBuildErr != nil {
				t.Fatalf(
					"first go build failed: %v\n%s",
					firstBuildErr,
					firstBuildOutput,
				)
			}

			firstRuntimeOutput, firstRuntimeErr := testkit.RunCommandAndCaptureOutput(
				fixture.RootDir,
				compiledBinaryPath,
			)
			if firstRuntimeErr != nil {
				t.Fatalf(
					"first compiled runtime check failed: %v\n%s",
					firstRuntimeErr,
					firstRuntimeOutput,
				)
			}
			if !strings.Contains(firstRuntimeOutput, "v1") {
				t.Fatalf(
					"first runtime output = %q, expected version marker %q",
					firstRuntimeOutput,
					"v1",
				)
			}

			testkit.MustWriteFile(
				t,
				"backend/lib/version/version.go",
				[]byte(`
package version

func LatestVersion() string {
	return "v2"
}
`),
			)

			secondOverlay, err := prepareDiscoveredRouteRegistrarOverlayWithArtifactCacheAndDiscoveryDependencies(
				fixture.App,
				discoveredRegistrarArtifactsCache,
				discoveryDependencies,
			)
			if err != nil {
				t.Fatalf(
					"prepareDiscoveredRouteRegistrarOverlay second call returned error: %v",
					err,
				)
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

			secondBuildOutput, secondBuildErr := testkit.RunCommandAndCaptureOutput(
				fixture.RootDir,
				"go",
				"build",
				"-mod=mod",
				"-overlay="+secondOverlay.GoOverlayConfigPath(),
				"-o",
				compiledBinaryPath,
				"./backend/cmd/check",
			)
			if cleanupErr := secondOverlay.Cleanup(); cleanupErr != nil {
				t.Fatalf("cleanup second discovered overlay: %v", cleanupErr)
			}
			if secondBuildErr != nil {
				t.Fatalf(
					"second go build failed: %v\n%s",
					secondBuildErr,
					secondBuildOutput,
				)
			}

			secondRuntimeOutput, secondRuntimeErr := testkit.RunCommandAndCaptureOutput(
				fixture.RootDir,
				compiledBinaryPath,
			)
			if secondRuntimeErr != nil {
				t.Fatalf(
					"second compiled runtime check failed: %v\n%s",
					secondRuntimeErr,
					secondRuntimeOutput,
				)
			}
			if !strings.Contains(secondRuntimeOutput, "v2") {
				t.Fatalf(
					"second runtime output = %q, expected version marker %q",
					secondRuntimeOutput,
					"v2",
				)
			}
		},
	)
}

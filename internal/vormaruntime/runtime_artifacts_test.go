package vormaruntime

import (
	"html/template"
	"reflect"
	"sync"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/runtimecore"
)

func TestRuntimeRoutePathConversionHelpers(t *testing.T) {
	pathValue := &Path{
		OriginalPattern: "/products/:id",
		SrcPath:         "frontend/src/routes/products.$id.tsx",
		OutPath:         "vorma_out/routes/products.$id.js",
		ExportKey:       "default",
		ErrorExportKey:  "ProductErrorBoundary",
		Deps:            []string{"vorma_out/products.js"},
	}

	runtimeCorePath := toRuntimeCoreRoutePath(pathValue)
	if runtimeCorePath == nil {
		t.Fatal("toRuntimeCoreRoutePath returned nil")
	}
	pathValue.Deps[0] = "mutated"
	if got, want := runtimeCorePath.Deps[0], "vorma_out/products.js"; got != want {
		t.Fatalf("runtimecore deps = %q, want %q", got, want)
	}

	convertedBack := fromRuntimeCoreRoutePath(runtimeCorePath)
	if convertedBack == nil {
		t.Fatal("fromRuntimeCoreRoutePath returned nil")
	}
	runtimeCorePath.Deps[0] = "runtimecore-mutated"
	if got, want := convertedBack.Deps[0], "vorma_out/products.js"; got != want {
		t.Fatalf("converted-back deps = %q, want %q", got, want)
	}
}

func TestRuntimeRoutePathMapConversionHelpers(t *testing.T) {
	paths := map[string]*Path{
		"/a": {
			OriginalPattern: "/a",
			SrcPath:         "frontend/src/routes/a.tsx",
			OutPath:         "vorma_out/routes/a.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/a.js"},
		},
		"/nil": nil,
	}

	runtimeCorePaths := toRuntimeCoreRoutePaths(paths)
	if runtimeCorePaths == nil {
		t.Fatal("toRuntimeCoreRoutePaths returned nil")
	}
	if runtimeCorePaths["/nil"] != nil {
		t.Fatalf(
			"expected nil entry to remain nil, got %#v",
			runtimeCorePaths["/nil"],
		)
	}
	paths["/a"].Deps[0] = "mutated"
	if got, want := runtimeCorePaths["/a"].Deps[0], "vorma_out/a.js"; got != want {
		t.Fatalf("runtimecore deps = %q, want %q", got, want)
	}

	convertedBack := fromRuntimeCoreRoutePaths(runtimeCorePaths)
	if convertedBack == nil {
		t.Fatal("fromRuntimeCoreRoutePaths returned nil")
	}
	runtimeCorePaths["/a"].Deps[0] = "runtimecore-mutated"
	if got, want := convertedBack["/a"].Deps[0], "vorma_out/a.js"; got != want {
		t.Fatalf("converted-back deps = %q, want %q", got, want)
	}
}

func TestClonePathsMapHelpers(t *testing.T) {
	paths := map[string]*Path{
		"/a": {
			OriginalPattern: "/a",
			SrcPath:         "frontend/src/routes/a.tsx",
			OutPath:         "vorma_out/routes/a.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/a.js"},
		},
	}

	cloned := clonePathsMap(paths)
	if got, want := cloned["/a"].OriginalPattern, "/a"; got != want {
		t.Fatalf("cloned original pattern = %q, want %q", got, want)
	}
	paths["/a"].Deps[0] = "mutated"
	if got, want := cloned["/a"].Deps[0], "vorma_out/a.js"; got != want {
		t.Fatalf("cloned deps = %q, want %q", got, want)
	}

	if got := clonePathsMapOrNil(nil); got != nil {
		t.Fatalf("clonePathsMapOrNil(nil) = %#v, want nil", got)
	}
}

func TestCaptureRuntimeServingSnapshot(t *testing.T) {
	routeDataCache := &sync.Map{}
	rootTemplate := template.Must(
		template.New("root").Parse("<html>{{.}}</html>"),
	)

	snapshot := captureRuntimeServingSnapshot(runtimeServingSnapshotInput{
		BuildID: "build-snapshot",
		IsDev:   true,
		Paths: map[string]*runtimecore.RoutePath{
			"/": {OriginalPattern: "/"},
		},
		ClientEntryDeps: []string{"vorma_out/client.js"},
		ClientEntryOut:  "vorma_out/entry.js",
		DepToCSSBundleMap: map[string][]string{
			"vorma_out/client.js": {"vorma_out/client.css"},
		},
		RootTemplate:             rootTemplate,
		RouteManifestFile:        "vorma_out/route-manifest.js",
		RouteDataSnapshotVersion: 7,
		RouteDataCache:           routeDataCache,
	})

	if got, want := snapshot.BuildID, "build-snapshot"; got != want {
		t.Fatalf("BuildID = %q, want %q", got, want)
	}
	if got, want := snapshot.IsDev, true; got != want {
		t.Fatalf("IsDev = %v, want %v", got, want)
	}
	if snapshot.RouteDataCache != routeDataCache {
		t.Fatal("RouteDataCache pointer should be preserved by capture")
	}
	if snapshot.RootTemplate != rootTemplate {
		t.Fatal("RootTemplate pointer should be preserved by capture")
	}
}

func TestRuntimeServingSnapshotToLoadersHTMLRenderSnapshot(t *testing.T) {
	rootTemplate := template.Must(
		template.New("root").Parse("<html>{{.}}</html>"),
	)
	snapshot := runtimeServingSnapshot{
		IsDev:          true,
		ClientEntryOut: "vorma_out/entry.js",
		RootTemplate:   rootTemplate,
	}

	loadersHTMLRender := snapshot.ToLoadersHTMLRenderSnapshot()
	if got, want := loadersHTMLRender.IsDevMode, true; got != want {
		t.Fatalf("IsDevMode = %v, want %v", got, want)
	}
	if got, want := loadersHTMLRender.ClientEntryOut, "vorma_out/entry.js"; got != want {
		t.Fatalf("ClientEntryOut = %q, want %q", got, want)
	}
	if loadersHTMLRender.RootTemplate != rootTemplate {
		t.Fatal("RootTemplate pointer should be preserved in render snapshot")
	}
}

func TestRuntimeServingSnapshotToRoutePipelineSnapshot(t *testing.T) {
	routeDataCache := &sync.Map{}
	snapshot := runtimeServingSnapshot{
		BuildID: "build-route-pipeline",
		IsDev:   true,
		Paths: map[string]*runtimecore.RoutePath{
			"/products/:id": {
				OriginalPattern: "/products/:id",
				SrcPath:         "frontend/src/routes/products.$id.tsx",
				OutPath:         "vorma_out/routes/products.$id.js",
				ExportKey:       "default",
				ErrorExportKey:  "ProductErrorBoundary",
				Deps:            []string{"vorma_out/chunk-products.js"},
			},
			"/nil": nil,
		},
		ClientEntryDeps: []string{"vorma_out/chunk-client.js"},
		ClientEntryOut:  "vorma_out/client-entry.js",
		DepToCSSBundleMap: map[string][]string{
			"vorma_out/chunk-client.js": {"vorma_out/chunk-client.css"},
		},
		RouteManifestFile:        "vorma_out/route-manifest.js",
		RouteDataSnapshotVersion: 12,
		RouteDataCache:           routeDataCache,
	}

	routePipelineSnapshot := snapshot.ToRoutePipelineSnapshot()
	if got, want := routePipelineSnapshot.BuildID, "build-route-pipeline"; got != want {
		t.Fatalf("BuildID = %q, want %q", got, want)
	}
	if got, want := routePipelineSnapshot.IsDev, true; got != want {
		t.Fatalf("IsDev = %v, want %v", got, want)
	}
	if routePipelineSnapshot.RouteDataCache != routeDataCache {
		t.Fatal("RouteDataCache pointer should be preserved")
	}
	if got, want := routePipelineSnapshot.RouteManifestFile, "vorma_out/route-manifest.js"; got != want {
		t.Fatalf("RouteManifestFile = %q, want %q", got, want)
	}

	wantPaths := map[string]struct {
		srcPath string
		outPath string
		deps    []string
	}{
		"/products/:id": {
			srcPath: "frontend/src/routes/products.$id.tsx",
			outPath: "vorma_out/routes/products.$id.js",
			deps:    []string{"vorma_out/chunk-products.js"},
		},
	}
	for pattern, want := range wantPaths {
		gotPath := routePipelineSnapshot.Paths[pattern]
		if gotPath == nil {
			t.Fatalf("expected path entry for %q", pattern)
		}
		if got, wantSrc := gotPath.SrcPath, want.srcPath; got != wantSrc {
			t.Fatalf("SrcPath for %q = %q, want %q", pattern, got, wantSrc)
		}
		if got, wantOut := gotPath.OutPath, want.outPath; got != wantOut {
			t.Fatalf("OutPath for %q = %q, want %q", pattern, got, wantOut)
		}
		if got, wantDeps := gotPath.Deps, want.deps; !reflect.DeepEqual(
			got,
			wantDeps,
		) {
			t.Fatalf("Deps for %q = %#v, want %#v", pattern, got, wantDeps)
		}
	}
	if routePipelineSnapshot.Paths["/nil"] != nil {
		t.Fatalf(
			"expected nil route path to remain nil, got %#v",
			routePipelineSnapshot.Paths["/nil"],
		)
	}
}

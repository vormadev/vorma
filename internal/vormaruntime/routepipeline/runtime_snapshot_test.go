package routepipeline

import (
	"html/template"
	"reflect"
	"sync"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/runtimecore"
)

func TestBuildRuntimeSnapshotFromCore_PreservesCoreFields(t *testing.T) {
	routeDataCache := &sync.Map{}
	rootTemplate := template.Must(
		template.New("root").Parse("<html>{{.}}</html>"),
	)

	snapshot := BuildRuntimeSnapshotFromCore(RuntimeSnapshotFromCoreInput{
		BuildID: "build-route-pipeline",
		IsDev:   true,
		Paths: map[string]*runtimecore.RoutePath{
			"/products/:id": {
				OriginalPattern: "/products/:id",
				SrcPath:         "frontend/src/routes/products.$id.tsx",
				OutPath:         testWaveOutPath("routes/products.$id.js"),
				ExportKey:       "default",
				ErrorExportKey:  "ProductErrorBoundary",
				Deps:            []string{testWaveOutPath("chunk-products.js")},
			},
			"/nil": nil,
		},
		ClientEntryDeps: []string{testWaveOutPath("chunk-client.js")},
		ClientEntryOut:  testWaveOutPath("client-entry.js"),
		DepToCSSBundleMap: map[string][]string{
			testWaveOutPath("chunk-client.js"): {testWaveOutPath("chunk-client.css")},
		},
		RootTemplate:             rootTemplate,
		RouteManifestFile:        testWaveOutPath("route-manifest.js"),
		RouteDataSnapshotVersion: 12,
		RouteDataCache:           routeDataCache,
	})

	if got, want := snapshot.BuildID, "build-route-pipeline"; got != want {
		t.Fatalf("BuildID = %q, want %q", got, want)
	}
	if got, want := snapshot.IsDev, true; got != want {
		t.Fatalf("IsDev = %v, want %v", got, want)
	}
	if got, want := snapshot.RouteManifestFile, testWaveOutPath("route-manifest.js"); got != want {
		t.Fatalf("RouteManifestFile = %q, want %q", got, want)
	}
	if snapshot.RouteDataCache != routeDataCache {
		t.Fatal("RouteDataCache pointer should be preserved")
	}
	if snapshot.HTMLRenderSnapshot.RootTemplate != rootTemplate {
		t.Fatal(
			"RootTemplate pointer should be preserved in HTML render snapshot",
		)
	}
}

func TestBuildRuntimeSnapshotFromCore_PreservesRuntimeCorePathsReference(
	t *testing.T,
) {
	inputPaths := map[string]*runtimecore.RoutePath{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.tsx",
			OutPath:         testWaveOutPath("routes/products.$id.js"),
			ExportKey:       "default",
			ErrorExportKey:  "ProductErrorBoundary",
			Deps:            []string{testWaveOutPath("chunk-products.js")},
		},
		"/nil": nil,
	}
	snapshot := BuildRuntimeSnapshotFromCore(RuntimeSnapshotFromCoreInput{
		Paths: inputPaths,
	})

	gotPath := snapshot.Paths["/products/:id"]
	if gotPath == nil {
		t.Fatal("expected path entry for /products/:id")
	}
	if gotPath != inputPaths["/products/:id"] {
		t.Fatal("expected runtime snapshot to preserve route path pointers")
	}
	if got, want := gotPath.SrcPath, "frontend/src/routes/products.$id.tsx"; got != want {
		t.Fatalf("SrcPath = %q, want %q", got, want)
	}
	if got, want := gotPath.OutPath, testWaveOutPath("routes/products.$id.js"); got != want {
		t.Fatalf("OutPath = %q, want %q", got, want)
	}
	if got, want := gotPath.ExportKey, "default"; got != want {
		t.Fatalf("ExportKey = %q, want %q", got, want)
	}
	if got, want := gotPath.ErrorExportKey, "ProductErrorBoundary"; got != want {
		t.Fatalf("ErrorExportKey = %q, want %q", got, want)
	}
	if got, want := gotPath.Deps, []string{testWaveOutPath("chunk-products.js")}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("Deps = %#v, want %#v", got, want)
	}
	if snapshot.Paths["/nil"] != nil {
		t.Fatalf(
			"expected nil route path to remain nil, got %#v",
			snapshot.Paths["/nil"],
		)
	}
}

func TestBuildRuntimeSnapshotFromCore_NilPathsRemainNil(t *testing.T) {
	snapshot := BuildRuntimeSnapshotFromCore(RuntimeSnapshotFromCoreInput{})
	if snapshot.Paths != nil {
		t.Fatalf("snapshot paths = %#v, want nil", snapshot.Paths)
	}
}

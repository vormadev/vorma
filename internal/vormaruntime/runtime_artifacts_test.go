package vormaruntime

import (
	"reflect"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/runtimecore"
)

func TestBuildRuntimePathsFileSnapshot(t *testing.T) {
	t.Run("nil_paths_file_returns_nil_snapshot", func(t *testing.T) {
		if buildRuntimePathsFileSnapshot(nil) != nil {
			t.Fatal("expected nil snapshot for nil paths file input")
		}
	})

	t.Run("maps_paths_file_fields_to_runtimecore_snapshot", func(t *testing.T) {
		pathsFile := defaultPathsFile("build-artifacts", map[string]*Path{
			"/products/:id": {
				OriginalPattern: "/products/:id",
				SrcPath:         "frontend/src/routes/products.$id.tsx",
				OutPath:         "vorma_out/routes/products.$id.js",
				ExportKey:       "default",
				Deps:            []string{"vorma_out/products.js"},
			},
		})

		snapshot := buildRuntimePathsFileSnapshot(pathsFile)
		if snapshot == nil {
			t.Fatal("expected non-nil runtime paths snapshot")
		}
		if got, want := snapshot.BuildID, pathsFile.BuildID; got != want {
			t.Fatalf("BuildID = %q, want %q", got, want)
		}
		if got, want := snapshot.ClientEntrySrc, pathsFile.ClientEntrySrc; got != want {
			t.Fatalf("ClientEntrySrc = %q, want %q", got, want)
		}
		if got, want := snapshot.ClientEntryOut, pathsFile.ClientEntryOut; got != want {
			t.Fatalf("ClientEntryOut = %q, want %q", got, want)
		}
		if got, want := snapshot.RouteManifestFile, pathsFile.RouteManifestFile; got != want {
			t.Fatalf("RouteManifestFile = %q, want %q", got, want)
		}
		if got, want := snapshot.ClientEntryDeps, pathsFile.ClientEntryDeps; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("ClientEntryDeps = %#v, want %#v", got, want)
		}
		if got, want := snapshot.DepToCSSBundleMap, pathsFile.DepToCSSBundleMap; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("DepToCSSBundleMap = %#v, want %#v", got, want)
		}

		wantPaths := map[string]*runtimecore.RoutePath{
			"/products/:id": {
				OriginalPattern: "/products/:id",
				SrcPath:         "frontend/src/routes/products.$id.tsx",
				OutPath:         "vorma_out/routes/products.$id.js",
				ExportKey:       "default",
				Deps:            []string{"vorma_out/products.js"},
			},
		}
		if got := snapshot.Paths; !reflect.DeepEqual(got, wantPaths) {
			t.Fatalf("Paths = %#v, want %#v", got, wantPaths)
		}

		artifacts, err := runtimecore.BuildRuntimeRouteArtifacts(snapshot)
		if err != nil {
			t.Fatalf("BuildRuntimeRouteArtifacts: %v", err)
		}
		if got, want := artifacts.BuildID, snapshot.BuildID; got != want {
			t.Fatalf("BuildID = %q, want %q", got, want)
		}
		if got, want := artifacts.ClientEntrySrc, snapshot.ClientEntrySrc; got != want {
			t.Fatalf("ClientEntrySrc = %q, want %q", got, want)
		}
		if got, want := artifacts.ClientEntryOut, snapshot.ClientEntryOut; got != want {
			t.Fatalf("ClientEntryOut = %q, want %q", got, want)
		}
		if got, want := artifacts.RouteManifestFile, snapshot.RouteManifestFile; got != want {
			t.Fatalf("RouteManifestFile = %q, want %q", got, want)
		}
		if got, want := artifacts.ClientEntryDeps, snapshot.ClientEntryDeps; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("ClientEntryDeps = %#v, want %#v", got, want)
		}
		if got, want := artifacts.DepToCSSBundleMap, snapshot.DepToCSSBundleMap; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("DepToCSSBundleMap = %#v, want %#v", got, want)
		}
		if got, want := artifacts.ParsedClientPaths, snapshot.Paths; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("ParsedClientPaths = %#v, want %#v", got, want)
		}
	})
}

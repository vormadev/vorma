package vormaruntime

import (
	"reflect"
	"testing"
)

func TestBuildRuntimeRouteArtifacts(t *testing.T) {
	t.Run("nil_paths_file", func(t *testing.T) {
		_, err := buildRuntimeRouteArtifacts(nil)
		if err == nil {
			t.Fatal("expected error when building artifacts from nil paths file")
		}
	})

	t.Run("maps_paths_file_fields_to_runtime_artifacts", func(t *testing.T) {
		pathsFile := defaultPathsFile("build-artifacts", map[string]*Path{
			"/products/:id": {
				OriginalPattern: "/products/:id",
				SrcPath:         "frontend/src/routes/products.$id.tsx",
				OutPath:         "vorma_out/routes/products.$id.js",
				ExportKey:       "default",
				Deps:            []string{"vorma_out/products.js"},
			},
		})

		artifacts, err := buildRuntimeRouteArtifacts(pathsFile)
		if err != nil {
			t.Fatalf("buildRuntimeRouteArtifacts: %v", err)
		}
		if got, want := artifacts.buildID, pathsFile.BuildID; got != want {
			t.Fatalf("buildID = %q, want %q", got, want)
		}
		if got, want := artifacts.clientEntrySrc, pathsFile.ClientEntrySrc; got != want {
			t.Fatalf("clientEntrySrc = %q, want %q", got, want)
		}
		if got, want := artifacts.clientEntryOut, pathsFile.ClientEntryOut; got != want {
			t.Fatalf("clientEntryOut = %q, want %q", got, want)
		}
		if got, want := artifacts.routeManifestFile, pathsFile.RouteManifestFile; got != want {
			t.Fatalf("routeManifestFile = %q, want %q", got, want)
		}
		if got, want := artifacts.clientEntryDeps, pathsFile.ClientEntryDeps; !reflect.DeepEqual(got, want) {
			t.Fatalf("clientEntryDeps = %#v, want %#v", got, want)
		}
		if got, want := artifacts.depToCSSBundleMap, pathsFile.DepToCSSBundleMap; !reflect.DeepEqual(got, want) {
			t.Fatalf("depToCSSBundleMap = %#v, want %#v", got, want)
		}
		if got, want := artifacts.parsedClientPaths, pathsFile.Paths; !reflect.DeepEqual(got, want) {
			t.Fatalf("parsedClientPaths = %#v, want %#v", got, want)
		}
	})
}

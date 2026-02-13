package vormabuild

import (
	"errors"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestParseClientAndBackendRoutesForSync(t *testing.T) {
	originalParseClientRoutes := buildInnerRouteSyncDeps.parseClientRoutes
	originalParseBackendLoaderPatterns := buildInnerRouteSyncDeps.parseBackendLoaderPatterns
	originalMergeBackendLoaderPatternsInPath := buildInnerRouteSyncDeps.mergeBackendLoaderPatternsInPath
	t.Cleanup(func() {
		buildInnerRouteSyncDeps.parseClientRoutes = originalParseClientRoutes
		buildInnerRouteSyncDeps.parseBackendLoaderPatterns = originalParseBackendLoaderPatterns
		buildInnerRouteSyncDeps.mergeBackendLoaderPatternsInPath = originalMergeBackendLoaderPatternsInPath
	})

	t.Run("merges backend loader patterns into parsed client paths", func(t *testing.T) {
		buildInnerRouteSyncDeps.parseClientRoutes = func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
			return map[string]*vormaruntime.Path{
				"/": {
					OriginalPattern: "/",
					SrcPath:         "frontend/src/routes/home.tsx",
					ExportKey:       "default",
				},
			}, nil
		}
		buildInnerRouteSyncDeps.parseBackendLoaderPatterns = func(*vormaruntime.Vorma) ([]string, error) {
			return []string{"/", "/admin"}, nil
		}
		buildInnerRouteSyncDeps.mergeBackendLoaderPatternsInPath = mergeBackendLoaderPatternsInPath

		paths, err := parseClientAndBackendRoutesForSync(&vormaruntime.Vorma{})
		if err != nil {
			t.Fatalf("parseClientAndBackendRoutesForSync returned error: %v", err)
		}

		if len(paths) != 2 {
			t.Fatalf("paths length = %d, want 2", len(paths))
		}
		if paths["/"] == nil {
			t.Fatalf("expected client path to remain present")
		}
		if got := paths["/"].SrcPath; got != "frontend/src/routes/home.tsx" {
			t.Fatalf("client path src = %q, want %q", got, "frontend/src/routes/home.tsx")
		}
		if paths["/admin"] == nil {
			t.Fatalf("expected backend-only path to be merged")
		}
		if got := paths["/admin"].SrcPath; got != "" {
			t.Fatalf("backend-only path src = %q, want empty string", got)
		}
		if got := paths["/admin"].ExportKey; got != "default" {
			t.Fatalf("backend-only path export key = %q, want %q", got, "default")
		}
	})

	t.Run("returns backend parser error", func(t *testing.T) {
		expectedErr := errors.New("backend parse failed")
		buildInnerRouteSyncDeps.parseClientRoutes = func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
			return map[string]*vormaruntime.Path{}, nil
		}
		buildInnerRouteSyncDeps.parseBackendLoaderPatterns = func(*vormaruntime.Vorma) ([]string, error) {
			return nil, expectedErr
		}
		buildInnerRouteSyncDeps.mergeBackendLoaderPatternsInPath = func(
			map[string]*vormaruntime.Path,
			[]string,
		) map[string]*vormaruntime.Path {
			t.Fatal("did not expect merge after backend parse error")
			return nil
		}

		_, err := parseClientAndBackendRoutesForSync(&vormaruntime.Vorma{})
		if err == nil {
			t.Fatal("expected backend parser error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, want wrapped backend parser error", err)
		}
	})
}

func TestMergeBackendLoaderPatternsInPath(t *testing.T) {
	t.Run("returns original map when no backend patterns are provided", func(t *testing.T) {
		paths := map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		}
		merged := mergeBackendLoaderPatternsInPath(paths, nil)
		if merged["/"] == nil {
			t.Fatalf("expected existing path to remain in map")
		}
	})

	t.Run("initializes map when client paths are nil", func(t *testing.T) {
		merged := mergeBackendLoaderPatternsInPath(nil, []string{"/only-server"})
		if merged["/only-server"] == nil {
			t.Fatalf("expected server-only path to be present")
		}
		if got := merged["/only-server"].ExportKey; got != "default" {
			t.Fatalf("server-only path export key = %q, want %q", got, "default")
		}
	})
}

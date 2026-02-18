package esbuildutil

import (
	"strings"
	"testing"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

func TestCollectErrors_ReturnsNilForEmptyErrors(t *testing.T) {
	if err := CollectErrors(esbuild.BuildResult{}); err != nil {
		t.Fatalf("CollectErrors() error = %v, want nil", err)
	}
}

func TestCollectErrors_JoinsMultipleErrors(t *testing.T) {
	result := esbuild.BuildResult{
		Errors: []esbuild.Message{
			{Text: "first"},
			{Text: "second"},
		},
	}

	err := CollectErrors(result)
	if err == nil {
		t.Fatal("CollectErrors() returned nil for non-empty errors")
	}
	errMessage := err.Error()
	if !strings.Contains(errMessage, "first") || !strings.Contains(errMessage, "second") {
		t.Fatalf("CollectErrors() message %q should contain both error strings", errMessage)
	}
}

func TestFindAllDependencies_IgnoresDynamicImportsAndRecurses(t *testing.T) {
	metafile := &ESBuildMetafileSubset{
		Outputs: map[string]struct {
			Imports []struct {
				Path string `json:"path"`
				Kind string `json:"kind"`
			} `json:"imports"`
			EntryPoint string `json:"entryPoint"`
			CSSBundle  string `json:"cssBundle"`
		}{
			"assets/main.js": {
				Imports: []struct {
					Path string `json:"path"`
					Kind string `json:"kind"`
				}{
					{Path: "assets/chunk.js", Kind: "import-statement"},
					{Path: "assets/lazy.js", Kind: KindDynamicImport},
				},
			},
			"assets/chunk.js": {
				Imports: []struct {
					Path string `json:"path"`
					Kind string `json:"kind"`
				}{
					{Path: "assets/vendor.js", Kind: "import-statement"},
				},
			},
			"assets/vendor.js": {},
		},
	}

	dependencies := FindAllDependencies(metafile, "assets/main.js")
	expected := []string{"main.js", "chunk.js", "vendor.js"}
	if len(dependencies) != len(expected) {
		t.Fatalf("len(FindAllDependencies()) = %d, want %d", len(dependencies), len(expected))
	}
	for i := range expected {
		if dependencies[i] != expected[i] {
			t.Fatalf("FindAllDependencies()[%d] = %q, want %q", i, dependencies[i], expected[i])
		}
	}
}

func TestFindAllDependencies_NilMetafileDoesNotPanicAndReturnsEntrypoint(
	t *testing.T,
) {
	dependencies := FindAllDependencies(nil, "assets/main.js")
	expected := []string{"main.js"}
	if len(dependencies) != len(expected) {
		t.Fatalf("len(FindAllDependencies()) = %d, want %d", len(dependencies), len(expected))
	}
	for i := range expected {
		if dependencies[i] != expected[i] {
			t.Fatalf("FindAllDependencies()[%d] = %q, want %q", i, dependencies[i], expected[i])
		}
	}
}

func TestFindRelativeEntrypointPath_ReturnsMatchingOutputKey(t *testing.T) {
	metafile := &ESBuildMetafileSubset{
		Outputs: map[string]struct {
			Imports []struct {
				Path string `json:"path"`
				Kind string `json:"kind"`
			} `json:"imports"`
			EntryPoint string `json:"entryPoint"`
			CSSBundle  string `json:"cssBundle"`
		}{
			"assets/main.js":  {EntryPoint: "src/main.ts"},
			"assets/admin.js": {EntryPoint: "src/admin.ts"},
		},
	}

	got, err := FindRelativeEntrypointPath(metafile, "src/admin.ts")
	if err != nil {
		t.Fatalf("FindRelativeEntrypointPath() error = %v", err)
	}
	if got != "assets/admin.js" {
		t.Fatalf("FindRelativeEntrypointPath() = %q, want %q", got, "assets/admin.js")
	}
}

func TestFindRelativeEntrypointPath_ReturnsErrorWhenNotFound(t *testing.T) {
	metafile := &ESBuildMetafileSubset{
		Outputs: map[string]struct {
			Imports []struct {
				Path string `json:"path"`
				Kind string `json:"kind"`
			} `json:"imports"`
			EntryPoint string `json:"entryPoint"`
			CSSBundle  string `json:"cssBundle"`
		}{
			"assets/main.js": {EntryPoint: "src/main.ts"},
		},
	}

	_, err := FindRelativeEntrypointPath(metafile, "src/missing.ts")
	if err == nil {
		t.Fatal("expected error when entrypoint is missing")
	}
}

func TestFindRelativeEntrypointPath_NilMetafileReturnsError(t *testing.T) {
	_, err := FindRelativeEntrypointPath(nil, "src/main.ts")
	if err == nil {
		t.Fatal("expected error for nil metafile")
	}
}

func TestFindRelativeEntrypointPath_NormalizesEntrypointPath(
	t *testing.T,
) {
	metafile := &ESBuildMetafileSubset{
		Outputs: map[string]struct {
			Imports []struct {
				Path string `json:"path"`
				Kind string `json:"kind"`
			} `json:"imports"`
			EntryPoint string `json:"entryPoint"`
			CSSBundle  string `json:"cssBundle"`
		}{
			"assets/main.js": {EntryPoint: "src/main.ts"},
		},
	}

	got, err := FindRelativeEntrypointPath(metafile, "./src/main.ts")
	if err != nil {
		t.Fatalf("FindRelativeEntrypointPath() error = %v", err)
	}
	if got != "assets/main.js" {
		t.Fatalf("FindRelativeEntrypointPath() = %q, want %q", got, "assets/main.js")
	}
}

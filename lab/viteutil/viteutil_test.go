package viteutil

import (
	"strings"
	"testing"
)

func TestFindRelativeEntrypointPath_ResolvesFromManifestSourceEntryPath(
	t *testing.T,
) {
	manifest := Manifest{
		"src/main.ts": {
			Src:     "src/main.ts",
			File:    "assets/main-abcdef.js",
			IsEntry: true,
		},
	}

	got, err := FindRelativeEntrypointPath(manifest, "src/main.ts")
	if err != nil {
		t.Fatalf("FindRelativeEntrypointPath() error = %v", err)
	}
	if got != "src/main.ts" {
		t.Fatalf(
			"FindRelativeEntrypointPath() = %q, want %q",
			got,
			"src/main.ts",
		)
	}
}

func TestFindAllDependencies_RecursesThroughManifestImports(t *testing.T) {
	manifest := Manifest{
		"src/main.ts": {
			File:    "assets/main-123.js",
			IsEntry: true,
			Imports: []string{"src/chunk.ts"},
		},
		"src/chunk.ts": {
			File:    "assets/chunk-456.js",
			Imports: []string{"src/vendor.ts"},
		},
		"src/vendor.ts": {
			File: "assets/vendor-789.js",
		},
	}

	dependencies := FindAllDependencies(manifest, "src/main.ts")
	expected := []string{"main-123.js", "chunk-456.js", "vendor-789.js"}
	if len(dependencies) != len(expected) {
		t.Fatalf(
			"len(FindAllDependencies()) = %d, want %d",
			len(dependencies),
			len(expected),
		)
	}
	for i := range expected {
		if dependencies[i] != expected[i] {
			t.Fatalf(
				"FindAllDependencies()[%d] = %q, want %q",
				i,
				dependencies[i],
				expected[i],
			)
		}
	}
}

func TestToDevScripts_ReactIncludesRefreshPreambleAndClientScripts(
	t *testing.T,
) {
	scripts, err := ToDevScripts(ToDevScriptsOptions{
		ClientEntry: "/src/vorma.entry.tsx",
		Variant:     VariantReact,
	})
	if err != nil {
		t.Fatalf("ToDevScripts() error = %v", err)
	}

	scriptsAsString := string(scripts)
	requiredFragments := []string{
		"@react-refresh",
		"http://127.0.0.1:5173/@vite/client",
		"http://127.0.0.1:5173/src/vorma.entry.tsx",
	}
	for _, requiredFragment := range requiredFragments {
		if !strings.Contains(scriptsAsString, requiredFragment) {
			t.Fatalf(
				"expected scripts to contain %q, got %q",
				requiredFragment,
				scriptsAsString,
			)
		}
	}
}

package viteutil

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/netutil"
)

func TestInitPort_HonorsCallerDefaultPortWhenAvailable(
	t *testing.T,
) {
	t.Setenv(PortEnvName, "")

	candidatePort := findAvailablePortInRangeForInitPortTest(
		t,
		20000,
		24000,
	)

	vitePort, err := InitPort(candidatePort)
	if err != nil {
		t.Fatalf("InitPort() returned error: %v", err)
	}
	if vitePort != candidatePort {
		t.Fatalf("expected InitPort() to return caller default port %d, got %d", candidatePort, vitePort)
	}

	if got := GetVitePortStr(); got != fmt.Sprintf("%d", vitePort) {
		t.Fatalf("GetVitePortStr() = %q, want %d", got, vitePort)
	}
}

func findAvailablePortInRangeForInitPortTest(
	t *testing.T,
	rangeStart int,
	rangeEnd int,
) int {
	t.Helper()

	for candidatePort := rangeStart; candidatePort <= rangeEnd; candidatePort++ {
		if netutil.CheckAvailability(candidatePort) {
			return candidatePort
		}
	}

	t.Fatalf(
		"could not find an available port in range [%d, %d]",
		rangeStart,
		rangeEnd,
	)
	return 0
}

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
		t.Fatalf("FindRelativeEntrypointPath() = %q, want %q", got, "src/main.ts")
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
		t.Fatalf("len(FindAllDependencies()) = %d, want %d", len(dependencies), len(expected))
	}
	for i := range expected {
		if dependencies[i] != expected[i] {
			t.Fatalf("FindAllDependencies()[%d] = %q, want %q", i, dependencies[i], expected[i])
		}
	}
}

func TestToDevScripts_ReturnsErrorWhenVitePortIsUnset(t *testing.T) {
	t.Setenv(PortEnvName, "")

	_, err := ToDevScripts(ToDevScriptsOptions{
		ClientEntry: "/src/vorma.entry.tsx",
		Variant:     VariantOther,
	})
	if err == nil {
		t.Fatal("expected ToDevScripts() to fail when __VITE_PORT is unset")
	}
}

func TestToDevScripts_ReturnsErrorWhenVitePortIsInvalid(t *testing.T) {
	t.Setenv(PortEnvName, "not-a-port")

	_, err := ToDevScripts(ToDevScriptsOptions{
		ClientEntry: "/src/vorma.entry.tsx",
		Variant:     VariantOther,
	})
	if err == nil {
		t.Fatal("expected ToDevScripts() to fail when __VITE_PORT is invalid")
	}
}

func TestToDevScripts_ReactIncludesRefreshPreambleAndClientScripts(t *testing.T) {
	t.Setenv(PortEnvName, "5173")

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
			t.Fatalf("expected scripts to contain %q, got %q", requiredFragment, scriptsAsString)
		}
	}
}

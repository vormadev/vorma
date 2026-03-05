package e2e_test

import (
	"os"
	"strings"
	"testing"
)

func TestRuntimeHarnessPinsWavePortResolutionForFixtureRuntime(t *testing.T) {
	runtimeHarnessSourceBytes, readSourceError := os.ReadFile(
		"runtime_harness.ts",
	)
	if readSourceError != nil {
		t.Fatalf(
			"read internal/e2e/runtime_harness.ts: %v",
			readSourceError,
		)
	}
	runtimeHarnessSource := string(runtimeHarnessSourceBytes)

	if !strings.Contains(
		runtimeHarnessSource,
		`const wavePortPinnedEnvironmentVariableName = "__WAVE_PORT_HAS_BEEN_SET";`,
	) {
		t.Fatal(
			"runtime harness must declare the __WAVE_PORT_HAS_BEEN_SET pin variable",
		)
	}

	if !strings.Contains(
		runtimeHarnessSource,
		`[wavePortPinnedEnvironmentVariableName]: "true",`,
	) {
		t.Fatal(
			"runtime harness must pin __WAVE_PORT_HAS_BEEN_SET=true in base runtime env",
		)
	}
}

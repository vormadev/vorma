package vorma

import (
	"fmt"
	"testing"

	"github.com/vormadev/vorma/internal/runtimeguard"
)

// This contract protects runtime/buildtime separation for shipped binaries:
// runtime packages must not pull build/dev tooling dependencies transitively.
func TestRuntimeDependencyContract_DefaultRuntimeRootsExcludeBuildtimeDeps(
	t *testing.T,
) {
	report, evaluateError := runtimeguard.EvaluateDefaultRuntimeDependencyBoundaries()
	if evaluateError != nil {
		t.Fatalf(
			"EvaluateDefaultRuntimeDependencyBoundaries returned error: %v",
			evaluateError,
		)
	}
	if report.HasViolations() {
		t.Fatal(report.FormatViolationReport())
	}
}

func TestRuntimeDependencyContract_BuildtimeSentinelCoverage(t *testing.T) {
	presentSentinelPrefixes, findError := runtimeguard.FindPresentDependencyPrefixMatches(
		runtimeguard.DefaultBuildtimeSentinelRootPackagePath(),
		runtimeguard.DefaultBuildtimeSentinelDependencyPrefixes(),
	)
	if findError != nil {
		t.Fatalf("FindPresentDependencyPrefixMatches returned error: %v", findError)
	}
	if len(presentSentinelPrefixes) > 0 {
		return
	}

	t.Fatalf(
		"buildtime sentinel root %q unexpectedly lacks expected buildtime dependency prefixes: %v",
		runtimeguard.DefaultBuildtimeSentinelRootPackagePath(),
		fmt.Sprint(runtimeguard.DefaultBuildtimeSentinelDependencyPrefixes()),
	)
}

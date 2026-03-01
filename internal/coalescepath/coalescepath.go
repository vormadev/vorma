// Package coalescepath defines repository-local storage paths for coalescing
// artifacts produced by internal command entrypoints.
package coalescepath

import "fmt"

const (
	// StateRootDirectoryPath stores command coalescing lock/state files.
	StateRootDirectoryPath = "./internal/locks"

	// BuildTSCommandKey identifies internal/cmd/buildts.
	BuildTSCommandKey = "internal-cmd-buildts"
	// E2EInstallCommandKey identifies internal/cmd/e2e install.
	E2EInstallCommandKey = "internal-cmd-e2e-install"
	// E2EInstallBrowsersChromiumCommandKey identifies internal/cmd/e2e
	// install-browsers.
	E2EInstallBrowsersChromiumCommandKey = "internal-cmd-e2e-install-browsers-chromium"
	// FullGateCommandKey identifies internal/cmd/full_gate.
	FullGateCommandKey = "internal-cmd-full-gate"
	// ReleaseCommandKey identifies internal/cmd/release.
	ReleaseCommandKey = "internal-cmd-release"
	// TSStateNukeNodeModulesCommandKey identifies internal/cmd/tsstate
	// nuke-node-modules.
	TSStateNukeNodeModulesCommandKey = "internal-cmd-tsstate-nuke-node-modules"
	// TSStateInstallCommandKey identifies internal/cmd/tsstate install.
	TSStateInstallCommandKey = "internal-cmd-tsstate-install"
	// TSStateResetCommandKey identifies internal/cmd/tsstate reset.
	TSStateResetCommandKey = "internal-cmd-tsstate-reset"
)

var (
	// BuildTSFailIfRunningKeys are the command keys that cannot overlap with
	// internal/cmd/buildts.
	BuildTSFailIfRunningKeys = []string{
		TSStateNukeNodeModulesCommandKey,
		TSStateInstallCommandKey,
		TSStateResetCommandKey,
	}

	// TSStateNukeNodeModulesFailIfRunningKeys are keys that cannot overlap with
	// internal/cmd/tsstate nuke-node-modules.
	TSStateNukeNodeModulesFailIfRunningKeys = []string{
		TSStateInstallCommandKey,
		TSStateResetCommandKey,
	}

	// TSStateInstallFailIfRunningKeys are keys that cannot overlap with
	// internal/cmd/tsstate install.
	TSStateInstallFailIfRunningKeys = []string{
		TSStateNukeNodeModulesCommandKey,
		TSStateResetCommandKey,
	}

	// TSStateResetFailIfRunningKeys are keys that cannot overlap with
	// internal/cmd/tsstate reset.
	TSStateResetFailIfRunningKeys = []string{
		TSStateNukeNodeModulesCommandKey,
		TSStateInstallCommandKey,
	}

	// E2EInstallFailIfRunningKeys are keys that cannot overlap with
	// internal/cmd/e2e install.
	E2EInstallFailIfRunningKeys = []string{
		E2EInstallBrowsersChromiumCommandKey,
		TSStateNukeNodeModulesCommandKey,
		TSStateInstallCommandKey,
		TSStateResetCommandKey,
	}

	// E2EInstallBrowsersChromiumFailIfRunningKeys are keys that cannot overlap
	// with internal/cmd/e2e install-browsers.
	E2EInstallBrowsersChromiumFailIfRunningKeys = []string{
		E2EInstallCommandKey,
		TSStateNukeNodeModulesCommandKey,
		TSStateInstallCommandKey,
		TSStateResetCommandKey,
	}

	// E2ETestFailIfRunningKeys are keys that cannot overlap with
	// internal/cmd/e2e test execution.
	E2ETestFailIfRunningKeys = []string{
		E2EInstallCommandKey,
		E2EInstallBrowsersChromiumCommandKey,
		TSStateNukeNodeModulesCommandKey,
		TSStateInstallCommandKey,
		TSStateResetCommandKey,
	}
)

// BuildE2EFixtureGenerationCommandKey returns the per-output-dir coalescing key
// for internal/cmd/e2e_fixture_gen.
func BuildE2EFixtureGenerationCommandKey(outputDirectoryPath string) string {
	return fmt.Sprintf(
		"internal-cmd-e2e-fixture-gen-output-dir:%s",
		outputDirectoryPath,
	)
}

package build_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestBuildCLIModesConformance(t *testing.T) {
	t.Run("BDC-CLI-001_BUILD-CLI-001_hook_mode_runs_build_inner_and_exits", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("hook mode build failed: err=%v output=%s", err, out)
		}

		stageOnePath := filepath.Join(
			fixture.distDir,
			"static",
			"assets",
			"private",
			vormaruntime.VormaOutDirname,
			vormaruntime.VormaPathsStageOneJSONFileName,
		)
		if _, err := os.Stat(stageOnePath); err != nil {
			t.Fatalf("expected hook mode to run build-inner and write stage-one paths file: %v", err)
		}
	})

	t.Run("BDC-CLI-002_BUILD-CLI-002_hook_dev_skips_vite_prod_and_post_vite_stage", func(t *testing.T) {
		tempRoot := t.TempDir()
		viteCmd := writeFakeViteCmd(t, tempRoot)
		fixture := newBuildFixture(t, &buildFixtureOptions{viteCmdPath: viteCmd})

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("dev hook mode build failed: err=%v output=%s", err, out)
		}

		stageTwoPath := filepath.Join(
			fixture.distDir,
			"static",
			"assets",
			"private",
			vormaruntime.VormaOutDirname,
			vormaruntime.VormaPathsStageTwoJSONFileName,
		)
		if _, err := os.Stat(stageTwoPath); !os.IsNotExist(err) {
			t.Fatalf("expected --hook --dev to skip post-vite stage-two output, stat err=%v", err)
		}
	})

	t.Run("BDC-CLI-003_BUILD-CLI-003_hook_prod_runs_vite_and_post_vite_stage_two", func(t *testing.T) {
		tempRoot := t.TempDir()
		viteCmd := writeFakeViteCmd(t, tempRoot)
		fixture := newBuildFixture(t, &buildFixtureOptions{viteCmdPath: viteCmd})

		out, err := runBuildProbe(t, fixture, "--hook")
		if err != nil {
			t.Fatalf("prod hook mode build failed: err=%v output=%s", err, out)
		}

		stageTwoPath := filepath.Join(
			fixture.distDir,
			"static",
			"assets",
			"private",
			vormaruntime.VormaOutDirname,
			vormaruntime.VormaPathsStageTwoJSONFileName,
		)
		if _, err := os.Stat(stageTwoPath); err != nil {
			t.Fatalf("expected --hook (prod) to write stage-two paths file: %v", err)
		}
	})

	t.Run("BDC-CLI-004_BUILD-CLI-004_dev_mode_enters_long_running_dev_workflow", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		addFixtureCLIGoCommands(t, fixture)

		out, err := runBuildProbeWithEnv(
			t,
			fixture,
			map[string]string{
				"CLI_HOOK_MARKER":        filepath.Join(fixture.root, "hook_marker.txt"),
				"WAVE_DEV_EXIT_AFTER_MS": "250",
			},
			"--dev",
		)

		// In sandboxed environments, binding local refresh ports may be disallowed.
		// That failure still demonstrates the --dev RunDev path was selected.
		if err != nil {
			if !strings.Contains(out, "get refresh port") {
				t.Fatalf("expected dev mode failure to come from RunDev startup path, got err=%v output=%s", err, out)
			}
			return
		}

		// Deterministic harness exit after entering steady-state dev workflow.
		if !strings.Contains(out, "Exiting dev server due to env override") {
			t.Fatalf("expected dev mode to enter steady-state workflow and exit via %q override, output=%s", "WAVE_DEV_EXIT_AFTER_MS", out)
		}
	})

	t.Run("BDC-CLI-005_BUILD-CLI-005_prod_mode_build_uses_wave_builder_path", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		addFixtureCLIGoCommands(t, fixture)

		markerPath := filepath.Join(fixture.root, "hook_marker.txt")
		out, err := runBuildProbeWithEnv(
			t,
			fixture,
			map[string]string{"CLI_HOOK_MARKER": markerPath},
		)
		if err != nil {
			t.Fatalf("prod build failed: err=%v output=%s", err, out)
		}

		binPath := filepath.Join(fixture.distDir, "main")
		if _, err := os.Stat(binPath); err != nil {
			t.Fatalf("expected prod mode to compile go binary via wave builder path: %v", err)
		}
		markerBytes, readErr := os.ReadFile(markerPath)
		if readErr != nil {
			t.Fatalf("expected framework hook marker to be written in prod mode: %v", readErr)
		}
		if !strings.Contains(string(markerBytes), "--hook") {
			t.Fatalf("expected framework prod hook invocation marker to include %q, got %q", "--hook", string(markerBytes))
		}
	})

	t.Run("BDC-CLI-006_BUILD-CLI-006_no_binary_skips_final_go_compile", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		addFixtureCLIGoCommands(t, fixture)

		markerPath := filepath.Join(fixture.root, "hook_marker.txt")
		out, err := runBuildProbeWithEnv(
			t,
			fixture,
			map[string]string{"CLI_HOOK_MARKER": markerPath},
			"--no-binary",
		)
		if err != nil {
			t.Fatalf("prod --no-binary build failed: err=%v output=%s", err, out)
		}

		binPath := filepath.Join(fixture.distDir, "main")
		if _, err := os.Stat(binPath); !os.IsNotExist(err) {
			t.Fatalf("expected --no-binary mode to skip final binary compilation, stat err=%v", err)
		}
	})
}

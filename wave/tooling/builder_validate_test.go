package tooling

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestValidateWatchedFile_RunOnChangeOnlyTimingRules(t *testing.T) {
	t.Run("allows non-run-on-change-only regardless of timing", func(t *testing.T) {
		wf := &wave.WatchedFile{
			RunOnChangeOnly: false,
			OnChangeHooks: []wave.OnChangeHook{
				{Cmd: "echo hello", Timing: wave.OnChangeStrategyPost},
			},
		}
		if err := validateWatchedFile(wf, 0); err != nil {
			t.Fatalf("validateWatchedFile returned error: %v", err)
		}
	})

	t.Run("allows pre timing and default timing when run-on-change-only", func(t *testing.T) {
		wf := &wave.WatchedFile{
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{Cmd: "echo hello"},
				{Cmd: "echo world", Timing: wave.OnChangeStrategyPre},
			},
		}
		if err := validateWatchedFile(wf, 1); err != nil {
			t.Fatalf("validateWatchedFile returned error: %v", err)
		}
	})

	t.Run("allows callback-only hooks with non-pre timing", func(t *testing.T) {
		wf := &wave.WatchedFile{
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Timing:   wave.OnChangeStrategyPost,
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) { return nil, nil },
				},
			},
		}
		if err := validateWatchedFile(wf, 2); err != nil {
			t.Fatalf("validateWatchedFile returned error: %v", err)
		}
	})

	t.Run("rejects non-pre command timing when run-on-change-only", func(t *testing.T) {
		wf := &wave.WatchedFile{
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{Cmd: "echo hello", Timing: wave.OnChangeStrategyConcurrent},
			},
		}
		err := validateWatchedFile(wf, 3)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "RunOnChangeOnly") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

func TestValidateConfig_StaticDirRules(t *testing.T) {
	base := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      "dist",
		},
	}

	t.Run("requires static dirs in browser mode", func(t *testing.T) {
		cfg := *base
		cfg.Core = &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      "dist",
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "StaticAssetDirs.Private") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("does not require static dirs in server-only mode", func(t *testing.T) {
		cfg := *base
		cfg.Core = &wave.CoreConfig{
			MainAppEntry:   "cmd/app",
			DistDir:        "dist",
			ServerOnlyMode: true,
		}

		if err := ValidateConfig(&cfg); err != nil {
			t.Fatalf("ValidateConfig returned error: %v", err)
		}
	})
}

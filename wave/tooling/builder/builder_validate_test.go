package builder

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestValidateWatchedFile_RunOnChangeOnlyTimingRules(t *testing.T) {
	t.Run("allows non-run-on-change-only regardless of timing", func(t *testing.T) {
		wf := &wave.WatchedFile{
			Pattern:         "**/*.go",
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
			Pattern:         "**/*.go",
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
			Pattern:         "**/*.go",
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

	t.Run("allows run-combined-dev-build-hook commands with pre timing when run-on-change-only", func(t *testing.T) {
		wf := &wave.WatchedFile{
			Pattern:         "**/*.go",
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{RunCombinedDevBuildHookCommands: true},
				{RunCombinedDevBuildHookCommands: true, Timing: wave.OnChangeStrategyPre},
			},
		}
		if err := validateWatchedFile(wf, 3); err != nil {
			t.Fatalf("validateWatchedFile returned error: %v", err)
		}
	})

	t.Run("rejects non-pre command timing when run-on-change-only", func(t *testing.T) {
		wf := &wave.WatchedFile{
			Pattern:         "**/*.go",
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{Cmd: "echo hello", Timing: wave.OnChangeStrategyConcurrent},
			},
		}
		err := validateWatchedFile(wf, 4)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "RunOnChangeOnly") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects cmd and run-combined-dev-build-hook combination", func(t *testing.T) {
		wf := &wave.WatchedFile{
			Pattern:         "**/*.go",
			RunOnChangeOnly: false,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Cmd:                             "echo hello",
					RunCombinedDevBuildHookCommands: true,
				},
			},
		}
		err := validateWatchedFile(wf, 5)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "RunCombinedDevBuildHookCommands") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects negative per-hook command timeout", func(t *testing.T) {
		wf := &wave.WatchedFile{
			Pattern: "**/*.go",
			OnChangeHooks: []wave.OnChangeHook{
				{
					Cmd:                        "echo hello",
					CommandTimeoutMilliseconds: -1,
				},
			},
		}

		err := validateWatchedFile(wf, 6)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "CommandTimeoutMilliseconds") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects simultaneous disable-stage-timeout and per-hook-timeout", func(t *testing.T) {
		wf := &wave.WatchedFile{
			Pattern: "**/*.go",
			OnChangeHooks: []wave.OnChangeHook{
				{
					Cmd:                        "echo hello",
					CommandTimeoutMilliseconds: 100,
					DisableStageCommandTimeout: true,
				},
			},
		}

		err := validateWatchedFile(wf, 7)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "DisableStageCommandTimeout") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects negative per-hook callback timeout", func(t *testing.T) {
		wf := &wave.WatchedFile{
			Pattern: "**/*.go",
			OnChangeHooks: []wave.OnChangeHook{
				{
					CallbackTimeoutMilliseconds: -1,
				},
			},
		}

		err := validateWatchedFile(wf, 8)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "CallbackTimeoutMilliseconds") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects simultaneous disable-stage-callback-timeout and per-hook-callback-timeout", func(t *testing.T) {
		wf := &wave.WatchedFile{
			Pattern: "**/*.go",
			OnChangeHooks: []wave.OnChangeHook{
				{
					CallbackTimeoutMilliseconds: 100,
					DisableStageCallbackTimeout: true,
				},
			},
		}

		err := validateWatchedFile(wf, 9)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "DisableStageCallbackTimeout") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

func TestValidateConfig_StaticDirRules(t *testing.T) {
	t.Run("rejects nil parsed config", func(t *testing.T) {
		err := ValidateConfig(nil)
		if err == nil {
			t.Fatal("expected validation error for nil parsed config")
		}
		if !strings.Contains(err.Error(), "parsed config is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

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

func TestValidateConfig_HookCommandTimeoutValidation(t *testing.T) {
	baseConfig := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      "dist",
			StaticAssetDirs: staticAssetDirsForTests{
				Private: "static/private",
				Public:  "static/public",
			},
		},
		Watch: &wave.WatchConfig{},
	}

	t.Run("accepts non-negative timeout values", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookCommandTimeouts: wave.HookCommandTimeoutConfig{
				PreCommandTimeoutMilliseconds:              250,
				ConcurrentCommandTimeoutMilliseconds:       500,
				ConcurrentNoWaitCommandTimeoutMilliseconds: 600,
				PostCommandTimeoutMilliseconds:             750,
			},
		}

		if err := ValidateConfig(&cfg); err != nil {
			t.Fatalf("ValidateConfig returned error: %v", err)
		}
	})

	t.Run("rejects negative pre timeout", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookCommandTimeouts: wave.HookCommandTimeoutConfig{
				PreCommandTimeoutMilliseconds: -1,
			},
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error for negative pre timeout")
		}
		if !strings.Contains(err.Error(), "PreCommandTimeoutMilliseconds") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects negative concurrent timeout", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookCommandTimeouts: wave.HookCommandTimeoutConfig{
				ConcurrentCommandTimeoutMilliseconds: -1,
			},
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error for negative concurrent timeout")
		}
		if !strings.Contains(err.Error(), "ConcurrentCommandTimeoutMilliseconds") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects negative concurrent-no-wait timeout", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookCommandTimeouts: wave.HookCommandTimeoutConfig{
				ConcurrentNoWaitCommandTimeoutMilliseconds: -1,
			},
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error for negative concurrent-no-wait timeout")
		}
		if !strings.Contains(err.Error(), "ConcurrentNoWaitCommandTimeoutMilliseconds") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects negative post timeout", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookCommandTimeouts: wave.HookCommandTimeoutConfig{
				PostCommandTimeoutMilliseconds: -1,
			},
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error for negative post timeout")
		}
		if !strings.Contains(err.Error(), "PostCommandTimeoutMilliseconds") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

func TestValidateConfig_RejectsInvalidWatchGlobPatterns(t *testing.T) {
	baseConfig := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      "dist",
			StaticAssetDirs: staticAssetDirsForTests{
				Private: "static/private",
				Public:  "static/public",
			},
		},
		Watch: &wave.WatchConfig{},
	}

	t.Run("rejects invalid Watch.Include pattern", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			Include: []wave.WatchedFile{
				{
					Pattern: "[",
				},
			},
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "Watch.Include[0].Pattern") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects invalid Watch.Exclude.Dirs pattern", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{}
		cfg.Watch.Exclude.Dirs = []string{"["}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "Watch.Exclude.Dirs[0]") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects invalid Watch.Exclude.Files pattern", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{}
		cfg.Watch.Exclude.Files = []string{"["}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "Watch.Exclude.Files[0]") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects invalid OnChangeHooks exclude pattern", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			Include: []wave.WatchedFile{
				{
					Pattern: "**/*.go",
					OnChangeHooks: []wave.OnChangeHook{
						{
							Cmd:     "echo hello",
							Exclude: []string{"["},
						},
					},
				},
			},
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "Watch.Include[0].OnChangeHooks[0].Exclude[0]") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

func TestValidateConfig_BuildHookTimeoutValidation(t *testing.T) {
	baseConfig := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      "dist",
			StaticAssetDirs: staticAssetDirsForTests{
				Private: "static/private",
				Public:  "static/public",
			},
		},
	}

	t.Run("accepts non-negative core build hook timeouts", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Core = &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      "dist",
			StaticAssetDirs: staticAssetDirsForTests{
				Private: "static/private",
				Public:  "static/public",
			},
			DevBuildHookTimeoutMilliseconds:  250,
			ProdBuildHookTimeoutMilliseconds: 500,
		}

		if err := ValidateConfig(&cfg); err != nil {
			t.Fatalf("ValidateConfig returned error: %v", err)
		}
	})

	t.Run("rejects negative dev build hook timeout", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Core = &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      "dist",
			StaticAssetDirs: staticAssetDirsForTests{
				Private: "static/private",
				Public:  "static/public",
			},
			DevBuildHookTimeoutMilliseconds: -1,
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error for negative dev build hook timeout")
		}
		if !strings.Contains(err.Error(), "DevBuildHookTimeoutMilliseconds") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects negative prod build hook timeout", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Core = &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      "dist",
			StaticAssetDirs: staticAssetDirsForTests{
				Private: "static/private",
				Public:  "static/public",
			},
			ProdBuildHookTimeoutMilliseconds: -1,
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error for negative prod build hook timeout")
		}
		if !strings.Contains(err.Error(), "ProdBuildHookTimeoutMilliseconds") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

func TestValidateConfig_HookCallbackTimeoutValidation(t *testing.T) {
	baseConfig := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      "dist",
			StaticAssetDirs: staticAssetDirsForTests{
				Private: "static/private",
				Public:  "static/public",
			},
		},
		Watch: &wave.WatchConfig{},
	}

	t.Run("accepts non-negative callback timeout values", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookCallbackTimeouts: wave.HookCallbackTimeoutConfig{
				PreCallbackTimeoutMilliseconds:              250,
				ConcurrentCallbackTimeoutMilliseconds:       500,
				ConcurrentNoWaitCallbackTimeoutMilliseconds: 600,
				PostCallbackTimeoutMilliseconds:             750,
			},
		}

		if err := ValidateConfig(&cfg); err != nil {
			t.Fatalf("ValidateConfig returned error: %v", err)
		}
	})

	t.Run("rejects negative pre callback timeout", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookCallbackTimeouts: wave.HookCallbackTimeoutConfig{
				PreCallbackTimeoutMilliseconds: -1,
			},
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error for negative pre callback timeout")
		}
		if !strings.Contains(err.Error(), "PreCallbackTimeoutMilliseconds") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects negative concurrent callback timeout", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookCallbackTimeouts: wave.HookCallbackTimeoutConfig{
				ConcurrentCallbackTimeoutMilliseconds: -1,
			},
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error for negative concurrent callback timeout")
		}
		if !strings.Contains(err.Error(), "ConcurrentCallbackTimeoutMilliseconds") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects negative concurrent-no-wait callback timeout", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookCallbackTimeouts: wave.HookCallbackTimeoutConfig{
				ConcurrentNoWaitCallbackTimeoutMilliseconds: -1,
			},
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error for negative concurrent-no-wait callback timeout")
		}
		if !strings.Contains(err.Error(), "ConcurrentNoWaitCallbackTimeoutMilliseconds") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects negative post callback timeout", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookCallbackTimeouts: wave.HookCallbackTimeoutConfig{
				PostCallbackTimeoutMilliseconds: -1,
			},
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error for negative post callback timeout")
		}
		if !strings.Contains(err.Error(), "PostCallbackTimeoutMilliseconds") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

func TestValidateConfig_HealthcheckEndpointValidation(t *testing.T) {
	baseConfig := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      "dist",
			StaticAssetDirs: staticAssetDirsForTests{
				Private: "static/private",
				Public:  "static/public",
			},
		},
		Watch: &wave.WatchConfig{},
	}

	t.Run("accepts empty healthcheck endpoint", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{HealthcheckEndpoint: ""}
		if err := ValidateConfig(&cfg); err != nil {
			t.Fatalf("ValidateConfig returned error: %v", err)
		}
	})

	t.Run("accepts absolute path endpoint", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{HealthcheckEndpoint: "/healthz"}
		if err := ValidateConfig(&cfg); err != nil {
			t.Fatalf("ValidateConfig returned error: %v", err)
		}
	})

	testCases := []struct {
		Name                string
		HealthcheckEndpoint string
		ExpectedSubstring   string
	}{
		{
			Name:                "rejects missing leading slash",
			HealthcheckEndpoint: "healthz",
			ExpectedSubstring:   "must start with '/'",
		},
		{
			Name:                "rejects full URL",
			HealthcheckEndpoint: "https://example.com/healthz",
			ExpectedSubstring:   "must be a path, not a URL",
		},
		{
			Name:                "rejects query strings",
			HealthcheckEndpoint: "/healthz?full=1",
			ExpectedSubstring:   "must not include query or fragment",
		},
		{
			Name:                "rejects fragments",
			HealthcheckEndpoint: "/healthz#ready",
			ExpectedSubstring:   "must not include query or fragment",
		},
		{
			Name:                "rejects surrounding whitespace",
			HealthcheckEndpoint: " /healthz",
			ExpectedSubstring:   "must not include surrounding whitespace",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			cfg := *baseConfig
			cfg.Watch = &wave.WatchConfig{
				HealthcheckEndpoint: testCase.HealthcheckEndpoint,
			}

			err := ValidateConfig(&cfg)
			if err == nil {
				t.Fatal("expected validation error for healthcheck endpoint")
			}
			if !strings.Contains(err.Error(), "Watch.HealthcheckEndpoint") {
				t.Fatalf("unexpected error message: %v", err)
			}
			if !strings.Contains(err.Error(), testCase.ExpectedSubstring) {
				t.Fatalf("unexpected error message: %v", err)
			}
		})
	}
}

func TestValidateConfig_HookStageFailurePolicyValidation(t *testing.T) {
	baseConfig := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      "dist",
			StaticAssetDirs: staticAssetDirsForTests{
				Private: "static/private",
				Public:  "static/public",
			},
		},
		Watch: &wave.WatchConfig{},
	}

	t.Run("accepts default empty value", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{}
		if err := ValidateConfig(&cfg); err != nil {
			t.Fatalf("ValidateConfig returned error: %v", err)
		}
	})

	t.Run("accepts explicit fail-open", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookStageFailurePolicy: configuredHookStageFailurePolicyFailOpen,
		}
		if err := ValidateConfig(&cfg); err != nil {
			t.Fatalf("ValidateConfig returned error: %v", err)
		}
	})

	t.Run("accepts explicit fail-closed", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookStageFailurePolicy: configuredHookStageFailurePolicyFailClosed,
		}
		if err := ValidateConfig(&cfg); err != nil {
			t.Fatalf("ValidateConfig returned error: %v", err)
		}
	})

	t.Run("rejects unknown hook-stage failure policy", func(t *testing.T) {
		cfg := *baseConfig
		cfg.Watch = &wave.WatchConfig{
			HookStageFailurePolicy: "invalid-policy",
		}

		err := ValidateConfig(&cfg)
		if err == nil {
			t.Fatal("expected validation error for invalid hook-stage failure policy")
		}
		if !strings.Contains(err.Error(), "Watch.HookStageFailurePolicy") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

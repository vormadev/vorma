package tooling

import (
	"fmt"
	"strings"

	"github.com/vormadev/vorma/wave"
)

// ValidateConfig performs full validation of the Wave configuration.
// This should be called at build time before any build operations.
func ValidateConfig(cfg *wave.ParsedConfig) error {
	if cfg.Core == nil {
		return fmt.Errorf("config: Core section is required")
	}
	if cfg.Core.MainAppEntry == "" {
		return fmt.Errorf("config: Core.MainAppEntry is required")
	}
	if cfg.Core.DistDir == "" {
		return fmt.Errorf("config: Core.DistDir is required")
	}
	if cfg.Core.DevBuildHookTimeoutMilliseconds < 0 {
		return fmt.Errorf("config: Core.DevBuildHookTimeoutMilliseconds must be >= 0")
	}
	if cfg.Core.ProdBuildHookTimeoutMilliseconds < 0 {
		return fmt.Errorf("config: Core.ProdBuildHookTimeoutMilliseconds must be >= 0")
	}

	if !cfg.Core.ServerOnlyMode {
		if cfg.Core.StaticAssetDirs.Private == "" {
			return fmt.Errorf("config: Core.StaticAssetDirs.Private is required")
		}
		if cfg.Core.StaticAssetDirs.Public == "" {
			return fmt.Errorf("config: Core.StaticAssetDirs.Public is required")
		}
	}

	if cfg.Vite != nil {
		if cfg.Vite.JSPackageManagerBaseCmd == "" {
			return fmt.Errorf("config: Vite.JSPackageManagerBaseCmd is required")
		}
	}

	if cfg.Watch != nil {
		if err := validateHealthcheckEndpoint(cfg.Watch.HealthcheckEndpoint); err != nil {
			return err
		}
		if err := validateHookCommandTimeoutConfig(cfg.Watch.HookCommandTimeouts); err != nil {
			return err
		}
		if err := validateHookCallbackTimeoutConfig(cfg.Watch.HookCallbackTimeouts); err != nil {
			return err
		}

		for i, watchedFile := range cfg.Watch.Include {
			if err := validateWatchedFile(&watchedFile, i); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateHookCallbackTimeoutConfig(
	hookCallbackTimeoutConfig wave.HookCallbackTimeoutConfig,
) error {
	if hookCallbackTimeoutConfig.PreCallbackTimeoutMilliseconds < 0 {
		return fmt.Errorf(
			"config: Watch.HookCallbackTimeouts.PreCallbackTimeoutMilliseconds must be >= 0",
		)
	}

	if hookCallbackTimeoutConfig.ConcurrentCallbackTimeoutMilliseconds < 0 {
		return fmt.Errorf(
			"config: Watch.HookCallbackTimeouts.ConcurrentCallbackTimeoutMilliseconds must be >= 0",
		)
	}

	if hookCallbackTimeoutConfig.ConcurrentNoWaitCallbackTimeoutMilliseconds < 0 {
		return fmt.Errorf(
			"config: Watch.HookCallbackTimeouts.ConcurrentNoWaitCallbackTimeoutMilliseconds must be >= 0",
		)
	}

	if hookCallbackTimeoutConfig.PostCallbackTimeoutMilliseconds < 0 {
		return fmt.Errorf(
			"config: Watch.HookCallbackTimeouts.PostCallbackTimeoutMilliseconds must be >= 0",
		)
	}

	return nil
}

func validateHookCommandTimeoutConfig(
	hookCommandTimeoutConfig wave.HookCommandTimeoutConfig,
) error {
	if hookCommandTimeoutConfig.PreCommandTimeoutMilliseconds < 0 {
		return fmt.Errorf(
			"config: Watch.HookCommandTimeouts.PreCommandTimeoutMilliseconds must be >= 0",
		)
	}

	if hookCommandTimeoutConfig.ConcurrentCommandTimeoutMilliseconds < 0 {
		return fmt.Errorf(
			"config: Watch.HookCommandTimeouts.ConcurrentCommandTimeoutMilliseconds must be >= 0",
		)
	}

	if hookCommandTimeoutConfig.ConcurrentNoWaitCommandTimeoutMilliseconds < 0 {
		return fmt.Errorf(
			"config: Watch.HookCommandTimeouts.ConcurrentNoWaitCommandTimeoutMilliseconds must be >= 0",
		)
	}

	if hookCommandTimeoutConfig.PostCommandTimeoutMilliseconds < 0 {
		return fmt.Errorf(
			"config: Watch.HookCommandTimeouts.PostCommandTimeoutMilliseconds must be >= 0",
		)
	}

	return nil
}

func validateWatchedFile(wf *wave.WatchedFile, index int) error {
	for hookIndex, hook := range wf.OnChangeHooks {
		if hook.CommandTimeoutMilliseconds < 0 {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d].CommandTimeoutMilliseconds must be >= 0",
				index,
				hookIndex,
			)
		}
		if hook.DisableStageCommandTimeout && hook.CommandTimeoutMilliseconds > 0 {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] cannot set both DisableStageCommandTimeout and CommandTimeoutMilliseconds",
				index,
				hookIndex,
			)
		}
		if hook.CallbackTimeoutMilliseconds < 0 {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d].CallbackTimeoutMilliseconds must be >= 0",
				index,
				hookIndex,
			)
		}
		if hook.DisableStageCallbackTimeout && hook.CallbackTimeoutMilliseconds > 0 {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] cannot set both DisableStageCallbackTimeout and CallbackTimeoutMilliseconds",
				index,
				hookIndex,
			)
		}

		if strings.TrimSpace(hook.Cmd) != "" && hook.RunCombinedDevBuildHookCommands {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] cannot set both Cmd and RunCombinedDevBuildHookCommands",
				index,
				hookIndex,
			)
		}

		if !wf.RunOnChangeOnly {
			continue
		}

		// Callbacks can use any timing - they return RefreshAction to control behavior.
		// This validation only applies to command-like hooks.
		if strings.TrimSpace(hook.Cmd) == "" && !hook.RunCombinedDevBuildHookCommands {
			continue
		}

		if hook.Timing != "" && hook.Timing != wave.OnChangeStrategyPre {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] has Timing %q but RunOnChangeOnly requires all command hooks to use \"pre\" timing (the default)",
				index,
				hookIndex,
				hook.Timing,
			)
		}
	}

	return nil
}

func validateHealthcheckEndpoint(healthcheckEndpoint string) error {
	if healthcheckEndpoint == "" {
		return nil
	}

	if strings.TrimSpace(healthcheckEndpoint) != healthcheckEndpoint {
		return fmt.Errorf("config: Watch.HealthcheckEndpoint must not include surrounding whitespace")
	}

	if strings.ContainsAny(healthcheckEndpoint, "\t\r\n ") {
		return fmt.Errorf("config: Watch.HealthcheckEndpoint must not contain whitespace")
	}

	if strings.Contains(healthcheckEndpoint, "://") {
		return fmt.Errorf("config: Watch.HealthcheckEndpoint must be a path, not a URL")
	}

	if !strings.HasPrefix(healthcheckEndpoint, "/") {
		return fmt.Errorf("config: Watch.HealthcheckEndpoint must start with '/'")
	}

	if strings.HasPrefix(healthcheckEndpoint, "//") {
		return fmt.Errorf("config: Watch.HealthcheckEndpoint must be a single absolute path")
	}

	if strings.Contains(healthcheckEndpoint, "?") || strings.Contains(healthcheckEndpoint, "#") {
		return fmt.Errorf("config: Watch.HealthcheckEndpoint must not include query or fragment segments")
	}

	return nil
}

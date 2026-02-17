package tooling

import (
	"fmt"
	"strings"

	"github.com/vormadev/vorma/wave"
)

// ValidateConfig performs full validation of the Wave configuration.
// This should be called at build time before any build operations.
func ValidateConfig(cfg *wave.ParsedConfig) error {
	if cfg == nil {
		return fmt.Errorf("config: parsed config is required")
	}
	if cfg.Core == nil {
		return fmt.Errorf("config: Core section is required")
	}
	if cfg.Core.MainAppEntry == "" {
		return fmt.Errorf("config: Core.MainAppEntry is required")
	}
	if cfg.Core.DistDir == "" {
		return fmt.Errorf("config: Core.DistDir is required")
	}
	if err := validateNonNegativeTimeoutFields(
		[]timeoutFieldValidation{
			{
				fieldPath:           "Core.DevBuildHookTimeoutMilliseconds",
				timeoutMilliseconds: cfg.Core.DevBuildHookTimeoutMilliseconds,
			},
			{
				fieldPath:           "Core.ProdBuildHookTimeoutMilliseconds",
				timeoutMilliseconds: cfg.Core.ProdBuildHookTimeoutMilliseconds,
			},
		},
	); err != nil {
		return err
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
		if err := validateHookStageFailurePolicy(cfg.Watch.HookStageFailurePolicy); err != nil {
			return err
		}
		if err := validateHookCommandTimeoutConfig(cfg.Watch.HookCommandTimeouts); err != nil {
			return err
		}
		if err := validateHookCallbackTimeoutConfig(cfg.Watch.HookCallbackTimeouts); err != nil {
			return err
		}

		for excludedDirectoryPatternIndex, excludedDirectoryPattern := range cfg.Watch.Exclude.Dirs {
			if err := validateWatchGlobPattern(
				fmt.Sprintf("Watch.Exclude.Dirs[%d]", excludedDirectoryPatternIndex),
				excludedDirectoryPattern,
			); err != nil {
				return err
			}
		}
		for excludedFilePatternIndex, excludedFilePattern := range cfg.Watch.Exclude.Files {
			if err := validateWatchGlobPattern(
				fmt.Sprintf("Watch.Exclude.Files[%d]", excludedFilePatternIndex),
				excludedFilePattern,
			); err != nil {
				return err
			}
		}

		for i, watchedFile := range cfg.Watch.Include {
			if err := validateWatchedFile(&watchedFile, i); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateHookStageFailurePolicy(
	hookStageFailurePolicy string,
) error {
	normalizedHookStageFailurePolicy := normalizeConfiguredHookStageFailurePolicy(
		hookStageFailurePolicy,
	)
	switch normalizedHookStageFailurePolicy {
	case "",
		configuredHookStageFailurePolicyFailOpen,
		configuredHookStageFailurePolicyFailClosed:
		return nil
	default:
		return fmt.Errorf(
			"config: Watch.HookStageFailurePolicy must be one of %q or %q",
			configuredHookStageFailurePolicyFailOpen,
			configuredHookStageFailurePolicyFailClosed,
		)
	}
}

func validateHookCallbackTimeoutConfig(
	hookCallbackTimeoutConfig wave.HookCallbackTimeoutConfig,
) error {
	return validateNonNegativeTimeoutFields(
		[]timeoutFieldValidation{
			{
				fieldPath:           "Watch.HookCallbackTimeouts.PreCallbackTimeoutMilliseconds",
				timeoutMilliseconds: hookCallbackTimeoutConfig.PreCallbackTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCallbackTimeouts.ConcurrentCallbackTimeoutMilliseconds",
				timeoutMilliseconds: hookCallbackTimeoutConfig.ConcurrentCallbackTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCallbackTimeouts.ConcurrentNoWaitCallbackTimeoutMilliseconds",
				timeoutMilliseconds: hookCallbackTimeoutConfig.ConcurrentNoWaitCallbackTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCallbackTimeouts.PostCallbackTimeoutMilliseconds",
				timeoutMilliseconds: hookCallbackTimeoutConfig.PostCallbackTimeoutMilliseconds,
			},
		},
	)
}

func validateHookCommandTimeoutConfig(
	hookCommandTimeoutConfig wave.HookCommandTimeoutConfig,
) error {
	return validateNonNegativeTimeoutFields(
		[]timeoutFieldValidation{
			{
				fieldPath:           "Watch.HookCommandTimeouts.PreCommandTimeoutMilliseconds",
				timeoutMilliseconds: hookCommandTimeoutConfig.PreCommandTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCommandTimeouts.ConcurrentCommandTimeoutMilliseconds",
				timeoutMilliseconds: hookCommandTimeoutConfig.ConcurrentCommandTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCommandTimeouts.ConcurrentNoWaitCommandTimeoutMilliseconds",
				timeoutMilliseconds: hookCommandTimeoutConfig.ConcurrentNoWaitCommandTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCommandTimeouts.PostCommandTimeoutMilliseconds",
				timeoutMilliseconds: hookCommandTimeoutConfig.PostCommandTimeoutMilliseconds,
			},
		},
	)
}

type timeoutFieldValidation struct {
	fieldPath           string
	timeoutMilliseconds int
}

func validateNonNegativeTimeoutFields(
	timeoutFieldValidations []timeoutFieldValidation,
) error {
	for _, timeoutFieldValidation := range timeoutFieldValidations {
		if timeoutFieldValidation.timeoutMilliseconds < 0 {
			return fmt.Errorf(
				"config: %s must be >= 0",
				timeoutFieldValidation.fieldPath,
			)
		}
	}

	return nil
}

func validateWatchedFile(wf *wave.WatchedFile, index int) error {
	if err := validateWatchGlobPattern(
		fmt.Sprintf("Watch.Include[%d].Pattern", index),
		wf.Pattern,
	); err != nil {
		return err
	}

	for hookIndex, hook := range wf.OnChangeHooks {
		for excludedPatternIndex, excludedPattern := range hook.Exclude {
			if err := validateWatchGlobPattern(
				fmt.Sprintf(
					"Watch.Include[%d].OnChangeHooks[%d].Exclude[%d]",
					index,
					hookIndex,
					excludedPatternIndex,
				),
				excludedPattern,
			); err != nil {
				return err
			}
		}

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

func validateWatchGlobPattern(
	fieldPath string,
	globPattern string,
) error {
	return validateNamedGlobPatternInput("config", fieldPath, globPattern)
}

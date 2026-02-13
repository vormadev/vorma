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
		for i, watchedFile := range cfg.Watch.Include {
			if err := validateWatchedFile(&watchedFile, i); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateWatchedFile(wf *wave.WatchedFile, index int) error {
	for hookIndex, hook := range wf.OnChangeHooks {
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

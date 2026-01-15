package wave

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ParseConfig parses and validates wave.config.json bytes
func ParseConfig(data []byte) (*ParsedConfig, error) {
	var cfg ParsedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	cfg.Dist = DistLayout{Root: filepath.Clean(cfg.Core.DistDir)}

	return &cfg, nil
}

// ParseConfigFile reads and parses a config file
func ParseConfigFile(path string) (*ParsedConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	return ParseConfig(data)
}

func validateConfig(cfg *ParsedConfig) error {
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
		for i, wf := range cfg.Watch.Include {
			if err := validateWatchedFile(&wf, i); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateWatchedFile(wf *WatchedFile, index int) error {
	if !wf.RunOnChangeOnly {
		return nil
	}

	for j, hook := range wf.OnChangeHooks {
		// Callbacks can use any timing - they return RefreshAction to control behavior.
		// This validation only applies to Cmd hooks.
		if hook.Cmd == "" {
			continue
		}

		if hook.Timing != "" && hook.Timing != OnChangeStrategyPre {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] has Timing %q but RunOnChangeOnly requires all Cmd hooks to use \"pre\" timing (the default)",
				index, j, hook.Timing,
			)
		}
	}

	return nil
}

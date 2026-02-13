package wave

import (
	"fmt"
	"strings"
)

func (cfg *ParsedConfig) GetResolvedConfigSource() ConfigSource {
	if cfg == nil {
		return nil
	}
	return cfg.ResolvedConfigSource
}

func (cfg *ParsedConfig) GetResolvedConfigDependencies() ConfigProviderDependencies {
	if cfg == nil {
		return ConfigProviderDependencies{}
	}

	configDependencyFiles := append([]string(nil), cfg.ResolvedConfigDependencies.Files...)
	configDependencyGlobs := append([]string(nil), cfg.ResolvedConfigDependencies.Globs...)
	configDependencyEnvVars := append([]string(nil), cfg.ResolvedConfigDependencies.Env...)

	return ConfigProviderDependencies{
		Files: normalizeProviderDependencyValues(configDependencyFiles),
		Globs: normalizeProviderDependencyValues(configDependencyGlobs),
		Env:   normalizeProviderDependencyValues(configDependencyEnvVars),
	}
}

func (cfg *ParsedConfig) GetResolvedConfigFingerprint() string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.ResolvedConfigFingerprint)
}

func (cfg *ParsedConfig) IsResolvedConfigDependencyPath(
	path string,
) bool {
	if cfg == nil {
		return false
	}

	configDependencyMatcher := cfg.resolvedConfigDependencyMatcher
	if configDependencyMatcher == nil {
		var err error
		configDependencyMatcher, err = newResolvedConfigDependencyMatcher(
			cfg.GetResolvedConfigDependencies(),
		)
		if err != nil {
			return false
		}
	}

	return configDependencyMatcher.matchesPath(path)
}

func setResolvedConfigRuntimeState(
	cfg *ParsedConfig,
	configSource ConfigSource,
	configDependencies ConfigProviderDependencies,
	configFingerprint string,
) error {
	if cfg == nil {
		return nil
	}

	cfg.ResolvedConfigSource = configSource
	cfg.ResolvedConfigDependencies = ConfigProviderDependencies{
		Files: append([]string(nil), configDependencies.Files...),
		Globs: append([]string(nil), configDependencies.Globs...),
		Env:   append([]string(nil), configDependencies.Env...),
	}
	cfg.ResolvedConfigFingerprint = strings.TrimSpace(configFingerprint)

	resolvedDependencies := cfg.GetResolvedConfigDependencies()
	cfg.ResolvedConfigDependencies = resolvedDependencies

	resolvedConfigDependencyMatcher, err := newResolvedConfigDependencyMatcher(
		resolvedDependencies,
	)
	if err != nil {
		return fmt.Errorf("set resolved config runtime state: %w", err)
	}
	cfg.resolvedConfigDependencyMatcher = resolvedConfigDependencyMatcher

	return nil
}

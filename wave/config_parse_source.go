package wave

import "fmt"

func ParseConfigFromLoadedConfig(
	loadedConfig *LoadedConfig,
	configSource ConfigSource,
) (*ParsedConfig, error) {
	if loadedConfig == nil {
		return nil, fmt.Errorf("parse config from loaded config: loaded config is nil")
	}

	parsedConfig, err := ParseConfig(loadedConfig.ConfigJSON)
	if err != nil {
		return nil, fmt.Errorf("parse config from loaded config: %w", err)
	}

	if err := setResolvedConfigRuntimeState(
		parsedConfig,
		configSource,
		loadedConfig.Dependencies,
		loadedConfig.Fingerprint,
	); err != nil {
		return nil, fmt.Errorf("parse config from loaded config: %w", err)
	}

	return parsedConfig, nil
}

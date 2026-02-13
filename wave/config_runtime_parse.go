package wave

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// ParseConfigWithRuntimeMetadata parses config JSON and attaches reload metadata.
func ParseConfigWithRuntimeMetadata(
	configJSON []byte,
	configFilePath string,
) (*ParsedConfig, error) {
	parsedConfig, err := ParseConfig(configJSON)
	if err != nil {
		return nil, fmt.Errorf("parse config with runtime metadata: %w", err)
	}

	trimmedConfigFilePath := strings.TrimSpace(configFilePath)
	configFingerprint, err := computeConfigFingerprint(configJSON)
	if err != nil {
		return nil, fmt.Errorf("parse config with runtime metadata: %w", err)
	}

	setResolvedConfigRuntimeState(
		parsedConfig,
		trimmedConfigFilePath,
		configFingerprint,
	)

	return parsedConfig, nil
}

func computeConfigFingerprint(
	configJSON []byte,
) (string, error) {
	var configAny any
	if err := json.Unmarshal(configJSON, &configAny); err != nil {
		return "", fmt.Errorf("compute config fingerprint: invalid config JSON: %w", err)
	}

	canonicalConfigJSON, err := json.Marshal(configAny)
	if err != nil {
		return "", fmt.Errorf("compute config fingerprint: canonicalize config JSON: %w", err)
	}

	hashSum := sha256.Sum256(canonicalConfigJSON)
	return hex.EncodeToString(hashSum[:]), nil
}

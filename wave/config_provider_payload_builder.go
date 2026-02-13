package wave

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func BuildConfigProviderPayload(
	configJSON []byte,
	configDependencies ConfigProviderDependencies,
) (*ConfigProviderPayload, error) {
	if len(configJSON) == 0 {
		return nil, fmt.Errorf("build provider payload: config JSON is required")
	}

	if _, err := ParseConfig(configJSON); err != nil {
		return nil, fmt.Errorf("build provider payload: %w", err)
	}

	canonicalConfigJSON, err := canonicalizeJSON(configJSON)
	if err != nil {
		return nil, fmt.Errorf("build provider payload: canonicalize config JSON: %w", err)
	}

	normalizedDependencies := ConfigProviderDependencies{
		Files: normalizeProviderDependencyValues(configDependencies.Files),
		Globs: normalizeProviderDependencyValues(configDependencies.Globs),
		Env:   normalizeProviderDependencyValues(configDependencies.Env),
	}

	return &ConfigProviderPayload{
		Version:      1,
		Config:       append([]byte(nil), configJSON...),
		Dependencies: normalizedDependencies,
		Fingerprint:  computeConfigProviderFingerprint(canonicalConfigJSON, normalizedDependencies),
	}, nil
}

func MarshalConfigProviderPayload(
	configJSON []byte,
	configDependencies ConfigProviderDependencies,
) ([]byte, error) {
	payload, err := BuildConfigProviderPayload(configJSON, configDependencies)
	if err != nil {
		return nil, err
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal provider payload: %w", err)
	}

	return payloadJSON, nil
}

func canonicalizeJSON(configJSON []byte) ([]byte, error) {
	var configAny any
	if err := json.Unmarshal(configJSON, &configAny); err != nil {
		return nil, err
	}
	return json.Marshal(configAny)
}

func computeConfigProviderFingerprint(
	canonicalConfigJSON []byte,
	configDependencies ConfigProviderDependencies,
) string {
	hashInput := make([]byte, 0, len(canonicalConfigJSON)+256)
	hashInput = append(hashInput, canonicalConfigJSON...)
	hashInput = append(hashInput, '\n')

	appendDependencyValuesWithPrefix := func(prefix string, dependencyValues []string) {
		for _, dependencyValue := range dependencyValues {
			hashInput = append(hashInput, prefix...)
			hashInput = append(hashInput, dependencyValue...)
			hashInput = append(hashInput, '\n')
		}
	}

	appendDependencyValuesWithPrefix("file:", configDependencies.Files)
	appendDependencyValuesWithPrefix("glob:", configDependencies.Globs)
	appendDependencyValuesWithPrefix("env:", configDependencies.Env)

	hashSum := sha256.Sum256(hashInput)
	return hex.EncodeToString(hashSum[:])
}

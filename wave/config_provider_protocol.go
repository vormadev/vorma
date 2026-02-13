package wave

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type ConfigProviderDependencies struct {
	Files []string `json:"files,omitempty"`
	Globs []string `json:"globs,omitempty"`
	Env   []string `json:"env,omitempty"`
}

type ConfigProviderPayload struct {
	Version      int                        `json:"version"`
	Config       json.RawMessage            `json:"config"`
	Dependencies ConfigProviderDependencies `json:"dependencies,omitempty"`
	Fingerprint  string                     `json:"fingerprint,omitempty"`
}

func ParseConfigProviderPayload(payloadBytes []byte) (*ConfigProviderPayload, error) {
	var payload ConfigProviderPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("parse provider payload: %w", err)
	}

	if payload.Version <= 0 {
		return nil, fmt.Errorf("parse provider payload: version must be > 0")
	}

	if len(payload.Config) == 0 {
		return nil, fmt.Errorf("parse provider payload: config is required")
	}

	var configAny any
	if err := json.Unmarshal(payload.Config, &configAny); err != nil {
		return nil, fmt.Errorf("parse provider payload: config must be valid JSON: %w", err)
	}
	if _, isObject := configAny.(map[string]any); !isObject {
		return nil, fmt.Errorf("parse provider payload: config must be a JSON object")
	}

	payload.Dependencies.Files = normalizeProviderDependencyValues(payload.Dependencies.Files)
	payload.Dependencies.Globs = normalizeProviderDependencyValues(payload.Dependencies.Globs)
	payload.Dependencies.Env = normalizeProviderDependencyValues(payload.Dependencies.Env)

	return &payload, nil
}

func normalizeProviderDependencyValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seenValues := make(map[string]struct{}, len(values))
	normalizedValues := make([]string, 0, len(values))
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		if _, alreadySeen := seenValues[trimmedValue]; alreadySeen {
			continue
		}
		seenValues[trimmedValue] = struct{}{}
		normalizedValues = append(normalizedValues, trimmedValue)
	}

	sort.Strings(normalizedValues)
	return normalizedValues
}

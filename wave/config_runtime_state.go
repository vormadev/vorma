package wave

import (
	"path/filepath"
	"strings"
)

func (cfg *ParsedConfig) GetResolvedConfigFilePath() string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.ResolvedConfigFilePath)
}

func (cfg *ParsedConfig) GetResolvedConfigFingerprint() string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.ResolvedConfigFingerprint)
}

func (cfg *ParsedConfig) IsResolvedConfigFilePath(
	path string,
) bool {
	if cfg == nil {
		return false
	}

	resolvedConfigPath := normalizeResolvedConfigPathForMatch(cfg.GetResolvedConfigFilePath())
	if resolvedConfigPath == "" {
		return false
	}

	normalizedCandidatePath := normalizeResolvedConfigPathForMatch(path)
	if normalizedCandidatePath == "" {
		return false
	}

	return normalizedCandidatePath == resolvedConfigPath
}

func setResolvedConfigRuntimeState(
	cfg *ParsedConfig,
	configFilePath string,
	configFingerprint string,
) {
	if cfg == nil {
		return
	}

	cfg.ResolvedConfigFilePath = strings.TrimSpace(configFilePath)
	cfg.ResolvedConfigFingerprint = strings.TrimSpace(configFingerprint)
}

func normalizeResolvedConfigPathForMatch(path string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return ""
	}

	absolutePath, err := filepath.Abs(trimmedPath)
	if err == nil {
		trimmedPath = absolutePath
	}

	return filepath.Clean(trimmedPath)
}

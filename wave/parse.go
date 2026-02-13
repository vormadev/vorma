package wave

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

// ParseConfig parses Wave config JSON bytes into a ParsedConfig.
// This performs minimal validation to prevent nil pointer panics during parsing.
// Full validation of required fields should be done at build time via tooling.ValidateConfig.
func ParseConfig(data []byte) (*ParsedConfig, error) {
	var cfg ParsedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Minimal safety check: Core must exist to access Core.DistDir below
	if cfg.Core == nil {
		return nil, fmt.Errorf("config: Core section is required")
	}

	cfg.Dist = DistLayout{Root: filepath.Clean(cfg.Core.DistDir)}

	return &cfg, nil
}

// ParseConfigFile reads and parses a config file.
func ParseConfigFile(path string) (*ParsedConfig, error) {
	configFilePath := strings.TrimSpace(path)
	if configFilePath == "" {
		return nil, fmt.Errorf("config file path is required")
	}

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg, err := ParseConfig(data)
	if err != nil {
		return nil, err
	}

	cfg.Core.ConfigLocation = pathnorm.Absolute(configFilePath)

	return cfg, nil
}

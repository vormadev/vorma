// Package wave2 is the app-facing runtime API for Wave2 applications.
//
// This package intentionally excludes build/dev orchestration concerns so
// production applications keep a lean runtime dependency surface.
package wave2

import (
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
)

// Config configures Wave2 runtime construction.
type Config struct {
	// Required filesystem root used for config discovery/read operations.
	FS fs.FS

	// Required config path relative to Config.FS root.
	ConfigPath string

	// Optional logger used by runtime consumers.
	Logger *slog.Logger
}

// Wave is the app-facing runtime surface.
type Wave struct {
	configFS   fs.FS
	configPath string
	logger     *slog.Logger
}

// New constructs one Wave2 runtime instance.
func New(config Config) *Wave {
	if config.FS == nil {
		panic("wave2.New: FS is required")
	}
	normalizedConfigPath, normalizeConfigPathError := normalizeConfigPathForFS(
		config.ConfigPath,
	)
	if normalizeConfigPathError != nil {
		panic("wave2.New: " + normalizeConfigPathError.Error())
	}
	return &Wave{
		configFS:   config.FS,
		configPath: normalizedConfigPath,
		logger:     config.Logger,
	}
}

// ConfigFS returns the configured filesystem root used by this runtime.
func (waveRuntime *Wave) ConfigFS() fs.FS {
	if waveRuntime == nil {
		return nil
	}
	return waveRuntime.configFS
}

// ConfigFile returns the normalized config path relative to ConfigFS root.
func (waveRuntime *Wave) ConfigFile() string {
	if waveRuntime == nil {
		return ""
	}
	return waveRuntime.configPath
}

// Logger returns the configured logger.
func (waveRuntime *Wave) Logger() *slog.Logger {
	if waveRuntime == nil {
		return nil
	}
	return waveRuntime.logger
}

// SetLogger replaces the runtime logger.
func (waveRuntime *Wave) SetLogger(logger *slog.Logger) {
	if waveRuntime == nil {
		return
	}
	waveRuntime.logger = logger
}

func normalizeConfigPathForFS(configPath string) (string, error) {
	trimmedConfigPath := strings.TrimSpace(configPath)
	if trimmedConfigPath == "" {
		return "", fmt.Errorf("ConfigPath is required")
	}
	normalizedConfigPath := path.Clean(filepath.ToSlash(trimmedConfigPath))
	if normalizedConfigPath == "." {
		return "", fmt.Errorf("ConfigPath is required")
	}
	if strings.HasPrefix(normalizedConfigPath, "/") {
		return "", fmt.Errorf(
			"ConfigPath must be relative to FS root: %q",
			configPath,
		)
	}
	if normalizedConfigPath == ".." ||
		strings.HasPrefix(normalizedConfigPath, "../") {
		return "", fmt.Errorf(
			"ConfigPath must not escape FS root: %q",
			configPath,
		)
	}
	return normalizedConfigPath, nil
}

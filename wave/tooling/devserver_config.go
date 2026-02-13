package tooling

import (
	"fmt"
	"os"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

func (s *server) initWatcher() error {
	watcher, err := NewWatcher(s.cfg, s.log)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	s.mu.Lock()
	s.watcher = watcher
	s.mu.Unlock()

	if err := watcher.AddDir(s.cfg.WatchRoot()); err != nil {
		return fmt.Errorf("watch root: %w", err)
	}

	if err := s.addConfigFileDirectory(watcher, s.cfg.Core.ConfigLocation); err != nil {
		return fmt.Errorf("watch config file directory: %w", err)
	}

	return nil
}

func (s *server) addConfigFileDirectory(
	watcher *Watcher,
	configFilePath string,
) error {
	normalizedConfigDirectoryPath := pathnorm.AbsoluteDirectory(configFilePath)
	if normalizedConfigDirectoryPath == "" {
		return nil
	}

	if err := watcher.AddDir(normalizedConfigDirectoryPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (s *server) reloadConfig() error {
	newCfg, err := s.loadParsedConfigForReload()
	if err != nil {
		return err
	}
	if newCfg == nil {
		return nil
	}

	// Validate the new config
	if err := ValidateConfig(newCfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Preserve framework-injected runtime configuration.
	// These are set by frameworks (like Vorma) via AddFrameworkWatchPatterns()
	// and are not persisted in the base config payload.
	newCfg.CopyFrameworkRuntimeFieldsFrom(s.cfg)

	s.cfg = newCfg
	return nil
}

func (s *server) loadParsedConfigForReload() (*wave.ParsedConfig, error) {
	configFilePath := pathnorm.Absolute(s.cfg.Core.ConfigLocation)
	if configFilePath == "" {
		return nil, nil
	}

	s.log.Info("Reloading config", "path", configFilePath)
	newCfg, err := wave.ParseConfigFile(configFilePath)
	if err != nil {
		return nil, err
	}

	return newCfg, nil
}

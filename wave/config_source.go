package wave

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type LoadedConfig struct {
	ConfigJSON   []byte
	Dependencies ConfigProviderDependencies
	Fingerprint  string
}

type ConfigSource interface {
	LoadConfig(context.Context) (*LoadedConfig, error)
}

type StaticConfigSource struct {
	ConfigJSON    []byte
	Dependencies  ConfigProviderDependencies
	Fingerprint   string
	resolveSource func() ([]byte, error)
}

func NewStaticConfigSource(configJSON []byte) *StaticConfigSource {
	return &StaticConfigSource{
		ConfigJSON: append([]byte(nil), configJSON...),
	}
}

func (source *StaticConfigSource) LoadConfig(ctx context.Context) (*LoadedConfig, error) {
	_ = ctx

	if source == nil {
		return nil, fmt.Errorf("static config source is nil")
	}

	configJSON := source.ConfigJSON
	resolveSource := source.resolveSource
	if resolveSource != nil {
		var err error
		configJSON, err = resolveSource()
		if err != nil {
			return nil, fmt.Errorf("load static config source: %w", err)
		}
	}

	if len(configJSON) == 0 {
		return nil, fmt.Errorf("static config source config JSON is required")
	}

	return &LoadedConfig{
		ConfigJSON:   append([]byte(nil), configJSON...),
		Dependencies: source.Dependencies,
		Fingerprint:  strings.TrimSpace(source.Fingerprint),
	}, nil
}

type ProviderConfigSource struct {
	Invocation ConfigProviderInvocation
	Runner     *ConfigProviderRunner
}

func NewProviderConfigSource(
	invocation ConfigProviderInvocation,
) *ProviderConfigSource {
	return &ProviderConfigSource{
		Invocation: invocation,
		Runner:     NewConfigProviderRunner(),
	}
}

func (source *ProviderConfigSource) LoadConfig(ctx context.Context) (*LoadedConfig, error) {
	if source == nil {
		return nil, fmt.Errorf("provider config source is nil")
	}

	runner := source.Runner
	if runner == nil {
		runner = NewConfigProviderRunner()
	}

	payload, err := runner.Run(ctx, source.Invocation)
	if err != nil {
		return nil, fmt.Errorf("run provider config source: %w", err)
	}

	configDependencies := payload.Dependencies
	if source.Invocation.WorkingDir != "" {
		configDependencies = resolveConfigDependenciesRelativeToWorkingDir(
			configDependencies,
			source.Invocation.WorkingDir,
		)
	}

	return &LoadedConfig{
		ConfigJSON:   append([]byte(nil), payload.Config...),
		Dependencies: configDependencies,
		Fingerprint:  payload.Fingerprint,
	}, nil
}

func resolveConfigDependenciesRelativeToWorkingDir(
	configDependencies ConfigProviderDependencies,
	workingDir string,
) ConfigProviderDependencies {
	resolvePathValues := func(values []string) []string {
		if len(values) == 0 {
			return nil
		}

		resolvedValues := make([]string, 0, len(values))
		for _, value := range values {
			trimmedValue := strings.TrimSpace(value)
			if trimmedValue == "" {
				continue
			}

			if !filepath.IsAbs(trimmedValue) {
				trimmedValue = filepath.Join(workingDir, trimmedValue)
			}

			resolvedValues = append(resolvedValues, filepath.ToSlash(filepath.Clean(trimmedValue)))
		}

		return normalizeProviderDependencyValues(resolvedValues)
	}

	return ConfigProviderDependencies{
		Files: resolvePathValues(configDependencies.Files),
		Globs: resolvePathValues(configDependencies.Globs),
		Env:   normalizeProviderDependencyValues(configDependencies.Env),
	}
}

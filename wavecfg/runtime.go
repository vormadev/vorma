package wavecfg

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/wave"
)

func NewWave(configDocument *Document) *wave.Wave {
	return wave.New(NewWaveConfig(configDocument))
}

func NewWaveConfig(configDocument *Document) wave.Config {
	if configDocument == nil {
		panic("config document is required")
	}

	if strings.TrimSpace(configDocument.Core.DistDir) == "" {
		panic("config document core dist dir is required")
	}

	distStaticDirectory := filepath.Join(configDocument.Core.DistDir, "static")
	return wave.Config{
		ConfigSource: NewStaticConfigSourceFromDocument(configDocument),
		DistStaticFS: os.DirFS(distStaticDirectory),
	}
}

func ConfigDependenciesForGoConfigPackage(
	configPackagePath string,
) wave.ConfigProviderDependencies {
	normalizedConfigPackagePath := filepathLike(configPackagePath)
	if filepath.IsAbs(filepath.FromSlash(normalizedConfigPackagePath)) {
		panic("config package path must be relative (for example: backend/config)")
	}

	trimmedConfigPackagePath := strings.Trim(normalizedConfigPackagePath, "/")
	trimmedConfigPackagePath = strings.TrimPrefix(trimmedConfigPackagePath, "./")
	if trimmedConfigPackagePath == "" {
		panic("config package path is required")
	}

	return wave.ConfigProviderDependencies{
		Globs: []string{
			trimmedConfigPackagePath + "/**/*.go",
		},
	}
}

func filepathLike(path string) string {
	return strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
}

func NewStaticConfigSourceFromDocument(
	configDocument *Document,
) *wave.StaticConfigSource {
	staticConfigSource := wave.NewStaticConfigSource(
		configDocument.MustMarshalConfigJSON(),
	)
	staticConfigSource.Dependencies = configDocument.ConfigDependencies
	return staticConfigSource
}

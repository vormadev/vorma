package wavecfg

import (
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/wave"
)

func NewGoRunProviderConfigSource(
	providerCommandPackagePath string,
) *wave.ProviderConfigSource {
	return wave.NewProviderConfigSource(
		GoRunProviderInvocation(providerCommandPackagePath),
	)
}

func GoRunProviderInvocation(
	providerCommandPackagePath string,
) wave.ConfigProviderInvocation {
	normalizedProviderPackagePath := normalizeProviderCommandPackagePath(providerCommandPackagePath)
	return wave.ConfigProviderInvocation{
		Command: "go",
		Args: []string{
			"run",
			normalizedProviderPackagePath,
		},
	}
}

func normalizeProviderCommandPackagePath(
	providerCommandPackagePath string,
) string {
	normalizedProviderCommandPackagePath := filepath.ToSlash(
		strings.TrimSpace(providerCommandPackagePath),
	)
	if normalizedProviderCommandPackagePath == "" {
		panic("provider command package path is required")
	}

	if filepath.IsAbs(filepath.FromSlash(normalizedProviderCommandPackagePath)) {
		panic("provider command package path must be relative (for example: backend/cmd/config)")
	}

	if strings.HasPrefix(normalizedProviderCommandPackagePath, "../") {
		return normalizedProviderCommandPackagePath
	}

	return "./" + strings.TrimPrefix(normalizedProviderCommandPackagePath, "./")
}

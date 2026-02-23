package vormabuild

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

type buildDiagnosticsSnapshot struct {
	GeneratedAtUTC string `json:"generatedAtUTC"`

	ConfigFile       string `json:"configFile"`
	DistDir          string `json:"distDir"`
	StaticPrivateOut string `json:"staticPrivateOut"`
	StaticPublicOut  string `json:"staticPublicOut"`
	MainBuildEntry   string `json:"mainBuildEntry"`
	ClientEntry      string `json:"clientEntry"`
	UIVariant        string `json:"uiVariant"`
	TSGenOutDir      string `json:"tsGenOutDir"`
	IsDevMode        bool   `json:"isDevMode"`
	CurrentBuildID   string `json:"currentBuildID"`
	CurrentRoutes    int    `json:"currentRoutes"`
	CurrentManifest  string `json:"currentRouteManifest"`
	CurrentClientOut string `json:"currentClientEntryOut"`

	ClientRouteDefinitionPatterns []string `json:"clientRouteDefinitionPatterns"`
	ServerRouteDefinitionPatterns []string `json:"serverRouteDefinitionPatterns"`
	ResolvedClientRouteFiles      []string `json:"resolvedClientRouteFiles"`
	ResolvedServerRouteFiles      []string `json:"resolvedServerRouteFiles"`

	DiscoveredRegistrarCacheKey        string `json:"discoveredRegistrarCacheKey"`
	DiscoveredRegistrarCacheEntryCount int    `json:"discoveredRegistrarCacheEntryCount"`
	DiscoveredRegistrarCacheMaxEntries int    `json:"discoveredRegistrarCacheMaxEntries"`
}

type buildDiagnosticsDependencies struct {
	nowUTC                            func() time.Time
	resolveClientRouteDefinitionFiles func(*vormaruntime.Vorma) ([]string, error)
	resolveServerRouteDefinitionFiles func(*vormaruntime.Vorma) ([]string, error)
	discoveredRegistrarCacheKey       func(*vormaruntime.Vorma) string
	discoveredRegistrarCacheStats     func() (int, int)
	marshalBuildDiagnosticsJSON       func(any) ([]byte, error)
	writeBuildDiagnosticsOutput       func([]byte) error
}

type buildDiagnosticsExecutor struct {
	dependencies buildDiagnosticsDependencies
}

var defaultBuildDiagnosticsExecutor = newBuildDiagnosticsExecutor(
	buildDiagnosticsDependencies{},
)

func defaultBuildDiagnosticsDependencies() buildDiagnosticsDependencies {
	return buildDiagnosticsDependencies{
		nowUTC:                            time.Now().UTC,
		resolveClientRouteDefinitionFiles: resolveClientRouteDefinitionFiles,
		resolveServerRouteDefinitionFiles: resolveServerRouteDefinitionFiles,
		discoveredRegistrarCacheKey:       discoveredRouteRegistrarDiscoveryCacheKey,
		discoveredRegistrarCacheStats: func() (int, int) {
			return 0, discoveredRouteRegistrarArtifactCacheDefaultMaxEntries
		},
		marshalBuildDiagnosticsJSON: func(input any) ([]byte, error) {
			return json.MarshalIndent(input, "", "\t")
		},
		writeBuildDiagnosticsOutput: func(output []byte) error {
			if _, err := os.Stdout.Write(append(output, '\n')); err != nil {
				return err
			}
			return nil
		},
	}
}

func normalizeBuildDiagnosticsDependencies(
	dependencies buildDiagnosticsDependencies,
) buildDiagnosticsDependencies {
	defaultDependencies := defaultBuildDiagnosticsDependencies()
	if dependencies.nowUTC == nil {
		dependencies.nowUTC = defaultDependencies.nowUTC
	}
	if dependencies.resolveClientRouteDefinitionFiles == nil {
		dependencies.resolveClientRouteDefinitionFiles = defaultDependencies.resolveClientRouteDefinitionFiles
	}
	if dependencies.resolveServerRouteDefinitionFiles == nil {
		dependencies.resolveServerRouteDefinitionFiles = defaultDependencies.resolveServerRouteDefinitionFiles
	}
	if dependencies.discoveredRegistrarCacheKey == nil {
		dependencies.discoveredRegistrarCacheKey = defaultDependencies.discoveredRegistrarCacheKey
	}
	if dependencies.discoveredRegistrarCacheStats == nil {
		dependencies.discoveredRegistrarCacheStats = defaultDependencies.discoveredRegistrarCacheStats
	}
	if dependencies.marshalBuildDiagnosticsJSON == nil {
		dependencies.marshalBuildDiagnosticsJSON = defaultDependencies.marshalBuildDiagnosticsJSON
	}
	if dependencies.writeBuildDiagnosticsOutput == nil {
		dependencies.writeBuildDiagnosticsOutput = defaultDependencies.writeBuildDiagnosticsOutput
	}
	return dependencies
}

func newBuildDiagnosticsExecutor(
	dependencies buildDiagnosticsDependencies,
) buildDiagnosticsExecutor {
	return buildDiagnosticsExecutor{
		dependencies: normalizeBuildDiagnosticsDependencies(dependencies),
	}
}

func printBuildDiagnostics(v *vormaruntime.Vorma) error {
	return defaultBuildDiagnosticsExecutor.printBuildDiagnostics(v)
}

func (executor buildDiagnosticsExecutor) printBuildDiagnostics(
	v *vormaruntime.Vorma,
) error {
	diagnosticsSnapshot, err := executor.collectBuildDiagnostics(v)
	if err != nil {
		return err
	}

	diagnosticsJSON, err := executor.dependencies.marshalBuildDiagnosticsJSON(
		diagnosticsSnapshot,
	)
	if err != nil {
		return fmt.Errorf("marshal build diagnostics JSON: %w", err)
	}
	if err := executor.dependencies.writeBuildDiagnosticsOutput(diagnosticsJSON); err != nil {
		return fmt.Errorf("write build diagnostics output: %w", err)
	}
	return nil
}

func collectBuildDiagnostics(
	v *vormaruntime.Vorma,
) (*buildDiagnosticsSnapshot, error) {
	return defaultBuildDiagnosticsExecutor.collectBuildDiagnostics(v)
}

func (executor buildDiagnosticsExecutor) collectBuildDiagnostics(
	v *vormaruntime.Vorma,
) (*buildDiagnosticsSnapshot, error) {
	if v == nil {
		return nil, errors.New("vorma runtime is required")
	}
	if v.Config == nil {
		return nil, errors.New("vorma config is required")
	}

	resolvedClientRouteFiles, err := executor.dependencies.resolveClientRouteDefinitionFiles(
		v,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve client route definition files: %w", err)
	}

	resolvedServerRouteFiles, err := executor.dependencies.resolveServerRouteDefinitionFiles(
		v,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve server route definition files: %w", err)
	}
	normalizedClientRouteDefinitionPatterns, err := normalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ClientRouteDefinitionPatterns,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"normalize client route definition patterns: %w",
			err,
		)
	}
	normalizedServerRouteDefinitionPatterns, err := normalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ServerRouteDefinitionPatterns,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"normalize server route definition patterns: %w",
			err,
		)
	}

	cacheEntryCount, cacheMaxEntries := executor.dependencies.discoveredRegistrarCacheStats()
	currentRouteSnapshot := v.Paths()

	return &buildDiagnosticsSnapshot{
		GeneratedAtUTC: executor.dependencies.nowUTC().Format(time.RFC3339Nano),

		ConfigFile: filepath.ToSlash(filepath.Clean(v.Wave.ConfigFile())),
		DistDir:    filepath.ToSlash(filepath.Clean(v.Wave.DistDir())),
		StaticPrivateOut: filepath.ToSlash(
			filepath.Clean(v.Wave.StaticPrivateOutDir()),
		),
		StaticPublicOut: filepath.ToSlash(
			filepath.Clean(v.Wave.StaticPublicOutDir()),
		),
		MainBuildEntry:   v.Config.MainBuildEntry,
		ClientEntry:      v.Config.ClientEntry,
		UIVariant:        v.Config.UIVariant,
		TSGenOutDir:      v.Config.TSGenOutDir,
		IsDevMode:        v.IsDevMode(),
		CurrentBuildID:   v.BuildID(),
		CurrentRoutes:    len(currentRouteSnapshot),
		CurrentManifest:  v.RouteManifestFile(),
		CurrentClientOut: v.ClientEntryOut(),

		ClientRouteDefinitionPatterns: normalizedClientRouteDefinitionPatterns,
		ServerRouteDefinitionPatterns: normalizedServerRouteDefinitionPatterns,
		ResolvedClientRouteFiles:      resolvedClientRouteFiles,
		ResolvedServerRouteFiles:      resolvedServerRouteFiles,

		DiscoveredRegistrarCacheKey: executor.dependencies.discoveredRegistrarCacheKey(
			v,
		),
		DiscoveredRegistrarCacheEntryCount: cacheEntryCount,
		DiscoveredRegistrarCacheMaxEntries: cacheMaxEntries,
	}, nil
}

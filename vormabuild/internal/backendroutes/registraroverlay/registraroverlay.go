// Package registraroverlay owns discovered route registrar overlay artifacts and
// cache/fingerprint mechanics.
//
// This package isolates overlay-specific file generation and cache behavior so
// route discovery code can focus on semantic registration analysis.
package registraroverlay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/internal/routeparse"
)

// GeneratedFilename is the generated discovered registrar filename within each
// package.
const GeneratedFilename = "vorma_discovered_routes.gen.go"

// SourceArtifact describes one generated discovered registrar source artifact.
type SourceArtifact struct {
	TargetFilePath string
	SourceBytes    []byte
}

// DiscoveredRouteRegistrarOverlay points at a temporary go overlay config and
// cleanup function.
type DiscoveredRouteRegistrarOverlay struct {
	goOverlayConfigPath   string
	cleanupTemporaryFiles func() error
}

// NewDiscoveredRouteRegistrarOverlay creates an overlay value that can be
// returned from build-hook dependencies in tests or injected execution paths.
func NewDiscoveredRouteRegistrarOverlay(
	goOverlayConfigPath string,
	cleanupTemporaryFiles func() error,
) *DiscoveredRouteRegistrarOverlay {
	return &DiscoveredRouteRegistrarOverlay{
		goOverlayConfigPath:   goOverlayConfigPath,
		cleanupTemporaryFiles: cleanupTemporaryFiles,
	}
}

// GoOverlayConfigPath returns the go overlay config file path for this overlay.
func (overlay *DiscoveredRouteRegistrarOverlay) GoOverlayConfigPath() string {
	if overlay == nil {
		return ""
	}
	return overlay.goOverlayConfigPath
}

// Cleanup removes temporary overlay files created for discovered registrars.
func (overlay *DiscoveredRouteRegistrarOverlay) Cleanup() error {
	if overlay == nil || overlay.cleanupTemporaryFiles == nil {
		return nil
	}
	return overlay.cleanupTemporaryFiles()
}

type discoveredRouteRegistrarArtifactCacheEntry struct {
	sourceFingerprint string
	artifacts         []SourceArtifact
	lastAccessSeq     uint64
}

// DiscoveredRouteRegistrarArtifactCache memoizes discovered registrar artifacts
// by project key and source fingerprint.
type DiscoveredRouteRegistrarArtifactCache struct {
	mutex      sync.Mutex
	maxEntries int
	accessSeq  uint64
	entries    map[string]discoveredRouteRegistrarArtifactCacheEntry
}

const discoveredRouteRegistrarArtifactCacheDefaultMaxEntries = 128

// DiscoveredRouteRegistrarArtifactCacheDefaultMaxEntries is the default cache
// entry count for discovered registrar artifacts.
const DiscoveredRouteRegistrarArtifactCacheDefaultMaxEntries = discoveredRouteRegistrarArtifactCacheDefaultMaxEntries

func newDiscoveredRouteRegistrarArtifactCache(
	maxEntries int,
) *DiscoveredRouteRegistrarArtifactCache {
	if maxEntries <= 0 {
		maxEntries = discoveredRouteRegistrarArtifactCacheDefaultMaxEntries
	}

	return &DiscoveredRouteRegistrarArtifactCache{
		maxEntries: maxEntries,
		entries:    map[string]discoveredRouteRegistrarArtifactCacheEntry{},
	}
}

// NewDiscoveredRouteRegistrarArtifactCache returns a cache that memoizes
// discovered registrar source artifacts keyed by project identity and source
// fingerprint.
func NewDiscoveredRouteRegistrarArtifactCache(
	maxEntries int,
) *DiscoveredRouteRegistrarArtifactCache {
	return newDiscoveredRouteRegistrarArtifactCache(maxEntries)
}

// Get reads cached artifacts for cache key and source fingerprint. A stale
// fingerprint invalidates the entry and reports a miss.
func (cache *DiscoveredRouteRegistrarArtifactCache) Get(
	cacheKey string,
	sourceFingerprint string,
) ([]SourceArtifact, bool) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cacheEntry, hasCacheEntry := cache.entries[cacheKey]
	if !hasCacheEntry {
		return nil, false
	}
	if cacheEntry.sourceFingerprint != sourceFingerprint {
		delete(cache.entries, cacheKey)
		return nil, false
	}

	cache.accessSeq++
	cacheEntry.lastAccessSeq = cache.accessSeq
	cache.entries[cacheKey] = cacheEntry

	return cloneSourceArtifacts(cacheEntry.artifacts), true
}

// Set inserts/updates cached artifacts and evicts LRU entries over capacity.
func (cache *DiscoveredRouteRegistrarArtifactCache) Set(
	cacheKey string,
	sourceFingerprint string,
	artifacts []SourceArtifact,
) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cache.accessSeq++
	cache.entries[cacheKey] = discoveredRouteRegistrarArtifactCacheEntry{
		sourceFingerprint: sourceFingerprint,
		artifacts:         cloneSourceArtifacts(artifacts),
		lastAccessSeq:     cache.accessSeq,
	}
	cache.evictEntriesOverCapacityLocked()
}

func (cache *DiscoveredRouteRegistrarArtifactCache) evictEntriesOverCapacityLocked() {
	for cache.maxEntries > 0 && len(cache.entries) > cache.maxEntries {
		leastRecentlyUsedCacheKey := ""
		var leastRecentlyUsedAccessSeq uint64
		for candidateCacheKey, candidateCacheEntry := range cache.entries {
			if leastRecentlyUsedCacheKey == "" {
				leastRecentlyUsedCacheKey = candidateCacheKey
				leastRecentlyUsedAccessSeq = candidateCacheEntry.lastAccessSeq
				continue
			}

			if candidateCacheEntry.lastAccessSeq < leastRecentlyUsedAccessSeq {
				leastRecentlyUsedCacheKey = candidateCacheKey
				leastRecentlyUsedAccessSeq = candidateCacheEntry.lastAccessSeq
				continue
			}
			if candidateCacheEntry.lastAccessSeq == leastRecentlyUsedAccessSeq &&
				candidateCacheKey < leastRecentlyUsedCacheKey {
				leastRecentlyUsedCacheKey = candidateCacheKey
			}
		}
		delete(cache.entries, leastRecentlyUsedCacheKey)
	}
}

func cloneSourceArtifacts(
	artifacts []SourceArtifact,
) []SourceArtifact {
	if len(artifacts) == 0 {
		return nil
	}

	clonedArtifacts := make([]SourceArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		clonedArtifacts = append(
			clonedArtifacts,
			SourceArtifact{
				TargetFilePath: artifact.TargetFilePath,
				SourceBytes:    append([]byte(nil), artifact.SourceBytes...),
			},
		)
	}
	return clonedArtifacts
}

// DiscoveryCacheKey derives a stable cache key for discovered registrar
// artifacts from runtime identity and server route definition patterns.
func DiscoveryCacheKey(v *vormaruntime.Vorma) string {
	if v == nil || v.Wave == nil || v.Config == nil {
		return "<nil-vorma>"
	}

	normalizedServerRoutePatterns, err := routeparse.NormalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ServerRouteDefinitionPatterns(),
	)
	if err != nil {
		normalizedServerRoutePatterns = []string{
			"<invalid-server-route-patterns>",
		}
	}
	return strings.Join(
		[]string{
			filepath.ToSlash(filepath.Clean(v.Wave.ConfigFile())),
			filepath.ToSlash(filepath.Clean(v.Wave.DistDir())),
			filepath.ToSlash(filepath.Clean(v.Wave.StaticPrivateOutDir())),
			filepath.ToSlash(filepath.Clean(v.Wave.StaticPublicOutDir())),
			strings.TrimSpace(v.Config.MainBuildEntry()),
			strings.Join(normalizedServerRoutePatterns, ","),
		},
		"|",
	)
}

// FingerprintDependencies configures filesystem operations used by discovery
// fingerprint computation.
type FingerprintDependencies struct {
	ReadDir      func(string) ([]os.DirEntry, error)
	ReadFile     func(string) ([]byte, error)
	AbsolutePath func(string) (string, error)
}

func defaultFingerprintDependencies() FingerprintDependencies {
	return FingerprintDependencies{
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		AbsolutePath: filepath.Abs,
	}
}

func normalizeFingerprintDependencies(
	dependencies FingerprintDependencies,
) FingerprintDependencies {
	defaultDependencies := defaultFingerprintDependencies()
	if dependencies.ReadDir == nil {
		dependencies.ReadDir = defaultDependencies.ReadDir
	}
	if dependencies.ReadFile == nil {
		dependencies.ReadFile = defaultDependencies.ReadFile
	}
	if dependencies.AbsolutePath == nil {
		dependencies.AbsolutePath = defaultDependencies.AbsolutePath
	}
	return dependencies
}

// ComputeDiscoveredRouteRegistrarDiscoveryFingerprint computes a stable
// fingerprint for discovered registrar inputs.
func ComputeDiscoveredRouteRegistrarDiscoveryFingerprint(
	serverRouteDefinitionFiles []string,
) (string, error) {
	return ComputeDiscoveredRouteRegistrarDiscoveryFingerprintWithDependencies(
		serverRouteDefinitionFiles,
		FingerprintDependencies{},
	)
}

// ComputeDiscoveredRouteRegistrarDiscoveryFingerprintWithDependencies computes
// discovery fingerprint using custom dependencies.
func ComputeDiscoveredRouteRegistrarDiscoveryFingerprintWithDependencies(
	serverRouteDefinitionFiles []string,
	dependencies FingerprintDependencies,
) (string, error) {
	dependencies = normalizeFingerprintDependencies(dependencies)

	normalizedRouteDefinitionFiles := make(
		[]string,
		0,
		len(serverRouteDefinitionFiles),
	)
	packageDirSet := map[string]struct{}{}
	for _, serverRouteDefinitionFile := range serverRouteDefinitionFiles {
		absoluteRouteDefinitionFilePath, err := dependencies.AbsolutePath(
			serverRouteDefinitionFile,
		)
		if err != nil {
			return "", fmt.Errorf(
				"resolve absolute server route definition file %q for discovered route registrar cache fingerprint: %w",
				serverRouteDefinitionFile,
				err,
			)
		}

		normalizedRouteDefinitionFile := filepath.ToSlash(
			filepath.Clean(absoluteRouteDefinitionFilePath),
		)
		normalizedRouteDefinitionFiles = append(
			normalizedRouteDefinitionFiles,
			normalizedRouteDefinitionFile,
		)
		packageDirSet[filepath.ToSlash(filepath.Clean(filepath.Dir(normalizedRouteDefinitionFile)))] = struct{}{}
	}
	sort.Strings(normalizedRouteDefinitionFiles)

	packageDirs := make([]string, 0, len(packageDirSet))
	for packageDir := range packageDirSet {
		packageDirs = append(packageDirs, packageDir)
	}
	sort.Strings(packageDirs)

	discoveryFingerprintHasher := sha256.New()
	appendFingerprintSegment := func(value string) {
		_, _ = discoveryFingerprintHasher.Write([]byte(value))
		_, _ = discoveryFingerprintHasher.Write([]byte{0})
	}
	appendFingerprintBytes := func(value []byte) {
		_, _ = discoveryFingerprintHasher.Write(value)
		_, _ = discoveryFingerprintHasher.Write([]byte{0})
	}

	appendFingerprintSegment("server-route-definition-files")
	for _, routeDefinitionFile := range normalizedRouteDefinitionFiles {
		appendFingerprintSegment(routeDefinitionFile)
	}

	for _, packageDir := range packageDirs {
		appendFingerprintSegment("package-dir")
		appendFingerprintSegment(packageDir)

		packageDirEntries, err := dependencies.ReadDir(packageDir)
		if err != nil {
			return "", fmt.Errorf(
				"read package directory %q for discovered route registrar cache fingerprint: %w",
				packageDir,
				err,
			)
		}

		packageGoFiles := make([]string, 0, len(packageDirEntries))
		for _, packageDirEntry := range packageDirEntries {
			if packageDirEntry.IsDir() {
				continue
			}

			entryName := packageDirEntry.Name()
			if filepath.Ext(entryName) != ".go" ||
				strings.HasSuffix(entryName, "_test.go") {
				continue
			}

			packageGoFiles = append(packageGoFiles, entryName)
		}
		sort.Strings(packageGoFiles)

		for _, packageGoFile := range packageGoFiles {
			packageGoFilePath := filepath.ToSlash(
				filepath.Clean(filepath.Join(packageDir, packageGoFile)),
			)
			packageGoFileBytes, err := dependencies.ReadFile(
				packageGoFilePath,
			)
			if err != nil {
				return "", fmt.Errorf(
					"read package Go source file %q for discovered route registrar cache fingerprint: %w",
					packageGoFilePath,
					err,
				)
			}

			appendFingerprintSegment(packageGoFilePath)
			appendFingerprintBytes(packageGoFileBytes)
		}
	}

	return hex.EncodeToString(discoveryFingerprintHasher.Sum(nil)), nil
}

type goOverlayReplaceConfiguration struct {
	Replace map[string]string `json:"Replace"`
}

// OverlayWriteDependencies configures temporary overlay write operations.
type OverlayWriteDependencies struct {
	MakeTempDir func(string, string) (string, error)
	WriteFile   func(string, []byte, os.FileMode) error
	RemoveAll   func(string) error
	MarshalJSON func(any) ([]byte, error)
}

func defaultOverlayWriteDependencies() OverlayWriteDependencies {
	return OverlayWriteDependencies{
		MakeTempDir: os.MkdirTemp,
		WriteFile:   os.WriteFile,
		RemoveAll:   os.RemoveAll,
		MarshalJSON: json.Marshal,
	}
}

func normalizeOverlayWriteDependencies(
	dependencies OverlayWriteDependencies,
) OverlayWriteDependencies {
	defaultDependencies := defaultOverlayWriteDependencies()
	if dependencies.MakeTempDir == nil {
		dependencies.MakeTempDir = defaultDependencies.MakeTempDir
	}
	if dependencies.WriteFile == nil {
		dependencies.WriteFile = defaultDependencies.WriteFile
	}
	if dependencies.RemoveAll == nil {
		dependencies.RemoveAll = defaultDependencies.RemoveAll
	}
	if dependencies.MarshalJSON == nil {
		dependencies.MarshalJSON = defaultDependencies.MarshalJSON
	}
	return dependencies
}

// WriteDiscoveredRouteRegistrarOverlay writes source artifacts into temporary
// overlay files and returns a go overlay config.
func WriteDiscoveredRouteRegistrarOverlay(
	discoveredRegistrarArtifacts []SourceArtifact,
) (*DiscoveredRouteRegistrarOverlay, error) {
	return WriteDiscoveredRouteRegistrarOverlayWithDependencies(
		discoveredRegistrarArtifacts,
		OverlayWriteDependencies{},
	)
}

// WriteDiscoveredRouteRegistrarOverlayWithDependencies writes overlay artifacts
// using supplied dependencies.
func WriteDiscoveredRouteRegistrarOverlayWithDependencies(
	discoveredRegistrarArtifacts []SourceArtifact,
	dependencies OverlayWriteDependencies,
) (*DiscoveredRouteRegistrarOverlay, error) {
	dependencies = normalizeOverlayWriteDependencies(dependencies)

	if len(discoveredRegistrarArtifacts) == 0 {
		return nil, nil
	}

	overlayTempDir, err := dependencies.MakeTempDir(
		"",
		"vorma_discovered_route_registrars_*",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create temporary discovered route registrar overlay directory: %w",
			err,
		)
	}
	cleanupOverlayTempDir := func() error {
		return dependencies.RemoveAll(overlayTempDir)
	}

	overlayReplacements := map[string]string{}
	for artifactIndex, discoveredArtifact := range discoveredRegistrarArtifacts {
		overlaySourceFilePath := filepath.ToSlash(filepath.Join(
			overlayTempDir,
			fmt.Sprintf("discovered_route_registrar_%d.gen.go", artifactIndex),
		))
		if err := dependencies.WriteFile(
			overlaySourceFilePath,
			discoveredArtifact.SourceBytes,
			0o644,
		); err != nil {
			if cleanupErr := cleanupOverlayTempDir(); cleanupErr != nil {
				return nil, fmt.Errorf(
					"write discovered route registrar overlay source %q: %w (cleanup overlay dir failed: %v)",
					overlaySourceFilePath,
					err,
					cleanupErr,
				)
			}
			return nil, fmt.Errorf(
				"write discovered route registrar overlay source %q: %w",
				overlaySourceFilePath,
				err,
			)
		}
		overlayReplacements[discoveredArtifact.TargetFilePath] = overlaySourceFilePath
	}

	overlayConfigBytes, err := dependencies.MarshalJSON(
		goOverlayReplaceConfiguration{Replace: overlayReplacements},
	)
	if err != nil {
		if cleanupErr := cleanupOverlayTempDir(); cleanupErr != nil {
			return nil, fmt.Errorf(
				"marshal discovered route registrar go overlay config: %w (cleanup overlay dir failed: %v)",
				err,
				cleanupErr,
			)
		}
		return nil, fmt.Errorf(
			"marshal discovered route registrar go overlay config: %w",
			err,
		)
	}

	overlayConfigPath := filepath.ToSlash(filepath.Join(
		overlayTempDir,
		"go-overlay.json",
	))
	if err := dependencies.WriteFile(
		overlayConfigPath,
		overlayConfigBytes,
		0o644,
	); err != nil {
		if cleanupErr := cleanupOverlayTempDir(); cleanupErr != nil {
			return nil, fmt.Errorf(
				"write discovered route registrar go overlay config %q: %w (cleanup overlay dir failed: %v)",
				overlayConfigPath,
				err,
				cleanupErr,
			)
		}
		return nil, fmt.Errorf(
			"write discovered route registrar go overlay config %q: %w",
			overlayConfigPath,
			err,
		)
	}

	return &DiscoveredRouteRegistrarOverlay{
		goOverlayConfigPath: overlayConfigPath,
		cleanupTemporaryFiles: func() error {
			return cleanupOverlayTempDir()
		},
	}, nil
}

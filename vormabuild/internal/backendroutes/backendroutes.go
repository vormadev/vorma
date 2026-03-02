// Package backendroutes owns backend route discovery and discovered registrar
// overlay generation for vormabuild.
//
// It keeps top-level orchestration for file discovery and go-overlay assembly,
// while delegating AST/go-types registration analysis to dedicated subpackages.
package backendroutes

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/internal/backendroutes/registraroverlay"
	"github.com/vormadev/vorma/vormabuild/internal/backendroutes/registrationgraph"
	"github.com/vormadev/vorma/vormabuild/internal/backendroutes/sourceparse"
)

type backendRouteDiscoveryDependencies struct {
	expandPattern      func(string) ([]string, error)
	statPath           func(string) (fs.FileInfo, error)
	readFile           func(string) ([]byte, error)
	parseGoSourceAST   func(*token.FileSet, string, any, parser.Mode) (*ast.File, error)
	importGoPackageDir func(string) (*build.Package, error)
}

type backendRouteDiscoveryExecutor struct {
	dependencies backendRouteDiscoveryDependencies
}

var defaultBackendRouteDiscoveryExecutor = newBackendRouteDiscoveryExecutor(
	backendRouteDiscoveryDependencies{},
)

func defaultBackendRouteDiscoveryDependencies() backendRouteDiscoveryDependencies {
	defaultDependencies := sourceparse.DefaultDependencies()
	return backendRouteDiscoveryDependencies{
		expandPattern:      defaultDependencies.ExpandPattern,
		statPath:           defaultDependencies.StatPath,
		readFile:           defaultDependencies.ReadFile,
		parseGoSourceAST:   defaultDependencies.ParseGoSourceAST,
		importGoPackageDir: defaultDependencies.ImportGoPackageDir,
	}
}

func normalizeBackendRouteDiscoveryDependencies(
	dependencies backendRouteDiscoveryDependencies,
) backendRouteDiscoveryDependencies {
	defaultDependencies := defaultBackendRouteDiscoveryDependencies()

	if dependencies.expandPattern == nil {
		dependencies.expandPattern = defaultDependencies.expandPattern
	}
	if dependencies.statPath == nil {
		dependencies.statPath = defaultDependencies.statPath
	}
	if dependencies.readFile == nil {
		dependencies.readFile = defaultDependencies.readFile
	}
	if dependencies.parseGoSourceAST == nil {
		dependencies.parseGoSourceAST = defaultDependencies.parseGoSourceAST
	}
	if dependencies.importGoPackageDir == nil {
		dependencies.importGoPackageDir = defaultDependencies.importGoPackageDir
	}

	return dependencies
}

func sourceParseDependenciesFromBackendRouteDiscoveryDependencies(
	dependencies backendRouteDiscoveryDependencies,
) sourceparse.Dependencies {
	normalizedDependencies := normalizeBackendRouteDiscoveryDependencies(
		dependencies,
	)
	return sourceparse.Dependencies{
		ExpandPattern:      normalizedDependencies.expandPattern,
		StatPath:           normalizedDependencies.statPath,
		ReadFile:           normalizedDependencies.readFile,
		ParseGoSourceAST:   normalizedDependencies.parseGoSourceAST,
		ImportGoPackageDir: normalizedDependencies.importGoPackageDir,
	}
}

func newBackendRouteDiscoveryExecutor(
	dependencies backendRouteDiscoveryDependencies,
) backendRouteDiscoveryExecutor {
	return backendRouteDiscoveryExecutor{
		dependencies: normalizeBackendRouteDiscoveryDependencies(dependencies),
	}
}

func parseBackendLoaderPatterns(v *vormaruntime.Vorma) ([]string, error) {
	return defaultBackendRouteDiscoveryExecutor.parseBackendLoaderPatterns(v)
}

// ParseBackendLoaderPatterns discovers backend loader route patterns by parsing
// configured server route definition files and following init-reachable
// registration calls.
func ParseBackendLoaderPatterns(v *vormaruntime.Vorma) ([]string, error) {
	return parseBackendLoaderPatterns(v)
}

func (executor backendRouteDiscoveryExecutor) parseBackendLoaderPatterns(
	v *vormaruntime.Vorma,
) ([]string, error) {
	serverRouteDefinitionFiles, err := executor.resolveServerRouteDefinitionFiles(
		v,
	)
	if err != nil {
		return nil, err
	}
	if len(serverRouteDefinitionFiles) == 0 {
		return nil, nil
	}

	packageAnalyses, err := executor.parseServerRouteFilesIntoPackageAnalyses(
		serverRouteDefinitionFiles,
	)
	if err != nil {
		return nil, err
	}

	loaderPatternSet := map[string]struct{}{}
	for _, packageAnalysis := range packageAnalyses {
		packageLoaderPatterns, err := packageAnalysis.DiscoverLoaderPatterns()
		if err != nil {
			return nil, err
		}
		for _, loaderPattern := range packageLoaderPatterns {
			loaderPatternSet[loaderPattern] = struct{}{}
		}
	}

	loaderPatterns := make([]string, 0, len(loaderPatternSet))
	for loaderPattern := range loaderPatternSet {
		loaderPatterns = append(loaderPatterns, loaderPattern)
	}
	sort.Strings(loaderPatterns)
	return loaderPatterns, nil
}

func (executor backendRouteDiscoveryExecutor) parseServerRouteFilesIntoPackageAnalyses(
	serverRouteDefinitionFiles []string,
) ([]*registrationgraph.PackageAnalysis, error) {
	return registrationgraph.ParseServerRouteFilesIntoPackageAnalyses(
		serverRouteDefinitionFiles,
		sourceParseDependenciesFromBackendRouteDiscoveryDependencies(
			executor.dependencies,
		),
	)
}

func resolveServerRouteDefinitionFiles(
	v *vormaruntime.Vorma,
) ([]string, error) {
	return defaultBackendRouteDiscoveryExecutor.resolveServerRouteDefinitionFiles(
		v,
	)
}

func (executor backendRouteDiscoveryExecutor) resolveServerRouteDefinitionFiles(
	v *vormaruntime.Vorma,
) ([]string, error) {
	return sourceparse.ResolveServerRouteDefinitionFiles(
		v,
		sourceParseDependenciesFromBackendRouteDiscoveryDependencies(
			executor.dependencies,
		),
	)
}

type backendRouteRegistrarOverlayDependencies struct {
	computeDiscoveredRouteRegistrarDiscoveryFingerprint func([]string) (string, error)
	writeDiscoveredRouteRegistrarOverlay                func([]registraroverlay.SourceArtifact) (*registraroverlay.DiscoveredRouteRegistrarOverlay, error)
	absolutePath                                        func(string) (string, error)
}

type backendRouteRegistrarOverlayExecutor struct {
	dependencies           backendRouteRegistrarOverlayDependencies
	routeDiscoveryExecutor backendRouteDiscoveryExecutor
}

var defaultBackendRouteRegistrarOverlayExecutor = newBackendRouteRegistrarOverlayExecutor(
	backendRouteRegistrarOverlayDependencies{},
	backendRouteDiscoveryExecutor{},
)

func defaultBackendRouteRegistrarOverlayDependencies() backendRouteRegistrarOverlayDependencies {
	return backendRouteRegistrarOverlayDependencies{
		computeDiscoveredRouteRegistrarDiscoveryFingerprint: registraroverlay.ComputeDiscoveredRouteRegistrarDiscoveryFingerprint,
		writeDiscoveredRouteRegistrarOverlay:                registraroverlay.WriteDiscoveredRouteRegistrarOverlay,
		absolutePath:                                        filepath.Abs,
	}
}

func normalizeBackendRouteRegistrarOverlayDependencies(
	dependencies backendRouteRegistrarOverlayDependencies,
) backendRouteRegistrarOverlayDependencies {
	defaultDependencies := defaultBackendRouteRegistrarOverlayDependencies()
	if dependencies.computeDiscoveredRouteRegistrarDiscoveryFingerprint == nil {
		dependencies.computeDiscoveredRouteRegistrarDiscoveryFingerprint = defaultDependencies.computeDiscoveredRouteRegistrarDiscoveryFingerprint
	}
	if dependencies.writeDiscoveredRouteRegistrarOverlay == nil {
		dependencies.writeDiscoveredRouteRegistrarOverlay = defaultDependencies.writeDiscoveredRouteRegistrarOverlay
	}
	if dependencies.absolutePath == nil {
		dependencies.absolutePath = defaultDependencies.absolutePath
	}
	return dependencies
}

func newBackendRouteRegistrarOverlayExecutor(
	dependencies backendRouteRegistrarOverlayDependencies,
	routeDiscoveryExecutor backendRouteDiscoveryExecutor,
) backendRouteRegistrarOverlayExecutor {
	if routeDiscoveryExecutor.dependencies.expandPattern == nil {
		routeDiscoveryExecutor = defaultBackendRouteDiscoveryExecutor
	}
	return backendRouteRegistrarOverlayExecutor{
		dependencies: normalizeBackendRouteRegistrarOverlayDependencies(
			dependencies,
		),
		routeDiscoveryExecutor: routeDiscoveryExecutor,
	}
}

func prepareDiscoveredRouteRegistrarOverlay(
	v *vormaruntime.Vorma,
) (*registraroverlay.DiscoveredRouteRegistrarOverlay, error) {
	return defaultBackendRouteRegistrarOverlayExecutor.prepareDiscoveredRouteRegistrarOverlay(
		v,
	)
}

func prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
	v *vormaruntime.Vorma,
	discoveredRouteRegistrarArtifactsCache *registraroverlay.DiscoveredRouteRegistrarArtifactCache,
) (*registraroverlay.DiscoveredRouteRegistrarOverlay, error) {
	return defaultBackendRouteRegistrarOverlayExecutor.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
		v,
		discoveredRouteRegistrarArtifactsCache,
	)
}

// PrepareDiscoveredRouteRegistrarOverlayWithArtifactCache builds a go overlay
// that rewrites discovered registrar source files for the provided runtime.
func PrepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
	v *vormaruntime.Vorma,
	discoveredRouteRegistrarArtifactsCache *registraroverlay.DiscoveredRouteRegistrarArtifactCache,
) (*registraroverlay.DiscoveredRouteRegistrarOverlay, error) {
	return prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
		v,
		discoveredRouteRegistrarArtifactsCache,
	)
}

func prepareDiscoveredRouteRegistrarOverlayWithArtifactCacheAndDiscoveryDependencies(
	v *vormaruntime.Vorma,
	discoveredRouteRegistrarArtifactsCache *registraroverlay.DiscoveredRouteRegistrarArtifactCache,
	discoveryDependencies backendRouteDiscoveryDependencies,
) (*registraroverlay.DiscoveredRouteRegistrarOverlay, error) {
	overlayExecutor := newBackendRouteRegistrarOverlayExecutor(
		backendRouteRegistrarOverlayDependencies{},
		newBackendRouteDiscoveryExecutor(discoveryDependencies),
	)
	return overlayExecutor.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
		v,
		discoveredRouteRegistrarArtifactsCache,
	)
}

func (executor backendRouteRegistrarOverlayExecutor) prepareDiscoveredRouteRegistrarOverlay(
	v *vormaruntime.Vorma,
) (*registraroverlay.DiscoveredRouteRegistrarOverlay, error) {
	return executor.prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
		v,
		registraroverlay.NewDiscoveredRouteRegistrarArtifactCache(
			registraroverlay.DiscoveredRouteRegistrarArtifactCacheDefaultMaxEntries,
		),
	)
}

func (executor backendRouteRegistrarOverlayExecutor) prepareDiscoveredRouteRegistrarOverlayWithArtifactCache(
	v *vormaruntime.Vorma,
	discoveredRouteRegistrarArtifactsCache *registraroverlay.DiscoveredRouteRegistrarArtifactCache,
) (*registraroverlay.DiscoveredRouteRegistrarOverlay, error) {
	if discoveredRouteRegistrarArtifactsCache == nil {
		discoveredRouteRegistrarArtifactsCache = registraroverlay.NewDiscoveredRouteRegistrarArtifactCache(
			registraroverlay.DiscoveredRouteRegistrarArtifactCacheDefaultMaxEntries,
		)
	}

	serverRouteDefinitionFiles, err := executor.routeDiscoveryExecutor.resolveServerRouteDefinitionFiles(
		v,
	)
	if err != nil {
		return nil, err
	}
	if len(serverRouteDefinitionFiles) == 0 {
		return nil, nil
	}

	discoverySourceFingerprint, err := executor.dependencies.computeDiscoveredRouteRegistrarDiscoveryFingerprint(
		serverRouteDefinitionFiles,
	)
	if err != nil {
		return nil, err
	}

	discoveryCacheKey := registraroverlay.DiscoveryCacheKey(v)
	discoveredRegistrarArtifacts, hasCachedArtifacts := discoveredRouteRegistrarArtifactsCache.Get(
		discoveryCacheKey,
		discoverySourceFingerprint,
	)
	if !hasCachedArtifacts {
		discoveredRegistrarArtifacts, err = executor.discoverRouteRegistrarSourceArtifacts(
			serverRouteDefinitionFiles,
		)
		if err != nil {
			return nil, err
		}
		discoveredRouteRegistrarArtifactsCache.Set(
			discoveryCacheKey,
			discoverySourceFingerprint,
			discoveredRegistrarArtifacts,
		)
	}

	return executor.dependencies.writeDiscoveredRouteRegistrarOverlay(
		discoveredRegistrarArtifacts,
	)
}

func (executor backendRouteRegistrarOverlayExecutor) discoverRouteRegistrarSourceArtifacts(
	serverRouteDefinitionFiles []string,
) ([]registraroverlay.SourceArtifact, error) {
	packageAnalyses, err := executor.routeDiscoveryExecutor.parseServerRouteFilesIntoPackageAnalyses(
		serverRouteDefinitionFiles,
	)
	if err != nil {
		return nil, err
	}

	discoveredRegistrarArtifacts := make([]registraroverlay.SourceArtifact, 0)
	for _, packageAnalysis := range packageAnalyses {
		routeRegistrarSource, err := packageAnalysis.DiscoverRouteRegistrarSource()
		if err != nil {
			return nil, err
		}
		if routeRegistrarSource == nil {
			continue
		}

		generatedFilePath := filepath.Join(
			routeRegistrarSource.PackageDir,
			registraroverlay.GeneratedFilename,
		)
		absoluteGeneratedFilePath, err := executor.dependencies.absolutePath(
			generatedFilePath,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"resolve absolute generated backend route registrar path %q: %w",
				generatedFilePath,
				err,
			)
		}

		discoveredRegistrarArtifacts = append(
			discoveredRegistrarArtifacts,
			registraroverlay.SourceArtifact{
				TargetFilePath: filepath.ToSlash(
					filepath.Clean(absoluteGeneratedFilePath),
				),
				SourceBytes: routeRegistrarSource.SourceBytes,
			},
		)
	}
	sort.Slice(discoveredRegistrarArtifacts, func(i int, j int) bool {
		return discoveredRegistrarArtifacts[i].TargetFilePath < discoveredRegistrarArtifacts[j].TargetFilePath
	})
	return discoveredRegistrarArtifacts, nil
}

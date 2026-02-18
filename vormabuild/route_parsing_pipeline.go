package vormabuild

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/internal/vormaruntime"
)

var importRegex = regexp.MustCompile(`import\((` + "`" + `[^` + "`" + `]+` + "`" + `|'[^']+'|"[^"]+")\)`)

type routeParsingPipelineDependencies struct {
	resolveClientRouteDefinitionFiles func(*vormaruntime.Vorma) ([]string, error)
	parseRouteDefinitionFileIntoCalls func(*vormaruntime.Vorma, string) (parsedRouteDefinitionsCode, error)
	handleUnresolvedRouteCalls        func(*vormaruntime.Vorma, string, []unresolvedRouteCall) error
	mergeRouteCallsIntoPaths          func(*vormaruntime.Vorma, map[string]*vormaruntime.Path, string, []routeCall) error
}

type routeDefinitionsCodeParsingDependencies struct {
	transformRouteDefinitionsCode        func(*vormaruntime.Vorma, []byte) (string, error)
	extractRouteCallsFromTransformedCode func(string) ([]routeCall, []unresolvedRouteCall, error)
}

type routeModuleResolutionDependencies struct {
	computeRelativeModulePath func(string, string) (string, error)
	statRouteModulePath       func(string) (fs.FileInfo, error)
}

type routeDefinitionsFileResolutionDependencies struct {
	expandRouteDefinitionPattern func(string) ([]string, error)
	statRouteDefinitionPath      func(string) (fs.FileInfo, error)
}

type routeParsingExecutorDependencies struct {
	routeParsingPipelineDependencies           routeParsingPipelineDependencies
	routeDefinitionsCodeParsingDependencies    routeDefinitionsCodeParsingDependencies
	routeModuleResolutionDependencies          routeModuleResolutionDependencies
	routeDefinitionsFileResolutionDependencies routeDefinitionsFileResolutionDependencies
	readRouteDefinitionFile                    func(string) ([]byte, error)
}

type routeParsingExecutor struct {
	dependencies routeParsingExecutorDependencies
}

var defaultRouteParsingExecutor = newRouteParsingExecutor(
	routeParsingExecutorDependencies{},
)

func defaultRouteParsingExecutorDependencies() routeParsingExecutorDependencies {
	return routeParsingExecutorDependencies{
		routeDefinitionsCodeParsingDependencies: routeDefinitionsCodeParsingDependencies{
			transformRouteDefinitionsCode:        transformRouteDefinitionsCode,
			extractRouteCallsFromTransformedCode: extractRouteCalls,
		},
		routeModuleResolutionDependencies: routeModuleResolutionDependencies{
			computeRelativeModulePath: filepath.Rel,
			statRouteModulePath:       os.Stat,
		},
		routeDefinitionsFileResolutionDependencies: routeDefinitionsFileResolutionDependencies{
			expandRouteDefinitionPattern: expandRouteDefinitionPatternWithDoublestar,
			statRouteDefinitionPath:      os.Stat,
		},
		readRouteDefinitionFile: os.ReadFile,
	}
}

func normalizeRouteParsingExecutorDependencies(
	dependencies routeParsingExecutorDependencies,
) routeParsingExecutorDependencies {
	defaultDependencies := defaultRouteParsingExecutorDependencies()

	if dependencies.routeDefinitionsCodeParsingDependencies.transformRouteDefinitionsCode == nil {
		dependencies.routeDefinitionsCodeParsingDependencies.transformRouteDefinitionsCode = defaultDependencies.routeDefinitionsCodeParsingDependencies.transformRouteDefinitionsCode
	}
	if dependencies.routeDefinitionsCodeParsingDependencies.extractRouteCallsFromTransformedCode == nil {
		dependencies.routeDefinitionsCodeParsingDependencies.extractRouteCallsFromTransformedCode = defaultDependencies.routeDefinitionsCodeParsingDependencies.extractRouteCallsFromTransformedCode
	}

	if dependencies.routeModuleResolutionDependencies.computeRelativeModulePath == nil {
		dependencies.routeModuleResolutionDependencies.computeRelativeModulePath = defaultDependencies.routeModuleResolutionDependencies.computeRelativeModulePath
	}
	if dependencies.routeModuleResolutionDependencies.statRouteModulePath == nil {
		dependencies.routeModuleResolutionDependencies.statRouteModulePath = defaultDependencies.routeModuleResolutionDependencies.statRouteModulePath
	}

	if dependencies.routeDefinitionsFileResolutionDependencies.expandRouteDefinitionPattern == nil {
		dependencies.routeDefinitionsFileResolutionDependencies.expandRouteDefinitionPattern = defaultDependencies.routeDefinitionsFileResolutionDependencies.expandRouteDefinitionPattern
	}
	if dependencies.routeDefinitionsFileResolutionDependencies.statRouteDefinitionPath == nil {
		dependencies.routeDefinitionsFileResolutionDependencies.statRouteDefinitionPath = defaultDependencies.routeDefinitionsFileResolutionDependencies.statRouteDefinitionPath
	}

	if dependencies.routeParsingPipelineDependencies.handleUnresolvedRouteCalls == nil {
		dependencies.routeParsingPipelineDependencies.handleUnresolvedRouteCalls = handleUnresolvedRouteCalls
	}

	if dependencies.readRouteDefinitionFile == nil {
		dependencies.readRouteDefinitionFile = defaultDependencies.readRouteDefinitionFile
	}

	return dependencies
}

func newRouteParsingExecutor(
	dependencies routeParsingExecutorDependencies,
) routeParsingExecutor {
	dependencies = normalizeRouteParsingExecutorDependencies(dependencies)
	executor := routeParsingExecutor{dependencies: dependencies}

	if executor.dependencies.routeParsingPipelineDependencies.resolveClientRouteDefinitionFiles == nil {
		executor.dependencies.routeParsingPipelineDependencies.resolveClientRouteDefinitionFiles = executor.resolveClientRouteDefinitionFiles
	}
	if executor.dependencies.routeParsingPipelineDependencies.parseRouteDefinitionFileIntoCalls == nil {
		executor.dependencies.routeParsingPipelineDependencies.parseRouteDefinitionFileIntoCalls = executor.parseRouteDefinitionFileIntoCalls
	}
	if executor.dependencies.routeParsingPipelineDependencies.mergeRouteCallsIntoPaths == nil {
		executor.dependencies.routeParsingPipelineDependencies.mergeRouteCallsIntoPaths = executor.mergeRouteCallsIntoPaths
	}

	return executor
}

type parsedRouteDefinitionsCode struct {
	routeCalls       []routeCall
	unresolvedRoutes []unresolvedRouteCall
}

func parseClientRoutes(v *vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
	return defaultRouteParsingExecutor.parseClientRoutes(v)
}

func (executor routeParsingExecutor) parseClientRoutes(
	v *vormaruntime.Vorma,
) (map[string]*vormaruntime.Path, error) {
	routeDefinitionFiles, err := executor.dependencies.routeParsingPipelineDependencies.resolveClientRouteDefinitionFiles(v)
	if err != nil {
		return nil, err
	}

	paths := make(map[string]*vormaruntime.Path)
	for _, routeDefinitionFile := range routeDefinitionFiles {
		parsedRouteDefinitions, err := executor.dependencies.routeParsingPipelineDependencies.parseRouteDefinitionFileIntoCalls(
			v,
			routeDefinitionFile,
		)
		if err != nil {
			return nil, err
		}

		if err := executor.dependencies.routeParsingPipelineDependencies.handleUnresolvedRouteCalls(
			v,
			routeDefinitionFile,
			parsedRouteDefinitions.unresolvedRoutes,
		); err != nil {
			return nil, err
		}

		if err := executor.dependencies.routeParsingPipelineDependencies.mergeRouteCallsIntoPaths(
			v,
			paths,
			routeDefinitionFile,
			parsedRouteDefinitions.routeCalls,
		); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func resolveClientRouteDefinitionFiles(v *vormaruntime.Vorma) ([]string, error) {
	return defaultRouteParsingExecutor.resolveClientRouteDefinitionFiles(v)
}

func (executor routeParsingExecutor) resolveClientRouteDefinitionFiles(
	v *vormaruntime.Vorma,
) ([]string, error) {
	if v == nil {
		return nil, errors.New("Vorma runtime is required")
	}
	if v.Config == nil {
		return nil, errors.New("Vorma config is required")
	}
	if len(v.Config.ClientRouteDefinitionPatterns) == 0 {
		return nil, errors.New("Vorma.ClientRouteDefinitionPatterns is required")
	}

	normalizedRouteDefinitionPatterns := normalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ClientRouteDefinitionPatterns,
	)
	if len(normalizedRouteDefinitionPatterns) == 0 {
		return nil, errors.New("Vorma.ClientRouteDefinitionPatterns cannot contain only empty values")
	}

	matchedFilesByPath := make(map[string]struct{})
	for _, routeDefinitionPattern := range normalizedRouteDefinitionPatterns {
		if patternContainsGlobMeta(routeDefinitionPattern) {
			routeDefinitionMatches, err := executor.dependencies.routeDefinitionsFileResolutionDependencies.expandRouteDefinitionPattern(
				routeDefinitionPattern,
			)
			if err != nil {
				return nil, fmt.Errorf("expand route definition pattern %q: %w", routeDefinitionPattern, err)
			}
			for _, routeDefinitionMatch := range routeDefinitionMatches {
				routeDefinitionInfo, err := executor.dependencies.routeDefinitionsFileResolutionDependencies.statRouteDefinitionPath(
					routeDefinitionMatch,
				)
				if err != nil {
					return nil, fmt.Errorf("stat route definition path %q: %w", routeDefinitionMatch, err)
				}
				if routeDefinitionInfo.IsDir() {
					continue
				}
				matchedFilesByPath[filepath.Clean(routeDefinitionMatch)] = struct{}{}
			}
			continue
		}

		routeDefinitionInfo, err := executor.dependencies.routeDefinitionsFileResolutionDependencies.statRouteDefinitionPath(
			routeDefinitionPattern,
		)
		if err != nil {
			return nil, fmt.Errorf("stat route definition path %q: %w", routeDefinitionPattern, err)
		}
		if routeDefinitionInfo.IsDir() {
			return nil, fmt.Errorf("route definition path %q is a directory", routeDefinitionPattern)
		}
		matchedFilesByPath[filepath.Clean(routeDefinitionPattern)] = struct{}{}
	}

	if len(matchedFilesByPath) == 0 {
		return nil, fmt.Errorf(
			"no route definition files matched patterns: %s",
			strings.Join(normalizedRouteDefinitionPatterns, ", "),
		)
	}

	routeDefinitionFiles := make([]string, 0, len(matchedFilesByPath))
	for routeDefinitionFile := range matchedFilesByPath {
		routeDefinitionFiles = append(routeDefinitionFiles, filepath.ToSlash(routeDefinitionFile))
	}
	sort.Strings(routeDefinitionFiles)
	return routeDefinitionFiles, nil
}

func patternContainsGlobMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[{")
}

func expandRouteDefinitionPatternWithDoublestar(pattern string) ([]string, error) {
	return doublestar.FilepathGlob(pattern)
}

func parseRouteDefinitionFileIntoCalls(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
) (parsedRouteDefinitionsCode, error) {
	return defaultRouteParsingExecutor.parseRouteDefinitionFileIntoCalls(v, routeDefinitionFile)
}

func (executor routeParsingExecutor) parseRouteDefinitionFileIntoCalls(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
) (parsedRouteDefinitionsCode, error) {
	code, err := executor.dependencies.readRouteDefinitionFile(routeDefinitionFile)
	if err != nil {
		return parsedRouteDefinitionsCode{}, fmt.Errorf("read route definitions file %q: %w", routeDefinitionFile, err)
	}
	parsedRouteDefinitions, err := executor.parseRouteDefinitionsCodeIntoCalls(v, code)
	if err != nil {
		return parsedRouteDefinitionsCode{}, fmt.Errorf("parse route definitions file %q: %w", routeDefinitionFile, err)
	}
	return parsedRouteDefinitions, nil
}

func (executor routeParsingExecutor) parseRouteDefinitionsCodeIntoCalls(
	v *vormaruntime.Vorma,
	code []byte,
) (parsedRouteDefinitionsCode, error) {
	transformedCode, err := executor.dependencies.routeDefinitionsCodeParsingDependencies.transformRouteDefinitionsCode(
		v,
		code,
	)
	if err != nil {
		return parsedRouteDefinitionsCode{}, err
	}

	routeCalls, unresolvedRoutes, err := executor.dependencies.routeDefinitionsCodeParsingDependencies.extractRouteCallsFromTransformedCode(
		transformedCode,
	)
	if err != nil {
		return parsedRouteDefinitionsCode{}, fmt.Errorf("extract route calls: %w", err)
	}

	return parsedRouteDefinitionsCode{
		routeCalls:       routeCalls,
		unresolvedRoutes: unresolvedRoutes,
	}, nil
}

func transformRouteDefinitionsCode(v *vormaruntime.Vorma, code []byte) (string, error) {
	transformResult := esbuild.Transform(string(code), esbuild.TransformOptions{
		Format:            esbuild.FormatESModule,
		Platform:          esbuild.PlatformNode,
		MinifyWhitespace:  true,
		MinifySyntax:      true,
		MinifyIdentifiers: false,
		Loader:            esbuild.LoaderTSX,
		Target:            esbuild.ES2020,
	})

	if len(transformResult.Errors) > 0 {
		logEsbuildTransformErrors(v, transformResult.Errors)
		return "", errors.New("esbuild transform failed")
	}

	return importRegex.ReplaceAllString(string(transformResult.Code), "$1"), nil
}

func logEsbuildTransformErrors(v *vormaruntime.Vorma, messages []esbuild.Message) {
	for _, message := range messages {
		v.Log.Error(fmt.Sprintf("esbuild error: %s", message.Text))
	}
}

func handleUnresolvedRouteCalls(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
	unresolvedRoutes []unresolvedRouteCall,
) error {
	if len(unresolvedRoutes) == 0 {
		return nil
	}

	unresolvedRoutePolicy, err := resolveUnresolvedRoutePolicy(v)
	if err != nil {
		return err
	}

	if unresolvedRoutePolicy == vormaruntime.UnresolvedRoutePolicyWarn {
		logUnresolvedRouteCallsAsWarnings(v, routeDefinitionFile, unresolvedRoutes)
		return nil
	}

	return buildUnresolvedRouteCallsError(routeDefinitionFile, unresolvedRoutes)
}

func resolveUnresolvedRoutePolicy(v *vormaruntime.Vorma) (string, error) {
	if v == nil {
		return "", errors.New("Vorma runtime is required to resolve unresolved route policy")
	}
	if v.Config == nil {
		return "", errors.New("Vorma config is required to resolve unresolved route policy")
	}

	configuredPolicy := strings.TrimSpace(v.Config.UnresolvedRoutePolicy)
	if configuredPolicy != "" {
		switch configuredPolicy {
		case vormaruntime.UnresolvedRoutePolicyWarn:
			return vormaruntime.UnresolvedRoutePolicyWarn, nil
		case vormaruntime.UnresolvedRoutePolicyError:
			return vormaruntime.UnresolvedRoutePolicyError, nil
		default:
			return "", fmt.Errorf(
				"Vorma.UnresolvedRoutePolicy must be %q or %q",
				vormaruntime.UnresolvedRoutePolicyWarn,
				vormaruntime.UnresolvedRoutePolicyError,
			)
		}
	}

	if v.IsDevMode() {
		return vormaruntime.UnresolvedRoutePolicyWarn, nil
	}
	return vormaruntime.UnresolvedRoutePolicyError, nil
}

func logUnresolvedRouteCallsAsWarnings(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
	unresolvedRoutes []unresolvedRouteCall,
) {
	for _, unresolved := range unresolvedRoutes {
		v.Log.Warn(
			fmt.Sprintf("Route pattern %q has a module path that cannot be statically resolved", unresolved.Pattern),
			"file", routeDefinitionFile,
			"expression", unresolved.RawModuleExpr,
			"reason", unresolved.Reason,
		)
		v.Log.Warn(
			"This route will be ignored. Use a static string path or a const variable assigned to a string literal.",
		)
	}
}

func buildUnresolvedRouteCallsError(
	routeDefinitionFile string,
	unresolvedRoutes []unresolvedRouteCall,
) error {
	unresolvedRouteErrors := make([]error, 0, len(unresolvedRoutes))
	for _, unresolvedRoute := range unresolvedRoutes {
		unresolvedRouteErrors = append(
			unresolvedRouteErrors,
			fmt.Errorf(
				"pattern %q has unresolved module expression %q (%s)",
				unresolvedRoute.Pattern,
				unresolvedRoute.RawModuleExpr,
				unresolvedRoute.Reason,
			),
		)
	}

	return fmt.Errorf(
		"unresolved route calls are not allowed in %q: %w",
		routeDefinitionFile,
		errors.Join(unresolvedRouteErrors...),
	)
}

func mergeRouteCallsIntoPaths(
	v *vormaruntime.Vorma,
	paths map[string]*vormaruntime.Path,
	routeDefinitionFile string,
	routeCalls []routeCall,
) error {
	return defaultRouteParsingExecutor.mergeRouteCallsIntoPaths(v, paths, routeDefinitionFile, routeCalls)
}

func (executor routeParsingExecutor) mergeRouteCallsIntoPaths(
	v *vormaruntime.Vorma,
	paths map[string]*vormaruntime.Path,
	routeDefinitionFile string,
	routeCalls []routeCall,
) error {
	for _, routeCall := range routeCalls {
		if routeCall.Module == "" {
			return fmt.Errorf("component module is required for pattern: %s", routeCall.Pattern)
		}

		if _, hasExistingPattern := paths[routeCall.Pattern]; hasExistingPattern {
			return fmt.Errorf("duplicate route pattern: %s", routeCall.Pattern)
		}

		modulePath := executor.resolveRouteModulePath(v, routeDefinitionFile, routeCall)
		if err := executor.ensureRouteModuleExists(modulePath, routeCall.Pattern); err != nil {
			return err
		}

		paths[routeCall.Pattern] = &vormaruntime.Path{
			OriginalPattern: routeCall.Pattern,
			SrcPath:         modulePath,
			ExportKey:       routeCall.Key,
			ErrorExportKey:  routeCall.ErrorKey,
		}
	}
	return nil
}

func (executor routeParsingExecutor) resolveRouteModulePath(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
	routeCall routeCall,
) string {
	routeDefinitionsDirectory := filepath.Dir(routeDefinitionFile)
	resolvedModulePath, err := executor.dependencies.routeModuleResolutionDependencies.computeRelativeModulePath(
		".",
		filepath.Join(routeDefinitionsDirectory, routeCall.Module),
	)
	if err != nil {
		v.Log.Warn(fmt.Sprintf("could not make module path relative: %s", err))
		resolvedModulePath = routeCall.Module
	}
	return filepath.ToSlash(resolvedModulePath)
}

func ensureRouteModuleExists(modulePath string, pattern string) error {
	return defaultRouteParsingExecutor.ensureRouteModuleExists(modulePath, pattern)
}

func (executor routeParsingExecutor) ensureRouteModuleExists(
	modulePath string,
	pattern string,
) error {
	fileInfo, err := executor.dependencies.routeModuleResolutionDependencies.statRouteModulePath(modulePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("component module does not exist: %s (pattern: %s)", modulePath, pattern)
		}
		return fmt.Errorf("access component module %s: %w", modulePath, err)
	}

	if fileInfo != nil && fileInfo.IsDir() {
		return fmt.Errorf("component module is a directory: %s (pattern: %s)", modulePath, pattern)
	}

	return nil
}

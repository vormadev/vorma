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
	"github.com/vormadev/vorma/vormaruntime"
)

var importRegex = regexp.MustCompile(`import\((` + "`" + `[^` + "`" + `]+` + "`" + `|'[^']+'|"[^"]+")\)`)

type routeParsingPipelineDependencies struct {
	resolveClientRouteDefinitionFiles func(*vormaruntime.Vorma) ([]string, error)
	parseRouteDefinitionFileIntoCalls func(*vormaruntime.Vorma, string) (parsedRouteDefinitionsCode, error)
	warnUnresolvedRouteCalls          func(*vormaruntime.Vorma, string, []unresolvedRouteCall)
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

var routeParsingPipelineDeps = routeParsingPipelineDependencies{
	resolveClientRouteDefinitionFiles: resolveClientRouteDefinitionFiles,
	parseRouteDefinitionFileIntoCalls: parseRouteDefinitionFileIntoCalls,
	warnUnresolvedRouteCalls:          warnUnresolvedRouteCalls,
	mergeRouteCallsIntoPaths:          mergeRouteCallsIntoPaths,
}

var routeDefinitionsCodeParsingDeps = routeDefinitionsCodeParsingDependencies{
	transformRouteDefinitionsCode:        transformRouteDefinitionsCode,
	extractRouteCallsFromTransformedCode: extractRouteCalls,
}

var routeModuleResolutionDeps = routeModuleResolutionDependencies{
	computeRelativeModulePath: filepath.Rel,
	statRouteModulePath:       os.Stat,
}

var routeDefinitionsFileResolutionDeps = routeDefinitionsFileResolutionDependencies{
	expandRouteDefinitionPattern: expandRouteDefinitionPatternWithDoublestar,
	statRouteDefinitionPath:      os.Stat,
}

type parsedRouteDefinitionsCode struct {
	routeCalls       []routeCall
	unresolvedRoutes []unresolvedRouteCall
}

func parseClientRoutes(v *vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
	routeDefinitionFiles, err := routeParsingPipelineDeps.resolveClientRouteDefinitionFiles(v)
	if err != nil {
		return nil, err
	}

	paths := make(map[string]*vormaruntime.Path)
	for _, routeDefinitionFile := range routeDefinitionFiles {
		parsedRouteDefinitions, err := routeParsingPipelineDeps.parseRouteDefinitionFileIntoCalls(
			v,
			routeDefinitionFile,
		)
		if err != nil {
			return nil, err
		}

		routeParsingPipelineDeps.warnUnresolvedRouteCalls(
			v,
			routeDefinitionFile,
			parsedRouteDefinitions.unresolvedRoutes,
		)

		if err := routeParsingPipelineDeps.mergeRouteCallsIntoPaths(
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
			routeDefinitionMatches, err := routeDefinitionsFileResolutionDeps.expandRouteDefinitionPattern(routeDefinitionPattern)
			if err != nil {
				return nil, fmt.Errorf("expand route definition pattern %q: %w", routeDefinitionPattern, err)
			}
			for _, routeDefinitionMatch := range routeDefinitionMatches {
				routeDefinitionInfo, err := routeDefinitionsFileResolutionDeps.statRouteDefinitionPath(routeDefinitionMatch)
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

		routeDefinitionInfo, err := routeDefinitionsFileResolutionDeps.statRouteDefinitionPath(routeDefinitionPattern)
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
	code, err := os.ReadFile(routeDefinitionFile)
	if err != nil {
		return parsedRouteDefinitionsCode{}, fmt.Errorf("read route definitions file %q: %w", routeDefinitionFile, err)
	}
	parsedRouteDefinitions, err := parseRouteDefinitionsCodeIntoCalls(v, code)
	if err != nil {
		return parsedRouteDefinitionsCode{}, fmt.Errorf("parse route definitions file %q: %w", routeDefinitionFile, err)
	}
	return parsedRouteDefinitions, nil
}

func parseRouteDefinitionsCodeIntoCalls(
	v *vormaruntime.Vorma,
	code []byte,
) (parsedRouteDefinitionsCode, error) {
	transformedCode, err := routeDefinitionsCodeParsingDeps.transformRouteDefinitionsCode(v, code)
	if err != nil {
		return parsedRouteDefinitionsCode{}, err
	}

	routeCalls, unresolvedRoutes, err := routeDefinitionsCodeParsingDeps.extractRouteCallsFromTransformedCode(transformedCode)
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

func warnUnresolvedRouteCalls(
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

func mergeRouteCallsIntoPaths(
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

		modulePath := resolveRouteModulePath(v, routeDefinitionFile, routeCall)
		if err := ensureRouteModuleExists(modulePath, routeCall.Pattern); err != nil {
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

func resolveRouteModulePath(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
	routeCall routeCall,
) string {
	routeDefinitionsDirectory := filepath.Dir(routeDefinitionFile)
	resolvedModulePath, err := routeModuleResolutionDeps.computeRelativeModulePath(
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
	fileInfo, err := routeModuleResolutionDeps.statRouteModulePath(modulePath)
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

package vormabuild

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/vormaruntime"
)

var importRegex = regexp.MustCompile(`import\((` + "`" + `[^` + "`" + `]+` + "`" + `|'[^']+'|"[^"]+")\)`)

type routeParsingPipelineDependencies struct {
	readClientRouteDefinitionsFile     func(string) ([]byte, error)
	parseRouteDefinitionsCodeIntoCalls func(*vormaruntime.Vorma, []byte) (parsedRouteDefinitionsCode, error)
	warnUnresolvedRouteCalls           func(*vormaruntime.Vorma, []UnresolvedRouteCall)
	buildPathsFromRouteCalls           func(*vormaruntime.Vorma, []RouteCall) (map[string]*vormaruntime.Path, error)
}

type routeDefinitionsCodeParsingDependencies struct {
	transformRouteDefinitionsCode        func(*vormaruntime.Vorma, []byte) (string, error)
	extractRouteCallsFromTransformedCode func(string) ([]RouteCall, []UnresolvedRouteCall, error)
}

type routeModuleResolutionDependencies struct {
	computeRelativeModulePath func(string, string) (string, error)
	statRouteModulePath       func(string) (fs.FileInfo, error)
}

var routeParsingPipelineDeps = routeParsingPipelineDependencies{
	readClientRouteDefinitionsFile:     os.ReadFile,
	parseRouteDefinitionsCodeIntoCalls: parseRouteDefinitionsCodeIntoCalls,
	warnUnresolvedRouteCalls:           warnUnresolvedRouteCalls,
	buildPathsFromRouteCalls:           buildPathsFromRouteCalls,
}

var routeDefinitionsCodeParsingDeps = routeDefinitionsCodeParsingDependencies{
	transformRouteDefinitionsCode:        transformRouteDefinitionsCode,
	extractRouteCallsFromTransformedCode: extractRouteCalls,
}

var routeModuleResolutionDeps = routeModuleResolutionDependencies{
	computeRelativeModulePath: filepath.Rel,
	statRouteModulePath:       os.Stat,
}

type parsedRouteDefinitionsCode struct {
	routeCalls       []RouteCall
	unresolvedRoutes []UnresolvedRouteCall
}

func parseClientRoutes(v *vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
	code, err := routeParsingPipelineDeps.readClientRouteDefinitionsFile(v.Config.ClientRouteDefsFile)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	parsedRouteDefinitions, err := routeParsingPipelineDeps.parseRouteDefinitionsCodeIntoCalls(v, code)
	if err != nil {
		return nil, err
	}

	routeParsingPipelineDeps.warnUnresolvedRouteCalls(v, parsedRouteDefinitions.unresolvedRoutes)

	return routeParsingPipelineDeps.buildPathsFromRouteCalls(v, parsedRouteDefinitions.routeCalls)
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

func warnUnresolvedRouteCalls(v *vormaruntime.Vorma, unresolvedRoutes []UnresolvedRouteCall) {
	for _, unresolved := range unresolvedRoutes {
		v.Log.Warn(
			fmt.Sprintf("Route pattern %q has a module path that cannot be statically resolved", unresolved.Pattern),
			"file", v.Config.ClientRouteDefsFile,
			"expression", unresolved.RawModuleExpr,
			"reason", unresolved.Reason,
		)
		v.Log.Warn(
			"This route will be ignored. Use a static string path or a const variable assigned to a string literal.",
		)
	}
}

func buildPathsFromRouteCalls(v *vormaruntime.Vorma, routeCalls []RouteCall) (map[string]*vormaruntime.Path, error) {
	paths := make(map[string]*vormaruntime.Path, len(routeCalls))
	routesDir := filepath.Dir(v.Config.ClientRouteDefsFile)

	for _, routeCall := range routeCalls {
		if routeCall.Module == "" {
			return nil, fmt.Errorf("component module is required for pattern: %s", routeCall.Pattern)
		}

		if _, hasExistingPattern := paths[routeCall.Pattern]; hasExistingPattern {
			return nil, fmt.Errorf("duplicate route pattern: %s", routeCall.Pattern)
		}

		modulePath := resolveRouteModulePath(v, routesDir, routeCall)
		if err := ensureRouteModuleExists(modulePath, routeCall.Pattern); err != nil {
			return nil, err
		}

		paths[routeCall.Pattern] = &vormaruntime.Path{
			OriginalPattern: routeCall.Pattern,
			SrcPath:         modulePath,
			ExportKey:       routeCall.Key,
			ErrorExportKey:  routeCall.ErrorKey,
		}
	}

	return paths, nil
}

func resolveRouteModulePath(v *vormaruntime.Vorma, routesDir string, routeCall RouteCall) string {
	resolvedModulePath, err := routeModuleResolutionDeps.computeRelativeModulePath(".", filepath.Join(routesDir, routeCall.Module))
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

// Package routeparse owns Vorma route-definition parsing for build time.
//
// It resolves client route definition files, extracts route() declarations,
// enforces unresolved-route policy, and produces normalized runtime path maps
// consumed by build orchestration.
package routeparse

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimeconfig"
	"github.com/vormadev/vorma/kit/nestedmatcher"
)

type routeCall struct {
	Pattern  string
	Module   string
	Key      string
	ErrorKey string
}

// unresolvedRouteCall represents a route() call where the module path could not
// be statically determined. This happens when the module argument is a variable,
// function call, or other dynamic expression.
type unresolvedRouteCall struct {
	Pattern       string
	RawModuleExpr string
	Reason        string
}

type routeCallVisitor struct {
	routeFuncNames    map[string]bool
	trackedModuleVars map[string]string
	routes            []routeCall
	unresolvedRoutes  []unresolvedRouteCall
}

func (rv *routeCallVisitor) Enter(n js.INode) js.IVisitor {
	call, isCall := n.(*js.CallExpr)
	if !isCall {
		return rv
	}

	ident, isIdent := call.X.(*js.Var)
	if !isIdent {
		return rv
	}

	if _, isRouteFunc := rv.routeFuncNames[string(ident.Data)]; !isRouteFunc {
		return rv
	}

	routeCall, unresolvedRoute := rv.extractRouteCall(call.Args.List)
	if unresolvedRoute != nil {
		rv.unresolvedRoutes = append(rv.unresolvedRoutes, *unresolvedRoute)
		return rv
	}
	if routeCall == nil {
		return rv
	}

	rv.routes = append(rv.routes, *routeCall)
	return rv
}

func (rv *routeCallVisitor) Exit(n js.INode) {
	_ = n
}

func (rv *routeCallVisitor) extractRouteCall(
	argsList []js.Arg,
) (*routeCall, *unresolvedRouteCall) {
	route := routeCall{Key: "default"}

	pattern, ok := extractStaticStringArg(argsList, 0)
	if !ok {
		return nil, nil
	}
	route.Pattern = pattern

	if len(argsList) > 1 {
		modulePath, unresolvedModule := rv.resolveModuleArgument(
			route.Pattern,
			argsList[1].Value,
		)
		if unresolvedModule != nil {
			return nil, unresolvedModule
		}
		route.Module = modulePath
	}

	if key, ok := extractStaticStringArg(argsList, 2); ok {
		route.Key = key
	}
	if errorKey, ok := extractStaticStringArg(argsList, 3); ok {
		route.ErrorKey = errorKey
	}

	return &route, nil
}

func (rv *routeCallVisitor) resolveModuleArgument(
	routePattern string,
	moduleExpr js.IExpr,
) (string, *unresolvedRouteCall) {
	if varRef, ok := moduleExpr.(*js.Var); ok {
		varName := string(varRef.Data)
		if trackedModulePath, exists := rv.trackedModuleVars[varName]; exists {
			return trackedModulePath, nil
		}
		return "", unresolvedModuleArgument(
			routePattern,
			varName,
			fmt.Sprintf(
				"variable '%s' is not a tracked import or const string",
				varName,
			),
		)
	}

	if functionCall, ok := moduleExpr.(*js.CallExpr); ok {
		return resolveModuleArgumentFromFunctionCall(routePattern, functionCall)
	}

	modulePath, ok := extractStaticStringLiteral(moduleExpr)
	if !ok {
		return "", unresolvedModuleArgument(
			routePattern,
			"<expression>",
			"module argument is not a static string, variable, or import() call",
		)
	}
	return modulePath, nil
}

func resolveModuleArgumentFromFunctionCall(
	routePattern string,
	functionCall *js.CallExpr,
) (string, *unresolvedRouteCall) {
	if functionCallTargetsJSImport(functionCall) {
		if modulePath, ok := extractStaticStringArg(functionCall.Args.List, 0); ok {
			return modulePath, nil
		}
		return "", unresolvedModuleArgument(
			routePattern,
			"import(...)",
			"dynamic import() argument is not a static string",
		)
	}

	functionName := "<unknown>"
	if functionIdent, ok := functionCall.X.(*js.Var); ok {
		functionName = string(functionIdent.Data)
	}
	return "", unresolvedModuleArgument(
		routePattern,
		functionName+"(...)",
		"module argument is a function call, which cannot be statically analyzed",
	)
}

func unresolvedModuleArgument(
	routePattern string,
	rawModuleExpr string,
	reason string,
) *unresolvedRouteCall {
	return &unresolvedRouteCall{
		Pattern:       routePattern,
		RawModuleExpr: rawModuleExpr,
		Reason:        reason,
	}
}

func functionCallTargetsJSImport(functionCall *js.CallExpr) bool {
	functionIdent, ok := functionCall.X.(*js.Var)
	return ok && string(functionIdent.Data) == "import"
}

func extractStaticStringArg(args []js.Arg, idx int) (string, bool) {
	if idx >= len(args) {
		return "", false
	}
	return extractStaticStringLiteral(args[idx].Value)
}

func extractStaticStringLiteral(expr js.IExpr) (string, bool) {
	strLit, ok := expr.(*js.LiteralExpr)
	if !ok || strLit.TokenType != js.StringToken {
		return "", false
	}
	unquoted, err := strconv.Unquote(string(strLit.Data))
	if err != nil {
		return "", false
	}
	return unquoted, true
}

func extractRouteCalls(
	code string,
) ([]routeCall, []unresolvedRouteCall, error) {
	parsedAST, err := js.Parse(parse.NewInputString(code), js.Options{})
	if err != nil {
		return nil, nil, fmt.Errorf("parse JS/TS: %w", err)
	}

	metadata := collectRouteParsingMetadata(parsedAST)

	visitor := &routeCallVisitor{
		routeFuncNames:    metadata.routeFuncNames,
		trackedModuleVars: metadata.trackedModuleVars,
	}
	js.Walk(visitor, parsedAST)

	return visitor.routes, visitor.unresolvedRoutes, nil
}

var importRegex = regexp.MustCompile(
	`import\((` + "`" + `[^` + "`" + `]+` + "`" + `|'[^']+'|"[^"]+")\)`,
)

type routeParsingExecutorDependencies struct {
	resolveClientRouteDefinitionFiles    func(*vormaruntime.Vorma) ([]string, error)
	parseRouteDefinitionFileIntoCalls    func(*vormaruntime.Vorma, string) (parsedRouteDefinitionsCode, error)
	handleUnresolvedRouteCalls           func(*vormaruntime.Vorma, string, []unresolvedRouteCall) error
	mergeRouteCallsIntoPaths             func(*vormaruntime.Vorma, map[string]*vormaruntime.Path, string, []routeCall) error
	transformRouteDefinitionsCode        func(*vormaruntime.Vorma, []byte) (string, error)
	extractRouteCallsFromTransformedCode func(string) ([]routeCall, []unresolvedRouteCall, error)
	computeRelativeModulePath            func(string, string) (string, error)
	statRouteModulePath                  func(string) (fs.FileInfo, error)
	expandRouteDefinitionPattern         func(string) ([]string, error)
	statRouteDefinitionPath              func(string) (fs.FileInfo, error)
	readRouteDefinitionFile              func(string) ([]byte, error)
}

type routeParsingExecutor struct {
	dependencies routeParsingExecutorDependencies
}

var defaultRouteParsingExecutor = newRouteParsingExecutor(
	routeParsingExecutorDependencies{},
)

func defaultRouteParsingExecutorDependencies() routeParsingExecutorDependencies {
	return routeParsingExecutorDependencies{
		transformRouteDefinitionsCode:        transformRouteDefinitionsCode,
		extractRouteCallsFromTransformedCode: extractRouteCalls,
		computeRelativeModulePath:            filepath.Rel,
		statRouteModulePath:                  os.Stat,
		expandRouteDefinitionPattern:         expandRouteDefinitionPatternWithDoublestar,
		statRouteDefinitionPath:              os.Stat,
		handleUnresolvedRouteCalls:           handleUnresolvedRouteCalls,
		readRouteDefinitionFile:              os.ReadFile,
	}
}

func withDefaultRouteParsingExecutorDependencies(
	dependencies routeParsingExecutorDependencies,
) routeParsingExecutorDependencies {
	defaultDependencies := defaultRouteParsingExecutorDependencies()

	if dependencies.transformRouteDefinitionsCode == nil {
		dependencies.transformRouteDefinitionsCode = defaultDependencies.transformRouteDefinitionsCode
	}
	if dependencies.extractRouteCallsFromTransformedCode == nil {
		dependencies.extractRouteCallsFromTransformedCode = defaultDependencies.extractRouteCallsFromTransformedCode
	}

	if dependencies.computeRelativeModulePath == nil {
		dependencies.computeRelativeModulePath = defaultDependencies.computeRelativeModulePath
	}
	if dependencies.statRouteModulePath == nil {
		dependencies.statRouteModulePath = defaultDependencies.statRouteModulePath
	}

	if dependencies.expandRouteDefinitionPattern == nil {
		dependencies.expandRouteDefinitionPattern = defaultDependencies.expandRouteDefinitionPattern
	}
	if dependencies.statRouteDefinitionPath == nil {
		dependencies.statRouteDefinitionPath = defaultDependencies.statRouteDefinitionPath
	}

	if dependencies.handleUnresolvedRouteCalls == nil {
		dependencies.handleUnresolvedRouteCalls = handleUnresolvedRouteCalls
	}

	if dependencies.readRouteDefinitionFile == nil {
		dependencies.readRouteDefinitionFile = defaultDependencies.readRouteDefinitionFile
	}

	return dependencies
}

func newRouteParsingExecutor(
	dependencies routeParsingExecutorDependencies,
) routeParsingExecutor {
	return routeParsingExecutor{
		dependencies: withDefaultRouteParsingExecutorDependencies(dependencies),
	}
}

func (executor routeParsingExecutor) resolveClientRouteDefinitionFilesStep() func(*vormaruntime.Vorma) ([]string, error) {
	if executor.dependencies.resolveClientRouteDefinitionFiles != nil {
		return executor.dependencies.resolveClientRouteDefinitionFiles
	}
	return executor.resolveClientRouteDefinitionFiles
}

func (executor routeParsingExecutor) parseRouteDefinitionFileIntoCallsStep() func(*vormaruntime.Vorma, string) (parsedRouteDefinitionsCode, error) {
	if executor.dependencies.parseRouteDefinitionFileIntoCalls != nil {
		return executor.dependencies.parseRouteDefinitionFileIntoCalls
	}
	return executor.parseRouteDefinitionFileIntoCalls
}

func (executor routeParsingExecutor) handleUnresolvedRouteCallsStep() func(*vormaruntime.Vorma, string, []unresolvedRouteCall) error {
	return executor.dependencies.handleUnresolvedRouteCalls
}

func (executor routeParsingExecutor) mergeRouteCallsIntoPathsStep() func(*vormaruntime.Vorma, map[string]*vormaruntime.Path, string, []routeCall) error {
	if executor.dependencies.mergeRouteCallsIntoPaths != nil {
		return executor.dependencies.mergeRouteCallsIntoPaths
	}
	return executor.mergeRouteCallsIntoPaths
}

type parsedRouteDefinitionsCode struct {
	routeCalls       []routeCall
	unresolvedRoutes []unresolvedRouteCall
}

func ParseClientRoutes(
	v *vormaruntime.Vorma,
) (map[string]*vormaruntime.Path, error) {
	return defaultRouteParsingExecutor.ParseClientRoutes(v)
}

func (executor routeParsingExecutor) ParseClientRoutes(
	v *vormaruntime.Vorma,
) (map[string]*vormaruntime.Path, error) {
	resolveRouteDefinitionFiles := executor.resolveClientRouteDefinitionFilesStep()
	parseRouteDefinitionFile := executor.parseRouteDefinitionFileIntoCallsStep()
	handleUnresolvedRoutes := executor.handleUnresolvedRouteCallsStep()
	mergeRouteCalls := executor.mergeRouteCallsIntoPathsStep()

	routeDefinitionFiles, err := resolveRouteDefinitionFiles(v)
	if err != nil {
		return nil, err
	}

	paths := make(map[string]*vormaruntime.Path)
	for _, routeDefinitionFile := range routeDefinitionFiles {
		parsedRouteDefinitions, err := parseRouteDefinitionFile(
			v,
			routeDefinitionFile,
		)
		if err != nil {
			return nil, err
		}

		if err := handleUnresolvedRoutes(
			v,
			routeDefinitionFile,
			parsedRouteDefinitions.unresolvedRoutes,
		); err != nil {
			return nil, err
		}

		if err := mergeRouteCalls(
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

func resolveClientRouteDefinitionFiles(
	v *vormaruntime.Vorma,
) ([]string, error) {
	return defaultRouteParsingExecutor.resolveClientRouteDefinitionFiles(v)
}

func (executor routeParsingExecutor) resolveClientRouteDefinitionFiles(
	v *vormaruntime.Vorma,
) ([]string, error) {
	if v == nil {
		return nil, errors.New("vorma runtime is required")
	}
	if v.Config == nil {
		return nil, errors.New("vorma config is required")
	}
	if len(v.Config.ClientRouteDefinitionPatterns) == 0 {
		return nil, errors.New(
			"Vorma.ClientRouteDefinitionPatterns is required",
		)
	}

	normalizedRouteDefinitionPatterns, err := NormalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ClientRouteDefinitionPatterns,
	)
	if err != nil {
		return nil, err
	}

	matchedFilesByPath := make(map[string]struct{})
	for _, routeDefinitionPattern := range normalizedRouteDefinitionPatterns {
		if patternContainsGlobMeta(routeDefinitionPattern) {
			routeDefinitionMatches, err := executor.dependencies.expandRouteDefinitionPattern(
				routeDefinitionPattern,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"expand route definition pattern %q: %w",
					routeDefinitionPattern,
					err,
				)
			}
			for _, routeDefinitionMatch := range routeDefinitionMatches {
				routeDefinitionInfo, err := executor.dependencies.statRouteDefinitionPath(
					routeDefinitionMatch,
				)
				if err != nil {
					return nil, fmt.Errorf(
						"stat route definition path %q: %w",
						routeDefinitionMatch,
						err,
					)
				}
				if routeDefinitionInfo.IsDir() {
					continue
				}
				matchedFilesByPath[filepath.Clean(routeDefinitionMatch)] = struct{}{}
			}
			continue
		}

		routeDefinitionInfo, err := executor.dependencies.statRouteDefinitionPath(
			routeDefinitionPattern,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"stat route definition path %q: %w",
				routeDefinitionPattern,
				err,
			)
		}
		if routeDefinitionInfo.IsDir() {
			return nil, fmt.Errorf(
				"route definition path %q is a directory",
				routeDefinitionPattern,
			)
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
		routeDefinitionFiles = append(
			routeDefinitionFiles,
			filepath.ToSlash(routeDefinitionFile),
		)
	}
	sort.Strings(routeDefinitionFiles)
	return routeDefinitionFiles, nil
}

func patternContainsGlobMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[{")
}

func expandRouteDefinitionPatternWithDoublestar(
	pattern string,
) ([]string, error) {
	return doublestar.FilepathGlob(pattern)
}

func parseRouteDefinitionFileIntoCalls(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
) (parsedRouteDefinitionsCode, error) {
	return defaultRouteParsingExecutor.parseRouteDefinitionFileIntoCalls(
		v,
		routeDefinitionFile,
	)
}

func (executor routeParsingExecutor) parseRouteDefinitionFileIntoCalls(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
) (parsedRouteDefinitionsCode, error) {
	code, err := executor.dependencies.readRouteDefinitionFile(
		routeDefinitionFile,
	)
	if err != nil {
		return parsedRouteDefinitionsCode{}, fmt.Errorf(
			"read route definitions file %q: %w",
			routeDefinitionFile,
			err,
		)
	}
	parsedRouteDefinitions, err := executor.parseRouteDefinitionsCodeIntoCalls(
		v,
		code,
	)
	if err != nil {
		return parsedRouteDefinitionsCode{}, fmt.Errorf(
			"parse route definitions file %q: %w",
			routeDefinitionFile,
			err,
		)
	}
	return parsedRouteDefinitions, nil
}

func (executor routeParsingExecutor) parseRouteDefinitionsCodeIntoCalls(
	v *vormaruntime.Vorma,
	code []byte,
) (parsedRouteDefinitionsCode, error) {
	transformedCode, err := executor.dependencies.transformRouteDefinitionsCode(
		v,
		code,
	)
	if err != nil {
		return parsedRouteDefinitionsCode{}, err
	}

	routeCalls, unresolvedRoutes, err := executor.dependencies.extractRouteCallsFromTransformedCode(
		transformedCode,
	)
	if err != nil {
		return parsedRouteDefinitionsCode{}, fmt.Errorf(
			"extract route calls: %w",
			err,
		)
	}

	return parsedRouteDefinitionsCode{
		routeCalls:       routeCalls,
		unresolvedRoutes: unresolvedRoutes,
	}, nil
}

func transformRouteDefinitionsCode(
	v *vormaruntime.Vorma,
	code []byte,
) (string, error) {
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

func logEsbuildTransformErrors(
	v *vormaruntime.Vorma,
	messages []esbuild.Message,
) {
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
		logUnresolvedRouteCallsAsWarnings(
			v,
			routeDefinitionFile,
			unresolvedRoutes,
		)
		return nil
	}

	return buildUnresolvedRouteCallsError(routeDefinitionFile, unresolvedRoutes)
}

func resolveUnresolvedRoutePolicy(v *vormaruntime.Vorma) (string, error) {
	if v == nil {
		return "", errors.New(
			"vorma runtime is required to resolve unresolved route policy",
		)
	}
	if v.Config == nil {
		return "", errors.New(
			"vorma config is required to resolve unresolved route policy",
		)
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

	return vormaruntime.UnresolvedRoutePolicyError, nil
}

func logUnresolvedRouteCallsAsWarnings(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
	unresolvedRoutes []unresolvedRouteCall,
) {
	for _, unresolved := range unresolvedRoutes {
		v.Log.Warn(
			fmt.Sprintf(
				"Route pattern %q has a module path that cannot be statically resolved",
				unresolved.Pattern,
			),
			"file",
			routeDefinitionFile,
			"expression",
			unresolved.RawModuleExpr,
			"reason",
			unresolved.Reason,
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
	return defaultRouteParsingExecutor.mergeRouteCallsIntoPaths(
		v,
		paths,
		routeDefinitionFile,
		routeCalls,
	)
}

func (executor routeParsingExecutor) mergeRouteCallsIntoPaths(
	v *vormaruntime.Vorma,
	paths map[string]*vormaruntime.Path,
	routeDefinitionFile string,
	routeCalls []routeCall,
) error {
	routePatternNormalizer := buildRoutePatternNormalizer(v)
	normalizedPatternToOriginalPattern := make(
		map[string]string,
		len(paths)+len(routeCalls),
	)
	for existingOriginalPattern := range paths {
		normalizedPattern, err := normalizeRoutePatternForCollisionChecks(
			routePatternNormalizer,
			existingOriginalPattern,
		)
		if err != nil {
			return err
		}
		normalizedPatternToOriginalPattern[normalizedPattern] =
			existingOriginalPattern
	}

	for _, routeCall := range routeCalls {
		if routeCall.Module == "" {
			return fmt.Errorf(
				"component module is required for pattern: %s",
				routeCall.Pattern,
			)
		}

		if _, hasExistingPattern := paths[routeCall.Pattern]; hasExistingPattern {
			return fmt.Errorf("duplicate route pattern: %s", routeCall.Pattern)
		}
		normalizedPattern, err := normalizeRoutePatternForCollisionChecks(
			routePatternNormalizer,
			routeCall.Pattern,
		)
		if err != nil {
			return err
		}
		if existingOriginalPattern, hasExistingNormalizedPattern :=
			normalizedPatternToOriginalPattern[normalizedPattern]; hasExistingNormalizedPattern {
			return fmt.Errorf(
				"normalized route pattern collision: %s and %s both normalize to %s",
				existingOriginalPattern,
				routeCall.Pattern,
				normalizedPattern,
			)
		}
		normalizedPatternToOriginalPattern[normalizedPattern] =
			routeCall.Pattern

		modulePath := executor.resolveRouteModulePath(
			v,
			routeDefinitionFile,
			routeCall,
		)
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

func buildRoutePatternNormalizer(
	v *vormaruntime.Vorma,
) *nestedmatcher.Matcher {
	routePatternNormalizerOptions := &nestedmatcher.Options{
		DynamicParamPrefix:             ':',
		SplatSegmentIdentifier:         '*',
		ExplicitIndexSegmentIdentifier: "_index",
	}
	if v == nil || v.LoadersRouter() == nil ||
		v.LoadersRouter().NestedRouter == nil {
		return nestedmatcher.New(routePatternNormalizerOptions)
	}

	loadersNestedRouter := v.LoadersRouter().NestedRouter
	routePatternNormalizerOptions.DynamicParamPrefix =
		loadersNestedRouter.DynamicParamPrefix()
	routePatternNormalizerOptions.SplatSegmentIdentifier =
		loadersNestedRouter.SplatSegmentIdentifier()
	routePatternNormalizerOptions.ExplicitIndexSegmentIdentifier =
		loadersNestedRouter.ExplicitIndexSegmentIdentifier()

	return nestedmatcher.New(routePatternNormalizerOptions)
}

func normalizeRoutePatternForCollisionChecks(
	routePatternNormalizer *nestedmatcher.Matcher,
	pattern string,
) (normalizedPattern string, returnedErr error) {
	defer func() {
		recoveredPanicValue := recover()
		if recoveredPanicValue == nil {
			return
		}

		returnedErr = fmt.Errorf(
			"invalid route pattern %q for normalization: %v",
			pattern,
			recoveredPanicValue,
		)
	}()

	normalizedPattern = routePatternNormalizer.NormalizePattern(
		pattern,
	).NormalizedPattern()
	return normalizedPattern, nil
}

func (executor routeParsingExecutor) resolveRouteModulePath(
	v *vormaruntime.Vorma,
	routeDefinitionFile string,
	routeCall routeCall,
) string {
	routeDefinitionsDirectory := filepath.Dir(routeDefinitionFile)
	resolvedModulePath, err := executor.dependencies.computeRelativeModulePath(
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
	return defaultRouteParsingExecutor.ensureRouteModuleExists(
		modulePath,
		pattern,
	)
}

func (executor routeParsingExecutor) ensureRouteModuleExists(
	modulePath string,
	pattern string,
) error {
	fileInfo, err := executor.dependencies.statRouteModulePath(modulePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"component module does not exist: %s (pattern: %s)",
				modulePath,
				pattern,
			)
		}
		return fmt.Errorf("access component module %s: %w", modulePath, err)
	}

	if fileInfo != nil && fileInfo.IsDir() {
		return fmt.Errorf(
			"component module is a directory: %s (pattern: %s)",
			modulePath,
			pattern,
		)
	}

	return nil
}

func NormalizeRouteDefinitionPatternsInInputOrder(
	routeDefinitionPatterns []string,
) ([]string, error) {
	return runtimeconfig.NormalizeAndValidateClientRouteDefinitionPatternsInInputOrder(
		routeDefinitionPatterns,
	)
}

type routeParsingMetadata struct {
	routeFuncNames    map[string]bool
	trackedModuleVars map[string]string
}

func collectRouteParsingMetadata(parsedAST *js.AST) routeParsingMetadata {
	routeFuncNames := make(map[string]bool)
	trackedModuleVars := make(map[string]string)

	for _, statement := range parsedAST.BlockStmt.List {
		if importStmt, isImportStmt := statement.(*js.ImportStmt); isImportStmt {
			if !isBuildtimeImportStatement(importStmt) {
				continue
			}
			for _, alias := range importStmt.List {
				if !isRouteImportAlias(alias) {
					continue
				}
				routeFuncName := routeImportAliasBinding(alias)
				if routeFuncName != "" {
					routeFuncNames[routeFuncName] = true
				}
			}
			continue
		}

		varDecl, isVarDecl := statement.(*js.VarDecl)
		if !isVarDecl {
			continue
		}
		for _, binding := range varDecl.List {
			varBinding, ok := binding.Binding.(*js.Var)
			if !ok {
				continue
			}
			modulePath, ok := extractStaticStringLiteral(binding.Default)
			if !ok {
				continue
			}
			trackedModuleVars[string(varBinding.Data)] = modulePath
		}
	}
	return routeParsingMetadata{
		routeFuncNames:    routeFuncNames,
		trackedModuleVars: trackedModuleVars,
	}
}

func isBuildtimeImportStatement(importStmt *js.ImportStmt) bool {
	return strings.Trim(
		string(importStmt.Module),
		`"'`+"`",
	) == "vorma/buildtime"
}

func isRouteImportAlias(alias js.Alias) bool {
	aliasName := string(alias.Name)
	aliasBinding := string(alias.Binding)
	return aliasName == "route" || (aliasName == "" && aliasBinding == "route")
}

func routeImportAliasBinding(alias js.Alias) string {
	if len(alias.Binding) > 0 {
		return string(alias.Binding)
	}
	return string(alias.Name)
}

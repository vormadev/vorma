package vormabuild

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
	"github.com/vormadev/vorma/vormaruntime"
)

var importRegex = regexp.MustCompile(`import\((` + "`" + `[^` + "`" + `]+` + "`" + `|'[^']+'|"[^"]+")\)`)

type RouteCall struct {
	Pattern  string
	Module   string
	Key      string
	ErrorKey string
}

// UnresolvedRouteCall represents a route() call where the module path could not
// be statically determined. This happens when the module argument is a variable,
// function call, or other dynamic expression.
type UnresolvedRouteCall struct {
	Pattern       string
	RawModuleExpr string
	Reason        string
}

type routeCallVisitor struct {
	routeFuncNames    map[string]bool
	trackedModuleVars map[string]string
	routes            []RouteCall
	unresolvedRoutes  []UnresolvedRouteCall
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

	route, unresolved, resolved := rv.extractRouteCall(call.Args.List)
	if unresolved != nil {
		rv.unresolvedRoutes = append(rv.unresolvedRoutes, *unresolved)
		return rv
	}
	if !resolved {
		return rv
	}

	rv.routes = append(rv.routes, route)
	return rv
}

func (rv *routeCallVisitor) Exit(n js.INode) {}

func (rv *routeCallVisitor) extractRouteCall(argsList []js.Arg) (RouteCall, *UnresolvedRouteCall, bool) {
	route := RouteCall{Key: "default"}

	pattern, ok := extractStaticStringArg(argsList, 0)
	if !ok {
		return RouteCall{}, nil, false
	}
	route.Pattern = pattern

	if len(argsList) > 1 {
		modulePath, unresolvedRoute, resolved := rv.resolveModuleArgument(route.Pattern, argsList[1].Value)
		if unresolvedRoute != nil {
			return RouteCall{}, unresolvedRoute, false
		}
		if !resolved {
			return RouteCall{}, nil, false
		}
		route.Module = modulePath
	}

	if key, ok := extractStaticStringArg(argsList, 2); ok {
		route.Key = key
	}
	if errorKey, ok := extractStaticStringArg(argsList, 3); ok {
		route.ErrorKey = errorKey
	}

	return route, nil, true
}

func (rv *routeCallVisitor) resolveModuleArgument(
	routePattern string,
	moduleExpr js.IExpr,
) (string, *UnresolvedRouteCall, bool) {
	if varRef, ok := moduleExpr.(*js.Var); ok {
		varName := string(varRef.Data)
		if trackedModulePath, exists := rv.trackedModuleVars[varName]; exists {
			return trackedModulePath, nil, true
		}
		return "", &UnresolvedRouteCall{
			Pattern:       routePattern,
			RawModuleExpr: varName,
			Reason:        fmt.Sprintf("variable '%s' is not a tracked import or const string", varName),
		}, false
	}

	if functionCall, ok := moduleExpr.(*js.CallExpr); ok {
		return resolveModuleArgumentFromFunctionCall(routePattern, functionCall)
	}

	modulePath, ok := extractStaticStringLiteral(moduleExpr)
	if !ok {
		return "", &UnresolvedRouteCall{
			Pattern:       routePattern,
			RawModuleExpr: "<expression>",
			Reason:        "module argument is not a static string, variable, or import() call",
		}, false
	}
	return modulePath, nil, true
}

func resolveModuleArgumentFromFunctionCall(
	routePattern string,
	functionCall *js.CallExpr,
) (string, *UnresolvedRouteCall, bool) {
	if functionCallTargetsJSImport(functionCall) {
		if modulePath, ok := extractStaticStringArg(functionCall.Args.List, 0); ok {
			return modulePath, nil, true
		}
		return "", &UnresolvedRouteCall{
			Pattern:       routePattern,
			RawModuleExpr: "import(...)",
			Reason:        "dynamic import() argument is not a static string",
		}, false
	}

	functionName := "<unknown>"
	if functionIdent, ok := functionCall.X.(*js.Var); ok {
		functionName = string(functionIdent.Data)
	}
	return "", &UnresolvedRouteCall{
		Pattern:       routePattern,
		RawModuleExpr: functionName + "(...)",
		Reason:        "module argument is a function call, which cannot be statically analyzed",
	}, false
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

func extractRouteCalls(code string) ([]RouteCall, []UnresolvedRouteCall, error) {
	parsedAST, err := js.Parse(parse.NewInputString(code), js.Options{})
	if err != nil {
		return nil, nil, fmt.Errorf("parse JS/TS: %w", err)
	}

	routeFuncNames := collectBuildtimeRouteFunctionNames(parsedAST)
	trackedModuleVars := collectStaticStringVariableAssignments(parsedAST)

	visitor := &routeCallVisitor{
		routeFuncNames:    routeFuncNames,
		trackedModuleVars: trackedModuleVars,
	}
	js.Walk(visitor, parsedAST)

	return visitor.routes, visitor.unresolvedRoutes, nil
}

func collectBuildtimeRouteFunctionNames(parsedAST *js.AST) map[string]bool {
	routeFuncNames := make(map[string]bool)
	for _, statement := range parsedAST.BlockStmt.List {
		importStmt, isImportStmt := statement.(*js.ImportStmt)
		if !isImportStmt {
			continue
		}
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
	}
	return routeFuncNames
}

func isBuildtimeImportStatement(importStmt *js.ImportStmt) bool {
	return strings.Trim(string(importStmt.Module), `"'`+"`") == "vorma/buildtime"
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

func collectStaticStringVariableAssignments(parsedAST *js.AST) map[string]string {
	trackedModuleVars := make(map[string]string)
	for _, statement := range parsedAST.BlockStmt.List {
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
	return trackedModuleVars
}

func parseClientRoutes(v *vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
	code, err := os.ReadFile(v.Config.ClientRouteDefsFile)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	transformedCode, err := transformRouteDefinitionsCode(v, code)
	if err != nil {
		return nil, err
	}

	routeCalls, unresolvedRoutes, err := extractRouteCalls(transformedCode)
	if err != nil {
		return nil, fmt.Errorf("extract route calls: %w", err)
	}

	warnUnresolvedRouteCalls(v, unresolvedRoutes)

	return buildPathsFromRouteCalls(v, routeCalls)
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
	resolvedModulePath, err := filepath.Rel(".", filepath.Join(routesDir, routeCall.Module))
	if err != nil {
		v.Log.Warn(fmt.Sprintf("could not make module path relative: %s", err))
		resolvedModulePath = routeCall.Module
	}
	return filepath.ToSlash(resolvedModulePath)
}

func ensureRouteModuleExists(modulePath string, pattern string) error {
	if _, err := os.Stat(modulePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("component module does not exist: %s (pattern: %s)", modulePath, pattern)
		}
		return fmt.Errorf("access component module %s: %w", modulePath, err)
	}
	return nil
}

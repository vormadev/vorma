package vormabuild

import (
	"fmt"
	"strconv"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
)

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
) (*RouteCall, *UnresolvedRouteCall) {
	route := RouteCall{Key: "default"}

	pattern, ok := extractStaticStringArg(argsList, 0)
	if !ok {
		return nil, nil
	}
	route.Pattern = pattern

	if len(argsList) > 1 {
		modulePath, unresolvedModule := rv.resolveModuleArgument(route.Pattern, argsList[1].Value)
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
) (string, *UnresolvedRouteCall) {
	if varRef, ok := moduleExpr.(*js.Var); ok {
		varName := string(varRef.Data)
		if trackedModulePath, exists := rv.trackedModuleVars[varName]; exists {
			return trackedModulePath, nil
		}
		return "", unresolvedModuleArgument(
			routePattern,
			varName,
			fmt.Sprintf("variable '%s' is not a tracked import or const string", varName),
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
) (string, *UnresolvedRouteCall) {
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
) *UnresolvedRouteCall {
	return &UnresolvedRouteCall{
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

func extractRouteCalls(code string) ([]RouteCall, []UnresolvedRouteCall, error) {
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

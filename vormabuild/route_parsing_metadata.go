package vormabuild

import (
	"strings"

	"github.com/tdewolff/parse/v2/js"
)

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

package vormabuild

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	importpath "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/vormadev/vorma/vormaruntime"
)

type backendRouteDiscoveryDependencies struct {
	expandPattern    func(string) ([]string, error)
	statPath         func(string) (fs.FileInfo, error)
	readFile         func(string) ([]byte, error)
	parseGoSourceAST func(*token.FileSet, string, any, parser.Mode) (*ast.File, error)
}

var backendRouteDiscoveryDeps = backendRouteDiscoveryDependencies{
	expandPattern: func(pattern string) ([]string, error) {
		return doublestar.FilepathGlob(pattern)
	},
	statPath:         os.Stat,
	readFile:         os.ReadFile,
	parseGoSourceAST: parser.ParseFile,
}

type parsedServerRouteFile struct {
	path           string
	parsedAST      *ast.File
	importAliases  map[string]string
	dotImportPaths map[string]struct{}
}

type localFunctionDeclaration struct {
	name string
	decl *ast.FuncDecl
	file *parsedServerRouteFile
	obj  *ast.Object
}

type backendRoutePackageAnalysis struct {
	goFileSet              *token.FileSet
	files                  []*parsedServerRouteFile
	stringConstResolver    *goStringConstResolver
	localFunctionsByName   map[string][]*localFunctionDeclaration
	localFunctionsByObject map[*ast.Object]*localFunctionDeclaration
}

type backendRouteDiscoveryState struct {
	loaderPatternSet map[string]struct{}
	activeFunctions  map[*ast.FuncDecl]struct{}
}

type expressionBindings struct {
	values map[string]ast.Expr
	parent *expressionBindings
}

type canonicalRouteRegistrationCall struct {
	isLoader bool
	pattern  string
}

func parseBackendLoaderPatterns(v *vormaruntime.Vorma) ([]string, error) {
	serverRouteDefinitionFiles, err := resolveServerRouteDefinitionFiles(v)
	if err != nil {
		return nil, err
	}
	if len(serverRouteDefinitionFiles) == 0 {
		return nil, nil
	}

	packageAnalyses, err := parseServerRouteFilesIntoPackageAnalyses(
		serverRouteDefinitionFiles,
	)
	if err != nil {
		return nil, err
	}

	loaderPatternSet := map[string]struct{}{}
	for _, packageAnalysis := range packageAnalyses {
		packageLoaderPatterns, err := packageAnalysis.discoverLoaderPatterns()
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

func parseServerRouteFilesIntoPackageAnalyses(
	serverRouteDefinitionFiles []string,
) ([]*backendRoutePackageAnalysis, error) {
	goFileSet := token.NewFileSet()
	analysesByPackageKey := map[string]*backendRoutePackageAnalysis{}

	for _, serverRouteDefinitionFile := range serverRouteDefinitionFiles {
		parsedAST, err := parseServerRouteDefinitionFile(
			goFileSet,
			serverRouteDefinitionFile,
		)
		if err != nil {
			return nil, err
		}

		parsedServerFile := parseServerRouteFileMetadata(
			serverRouteDefinitionFile,
			parsedAST,
		)
		packageKey := deriveServerRoutePackageAnalysisKey(parsedServerFile)
		packageAnalysis, hasPackageAnalysis := analysesByPackageKey[packageKey]
		if !hasPackageAnalysis {
			packageAnalysis = &backendRoutePackageAnalysis{
				goFileSet:              goFileSet,
				localFunctionsByName:   map[string][]*localFunctionDeclaration{},
				localFunctionsByObject: map[*ast.Object]*localFunctionDeclaration{},
			}
			analysesByPackageKey[packageKey] = packageAnalysis
		}
		packageAnalysis.files = append(packageAnalysis.files, parsedServerFile)
	}

	packageKeys := make([]string, 0, len(analysesByPackageKey))
	for packageKey := range analysesByPackageKey {
		packageKeys = append(packageKeys, packageKey)
	}
	sort.Strings(packageKeys)

	packageAnalyses := make([]*backendRoutePackageAnalysis, 0, len(packageKeys))
	for _, packageKey := range packageKeys {
		packageAnalysis := analysesByPackageKey[packageKey]
		if err := packageAnalysis.initialize(); err != nil {
			return nil, err
		}
		packageAnalyses = append(packageAnalyses, packageAnalysis)
	}
	return packageAnalyses, nil
}

func parseServerRouteFileMetadata(
	serverRouteDefinitionFile string,
	parsedAST *ast.File,
) *parsedServerRouteFile {
	parsedServerFile := &parsedServerRouteFile{
		path:           filepath.ToSlash(filepath.Clean(serverRouteDefinitionFile)),
		parsedAST:      parsedAST,
		importAliases:  map[string]string{},
		dotImportPaths: map[string]struct{}{},
	}

	for _, importSpec := range parsedAST.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			continue
		}
		if importSpec.Name == nil {
			defaultAlias := importpath.Base(importPath)
			if defaultAlias != "" {
				parsedServerFile.importAliases[defaultAlias] = importPath
			}
			continue
		}

		importAlias := importSpec.Name.Name
		switch importAlias {
		case "_":
			continue
		case ".":
			parsedServerFile.dotImportPaths[importPath] = struct{}{}
		default:
			parsedServerFile.importAliases[importAlias] = importPath
		}
	}

	return parsedServerFile
}

func deriveServerRoutePackageAnalysisKey(
	parsedServerFile *parsedServerRouteFile,
) string {
	return filepath.ToSlash(filepath.Dir(parsedServerFile.path)) + "|" + parsedServerFile.parsedAST.Name.Name
}

func (analysis *backendRoutePackageAnalysis) initialize() error {
	sort.Slice(analysis.files, func(i int, j int) bool {
		return analysis.files[i].path < analysis.files[j].path
	})

	parsedGoFiles := make([]*ast.File, 0, len(analysis.files))
	for _, parsedServerFile := range analysis.files {
		parsedGoFiles = append(parsedGoFiles, parsedServerFile.parsedAST)
	}
	analysis.stringConstResolver = newGoStringConstResolver(
		collectPackageStringConstExpressions(parsedGoFiles),
	)

	for _, parsedServerFile := range analysis.files {
		for _, declaration := range parsedServerFile.parsedAST.Decls {
			functionDeclaration, isFunctionDeclaration := declaration.(*ast.FuncDecl)
			if !isFunctionDeclaration || functionDeclaration.Recv != nil {
				continue
			}
			localFunction := &localFunctionDeclaration{
				name: functionDeclaration.Name.Name,
				decl: functionDeclaration,
				file: parsedServerFile,
				obj:  functionDeclaration.Name.Obj,
			}
			analysis.localFunctionsByName[localFunction.name] = append(
				analysis.localFunctionsByName[localFunction.name],
				localFunction,
			)
			if localFunction.obj != nil {
				analysis.localFunctionsByObject[localFunction.obj] = localFunction
			}
		}
	}

	return nil
}

func (analysis *backendRoutePackageAnalysis) discoverLoaderPatterns() ([]string, error) {
	state := &backendRouteDiscoveryState{
		loaderPatternSet: map[string]struct{}{},
		activeFunctions:  map[*ast.FuncDecl]struct{}{},
	}

	for _, parsedServerFile := range analysis.files {
		for _, declaration := range parsedServerFile.parsedAST.Decls {
			switch typedDeclaration := declaration.(type) {
			case *ast.GenDecl:
				if typedDeclaration.Tok != token.VAR {
					continue
				}
				if err := analysis.discoverLoaderPatternsFromVarDeclaration(
					typedDeclaration,
					parsedServerFile,
					state,
				); err != nil {
					return nil, err
				}
			case *ast.FuncDecl:
				if typedDeclaration.Recv != nil || typedDeclaration.Name.Name != "init" {
					continue
				}
				if err := analysis.discoverLoaderPatternsFromFunctionDeclaration(
					&localFunctionDeclaration{
						name: "init",
						decl: typedDeclaration,
						file: parsedServerFile,
						obj:  typedDeclaration.Name.Obj,
					},
					nil,
					state,
				); err != nil {
					return nil, err
				}
			}
		}
	}

	loaderPatterns := make([]string, 0, len(state.loaderPatternSet))
	for loaderPattern := range state.loaderPatternSet {
		loaderPatterns = append(loaderPatterns, loaderPattern)
	}
	sort.Strings(loaderPatterns)
	return loaderPatterns, nil
}

func (analysis *backendRoutePackageAnalysis) discoverLoaderPatternsFromVarDeclaration(
	varDeclaration *ast.GenDecl,
	parsedServerFile *parsedServerRouteFile,
	state *backendRouteDiscoveryState,
) error {
	for _, specNode := range varDeclaration.Specs {
		valueSpec, isValueSpec := specNode.(*ast.ValueSpec)
		if !isValueSpec {
			continue
		}
		for _, valueExpression := range valueSpec.Values {
			if err := walkCallExpressionsInNode(
				valueExpression,
				func(call *ast.CallExpr) error {
					return analysis.analyzeCallExpression(
						call,
						parsedServerFile,
						nil,
						state,
					)
				},
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (analysis *backendRoutePackageAnalysis) analyzeCallExpression(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
	state *backendRouteDiscoveryState,
) error {
	canonicalCall, isCanonicalCall, err := analysis.parseCanonicalRouteRegistrationCall(
		call,
		parsedServerFile,
		bindings,
	)
	if err != nil {
		return analysis.withPositionError(call.Pos(), err)
	}
	if isCanonicalCall {
		if canonicalCall.isLoader {
			state.loaderPatternSet[canonicalCall.pattern] = struct{}{}
		}
		return nil
	}

	localFunctionDeclaration, hasLocalFunction, err := analysis.resolveLocalFunctionDeclarationForCall(
		call,
		call.Pos(),
	)
	if err != nil {
		return err
	}
	if !hasLocalFunction {
		return nil
	}

	childBindings := bindFunctionCallArgumentsToParameterNames(
		localFunctionDeclaration.decl,
		call.Args,
		bindings,
	)
	return analysis.discoverLoaderPatternsFromFunctionDeclaration(
		localFunctionDeclaration,
		childBindings,
		state,
	)
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalRouteRegistrationCall(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
) (*canonicalRouteRegistrationCall, bool, error) {
	calleeExpression := unwrapGenericCalleeExpression(call.Fun)

	switch typedCallee := calleeExpression.(type) {
	case *ast.SelectorExpr:
		importAlias, isImportAlias := typedCallee.X.(*ast.Ident)
		if !isImportAlias {
			return nil, false, nil
		}
		if importAlias.Obj != nil {
			return nil, false, nil
		}
		importPath, hasImportAlias := parsedServerFile.importAliases[importAlias.Name]
		if !hasImportAlias {
			return nil, false, nil
		}
		return analysis.parseCanonicalRouteRegistrationCallByImportPath(
			call,
			bindings,
			importPath,
			typedCallee.Sel.Name,
		)
	case *ast.Ident:
		if typedCallee.Obj != nil {
			return nil, false, nil
		}
		for importPath := range parsedServerFile.dotImportPaths {
			registrationCall, isRegistrationCall, err := analysis.parseCanonicalRouteRegistrationCallByImportPath(
				call,
				bindings,
				importPath,
				typedCallee.Name,
			)
			if err != nil {
				return nil, false, err
			}
			if isRegistrationCall {
				return registrationCall, true, nil
			}
		}
		return nil, false, nil
	default:
		return nil, false, nil
	}
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalRouteRegistrationCallByImportPath(
	call *ast.CallExpr,
	bindings *expressionBindings,
	importPath string,
	functionName string,
) (*canonicalRouteRegistrationCall, bool, error) {
	switch {
	case importPath == "github.com/vormadev/vorma" && functionName == "NewLoader":
		return analysis.parseCanonicalLoaderRegistrationCall(
			call,
			bindings,
			"github.com/vormadev/vorma.NewLoader",
			1,
		)
	case importPath == "github.com/vormadev/vorma" && functionName == "NewAction":
		return analysis.parseCanonicalActionRegistrationCall(
			call,
			bindings,
			"github.com/vormadev/vorma.NewAction",
			1,
			2,
		)
	case importPath == "github.com/vormadev/vorma/kit/mux" && functionName == "RegisterNestedTaskHandler":
		return analysis.parseCanonicalLoaderRegistrationCall(
			call,
			bindings,
			"github.com/vormadev/vorma/kit/mux.RegisterNestedTaskHandler",
			1,
		)
	case importPath == "github.com/vormadev/vorma/kit/mux" && functionName == "RegisterTaskHandler":
		return analysis.parseCanonicalActionRegistrationCall(
			call,
			bindings,
			"github.com/vormadev/vorma/kit/mux.RegisterTaskHandler",
			1,
			2,
		)
	default:
		return nil, false, nil
	}
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalLoaderRegistrationCall(
	call *ast.CallExpr,
	bindings *expressionBindings,
	calleeDisplayName string,
	patternArgumentIndex int,
) (*canonicalRouteRegistrationCall, bool, error) {
	if len(call.Args) <= patternArgumentIndex {
		return nil, false, fmt.Errorf(
			"%s requires a pattern argument",
			calleeDisplayName,
		)
	}

	loaderPattern, isCompileTimePattern := resolveCompileTimeStringExpression(
		call.Args[patternArgumentIndex],
		bindings,
		analysis.stringConstResolver,
		0,
	)
	if !isCompileTimePattern {
		return nil, false, fmt.Errorf(
			"%s pattern argument must resolve to compile-time string",
			calleeDisplayName,
		)
	}

	return &canonicalRouteRegistrationCall{
		isLoader: true,
		pattern:  loaderPattern,
	}, true, nil
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalActionRegistrationCall(
	call *ast.CallExpr,
	bindings *expressionBindings,
	calleeDisplayName string,
	methodArgumentIndex int,
	patternArgumentIndex int,
) (*canonicalRouteRegistrationCall, bool, error) {
	if len(call.Args) <= patternArgumentIndex {
		return nil, false, fmt.Errorf(
			"%s requires method and pattern arguments",
			calleeDisplayName,
		)
	}

	if _, isCompileTimeMethod := resolveCompileTimeStringExpression(
		call.Args[methodArgumentIndex],
		bindings,
		analysis.stringConstResolver,
		0,
	); !isCompileTimeMethod {
		return nil, false, fmt.Errorf(
			"%s method argument must resolve to compile-time string",
			calleeDisplayName,
		)
	}

	if _, isCompileTimePattern := resolveCompileTimeStringExpression(
		call.Args[patternArgumentIndex],
		bindings,
		analysis.stringConstResolver,
		0,
	); !isCompileTimePattern {
		return nil, false, fmt.Errorf(
			"%s pattern argument must resolve to compile-time string",
			calleeDisplayName,
		)
	}

	return &canonicalRouteRegistrationCall{isLoader: false}, true, nil
}

func unwrapGenericCalleeExpression(calleeExpression ast.Expr) ast.Expr {
	currentExpression := calleeExpression
	for {
		switch typedExpression := currentExpression.(type) {
		case *ast.IndexExpr:
			currentExpression = typedExpression.X
		case *ast.IndexListExpr:
			currentExpression = typedExpression.X
		default:
			return currentExpression
		}
	}
}

func (analysis *backendRoutePackageAnalysis) resolveLocalFunctionDeclarationForCall(
	call *ast.CallExpr,
	position token.Pos,
) (*localFunctionDeclaration, bool, error) {
	calleeExpression := unwrapGenericCalleeExpression(call.Fun)
	calleeIdentifier, isIdentifier := calleeExpression.(*ast.Ident)
	if !isIdentifier {
		return nil, false, nil
	}

	if calleeIdentifier.Obj != nil {
		if calleeIdentifier.Obj.Kind != ast.Fun {
			return nil, false, nil
		}
		localFunctionDeclaration, hasLocalFunction := analysis.localFunctionsByObject[calleeIdentifier.Obj]
		if !hasLocalFunction {
			return nil, false, nil
		}
		return localFunctionDeclaration, true, nil
	}

	localFunctionDeclarations := analysis.localFunctionsByName[calleeIdentifier.Name]
	switch len(localFunctionDeclarations) {
	case 0:
		return nil, false, nil
	case 1:
		return localFunctionDeclarations[0], true, nil
	default:
		return nil, false, analysis.withPositionError(
			position,
			fmt.Errorf(
				"ambiguous local function %q in route discovery; choose unique function name",
				calleeIdentifier.Name,
			),
		)
	}
}

func bindFunctionCallArgumentsToParameterNames(
	functionDeclaration *ast.FuncDecl,
	callArguments []ast.Expr,
	parentBindings *expressionBindings,
) *expressionBindings {
	if functionDeclaration.Type == nil || functionDeclaration.Type.Params == nil {
		return &expressionBindings{values: map[string]ast.Expr{}, parent: parentBindings}
	}

	parameterNames := make([]string, 0)
	for _, parameterField := range functionDeclaration.Type.Params.List {
		for _, parameterName := range parameterField.Names {
			parameterNames = append(parameterNames, parameterName.Name)
		}
	}

	childBindings := &expressionBindings{
		values: map[string]ast.Expr{},
		parent: parentBindings,
	}
	for parameterIndex, parameterName := range parameterNames {
		if parameterIndex >= len(callArguments) {
			continue
		}
		childBindings.values[parameterName] = callArguments[parameterIndex]
	}
	return childBindings
}

func (analysis *backendRoutePackageAnalysis) discoverLoaderPatternsFromFunctionDeclaration(
	localFunction *localFunctionDeclaration,
	bindings *expressionBindings,
	state *backendRouteDiscoveryState,
) error {
	if localFunction.decl == nil || localFunction.decl.Body == nil {
		return nil
	}

	if _, isActive := state.activeFunctions[localFunction.decl]; isActive {
		return analysis.withPositionError(
			localFunction.decl.Pos(),
			fmt.Errorf(
				"recursive route registration call graph through %q is unsupported",
				localFunction.name,
			),
		)
	}
	state.activeFunctions[localFunction.decl] = struct{}{}
	defer delete(state.activeFunctions, localFunction.decl)

	for _, statement := range localFunction.decl.Body.List {
		if err := walkCallExpressionsInNode(statement, func(call *ast.CallExpr) error {
			return analysis.analyzeCallExpression(
				call,
				localFunction.file,
				bindings,
				state,
			)
		}); err != nil {
			return err
		}
	}

	return nil
}

func walkCallExpressionsInNode(
	node ast.Node,
	visitCall func(*ast.CallExpr) error,
) error {
	var walkError error
	ast.Inspect(node, func(currentNode ast.Node) bool {
		if walkError != nil {
			return false
		}
		if currentNode == nil {
			return true
		}
		if _, isFunctionLiteral := currentNode.(*ast.FuncLit); isFunctionLiteral {
			return false
		}
		callExpression, isCallExpression := currentNode.(*ast.CallExpr)
		if !isCallExpression {
			return true
		}
		walkError = visitCall(callExpression)
		return walkError == nil
	})
	return walkError
}

func resolveCompileTimeStringExpression(
	expression ast.Expr,
	bindings *expressionBindings,
	stringConstResolver *goStringConstResolver,
	depth int,
) (string, bool) {
	if depth > 64 {
		return "", false
	}

	switch typedExpression := expression.(type) {
	case *ast.BasicLit:
		if typedExpression.Kind != token.STRING {
			return "", false
		}
		unquotedValue, err := strconv.Unquote(typedExpression.Value)
		if err != nil {
			return "", false
		}
		return unquotedValue, true
	case *ast.ParenExpr:
		return resolveCompileTimeStringExpression(
			typedExpression.X,
			bindings,
			stringConstResolver,
			depth+1,
		)
	case *ast.BinaryExpr:
		if typedExpression.Op != token.ADD {
			return "", false
		}
		leftValue, leftIsString := resolveCompileTimeStringExpression(
			typedExpression.X,
			bindings,
			stringConstResolver,
			depth+1,
		)
		if !leftIsString {
			return "", false
		}
		rightValue, rightIsString := resolveCompileTimeStringExpression(
			typedExpression.Y,
			bindings,
			stringConstResolver,
			depth+1,
		)
		if !rightIsString {
			return "", false
		}
		return leftValue + rightValue, true
	case *ast.Ident:
		if bindings != nil {
			if boundExpression, hasBoundExpression := bindings.resolveBoundExpression(typedExpression.Name); hasBoundExpression {
				return resolveCompileTimeStringExpression(
					boundExpression,
					bindings,
					stringConstResolver,
					depth+1,
				)
			}
		}
		return stringConstResolver.resolveConstIdentifier(typedExpression.Name)
	default:
		return "", false
	}
}

func (bindings *expressionBindings) resolveBoundExpression(
	expressionName string,
) (ast.Expr, bool) {
	for currentBindings := bindings; currentBindings != nil; currentBindings = currentBindings.parent {
		boundExpression, hasBoundExpression := currentBindings.values[expressionName]
		if !hasBoundExpression {
			continue
		}
		if boundIdentifier, isBoundIdentifier := boundExpression.(*ast.Ident); isBoundIdentifier && boundIdentifier.Name == expressionName {
			continue
		}
		return boundExpression, true
	}
	return nil, false
}

func (analysis *backendRoutePackageAnalysis) withPositionError(
	position token.Pos,
	err error,
) error {
	filePosition := analysis.goFileSet.Position(position)
	if filePosition.Filename == "" {
		return err
	}
	lineNumber := filePosition.Line
	if lineNumber <= 0 {
		lineNumber = 1
	}
	return fmt.Errorf("%s:%d: %w", filepath.ToSlash(filePosition.Filename), lineNumber, err)
}

func resolveServerRouteDefinitionFiles(v *vormaruntime.Vorma) ([]string, error) {
	if v == nil {
		return nil, fmt.Errorf("Vorma runtime is required")
	}
	if v.Config == nil {
		return nil, fmt.Errorf("Vorma config is required")
	}

	normalizedPatterns := normalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ServerRouteDefinitionPatterns,
	)
	if len(normalizedPatterns) == 0 {
		return nil, nil
	}

	matchedFilesByPath := map[string]struct{}{}
	for _, routeDefinitionPattern := range normalizedPatterns {
		matchedFiles, err := resolveServerRouteDefinitionPattern(routeDefinitionPattern)
		if err != nil {
			return nil, err
		}
		for _, matchedFile := range matchedFiles {
			matchedFilesByPath[filepath.ToSlash(filepath.Clean(matchedFile))] = struct{}{}
		}
	}

	if len(matchedFilesByPath) == 0 {
		return nil, fmt.Errorf(
			"no server route definition files matched patterns: %s",
			strings.Join(normalizedPatterns, ", "),
		)
	}

	serverRouteDefinitionFiles := make([]string, 0, len(matchedFilesByPath))
	for serverRouteDefinitionFile := range matchedFilesByPath {
		serverRouteDefinitionFiles = append(
			serverRouteDefinitionFiles,
			serverRouteDefinitionFile,
		)
	}
	sort.Strings(serverRouteDefinitionFiles)
	return serverRouteDefinitionFiles, nil
}

func resolveServerRouteDefinitionPattern(routeDefinitionPattern string) ([]string, error) {
	if !patternContainsGlobMeta(routeDefinitionPattern) {
		fileInfo, err := backendRouteDiscoveryDeps.statPath(routeDefinitionPattern)
		if err != nil {
			return nil, fmt.Errorf(
				"stat server route definition path %q: %w",
				routeDefinitionPattern,
				err,
			)
		}
		if fileInfo.IsDir() {
			return nil, fmt.Errorf(
				"server route definition path %q is a directory",
				routeDefinitionPattern,
			)
		}
		if filepath.Ext(routeDefinitionPattern) != ".go" {
			return nil, fmt.Errorf(
				"server route definition path %q must be a .go file",
				routeDefinitionPattern,
			)
		}
		return []string{routeDefinitionPattern}, nil
	}

	matchedPaths, err := backendRouteDiscoveryDeps.expandPattern(routeDefinitionPattern)
	if err != nil {
		return nil, fmt.Errorf(
			"expand server route definition pattern %q: %w",
			routeDefinitionPattern,
			err,
		)
	}

	matchedGoFiles := make([]string, 0, len(matchedPaths))
	for _, matchedPath := range matchedPaths {
		fileInfo, err := backendRouteDiscoveryDeps.statPath(matchedPath)
		if err != nil {
			return nil, fmt.Errorf(
				"stat server route definition path %q: %w",
				matchedPath,
				err,
			)
		}
		if fileInfo.IsDir() {
			continue
		}
		if filepath.Ext(matchedPath) != ".go" {
			continue
		}
		matchedGoFiles = append(matchedGoFiles, matchedPath)
	}
	return matchedGoFiles, nil
}

func parseServerRouteDefinitionFile(
	goFileSet *token.FileSet,
	serverRouteDefinitionFile string,
) (*ast.File, error) {
	fileBytes, err := backendRouteDiscoveryDeps.readFile(serverRouteDefinitionFile)
	if err != nil {
		return nil, fmt.Errorf(
			"read server route definition file %q: %w",
			serverRouteDefinitionFile,
			err,
		)
	}
	parsedFile, err := backendRouteDiscoveryDeps.parseGoSourceAST(
		goFileSet,
		serverRouteDefinitionFile,
		fileBytes,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"parse server route definition file %q: %w",
			serverRouteDefinitionFile,
			err,
		)
	}
	return parsedFile, nil
}

func collectPackageStringConstExpressions(parsedGoFiles []*ast.File) map[string]ast.Expr {
	stringConstExpressions := map[string]ast.Expr{}
	for _, parsedFile := range parsedGoFiles {
		for _, declaration := range parsedFile.Decls {
			constDeclaration, isConstDeclaration := declaration.(*ast.GenDecl)
			if !isConstDeclaration || constDeclaration.Tok != token.CONST {
				continue
			}
			for _, specNode := range constDeclaration.Specs {
				valueSpec, isValueSpec := specNode.(*ast.ValueSpec)
				if !isValueSpec || len(valueSpec.Values) == 0 {
					continue
				}

				if len(valueSpec.Values) == 1 {
					for _, constName := range valueSpec.Names {
						stringConstExpressions[constName.Name] = valueSpec.Values[0]
					}
					continue
				}

				if len(valueSpec.Values) != len(valueSpec.Names) {
					continue
				}
				for valueIndex, constName := range valueSpec.Names {
					stringConstExpressions[constName.Name] = valueSpec.Values[valueIndex]
				}
			}
		}
	}
	return stringConstExpressions
}

type goStringConstResolver struct {
	stringConstExpressions map[string]ast.Expr
	resolvedValues         map[string]string
	currentlyResolving     map[string]struct{}
}

func newGoStringConstResolver(
	stringConstExpressions map[string]ast.Expr,
) *goStringConstResolver {
	return &goStringConstResolver{
		stringConstExpressions: stringConstExpressions,
		resolvedValues:         map[string]string{},
		currentlyResolving:     map[string]struct{}{},
	}
}

func (resolver *goStringConstResolver) Resolve(expression ast.Expr) (string, bool) {
	switch typedExpression := expression.(type) {
	case *ast.BasicLit:
		if typedExpression.Kind != token.STRING {
			return "", false
		}
		unquotedValue, err := strconv.Unquote(typedExpression.Value)
		if err != nil {
			return "", false
		}
		return unquotedValue, true
	case *ast.Ident:
		return resolver.resolveConstIdentifier(typedExpression.Name)
	case *ast.ParenExpr:
		return resolver.Resolve(typedExpression.X)
	case *ast.BinaryExpr:
		if typedExpression.Op != token.ADD {
			return "", false
		}
		leftValue, leftIsString := resolver.Resolve(typedExpression.X)
		if !leftIsString {
			return "", false
		}
		rightValue, rightIsString := resolver.Resolve(typedExpression.Y)
		if !rightIsString {
			return "", false
		}
		return leftValue + rightValue, true
	default:
		return "", false
	}
}

func (resolver *goStringConstResolver) resolveConstIdentifier(
	constIdentifier string,
) (string, bool) {
	if cachedValue, hasCachedValue := resolver.resolvedValues[constIdentifier]; hasCachedValue {
		return cachedValue, true
	}
	if _, isResolving := resolver.currentlyResolving[constIdentifier]; isResolving {
		return "", false
	}

	constExpression, hasConstExpression := resolver.stringConstExpressions[constIdentifier]
	if !hasConstExpression {
		return "", false
	}

	resolver.currentlyResolving[constIdentifier] = struct{}{}
	defer delete(resolver.currentlyResolving, constIdentifier)

	resolvedValue, ok := resolver.Resolve(constExpression)
	if !ok {
		return "", false
	}
	resolver.resolvedValues[constIdentifier] = resolvedValue
	return resolvedValue, true
}

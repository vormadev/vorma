package vormabuild

import (
	"bytes"
	"fmt"
	"go/ast"
	gobuild "go/build"
	goimporter "go/importer"
	"go/parser"
	"go/token"
	"go/types"
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
	expandPattern      func(string) ([]string, error)
	statPath           func(string) (fs.FileInfo, error)
	readFile           func(string) ([]byte, error)
	parseGoSourceAST   func(*token.FileSet, string, any, parser.Mode) (*ast.File, error)
	importGoPackageDir func(string) (*gobuild.Package, error)
}

var backendRouteDiscoveryDeps = backendRouteDiscoveryDependencies{
	expandPattern: func(pattern string) ([]string, error) {
		return doublestar.FilepathGlob(pattern)
	},
	statPath:         os.Stat,
	readFile:         os.ReadFile,
	parseGoSourceAST: parser.ParseFile,
	importGoPackageDir: func(packageDir string) (*gobuild.Package, error) {
		return gobuild.Default.ImportDir(packageDir, 0)
	},
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
	goFileSet                          *token.FileSet
	files                              []*parsedServerRouteFile
	rootFilePathSet                    map[string]struct{}
	stringConstResolver                *goStringConstResolver
	localFunctionsByName               map[string][]*localFunctionDeclaration
	localFunctionsByObject             map[*ast.Object]*localFunctionDeclaration
	packageLevelIdentifierNames        map[string]struct{}
	packageDir                         string
	packageName                        string
	filesByPath                        map[string]*parsedServerRouteFile
	goTypesInfo                        *types.Info
	goTypesPackagePath                 string
	goTypesInfoInitializationAttempted bool
	goTypesInitializationCount         int
	functionScopeRanges                []tokenPosRange
}

type tokenPosRange struct {
	start token.Pos
	end   token.Pos
}

type backendRouteDiscoveryState struct {
	loaderPatternSet                    map[string]struct{}
	activeFunctions                     map[*ast.FuncDecl]struct{}
	discoveredVormaRegistrationCallByID map[string]*discoveredVormaRegistrationCall
}

type expressionBindings struct {
	values map[string]ast.Expr
	parent *expressionBindings
}

type canonicalRouteRegistrationCall struct {
	isLoader                        bool
	pattern                         string
	discoveredVormaRegistrationCall *discoveredVormaRegistrationCall
}

type discoveredVormaRegistrationCall struct {
	isLoader              bool
	appExpression         ast.Expr
	methodExpression      ast.Expr
	patternExpression     ast.Expr
	handlerExpression     ast.Expr
	decorateCtxExpression ast.Expr
	sourcePosition        token.Pos
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
		absoluteRouteDefinitionFile, err := filepath.Abs(serverRouteDefinitionFile)
		if err != nil {
			return nil, fmt.Errorf(
				"resolve absolute server route definition file %q: %w",
				serverRouteDefinitionFile,
				err,
			)
		}

		parsedAST, err := parseServerRouteDefinitionFile(
			goFileSet,
			absoluteRouteDefinitionFile,
		)
		if err != nil {
			return nil, err
		}

		parsedServerFile := parseServerRouteFileMetadata(
			absoluteRouteDefinitionFile,
			parsedAST,
		)
		packageKey := deriveServerRoutePackageAnalysisKey(parsedServerFile)
		packageAnalysis, hasPackageAnalysis := analysesByPackageKey[packageKey]
		if !hasPackageAnalysis {
			packageAnalysis = &backendRoutePackageAnalysis{
				goFileSet:                   goFileSet,
				rootFilePathSet:             map[string]struct{}{},
				localFunctionsByName:        map[string][]*localFunctionDeclaration{},
				localFunctionsByObject:      map[*ast.Object]*localFunctionDeclaration{},
				packageLevelIdentifierNames: map[string]struct{}{},
				packageDir:                  filepath.ToSlash(filepath.Dir(parsedServerFile.path)),
				packageName:                 parsedServerFile.parsedAST.Name.Name,
				filesByPath:                 map[string]*parsedServerRouteFile{},
			}
			analysesByPackageKey[packageKey] = packageAnalysis
		}
		packageAnalysis.files = append(packageAnalysis.files, parsedServerFile)
		packageAnalysis.filesByPath[parsedServerFile.path] = parsedServerFile
		packageAnalysis.rootFilePathSet[parsedServerFile.path] = struct{}{}
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
	containsRouteRegistrationHints, err := analysis.packageContainsRouteRegistrationHints()
	if err != nil {
		return err
	}
	if !containsRouteRegistrationHints {
		return nil
	}

	if err := analysis.expandAndFilterFilesForCompiledPackage(); err != nil {
		return err
	}

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
			analysis.collectPackageLevelIdentifierNamesFromDeclaration(declaration)

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

	analysis.functionScopeRanges = collectFunctionScopeRangesFromParsedFiles(analysis.files)

	return nil
}

var routeRegistrationHintNeedles = [][]byte{
	[]byte("NewLoader"),
	[]byte("NewAction"),
	[]byte("RegisterNestedTaskHandler"),
	[]byte("RegisterTaskHandler"),
}

func (analysis *backendRoutePackageAnalysis) packageContainsRouteRegistrationHints() (bool, error) {
	packageDirEntries, readDirErr := os.ReadDir(analysis.packageDir)
	if readDirErr != nil {
		return false, fmt.Errorf(
			"read package directory %q for route registration hint scan: %w",
			analysis.packageDir,
			readDirErr,
		)
	}

	for _, packageDirEntry := range packageDirEntries {
		if packageDirEntry.IsDir() {
			continue
		}
		entryName := packageDirEntry.Name()
		if filepath.Ext(entryName) != ".go" || strings.HasSuffix(entryName, "_test.go") {
			continue
		}
		entryFilePath := filepath.ToSlash(filepath.Clean(filepath.Join(analysis.packageDir, entryName)))
		entryFileBytes, readErr := backendRouteDiscoveryDeps.readFile(entryFilePath)
		if readErr != nil {
			return false, fmt.Errorf(
				"read file %q for route registration hint scan: %w",
				entryFilePath,
				readErr,
			)
		}
		for _, routeRegistrationHintNeedle := range routeRegistrationHintNeedles {
			if bytes.Contains(entryFileBytes, routeRegistrationHintNeedle) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (analysis *backendRoutePackageAnalysis) expandAndFilterFilesForCompiledPackage() error {
	compiledFilePathSet, err := analysis.resolveCompiledFilePathSetForPackageDir()
	if err != nil {
		return err
	}

	filteredFiles := make([]*parsedServerRouteFile, 0, len(analysis.files))
	filteredFilesByPath := map[string]*parsedServerRouteFile{}
	filteredRootFilePathSet := map[string]struct{}{}

	for _, parsedServerFile := range analysis.files {
		if len(compiledFilePathSet) > 0 {
			if _, isCompiledFile := compiledFilePathSet[parsedServerFile.path]; !isCompiledFile {
				continue
			}
		}
		filteredFiles = append(filteredFiles, parsedServerFile)
		filteredFilesByPath[parsedServerFile.path] = parsedServerFile
		if _, isRootFile := analysis.rootFilePathSet[parsedServerFile.path]; isRootFile {
			filteredRootFilePathSet[parsedServerFile.path] = struct{}{}
		}
	}

	analysis.files = filteredFiles
	analysis.filesByPath = filteredFilesByPath
	analysis.rootFilePathSet = filteredRootFilePathSet

	if len(compiledFilePathSet) == 0 {
		return nil
	}

	for compiledFilePath := range compiledFilePathSet {
		if _, hasParsedFile := analysis.filesByPath[compiledFilePath]; hasParsedFile {
			continue
		}
		parsedAST, err := parseServerRouteDefinitionFile(analysis.goFileSet, compiledFilePath)
		if err != nil {
			return err
		}
		parsedServerFile := parseServerRouteFileMetadata(compiledFilePath, parsedAST)
		analysis.files = append(analysis.files, parsedServerFile)
		analysis.filesByPath[parsedServerFile.path] = parsedServerFile
	}

	return nil
}

func (analysis *backendRoutePackageAnalysis) resolveCompiledFilePathSetForPackageDir() (map[string]struct{}, error) {
	importedPackage, err := backendRouteDiscoveryDeps.importGoPackageDir(analysis.packageDir)
	if err != nil {
		if _, isNoGoError := err.(*gobuild.NoGoError); isNoGoError {
			return map[string]struct{}{}, nil
		}
		return nil, fmt.Errorf(
			"resolve compiled Go files for package directory %q: %w",
			analysis.packageDir,
			err,
		)
	}

	compiledFilePathSet := map[string]struct{}{}
	compiledGoFiles := append([]string{}, importedPackage.GoFiles...)
	compiledGoFiles = append(compiledGoFiles, importedPackage.CgoFiles...)
	for _, compiledGoFile := range compiledGoFiles {
		compiledFilePath := filepath.ToSlash(filepath.Clean(filepath.Join(importedPackage.Dir, compiledGoFile)))
		compiledFilePathSet[compiledFilePath] = struct{}{}
	}
	return compiledFilePathSet, nil
}

func (analysis *backendRoutePackageAnalysis) initializeGoTypesInfo() error {
	analysis.goTypesInitializationCount++

	if len(analysis.files) == 0 {
		return nil
	}

	parsedGoFiles := make([]*ast.File, 0, len(analysis.files))
	for _, parsedServerFile := range analysis.files {
		parsedGoFiles = append(parsedGoFiles, parsedServerFile.parsedAST)
	}

	goTypesInfo := &types.Info{
		Uses:       map[*ast.Ident]types.Object{},
		Defs:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	goTypesConfig := &types.Config{
		// Use export-data importer for speed. We only need best-effort symbol
		// identity and fall back when type info is incomplete.
		Importer: goimporter.Default(),
	}
	checkedPackage, _ := goTypesConfig.Check(
		analysis.packageDir,
		analysis.goFileSet,
		parsedGoFiles,
		goTypesInfo,
	)
	analysis.goTypesInfo = goTypesInfo
	if checkedPackage != nil {
		analysis.goTypesPackagePath = checkedPackage.Path()
	}
	return nil
}

func (analysis *backendRoutePackageAnalysis) ensureGoTypesInfoInitialized() error {
	if analysis.goTypesInfo != nil || analysis.goTypesInfoInitializationAttempted {
		return nil
	}
	analysis.goTypesInfoInitializationAttempted = true
	return analysis.initializeGoTypesInfo()
}

func (analysis *backendRoutePackageAnalysis) collectPackageLevelIdentifierNamesFromDeclaration(
	declaration ast.Decl,
) {
	switch typedDeclaration := declaration.(type) {
	case *ast.FuncDecl:
		if typedDeclaration.Recv != nil || typedDeclaration.Name == nil {
			return
		}
		analysis.packageLevelIdentifierNames[typedDeclaration.Name.Name] = struct{}{}
	case *ast.GenDecl:
		for _, declarationSpec := range typedDeclaration.Specs {
			switch typedDeclarationSpec := declarationSpec.(type) {
			case *ast.ValueSpec:
				for _, declaredIdentifier := range typedDeclarationSpec.Names {
					if declaredIdentifier == nil {
						continue
					}
					analysis.packageLevelIdentifierNames[declaredIdentifier.Name] = struct{}{}
				}
			case *ast.TypeSpec:
				if typedDeclarationSpec.Name == nil {
					continue
				}
				analysis.packageLevelIdentifierNames[typedDeclarationSpec.Name.Name] = struct{}{}
			}
		}
	}
}

func (analysis *backendRoutePackageAnalysis) isKnownPackageLevelIdentifierName(
	identifierName string,
) bool {
	if analysis == nil || analysis.packageLevelIdentifierNames == nil {
		return false
	}
	_, hasIdentifier := analysis.packageLevelIdentifierNames[identifierName]
	return hasIdentifier
}

func isPredeclaredUnqualifiedIdentifierName(identifierName string) bool {
	if identifierName == "" {
		return false
	}
	return types.Universe.Lookup(identifierName) != nil
}

func collectFunctionScopeRangesFromParsedFiles(
	parsedServerFiles []*parsedServerRouteFile,
) []tokenPosRange {
	functionScopeRanges := make([]tokenPosRange, 0)
	for _, parsedServerFile := range parsedServerFiles {
		ast.Inspect(parsedServerFile.parsedAST, func(currentNode ast.Node) bool {
			if currentNode == nil {
				return true
			}
			switch typedNode := currentNode.(type) {
			case *ast.FuncDecl:
				if typedNode.Body == nil {
					return true
				}
				functionScopeRanges = append(functionScopeRanges, tokenPosRange{
					start: typedNode.Body.Pos(),
					end:   typedNode.Body.End(),
				})
			case *ast.FuncLit:
				if typedNode.Body == nil {
					return true
				}
				functionScopeRanges = append(functionScopeRanges, tokenPosRange{
					start: typedNode.Body.Pos(),
					end:   typedNode.Body.End(),
				})
			}
			return true
		})
	}
	return functionScopeRanges
}

func (analysis *backendRoutePackageAnalysis) isPositionInFunctionScope(position token.Pos) bool {
	if position == token.NoPos {
		return false
	}
	for _, functionScopeRange := range analysis.functionScopeRanges {
		if position >= functionScopeRange.start && position <= functionScopeRange.end {
			return true
		}
	}
	return false
}

func (analysis *backendRoutePackageAnalysis) discoveryRootFiles() []*parsedServerRouteFile {
	rootFiles := make([]*parsedServerRouteFile, 0, len(analysis.rootFilePathSet))
	for _, parsedServerFile := range analysis.files {
		if _, isRootFile := analysis.rootFilePathSet[parsedServerFile.path]; !isRootFile {
			continue
		}
		rootFiles = append(rootFiles, parsedServerFile)
	}
	sort.Slice(rootFiles, func(i int, j int) bool {
		return rootFiles[i].path < rootFiles[j].path
	})
	return rootFiles
}

func (analysis *backendRoutePackageAnalysis) discoverLoaderPatterns() ([]string, error) {
	state := &backendRouteDiscoveryState{
		loaderPatternSet:                    map[string]struct{}{},
		activeFunctions:                     map[*ast.FuncDecl]struct{}{},
		discoveredVormaRegistrationCallByID: map[string]*discoveredVormaRegistrationCall{},
	}

	if err := analysis.discoverRouteRegistrationsInRootFiles(state); err != nil {
		return nil, err
	}

	loaderPatterns := make([]string, 0, len(state.loaderPatternSet))
	for loaderPattern := range state.loaderPatternSet {
		loaderPatterns = append(loaderPatterns, loaderPattern)
	}
	sort.Strings(loaderPatterns)
	return loaderPatterns, nil
}

func (analysis *backendRoutePackageAnalysis) discoverRouteRegistrationsInRootFiles(
	state *backendRouteDiscoveryState,
) error {
	for _, parsedServerFile := range analysis.discoveryRootFiles() {
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
					return err
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
					return err
				}
			}
		}
	}
	return nil
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
		if canonicalCall.discoveredVormaRegistrationCall != nil {
			callID, err := analysis.discoveredVormaRegistrationCallID(
				canonicalCall.discoveredVormaRegistrationCall,
			)
			if err != nil {
				return analysis.withPositionError(call.Pos(), err)
			}
			state.discoveredVormaRegistrationCallByID[callID] = canonicalCall.discoveredVormaRegistrationCall
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
		calleeExpression := unwrapGenericCalleeExpression(call.Fun)
		functionLiteral, isFunctionLiteral := calleeExpression.(*ast.FuncLit)
		if !isFunctionLiteral {
			return nil
		}
		childBindings := bindCallArgumentsToParameterNames(
			functionLiteral.Type,
			call.Args,
			bindings,
		)
		return analysis.discoverLoaderPatternsFromFunctionLiteral(
			functionLiteral,
			parsedServerFile,
			childBindings,
			state,
		)
	}

	childBindings := bindCallArgumentsToParameterNames(
		localFunctionDeclaration.decl.Type,
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
		if isImportAlias && importAlias.Obj == nil {
			importPath, hasImportAlias := parsedServerFile.importAliases[importAlias.Name]
			if hasImportAlias {
				registrationCall, isRegistrationCall, err := analysis.parseCanonicalRouteRegistrationCallByImportPath(
					call,
					parsedServerFile,
					bindings,
					importPath,
					typedCallee.Sel.Name,
				)
				if err != nil {
					return nil, false, err
				}
				if isRegistrationCall {
					return registrationCall, true, nil
				}
			}
		}
	case *ast.Ident:
		if typedCallee.Obj == nil {
			for importPath := range parsedServerFile.dotImportPaths {
				registrationCall, isRegistrationCall, err := analysis.parseCanonicalRouteRegistrationCallByImportPath(
					call,
					parsedServerFile,
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
		}
	default:
		// Continue to fallback resolution when canonical AST import matching is unavailable.
	}

	if !callExpressionCouldReferenceCanonicalRouteRegistrationByName(call) {
		return nil, false, nil
	}

	if goTypesRegistrationCall, isRegistrationCall, err := analysis.parseCanonicalRouteRegistrationCallByGoTypesIdentity(
		call,
		parsedServerFile,
		bindings,
	); err != nil {
		return nil, false, err
	} else if isRegistrationCall {
		return goTypesRegistrationCall, true, nil
	}

	return nil, false, nil
}

var canonicalRouteRegistrationFunctionNames = map[string]struct{}{
	"NewLoader":                 {},
	"NewAction":                 {},
	"RegisterNestedTaskHandler": {},
	"RegisterTaskHandler":       {},
}

func callExpressionCouldReferenceCanonicalRouteRegistrationByName(
	call *ast.CallExpr,
) bool {
	if call == nil {
		return false
	}

	calleeExpression := unwrapGenericCalleeExpression(call.Fun)
	switch typedCallee := calleeExpression.(type) {
	case *ast.SelectorExpr:
		_, isCanonicalName := canonicalRouteRegistrationFunctionNames[typedCallee.Sel.Name]
		return isCanonicalName
	case *ast.Ident:
		_, isCanonicalName := canonicalRouteRegistrationFunctionNames[typedCallee.Name]
		return isCanonicalName
	default:
		return false
	}
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalRouteRegistrationCallByGoTypesIdentity(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
) (*canonicalRouteRegistrationCall, bool, error) {
	if err := analysis.ensureGoTypesInfoInitialized(); err != nil {
		return nil, false, err
	}
	if analysis.goTypesInfo == nil {
		return nil, false, nil
	}

	importPath, functionName, hasGoTypesIdentity := analysis.resolveGoTypesFunctionIdentityForCall(call)
	if !hasGoTypesIdentity {
		return nil, false, nil
	}
	return analysis.parseCanonicalRouteRegistrationCallByImportPath(
		call,
		parsedServerFile,
		bindings,
		importPath,
		functionName,
	)
}

func (analysis *backendRoutePackageAnalysis) resolveGoTypesFunctionIdentityForCall(
	call *ast.CallExpr,
) (string, string, bool) {
	if analysis.goTypesInfo == nil {
		return "", "", false
	}

	calleeExpression := unwrapGenericCalleeExpression(call.Fun)
	switch typedCallee := calleeExpression.(type) {
	case *ast.Ident:
		functionObject, hasFunctionObject := analysis.goTypesInfo.Uses[typedCallee]
		if !hasFunctionObject || functionObject == nil {
			return "", "", false
		}
		return qualifiedGoTypesObjectIdentity(functionObject)
	case *ast.SelectorExpr:
		functionObject, hasFunctionObject := analysis.goTypesInfo.Uses[typedCallee.Sel]
		if !hasFunctionObject || functionObject == nil {
			return "", "", false
		}
		return qualifiedGoTypesObjectIdentity(functionObject)
	default:
		return "", "", false
	}
}

func qualifiedGoTypesObjectIdentity(functionObject types.Object) (string, string, bool) {
	if functionObject == nil {
		return "", "", false
	}

	functionPackage := functionObject.Pkg()
	if functionPackage == nil {
		return "", "", false
	}
	return functionPackage.Path(), functionObject.Name(), true
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalRouteRegistrationCallByImportPath(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
	importPath string,
	functionName string,
) (*canonicalRouteRegistrationCall, bool, error) {
	switch {
	case importPath == "github.com/vormadev/vorma" && functionName == "NewLoader":
		return analysis.parseCanonicalVormaLoaderRegistrationCall(
			call,
			parsedServerFile,
			bindings,
		)
	case importPath == "github.com/vormadev/vorma" && functionName == "NewAction":
		return analysis.parseCanonicalVormaActionRegistrationCall(
			call,
			parsedServerFile,
			bindings,
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

func (analysis *backendRoutePackageAnalysis) parseCanonicalVormaLoaderRegistrationCall(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
) (*canonicalRouteRegistrationCall, bool, error) {
	_ = parsedServerFile
	canonicalCall, isCanonicalCall, err := analysis.parseCanonicalLoaderRegistrationCall(
		call,
		bindings,
		"github.com/vormadev/vorma.NewLoader",
		1,
	)
	if err != nil || !isCanonicalCall {
		return canonicalCall, isCanonicalCall, err
	}
	if len(call.Args) < 4 {
		return nil, false, fmt.Errorf(
			"github.com/vormadev/vorma.NewLoader requires app, pattern, handler, and decorateCtx arguments",
		)
	}

	appExpression := resolveExpressionWithBindingsForDiscoveredRegistration(call.Args[0], bindings, 0)
	handlerExpression := resolveExpressionWithBindingsForDiscoveredRegistration(call.Args[2], bindings, 0)
	decorateCtxExpression := resolveExpressionWithBindingsForDiscoveredRegistration(call.Args[3], bindings, 0)

	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		appExpression,
		"github.com/vormadev/vorma.NewLoader app argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}
	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		handlerExpression,
		"github.com/vormadev/vorma.NewLoader handler argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}
	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		decorateCtxExpression,
		"github.com/vormadev/vorma.NewLoader decorateCtx argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}

	canonicalCall.discoveredVormaRegistrationCall = &discoveredVormaRegistrationCall{
		isLoader:              true,
		appExpression:         appExpression,
		patternExpression:     quotedStringExpression(canonicalCall.pattern),
		handlerExpression:     handlerExpression,
		decorateCtxExpression: decorateCtxExpression,
		sourcePosition:        call.Pos(),
	}
	return canonicalCall, true, nil
}

func (analysis *backendRoutePackageAnalysis) parseCanonicalVormaActionRegistrationCall(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
) (*canonicalRouteRegistrationCall, bool, error) {
	canonicalCall, isCanonicalCall, err := analysis.parseCanonicalActionRegistrationCall(
		call,
		bindings,
		"github.com/vormadev/vorma.NewAction",
		1,
		2,
	)
	if err != nil || !isCanonicalCall {
		return canonicalCall, isCanonicalCall, err
	}
	if len(call.Args) < 5 {
		return nil, false, fmt.Errorf(
			"github.com/vormadev/vorma.NewAction requires app, method, pattern, handler, and decorateCtx arguments",
		)
	}

	resolvedMethod, isMethod := resolveCompileTimeStringExpression(
		call.Args[1],
		bindings,
		analysis.stringConstResolver,
		0,
	)
	if !isMethod {
		return nil, false, fmt.Errorf(
			"github.com/vormadev/vorma.NewAction method argument must resolve to compile-time string",
		)
	}
	resolvedPattern, isPattern := resolveCompileTimeStringExpression(
		call.Args[2],
		bindings,
		analysis.stringConstResolver,
		0,
	)
	if !isPattern {
		return nil, false, fmt.Errorf(
			"github.com/vormadev/vorma.NewAction pattern argument must resolve to compile-time string",
		)
	}

	appExpression := resolveExpressionWithBindingsForDiscoveredRegistration(call.Args[0], bindings, 0)
	handlerExpression := resolveExpressionWithBindingsForDiscoveredRegistration(call.Args[3], bindings, 0)
	decorateCtxExpression := resolveExpressionWithBindingsForDiscoveredRegistration(call.Args[4], bindings, 0)

	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		appExpression,
		"github.com/vormadev/vorma.NewAction app argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}
	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		handlerExpression,
		"github.com/vormadev/vorma.NewAction handler argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}
	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		decorateCtxExpression,
		"github.com/vormadev/vorma.NewAction decorateCtx argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}

	canonicalCall.discoveredVormaRegistrationCall = &discoveredVormaRegistrationCall{
		isLoader:              false,
		appExpression:         appExpression,
		methodExpression:      quotedStringExpression(resolvedMethod),
		patternExpression:     quotedStringExpression(resolvedPattern),
		handlerExpression:     handlerExpression,
		decorateCtxExpression: decorateCtxExpression,
		sourcePosition:        call.Pos(),
	}
	_ = parsedServerFile
	return canonicalCall, true, nil
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

func bindCallArgumentsToParameterNames(
	functionType *ast.FuncType,
	callArguments []ast.Expr,
	parentBindings *expressionBindings,
) *expressionBindings {
	if functionType == nil || functionType.Params == nil {
		return &expressionBindings{values: map[string]ast.Expr{}, parent: parentBindings}
	}

	parameterNames := make([]string, 0)
	for _, parameterField := range functionType.Params.List {
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

func (analysis *backendRoutePackageAnalysis) discoverLoaderPatternsFromFunctionLiteral(
	functionLiteral *ast.FuncLit,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
	state *backendRouteDiscoveryState,
) error {
	if functionLiteral == nil || functionLiteral.Body == nil {
		return nil
	}

	for _, statement := range functionLiteral.Body.List {
		if err := walkCallExpressionsInNode(statement, func(call *ast.CallExpr) error {
			return analysis.analyzeCallExpression(
				call,
				parsedServerFile,
				bindings,
				state,
			)
		}); err != nil {
			return err
		}
	}
	return nil
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

func quotedStringExpression(value string) ast.Expr {
	return &ast.BasicLit{
		Kind:  token.STRING,
		Value: strconv.Quote(value),
	}
}

func resolveExpressionWithBindingsForDiscoveredRegistration(
	expression ast.Expr,
	bindings *expressionBindings,
	depth int,
) ast.Expr {
	if depth > 64 {
		return expression
	}
	switch typedExpression := expression.(type) {
	case *ast.Ident:
		if bindings != nil {
			if boundExpression, hasBoundExpression := bindings.resolveBoundExpression(typedExpression.Name); hasBoundExpression {
				return resolveExpressionWithBindingsForDiscoveredRegistration(
					boundExpression,
					bindings,
					depth+1,
				)
			}
		}
		return typedExpression
	default:
		return typedExpression
	}
}

func (analysis *backendRoutePackageAnalysis) validateDiscoveredRegistrationExpressionForPackageInitScope(
	expression ast.Expr,
	argumentDisplayName string,
	callPosition token.Pos,
) error {
	expressionStart := expression.Pos()
	expressionEnd := expression.End()

	var validationError error
	ast.Inspect(expression, func(currentNode ast.Node) bool {
		if validationError != nil {
			return false
		}
		identifier, isIdentifier := currentNode.(*ast.Ident)
		if !isIdentifier {
			return true
		}
		if identifier.Obj == nil {
			return true
		}

		identifierDefinitionPosition := identifier.Obj.Pos()
		if identifierDefinitionPosition == token.NoPos {
			return true
		}
		// Symbols declared inside the emitted expression itself are safe.
		if expressionStart != token.NoPos && expressionEnd != token.NoPos {
			if identifierDefinitionPosition >= expressionStart && identifierDefinitionPosition <= expressionEnd {
				return true
			}
		}
		if !analysis.isPositionInFunctionScope(identifierDefinitionPosition) {
			return true
		}

		validationError = analysis.withPositionError(
			callPosition,
			fmt.Errorf(
				"%s references function-local symbol %q which cannot be emitted at package init scope",
				argumentDisplayName,
				identifier.Name,
			),
		)
		return false
	})
	if validationError != nil {
		return validationError
	}
	return nil
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

// Package registrationgraph owns route-registration graph traversal and
// canonical registration-call analysis for vormabuild backend route discovery.
//
// This package isolates the AST/go-types engine that resolves init-reachable
// registrations, compile-time route strings, and generated registrar source so
// backendroutes orchestration can stay focused on top-level build flow.
package registrationgraph

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/printer"
	"go/token"
	"go/types"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/vormadev/vorma/vormabuild/internal/backendroutes/sourceparse"
)

type RouteRegistrarSource struct {
	PackageDir        string
	PackageImportPath string
	SourceBytes       []byte
}
type parsedServerRouteFile struct {
	path           string
	parsedAST      *ast.File
	importAliases  map[string]string
	dotImportPaths map[string]struct{}
}

type localFunctionDeclaration struct {
	name          string
	decl          *ast.FuncDecl
	file          *parsedServerRouteFile
	ownerAnalysis *PackageAnalysis
}

type PackageAnalysis struct {
	goFileSet                           *token.FileSet
	files                               []*parsedServerRouteFile
	rootFilePathSet                     map[string]struct{}
	stringConstResolver                 *sourceparse.GoStringConstResolver
	localFunctionsByName                map[string][]*localFunctionDeclaration
	localFunctionsByDeclarationPosition map[token.Pos]*localFunctionDeclaration
	packageLevelIdentifierNames         map[string]struct{}
	packageDir                          string
	packageName                         string
	packageImportPath                   string
	filesByPath                         map[string]*parsedServerRouteFile
	allFilesByPath                      map[string]*parsedServerRouteFile
	packageAnalysesByImportPath         map[string]*PackageAnalysis
	goTypesInfo                         *types.Info
	goTypesPackagePath                  string
	goTypesInfoInitializationAttempted  bool
	goTypesInitializationCount          int
	functionScopeRanges                 []tokenPosRange
	dependencies                        sourceparse.Dependencies
}

type tokenPosRange struct {
	start token.Pos
	end   token.Pos
}

type backendRouteDiscoveryState struct {
	loaderPatternSet                    map[string]struct{}
	activeFunctions                     map[*ast.FuncDecl]struct{}
	discoveredVormaRegistrationCallByID map[string]*discoveredVormaRegistrationCall
	rootAnalysis                        *PackageAnalysis
}

type expressionBindings struct {
	values map[string]ast.Expr
	parent *expressionBindings
}

type canonicalRouteRegistrationCall struct {
	isLoader                        bool
	method                          string
	pattern                         string
	discoveredVormaRegistrationCall *discoveredVormaRegistrationCall
}

type discoveredVormaRegistrationCall struct {
	isLoader                         bool
	appExpression                    ast.Expr
	methodExpression                 ast.Expr
	patternExpression                ast.Expr
	handlerExpression                ast.Expr
	decorateCtxExpression            ast.Expr
	registerDiscoveredTaskExpression ast.Expr
	sourcePosition                   token.Pos
}

// ParseServerRouteFilesIntoPackageAnalyses parses matched route-definition
// source files into package-scoped analyses ready for registration discovery.
func ParseServerRouteFilesIntoPackageAnalyses(
	serverRouteDefinitionFiles []string,
	dependencies sourceparse.Dependencies,
) ([]*PackageAnalysis, error) {
	dependencies = sourceparse.NormalizeDependencies(dependencies)

	goFileSet := token.NewFileSet()
	analysesByPackageKey := map[string]*PackageAnalysis{}

	for _, serverRouteDefinitionFile := range serverRouteDefinitionFiles {
		absoluteRouteDefinitionFile, err := filepath.Abs(
			serverRouteDefinitionFile,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"resolve absolute server route definition file %q: %w",
				serverRouteDefinitionFile,
				err,
			)
		}

		parsedAST, err := parseServerRouteDefinitionFileWithDependencies(
			goFileSet,
			absoluteRouteDefinitionFile,
			dependencies,
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
			packageAnalysis = &PackageAnalysis{
				goFileSet:                           goFileSet,
				rootFilePathSet:                     map[string]struct{}{},
				localFunctionsByName:                map[string][]*localFunctionDeclaration{},
				localFunctionsByDeclarationPosition: map[token.Pos]*localFunctionDeclaration{},
				packageLevelIdentifierNames:         map[string]struct{}{},
				packageDir: filepath.ToSlash(
					filepath.Dir(parsedServerFile.path),
				),
				packageName:  parsedServerFile.parsedAST.Name.Name,
				filesByPath:  map[string]*parsedServerRouteFile{},
				dependencies: dependencies,
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

	packageAnalyses := make([]*PackageAnalysis, 0, len(packageKeys))
	for _, packageKey := range packageKeys {
		packageAnalysis := analysesByPackageKey[packageKey]
		if err := packageAnalysis.initialize(); err != nil {
			return nil, err
		}
		packageAnalyses = append(packageAnalyses, packageAnalysis)
	}

	allFilesByPath := map[string]*parsedServerRouteFile{}
	for _, packageAnalysis := range packageAnalyses {
		for filePath, parsedServerFile := range packageAnalysis.filesByPath {
			allFilesByPath[filePath] = parsedServerFile
		}
	}

	packageAnalysesByImportPath := map[string]*PackageAnalysis{}
	for _, packageAnalysis := range packageAnalyses {
		if packageAnalysis.packageImportPath == "" ||
			packageAnalysis.packageImportPath == "." ||
			strings.HasPrefix(packageAnalysis.packageImportPath, "./") ||
			strings.HasPrefix(packageAnalysis.packageImportPath, "../") {
			continue
		}
		existingAnalysis, hasExistingAnalysis := packageAnalysesByImportPath[packageAnalysis.packageImportPath]
		if hasExistingAnalysis && existingAnalysis != packageAnalysis {
			continue
		}
		packageAnalysesByImportPath[packageAnalysis.packageImportPath] = packageAnalysis
	}
	for _, packageAnalysis := range packageAnalyses {
		packageAnalysis.allFilesByPath = allFilesByPath
		packageAnalysis.packageAnalysesByImportPath = packageAnalysesByImportPath
	}

	return packageAnalyses, nil
}
func parseServerRouteFileMetadata(
	serverRouteDefinitionFile string,
	parsedAST *ast.File,
) *parsedServerRouteFile {
	parsedServerFile := &parsedServerRouteFile{
		path: filepath.ToSlash(
			filepath.Clean(serverRouteDefinitionFile),
		),
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
			defaultAlias := path.Base(importPath)
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
	return filepath.ToSlash(
		filepath.Dir(parsedServerFile.path),
	) + "|" + parsedServerFile.parsedAST.Name.Name
}

func (analysis *PackageAnalysis) initialize() error {
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
	analysis.stringConstResolver = sourceparse.NewGoStringConstResolver(
		sourceparse.CollectPackageStringConstExpressions(parsedGoFiles),
	)

	for _, parsedServerFile := range analysis.files {
		for _, declaration := range parsedServerFile.parsedAST.Decls {
			analysis.collectPackageLevelIdentifierNamesFromDeclaration(
				declaration,
			)

			functionDeclaration, isFunctionDeclaration := declaration.(*ast.FuncDecl)
			if !isFunctionDeclaration || functionDeclaration.Recv != nil {
				continue
			}
			localFunction := &localFunctionDeclaration{
				name:          functionDeclaration.Name.Name,
				decl:          functionDeclaration,
				file:          parsedServerFile,
				ownerAnalysis: analysis,
			}
			analysis.localFunctionsByName[localFunction.name] = append(
				analysis.localFunctionsByName[localFunction.name],
				localFunction,
			)
			functionDeclarationPosition := token.NoPos
			if functionDeclaration.Name != nil {
				functionDeclarationPosition = functionDeclaration.Name.Pos()
			}
			if functionDeclarationPosition != token.NoPos {
				analysis.localFunctionsByDeclarationPosition[functionDeclarationPosition] = localFunction
			}
		}
	}

	analysis.functionScopeRanges = collectFunctionScopeRangesFromParsedFiles(
		analysis.files,
	)

	return nil
}

func (analysis *PackageAnalysis) expandAndFilterFilesForCompiledPackage() error {
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
		parsedAST, err := parseServerRouteDefinitionFileWithDependencies(
			analysis.goFileSet,
			compiledFilePath,
			analysis.dependencies,
		)
		if err != nil {
			return err
		}
		parsedServerFile := parseServerRouteFileMetadata(
			compiledFilePath,
			parsedAST,
		)
		analysis.files = append(analysis.files, parsedServerFile)
		analysis.filesByPath[parsedServerFile.path] = parsedServerFile
	}

	return nil
}

func (analysis *PackageAnalysis) resolveCompiledFilePathSetForPackageDir() (map[string]struct{}, error) {
	importedPackage, err := analysis.dependencies.ImportGoPackageDir(
		analysis.packageDir,
	)
	if err != nil {
		if _, isNoGoError := err.(*build.NoGoError); isNoGoError {
			return map[string]struct{}{}, nil
		}
		return nil, fmt.Errorf(
			"resolve compiled Go files for package directory %q: %w",
			analysis.packageDir,
			err,
		)
	}

	compiledFilePathSet := map[string]struct{}{}
	analysis.packageImportPath = strings.TrimSpace(importedPackage.ImportPath)
	if analysis.packageImportPath == "" || analysis.packageImportPath == "." {
		derivedImportPath, hasDerivedImportPath, err := analysis.deriveModuleImportPathForPackageDir()
		if err != nil {
			return nil, err
		}
		if hasDerivedImportPath {
			analysis.packageImportPath = derivedImportPath
		}
	}
	compiledGoFiles := append([]string{}, importedPackage.GoFiles...)
	compiledGoFiles = append(compiledGoFiles, importedPackage.CgoFiles...)
	for _, compiledGoFile := range compiledGoFiles {
		compiledFilePath := filepath.ToSlash(
			filepath.Clean(filepath.Join(importedPackage.Dir, compiledGoFile)),
		)
		compiledFilePathSet[compiledFilePath] = struct{}{}
	}
	return compiledFilePathSet, nil
}

func (analysis *PackageAnalysis) deriveModuleImportPathForPackageDir() (string, bool, error) {
	normalizedPackageDir := filepath.ToSlash(
		filepath.Clean(analysis.packageDir),
	)
	currentDir := normalizedPackageDir
	for {
		goModFilePath := filepath.ToSlash(
			filepath.Clean(filepath.Join(currentDir, "go.mod")),
		)
		goModFileBytes, readErr := analysis.dependencies.ReadFile(goModFilePath)
		if readErr == nil {
			modulePath, hasModulePath := parseModulePathFromGoModBytes(
				goModFileBytes,
			)
			if !hasModulePath {
				return "", false, nil
			}

			relativePackagePath, err := filepath.Rel(
				currentDir,
				normalizedPackageDir,
			)
			if err != nil {
				return "", false, fmt.Errorf(
					"resolve package import path relative to module root %q for package directory %q: %w",
					currentDir,
					normalizedPackageDir,
					err,
				)
			}
			normalizedRelativePackagePath := filepath.ToSlash(
				filepath.Clean(relativePackagePath),
			)
			if normalizedRelativePackagePath == "." {
				return modulePath, true, nil
			}
			return strings.TrimSuffix(
				modulePath,
				"/",
			) + "/" + strings.TrimPrefix(
				normalizedRelativePackagePath,
				"./",
			), true, nil
		}

		parentDir := filepath.ToSlash(filepath.Clean(filepath.Dir(currentDir)))
		if parentDir == currentDir {
			break
		}
		currentDir = parentDir
	}
	return "", false, nil
}

func parseModulePathFromGoModBytes(goModFileBytes []byte) (string, bool) {
	for _, rawLine := range strings.Split(string(goModFileBytes), "\n") {
		trimmedLine := strings.TrimSpace(rawLine)
		fields := strings.Fields(trimmedLine)
		if len(fields) < 2 || fields[0] != "module" {
			continue
		}
		return strings.TrimSpace(fields[1]), true
	}
	return "", false
}

func (analysis *PackageAnalysis) initializeGoTypesInfo() error {
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
		Importer: importer.Default(),
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

func (analysis *PackageAnalysis) ensureGoTypesInfoInitialized() error {
	if analysis.goTypesInfo != nil ||
		analysis.goTypesInfoInitializationAttempted {
		return nil
	}
	analysis.goTypesInfoInitializationAttempted = true
	return analysis.initializeGoTypesInfo()
}

func (analysis *PackageAnalysis) collectPackageLevelIdentifierNamesFromDeclaration(
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

func (analysis *PackageAnalysis) isKnownPackageLevelIdentifierName(
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
		ast.Inspect(
			parsedServerFile.parsedAST,
			func(currentNode ast.Node) bool {
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
			},
		)
	}
	return functionScopeRanges
}

func (analysis *PackageAnalysis) isPositionInFunctionScope(
	position token.Pos,
) bool {
	if position == token.NoPos {
		return false
	}
	for _, functionScopeRange := range analysis.functionScopeRanges {
		if position >= functionScopeRange.start &&
			position <= functionScopeRange.end {
			return true
		}
	}
	return false
}

func (analysis *PackageAnalysis) discoveryRootFiles() []*parsedServerRouteFile {
	rootFiles := make(
		[]*parsedServerRouteFile,
		0,
		len(analysis.rootFilePathSet),
	)
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

func (analysis *PackageAnalysis) DiscoverLoaderPatterns() ([]string, error) {
	state := &backendRouteDiscoveryState{
		loaderPatternSet:                    map[string]struct{}{},
		activeFunctions:                     map[*ast.FuncDecl]struct{}{},
		discoveredVormaRegistrationCallByID: map[string]*discoveredVormaRegistrationCall{},
		rootAnalysis:                        analysis,
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

func (analysis *PackageAnalysis) discoverRouteRegistrationsInRootFiles(
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
						name:          "init",
						decl:          typedDeclaration,
						file:          parsedServerFile,
						ownerAnalysis: analysis,
					},
					nil,
					state,
					nil,
				); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (analysis *PackageAnalysis) discoverLoaderPatternsFromVarDeclaration(
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
						nil,
					)
				},
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (analysis *PackageAnalysis) analyzeCallExpression(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
	state *backendRouteDiscoveryState,
	discoveredTaskExpression ast.Expr,
) error {
	if discoveredTaskExpression == nil {
		discoveredTaskExpression = call
	}

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
			discoveredCall := canonicalCall.discoveredVormaRegistrationCall
			if state.rootAnalysis != nil && analysis != state.rootAnalysis {
				discoveredCall = &discoveredVormaRegistrationCall{
					isLoader:      canonicalCall.isLoader,
					appExpression: canonicalCall.discoveredVormaRegistrationCall.appExpression,
					patternExpression: quotedStringExpression(
						canonicalCall.pattern,
					),
					registerDiscoveredTaskExpression: discoveredTaskExpression,
					sourcePosition:                   discoveredTaskExpression.Pos(),
				}
				if !canonicalCall.isLoader {
					discoveredCall.methodExpression = quotedStringExpression(
						canonicalCall.method,
					)
				}
				if err := state.rootAnalysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
					discoveredCall.appExpression,
					"discovered registration app argument",
					discoveredCall.sourcePosition,
				); err != nil {
					return err
				}
				if err := state.rootAnalysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
					discoveredCall.registerDiscoveredTaskExpression,
					"discovered registration task argument",
					discoveredCall.sourcePosition,
				); err != nil {
					return err
				}
			}

			callID, err := state.rootAnalysis.discoveredVormaRegistrationCallID(
				discoveredCall,
			)
			if err != nil {
				return err
			}
			state.discoveredVormaRegistrationCallByID[callID] = discoveredCall
		}
		return nil
	}

	localFunctionDeclaration, hasLocalFunction, err := analysis.resolveLocalFunctionDeclarationForCall(
		call,
		parsedServerFile,
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
			discoveredTaskExpression,
		)
	}

	childBindings := bindCallArgumentsToParameterNames(
		localFunctionDeclaration.decl.Type,
		call.Args,
		bindings,
	)
	return localFunctionDeclaration.ownerAnalysis.discoverLoaderPatternsFromFunctionDeclaration(
		localFunctionDeclaration,
		childBindings,
		state,
		discoveredTaskExpression,
	)
}

func (analysis *PackageAnalysis) parseCanonicalRouteRegistrationCall(
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

func (analysis *PackageAnalysis) parseCanonicalRouteRegistrationCallByGoTypesIdentity(
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

	importPath, functionName, hasGoTypesIdentity := analysis.resolveGoTypesFunctionIdentityForCall(
		call,
	)
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

func (analysis *PackageAnalysis) resolveGoTypesFunctionIdentityForCall(
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

func qualifiedGoTypesObjectIdentity(
	functionObject types.Object,
) (string, string, bool) {
	if functionObject == nil {
		return "", "", false
	}

	functionPackage := functionObject.Pkg()
	if functionPackage == nil {
		return "", "", false
	}
	return functionPackage.Path(), functionObject.Name(), true
}

func (analysis *PackageAnalysis) parseCanonicalRouteRegistrationCallByImportPath(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
	importPath string,
	functionName string,
) (*canonicalRouteRegistrationCall, bool, error) {
	switch {
	case importPath == "github.com/vormadev/vorma" && functionName == "DefineLoaderForRegistration":
		return analysis.parseCanonicalVormaLoaderRegistrationCall(
			call,
			parsedServerFile,
			bindings,
		)
	case importPath == "github.com/vormadev/vorma" && functionName == "DefineActionForRegistration":
		return analysis.parseCanonicalVormaActionRegistrationCall(
			call,
			parsedServerFile,
			bindings,
		)
	default:
		return nil, false, nil
	}
}

func (analysis *PackageAnalysis) parseCanonicalVormaLoaderRegistrationCall(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
) (*canonicalRouteRegistrationCall, bool, error) {
	_ = parsedServerFile
	canonicalCall, isCanonicalCall, err := analysis.parseCanonicalLoaderRegistrationCall(
		call,
		bindings,
		"github.com/vormadev/vorma.DefineLoaderForRegistration",
		1,
	)
	if err != nil || !isCanonicalCall {
		return canonicalCall, isCanonicalCall, err
	}
	if len(call.Args) < 4 {
		return nil, false, fmt.Errorf(
			"github.com/vormadev/vorma.DefineLoaderForRegistration requires app, pattern, handler, and decorateCtx arguments",
		)
	}

	appExpression := resolveExpressionWithBindingsForDiscoveredRegistration(
		call.Args[0],
		bindings,
		0,
	)
	handlerExpression := resolveExpressionWithBindingsForDiscoveredRegistration(
		call.Args[2],
		bindings,
		0,
	)
	decorateCtxExpression := resolveExpressionWithBindingsForDiscoveredRegistration(
		call.Args[3],
		bindings,
		0,
	)

	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		appExpression,
		"github.com/vormadev/vorma.DefineLoaderForRegistration app argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}
	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		handlerExpression,
		"github.com/vormadev/vorma.DefineLoaderForRegistration handler argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}
	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		decorateCtxExpression,
		"github.com/vormadev/vorma.DefineLoaderForRegistration decorateCtx argument",
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

func (analysis *PackageAnalysis) parseCanonicalVormaActionRegistrationCall(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
) (*canonicalRouteRegistrationCall, bool, error) {
	canonicalCall, isCanonicalCall, err := analysis.parseCanonicalActionRegistrationCall(
		call,
		bindings,
		"github.com/vormadev/vorma.DefineActionForRegistration",
		1,
		2,
	)
	if err != nil || !isCanonicalCall {
		return canonicalCall, isCanonicalCall, err
	}
	if len(call.Args) < 5 {
		return nil, false, fmt.Errorf(
			"github.com/vormadev/vorma.DefineActionForRegistration requires app, method, pattern, handler, and decorateCtx arguments",
		)
	}

	appExpression := resolveExpressionWithBindingsForDiscoveredRegistration(
		call.Args[0],
		bindings,
		0,
	)
	handlerExpression := resolveExpressionWithBindingsForDiscoveredRegistration(
		call.Args[3],
		bindings,
		0,
	)
	decorateCtxExpression := resolveExpressionWithBindingsForDiscoveredRegistration(
		call.Args[4],
		bindings,
		0,
	)

	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		appExpression,
		"github.com/vormadev/vorma.DefineActionForRegistration app argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}
	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		handlerExpression,
		"github.com/vormadev/vorma.DefineActionForRegistration handler argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}
	if err := analysis.validateDiscoveredRegistrationExpressionForPackageInitScope(
		decorateCtxExpression,
		"github.com/vormadev/vorma.DefineActionForRegistration decorateCtx argument",
		call.Pos(),
	); err != nil {
		return nil, false, err
	}

	canonicalCall.discoveredVormaRegistrationCall = &discoveredVormaRegistrationCall{
		isLoader:              false,
		appExpression:         appExpression,
		methodExpression:      quotedStringExpression(canonicalCall.method),
		patternExpression:     quotedStringExpression(canonicalCall.pattern),
		handlerExpression:     handlerExpression,
		decorateCtxExpression: decorateCtxExpression,
		sourcePosition:        call.Pos(),
	}
	_ = parsedServerFile
	return canonicalCall, true, nil
}

func (analysis *PackageAnalysis) parseCanonicalLoaderRegistrationCall(
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

func (analysis *PackageAnalysis) parseCanonicalActionRegistrationCall(
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

	actionMethod, isCompileTimeMethod := resolveCompileTimeStringExpression(
		call.Args[methodArgumentIndex],
		bindings,
		analysis.stringConstResolver,
		0,
	)
	if !isCompileTimeMethod {
		return nil, false, fmt.Errorf(
			"%s method argument must resolve to compile-time string",
			calleeDisplayName,
		)
	}

	actionPattern, isCompileTimePattern := resolveCompileTimeStringExpression(
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
		isLoader: false,
		method:   actionMethod,
		pattern:  actionPattern,
	}, true, nil
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

func (analysis *PackageAnalysis) resolveLocalFunctionDeclarationForCall(
	call *ast.CallExpr,
	parsedServerFile *parsedServerRouteFile,
	position token.Pos,
) (*localFunctionDeclaration, bool, error) {
	calleeExpression := unwrapGenericCalleeExpression(call.Fun)
	switch typedCallee := calleeExpression.(type) {
	case *ast.SelectorExpr:
		importAlias, isImportAlias := typedCallee.X.(*ast.Ident)
		if !isImportAlias || importAlias.Obj != nil {
			return nil, false, nil
		}
		importPath, hasImportAlias := parsedServerFile.importAliases[importAlias.Name]
		if !hasImportAlias {
			return nil, false, nil
		}
		return analysis.resolveImportedFunctionDeclarationByImportPath(
			importPath,
			typedCallee.Sel.Name,
			position,
		)

	case *ast.Ident:
		if typedCallee.Obj != nil {
			if typedCallee.Obj.Kind != ast.Fun {
				return nil, false, nil
			}
			functionDeclaration, isFunctionDeclaration := typedCallee.Obj.Decl.(*ast.FuncDecl)
			if !isFunctionDeclaration || functionDeclaration.Name == nil {
				return nil, false, nil
			}
			functionDeclarationPosition := functionDeclaration.Name.Pos()
			if functionDeclarationPosition == token.NoPos {
				return nil, false, nil
			}
			localFunctionDeclaration, hasLocalFunction := analysis.localFunctionsByDeclarationPosition[functionDeclarationPosition]
			if !hasLocalFunction {
				return nil, false, nil
			}
			return localFunctionDeclaration, true, nil
		}

		localFunctionDeclarations := analysis.localFunctionsByName[typedCallee.Name]
		switch len(localFunctionDeclarations) {
		case 0:
		case 1:
			return localFunctionDeclarations[0], true, nil
		default:
			return nil, false, analysis.withPositionError(
				position,
				fmt.Errorf(
					"ambiguous local function %q in route discovery; choose unique function name",
					typedCallee.Name,
				),
			)
		}

		resolvedDeclaration := (*localFunctionDeclaration)(nil)
		for importPath := range parsedServerFile.dotImportPaths {
			candidateDeclaration, hasCandidate, err := analysis.resolveImportedFunctionDeclarationByImportPath(
				importPath,
				typedCallee.Name,
				position,
			)
			if err != nil {
				return nil, false, err
			}
			if !hasCandidate {
				continue
			}
			if resolvedDeclaration != nil &&
				resolvedDeclaration != candidateDeclaration {
				return nil, false, analysis.withPositionError(
					position,
					fmt.Errorf(
						"ambiguous dot-imported function %q in route discovery; avoid dot imports for helpers",
						typedCallee.Name,
					),
				)
			}
			resolvedDeclaration = candidateDeclaration
		}
		if resolvedDeclaration != nil {
			return resolvedDeclaration, true, nil
		}
		return nil, false, nil

	default:
		return nil, false, nil
	}
}

func (analysis *PackageAnalysis) resolveImportedFunctionDeclarationByImportPath(
	importPath string,
	functionName string,
	position token.Pos,
) (*localFunctionDeclaration, bool, error) {
	packageAnalysis, hasImportedPackageAnalysis := analysis.packageAnalysesByImportPath[importPath]
	if !hasImportedPackageAnalysis || packageAnalysis == nil {
		return nil, false, nil
	}

	functionDeclarations := packageAnalysis.localFunctionsByName[functionName]
	switch len(functionDeclarations) {
	case 0:
		return nil, false, nil
	case 1:
		return functionDeclarations[0], true, nil
	default:
		return nil, false, analysis.withPositionError(
			position,
			fmt.Errorf(
				"ambiguous imported function %q from %q in route discovery; choose unique function name",
				functionName,
				importPath,
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
		return &expressionBindings{
			values: map[string]ast.Expr{},
			parent: parentBindings,
		}
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

func (analysis *PackageAnalysis) discoverLoaderPatternsFromFunctionLiteral(
	functionLiteral *ast.FuncLit,
	parsedServerFile *parsedServerRouteFile,
	bindings *expressionBindings,
	state *backendRouteDiscoveryState,
	discoveredTaskExpression ast.Expr,
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
				discoveredTaskExpression,
			)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (analysis *PackageAnalysis) discoverLoaderPatternsFromFunctionDeclaration(
	localFunction *localFunctionDeclaration,
	bindings *expressionBindings,
	state *backendRouteDiscoveryState,
	discoveredTaskExpression ast.Expr,
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
				discoveredTaskExpression,
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
	stringConstResolver *sourceparse.GoStringConstResolver,
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
		return stringConstResolver.ResolveConstIdentifier(
			typedExpression.Name,
		)
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

func (analysis *PackageAnalysis) validateDiscoveredRegistrationExpressionForPackageInitScope(
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
			if identifierDefinitionPosition >= expressionStart &&
				identifierDefinitionPosition <= expressionEnd {
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
		if boundIdentifier, isBoundIdentifier := boundExpression.(*ast.Ident); isBoundIdentifier &&
			boundIdentifier.Name == expressionName {
			continue
		}
		return boundExpression, true
	}
	return nil, false
}

func (analysis *PackageAnalysis) withPositionError(
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
	return fmt.Errorf(
		"%s:%d: %w",
		filepath.ToSlash(filePosition.Filename),
		lineNumber,
		err,
	)
}

func (analysis *PackageAnalysis) discoverVormaRegistrationCalls() ([]*discoveredVormaRegistrationCall, error) {
	state := &backendRouteDiscoveryState{
		loaderPatternSet:                    map[string]struct{}{},
		activeFunctions:                     map[*ast.FuncDecl]struct{}{},
		discoveredVormaRegistrationCallByID: map[string]*discoveredVormaRegistrationCall{},
		rootAnalysis:                        analysis,
	}

	if err := analysis.discoverRouteRegistrationsInRootFiles(state); err != nil {
		return nil, err
	}

	callIDs := make([]string, 0, len(state.discoveredVormaRegistrationCallByID))
	for callID := range state.discoveredVormaRegistrationCallByID {
		callIDs = append(callIDs, callID)
	}
	sort.Strings(callIDs)

	discoveredCalls := make([]*discoveredVormaRegistrationCall, 0, len(callIDs))
	for _, callID := range callIDs {
		discoveredCalls = append(
			discoveredCalls,
			state.discoveredVormaRegistrationCallByID[callID],
		)
	}
	return discoveredCalls, nil
}

func (analysis *PackageAnalysis) renderDiscoveredRouteRegistrarSource(
	discoveredCalls []*discoveredVormaRegistrationCall,
) ([]byte, error) {
	requiredImports := map[string]string{}
	for _, discoveredCall := range discoveredCalls {
		if err := analysis.collectRequiredImportsForDiscoveredCall(discoveredCall, requiredImports); err != nil {
			return nil, err
		}
	}

	importAliases := make([]string, 0, len(requiredImports))
	for importAlias := range requiredImports {
		importAliases = append(importAliases, importAlias)
	}
	sort.Strings(importAliases)

	var sb strings.Builder
	sb.WriteString("// Code generated by vormabuild. DO NOT EDIT.\n")
	sb.WriteString("package " + analysis.packageName + "\n\n")
	sb.WriteString("import (\n")
	for _, importAlias := range importAliases {
		sb.WriteString(
			fmt.Sprintf("\t%s %q\n", importAlias, requiredImports[importAlias]),
		)
	}
	sb.WriteString(")\n\n")
	sb.WriteString("func init() {\n")
	for _, discoveredCall := range discoveredCalls {
		callExpression, err := analysis.renderDiscoveredCallExpression(
			discoveredCall,
		)
		if err != nil {
			return nil, err
		}
		// Call discovered registration entrypoints directly in init. Some
		// registration helpers return tasks while others return no value.
		// Wrapping calls in blank-identifier assignments breaks no-return
		// helpers at compile time.
		sb.WriteString("\t")
		sb.WriteString(callExpression)
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	return []byte(sb.String()), nil
}

func (analysis *PackageAnalysis) renderDiscoveredCallExpression(
	discoveredCall *discoveredVormaRegistrationCall,
) (string, error) {
	appExpression, err := analysis.formatDiscoveredCallExpression(
		discoveredCall.appExpression,
	)
	if err != nil {
		return "", err
	}
	patternExpression, err := analysis.formatDiscoveredCallExpression(
		discoveredCall.patternExpression,
	)
	if err != nil {
		return "", err
	}

	if discoveredCall.registerDiscoveredTaskExpression != nil {
		taskExpression, err := analysis.formatDiscoveredCallExpression(
			discoveredCall.registerDiscoveredTaskExpression,
		)
		if err != nil {
			return "", err
		}
		if discoveredCall.isLoader {
			return fmt.Sprintf(
				"vorma.RegisterDiscoveredLoaderTask(%s, %s, %s)",
				appExpression,
				patternExpression,
				taskExpression,
			), nil
		}

		methodExpression, err := analysis.formatDiscoveredCallExpression(
			discoveredCall.methodExpression,
		)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(
			"vorma.RegisterDiscoveredActionTask(%s, %s, %s, %s)",
			appExpression,
			methodExpression,
			patternExpression,
			taskExpression,
		), nil
	}

	handlerExpression, err := analysis.formatDiscoveredCallExpression(
		discoveredCall.handlerExpression,
	)
	if err != nil {
		return "", err
	}
	decorateCtxExpression, err := analysis.formatDiscoveredCallExpression(
		discoveredCall.decorateCtxExpression,
	)
	if err != nil {
		return "", err
	}

	if discoveredCall.isLoader {
		return fmt.Sprintf(
			"vormagogen.RegisterLoaderDiscoveredByBuild(%s, %s, %s, %s)",
			appExpression,
			patternExpression,
			handlerExpression,
			decorateCtxExpression,
		), nil
	}

	methodExpression, err := analysis.formatDiscoveredCallExpression(
		discoveredCall.methodExpression,
	)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"vormagogen.RegisterActionDiscoveredByBuild(%s, %s, %s, %s, %s)",
		appExpression,
		methodExpression,
		patternExpression,
		handlerExpression,
		decorateCtxExpression,
	), nil
}

func (analysis *PackageAnalysis) formatDiscoveredCallExpression(
	expression ast.Expr,
) (string, error) {
	var expressionBuffer bytes.Buffer
	if err := printer.Fprint(&expressionBuffer, analysis.goFileSet, expression); err != nil {
		return "", fmt.Errorf(
			"format discovered registration expression: %w",
			err,
		)
	}
	return strings.TrimSpace(expressionBuffer.String()), nil
}

func (analysis *PackageAnalysis) discoveredVormaRegistrationCallID(
	discoveredCall *discoveredVormaRegistrationCall,
) (string, error) {
	if discoveredCall == nil {
		return "", fmt.Errorf("discovered registration call is nil")
	}

	renderedExpression, err := analysis.renderDiscoveredCallExpression(
		discoveredCall,
	)
	if err != nil {
		return "", err
	}

	positionKey := fmt.Sprintf("pos:%d", discoveredCall.sourcePosition)
	if analysis != nil && analysis.goFileSet != nil &&
		discoveredCall.sourcePosition != token.NoPos {
		sourceFilePosition := analysis.goFileSet.Position(
			discoveredCall.sourcePosition,
		)
		if sourceFilePosition.Filename != "" {
			lineNumber := sourceFilePosition.Line
			if lineNumber <= 0 {
				lineNumber = 1
			}
			columnNumber := sourceFilePosition.Column
			if columnNumber <= 0 {
				columnNumber = 1
			}
			positionKey = fmt.Sprintf(
				"%s:%d:%d",
				filepath.ToSlash(sourceFilePosition.Filename),
				lineNumber,
				columnNumber,
			)
		}
	}

	return positionKey + "|" + renderedExpression, nil
}

func (analysis *PackageAnalysis) collectRequiredImportsForDiscoveredCall(
	discoveredCall *discoveredVormaRegistrationCall,
	requiredImports map[string]string,
) error {
	if discoveredCall.registerDiscoveredTaskExpression != nil {
		requiredImports["vorma"] = "github.com/vormadev/vorma"
		callExpressions := []ast.Expr{
			discoveredCall.appExpression,
			discoveredCall.patternExpression,
			discoveredCall.registerDiscoveredTaskExpression,
		}
		if !discoveredCall.isLoader {
			callExpressions = append(
				callExpressions,
				discoveredCall.methodExpression,
			)
		}
		for _, callExpression := range callExpressions {
			if callExpression == nil {
				continue
			}
			if err := analysis.collectRequiredImportsForExpression(callExpression, requiredImports); err != nil {
				return err
			}
		}
		return nil
	}

	requiredImports["vormagogen"] = "github.com/vormadev/vorma/vormagogen"
	callExpressions := []ast.Expr{
		discoveredCall.appExpression,
		discoveredCall.patternExpression,
		discoveredCall.handlerExpression,
		discoveredCall.decorateCtxExpression,
	}
	if !discoveredCall.isLoader {
		callExpressions = append(
			callExpressions,
			discoveredCall.methodExpression,
		)
	}

	for _, callExpression := range callExpressions {
		if callExpression == nil {
			continue
		}
		if err := analysis.collectRequiredImportsForExpression(callExpression, requiredImports); err != nil {
			return err
		}
	}
	return nil
}

func (analysis *PackageAnalysis) collectRequiredImportsForExpression(
	expression ast.Expr,
	requiredImports map[string]string,
) error {
	var importCollectionErr error
	goTypesInitializationChecked := false
	selectorExpressionIdentifierSet := map[*ast.Ident]struct{}{}
	ast.Inspect(expression, func(currentNode ast.Node) bool {
		if importCollectionErr != nil {
			return false
		}
		switch typedNode := currentNode.(type) {
		case *ast.SelectorExpr:
			selectorExpressionIdentifierSet[typedNode.Sel] = struct{}{}
			importAlias, isImportAlias := typedNode.X.(*ast.Ident)
			if isImportAlias {
				selectorExpressionIdentifierSet[importAlias] = struct{}{}
			}
			if !isImportAlias || importAlias.Obj != nil {
				return true
			}

			importPath, hasImportPath := analysis.resolveImportPathForAliasAtPosition(
				importAlias.Name,
				typedNode.Pos(),
			)
			if !hasImportPath {
				return true
			}

			existingImportPath, hasExistingImportAlias := requiredImports[importAlias.Name]
			if hasExistingImportAlias && existingImportPath != importPath {
				importCollectionErr = fmt.Errorf(
					"conflicting import alias %q for discovered route registrar generation: %q vs %q",
					importAlias.Name,
					existingImportPath,
					importPath,
				)
				return false
			}
			requiredImports[importAlias.Name] = importPath
			return true
		case *ast.Ident:
			if _, isSelectorExpressionIdentifier := selectorExpressionIdentifierSet[typedNode]; isSelectorExpressionIdentifier {
				return true
			}
			if typedNode.Obj != nil {
				return true
			}
			if analysis.isKnownPackageLevelIdentifierName(typedNode.Name) {
				return true
			}
			if isPredeclaredUnqualifiedIdentifierName(typedNode.Name) {
				return true
			}
			if !goTypesInitializationChecked {
				goTypesInitializationChecked = true
				if err := analysis.ensureGoTypesInfoInitialized(); err != nil {
					importCollectionErr = err
					return false
				}
			}
			if analysis.goTypesInfo == nil {
				return true
			}

			usedObject, hasUsedObject := analysis.goTypesInfo.Uses[typedNode]
			if !hasUsedObject || usedObject == nil {
				return true
			}
			usedObjectPackage := usedObject.Pkg()
			if usedObjectPackage == nil {
				return true
			}
			if analysis.goTypesPackagePath != "" && usedObjectPackage.Path() == analysis.goTypesPackagePath {
				return true
			}

			importCollectionErr = fmt.Errorf(
				"discovered registration expression references unqualified external symbol %q from %q; use explicit import alias qualification instead of dot-imported symbols",
				typedNode.Name,
				usedObjectPackage.Path(),
			)
			return false
		default:
			return true
		}
	})
	if importCollectionErr != nil {
		return importCollectionErr
	}
	return nil
}

func (analysis *PackageAnalysis) resolveImportPathForAliasAtPosition(
	importAlias string,
	position token.Pos,
) (string, bool) {
	parsedServerFile, hasParsedServerFile := analysis.getParsedServerRouteFileForPosition(
		position,
	)
	if !hasParsedServerFile {
		return "", false
	}
	importPath, hasImportAlias := parsedServerFile.importAliases[importAlias]
	if !hasImportAlias {
		return "", false
	}
	return importPath, true
}

func (analysis *PackageAnalysis) getParsedServerRouteFileForPosition(
	position token.Pos,
) (*parsedServerRouteFile, bool) {
	if position == token.NoPos {
		return nil, false
	}
	filePosition := analysis.goFileSet.Position(position)
	if filePosition.Filename == "" {
		return nil, false
	}
	filePath := filepath.ToSlash(filepath.Clean(filePosition.Filename))
	parsedServerFile, hasParsedServerFile := analysis.filesByPath[filePath]
	if !hasParsedServerFile {
		if analysis.allFilesByPath == nil {
			return nil, false
		}
		parsedServerFile, hasParsedServerFile = analysis.allFilesByPath[filePath]
		if !hasParsedServerFile {
			return nil, false
		}
	}
	return parsedServerFile, true
}

// DiscoverRouteRegistrarSource resolves canonical vorma registrations for one
// package analysis and renders generated init-registrar source.
func (analysis *PackageAnalysis) DiscoverRouteRegistrarSource() (*RouteRegistrarSource, error) {
	discoveredCalls, err := analysis.discoverVormaRegistrationCalls()
	if err != nil {
		return nil, err
	}
	if len(discoveredCalls) == 0 {
		return nil, nil
	}

	generatedSource, err := analysis.renderDiscoveredRouteRegistrarSource(
		discoveredCalls,
	)
	if err != nil {
		return nil, err
	}

	return &RouteRegistrarSource{
		PackageDir:        analysis.packageDir,
		PackageImportPath: analysis.packageImportPath,
		SourceBytes:       generatedSource,
	}, nil
}

func parseServerRouteDefinitionFileWithDependencies(
	goFileSet *token.FileSet,
	serverRouteDefinitionFile string,
	dependencies sourceparse.Dependencies,
) (*ast.File, error) {
	return sourceparse.ParseServerRouteDefinitionFile(
		goFileSet,
		serverRouteDefinitionFile,
		dependencies,
	)
}

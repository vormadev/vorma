package vormabuild

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/vormadev/vorma/vormaruntime"
)

const discoveredRouteRegistrarGeneratedFilename = "vorma_discovered_routes.gen.go"

type discoveredRouteRegistrarOverlay struct {
	goOverlayConfigPath   string
	cleanupTemporaryFiles func() error
}

func (overlay *discoveredRouteRegistrarOverlay) cleanup() error {
	if overlay == nil || overlay.cleanupTemporaryFiles == nil {
		return nil
	}
	return overlay.cleanupTemporaryFiles()
}

type backendRouteRegistrarOverlayDependencies struct {
	makeTempDir  func(string, string) (string, error)
	writeFile    func(string, []byte, os.FileMode) error
	readDir      func(string) ([]os.DirEntry, error)
	readFile     func(string) ([]byte, error)
	removeAll    func(string) error
	marshalJSON  func(any) ([]byte, error)
	absolutePath func(string) (string, error)
}

var backendRouteRegistrarOverlayDeps = backendRouteRegistrarOverlayDependencies{
	makeTempDir:  os.MkdirTemp,
	writeFile:    os.WriteFile,
	readDir:      os.ReadDir,
	readFile:     os.ReadFile,
	removeAll:    os.RemoveAll,
	marshalJSON:  json.Marshal,
	absolutePath: filepath.Abs,
}

type goOverlayReplaceConfiguration struct {
	Replace map[string]string `json:"Replace"`
}

type discoveredRouteRegistrarSourceArtifact struct {
	targetFilePath string
	sourceBytes    []byte
}

type discoveredRouteRegistrarArtifactCacheEntry struct {
	sourceFingerprint string
	artifacts         []discoveredRouteRegistrarSourceArtifact
}

type discoveredRouteRegistrarArtifactCache struct {
	mutex   sync.Mutex
	entries map[string]discoveredRouteRegistrarArtifactCacheEntry
}

var discoveredRouteRegistrarArtifactsCache = discoveredRouteRegistrarArtifactCache{
	entries: map[string]discoveredRouteRegistrarArtifactCacheEntry{},
}

func (cache *discoveredRouteRegistrarArtifactCache) get(
	cacheKey string,
	sourceFingerprint string,
) ([]discoveredRouteRegistrarSourceArtifact, bool) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cacheEntry, hasCacheEntry := cache.entries[cacheKey]
	if !hasCacheEntry {
		return nil, false
	}
	if cacheEntry.sourceFingerprint != sourceFingerprint {
		return nil, false
	}
	return cloneDiscoveredRouteRegistrarSourceArtifacts(cacheEntry.artifacts), true
}

func (cache *discoveredRouteRegistrarArtifactCache) set(
	cacheKey string,
	sourceFingerprint string,
	artifacts []discoveredRouteRegistrarSourceArtifact,
) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cache.entries[cacheKey] = discoveredRouteRegistrarArtifactCacheEntry{
		sourceFingerprint: sourceFingerprint,
		artifacts:         cloneDiscoveredRouteRegistrarSourceArtifacts(artifacts),
	}
}

func (cache *discoveredRouteRegistrarArtifactCache) clear() {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cache.entries = map[string]discoveredRouteRegistrarArtifactCacheEntry{}
}

func cloneDiscoveredRouteRegistrarSourceArtifacts(
	artifacts []discoveredRouteRegistrarSourceArtifact,
) []discoveredRouteRegistrarSourceArtifact {
	if len(artifacts) == 0 {
		return nil
	}

	clonedArtifacts := make([]discoveredRouteRegistrarSourceArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		clonedArtifacts = append(
			clonedArtifacts,
			discoveredRouteRegistrarSourceArtifact{
				targetFilePath: artifact.targetFilePath,
				sourceBytes:    append([]byte(nil), artifact.sourceBytes...),
			},
		)
	}
	return clonedArtifacts
}

func prepareDiscoveredRouteRegistrarOverlay(
	v *vormaruntime.Vorma,
) (*discoveredRouteRegistrarOverlay, error) {
	serverRouteDefinitionFiles, err := resolveServerRouteDefinitionFiles(v)
	if err != nil {
		return nil, err
	}
	if len(serverRouteDefinitionFiles) == 0 {
		return nil, nil
	}

	discoverySourceFingerprint, err := computeDiscoveredRouteRegistrarDiscoveryFingerprint(
		serverRouteDefinitionFiles,
	)
	if err != nil {
		return nil, err
	}

	discoveryCacheKey := discoveredRouteRegistrarDiscoveryCacheKey(v)
	discoveredRegistrarArtifacts, hasCachedArtifacts := discoveredRouteRegistrarArtifactsCache.get(
		discoveryCacheKey,
		discoverySourceFingerprint,
	)
	if !hasCachedArtifacts {
		discoveredRegistrarArtifacts, err = discoverRouteRegistrarSourceArtifacts(
			serverRouteDefinitionFiles,
		)
		if err != nil {
			return nil, err
		}
		discoveredRouteRegistrarArtifactsCache.set(
			discoveryCacheKey,
			discoverySourceFingerprint,
			discoveredRegistrarArtifacts,
		)
	}

	return writeDiscoveredRouteRegistrarOverlay(discoveredRegistrarArtifacts)
}

func discoveredRouteRegistrarDiscoveryCacheKey(v *vormaruntime.Vorma) string {
	if v == nil {
		return "<nil-vorma>"
	}
	return fmt.Sprintf("%p", v)
}

func computeDiscoveredRouteRegistrarDiscoveryFingerprint(
	serverRouteDefinitionFiles []string,
) (string, error) {
	normalizedRouteDefinitionFiles := make([]string, 0, len(serverRouteDefinitionFiles))
	packageDirSet := map[string]struct{}{}
	for _, serverRouteDefinitionFile := range serverRouteDefinitionFiles {
		absoluteRouteDefinitionFilePath, err := backendRouteRegistrarOverlayDeps.absolutePath(
			serverRouteDefinitionFile,
		)
		if err != nil {
			return "", fmt.Errorf(
				"resolve absolute server route definition file %q for discovered route registrar cache fingerprint: %w",
				serverRouteDefinitionFile,
				err,
			)
		}

		normalizedRouteDefinitionFile := filepath.ToSlash(filepath.Clean(absoluteRouteDefinitionFilePath))
		normalizedRouteDefinitionFiles = append(
			normalizedRouteDefinitionFiles,
			normalizedRouteDefinitionFile,
		)
		packageDirSet[filepath.ToSlash(filepath.Clean(filepath.Dir(normalizedRouteDefinitionFile)))] = struct{}{}
	}
	sort.Strings(normalizedRouteDefinitionFiles)

	packageDirs := make([]string, 0, len(packageDirSet))
	for packageDir := range packageDirSet {
		packageDirs = append(packageDirs, packageDir)
	}
	sort.Strings(packageDirs)

	discoveryFingerprintHasher := sha256.New()
	appendFingerprintSegment := func(value string) {
		_, _ = discoveryFingerprintHasher.Write([]byte(value))
		_, _ = discoveryFingerprintHasher.Write([]byte{0})
	}
	appendFingerprintBytes := func(value []byte) {
		_, _ = discoveryFingerprintHasher.Write(value)
		_, _ = discoveryFingerprintHasher.Write([]byte{0})
	}

	appendFingerprintSegment("server-route-definition-files")
	for _, routeDefinitionFile := range normalizedRouteDefinitionFiles {
		appendFingerprintSegment(routeDefinitionFile)
	}

	for _, packageDir := range packageDirs {
		appendFingerprintSegment("package-dir")
		appendFingerprintSegment(packageDir)

		packageDirEntries, err := backendRouteRegistrarOverlayDeps.readDir(packageDir)
		if err != nil {
			return "", fmt.Errorf(
				"read package directory %q for discovered route registrar cache fingerprint: %w",
				packageDir,
				err,
			)
		}

		packageGoFiles := make([]string, 0, len(packageDirEntries))
		for _, packageDirEntry := range packageDirEntries {
			if packageDirEntry.IsDir() {
				continue
			}

			entryName := packageDirEntry.Name()
			if filepath.Ext(entryName) != ".go" || strings.HasSuffix(entryName, "_test.go") {
				continue
			}

			packageGoFiles = append(packageGoFiles, entryName)
		}
		sort.Strings(packageGoFiles)

		for _, packageGoFile := range packageGoFiles {
			packageGoFilePath := filepath.ToSlash(filepath.Clean(filepath.Join(packageDir, packageGoFile)))
			packageGoFileBytes, err := backendRouteRegistrarOverlayDeps.readFile(packageGoFilePath)
			if err != nil {
				return "", fmt.Errorf(
					"read package Go source file %q for discovered route registrar cache fingerprint: %w",
					packageGoFilePath,
					err,
				)
			}

			appendFingerprintSegment(packageGoFilePath)
			appendFingerprintBytes(packageGoFileBytes)
		}
	}

	return hex.EncodeToString(discoveryFingerprintHasher.Sum(nil)), nil
}

func discoverRouteRegistrarSourceArtifacts(
	serverRouteDefinitionFiles []string,
) ([]discoveredRouteRegistrarSourceArtifact, error) {
	packageAnalyses, err := parseServerRouteFilesIntoPackageAnalyses(
		serverRouteDefinitionFiles,
	)
	if err != nil {
		return nil, err
	}

	discoveredRegistrarArtifacts := make([]discoveredRouteRegistrarSourceArtifact, 0)
	for _, packageAnalysis := range packageAnalyses {
		discoveredCalls, err := packageAnalysis.discoverVormaRegistrationCalls()
		if err != nil {
			return nil, err
		}
		if len(discoveredCalls) == 0 {
			continue
		}

		generatedSource, err := packageAnalysis.renderDiscoveredRouteRegistrarSource(discoveredCalls)
		if err != nil {
			return nil, err
		}

		generatedFilePath := filepath.Join(
			packageAnalysis.packageDir,
			discoveredRouteRegistrarGeneratedFilename,
		)
		absoluteGeneratedFilePath, err := backendRouteRegistrarOverlayDeps.absolutePath(generatedFilePath)
		if err != nil {
			return nil, fmt.Errorf(
				"resolve absolute generated backend route registrar path %q: %w",
				generatedFilePath,
				err,
			)
		}

		discoveredRegistrarArtifacts = append(
			discoveredRegistrarArtifacts,
			discoveredRouteRegistrarSourceArtifact{
				targetFilePath: filepath.ToSlash(filepath.Clean(absoluteGeneratedFilePath)),
				sourceBytes:    generatedSource,
			},
		)
	}
	sort.Slice(discoveredRegistrarArtifacts, func(i int, j int) bool {
		return discoveredRegistrarArtifacts[i].targetFilePath < discoveredRegistrarArtifacts[j].targetFilePath
	})
	return discoveredRegistrarArtifacts, nil
}

func writeDiscoveredRouteRegistrarOverlay(
	discoveredRegistrarArtifacts []discoveredRouteRegistrarSourceArtifact,
) (*discoveredRouteRegistrarOverlay, error) {
	if len(discoveredRegistrarArtifacts) == 0 {
		return nil, nil
	}

	overlayTempDir, err := backendRouteRegistrarOverlayDeps.makeTempDir(
		"",
		"vorma_discovered_route_registrars_*",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create temporary discovered route registrar overlay directory: %w",
			err,
		)
	}
	cleanupOverlayTempDir := func() error {
		return backendRouteRegistrarOverlayDeps.removeAll(overlayTempDir)
	}

	overlayReplacements := map[string]string{}
	for artifactIndex, discoveredArtifact := range discoveredRegistrarArtifacts {
		overlaySourceFilePath := filepath.ToSlash(filepath.Join(
			overlayTempDir,
			fmt.Sprintf("discovered_route_registrar_%d.gen.go", artifactIndex),
		))
		if err := backendRouteRegistrarOverlayDeps.writeFile(
			overlaySourceFilePath,
			discoveredArtifact.sourceBytes,
			0o644,
		); err != nil {
			if cleanupErr := cleanupOverlayTempDir(); cleanupErr != nil {
				return nil, fmt.Errorf(
					"write discovered route registrar overlay source %q: %w (cleanup overlay dir failed: %v)",
					overlaySourceFilePath,
					err,
					cleanupErr,
				)
			}
			return nil, fmt.Errorf(
				"write discovered route registrar overlay source %q: %w",
				overlaySourceFilePath,
				err,
			)
		}
		overlayReplacements[discoveredArtifact.targetFilePath] = overlaySourceFilePath
	}

	overlayConfigBytes, err := backendRouteRegistrarOverlayDeps.marshalJSON(
		goOverlayReplaceConfiguration{Replace: overlayReplacements},
	)
	if err != nil {
		if cleanupErr := cleanupOverlayTempDir(); cleanupErr != nil {
			return nil, fmt.Errorf(
				"marshal discovered route registrar go overlay config: %w (cleanup overlay dir failed: %v)",
				err,
				cleanupErr,
			)
		}
		return nil, fmt.Errorf(
			"marshal discovered route registrar go overlay config: %w",
			err,
		)
	}

	overlayConfigPath := filepath.ToSlash(filepath.Join(
		overlayTempDir,
		"go-overlay.json",
	))
	if err := backendRouteRegistrarOverlayDeps.writeFile(
		overlayConfigPath,
		overlayConfigBytes,
		0o644,
	); err != nil {
		if cleanupErr := cleanupOverlayTempDir(); cleanupErr != nil {
			return nil, fmt.Errorf(
				"write discovered route registrar go overlay config %q: %w (cleanup overlay dir failed: %v)",
				overlayConfigPath,
				err,
				cleanupErr,
			)
		}
		return nil, fmt.Errorf(
			"write discovered route registrar go overlay config %q: %w",
			overlayConfigPath,
			err,
		)
	}

	return &discoveredRouteRegistrarOverlay{
		goOverlayConfigPath: overlayConfigPath,
		cleanupTemporaryFiles: func() error {
			return cleanupOverlayTempDir()
		},
	}, nil
}

func (analysis *backendRoutePackageAnalysis) discoverVormaRegistrationCalls() ([]*discoveredVormaRegistrationCall, error) {
	state := &backendRouteDiscoveryState{
		loaderPatternSet:                    map[string]struct{}{},
		activeFunctions:                     map[*ast.FuncDecl]struct{}{},
		discoveredVormaRegistrationCallByID: map[string]*discoveredVormaRegistrationCall{},
	}

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

	callIDs := make([]string, 0, len(state.discoveredVormaRegistrationCallByID))
	for callID := range state.discoveredVormaRegistrationCallByID {
		callIDs = append(callIDs, callID)
	}
	sort.Strings(callIDs)

	discoveredCalls := make([]*discoveredVormaRegistrationCall, 0, len(callIDs))
	for _, callID := range callIDs {
		discoveredCalls = append(discoveredCalls, state.discoveredVormaRegistrationCallByID[callID])
	}
	return discoveredCalls, nil
}

func (analysis *backendRoutePackageAnalysis) renderDiscoveredRouteRegistrarSource(
	discoveredCalls []*discoveredVormaRegistrationCall,
) ([]byte, error) {
	requiredImports := map[string]string{
		"vorma": "github.com/vormadev/vorma",
	}
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
		sb.WriteString(fmt.Sprintf("\t%s %q\n", importAlias, requiredImports[importAlias]))
	}
	sb.WriteString(")\n\n")
	sb.WriteString("func init() {\n")
	for _, discoveredCall := range discoveredCalls {
		callExpression, err := analysis.renderDiscoveredCallExpression(discoveredCall)
		if err != nil {
			return nil, err
		}
		sb.WriteString("\t_ = ")
		sb.WriteString(callExpression)
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	return []byte(sb.String()), nil
}

func (analysis *backendRoutePackageAnalysis) renderDiscoveredCallExpression(
	discoveredCall *discoveredVormaRegistrationCall,
) (string, error) {
	appExpression, err := analysis.formatDiscoveredCallExpression(discoveredCall.appExpression)
	if err != nil {
		return "", err
	}
	patternExpression, err := analysis.formatDiscoveredCallExpression(discoveredCall.patternExpression)
	if err != nil {
		return "", err
	}
	handlerExpression, err := analysis.formatDiscoveredCallExpression(discoveredCall.handlerExpression)
	if err != nil {
		return "", err
	}
	decorateCtxExpression, err := analysis.formatDiscoveredCallExpression(discoveredCall.decorateCtxExpression)
	if err != nil {
		return "", err
	}

	if discoveredCall.isLoader {
		return fmt.Sprintf(
			"vorma.Internal__RegisterDiscoveredLoader(%s, %s, %s, %s)",
			appExpression,
			patternExpression,
			handlerExpression,
			decorateCtxExpression,
		), nil
	}

	methodExpression, err := analysis.formatDiscoveredCallExpression(discoveredCall.methodExpression)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"vorma.Internal__RegisterDiscoveredAction(%s, %s, %s, %s, %s)",
		appExpression,
		methodExpression,
		patternExpression,
		handlerExpression,
		decorateCtxExpression,
	), nil
}

func (analysis *backendRoutePackageAnalysis) formatDiscoveredCallExpression(
	expression ast.Expr,
) (string, error) {
	var expressionBuffer bytes.Buffer
	if err := printer.Fprint(&expressionBuffer, analysis.goFileSet, expression); err != nil {
		return "", fmt.Errorf("format discovered registration expression: %w", err)
	}
	return strings.TrimSpace(expressionBuffer.String()), nil
}

func (analysis *backendRoutePackageAnalysis) discoveredVormaRegistrationCallID(
	discoveredCall *discoveredVormaRegistrationCall,
) (string, error) {
	renderedExpression, err := analysis.renderDiscoveredCallExpression(discoveredCall)
	if err != nil {
		return "", err
	}
	return renderedExpression, nil
}

func (analysis *backendRoutePackageAnalysis) collectRequiredImportsForDiscoveredCall(
	discoveredCall *discoveredVormaRegistrationCall,
	requiredImports map[string]string,
) error {
	callExpressions := []ast.Expr{
		discoveredCall.appExpression,
		discoveredCall.patternExpression,
		discoveredCall.handlerExpression,
		discoveredCall.decorateCtxExpression,
	}
	if !discoveredCall.isLoader {
		callExpressions = append(callExpressions, discoveredCall.methodExpression)
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

func (analysis *backendRoutePackageAnalysis) collectRequiredImportsForExpression(
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

func (analysis *backendRoutePackageAnalysis) resolveImportPathForAliasAtPosition(
	importAlias string,
	position token.Pos,
) (string, bool) {
	parsedServerFile, hasParsedServerFile := analysis.getParsedServerRouteFileForPosition(position)
	if !hasParsedServerFile {
		return "", false
	}
	importPath, hasImportAlias := parsedServerFile.importAliases[importAlias]
	if !hasImportAlias {
		return "", false
	}
	return importPath, true
}

func (analysis *backendRoutePackageAnalysis) getParsedServerRouteFileForPosition(
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
		return nil, false
	}
	return parsedServerFile, true
}

package vormabuild

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
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

func parseBackendLoaderPatterns(v *vormaruntime.Vorma) ([]string, error) {
	serverRouteDefinitionFiles, err := resolveServerRouteDefinitionFiles(v)
	if err != nil {
		return nil, err
	}
	if len(serverRouteDefinitionFiles) == 0 {
		return nil, nil
	}

	goFileSet := token.NewFileSet()
	parsedGoFiles := make([]*ast.File, 0, len(serverRouteDefinitionFiles))
	for _, serverRouteDefinitionFile := range serverRouteDefinitionFiles {
		parsedFile, err := parseServerRouteDefinitionFile(
			goFileSet,
			serverRouteDefinitionFile,
		)
		if err != nil {
			return nil, err
		}
		parsedGoFiles = append(parsedGoFiles, parsedFile)
	}

	stringConstResolver := newGoStringConstResolver(
		collectPackageStringConstExpressions(parsedGoFiles),
	)

	loaderPatternSet := map[string]struct{}{}
	for fileIndex, parsedFile := range parsedGoFiles {
		serverRouteDefinitionFile := serverRouteDefinitionFiles[fileIndex]
		if err := walkTopLevelRouteRegistrationCalls(parsedFile, func(
			call *ast.CallExpr,
			position token.Pos,
		) error {
			pattern, isLoaderRoute, err := parseTopLevelRouteRegistrationCall(
				call,
				stringConstResolver,
			)
			if err != nil {
				return withGoFilePositionError(
					goFileSet,
					serverRouteDefinitionFile,
					position,
					err,
				)
			}
			if isLoaderRoute {
				loaderPatternSet[pattern] = struct{}{}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	loaderPatterns := make([]string, 0, len(loaderPatternSet))
	for loaderPattern := range loaderPatternSet {
		loaderPatterns = append(loaderPatterns, loaderPattern)
	}
	sort.Strings(loaderPatterns)
	return loaderPatterns, nil
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

func withGoFilePositionError(
	goFileSet *token.FileSet,
	serverRouteDefinitionFile string,
	position token.Pos,
	err error,
) error {
	filePosition := goFileSet.Position(position)
	lineNumber := filePosition.Line
	if lineNumber <= 0 {
		lineNumber = 1
	}
	return fmt.Errorf("%s:%d: %w", serverRouteDefinitionFile, lineNumber, err)
}

func walkTopLevelRouteRegistrationCalls(
	parsedFile *ast.File,
	visitCall func(*ast.CallExpr, token.Pos) error,
) error {
	for _, declaration := range parsedFile.Decls {
		varDeclaration, isVarDeclaration := declaration.(*ast.GenDecl)
		if !isVarDeclaration || varDeclaration.Tok != token.VAR {
			continue
		}
		for _, specNode := range varDeclaration.Specs {
			valueSpec, isValueSpec := specNode.(*ast.ValueSpec)
			if !isValueSpec {
				continue
			}
			for _, valueExpression := range valueSpec.Values {
				callExpression, isCallExpression := valueExpression.(*ast.CallExpr)
				if !isCallExpression {
					continue
				}
				if err := visitCall(callExpression, valueExpression.Pos()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func parseTopLevelRouteRegistrationCall(
	call *ast.CallExpr,
	stringConstResolver *goStringConstResolver,
) (pattern string, isLoaderRoute bool, err error) {
	functionIdentifier, isFunctionIdentifier := call.Fun.(*ast.Ident)
	if !isFunctionIdentifier {
		return "", false, nil
	}

	switch functionIdentifier.Name {
	case "NewLoader":
		if len(call.Args) < 1 {
			return "", false, fmt.Errorf("NewLoader requires a pattern argument")
		}
		loaderPattern, ok := stringConstResolver.Resolve(call.Args[0])
		if !ok {
			return "", false, fmt.Errorf(
				"NewLoader pattern must be a string literal or string const",
			)
		}
		return loaderPattern, true, nil
	case "NewAction":
		if len(call.Args) < 2 {
			return "", false, fmt.Errorf("NewAction requires method and pattern arguments")
		}
		if _, ok := stringConstResolver.Resolve(call.Args[0]); !ok {
			return "", false, fmt.Errorf(
				"NewAction method must be a string literal or string const",
			)
		}
		if _, ok := stringConstResolver.Resolve(call.Args[1]); !ok {
			return "", false, fmt.Errorf(
				"NewAction pattern must be a string literal or string const",
			)
		}
		return "", false, nil
	default:
		return "", false, nil
	}
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

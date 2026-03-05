// Package sourceparse owns the Go-source and route-pattern parsing primitives
// used by backend route discovery.
//
// Keeping this logic separate from registration graph traversal makes it easier
// to test file/pattern parsing and compile-time string resolution in isolation.
package sourceparse

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/internal/routeparse"
)

// Dependencies captures filesystem/parser behavior for source parsing.
type Dependencies struct {
	ExpandPattern      func(string) ([]string, error)
	StatPath           func(string) (fs.FileInfo, error)
	ReadFile           func(string) ([]byte, error)
	ParseGoSourceAST   func(*token.FileSet, string, any, parser.Mode) (*ast.File, error)
	ImportGoPackageDir func(string) (*build.Package, error)
}

// DefaultDependencies returns production source parsing dependencies.
func DefaultDependencies() Dependencies {
	return Dependencies{
		ExpandPattern: func(pattern string) ([]string, error) {
			return doublestar.FilepathGlob(pattern)
		},
		StatPath:         os.Stat,
		ReadFile:         os.ReadFile,
		ParseGoSourceAST: parser.ParseFile,
		ImportGoPackageDir: func(packageDir string) (*build.Package, error) {
			return build.Default.ImportDir(packageDir, 0)
		},
	}
}

// NormalizeDependencies fills unset dependency functions with defaults.
func NormalizeDependencies(dependencies Dependencies) Dependencies {
	defaultDependencies := DefaultDependencies()
	if dependencies.ExpandPattern == nil {
		dependencies.ExpandPattern = defaultDependencies.ExpandPattern
	}
	if dependencies.StatPath == nil {
		dependencies.StatPath = defaultDependencies.StatPath
	}
	if dependencies.ReadFile == nil {
		dependencies.ReadFile = defaultDependencies.ReadFile
	}
	if dependencies.ParseGoSourceAST == nil {
		dependencies.ParseGoSourceAST = defaultDependencies.ParseGoSourceAST
	}
	if dependencies.ImportGoPackageDir == nil {
		dependencies.ImportGoPackageDir = defaultDependencies.ImportGoPackageDir
	}
	return dependencies
}

// ResolveServerRouteDefinitionFiles resolves and validates configured server
// route definition files.
func ResolveServerRouteDefinitionFiles(
	v *vormaruntime.Vorma,
	dependencies Dependencies,
) ([]string, error) {
	dependencies = NormalizeDependencies(dependencies)

	if v == nil {
		return nil, fmt.Errorf("vorma runtime is required")
	}
	if v.Config == nil {
		return nil, fmt.Errorf("vorma config is required")
	}

	normalizedPatterns, err := routeparse.NormalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ServerRouteDefinitionPatterns(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"normalize server route definition patterns: %w",
			err,
		)
	}
	if len(normalizedPatterns) == 0 {
		return nil, nil
	}

	matchedFilesByPath := map[string]struct{}{}
	for _, routeDefinitionPattern := range normalizedPatterns {
		matchedFiles, resolveError := ResolveServerRouteDefinitionPattern(
			routeDefinitionPattern,
			dependencies,
		)
		if resolveError != nil {
			return nil, resolveError
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

// ResolveServerRouteDefinitionPattern resolves one server route definition
// pattern into concrete `.go` files.
func ResolveServerRouteDefinitionPattern(
	routeDefinitionPattern string,
	dependencies Dependencies,
) ([]string, error) {
	dependencies = NormalizeDependencies(dependencies)

	if !strings.ContainsAny(routeDefinitionPattern, "*?[{") {
		fileInfo, err := dependencies.StatPath(routeDefinitionPattern)
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

	matchedPaths, err := dependencies.ExpandPattern(routeDefinitionPattern)
	if err != nil {
		return nil, fmt.Errorf(
			"expand server route definition pattern %q: %w",
			routeDefinitionPattern,
			err,
		)
	}

	matchedGoFiles := make([]string, 0, len(matchedPaths))
	for _, matchedPath := range matchedPaths {
		fileInfo, statError := dependencies.StatPath(matchedPath)
		if statError != nil {
			return nil, fmt.Errorf(
				"stat server route definition path %q: %w",
				matchedPath,
				statError,
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

// ParseServerRouteDefinitionFile parses one server route definition file.
func ParseServerRouteDefinitionFile(
	goFileSet *token.FileSet,
	serverRouteDefinitionFile string,
	dependencies Dependencies,
) (*ast.File, error) {
	dependencies = NormalizeDependencies(dependencies)

	fileBytes, err := dependencies.ReadFile(serverRouteDefinitionFile)
	if err != nil {
		return nil, fmt.Errorf(
			"read server route definition file %q: %w",
			serverRouteDefinitionFile,
			err,
		)
	}
	parsedFile, err := dependencies.ParseGoSourceAST(
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

// CollectPackageStringConstExpressions maps package const names to expression
// nodes so callers can resolve compile-time string constants.
func CollectPackageStringConstExpressions(
	parsedGoFiles []*ast.File,
) map[string]ast.Expr {
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

// GoStringConstResolver resolves compile-time string const expressions.
type GoStringConstResolver struct {
	stringConstExpressions map[string]ast.Expr
	resolvedValues         map[string]string
	currentlyResolving     map[string]struct{}
}

// NewGoStringConstResolver creates a string const resolver from collected const
// expressions.
func NewGoStringConstResolver(
	stringConstExpressions map[string]ast.Expr,
) *GoStringConstResolver {
	return &GoStringConstResolver{
		stringConstExpressions: stringConstExpressions,
		resolvedValues:         map[string]string{},
		currentlyResolving:     map[string]struct{}{},
	}
}

// Resolve resolves compile-time string expressions.
func (resolver *GoStringConstResolver) Resolve(
	expression ast.Expr,
) (string, bool) {
	if resolver == nil {
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
	case *ast.Ident:
		return resolver.ResolveConstIdentifier(typedExpression.Name)
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

// ResolveConstIdentifier resolves a named const identifier to a string.
func (resolver *GoStringConstResolver) ResolveConstIdentifier(
	constIdentifier string,
) (string, bool) {
	if resolver == nil {
		return "", false
	}

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

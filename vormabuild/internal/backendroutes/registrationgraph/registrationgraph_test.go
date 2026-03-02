package registrationgraph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormabuild/internal/backendroutes/sourceparse"
)

func mustWriteFile(t *testing.T, filePath string, fileBytes []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("create parent directories for %q: %v", filePath, err)
	}
	if err := os.WriteFile(filePath, fileBytes, 0o644); err != nil {
		t.Fatalf("write %q: %v", filePath, err)
	}
}

func TestPackageAnalysis_GoTypesInitializationIsLazyAndFallbackSafe(
	t *testing.T,
) {
	t.Run(
		"does not initialize go types when canonical calls resolve by AST import metadata",
		func(t *testing.T) {
			tempDir := t.TempDir()
			routePackageDir := filepath.Join(
				tempDir,
				"backend",
				"src",
				"router",
			)

			mustWriteFile(
				t,
				filepath.Join(routePackageDir, "context.go"),
				[]byte(`
package router

import "github.com/vormadev/vorma"

var App = &vorma.Vorma{}

func decorateLoaderCtx(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
	return rd
}

func usersLoader(rd *vorma.LoaderReqData) (string, error) {
	return "ok", nil
}
`),
			)

			routesFilePath := filepath.Join(routePackageDir, "routes.go")
			mustWriteFile(t, routesFilePath, []byte(`
package router

import "github.com/vormadev/vorma"

var _ = vorma.DefineLoaderForRegistration(App, "/users", usersLoader, decorateLoaderCtx)
`))

			packageAnalyses, err := ParseServerRouteFilesIntoPackageAnalyses(
				[]string{routesFilePath},
				sourceparse.Dependencies{},
			)
			if err != nil {
				t.Fatalf(
					"ParseServerRouteFilesIntoPackageAnalyses returned error: %v",
					err,
				)
			}
			if len(packageAnalyses) != 1 {
				t.Fatalf(
					"package analysis length = %d, want 1",
					len(packageAnalyses),
				)
			}

			loaderPatterns, err := packageAnalyses[0].DiscoverLoaderPatterns()
			if err != nil {
				t.Fatalf("DiscoverLoaderPatterns returned error: %v", err)
			}
			expectedLoaderPatterns := []string{"/users"}
			if len(loaderPatterns) != len(expectedLoaderPatterns) {
				t.Fatalf(
					"loader patterns length = %d, want %d (%#v)",
					len(loaderPatterns),
					len(expectedLoaderPatterns),
					loaderPatterns,
				)
			}
			if loaderPatterns[0] != expectedLoaderPatterns[0] {
				t.Fatalf(
					"loader pattern[0] = %q, want %q",
					loaderPatterns[0],
					expectedLoaderPatterns[0],
				)
			}

			if packageAnalyses[0].goTypesInitializationCount != 0 {
				t.Fatalf(
					"go types initialization count = %d, want 0 for AST-resolved canonical call",
					packageAnalyses[0].goTypesInitializationCount,
				)
			}
		},
	)

	t.Run(
		"falls back to go types when AST import metadata is unavailable",
		func(t *testing.T) {
			expression, err := parser.ParseExpr(
				`vormaAlias.DefineLoaderForRegistration(App, "/fallback", usersLoader, decorateLoaderCtx)`,
			)
			if err != nil {
				t.Fatalf("ParseExpr returned error: %v", err)
			}

			registrationCall, isCallExpression := expression.(*ast.CallExpr)
			if !isCallExpression {
				t.Fatalf("expression type = %T, want *ast.CallExpr", expression)
			}
			calleeExpression := unwrapGenericCalleeExpression(
				registrationCall.Fun,
			)
			calleeSelectorExpression, isSelectorExpression := calleeExpression.(*ast.SelectorExpr)
			if !isSelectorExpression || calleeSelectorExpression.Sel == nil {
				t.Fatalf(
					"callee expression type = %T, want selector with Sel",
					calleeExpression,
				)
			}

			goTypesInfo := &types.Info{
				Uses: map[*ast.Ident]types.Object{
					calleeSelectorExpression.Sel: types.NewVar(
						token.NoPos,
						types.NewPackage("github.com/vormadev/vorma", "vorma"),
						"DefineLoaderForRegistration",
						types.Typ[types.Int],
					),
				},
			}
			analysis := &PackageAnalysis{
				goTypesInfo: goTypesInfo,
				stringConstResolver: sourceparse.NewGoStringConstResolver(
					map[string]ast.Expr{},
				),
			}
			parsedServerFile := &parsedServerRouteFile{
				importAliases: map[string]string{},
			}

			canonicalCall, isCanonicalCall, err := analysis.parseCanonicalRouteRegistrationCall(
				registrationCall,
				parsedServerFile,
				nil,
			)
			if err != nil {
				t.Fatalf(
					"parseCanonicalRouteRegistrationCall returned error: %v",
					err,
				)
			}
			if !isCanonicalCall || canonicalCall == nil {
				t.Fatal(
					"expected go/types fallback to classify canonical registration call",
				)
			}
			if canonicalCall.pattern != "/fallback" {
				t.Fatalf(
					"canonical call pattern = %q, want %q",
					canonicalCall.pattern,
					"/fallback",
				)
			}
		},
	)
}

func TestDiscoveredVormaRegistrationCallID_UsesSourcePosition(
	t *testing.T,
) {
	analysis := &PackageAnalysis{
		goFileSet: token.NewFileSet(),
	}

	callOne := &discoveredVormaRegistrationCall{
		isLoader:              true,
		appExpression:         &ast.Ident{Name: "App"},
		patternExpression:     quotedStringExpression("/same"),
		handlerExpression:     &ast.Ident{Name: "usersLoader"},
		decorateCtxExpression: &ast.Ident{Name: "decorateLoaderCtx"},
		sourcePosition:        token.Pos(11),
	}
	callTwo := &discoveredVormaRegistrationCall{
		isLoader:              true,
		appExpression:         &ast.Ident{Name: "App"},
		patternExpression:     quotedStringExpression("/same"),
		handlerExpression:     &ast.Ident{Name: "usersLoader"},
		decorateCtxExpression: &ast.Ident{Name: "decorateLoaderCtx"},
		sourcePosition:        token.Pos(22),
	}

	callIDOne, err := analysis.discoveredVormaRegistrationCallID(callOne)
	if err != nil {
		t.Fatalf(
			"discoveredVormaRegistrationCallID callOne returned error: %v",
			err,
		)
	}
	callIDTwo, err := analysis.discoveredVormaRegistrationCallID(callTwo)
	if err != nil {
		t.Fatalf(
			"discoveredVormaRegistrationCallID callTwo returned error: %v",
			err,
		)
	}

	if callIDOne == callIDTwo {
		t.Fatalf(
			"call IDs should differ for distinct source positions, got %q",
			callIDOne,
		)
	}
}

func TestDiscoveredVormaRegistrationCallID_UsesSourceColumnWhenAvailable(
	t *testing.T,
) {
	goFileSet := token.NewFileSet()
	goFile := goFileSet.AddFile(
		"/tmp/backend/src/router/routes.go",
		-1,
		256,
	)

	analysis := &PackageAnalysis{
		goFileSet: goFileSet,
	}

	callOne := &discoveredVormaRegistrationCall{
		isLoader:              true,
		appExpression:         &ast.Ident{Name: "App"},
		patternExpression:     quotedStringExpression("/same"),
		handlerExpression:     &ast.Ident{Name: "usersLoader"},
		decorateCtxExpression: &ast.Ident{Name: "decorateLoaderCtx"},
		sourcePosition:        goFile.Pos(10),
	}
	callTwo := &discoveredVormaRegistrationCall{
		isLoader:              true,
		appExpression:         &ast.Ident{Name: "App"},
		patternExpression:     quotedStringExpression("/same"),
		handlerExpression:     &ast.Ident{Name: "usersLoader"},
		decorateCtxExpression: &ast.Ident{Name: "decorateLoaderCtx"},
		sourcePosition:        goFile.Pos(20),
	}

	callIDOne, err := analysis.discoveredVormaRegistrationCallID(callOne)
	if err != nil {
		t.Fatalf(
			"discoveredVormaRegistrationCallID callOne returned error: %v",
			err,
		)
	}
	callIDTwo, err := analysis.discoveredVormaRegistrationCallID(callTwo)
	if err != nil {
		t.Fatalf(
			"discoveredVormaRegistrationCallID callTwo returned error: %v",
			err,
		)
	}

	if callIDOne == callIDTwo {
		t.Fatalf(
			"call IDs should differ for same-line source positions with different columns, got %q",
			callIDOne,
		)
	}
	if !strings.Contains(callIDOne, ":1:11|") {
		t.Fatalf(
			"call ID one = %q, expected column-aware source key",
			callIDOne,
		)
	}
	if !strings.Contains(callIDTwo, ":1:21|") {
		t.Fatalf(
			"call ID two = %q, expected column-aware source key",
			callIDTwo,
		)
	}
}

func TestCollectRequiredImportsForExpression_FailsOnUnqualifiedExternalSymbols(
	t *testing.T,
) {
	expression, err := parser.ParseExpr("ExternalSymbol")
	if err != nil {
		t.Fatalf("parse expression: %v", err)
	}
	expressionIdentifier, isExpressionIdentifier := expression.(*ast.Ident)
	if !isExpressionIdentifier {
		t.Fatalf("expression type = %T, want *ast.Ident", expression)
	}

	externalPackage := types.NewPackage(
		"example.com/dothelpers",
		"dothelpers",
	)
	externalObject := types.NewVar(
		token.NoPos,
		externalPackage,
		"ExternalSymbol",
		types.Typ[types.String],
	)
	analysis := &PackageAnalysis{
		goTypesInfo: &types.Info{
			Uses: map[*ast.Ident]types.Object{
				expressionIdentifier: externalObject,
			},
		},
		goTypesPackagePath: "example.com/localpkg",
	}

	err = analysis.collectRequiredImportsForExpression(
		expression,
		map[string]string{},
	)
	if err == nil {
		t.Fatal(
			"expected collectRequiredImportsForExpression to return error",
		)
	}
	if !strings.Contains(err.Error(), "unqualified external symbol") {
		t.Fatalf(
			"error = %q, expected unqualified external symbol guidance",
			err,
		)
	}
	if !strings.Contains(err.Error(), "dot-imported symbols") {
		t.Fatalf("error = %q, expected dot-import guidance", err)
	}
}

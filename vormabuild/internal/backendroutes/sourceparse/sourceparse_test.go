package sourceparse

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func TestResolveServerRouteDefinitionFiles_ValidationAndNoMatch(t *testing.T) {
	t.Run("rejects nil runtime", func(t *testing.T) {
		_, err := ResolveServerRouteDefinitionFiles(nil, Dependencies{})
		if err == nil {
			t.Fatal(
				"expected ResolveServerRouteDefinitionFiles to reject nil runtime",
			)
		}
		if !strings.Contains(err.Error(), "vorma runtime is required") {
			t.Fatalf("error = %q, expected nil-runtime context", err)
		}
	})

	t.Run(
		"returns no-match error when configured patterns match nothing",
		func(t *testing.T) {
			runtime := &vormaruntime.Vorma{
				Config: testkit.MustParseVormaConfigJSONForTest(
					t,
					vormaruntime.VormaConfigJSON{
						MainBuildEntry:       "backend/cmd/build",
						UIVariant:            string(vormaruntime.UIVariantReact),
						HTMLTemplateLocation: "entry.go.html",
						ClientEntry:          "frontend/src/vorma.entry.tsx",
						ClientRouteDefinitionPatterns: []string{
							"frontend/src/**/*.vorma.routes.ts",
						},
						ServerRouteDefinitionPatterns: []string{
							"server/routes/**/*.go",
						},
						TSGenOutDir: "frontend/src/vorma.gen",
					},
				),
			}

			_, err := ResolveServerRouteDefinitionFiles(runtime, Dependencies{})
			if err == nil {
				t.Fatal(
					"expected ResolveServerRouteDefinitionFiles to return no-match error",
				)
			}
			if !strings.Contains(
				err.Error(),
				"no server route definition files matched patterns",
			) {
				t.Fatalf("error = %q, expected no-match context", err)
			}
		},
	)
}

func TestResolveServerRouteDefinitionPattern(t *testing.T) {
	t.Run("glob patterns return only .go files", func(t *testing.T) {
		rootDir := t.TempDir()
		mustWriteFile(
			t,
			filepath.Join(rootDir, "routes.go"),
			"package routes\n",
		)
		mustWriteFile(t, filepath.Join(rootDir, "routes.txt"), "not go\n")
		mustWriteFile(
			t,
			filepath.Join(rootDir, "nested", "extra.go"),
			"package routes\n",
		)
		mustWriteFile(
			t,
			filepath.Join(rootDir, "nested", "extra.md"),
			"not go\n",
		)

		resolvedFiles, err := ResolveServerRouteDefinitionPattern(
			filepath.Join(rootDir, "**", "*"),
			Dependencies{},
		)
		if err != nil {
			t.Fatalf(
				"ResolveServerRouteDefinitionPattern returned error: %v",
				err,
			)
		}

		for index, resolvedFile := range resolvedFiles {
			resolvedFiles[index] = filepath.ToSlash(
				filepath.Clean(resolvedFile),
			)
		}
		slices.Sort(resolvedFiles)

		expectedFiles := []string{
			filepath.ToSlash(
				filepath.Clean(filepath.Join(rootDir, "nested", "extra.go")),
			),
			filepath.ToSlash(
				filepath.Clean(filepath.Join(rootDir, "routes.go")),
			),
		}
		slices.Sort(expectedFiles)
		if !reflect.DeepEqual(resolvedFiles, expectedFiles) {
			t.Fatalf(
				"resolved files = %#v, want %#v",
				resolvedFiles,
				expectedFiles,
			)
		}
	})

	t.Run("literal non-go file path is rejected", func(t *testing.T) {
		rootDir := t.TempDir()
		nonGoPath := filepath.Join(rootDir, "routes.txt")
		mustWriteFile(t, nonGoPath, "not go\n")

		_, err := ResolveServerRouteDefinitionPattern(
			nonGoPath,
			Dependencies{},
		)
		if err == nil {
			t.Fatal(
				"expected ResolveServerRouteDefinitionPattern to reject non-go path",
			)
		}
		if !strings.Contains(err.Error(), "must be a .go file") {
			t.Fatalf("error = %q, expected non-go validation context", err)
		}
	})
}

func TestParseServerRouteDefinitionFile_ErrorWrapping(t *testing.T) {
	rootDir := t.TempDir()
	sourcePath := filepath.Join(rootDir, "routes.go")
	mustWriteFile(t, sourcePath, "package routes\nfunc broken( {\n")

	_, err := ParseServerRouteDefinitionFile(
		token.NewFileSet(),
		sourcePath,
		Dependencies{},
	)
	if err == nil {
		t.Fatal("expected ParseServerRouteDefinitionFile to return parse error")
	}
	if !strings.Contains(err.Error(), "parse server route definition file") {
		t.Fatalf("error = %q, expected parse context", err)
	}
}

func TestCollectPackageStringConstExpressionsAndResolver(t *testing.T) {
	fileSet := token.NewFileSet()
	parsedFile, err := parser.ParseFile(
		fileSet,
		"routes.go",
		`package routes
const (
	Base = "/api"
	Users = Base + "/users"
	MultiA, MultiB = "/a", "/b"
)
`,
		0,
	)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	constExpressions := CollectPackageStringConstExpressions(
		[]*ast.File{parsedFile},
	)
	resolver := NewGoStringConstResolver(constExpressions)

	resolvedUsers, resolvedUsersOK := resolver.ResolveConstIdentifier("Users")
	if !resolvedUsersOK {
		t.Fatal("expected resolver to resolve Users const")
	}
	if resolvedUsers != "/api/users" {
		t.Fatalf("resolved Users = %q, want %q", resolvedUsers, "/api/users")
	}

	resolvedMultiA, resolvedMultiAOK := resolver.ResolveConstIdentifier(
		"MultiA",
	)
	if !resolvedMultiAOK || resolvedMultiA != "/a" {
		t.Fatalf(
			"resolved MultiA = %q (%v), want /a (true)",
			resolvedMultiA,
			resolvedMultiAOK,
		)
	}
	resolvedMultiB, resolvedMultiBOK := resolver.ResolveConstIdentifier(
		"MultiB",
	)
	if !resolvedMultiBOK || resolvedMultiB != "/b" {
		t.Fatalf(
			"resolved MultiB = %q (%v), want /b (true)",
			resolvedMultiB,
			resolvedMultiBOK,
		)
	}
}

func TestGoStringConstResolver_RejectsCycles(t *testing.T) {
	fileSet := token.NewFileSet()
	parsedFile, err := parser.ParseFile(
		fileSet,
		"routes.go",
		`package routes
const A = B
const B = A
`,
		0,
	)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	resolver := NewGoStringConstResolver(
		CollectPackageStringConstExpressions([]*ast.File{parsedFile}),
	)
	if _, ok := resolver.ResolveConstIdentifier("A"); ok {
		t.Fatal("expected resolver to reject cyclic const expression")
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent for %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %q: %v", path, err)
	}
}

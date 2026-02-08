package build_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildTypeScriptPackagingConformance(t *testing.T) {
	mainGoPath := filepath.Join(repoRoot(t), "internal", "scripts", "buildts", "main.go")
	mainSrc, mainFileSet, mainAST := mustParseGoSourceFile(t, mainGoPath)

	t.Run("BDC-TSPKG-001_BUILD-TSPKG-001_buildts_cleans_npm_dist_and_executes_defined_target_pipeline_order", func(t *testing.T) {
		if !strings.Contains(mainSrc, `var targetDir = "./npm_dist"`) {
			t.Fatalf("expected buildts targetDir to be ./npm_dist")
		}

		mainBody := mustFunctionBodySource(
			t,
			mainSrc,
			mainFileSet,
			mainAST,
			"main",
		)
		requireOrderedSubstrings(
			t,
			mainBody,
			[]string{
				"os.RemoveAll(targetDir)",
				"os.MkdirAll(targetDir, 0755)",
				"buildKit()",
				"buildClient()",
				"buildReact()",
				"buildSolid()",
				"buildPreact()",
				"buildVite()",
				"buildCreate()",
				"removeTestFiles()",
			},
		)
	})

	t.Run("BDC-TSPKG-002_BUILD-TSPKG-002_tsc_declaration_only_pipeline_emits_types_to_npm_dist", func(t *testing.T) {
		runTSCBody := mustFunctionBodySource(
			t,
			mainSrc,
			mainFileSet,
			mainAST,
			"runTSC",
		)
		for _, expected := range []string{
			"--declaration",
			"--emitDeclarationOnly",
			"--outDir ./npm_dist",
			"--sourceMap",
			"--declarationMap",
		} {
			if !strings.Contains(runTSCBody, expected) {
				t.Fatalf("expected runTSC declaration pipeline to include %q", expected)
			}
		}
	})

	t.Run("BDC-TSPKG-003_BUILD-TSPKG-003_esbuild_warnings_are_release_blocking_failures", func(t *testing.T) {
		buildBody := mustFunctionBodySource(
			t,
			mainSrc,
			mainFileSet,
			mainAST,
			"build",
		)
		for _, expected := range []string{
			"if len(result.Warnings) > 0",
			`log.Fatalf("%s: esbuild had warnings", label)`,
		} {
			if !strings.Contains(buildBody, expected) {
				t.Fatalf("expected build helper warning-failure behavior to include %q", expected)
			}
		}
	})

	t.Run("BDC-TSPKG-004_BUILD-TSPKG-004_solid_pipeline_uses_esbuild_plugin_solid_via_node_helper", func(t *testing.T) {
		solidBody := mustFunctionBodySource(
			t,
			mainSrc,
			mainFileSet,
			mainAST,
			"buildSolid",
		)
		for _, expected := range []string{
			`runTSC("./internal/framework/_typescript/solid/tsconfig.json")`,
			`executil.RunCmd("node", "./internal/scripts/buildts/build-solid.mjs")`,
		} {
			if !strings.Contains(solidBody, expected) {
				t.Fatalf("expected buildSolid behavior to include %q", expected)
			}
		}

		solidScriptPath := filepath.Join(repoRoot(t), "internal", "scripts", "buildts", "build-solid.mjs")
		solidScript := mustReadFileAsString(t, solidScriptPath)
		for _, expected := range []string{
			"solidPlugin()",
			`entryPoints: ["./internal/framework/_typescript/solid/index.tsx"]`,
			`external: ["vorma", "solid-js"]`,
			`outdir: "./npm_dist/internal/framework/_typescript/solid"`,
		} {
			if !strings.Contains(solidScript, expected) {
				t.Fatalf("expected build-solid.mjs to include %q", expected)
			}
		}
	})

	t.Run("BDC-TSPKG-005_BUILD-TSPKG-005_adapter_and_vite_peer_dependencies_remain_externalized", func(t *testing.T) {
		for _, expected := range []string{
			`"react", "react-dom"`,
			`"preact", "preact/hooks"`,
			`"preact/jsx-runtime", "preact/compat", "preact/test-utils"`,
			`"vite",`,
			`"node:fs",`,
			`"node:path",`,
		} {
			if !strings.Contains(mainSrc, expected) {
				t.Fatalf("expected buildts externalized peer list to include %q", expected)
			}
		}
	})

	t.Run("BDC-TSPKG-006_BUILD-TSPKG-006_test_and_bench_artifacts_are_purged_from_npm_dist_output", func(t *testing.T) {
		removeTestsBody := mustFunctionBodySource(
			t,
			mainSrc,
			mainFileSet,
			mainAST,
			"removeTestFiles",
		)
		for _, expected := range []string{
			`strings.Contains(path, ".test.")`,
			`strings.Contains(path, ".bench.")`,
			"return os.Remove(path)",
		} {
			if !strings.Contains(removeTestsBody, expected) {
				t.Fatalf("expected removeTestFiles contract to include %q", expected)
			}
		}
	})
}

func mustParseGoSourceFile(
	t *testing.T,
	path string,
) (
	source string,
	fileSet *token.FileSet,
	fileAST *ast.File,
) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read source file %s: %v", path, err)
	}
	source = string(b)

	fileSet = token.NewFileSet()
	fileAST, err = parser.ParseFile(fileSet, path, source, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse source file %s: %v", path, err)
	}
	return source, fileSet, fileAST
}

func mustFunctionBodySource(
	t *testing.T,
	source string,
	fileSet *token.FileSet,
	fileAST *ast.File,
	functionName string,
) string {
	t.Helper()
	for _, decl := range fileAST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != functionName {
			continue
		}
		if fn.Body == nil {
			t.Fatalf("function %s has no body", functionName)
		}

		start := fileSet.Position(fn.Body.Pos()).Offset
		end := fileSet.Position(fn.Body.End()).Offset
		if start < 0 || end < start || end > len(source) {
			t.Fatalf("invalid source offsets for function %s: start=%d end=%d len=%d", functionName, start, end, len(source))
		}
		return source[start:end]
	}

	t.Fatalf("function %s not found in parsed source", functionName)
	return ""
}

func requireOrderedSubstrings(
	t *testing.T,
	haystack string,
	needles []string,
) {
	t.Helper()
	searchFrom := 0
	for _, needle := range needles {
		relative := strings.Index(haystack[searchFrom:], needle)
		if relative < 0 {
			t.Fatalf("expected ordered content to include %q after index %d", needle, searchFrom)
		}
		searchFrom += relative + len(needle)
	}
}

func mustReadFileAsString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(b)
}

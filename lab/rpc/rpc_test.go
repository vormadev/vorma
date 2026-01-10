package rpc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cleanUpTestFiles(t *testing.T, path string) {
	err := os.RemoveAll(path)
	if err != nil {
		t.Fatalf("failed to clean up test files: %s", err)
	}
}

const testFileName = "api-types.ts"

func TestGenerateTypeScript(t *testing.T) {
	tempDir := t.TempDir()

	type ID struct{ ID int }
	type Success struct{ Success bool }
	type Name struct{ Name string }
	type Result struct{ Result string }

	opts := Opts{
		OutPath: filepath.Join(tempDir, testFileName),
		RouteDefs: []RouteDef{
			{
				Key:        "testMutation",
				ActionType: ActionTypeMutation,
				Input:      ID{},
				Output:     Success{},
			},
			{
				Key:        "testQuery",
				ActionType: ActionTypeQuery,
				Input:      Name{},
				Output:     Result{},
			},
		},
		AdHocTypes: []*AdHocType{
			{
				TypeInstance: struct{ Data string }{},
				TSTypeName:   "TestAdHocType",
			},
		},
	}

	err := GenerateTypeScript(opts)
	if err != nil {
		t.Fatalf("GenerateTypeScript failed: %s", err)
	}

	if _, err := os.Stat(opts.OutPath); os.IsNotExist(err) {
		t.Fatalf("Expected TypeScript file not found: %s", opts.OutPath)
	}

	content, err := os.ReadFile(opts.OutPath)
	if err != nil {
		t.Fatalf("Failed to read generated TypeScript file: %s", err)
	}

	if len(content) == 0 {
		t.Fatal("Generated TypeScript file is empty")
	}

	contentStr := string(content)

	contestStrMinimized := normalizeWhiteSpace(contentStr)

	for _, expectedStr := range expectedStrs {
		if !strings.Contains(contestStrMinimized, normalizeWhiteSpace(expectedStr)) {
			t.Errorf(
				"Expected string not found in generated TypeScript content: %s",
				normalizeWhiteSpace(expectedStr),
			)
		}
	}

	if !strings.Contains(contentStr, "export type ID = {") {
		t.Error("Expected TypeScript type for ID not found")
	}
	if !strings.Contains(contentStr, "export type Success = {") {
		t.Error("Expected TypeScript type for Success not found")
	}
	if !strings.Contains(contentStr, "export type Name = {") {
		t.Error("Expected TypeScript type for Name not found")
	}
	if !strings.Contains(contentStr, "export type Result = {") {
		t.Error("Expected TypeScript type for Result not found")
	}
	if !strings.Contains(contentStr, "export type TestAdHocType = {") {
		t.Error("Expected TypeScript type for TestAdHocType not found")
	}

	cleanUpTestFiles(t, tempDir)
}

func TestGenerateTypeScriptNoRoutes(t *testing.T) {
	tempDir := t.TempDir()

	opts := Opts{
		OutPath:   filepath.Join(tempDir, testFileName),
		RouteDefs: []RouteDef{},
	}

	err := GenerateTypeScript(opts)
	if err != nil {
		t.Fatalf("GenerateTypeScript failed: %s", err)
	}

	if _, err := os.Stat(opts.OutPath); os.IsNotExist(err) {
		t.Fatalf("Expected TypeScript file not found: %s", opts.OutPath)
	}

	content, err := os.ReadFile(opts.OutPath)
	if err != nil {
		t.Fatalf("Failed to read generated TypeScript file: %s", err)
	}

	if len(content) == 0 {
		t.Fatal("Generated TypeScript file is empty")
	}

	cleanUpTestFiles(t, tempDir)
}

func TestExtraTS(t *testing.T) {
	tempDir := t.TempDir()

	opts := Opts{
		OutPath:     filepath.Join(tempDir, testFileName),
		RouteDefs:   []RouteDef{},
		ExtraTSCode: "export const extraCode = 'extra';",
	}

	err := GenerateTypeScript(opts)
	if err != nil {
		t.Fatalf("GenerateTypeScript failed: %s", err)
	}

	if _, err := os.Stat(opts.OutPath); os.IsNotExist(err) {
		t.Fatalf("Expected TypeScript file not found: %s", opts.OutPath)
	}

	content, err := os.ReadFile(opts.OutPath)
	if err != nil {
		t.Fatalf("Failed to read generated TypeScript file: %s", err)
	}

	if len(content) == 0 {
		t.Fatal("Generated TypeScript file is empty")
	}

	contentStr := string(content)

	fmt.Println(contentStr)

	if !strings.Contains(contentStr, "export const extraCode = 'extra';") {
		t.Error("Expected extra TypeScript code not found")
	}

	cleanUpTestFiles(t, tempDir)
}

const routes = `const routes = [
	{
		actionType: "mutation",
		key: "testMutation",
		phantomInputType: null as unknown as ID,
		phantomOutputType: null as unknown as Success,
	},
	{
		actionType: "query",
		key: "testQuery",
		phantomInputType: null as unknown as Name,
		phantomOutputType: null as unknown as Result,
	},
] as const;`

var expectedStrs = []string{routes, extraTSCode}

func normalizeWhiteSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

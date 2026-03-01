package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMutationLabTemplatesUseConfigFirstURLBuilderInput(t *testing.T) {
	_, currentFilePath, _, hasCaller := runtime.Caller(0)
	if !hasCaller {
		t.Fatal("resolve current test file path: runtime.Caller failed")
	}

	repositoryRootPath := filepath.Clean(
		filepath.Join(filepath.Dir(currentFilePath), "..", "..", ".."),
	)

	templateRelativePaths := []string{
		"internal/e2e/overlay_templates/react_like/frontend/src/components/mutation_lab.tsx.txt",
		"internal/e2e/overlay_templates/preact_overrides/frontend/src/components/mutation_lab.tsx.txt",
		"internal/e2e/overlay_templates/solid/frontend/src/components/mutation_lab.tsx.txt",
	}

	for _, templateRelativePath := range templateRelativePaths {
		templateRelativePath := templateRelativePath
		t.Run(templateRelativePath, func(t *testing.T) {
			templatePath := filepath.Join(
				repositoryRootPath,
				filepath.FromSlash(templateRelativePath),
			)
			templateContents, readTemplateError := os.ReadFile(templatePath)
			if readTemplateError != nil {
				t.Fatalf(
					"read template file %q: %v",
					templatePath,
					readTemplateError,
				)
			}

			templateSource := string(templateContents)
			if !strings.Contains(templateSource, "buildMutationURL(vormaAppConfig, {") {
				t.Fatalf(
					"template %q does not use config-first buildMutationURL signature",
					templatePath,
				)
			}

			if strings.Contains(templateSource, "buildMutationURL({") {
				t.Fatalf(
					"template %q must not use object-based buildMutationURL input",
					templatePath,
				)
			}
		})
	}
}

func TestCanReuseExistingFixtureProject_ReturnsFalseWhenViteChunkFileMissing(
	t *testing.T,
) {
	outputDirectoryPath := t.TempDir()
	options := commandOptions{
		outputDirectoryPath: outputDirectoryPath,
		reuseIfPresent:      true,
	}

	for _, requiredFixtureEntryRelativePath := range reusableFixtureRequiredEntryRelativePaths() {
		if requiredFixtureEntryRelativePath ==
			"node_modules/vite/dist/node/chunks/chunk.js" {
			continue
		}
		writeReusableFixtureEntryForTest(
			t,
			outputDirectoryPath,
			requiredFixtureEntryRelativePath,
			doesReusableFixtureEntryPathRequireDirectory(
				requiredFixtureEntryRelativePath,
			),
		)
	}
	writeReusableFixtureEntryForTest(
		t,
		outputDirectoryPath,
		".vorma_e2e_fixture_ready",
		false,
	)

	canReuseExistingFixture, canReuseExistingFixtureError :=
		canReuseExistingFixtureProject(options)
	if canReuseExistingFixtureError != nil {
		t.Fatalf("expected nil reuse-check error, got %v", canReuseExistingFixtureError)
	}
	if canReuseExistingFixture {
		t.Fatal("expected reuse check to fail when vite chunk.js is missing")
	}
}

func TestCanReuseExistingFixtureProject_ReturnsTrueWhenRequiredEntriesExist(
	t *testing.T,
) {
	outputDirectoryPath := t.TempDir()
	options := commandOptions{
		outputDirectoryPath: outputDirectoryPath,
		reuseIfPresent:      true,
	}

	for _, requiredFixtureEntryRelativePath := range reusableFixtureRequiredEntryRelativePaths() {
		writeReusableFixtureEntryForTest(
			t,
			outputDirectoryPath,
			requiredFixtureEntryRelativePath,
			doesReusableFixtureEntryPathRequireDirectory(
				requiredFixtureEntryRelativePath,
			),
		)
	}
	writeReusableFixtureEntryForTest(
		t,
		outputDirectoryPath,
		".vorma_e2e_fixture_ready",
		false,
	)

	canReuseExistingFixture, canReuseExistingFixtureError :=
		canReuseExistingFixtureProject(options)
	if canReuseExistingFixtureError != nil {
		t.Fatalf("expected nil reuse-check error, got %v", canReuseExistingFixtureError)
	}
	if !canReuseExistingFixture {
		t.Fatal("expected reuse check to pass when required fixture entries exist")
	}
}

func writeReusableFixtureEntryForTest(
	t *testing.T,
	outputDirectoryPath string,
	relativePath string,
	isDirectory bool,
) {
	t.Helper()
	absolutePath := filepath.Join(outputDirectoryPath, filepath.FromSlash(relativePath))
	if isDirectory {
		if makeDirectoryError := os.MkdirAll(absolutePath, 0o755); makeDirectoryError != nil {
			t.Fatalf("mkdir %q: %v", absolutePath, makeDirectoryError)
		}
		return
	}

	parentDirectoryPath := filepath.Dir(absolutePath)
	if makeDirectoryError := os.MkdirAll(parentDirectoryPath, 0o755); makeDirectoryError != nil {
		t.Fatalf("mkdir %q: %v", parentDirectoryPath, makeDirectoryError)
	}
	if writeFileError := os.WriteFile(absolutePath, []byte("ok\n"), 0o644); writeFileError != nil {
		t.Fatalf("write %q: %v", absolutePath, writeFileError)
	}
}

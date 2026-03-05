package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const reusableFixtureRolldownBindingManifestRelativePath = "node_modules/@rolldown/binding-test/package.json"

func TestMutationLabTemplatesUseConfigFirstURLBuilderInput(t *testing.T) {
	repositoryRootPath := resolveRepositoryRootPathForTest(t)

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

func TestE2EPackageTemplatePinsCaniuseLiteOverride(t *testing.T) {
	repositoryRootPath := resolveRepositoryRootPathForTest(t)
	packageTemplatePath := filepath.Join(
		repositoryRootPath,
		"internal",
		"e2e",
		"overlay_templates",
		"common",
		"package.json.tmpl",
	)

	packageTemplateContents, readTemplateError := os.ReadFile(packageTemplatePath)
	if readTemplateError != nil {
		t.Fatalf(
			"read package template file %q: %v",
			packageTemplatePath,
			readTemplateError,
		)
	}

	packageTemplateSource := string(packageTemplateContents)
	if !strings.Contains(
		packageTemplateSource,
		`"caniuse-lite": "1.0.30001741"`,
	) {
		t.Fatalf(
			"package template %q does not pin caniuse-lite override",
			packageTemplatePath,
		)
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

	writeReusableFixtureEntriesForTest(
		t,
		outputDirectoryPath,
		"node_modules/vite/dist/node/chunks/chunk.js",
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

func TestCanReuseExistingFixtureProject_ReturnsFalseWhenCaniuseLiteAgentsFileMissing(
	t *testing.T,
) {
	outputDirectoryPath := t.TempDir()
	options := commandOptions{
		outputDirectoryPath: outputDirectoryPath,
		reuseIfPresent:      true,
	}

	writeReusableFixtureEntriesForTest(
		t,
		outputDirectoryPath,
		"node_modules/caniuse-lite/dist/unpacker/agents.js",
	)

	canReuseExistingFixture, canReuseExistingFixtureError :=
		canReuseExistingFixtureProject(options)
	if canReuseExistingFixtureError != nil {
		t.Fatalf("expected nil reuse-check error, got %v", canReuseExistingFixtureError)
	}
	if canReuseExistingFixture {
		t.Fatal("expected reuse check to fail when caniuse-lite agents.js is missing")
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

	writeReusableFixtureEntriesForTest(t, outputDirectoryPath)

	canReuseExistingFixture, canReuseExistingFixtureError :=
		canReuseExistingFixtureProject(options)
	if canReuseExistingFixtureError != nil {
		t.Fatalf("expected nil reuse-check error, got %v", canReuseExistingFixtureError)
	}
	if !canReuseExistingFixture {
		t.Fatal("expected reuse check to pass when required fixture entries exist")
	}
}

func TestCanReuseExistingFixtureProject_ReturnsFalseWhenRolldownBindingPackageMissing(
	t *testing.T,
) {
	outputDirectoryPath := t.TempDir()
	options := commandOptions{
		outputDirectoryPath: outputDirectoryPath,
		reuseIfPresent:      true,
	}

	writeReusableFixtureEntriesForTest(
		t,
		outputDirectoryPath,
		reusableFixtureRolldownBindingManifestRelativePath,
	)

	canReuseExistingFixture, canReuseExistingFixtureError :=
		canReuseExistingFixtureProject(options)
	if canReuseExistingFixtureError != nil {
		t.Fatalf("expected nil reuse-check error, got %v", canReuseExistingFixtureError)
	}
	if canReuseExistingFixture {
		t.Fatal("expected reuse check to fail when rolldown binding package is missing")
	}
}

func resolveRepositoryRootPathForTest(t *testing.T) string {
	t.Helper()
	_, currentFilePath, _, hasCaller := runtime.Caller(0)
	if !hasCaller {
		t.Fatal("resolve current test file path: runtime.Caller failed")
	}

	return filepath.Clean(
		filepath.Join(filepath.Dir(currentFilePath), "..", "..", ".."),
	)
}

func writeReusableFixtureEntriesForTest(
	t *testing.T,
	outputDirectoryPath string,
	missingRequiredFixtureEntryRelativePaths ...string,
) {
	t.Helper()

	missingRequiredFixtureEntryPathSet := make(map[string]struct{})
	for _, missingRequiredFixtureEntryRelativePath := range missingRequiredFixtureEntryRelativePaths {
		missingRequiredFixtureEntryPathSet[missingRequiredFixtureEntryRelativePath] =
			struct{}{}
	}

	for _, requiredFixtureEntryRelativePath := range reusableFixtureRequiredEntryRelativePaths() {
		_, shouldSkipRequiredEntry :=
			missingRequiredFixtureEntryPathSet[requiredFixtureEntryRelativePath]
		if shouldSkipRequiredEntry {
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

	_, shouldSkipRolldownBindingManifest := missingRequiredFixtureEntryPathSet[reusableFixtureRolldownBindingManifestRelativePath]
	if !shouldSkipRolldownBindingManifest {
		writeReusableFixtureEntryForTest(
			t,
			outputDirectoryPath,
			reusableFixtureRolldownBindingManifestRelativePath,
			false,
		)
	}
	writeReusableFixtureEntryForTest(
		t,
		outputDirectoryPath,
		".vorma_e2e_fixture_ready",
		false,
	)
}

func writeReusableFixtureEntryForTest(
	t *testing.T,
	outputDirectoryPath string,
	relativePath string,
	isDirectory bool,
) {
	t.Helper()
	normalizedRelativePath := filepath.ToSlash(relativePath)
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
	fileContents := []byte("ok\n")
	if normalizedRelativePath == "backend/wave.config.json" {
		fileContents = []byte(`{
	"Core": {
		"ProjectID": "e2e-fixture-gen-test",
		"ResolveRoot": ".",
		"MainAppEntry": "cmd/serve",
		"StaticAssetDirs": {
			"Private": "assets",
			"Public": "../frontend/assets"
		}
	}
}
`)
	}
	if writeFileError := os.WriteFile(absolutePath, fileContents, 0o644); writeFileError != nil {
		t.Fatalf("write %q: %v", absolutePath, writeFileError)
	}
}

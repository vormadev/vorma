package ki

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/htmltestutil"
)

func TestGetCriticalCSS(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	criticalCSS := "body { color: red; }"
	env.createTestFile(t, "dist/static/internal/critical.css", criticalCSS)

	result := env.config.GetCriticalCSS()
	if result != criticalCSS {
		t.Errorf("GetCriticalCSS() = %v, want: %v", result, criticalCSS)
	}
}

func TestGetCriticalCSSStyleElement(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	criticalCSS := "body { color: red; }"
	env.createTestFile(t, "dist/static/internal/critical.css", criticalCSS)

	result := env.config.GetCriticalCSSStyleElement()
	expected := template.HTML(`<style id="` + CriticalCSSElementID + "\">\nbody { color: red; }</style>")

	parsedResult, err := htmltestutil.ParseHTML(string(result))
	if err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}
	parsedExpected, err := htmltestutil.ParseHTML(string(expected))
	if err != nil {
		t.Fatalf("Failed to parse expected: %v", err)
	}

	if !htmltestutil.CompareNodes(parsedResult, parsedExpected) {
		t.Errorf("GetCriticalCSSStyleElement() = %v, want: %v", parsedResult, parsedExpected)
	}
}

func TestGetStyleSheetURL(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	normalCSSFile := "normal_1234567890.css"
	env.createTestFile(t, "dist/static/internal/normal_css_file_ref.txt", normalCSSFile)

	result := env.config.GetStyleSheetURL()
	expected := "/bob/" + normalCSSFile
	if result != expected {
		t.Errorf("GetStyleSheetURL() = %v, want: %v", result, expected)
	}
}

func TestGetStyleSheetLinkElement(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	normalCSSFile := "normal_1234567890.css"
	env.createTestFile(t, "dist/static/internal/normal_css_file_ref.txt", normalCSSFile)

	result := env.config.GetStyleSheetLinkElement()
	expected := template.HTML(`<link rel="stylesheet" href="/bob/` + normalCSSFile + `" id="` + StyleSheetElementID + `" />`)
	if result != expected {
		t.Errorf("GetStyleSheetLinkElement() = %v, want: %v", result, expected)
	}
}

func TestBuildCSS(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	// Create test CSS files in the source directory
	criticalCSS := "body { color: red; }"
	normalCSS := "p { font-size: 16px; }"
	env.createTestFile(t, "critical.css", criticalCSS)
	env.createTestFile(t, "main.css", normalCSS)

	minimizedCriticalCSS := "body{color:red}\n"
	minimizedNormalCSS := "p{font-size:16px}\n"

	err := env.config.buildCSS()
	if err != nil {
		t.Fatalf("buildCSS() error = %v", err)
	}

	// Check if critical CSS was processed correctly
	processedCriticalCSS, err := os.ReadFile(filepath.Join(testRootDir, "dist/static/internal/critical.css"))
	if err != nil {
		t.Fatalf("Failed to read processed critical CSS: %v", err)
	}
	if string(processedCriticalCSS) != minimizedCriticalCSS {
		t.Errorf("Processed critical CSS = %v, want: %v", string(processedCriticalCSS), minimizedCriticalCSS)
	}

	// Check if normal CSS reference file was created and points to an existing file
	normalCSSRef, err := os.ReadFile(filepath.Join(testRootDir, "dist/static/internal/normal_css_file_ref.txt"))
	if err != nil {
		t.Fatalf("Failed to read normal CSS reference file: %v", err)
	}
	normalCSSFilename := strings.TrimSpace(string(normalCSSRef))
	if !strings.HasPrefix(normalCSSFilename, "vorma_out_vorma_internal_normal_") || !strings.HasSuffix(normalCSSFilename, ".css") {
		t.Errorf("Invalid normal CSS reference: %v", normalCSSFilename)
	}

	// Check if the referenced normal CSS file exists and has correct content
	normalCSSPath := filepath.Join(testRootDir, "dist/static/assets/public", normalCSSFilename)
	if _, err := os.Stat(normalCSSPath); os.IsNotExist(err) {
		t.Fatalf("Referenced normal CSS file does not exist: %s", normalCSSPath)
	}

	processedNormalCSS, err := os.ReadFile(normalCSSPath)
	if err != nil {
		t.Fatalf("Failed to read processed normal CSS: %v", err)
	}
	if string(processedNormalCSS) != minimizedNormalCSS {
		t.Errorf("Processed normal CSS = %v, want: %v", string(processedNormalCSS), minimizedNormalCSS)
	}
}

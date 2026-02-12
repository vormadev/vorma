package tooling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestBuildCriticalCSS_ResolvesPublicURLTokensUsingFileMap(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = wave.CSSEntryFiles{
		Critical: filepath.Join(root, "styles", "critical.css"),
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	if err := os.MkdirAll(filepath.Dir(cfg.Core.CSSEntryFiles.Critical), 0755); err != nil {
		t.Fatalf("failed creating CSS entry parent dir: %v", err)
	}
	cssContent := `.hero{background-image:url("images/logo.png");}`
	if err := os.WriteFile(cfg.Core.CSSEntryFiles.Critical, []byte(cssContent), 0644); err != nil {
		t.Fatalf("failed writing CSS entry file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	fileMap := wave.FileMap{
		"images/logo.png": {
			DistName:    "vorma_out_images_logo_deadbeef.png",
			ContentHash: "vorma_out_images_logo_deadbeef.png",
		},
	}
	if err := builder.saveFileMap(fileMap, cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("saveFileMap returned error: %v", err)
	}

	if err := builder.BuildCriticalCSS(true); err != nil {
		t.Fatalf("BuildCriticalCSS returned error: %v", err)
	}

	output, err := os.ReadFile(cfg.Dist.CriticalCSS())
	if err != nil {
		t.Fatalf("failed reading generated critical CSS: %v", err)
	}
	outputCSS := string(output)

	if !strings.Contains(outputCSS, "/vorma_out_images_logo_deadbeef.png") {
		t.Fatalf("expected output CSS to contain resolved hashed URL, got:\n%s", outputCSS)
	}
}

func TestGetPublicURLBuildtimeCached_UsesCachedFileMapAfterFirstLoad(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	fileMap := wave.FileMap{
		"images/logo.png": {
			DistName:    "vorma_out_images_logo_deadbeef.png",
			ContentHash: "vorma_out_images_logo_deadbeef.png",
		},
	}
	if err := builder.saveFileMap(fileMap, cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("saveFileMap returned error: %v", err)
	}

	first := builder.getPublicURLBuildtimeCached("images/logo.png")
	if first != "/vorma_out_images_logo_deadbeef.png" {
		t.Fatalf("unexpected first cached lookup result: %q", first)
	}

	if err := os.Remove(cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("failed removing gob after initial cache load: %v", err)
	}

	second := builder.getPublicURLBuildtimeCached("images/logo.png")
	if second != first {
		t.Fatalf("expected cached lookup to remain stable after gob removal: first=%q second=%q", first, second)
	}
}

func TestCSSBuildAll_ReturnsCriticalErrorWithContext(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.CSSEntryFiles = wave.CSSEntryFiles{
		Critical: filepath.Join(t.TempDir(), "missing-critical.css"),
	}
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.css.buildAll(true)
	if err == nil {
		t.Fatal("expected buildAll to fail for missing critical CSS entry")
	}
	if !strings.Contains(err.Error(), "critical CSS") {
		t.Fatalf("unexpected critical buildAll error: %v", err)
	}
}

func TestCSSBuildAll_ReturnsNormalErrorWithContext(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.CSSEntryFiles = wave.CSSEntryFiles{
		NonCritical: filepath.Join(t.TempDir(), "missing-normal.css"),
	}
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.css.buildAll(true)
	if err == nil {
		t.Fatal("expected buildAll to fail for missing normal CSS entry")
	}
	if !strings.Contains(err.Error(), "normal CSS") {
		t.Fatalf("unexpected normal buildAll error: %v", err)
	}
}

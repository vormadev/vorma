package css_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/builder/internal/css"
)

type staticAssetDirsForTests = struct {
	Private string `json:"Private"`
	Public  string `json:"Public"`
}

type cssEntryFilesForTests = struct {
	Critical    string `json:"Critical,omitempty"`
	NonCritical string `json:"NonCritical,omitempty"`
}

func newParsedConfigForCSSPackageTestsAtRoot(root string) *wave.ParsedConfig {
	cfg := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
			StaticAssetDirs: staticAssetDirsForTests{
				Public:  filepath.Join(root, "static", "public"),
				Private: filepath.Join(root, "static", "private"),
			},
		},
		Watch: &wave.WatchConfig{
			WatchRoot: root,
		},
	}
	cfg.Dist.Root = cfg.Core.DistDir
	return cfg
}

func TestValidateCSSConfig(t *testing.T) {
	if validateError := css.ValidateCSSConfig(nil); validateError == nil {
		t.Fatal("expected ValidateCSSConfig(nil) to fail")
	}

	cfg := newParsedConfigForCSSPackageTestsAtRoot(t.TempDir())
	if validateError := css.ValidateCSSConfig(cfg); validateError != nil {
		t.Fatalf("expected empty CSS config to validate: %v", validateError)
	}

	cfg.Core.CSSEntryFiles.Critical = "styles/main.txt"
	if validateError := css.ValidateCSSConfig(cfg); validateError == nil {
		t.Fatal("expected non-.css critical entry to fail validation")
	}

	cfg.Core.CSSEntryFiles.Critical = "styles/main.css"
	cfg.Core.CSSEntryFiles.NonCritical = "styles/main.css"
	if validateError := css.ValidateCSSConfig(cfg); validateError == nil {
		t.Fatal("expected critical/non-critical collision to fail validation")
	}
}

func TestProcessor_ReadHotReloadOutputs(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForCSSPackageTestsAtRoot(root)
	cfg.Core.PublicPathPrefix = "/assets"

	if mkdirError := os.MkdirAll(cfg.Dist.Static(), 0o755); mkdirError != nil {
		t.Fatalf("mkdir dist static: %v", mkdirError)
	}
	if mkdirError := os.MkdirAll(filepath.Dir(cfg.Dist.CriticalCSS()), 0o755); mkdirError != nil {
		t.Fatalf("mkdir critical css dir: %v", mkdirError)
	}
	if writeError := os.WriteFile(cfg.Dist.CriticalCSS(), []byte("body { color: blue; }"), 0o644); writeError != nil {
		t.Fatalf("write critical css: %v", writeError)
	}
	if writeError := os.WriteFile(cfg.Dist.NormalCSSRef(), []byte("normal/main.css"), 0o644); writeError != nil {
		t.Fatalf("write normal css ref: %v", writeError)
	}

	processor := css.NewProcessor(cfg, nil, nil)
	criticalCSS, readCriticalError := processor.ReadCriticalCSSHotReloadOutput(false)
	if readCriticalError != nil {
		t.Fatalf("ReadCriticalCSSHotReloadOutput(false) returned error: %v", readCriticalError)
	}
	if !strings.Contains(criticalCSS, "color: blue") {
		t.Fatalf("unexpected critical CSS output: %q", criticalCSS)
	}

	normalCSSURL, readNormalError := processor.ReadNormalCSSHotReloadURL(false)
	if readNormalError != nil {
		t.Fatalf("ReadNormalCSSHotReloadURL(false) returned error: %v", readNormalError)
	}
	if !strings.Contains(normalCSSURL, "normal/main.css") {
		t.Fatalf("unexpected normal CSS URL: %q", normalCSSURL)
	}
}

func TestProcessor_ListTrackedCriticalCSSImportPaths(t *testing.T) {
	cfg := newParsedConfigForCSSPackageTestsAtRoot(t.TempDir())
	processor := css.NewProcessor(cfg, nil, nil)

	firstPath := filepath.Join(t.TempDir(), "b.css")
	secondPath := filepath.Join(t.TempDir(), "a.css")
	processor.SetTrackedCriticalCSSImportPaths([]string{firstPath, secondPath})

	importPaths := processor.ListTrackedCriticalCSSImportPaths()
	if len(importPaths) != 2 {
		t.Fatalf("expected 2 tracked import paths, got %d", len(importPaths))
	}
	if importPaths[0] > importPaths[1] {
		t.Fatalf("expected sorted import paths, got %#v", importPaths)
	}
}

func TestProcessor_BuildCriticalAndNormalCSS(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForCSSPackageTestsAtRoot(root)
	cfg.Core.PublicPathPrefix = "/assets"
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical:    filepath.Join(root, "src", "critical.css"),
		NonCritical: filepath.Join(root, "src", "normal.css"),
	}

	if mkdirError := os.MkdirAll(filepath.Join(root, "src"), 0o755); mkdirError != nil {
		t.Fatalf("mkdir src dir: %v", mkdirError)
	}
	if writeError := os.WriteFile(
		cfg.Core.CSSEntryFiles.Critical,
		[]byte(`body { background-image: url("images/logo.png"); }`),
		0o644,
	); writeError != nil {
		t.Fatalf("write critical entry: %v", writeError)
	}
	if writeError := os.WriteFile(
		cfg.Core.CSSEntryFiles.NonCritical,
		[]byte(`.app { color: black; }`),
		0o644,
	); writeError != nil {
		t.Fatalf("write normal entry: %v", writeError)
	}

	processor := css.NewProcessor(
		cfg,
		nil,
		func(originalPath string) (string, bool, error) {
			if originalPath == "images/logo.png" {
				return "/assets/images/logo_hashed.png", true, nil
			}
			return "", false, nil
		},
	)

	if buildError := processor.BuildCriticalCSSOnly(); buildError != nil {
		t.Fatalf("BuildCriticalCSSOnly returned error: %v", buildError)
	}
	if buildError := processor.BuildNormalCSSOnly(); buildError != nil {
		t.Fatalf("BuildNormalCSSOnly returned error: %v", buildError)
	}

	criticalCSS, hasCriticalCSS := processor.CriticalCSS()
	if !hasCriticalCSS {
		t.Fatal("expected critical CSS cache to be populated")
	}
	if !strings.Contains(criticalCSS, "/assets/images/logo_hashed.png") {
		t.Fatalf("expected critical CSS to include resolved public URL, got %q", criticalCSS)
	}

	normalCSSURL, hasNormalCSSURL := processor.NormalCSSURL()
	if !hasNormalCSSURL {
		t.Fatal("expected normal CSS URL cache to be populated")
	}
	if !strings.HasPrefix(normalCSSURL, "/assets/") {
		t.Fatalf("expected normal CSS URL to be prefixed with public path, got %q", normalCSSURL)
	}

	refBytes, readRefError := os.ReadFile(cfg.Dist.NormalCSSRef())
	if readRefError != nil {
		t.Fatalf("read normal CSS ref: %v", readRefError)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(refBytes)), "vorma_internal_normal_") {
		t.Fatalf("unexpected normal CSS ref value %q", strings.TrimSpace(string(refBytes)))
	}
}

func TestProcessor_BuildCriticalCSSOnly_LeavesProtocolRelativeURLsUntouched(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForCSSPackageTestsAtRoot(root)
	cfg.Core.PublicPathPrefix = "/assets"
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical: filepath.Join(root, "src", "critical.css"),
	}

	if mkdirError := os.MkdirAll(filepath.Join(root, "src"), 0o755); mkdirError != nil {
		t.Fatalf("mkdir src dir: %v", mkdirError)
	}
	if writeError := os.WriteFile(
		cfg.Core.CSSEntryFiles.Critical,
		[]byte(`body { background-image: url("//cdn.example.com/logo.png"); }`),
		0o644,
	); writeError != nil {
		t.Fatalf("write critical entry: %v", writeError)
	}

	processor := css.NewProcessor(
		cfg,
		nil,
		func(originalPath string) (string, bool, error) {
			return "/assets/should-not-be-used.png", true, nil
		},
	)

	if buildError := processor.BuildCriticalCSSOnly(); buildError != nil {
		t.Fatalf("BuildCriticalCSSOnly returned error: %v", buildError)
	}

	criticalCSS, hasCriticalCSS := processor.CriticalCSS()
	if !hasCriticalCSS {
		t.Fatal("expected critical CSS cache to be populated")
	}
	if !strings.Contains(criticalCSS, `url(//cdn.example.com/logo.png)`) {
		t.Fatalf(
			"expected protocol-relative URL to remain unchanged, got %q",
			criticalCSS,
		)
	}
	if strings.Contains(criticalCSS, "/assets/should-not-be-used.png") {
		t.Fatalf(
			"expected protocol-relative URL to skip resolver callback, got %q",
			criticalCSS,
		)
	}
}

func TestProcessor_IsCSSFileAndImportTracking(t *testing.T) {
	root := t.TempDir()
	criticalPath := filepath.Join(root, "src", "critical.css")
	normalPath := filepath.Join(root, "src", "normal.css")
	criticalImportPath := filepath.Join(root, "src", "components", "critical_import.css")
	normalImportPath := filepath.Join(root, "src", "components", "normal_import.css")

	cfg := newParsedConfigForCSSPackageTestsAtRoot(root)
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical:    criticalPath,
		NonCritical: normalPath,
	}

	processor := css.NewProcessor(cfg, nil, nil)

	if !processor.IsCriticalCSSFile(criticalPath) {
		t.Fatalf("expected configured critical CSS entry to match: %q", criticalPath)
	}
	if !processor.IsNormalCSSFile(normalPath) {
		t.Fatalf("expected configured normal CSS entry to match: %q", normalPath)
	}
	if !processor.IsCSSFile(criticalPath) || !processor.IsCSSFile(normalPath) {
		t.Fatalf("expected configured entries to be recognized as CSS inputs")
	}
	if processor.IsCSSFile(filepath.Join(root, "src", "notes.txt")) {
		t.Fatal("did not expect non-css path to match CSS file detection")
	}

	processor.SetTrackedCriticalCSSImportPaths([]string{
		criticalImportPath,
		criticalImportPath,
		" ",
	})
	if !processor.IsCriticalCSSFile(criticalImportPath) {
		t.Fatalf("expected tracked critical import to match: %q", criticalImportPath)
	}
	if processor.CountTrackedCriticalCSSImportPaths() != 1 {
		t.Fatalf(
			"critical import count=%d, expected=1",
			processor.CountTrackedCriticalCSSImportPaths(),
		)
	}

	processor.SetTrackedNormalCSSImportPaths([]string{
		normalImportPath,
		normalImportPath,
		"\t",
	})
	if !processor.IsNormalCSSFile(normalImportPath) {
		t.Fatalf("expected tracked normal import to match: %q", normalImportPath)
	}
	if !processor.IsCSSFile(normalImportPath) {
		t.Fatalf("expected tracked normal import to match IsCSSFile: %q", normalImportPath)
	}
}

func TestProcessor_ReloadCachedOutputs(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForCSSPackageTestsAtRoot(root)
	cfg.Core.PublicPathPrefix = "/assets"

	if mkdirError := os.MkdirAll(filepath.Dir(cfg.Dist.CriticalCSS()), 0o755); mkdirError != nil {
		t.Fatalf("mkdir critical css dir: %v", mkdirError)
	}
	if writeError := os.WriteFile(cfg.Dist.CriticalCSS(), []byte(".app { color: red; }"), 0o644); writeError != nil {
		t.Fatalf("write critical css output: %v", writeError)
	}
	if writeError := os.WriteFile(cfg.Dist.NormalCSSRef(), []byte("css/app.css"), 0o644); writeError != nil {
		t.Fatalf("write normal css ref output: %v", writeError)
	}

	processor := css.NewProcessor(cfg, nil, nil)
	processor.ReloadCachedOutputs()

	criticalCSS, hasCriticalCSS := processor.CriticalCSS()
	if !hasCriticalCSS {
		t.Fatal("expected critical CSS cache after reload")
	}
	if !strings.Contains(criticalCSS, "color: red") {
		t.Fatalf("critical CSS cache=%q, expected reloaded file content", criticalCSS)
	}

	normalURL, hasNormalURL := processor.NormalCSSURL()
	if !hasNormalURL {
		t.Fatal("expected normal CSS URL cache after reload")
	}
	if normalURL != "/assets/css/app.css" {
		t.Fatalf("normal CSS URL cache=%q, expected /assets/css/app.css", normalURL)
	}
}

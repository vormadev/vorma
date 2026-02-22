package builder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave"
)

func TestBuildCriticalCSS_ResolvesPublicURLTokensUsingFileMap(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical: filepath.Join(root, "styles", "critical.css"),
	}
	cfg.Dist.Root = cfg.Core.DistDir

	if err := os.MkdirAll(filepath.Dir(cfg.Core.CSSEntryFiles.Critical), 0755); err != nil {
		t.Fatalf("failed creating CSS entry parent dir: %v", err)
	}
	cssContent := `.hero{background-image:url("images/logo.png");}`
	if err := os.WriteFile(cfg.Core.CSSEntryFiles.Critical, []byte(cssContent), 0644); err != nil {
		t.Fatalf("failed writing CSS entry file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
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

	if err := builder.buildCriticalCSS(true); err != nil {
		t.Fatalf("buildCriticalCSS returned error: %v", err)
	}

	output, err := os.ReadFile(cfg.Dist.CriticalCSS())
	if err != nil {
		t.Fatalf("failed reading generated critical CSS: %v", err)
	}
	outputCSS := string(output)

	if !strings.Contains(outputCSS, "/vorma_out_images_logo_deadbeef.png") {
		t.Fatalf(
			"expected output CSS to contain resolved hashed URL, got:\n%s",
			outputCSS,
		)
	}
}

func TestPublicURLBuildtimeCached_UsesCachedFileMapAfterFirstLoad(
	t *testing.T,
) {
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
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
		t.Fatalf(
			"expected cached lookup to remain stable after gob removal: first=%q second=%q",
			first,
			second,
		)
	}
}

func TestPublicURLBuildtimeCached_MissingFileMapPanics(t *testing.T) {
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	defer func() {
		if recover() == nil {
			t.Fatal(
				"expected getPublicURLBuildtimeCached to panic when file map is missing",
			)
		}
	}()

	_ = builder.getPublicURLBuildtimeCached("images/logo.png")
}

func TestCSSBuildAll_ReturnsCriticalErrorWithContext(t *testing.T) {
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical: filepath.Join(t.TempDir(), "missing-critical.css"),
	}
	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	err := builder.buildAllCSS(true)
	if err == nil {
		t.Fatal("expected buildAll to fail for missing critical CSS entry")
	}
	if !strings.Contains(err.Error(), "critical CSS") {
		t.Fatalf("unexpected critical buildAll error: %v", err)
	}
}

func TestCSSBuildAll_ReturnsNormalErrorWithContext(t *testing.T) {
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		NonCritical: filepath.Join(t.TempDir(), "missing-normal.css"),
	}
	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	err := builder.buildAllCSS(true)
	if err == nil {
		t.Fatal("expected buildAll to fail for missing normal CSS entry")
	}
	if !strings.Contains(err.Error(), "normal CSS") {
		t.Fatalf("unexpected normal buildAll error: %v", err)
	}
}

func TestBuildCriticalCSS_UnchangedInputDoesNotRewriteOutput(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical: filepath.Join(root, "styles", "critical.css"),
	}
	cfg.Dist.Root = cfg.Core.DistDir

	if err := os.MkdirAll(filepath.Dir(cfg.Core.CSSEntryFiles.Critical), 0o755); err != nil {
		t.Fatalf("failed creating CSS entry parent dir: %v", err)
	}
	if err := os.WriteFile(
		cfg.Core.CSSEntryFiles.Critical,
		[]byte(`body { color: red; }`),
		0o644,
	); err != nil {
		t.Fatalf("failed writing critical CSS entry file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	if err := builder.buildCriticalCSS(true); err != nil {
		t.Fatalf("initial buildCriticalCSS returned error: %v", err)
	}

	criticalOutputPath := cfg.Dist.CriticalCSS()
	initialInfo, err := os.Stat(criticalOutputPath)
	if err != nil {
		t.Fatalf("stat critical output after initial Build: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	if err := builder.buildCriticalCSS(true); err != nil {
		t.Fatalf("second buildCriticalCSS returned error: %v", err)
	}

	updatedInfo, err := os.Stat(criticalOutputPath)
	if err != nil {
		t.Fatalf("stat critical output after second Build: %v", err)
	}
	if !updatedInfo.ModTime().Equal(initialInfo.ModTime()) {
		t.Fatalf(
			"expected unchanged critical css output mtime, before=%v after=%v",
			initialInfo.ModTime(),
			updatedInfo.ModTime(),
		)
	}
}

func TestBuildNormalCSS_UnchangedInputDoesNotRewriteOutputArtifacts(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		NonCritical: filepath.Join(root, "styles", "normal.css"),
	}
	cfg.Dist.Root = cfg.Core.DistDir

	if err := os.MkdirAll(filepath.Dir(cfg.Core.CSSEntryFiles.NonCritical), 0o755); err != nil {
		t.Fatalf("failed creating CSS entry parent dir: %v", err)
	}
	if err := os.WriteFile(
		cfg.Core.CSSEntryFiles.NonCritical,
		[]byte(`body { color: blue; }`),
		0o644,
	); err != nil {
		t.Fatalf("failed writing normal CSS entry file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	if err := builder.buildNormalCSS(true); err != nil {
		t.Fatalf("initial buildNormalCSS returned error: %v", err)
	}

	normalCSSRefPath := cfg.Dist.NormalCSSRef()
	initialRefData, err := os.ReadFile(normalCSSRefPath)
	if err != nil {
		t.Fatalf("read normal css ref after initial Build: %v", err)
	}
	initialHashedOutputPath := filepath.Join(
		cfg.Dist.StaticPublic(),
		strings.TrimSpace(string(initialRefData)),
	)

	initialRefInfo, err := os.Stat(normalCSSRefPath)
	if err != nil {
		t.Fatalf("stat normal css ref after initial Build: %v", err)
	}
	initialOutputInfo, err := os.Stat(initialHashedOutputPath)
	if err != nil {
		t.Fatalf("stat normal css output after initial Build: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	if err := builder.buildNormalCSS(true); err != nil {
		t.Fatalf("second buildNormalCSS returned error: %v", err)
	}

	updatedRefInfo, err := os.Stat(normalCSSRefPath)
	if err != nil {
		t.Fatalf("stat normal css ref after second Build: %v", err)
	}
	updatedOutputInfo, err := os.Stat(initialHashedOutputPath)
	if err != nil {
		t.Fatalf("stat normal css output after second Build: %v", err)
	}

	if !updatedRefInfo.ModTime().Equal(initialRefInfo.ModTime()) {
		t.Fatalf(
			"expected unchanged normal css ref mtime, before=%v after=%v",
			initialRefInfo.ModTime(),
			updatedRefInfo.ModTime(),
		)
	}
	if !updatedOutputInfo.ModTime().Equal(initialOutputInfo.ModTime()) {
		t.Fatalf(
			"expected unchanged normal css output mtime, before=%v after=%v",
			initialOutputInfo.ModTime(),
			updatedOutputInfo.ModTime(),
		)
	}
}

func TestBuildNormalCSS_ChangedInputReplacesHashedArtifact(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		NonCritical: filepath.Join(root, "styles", "normal.css"),
	}
	cfg.Dist.Root = cfg.Core.DistDir

	if err := os.MkdirAll(filepath.Dir(cfg.Core.CSSEntryFiles.NonCritical), 0o755); err != nil {
		t.Fatalf("failed creating CSS entry parent dir: %v", err)
	}
	if err := os.WriteFile(
		cfg.Core.CSSEntryFiles.NonCritical,
		[]byte(`body { color: blue; }`),
		0o644,
	); err != nil {
		t.Fatalf("failed writing initial normal css entry file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	if err := builder.buildNormalCSS(true); err != nil {
		t.Fatalf("initial buildNormalCSS returned error: %v", err)
	}

	initialRefData, err := os.ReadFile(cfg.Dist.NormalCSSRef())
	if err != nil {
		t.Fatalf("read normal css ref after initial Build: %v", err)
	}
	initialHashedOutputName := strings.TrimSpace(string(initialRefData))
	initialHashedOutputPath := filepath.Join(
		cfg.Dist.StaticPublic(),
		initialHashedOutputName,
	)

	if err := os.WriteFile(
		cfg.Core.CSSEntryFiles.NonCritical,
		[]byte(`body { color: green; }`),
		0o644,
	); err != nil {
		t.Fatalf("failed writing updated normal css entry file: %v", err)
	}

	if err := builder.buildNormalCSS(true); err != nil {
		t.Fatalf("second buildNormalCSS returned error: %v", err)
	}

	updatedRefData, err := os.ReadFile(cfg.Dist.NormalCSSRef())
	if err != nil {
		t.Fatalf("read normal css ref after second Build: %v", err)
	}
	updatedHashedOutputName := strings.TrimSpace(string(updatedRefData))
	updatedHashedOutputPath := filepath.Join(
		cfg.Dist.StaticPublic(),
		updatedHashedOutputName,
	)

	if updatedHashedOutputName == initialHashedOutputName {
		t.Fatalf(
			"expected normal css hash output name to change, both were %q",
			updatedHashedOutputName,
		)
	}
	if _, statError := os.Stat(updatedHashedOutputPath); statError != nil {
		t.Fatalf(
			"expected updated normal css output to exist, stat error: %v",
			statError,
		)
	}
	if _, statError := os.Stat(initialHashedOutputPath); !os.IsNotExist(
		statError,
	) {
		t.Fatalf(
			"expected initial normal css output to be removed, stat error: %v",
			statError,
		)
	}
}

func TestBuildCriticalCSS_EmptyEntryClearsTrackedImports(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical: filepath.Join(root, "styles", "critical.css"),
	}
	cfg.Dist.Root = cfg.Core.DistDir

	if err := os.MkdirAll(filepath.Dir(cfg.Core.CSSEntryFiles.Critical), 0o755); err != nil {
		t.Fatalf("failed creating critical css entry parent dir: %v", err)
	}
	if err := os.WriteFile(
		cfg.Core.CSSEntryFiles.Critical,
		[]byte(`body { color: red; }`),
		0o644,
	); err != nil {
		t.Fatalf("failed writing critical css entry file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	if err := builder.buildCriticalCSS(true); err != nil {
		t.Fatalf("initial buildCriticalCSS returned error: %v", err)
	}

	initialTrackedImportCount := builder.countTrackedCriticalCSSImportPaths()
	if initialTrackedImportCount == 0 {
		t.Fatal("expected critical css imports to be tracked after build")
	}

	cfg.Core.CSSEntryFiles.Critical = ""
	if err := builder.buildCriticalCSS(true); err != nil {
		t.Fatalf("buildCriticalCSS with empty entry returned error: %v", err)
	}

	updatedTrackedImportCount := builder.countTrackedCriticalCSSImportPaths()
	if updatedTrackedImportCount != 0 {
		t.Fatalf(
			"expected critical css imports to be cleared when entry is unset, got %d",
			updatedTrackedImportCount,
		)
	}
}

func TestReadCriticalCSSForHotReload_RequiresFreshOutputAfterFailedRebuild(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical: filepath.Join(root, "styles", "critical.css"),
	}
	cfg.Dist.Root = cfg.Core.DistDir

	if err := os.MkdirAll(filepath.Dir(cfg.Core.CSSEntryFiles.Critical), 0o755); err != nil {
		t.Fatalf("failed creating critical css entry parent dir: %v", err)
	}
	if err := os.WriteFile(
		cfg.Core.CSSEntryFiles.Critical,
		[]byte(`body { color: red; }`),
		0o644,
	); err != nil {
		t.Fatalf("failed writing critical css entry file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	if err := builder.buildCriticalCSS(true); err != nil {
		t.Fatalf("initial buildCriticalCSS returned error: %v", err)
	}

	previousCriticalCSSOutput, readError := builder.ReadCriticalCSSForHotReload(
		false,
	)
	if readError != nil {
		t.Fatalf(
			"ReadCriticalCSSForHotReload(false) after initial build returned error: %v",
			readError,
		)
	}

	cfg.Core.CSSEntryFiles.Critical = filepath.Join(
		root,
		"styles",
		"missing-critical.css",
	)
	if err := builder.buildCriticalCSS(true); err == nil {
		t.Fatal("expected buildCriticalCSS to fail when entry file is missing")
	}

	if _, readFreshError := builder.ReadCriticalCSSForHotReload(true); readFreshError == nil {
		t.Fatal(
			"expected ReadCriticalCSSForHotReload(true) to fail after failed rebuild",
		)
	}

	fallbackCriticalCSSOutput, fallbackReadError := builder.ReadCriticalCSSForHotReload(
		false,
	)
	if fallbackReadError != nil {
		t.Fatalf(
			"ReadCriticalCSSForHotReload(false) after failed rebuild returned error: %v",
			fallbackReadError,
		)
	}
	if fallbackCriticalCSSOutput != previousCriticalCSSOutput {
		t.Fatalf(
			"expected stale fallback critical css output to remain readable, before=%q after=%q",
			previousCriticalCSSOutput,
			fallbackCriticalCSSOutput,
		)
	}
}

func TestReadNormalCSSURLForHotReload_RequiresFreshOutputAfterFailedRebuild(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		NonCritical: filepath.Join(root, "styles", "normal.css"),
	}
	cfg.Dist.Root = cfg.Core.DistDir

	if err := os.MkdirAll(filepath.Dir(cfg.Core.CSSEntryFiles.NonCritical), 0o755); err != nil {
		t.Fatalf("failed creating normal css entry parent dir: %v", err)
	}
	if err := os.WriteFile(
		cfg.Core.CSSEntryFiles.NonCritical,
		[]byte(`body { color: blue; }`),
		0o644,
	); err != nil {
		t.Fatalf("failed writing normal css entry file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	if err := builder.buildNormalCSS(true); err != nil {
		t.Fatalf("initial buildNormalCSS returned error: %v", err)
	}

	previousNormalCSSURL, readError := builder.ReadNormalCSSURLForHotReload(
		false,
	)
	if readError != nil {
		t.Fatalf(
			"ReadNormalCSSURLForHotReload(false) after initial build returned error: %v",
			readError,
		)
	}

	cfg.Core.CSSEntryFiles.NonCritical = filepath.Join(
		root,
		"styles",
		"missing-normal.css",
	)
	if err := builder.buildNormalCSS(true); err == nil {
		t.Fatal("expected buildNormalCSS to fail when entry file is missing")
	}

	if _, readFreshError := builder.ReadNormalCSSURLForHotReload(true); readFreshError == nil {
		t.Fatal(
			"expected ReadNormalCSSURLForHotReload(true) to fail after failed rebuild",
		)
	}

	fallbackNormalCSSURL, fallbackReadError := builder.ReadNormalCSSURLForHotReload(
		false,
	)
	if fallbackReadError != nil {
		t.Fatalf(
			"ReadNormalCSSURLForHotReload(false) after failed rebuild returned error: %v",
			fallbackReadError,
		)
	}
	if fallbackNormalCSSURL != previousNormalCSSURL {
		t.Fatalf(
			"expected stale fallback normal css URL to remain readable, before=%q after=%q",
			previousNormalCSSURL,
			fallbackNormalCSSURL,
		)
	}
}

func TestReadNormalCSSURLForHotReload_NormalizesRefFilePath(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(root)
	cfg.Core.PublicPathPrefix = "/assets/"
	cfg.Dist.Root = cfg.Core.DistDir

	if err := SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}
	if err := os.WriteFile(cfg.Dist.NormalCSSRef(), []byte(" ../outside.css \n"), 0o644); err != nil {
		t.Fatalf("failed writing normal css ref file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	normalCSSURL, readError := builder.ReadNormalCSSURLForHotReload(false)
	if readError != nil {
		t.Fatalf("ReadNormalCSSURLForHotReload returned error: %v", readError)
	}
	if normalCSSURL != "/assets/outside.css" {
		t.Fatalf(
			"expected normalized normal css URL %q, got %q",
			"/assets/outside.css",
			normalCSSURL,
		)
	}

	normalCSSURLViaWrapper, wrapperReadError := builder.readNormalCSSURL()
	if wrapperReadError != nil {
		t.Fatalf("readNormalCSSURL returned error: %v", wrapperReadError)
	}
	if normalCSSURLViaWrapper != normalCSSURL {
		t.Fatalf(
			"expected readNormalCSSURL wrapper result %q, got %q",
			normalCSSURL,
			normalCSSURLViaWrapper,
		)
	}
}

func TestReadNormalCSSURLForHotReload_WhitespaceOnlyRefReturnsEmptyURL(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(root)
	cfg.Core.PublicPathPrefix = "/assets/"
	cfg.Dist.Root = cfg.Core.DistDir

	if err := SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}
	if err := os.WriteFile(cfg.Dist.NormalCSSRef(), []byte(" \n\t "), 0o644); err != nil {
		t.Fatalf("failed writing whitespace-only normal css ref file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	normalCSSURL, readError := builder.ReadNormalCSSURLForHotReload(false)
	if readError != nil {
		t.Fatalf("ReadNormalCSSURLForHotReload returned error: %v", readError)
	}
	if normalCSSURL != "" {
		t.Fatalf(
			"expected empty normal css URL for whitespace-only ref file, got %q",
			normalCSSURL,
		)
	}
}

func TestReadCriticalCSS_ReadsFromDist(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(root)
	cfg.Dist.Root = cfg.Core.DistDir

	if err := SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}
	if err := os.WriteFile(cfg.Dist.CriticalCSS(), []byte("body { color: navy; }"), 0o644); err != nil {
		t.Fatalf("failed writing critical css file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	criticalCSS, readError := builder.readCriticalCSS()
	if readError != nil {
		t.Fatalf("readCriticalCSS returned error: %v", readError)
	}
	if criticalCSS != "body { color: navy; }" {
		t.Fatalf("unexpected critical css output %q", criticalCSS)
	}
}

func TestCSSHotReloadCaches_DoNotLeakAcrossBuilderReplacement(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical:    filepath.Join(root, "styles", "critical.css"),
		NonCritical: filepath.Join(root, "styles", "normal.css"),
	}
	cfg.Dist.Root = cfg.Core.DistDir

	if err := os.MkdirAll(filepath.Dir(cfg.Core.CSSEntryFiles.Critical), 0o755); err != nil {
		t.Fatalf("failed creating css entry parent dir: %v", err)
	}
	if err := os.WriteFile(cfg.Core.CSSEntryFiles.Critical, []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatalf("failed writing critical css entry file: %v", err)
	}
	if err := os.WriteFile(cfg.Core.CSSEntryFiles.NonCritical, []byte("body { color: blue; }"), 0o644); err != nil {
		t.Fatalf("failed writing normal css entry file: %v", err)
	}

	firstBuilder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer firstBuilder.Close()

	if err := firstBuilder.buildCriticalCSS(true); err != nil {
		t.Fatalf("first builder buildCriticalCSS returned error: %v", err)
	}
	if err := firstBuilder.buildNormalCSS(true); err != nil {
		t.Fatalf("first builder buildNormalCSS returned error: %v", err)
	}

	firstBuilderCriticalCSS, criticalReadError := firstBuilder.ReadCriticalCSSForHotReload(
		false,
	)
	if criticalReadError != nil {
		t.Fatalf(
			"first builder ReadCriticalCSSForHotReload(false) returned error: %v",
			criticalReadError,
		)
	}
	firstBuilderNormalCSSURL, normalReadError := firstBuilder.ReadNormalCSSURLForHotReload(
		false,
	)
	if normalReadError != nil {
		t.Fatalf(
			"first builder ReadNormalCSSURLForHotReload(false) returned error: %v",
			normalReadError,
		)
	}

	if err := os.WriteFile(cfg.Dist.CriticalCSS(), []byte("body { color: green; }"), 0o644); err != nil {
		t.Fatalf("failed writing overridden critical css dist file: %v", err)
	}
	if err := os.WriteFile(cfg.Dist.NormalCSSRef(), []byte("styles-overridden.css"), 0o644); err != nil {
		t.Fatalf("failed writing overridden normal css ref file: %v", err)
	}

	stillCachedCriticalCSS, stillCachedCriticalReadError := firstBuilder.ReadCriticalCSSForHotReload(
		false,
	)
	if stillCachedCriticalReadError != nil {
		t.Fatalf(
			"first builder cached critical css read returned error: %v",
			stillCachedCriticalReadError,
		)
	}
	if stillCachedCriticalCSS != firstBuilderCriticalCSS {
		t.Fatalf(
			"expected first builder to keep in-memory critical css cache, before=%q after=%q",
			firstBuilderCriticalCSS,
			stillCachedCriticalCSS,
		)
	}

	stillCachedNormalCSSURL, stillCachedNormalReadError := firstBuilder.ReadNormalCSSURLForHotReload(
		false,
	)
	if stillCachedNormalReadError != nil {
		t.Fatalf(
			"first builder cached normal css URL read returned error: %v",
			stillCachedNormalReadError,
		)
	}
	if stillCachedNormalCSSURL != firstBuilderNormalCSSURL {
		t.Fatalf(
			"expected first builder to keep in-memory normal css url cache, before=%q after=%q",
			firstBuilderNormalCSSURL,
			stillCachedNormalCSSURL,
		)
	}

	secondBuilder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer secondBuilder.Close()

	secondBuilderCriticalCSS, secondCriticalReadError := secondBuilder.ReadCriticalCSSForHotReload(
		false,
	)
	if secondCriticalReadError != nil {
		t.Fatalf(
			"second builder ReadCriticalCSSForHotReload(false) returned error: %v",
			secondCriticalReadError,
		)
	}
	secondBuilderNormalCSSURL, secondNormalReadError := secondBuilder.ReadNormalCSSURLForHotReload(
		false,
	)
	if secondNormalReadError != nil {
		t.Fatalf(
			"second builder ReadNormalCSSURLForHotReload(false) returned error: %v",
			secondNormalReadError,
		)
	}

	if !strings.Contains(secondBuilderCriticalCSS, "green") {
		t.Fatalf(
			"expected second builder to read critical css from dist, got %q",
			secondBuilderCriticalCSS,
		)
	}
	if !strings.HasSuffix(secondBuilderNormalCSSURL, "styles-overridden.css") {
		t.Fatalf(
			"expected second builder to read normal css ref from dist, got %q",
			secondBuilderNormalCSSURL,
		)
	}
}

func TestIsCriticalCSSFile_RecognizesSymlinkAliasPath(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForBuilderBasicTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	stylesDirectoryPath := filepath.Join(root, "styles")
	criticalEntryPath := filepath.Join(stylesDirectoryPath, "critical.css")
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical: criticalEntryPath,
	}
	cfg.Dist.Root = cfg.Core.DistDir

	if err := os.MkdirAll(stylesDirectoryPath, 0o755); err != nil {
		t.Fatalf("failed creating styles directory: %v", err)
	}
	if err := os.WriteFile(criticalEntryPath, []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatalf("failed writing critical css entry file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLoggerForBuilderBasicTests())
	defer builder.Close()

	if err := builder.buildCriticalCSS(true); err != nil {
		t.Fatalf("buildCriticalCSS returned error: %v", err)
	}

	aliasDirectoryPath := filepath.Join(root, "styles-alias")
	if err := os.Symlink(stylesDirectoryPath, aliasDirectoryPath); err != nil {
		t.Fatalf("failed creating styles alias symlink: %v", err)
	}
	aliasCriticalPath := filepath.Join(aliasDirectoryPath, "critical.css")

	if !builder.IsCriticalCSSFile(aliasCriticalPath) {
		t.Fatalf(
			"expected symlink alias path %q to be recognized as critical css import",
			aliasCriticalPath,
		)
	}
	if !builder.isCSSFile(aliasCriticalPath) {
		t.Fatalf(
			"expected symlink alias path %q to be recognized as css import",
			aliasCriticalPath,
		)
	}
}

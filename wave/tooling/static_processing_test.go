package tooling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave"
)

func TestProcessPublicFilesOnly_HandlesHashedPrehashedAndNohashFiles(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(filepath.Join(publicDir, "images"), 0755); err != nil {
		t.Fatalf("failed creating images dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(publicDir, wave.PrehashedDirname), 0755); err != nil {
		t.Fatalf("failed creating prehashed dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(publicDir, wave.NohashDirname), 0755); err != nil {
		t.Fatalf("failed creating nohash dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(publicDir, "images", "logo.png"), []byte("logo"), 0644); err != nil {
		t.Fatalf("failed writing hashed candidate: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, wave.PrehashedDirname, "vendor.js"), []byte("vendor"), 0644); err != nil {
		t.Fatalf("failed writing prehashed file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, wave.NohashDirname, "raw.txt"), []byte("raw"), 0644); err != nil {
		t.Fatalf("failed writing nohash file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, ".DS_Store"), []byte("ignore"), 0644); err != nil {
		t.Fatalf("failed writing ignored file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("ProcessPublicFilesOnly returned error: %v", err)
	}

	fileMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap returned error: %v", err)
	}

	hashedEntry, hasHashed := fileMap["images/logo.png"]
	if !hasHashed {
		t.Fatalf("expected hashed entry for images/logo.png, map=%#v", fileMap)
	}
	if hashedEntry.IsPrehashed {
		t.Fatal("expected images/logo.png to be hashed output, not prehashed")
	}
	if !strings.HasPrefix(hashedEntry.DistName, wave.HashedOutputPrefix) || !strings.HasSuffix(hashedEntry.DistName, ".png") {
		t.Fatalf("unexpected hashed dist name: %q", hashedEntry.DistName)
	}

	prehashedEntry, hasPrehashed := fileMap["vendor.js"]
	if !hasPrehashed {
		t.Fatalf("expected prehashed entry for vendor.js, map=%#v", fileMap)
	}
	if !prehashedEntry.IsPrehashed || prehashedEntry.DistName != "vendor.js" {
		t.Fatalf("unexpected prehashed entry: %#v", prehashedEntry)
	}

	nohashEntry, hasNohash := fileMap["raw.txt"]
	if !hasNohash {
		t.Fatalf("expected nohash entry for raw.txt, map=%#v", fileMap)
	}
	if !nohashEntry.IsPrehashed || nohashEntry.DistName != "raw.txt" {
		t.Fatalf("unexpected nohash entry: %#v", nohashEntry)
	}

	if _, found := fileMap[".DS_Store"]; found {
		t.Fatal("did not expect ignored .DS_Store file in public file map")
	}

	expectedOutputs := []string{
		filepath.Join(cfg.Dist.StaticPublic(), hashedEntry.DistName),
		filepath.Join(cfg.Dist.StaticPublic(), prehashedEntry.DistName),
		filepath.Join(cfg.Dist.StaticPublic(), nohashEntry.DistName),
	}
	for _, outputPath := range expectedOutputs {
		if _, statErr := os.Stat(outputPath); statErr != nil {
			t.Fatalf("expected output file to exist: %s (error: %v)", outputPath, statErr)
		}
	}
}

func TestProcessPublicFilesOnly_GranularModeRemovesStaleOutputFiles(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}

	keptPath := filepath.Join(publicDir, "kept.txt")
	removedPath := filepath.Join(publicDir, "removed.txt")

	if err := os.WriteFile(keptPath, []byte("kept"), 0644); err != nil {
		t.Fatalf("failed writing kept file: %v", err)
	}
	if err := os.WriteFile(removedPath, []byte("removed"), 0644); err != nil {
		t.Fatalf("failed writing removed file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPublicFilesOnly returned error: %v", err)
	}

	initialMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap after initial build returned error: %v", err)
	}
	removedDistName := initialMap["removed.txt"].DistName
	removedDistPath := filepath.Join(cfg.Dist.StaticPublic(), removedDistName)

	if err := os.Remove(removedPath); err != nil {
		t.Fatalf("failed removing source file for stale cleanup test: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("second ProcessPublicFilesOnly returned error: %v", err)
	}

	updatedMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap after second build returned error: %v", err)
	}
	if _, found := updatedMap["removed.txt"]; found {
		t.Fatalf("expected removed.txt to be dropped from map, got %#v", updatedMap["removed.txt"])
	}

	if _, statErr := os.Stat(removedDistPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected stale dist file to be deleted, stat error: %v", statErr)
	}
}

func TestProcessPrivateFilesOnly_PreservesRelativePaths(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	privateDir := cfg.Core.StaticAssetDirs.Private
	privateFile := filepath.Join(privateDir, "templates", "home.html")
	if err := os.MkdirAll(filepath.Dir(privateFile), 0755); err != nil {
		t.Fatalf("failed creating private file dir: %v", err)
	}
	if err := os.WriteFile(privateFile, []byte("<h1>home</h1>"), 0644); err != nil {
		t.Fatalf("failed writing private file: %v", err)
	}

	if err := builder.ProcessPrivateFilesOnly(); err != nil {
		t.Fatalf("ProcessPrivateFilesOnly returned error: %v", err)
	}

	privateMap, err := builder.loadFileMapFromPath(cfg.Dist.PrivateFileMapGob())
	if err != nil {
		t.Fatalf("loadFileMapFromPath(private gob) returned error: %v", err)
	}

	entry, found := privateMap["templates/home.html"]
	if !found {
		t.Fatalf("expected private map entry for templates/home.html, map=%#v", privateMap)
	}
	if entry.DistName != "templates/home.html" {
		t.Fatalf("expected private dist name to preserve relative path, got %q", entry.DistName)
	}

	distPath := filepath.Join(cfg.Dist.StaticPrivate(), "templates", "home.html")
	if _, statErr := os.Stat(distPath); statErr != nil {
		t.Fatalf("expected private dist file to exist at %s: %v", distPath, statErr)
	}
}

func TestProcessPublicFilesOnly_RecopiesUnchangedFileWhenDistOutputIsMissing(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}

	sourcePath := filepath.Join(publicDir, "logo.png")
	if err := os.WriteFile(sourcePath, []byte("logo"), 0644); err != nil {
		t.Fatalf("failed writing source file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPublicFilesOnly returned error: %v", err)
	}

	fileMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap returned error: %v", err)
	}
	entry, exists := fileMap["logo.png"]
	if !exists {
		t.Fatalf("expected file map entry for logo.png, map=%#v", fileMap)
	}

	distPath := filepath.Join(cfg.Dist.StaticPublic(), entry.DistName)
	if err := os.Remove(distPath); err != nil {
		t.Fatalf("failed removing dist output to simulate partial cleanup: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("second ProcessPublicFilesOnly returned error: %v", err)
	}

	if _, statErr := os.Stat(distPath); statErr != nil {
		t.Fatalf("expected missing dist output to be recopied, stat error: %v", statErr)
	}
}

func TestProcessPublicFilesOnly_UnchangedInputsDoNotRewriteMapArtifacts(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "logo.png"), []byte("logo"), 0644); err != nil {
		t.Fatalf("failed writing source file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPublicFilesOnly returned error: %v", err)
	}

	refPath := cfg.Dist.PublicFileMapRef()
	refBefore, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("failed reading file map ref: %v", err)
	}

	jsPath := filepath.Join(cfg.Dist.StaticPublic(), strings.TrimSpace(string(refBefore)))
	refInfoBefore, err := os.Stat(refPath)
	if err != nil {
		t.Fatalf("failed stating file map ref: %v", err)
	}
	jsInfoBefore, err := os.Stat(jsPath)
	if err != nil {
		t.Fatalf("failed stating hashed file map artifact: %v", err)
	}
	gobInfoBefore, err := os.Stat(cfg.Dist.PublicFileMapGob())
	if err != nil {
		t.Fatalf("failed stating public file map gob: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("second ProcessPublicFilesOnly returned error: %v", err)
	}

	refInfoAfter, err := os.Stat(refPath)
	if err != nil {
		t.Fatalf("failed stating file map ref after rerun: %v", err)
	}
	jsInfoAfter, err := os.Stat(jsPath)
	if err != nil {
		t.Fatalf("failed stating hashed file map artifact after rerun: %v", err)
	}
	gobInfoAfter, err := os.Stat(cfg.Dist.PublicFileMapGob())
	if err != nil {
		t.Fatalf("failed stating public file map gob after rerun: %v", err)
	}

	if !refInfoAfter.ModTime().Equal(refInfoBefore.ModTime()) {
		t.Fatalf(
			"expected unchanged ref file mtime, before=%v after=%v",
			refInfoBefore.ModTime(),
			refInfoAfter.ModTime(),
		)
	}
	if !jsInfoAfter.ModTime().Equal(jsInfoBefore.ModTime()) {
		t.Fatalf(
			"expected unchanged js artifact mtime, before=%v after=%v",
			jsInfoBefore.ModTime(),
			jsInfoAfter.ModTime(),
		)
	}
	if !gobInfoAfter.ModTime().Equal(gobInfoBefore.ModTime()) {
		t.Fatalf(
			"expected unchanged gob artifact mtime, before=%v after=%v",
			gobInfoBefore.ModTime(),
			gobInfoAfter.ModTime(),
		)
	}
}

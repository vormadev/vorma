package tooling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave"
)

func TestDetermineStaticProcessingWorkerCount(t *testing.T) {
	tests := []struct {
		name            string
		gomaxprocs      int
		expectedWorkers int
	}{
		{
			name:            "non-positive gomaxprocs falls back to one worker",
			gomaxprocs:      0,
			expectedWorkers: 1,
		},
		{
			name:            "single gomaxprocs uses two workers",
			gomaxprocs:      1,
			expectedWorkers: 2,
		},
		{
			name:            "scales with gomaxprocs",
			gomaxprocs:      4,
			expectedWorkers: 8,
		},
		{
			name:            "caps at thirty-two workers",
			gomaxprocs:      64,
			expectedWorkers: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workerCount := determineStaticProcessingWorkerCount(tt.gomaxprocs)
			if workerCount != tt.expectedWorkers {
				t.Fatalf(
					"determineStaticProcessingWorkerCount(%d)=%d, want %d",
					tt.gomaxprocs,
					workerCount,
					tt.expectedWorkers,
				)
			}
		})
	}
}

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

func TestProcessPublicFilesOnly_ReturnsErrorWhenLogicalPathCollidesAcrossSourceLocations(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	prehashedDir := filepath.Join(publicDir, wave.PrehashedDirname)
	if err := os.MkdirAll(prehashedDir, 0o755); err != nil {
		t.Fatalf("failed creating prehashed dir: %v", err)
	}

	regularPath := filepath.Join(publicDir, "logo.svg")
	prehashedPath := filepath.Join(prehashedDir, "logo.svg")
	if err := os.WriteFile(regularPath, []byte("regular"), 0o644); err != nil {
		t.Fatalf("failed writing regular static file: %v", err)
	}
	if err := os.WriteFile(prehashedPath, []byte("prehashed"), 0o644); err != nil {
		t.Fatalf("failed writing prehashed static file: %v", err)
	}

	err := builder.ProcessPublicFilesOnly()
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), `static source path collision for logical path "logo.svg"`) {
		t.Fatalf("expected collision error, got %v", err)
	}
}

func TestProcessPublicFilesOnlyForChangedPaths_UpdatesOnlyChangedEntries(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}

	changedFilePath := filepath.Join(publicDir, "changed.txt")
	stableFilePath := filepath.Join(publicDir, "stable.txt")
	if err := os.WriteFile(changedFilePath, []byte("v1"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}
	if err := os.WriteFile(stableFilePath, []byte("stable"), 0o644); err != nil {
		t.Fatalf("failed writing stable file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPublicFilesOnly returned error: %v", err)
	}

	initialMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap after initial run returned error: %v", err)
	}
	initialChangedEntry := initialMap["changed.txt"]
	initialStableEntry := initialMap["stable.txt"]

	if err := os.WriteFile(changedFilePath, []byte("v2"), 0o644); err != nil {
		t.Fatalf("failed rewriting changed file: %v", err)
	}

	if err := builder.processPublicFilesOnlyForChangedPaths([]string{changedFilePath}); err != nil {
		t.Fatalf("processPublicFilesOnlyForChangedPaths returned error: %v", err)
	}

	updatedMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap after changed-path run returned error: %v", err)
	}

	updatedChangedEntry := updatedMap["changed.txt"]
	updatedStableEntry := updatedMap["stable.txt"]
	if updatedStableEntry.DistName != initialStableEntry.DistName {
		t.Fatalf(
			"expected unchanged stable entry dist name %q, got %q",
			initialStableEntry.DistName,
			updatedStableEntry.DistName,
		)
	}
	if updatedChangedEntry.DistName == initialChangedEntry.DistName {
		t.Fatalf(
			"expected changed entry dist name to change, both were %q",
			updatedChangedEntry.DistName,
		)
	}

	oldChangedDistPath := filepath.Join(cfg.Dist.StaticPublic(), initialChangedEntry.DistName)
	if _, statErr := os.Stat(oldChangedDistPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected old changed dist file to be removed, stat error: %v", statErr)
	}
}

func TestProcessPublicFilesOnlyForChangedPaths_RemovesDeletedEntry(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}

	removedFilePath := filepath.Join(publicDir, "removed.txt")
	if err := os.WriteFile(removedFilePath, []byte("remove-me"), 0o644); err != nil {
		t.Fatalf("failed writing removable source file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPublicFilesOnly returned error: %v", err)
	}

	initialMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap after initial run returned error: %v", err)
	}
	initialRemovedEntry := initialMap["removed.txt"]
	initialRemovedDistPath := filepath.Join(cfg.Dist.StaticPublic(), initialRemovedEntry.DistName)

	if err := os.Remove(removedFilePath); err != nil {
		t.Fatalf("failed removing source file: %v", err)
	}

	if err := builder.processPublicFilesOnlyForChangedPaths([]string{removedFilePath}); err != nil {
		t.Fatalf("processPublicFilesOnlyForChangedPaths returned error: %v", err)
	}

	updatedMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap after changed-path delete returned error: %v", err)
	}
	if _, exists := updatedMap["removed.txt"]; exists {
		t.Fatalf("expected removed.txt to be absent from map, got %#v", updatedMap["removed.txt"])
	}

	if _, statErr := os.Stat(initialRemovedDistPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected removed dist file to be deleted, stat error: %v", statErr)
	}
}

func TestProcessPublicFilesOnlyForChangedPaths_ReturnsErrorWhenCollisionExists(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	prehashedDir := filepath.Join(publicDir, wave.PrehashedDirname)
	if err := os.MkdirAll(prehashedDir, 0o755); err != nil {
		t.Fatalf("failed creating prehashed dir: %v", err)
	}

	regularPath := filepath.Join(publicDir, "logo.svg")
	if err := os.WriteFile(regularPath, []byte("regular"), 0o644); err != nil {
		t.Fatalf("failed writing regular static file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPublicFilesOnly returned error: %v", err)
	}

	prehashedPath := filepath.Join(prehashedDir, "logo.svg")
	if err := os.WriteFile(prehashedPath, []byte("prehashed"), 0o644); err != nil {
		t.Fatalf("failed writing prehashed static file: %v", err)
	}

	err := builder.processPublicFilesOnlyForChangedPaths([]string{prehashedPath})
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), `static source path collision for logical path "logo.svg"`) {
		t.Fatalf("expected collision error, got %v", err)
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

func TestProcessPrivateFilesOnlyForChangedPaths_RemovesDeletedEntry(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	privateDir := cfg.Core.StaticAssetDirs.Private
	privateFilePath := filepath.Join(privateDir, "templates", "gone.html")
	if err := os.MkdirAll(filepath.Dir(privateFilePath), 0o755); err != nil {
		t.Fatalf("failed creating private source dir: %v", err)
	}
	if err := os.WriteFile(privateFilePath, []byte("<h1>gone</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing private source file: %v", err)
	}

	if err := builder.ProcessPrivateFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPrivateFilesOnly returned error: %v", err)
	}

	initialMap, err := builder.loadFileMapFromPath(cfg.Dist.PrivateFileMapGob())
	if err != nil {
		t.Fatalf("loadFileMapFromPath returned error: %v", err)
	}
	initialEntry := initialMap["templates/gone.html"]
	initialDistPath := filepath.Join(cfg.Dist.StaticPrivate(), initialEntry.DistName)

	if err := os.Remove(privateFilePath); err != nil {
		t.Fatalf("failed removing private source file: %v", err)
	}

	if err := builder.processPrivateFilesOnlyForChangedPaths([]string{privateFilePath}); err != nil {
		t.Fatalf("processPrivateFilesOnlyForChangedPaths returned error: %v", err)
	}

	updatedMap, err := builder.loadFileMapFromPath(cfg.Dist.PrivateFileMapGob())
	if err != nil {
		t.Fatalf("loadFileMapFromPath after changed-path delete returned error: %v", err)
	}
	if _, exists := updatedMap["templates/gone.html"]; exists {
		t.Fatalf("expected templates/gone.html to be absent from map, got %#v", updatedMap["templates/gone.html"])
	}

	if _, statErr := os.Stat(initialDistPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected private dist file to be deleted, stat error: %v", statErr)
	}
}

func TestProcessPrivateFilesOnly_ReturnsErrorWhenLogicalPathCollidesAcrossSourceLocations(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	privateDir := cfg.Core.StaticAssetDirs.Private
	prehashedDir := filepath.Join(privateDir, wave.PrehashedDirname)
	if err := os.MkdirAll(prehashedDir, 0o755); err != nil {
		t.Fatalf("failed creating prehashed dir: %v", err)
	}

	regularPath := filepath.Join(privateDir, "templates", "home.html")
	prehashedPath := filepath.Join(prehashedDir, "templates", "home.html")
	if err := os.MkdirAll(filepath.Dir(regularPath), 0o755); err != nil {
		t.Fatalf("failed creating regular private file parent dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(prehashedPath), 0o755); err != nil {
		t.Fatalf("failed creating prehashed private file parent dir: %v", err)
	}
	if err := os.WriteFile(regularPath, []byte("<h1>regular</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing regular private file: %v", err)
	}
	if err := os.WriteFile(prehashedPath, []byte("<h1>prehashed</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing prehashed private file: %v", err)
	}

	err := builder.ProcessPrivateFilesOnly()
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), `static source path collision for logical path "templates/home.html"`) {
		t.Fatalf("expected collision error, got %v", err)
	}
}

func TestProcessPrivateFilesOnlyForChangedPaths_ReturnsErrorWhenCollisionExists(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	privateDir := cfg.Core.StaticAssetDirs.Private
	prehashedDir := filepath.Join(privateDir, wave.PrehashedDirname)
	if err := os.MkdirAll(prehashedDir, 0o755); err != nil {
		t.Fatalf("failed creating prehashed dir: %v", err)
	}

	regularPath := filepath.Join(privateDir, "templates", "home.html")
	if err := os.MkdirAll(filepath.Dir(regularPath), 0o755); err != nil {
		t.Fatalf("failed creating private file parent dir: %v", err)
	}
	if err := os.WriteFile(regularPath, []byte("<h1>regular</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing regular private file: %v", err)
	}

	if err := builder.ProcessPrivateFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPrivateFilesOnly returned error: %v", err)
	}

	prehashedPath := filepath.Join(prehashedDir, "templates", "home.html")
	if err := os.MkdirAll(filepath.Dir(prehashedPath), 0o755); err != nil {
		t.Fatalf("failed creating prehashed private file parent dir: %v", err)
	}
	if err := os.WriteFile(prehashedPath, []byte("<h1>prehashed</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing prehashed private file: %v", err)
	}

	err := builder.processPrivateFilesOnlyForChangedPaths([]string{prehashedPath})
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), `static source path collision for logical path "templates/home.html"`) {
		t.Fatalf("expected collision error, got %v", err)
	}
}

func TestProcessPublicFilesOnlyForChangedPaths_OutsideStaticRootIsNoOp(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}

	publicFilePath := filepath.Join(publicDir, "logo.png")
	if err := os.WriteFile(publicFilePath, []byte("logo"), 0o644); err != nil {
		t.Fatalf("failed writing public file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPublicFilesOnly returned error: %v", err)
	}

	initialMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap after initial run returned error: %v", err)
	}
	initialEntry := initialMap["logo.png"]

	gobInfoBefore, err := os.Stat(cfg.Dist.PublicFileMapGob())
	if err != nil {
		t.Fatalf("stat public file map gob before no-op run: %v", err)
	}
	refDataBefore, err := os.ReadFile(cfg.Dist.PublicFileMapRef())
	if err != nil {
		t.Fatalf("read public file map ref before no-op run: %v", err)
	}
	refInfoBefore, err := os.Stat(cfg.Dist.PublicFileMapRef())
	if err != nil {
		t.Fatalf("stat public file map ref before no-op run: %v", err)
	}
	jsPath := filepath.Join(cfg.Dist.StaticPublic(), strings.TrimSpace(string(refDataBefore)))
	jsInfoBefore, err := os.Stat(jsPath)
	if err != nil {
		t.Fatalf("stat hashed public file map JS before no-op run: %v", err)
	}

	outsidePath := filepath.Join(root, "not-static", "note.txt")
	if err := os.MkdirAll(filepath.Dir(outsidePath), 0o755); err != nil {
		t.Fatalf("failed creating outside path parent dir: %v", err)
	}
	if err := os.WriteFile(outsidePath, []byte("note"), 0o644); err != nil {
		t.Fatalf("failed writing outside path file: %v", err)
	}

	if err := builder.processPublicFilesOnlyForChangedPaths([]string{outsidePath}); err != nil {
		t.Fatalf("processPublicFilesOnlyForChangedPaths returned error: %v", err)
	}

	updatedMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap after no-op run returned error: %v", err)
	}
	updatedEntry := updatedMap["logo.png"]
	if updatedEntry != initialEntry {
		t.Fatalf("expected map entry unchanged after outside-root change, before=%#v after=%#v", initialEntry, updatedEntry)
	}

	gobInfoAfter, err := os.Stat(cfg.Dist.PublicFileMapGob())
	if err != nil {
		t.Fatalf("stat public file map gob after no-op run: %v", err)
	}
	refInfoAfter, err := os.Stat(cfg.Dist.PublicFileMapRef())
	if err != nil {
		t.Fatalf("stat public file map ref after no-op run: %v", err)
	}
	jsInfoAfter, err := os.Stat(jsPath)
	if err != nil {
		t.Fatalf("stat hashed public file map JS after no-op run: %v", err)
	}

	if !gobInfoAfter.ModTime().Equal(gobInfoBefore.ModTime()) {
		t.Fatalf(
			"expected unchanged gob mtime for outside-root no-op, before=%v after=%v",
			gobInfoBefore.ModTime(),
			gobInfoAfter.ModTime(),
		)
	}
	if !refInfoAfter.ModTime().Equal(refInfoBefore.ModTime()) {
		t.Fatalf(
			"expected unchanged ref mtime for outside-root no-op, before=%v after=%v",
			refInfoBefore.ModTime(),
			refInfoAfter.ModTime(),
		)
	}
	if !jsInfoAfter.ModTime().Equal(jsInfoBefore.ModTime()) {
		t.Fatalf(
			"expected unchanged js mtime for outside-root no-op, before=%v after=%v",
			jsInfoBefore.ModTime(),
			jsInfoAfter.ModTime(),
		)
	}
}

func TestProcessPrivateFilesOnlyForChangedPaths_RemovesDirectorySubtreeEntries(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	privateDir := cfg.Core.StaticAssetDirs.Private
	removedSubtreeDirectory := filepath.Join(privateDir, "templates", "removed")
	keptFilePath := filepath.Join(privateDir, "templates", "kept.html")
	removedFilePathA := filepath.Join(removedSubtreeDirectory, "a.html")
	removedFilePathB := filepath.Join(removedSubtreeDirectory, "nested", "b.html")

	if err := os.MkdirAll(filepath.Dir(keptFilePath), 0o755); err != nil {
		t.Fatalf("failed creating kept file parent dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(removedFilePathB), 0o755); err != nil {
		t.Fatalf("failed creating removed subtree parent dir: %v", err)
	}
	if err := os.WriteFile(keptFilePath, []byte("<h1>keep</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing kept file: %v", err)
	}
	if err := os.WriteFile(removedFilePathA, []byte("<h1>a</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing removed file a: %v", err)
	}
	if err := os.WriteFile(removedFilePathB, []byte("<h1>b</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing removed file b: %v", err)
	}

	if err := builder.ProcessPrivateFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPrivateFilesOnly returned error: %v", err)
	}

	initialMap, err := builder.loadFileMapFromPath(cfg.Dist.PrivateFileMapGob())
	if err != nil {
		t.Fatalf("loadFileMapFromPath after initial run returned error: %v", err)
	}
	removedEntryA := initialMap["templates/removed/a.html"]
	removedEntryB := initialMap["templates/removed/nested/b.html"]
	removedDistPathA := filepath.Join(cfg.Dist.StaticPrivate(), removedEntryA.DistName)
	removedDistPathB := filepath.Join(cfg.Dist.StaticPrivate(), removedEntryB.DistName)

	if err := os.RemoveAll(removedSubtreeDirectory); err != nil {
		t.Fatalf("failed removing private subtree directory: %v", err)
	}

	if err := builder.processPrivateFilesOnlyForChangedPaths([]string{removedSubtreeDirectory}); err != nil {
		t.Fatalf("processPrivateFilesOnlyForChangedPaths for subtree delete returned error: %v", err)
	}

	updatedMap, err := builder.loadFileMapFromPath(cfg.Dist.PrivateFileMapGob())
	if err != nil {
		t.Fatalf("loadFileMapFromPath after subtree delete returned error: %v", err)
	}
	if _, exists := updatedMap["templates/removed/a.html"]; exists {
		t.Fatalf("expected templates/removed/a.html to be removed from map, got %#v", updatedMap["templates/removed/a.html"])
	}
	if _, exists := updatedMap["templates/removed/nested/b.html"]; exists {
		t.Fatalf(
			"expected templates/removed/nested/b.html to be removed from map, got %#v",
			updatedMap["templates/removed/nested/b.html"],
		)
	}
	if _, exists := updatedMap["templates/kept.html"]; !exists {
		t.Fatalf("expected templates/kept.html to remain in map, got %#v", updatedMap)
	}

	if _, statErr := os.Stat(removedDistPathA); !os.IsNotExist(statErr) {
		t.Fatalf("expected removed dist file a to be deleted, stat error: %v", statErr)
	}
	if _, statErr := os.Stat(removedDistPathB); !os.IsNotExist(statErr) {
		t.Fatalf("expected removed dist file b to be deleted, stat error: %v", statErr)
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

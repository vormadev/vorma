package static_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/wave/buildtime/builder"
	"github.com/vormadev/vorma/wave/buildtime/builder/internal/static"
	"github.com/vormadev/vorma/wave/internal/wavefilemap"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveenv"
)

func TestDetermineStaticProcessingWorkerCount(t *testing.T) {
	tests := []struct {
		Name            string
		Gomaxprocs      int
		ExpectedWorkers int
	}{
		{
			Name:            "non-positive gomaxprocs falls back to one worker",
			Gomaxprocs:      0,
			ExpectedWorkers: 1,
		},
		{
			Name:            "single gomaxprocs uses two workers",
			Gomaxprocs:      1,
			ExpectedWorkers: 2,
		},
		{
			Name:            "scales with gomaxprocs",
			Gomaxprocs:      4,
			ExpectedWorkers: 8,
		},
		{
			Name:            "caps at thirty-two workers",
			Gomaxprocs:      64,
			ExpectedWorkers: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			workerCount := static.DetermineStaticProcessingWorkerCount(
				tt.Gomaxprocs,
			)
			if workerCount != tt.ExpectedWorkers {
				t.Fatalf(
					"static.DetermineStaticProcessingWorkerCount(%d)=%d, want %d",
					tt.Gomaxprocs,
					workerCount,
					tt.ExpectedWorkers,
				)
			}
		})
	}
}

func TestCanonicalizePathForLocationComparison_FollowsParentSymlinkForMissingLeaf(
	t *testing.T,
) {
	root := t.TempDir()
	targetDirectoryPath := filepath.Join(root, "target")
	aliasDirectoryPath := filepath.Join(root, "alias")
	missingLeafUnderTarget := filepath.Join(
		targetDirectoryPath,
		"styles",
		"site.css",
	)
	missingLeafUnderAlias := filepath.Join(
		aliasDirectoryPath,
		"styles",
		"site.css",
	)

	if err := os.MkdirAll(filepath.Join(targetDirectoryPath, "styles"), 0o755); err != nil {
		t.Fatalf("failed creating target styles directory: %v", err)
	}
	if err := os.Symlink(targetDirectoryPath, aliasDirectoryPath); err != nil {
		t.Fatalf("failed creating source alias directory symlink: %v", err)
	}

	canonicalizedPath := waveenv.CanonicalizePathForLocationComparison(
		missingLeafUnderAlias,
	)
	if !waveenv.PathsReferToSameLocation(
		canonicalizedPath,
		missingLeafUnderTarget,
	) {
		t.Fatalf(
			"expected canonicalized alias path %q to resolve to %q, got %q",
			missingLeafUnderAlias,
			missingLeafUnderTarget,
			canonicalizedPath,
		)
	}
}

func TestResolveStaticChangedPathResolutions_UsesSymlinkAliasPathsForMissingLeafs(
	t *testing.T,
) {
	root := t.TempDir()
	sourceDirectoryPath := filepath.Join(root, "source")
	aliasDirectoryPath := filepath.Join(root, "source_alias")
	missingAliasPath := filepath.Join(aliasDirectoryPath, "styles", "site.css")

	if err := os.MkdirAll(filepath.Join(sourceDirectoryPath, "styles"), 0o755); err != nil {
		t.Fatalf("failed creating source styles directory: %v", err)
	}
	if err := os.Symlink(sourceDirectoryPath, aliasDirectoryPath); err != nil {
		t.Fatalf("failed creating source alias directory symlink: %v", err)
	}

	changedResolutions, fullBuildRequired, resolutionError := static.ResolveStaticChangedPathResolutions(
		sourceDirectoryPath,
		[]string{missingAliasPath},
	)
	if resolutionError != nil {
		t.Fatalf(
			"resolveStaticChangedPathResolutions returned error: %v",
			resolutionError,
		)
	}
	if fullBuildRequired {
		t.Fatal(
			"expected fullBuildRequired=false for missing leaf changed path",
		)
	}
	if len(changedResolutions) != 1 {
		t.Fatalf(
			"expected one changed-path resolution, got %#v",
			changedResolutions,
		)
	}

	resolution, hasResolution := changedResolutions["styles/site.css"]
	if !hasResolution {
		t.Fatalf(
			"expected changed resolution for styles/site.css, got %#v",
			changedResolutions,
		)
	}
	if resolution.SourceExists {
		t.Fatalf(
			"expected sourceExists=false for missing leaf path, got %#v",
			resolution,
		)
	}

	changedResolutions, fullBuildRequired, resolutionError = static.ResolveStaticChangedPathResolutions(
		sourceDirectoryPath,
		[]string{aliasDirectoryPath},
	)
	if resolutionError != nil {
		t.Fatalf(
			"static.ResolveStaticChangedPathResolutions(root alias) returned error: %v",
			resolutionError,
		)
	}
	if !fullBuildRequired {
		t.Fatalf(
			"expected root alias path %q to require full build for source root %q",
			aliasDirectoryPath,
			sourceDirectoryPath,
		)
	}
	if len(changedResolutions) != 0 {
		t.Fatalf(
			"expected no per-file resolutions when fullBuildRequired=true, got %#v",
			changedResolutions,
		)
	}
}

func TestResolveStaticChangedPathResolutions_DirectoryChangedPathFullBuildBehavior(
	t *testing.T,
) {
	sourceDirectoryPath := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceDirectoryPath, 0o755); err != nil {
		t.Fatalf("failed creating source directory: %v", err)
	}

	emptyDirectoryChangedPath := filepath.Join(sourceDirectoryPath, "empty-dir")
	if err := os.MkdirAll(emptyDirectoryChangedPath, 0o755); err != nil {
		t.Fatalf("failed creating empty changed directory: %v", err)
	}

	changedResolutions, fullBuildRequired, resolutionError := static.ResolveStaticChangedPathResolutions(
		sourceDirectoryPath,
		[]string{emptyDirectoryChangedPath},
	)
	if resolutionError != nil {
		t.Fatalf(
			"ResolveStaticChangedPathResolutions returned error: %v",
			resolutionError,
		)
	}
	if fullBuildRequired {
		t.Fatalf(
			"expected empty directory changed path %q not to require full build",
			emptyDirectoryChangedPath,
		)
	}
	if len(changedResolutions) != 1 {
		t.Fatalf(
			"expected one resolution for empty directory changed path, got %#v",
			changedResolutions,
		)
	}
	if resolution := changedResolutions["empty-dir"]; resolution.SourceExists {
		t.Fatalf(
			"expected empty directory resolution to have SourceExists=false, got %#v",
			resolution,
		)
	}

	ignoredOnlyDirectoryChangedPath := filepath.Join(
		sourceDirectoryPath,
		"ignored-only-dir",
	)
	if err := os.MkdirAll(ignoredOnlyDirectoryChangedPath, 0o755); err != nil {
		t.Fatalf("failed creating ignored-only changed directory: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(ignoredOnlyDirectoryChangedPath, ".DS_Store"),
		[]byte("ignored"),
		0o644,
	); err != nil {
		t.Fatalf(
			"failed writing ignored-only changed directory fixture file: %v",
			err,
		)
	}

	changedResolutions, fullBuildRequired, resolutionError = static.ResolveStaticChangedPathResolutions(
		sourceDirectoryPath,
		[]string{ignoredOnlyDirectoryChangedPath},
	)
	if resolutionError != nil {
		t.Fatalf(
			"ResolveStaticChangedPathResolutions returned error: %v",
			resolutionError,
		)
	}
	if fullBuildRequired {
		t.Fatalf(
			"expected ignored-only directory changed path %q not to require full build",
			ignoredOnlyDirectoryChangedPath,
		)
	}
	if len(changedResolutions) != 1 {
		t.Fatalf(
			"expected one resolution for ignored-only directory changed path, got %#v",
			changedResolutions,
		)
	}
	if resolution := changedResolutions["ignored-only-dir"]; resolution.SourceExists {
		t.Fatalf(
			"expected ignored-only directory resolution to have SourceExists=false, got %#v",
			resolution,
		)
	}

	nonEmptyDirectoryChangedPath := filepath.Join(
		sourceDirectoryPath,
		"non-empty-dir",
	)
	if err := os.MkdirAll(nonEmptyDirectoryChangedPath, 0o755); err != nil {
		t.Fatalf("failed creating non-empty changed directory: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(nonEmptyDirectoryChangedPath, "asset.txt"),
		[]byte("asset"),
		0o644,
	); err != nil {
		t.Fatalf(
			"failed writing non-empty changed directory fixture file: %v",
			err,
		)
	}

	changedResolutions, fullBuildRequired, resolutionError = static.ResolveStaticChangedPathResolutions(
		sourceDirectoryPath,
		[]string{nonEmptyDirectoryChangedPath},
	)
	if resolutionError != nil {
		t.Fatalf(
			"ResolveStaticChangedPathResolutions returned error: %v",
			resolutionError,
		)
	}
	if !fullBuildRequired {
		t.Fatalf(
			"expected non-empty directory changed path %q to require full build",
			nonEmptyDirectoryChangedPath,
		)
	}
	if len(changedResolutions) != 0 {
		t.Fatalf(
			"expected no per-file resolutions when fullBuildRequired=true, got %#v",
			changedResolutions,
		)
	}
}

func TestResolveStaticChangedPathResolutionsWithProbeFunctions_MemoizesDuplicateAndAliasChangedPaths(
	t *testing.T,
) {
	sourceDirectoryPath := "/workspace/public"
	logicalSourcePath := "/workspace/public/images/logo.png"

	resolveProbeCallCount := 0
	sourceExistsProbeCallCount := 0
	collisionProbeCallCount := 0

	changedResolutions, fullBuildRequired, resolutionError := static.ResolveStaticChangedPathResolutionsWithProbeFunctions(
		sourceDirectoryPath,
		[]string{"logo", "logo_alias", "logo_dup"},
		static.StaticChangedPathResolutionProbeFunctions{
			NormalizeChangedSourcePath: func(changedSourcePath string) string {
				switch changedSourcePath {
				case "logo", "logo_alias", "logo_dup":
					return logicalSourcePath
				default:
					return ""
				}
			},
			ResolveStaticFileInfoFromSourcePath: func(
				_ string,
				sourcePath string,
			) (static.StaticFileInfo, bool, error) {
				resolveProbeCallCount++
				return static.StaticFileInfo{
					SourcePath:   sourcePath,
					RelativePath: "images/logo.png",
				}, true, nil
			},
			SourceFileExists: func(_ string) (bool, error) {
				sourceExistsProbeCallCount++
				return true, nil
			},
			EnsureNoStaticLogicalPathCollisionWithinSourceDirectoryForRelativePath: func(
				_ string,
				_ string,
			) error {
				collisionProbeCallCount++
				return nil
			},
		},
	)
	if resolutionError != nil {
		t.Fatalf(
			"resolveStaticChangedPathResolutionsWithProbeFunctions returned error: %v",
			resolutionError,
		)
	}
	if fullBuildRequired {
		t.Fatal("expected fullBuildRequired=false for file-only changed paths")
	}
	if len(changedResolutions) != 1 {
		t.Fatalf("expected one changed resolution, got %#v", changedResolutions)
	}

	resolvedFileInfo := changedResolutions["images/logo.png"]
	if resolvedFileInfo.FileInfo.SourcePath != logicalSourcePath {
		t.Fatalf(
			"expected resolved sourcePath %q, got %#v",
			logicalSourcePath,
			resolvedFileInfo.FileInfo,
		)
	}
	if !resolvedFileInfo.SourceExists {
		t.Fatalf("expected sourceExists=true, got %#v", resolvedFileInfo)
	}

	if resolveProbeCallCount != 1 {
		t.Fatalf(
			"expected resolve probe to run once for alias-equivalent changed paths, got %d",
			resolveProbeCallCount,
		)
	}
	if sourceExistsProbeCallCount != 1 {
		t.Fatalf(
			"expected source-exists probe to run once for alias-equivalent changed paths, got %d",
			sourceExistsProbeCallCount,
		)
	}
	if collisionProbeCallCount != 1 {
		t.Fatalf(
			"expected collision probe to run once for alias-equivalent changed paths, got %d",
			collisionProbeCallCount,
		)
	}
}

func TestResolveStaticChangedPathResolutionsWithProbeFunctions_ProbesCollisionOncePerRelativePath(
	t *testing.T,
) {
	sourceDirectoryPath := "/workspace/public"

	resolveProbeCallCount := 0
	sourceExistsProbeCallCount := 0
	collisionProbeCallCount := 0

	_, fullBuildRequired, resolutionError := static.ResolveStaticChangedPathResolutionsWithProbeFunctions(
		sourceDirectoryPath,
		[]string{"prehashed_path", "nohash_path"},
		static.StaticChangedPathResolutionProbeFunctions{
			NormalizeChangedSourcePath: func(changedSourcePath string) string {
				switch changedSourcePath {
				case "prehashed_path":
					return filepath.Join(
						sourceDirectoryPath,
						waveartifacts.PrehashedDirname,
						"logo.png",
					)
				case "nohash_path":
					return filepath.Join(
						sourceDirectoryPath,
						waveartifacts.NohashDirname,
						"logo.png",
					)
				default:
					return ""
				}
			},
			ResolveStaticFileInfoFromSourcePath: func(
				_ string,
				sourcePath string,
			) (static.StaticFileInfo, bool, error) {
				resolveProbeCallCount++
				if strings.Contains(sourcePath, waveartifacts.PrehashedDirname+"/") ||
					strings.Contains(sourcePath, waveartifacts.NohashDirname+"/") {
					return static.StaticFileInfo{
						SourcePath:   sourcePath,
						RelativePath: "logo.png",
						IsPrehashed:  true,
					}, true, nil
				}
				return static.StaticFileInfo{}, false, nil
			},
			SourceFileExists: func(_ string) (bool, error) {
				sourceExistsProbeCallCount++
				return true, nil
			},
			EnsureNoStaticLogicalPathCollisionWithinSourceDirectoryForRelativePath: func(
				_ string,
				_ string,
			) error {
				collisionProbeCallCount++
				return nil
			},
		},
	)
	if resolutionError != nil {
		t.Fatalf(
			"resolveStaticChangedPathResolutionsWithProbeFunctions returned error: %v",
			resolutionError,
		)
	}
	if fullBuildRequired {
		t.Fatal("expected fullBuildRequired=false for file-only changed paths")
	}
	if resolveProbeCallCount != 2 {
		t.Fatalf(
			"expected resolve probe to run once per unique normalized path, got %d",
			resolveProbeCallCount,
		)
	}
	if sourceExistsProbeCallCount != 2 {
		t.Fatalf(
			"expected source-exists probe to run once per unique normalized path, got %d",
			sourceExistsProbeCallCount,
		)
	}
	if collisionProbeCallCount != 1 {
		t.Fatalf(
			"expected collision probe to run once per relative path, got %d",
			collisionProbeCallCount,
		)
	}
}

func TestRemoveStaticMapEntriesForChangedRelativePaths_MixedExactAndSubtreeRemovals(
	t *testing.T,
) {
	distDirectoryPath := t.TempDir()
	staticMap := wavefilemap.FileMap{
		"templates/keep.html": {
			DistName: "templates/keep-dist.html",
		},
		"templates/removed/a.html": {
			DistName: "templates/removed-a-dist.html",
		},
		"templates/removed/nested/b.html": {
			DistName: "templates/removed-b-dist.html",
		},
		"images/logo.svg": {
			DistName: "images/logo-dist.svg",
		},
	}

	for _, mapValue := range staticMap {
		distArtifactPath := filepath.Join(distDirectoryPath, mapValue.DistName)
		if err := os.MkdirAll(filepath.Dir(distArtifactPath), 0o755); err != nil {
			t.Fatalf("failed creating dist artifact parent dir: %v", err)
		}
		if err := os.WriteFile(distArtifactPath, []byte("artifact"), 0o644); err != nil {
			t.Fatalf("failed writing dist artifact: %v", err)
		}
	}

	mapWasChanged := static.RemoveStaticMapEntriesForChangedRelativePaths(
		staticMap,
		distDirectoryPath,
		[]string{"templates/removed", "images/logo.svg"},
	)
	if !mapWasChanged {
		t.Fatal(
			"expected static map to change for mixed exact/subtree removals",
		)
	}

	if len(staticMap) != 1 {
		t.Fatalf("expected one remaining map entry, got %#v", staticMap)
	}
	if _, hasRemainingKeptPath := staticMap["templates/keep.html"]; !hasRemainingKeptPath {
		t.Fatalf("expected kept path to remain in map, got %#v", staticMap)
	}
	if _, hasRemovedSubtreePath := staticMap["templates/removed/a.html"]; hasRemovedSubtreePath {
		t.Fatalf(
			"expected subtree path templates/removed/a.html to be removed, got %#v",
			staticMap,
		)
	}
	if _, hasRemovedNestedSubtreePath := staticMap["templates/removed/nested/b.html"]; hasRemovedNestedSubtreePath {
		t.Fatalf(
			"expected subtree path templates/removed/nested/b.html to be removed, got %#v",
			staticMap,
		)
	}
	if _, hasRemovedExactPath := staticMap["images/logo.svg"]; hasRemovedExactPath {
		t.Fatalf(
			"expected exact path images/logo.svg to be removed, got %#v",
			staticMap,
		)
	}

	if _, statErr := os.Stat(filepath.Join(distDirectoryPath, "templates/keep-dist.html")); statErr != nil {
		t.Fatalf(
			"expected kept dist artifact to remain, stat error: %v",
			statErr,
		)
	}
	if _, statErr := os.Stat(filepath.Join(distDirectoryPath, "templates/removed-a-dist.html")); !os.IsNotExist(
		statErr,
	) {
		t.Fatalf(
			"expected removed dist artifact templates/removed-a-dist.html to be deleted, stat error: %v",
			statErr,
		)
	}
	if _, statErr := os.Stat(filepath.Join(distDirectoryPath, "templates/removed-b-dist.html")); !os.IsNotExist(
		statErr,
	) {
		t.Fatalf(
			"expected removed dist artifact templates/removed-b-dist.html to be deleted, stat error: %v",
			statErr,
		)
	}
	if _, statErr := os.Stat(filepath.Join(distDirectoryPath, "images/logo-dist.svg")); !os.IsNotExist(
		statErr,
	) {
		t.Fatalf(
			"expected removed dist artifact images/logo-dist.svg to be deleted, stat error: %v",
			statErr,
		)
	}
}

func TestRemoveStaticMapEntriesForChangedRelativePaths_NoMatchesNoChanges(
	t *testing.T,
) {
	distDirectoryPath := t.TempDir()
	staticMap := wavefilemap.FileMap{
		"templates/keep.html": {
			DistName: "templates/keep-dist.html",
		},
	}

	distArtifactPath := filepath.Join(
		distDirectoryPath,
		"templates/keep-dist.html",
	)
	if err := os.MkdirAll(filepath.Dir(distArtifactPath), 0o755); err != nil {
		t.Fatalf("failed creating dist artifact parent dir: %v", err)
	}
	if err := os.WriteFile(distArtifactPath, []byte("artifact"), 0o644); err != nil {
		t.Fatalf("failed writing dist artifact: %v", err)
	}

	mapWasChanged := static.RemoveStaticMapEntriesForChangedRelativePaths(
		staticMap,
		distDirectoryPath,
		[]string{"images"},
	)
	if mapWasChanged {
		t.Fatal(
			"expected static map to remain unchanged for non-matching changed path",
		)
	}

	if len(staticMap) != 1 {
		t.Fatalf("expected map size to remain 1, got %#v", staticMap)
	}
	if _, stillHasKeptPath := staticMap["templates/keep.html"]; !stillHasKeptPath {
		t.Fatalf("expected kept path to remain, got %#v", staticMap)
	}
	if _, statErr := os.Stat(distArtifactPath); statErr != nil {
		t.Fatalf(
			"expected kept dist artifact to remain, stat error: %v",
			statErr,
		)
	}
}

func TestProcessPublicFilesOnly_HandlesHashedPrehashedAndNohashFiles(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(filepath.Join(publicDir, "images"), 0755); err != nil {
		t.Fatalf("failed creating images dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(publicDir, waveartifacts.PrehashedDirname), 0755); err != nil {
		t.Fatalf("failed creating prehashed dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(publicDir, waveartifacts.NohashDirname), 0755); err != nil {
		t.Fatalf("failed creating nohash dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(publicDir, "images", "logo.png"), []byte("logo"), 0644); err != nil {
		t.Fatalf("failed writing hashed candidate: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, waveartifacts.PrehashedDirname, "vendor.js"), []byte("vendor"), 0644); err != nil {
		t.Fatalf("failed writing prehashed file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, waveartifacts.NohashDirname, "raw.txt"), []byte("raw"), 0644); err != nil {
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
	if !strings.HasPrefix(hashedEntry.DistName, waveartifacts.HashedOutputPrefix) ||
		!strings.HasSuffix(hashedEntry.DistName, ".png") {
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
			t.Fatalf(
				"expected output file to exist: %s (error: %v)",
				outputPath,
				statErr,
			)
		}
	}
}

func TestProcessPublicFilesOnly_GranularModeRemovesStaleOutputFiles(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
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
		t.Fatalf(
			"LoadPublicFileMap after initial build returned error: %v",
			err,
		)
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
		t.Fatalf(
			"expected removed.txt to be dropped from map, got %#v",
			updatedMap["removed.txt"],
		)
	}

	if _, statErr := os.Stat(removedDistPath); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected stale dist file to be deleted, stat error: %v",
			statErr,
		)
	}
}

func TestProcessPublicFilesOnly_SourceDirectoryRemovalCleansDistArtifacts(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("failed creating public source dir: %v", err)
	}

	removedFilePath := filepath.Join(publicDir, "removed.txt")
	if err := os.WriteFile(removedFilePath, []byte("removed"), 0o644); err != nil {
		t.Fatalf("failed writing removable source file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPublicFilesOnly returned error: %v", err)
	}

	initialMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf(
			"LoadPublicFileMap after initial build returned error: %v",
			err,
		)
	}
	initialEntry := initialMap["removed.txt"]
	initialDistPath := filepath.Join(
		cfg.Dist.StaticPublic(),
		initialEntry.DistName,
	)

	if err := os.RemoveAll(publicDir); err != nil {
		t.Fatalf("failed removing public source directory: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf(
			"ProcessPublicFilesOnly after source dir removal returned error: %v",
			err,
		)
	}

	updatedMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf(
			"LoadPublicFileMap after source dir removal returned error: %v",
			err,
		)
	}
	if len(updatedMap) != 0 {
		t.Fatalf(
			"expected empty public file map after source dir removal, got %#v",
			updatedMap,
		)
	}

	if _, statErr := os.Stat(initialDistPath); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected dist artifact to be deleted after source dir removal, stat error: %v",
			statErr,
		)
	}
}

func TestProcessPublicFilesOnly_ReturnsErrorWhenLogicalPathCollidesAcrossSourceLocations(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	prehashedDir := filepath.Join(publicDir, waveartifacts.PrehashedDirname)
	if err := os.MkdirAll(prehashedDir, 0o755); err != nil {
		t.Fatalf("failed creating prehashed dir: %v", err)
	}

	regularPath := filepath.Join(publicDir, "logo.svg")
	prehashedPath := filepath.Join(prehashedDir, "logo.svg")
	if err := os.WriteFile(regularPath, []byte("regular"), 0o644); err != nil {
		t.Fatalf("failed writing regular static file: %v", err)
	}
	if err := os.WriteFile(
		prehashedPath,
		[]byte(waveartifacts.PrehashedDirname),
		0o644,
	); err != nil {
		t.Fatalf("failed writing prehashed static file: %v", err)
	}

	err := builder.ProcessPublicFilesOnly()
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(
		err.Error(),
		`static source path collision for logical path "logo.svg"`,
	) {
		t.Fatalf("expected collision error, got %v", err)
	}
}

func TestProcessPublicFilesOnlyForChangedPaths_UpdatesOnlyChangedEntries(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
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

	if err := builder.ProcessPublicFilesOnlyForChangedPaths([]string{changedFilePath}); err != nil {
		t.Fatalf(
			"processPublicFilesOnlyForChangedPaths returned error: %v",
			err,
		)
	}

	updatedMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf(
			"LoadPublicFileMap after changed-path run returned error: %v",
			err,
		)
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

	oldChangedDistPath := filepath.Join(
		cfg.Dist.StaticPublic(),
		initialChangedEntry.DistName,
	)
	if _, statErr := os.Stat(oldChangedDistPath); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected old changed dist file to be removed, stat error: %v",
			statErr,
		)
	}
}

func TestProcessPublicFilesOnlyForChangedPaths_RemovesDeletedEntry(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
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
	initialRemovedDistPath := filepath.Join(
		cfg.Dist.StaticPublic(),
		initialRemovedEntry.DistName,
	)

	if err := os.Remove(removedFilePath); err != nil {
		t.Fatalf("failed removing source file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnlyForChangedPaths([]string{removedFilePath}); err != nil {
		t.Fatalf(
			"processPublicFilesOnlyForChangedPaths returned error: %v",
			err,
		)
	}

	updatedMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf(
			"LoadPublicFileMap after changed-path delete returned error: %v",
			err,
		)
	}
	if _, exists := updatedMap["removed.txt"]; exists {
		t.Fatalf(
			"expected removed.txt to be absent from map, got %#v",
			updatedMap["removed.txt"],
		)
	}

	if _, statErr := os.Stat(initialRemovedDistPath); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected removed dist file to be deleted, stat error: %v",
			statErr,
		)
	}
}

func TestProcessPublicFilesOnlyForChangedPaths_ReturnsErrorWhenCollisionExists(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	prehashedDir := filepath.Join(publicDir, waveartifacts.PrehashedDirname)
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
	if err := os.WriteFile(
		prehashedPath,
		[]byte(waveartifacts.PrehashedDirname),
		0o644,
	); err != nil {
		t.Fatalf("failed writing prehashed static file: %v", err)
	}

	err := builder.ProcessPublicFilesOnlyForChangedPaths(
		[]string{prehashedPath},
	)
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(
		err.Error(),
		`static source path collision for logical path "logo.svg"`,
	) {
		t.Fatalf("expected collision error, got %v", err)
	}
}

func TestProcessPublicFilesOnlyForChangedPaths_MixedCreateDeleteAndRenameLikeBatch(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}

	renamedFromPath := filepath.Join(publicDir, "old-logo.png")
	renamedToPath := filepath.Join(publicDir, "new-logo.png")
	deletedPath := filepath.Join(publicDir, "obsolete.txt")
	createdPath := filepath.Join(publicDir, "added.txt")

	if err := os.WriteFile(renamedFromPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("failed writing renamed source file: %v", err)
	}
	if err := os.WriteFile(deletedPath, []byte("obsolete"), 0o644); err != nil {
		t.Fatalf("failed writing deleted source file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPublicFilesOnly returned error: %v", err)
	}

	initialMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap after initial run returned error: %v", err)
	}
	initialRenamedFromEntry := initialMap["old-logo.png"]
	initialDeletedEntry := initialMap["obsolete.txt"]
	initialRenamedFromDistPath := filepath.Join(
		cfg.Dist.StaticPublic(),
		initialRenamedFromEntry.DistName,
	)
	initialDeletedDistPath := filepath.Join(
		cfg.Dist.StaticPublic(),
		initialDeletedEntry.DistName,
	)

	if err := os.Rename(renamedFromPath, renamedToPath); err != nil {
		t.Fatalf("failed renaming source file: %v", err)
	}
	if err := os.Remove(deletedPath); err != nil {
		t.Fatalf("failed removing source file: %v", err)
	}
	if err := os.WriteFile(createdPath, []byte("added"), 0o644); err != nil {
		t.Fatalf("failed writing created source file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnlyForChangedPaths([]string{
		renamedFromPath,
		renamedToPath,
		deletedPath,
		createdPath,
	}); err != nil {
		t.Fatalf(
			"processPublicFilesOnlyForChangedPaths returned error: %v",
			err,
		)
	}

	updatedMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf(
			"LoadPublicFileMap after changed-path batch returned error: %v",
			err,
		)
	}

	if _, exists := updatedMap["old-logo.png"]; exists {
		t.Fatalf(
			"expected renamed-from path to be removed, got %#v",
			updatedMap["old-logo.png"],
		)
	}
	if _, exists := updatedMap["obsolete.txt"]; exists {
		t.Fatalf(
			"expected deleted path to be removed, got %#v",
			updatedMap["obsolete.txt"],
		)
	}
	if _, exists := updatedMap["new-logo.png"]; !exists {
		t.Fatalf(
			"expected renamed-to path to exist in map, got %#v",
			updatedMap,
		)
	}
	if _, exists := updatedMap["added.txt"]; !exists {
		t.Fatalf("expected created path to exist in map, got %#v", updatedMap)
	}

	if _, statErr := os.Stat(initialRenamedFromDistPath); !os.IsNotExist(
		statErr,
	) {
		t.Fatalf(
			"expected old renamed dist artifact to be deleted, stat error: %v",
			statErr,
		)
	}
	if _, statErr := os.Stat(initialDeletedDistPath); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected deleted dist artifact to be deleted, stat error: %v",
			statErr,
		)
	}
}

func TestProcessPrivateFilesOnly_PreservesRelativePaths(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
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

	privateMap, err := loadFileMapFromPathForStaticProcessingTests(
		cfg,
		cfg.Dist.PrivateFileMapGob(),
	)
	if err != nil {
		t.Fatalf("loadFileMapFromPath(private gob) returned error: %v", err)
	}

	entry, found := privateMap["templates/home.html"]
	if !found {
		t.Fatalf(
			"expected private map entry for templates/home.html, map=%#v",
			privateMap,
		)
	}
	if entry.DistName != "templates/home.html" {
		t.Fatalf(
			"expected private dist name to preserve relative path, got %q",
			entry.DistName,
		)
	}

	distPath := filepath.Join(
		cfg.Dist.StaticPrivate(),
		"templates",
		"home.html",
	)
	if _, statErr := os.Stat(distPath); statErr != nil {
		t.Fatalf(
			"expected private dist file to exist at %s: %v",
			distPath,
			statErr,
		)
	}
}

func TestProcessPrivateFilesOnlyForChangedPaths_RemovesDeletedEntry(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
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

	initialMap, err := loadFileMapFromPathForStaticProcessingTests(
		cfg,
		cfg.Dist.PrivateFileMapGob(),
	)
	if err != nil {
		t.Fatalf("loadFileMapFromPath returned error: %v", err)
	}
	initialEntry := initialMap["templates/gone.html"]
	initialDistPath := filepath.Join(
		cfg.Dist.StaticPrivate(),
		initialEntry.DistName,
	)

	if err := os.Remove(privateFilePath); err != nil {
		t.Fatalf("failed removing private source file: %v", err)
	}

	if err := builder.ProcessPrivateFilesOnlyForChangedPaths([]string{privateFilePath}); err != nil {
		t.Fatalf(
			"processPrivateFilesOnlyForChangedPaths returned error: %v",
			err,
		)
	}

	updatedMap, err := loadFileMapFromPathForStaticProcessingTests(
		cfg,
		cfg.Dist.PrivateFileMapGob(),
	)
	if err != nil {
		t.Fatalf(
			"loadFileMapFromPath after changed-path delete returned error: %v",
			err,
		)
	}
	if _, exists := updatedMap["templates/gone.html"]; exists {
		t.Fatalf(
			"expected templates/gone.html to be absent from map, got %#v",
			updatedMap["templates/gone.html"],
		)
	}

	if _, statErr := os.Stat(initialDistPath); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected private dist file to be deleted, stat error: %v",
			statErr,
		)
	}
}

func TestProcessPrivateFilesOnly_SourceDirectoryRemovalCleansDistArtifacts(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	defer builder.Close()

	privateDir := cfg.Core.StaticAssetDirs.Private
	privateFilePath := filepath.Join(privateDir, "templates", "gone.html")
	if err := os.MkdirAll(filepath.Dir(privateFilePath), 0o755); err != nil {
		t.Fatalf("failed creating private source file parent dir: %v", err)
	}
	if err := os.WriteFile(privateFilePath, []byte("<h1>gone</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing private source file: %v", err)
	}

	if err := builder.ProcessPrivateFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPrivateFilesOnly returned error: %v", err)
	}

	initialMap, err := loadFileMapFromPathForStaticProcessingTests(
		cfg,
		cfg.Dist.PrivateFileMapGob(),
	)
	if err != nil {
		t.Fatalf(
			"loadFileMapFromPath after initial build returned error: %v",
			err,
		)
	}
	initialEntry := initialMap["templates/gone.html"]
	initialDistPath := filepath.Join(
		cfg.Dist.StaticPrivate(),
		initialEntry.DistName,
	)

	if err := os.RemoveAll(privateDir); err != nil {
		t.Fatalf("failed removing private source directory: %v", err)
	}

	if err := builder.ProcessPrivateFilesOnly(); err != nil {
		t.Fatalf(
			"ProcessPrivateFilesOnly after source dir removal returned error: %v",
			err,
		)
	}

	updatedMap, err := loadFileMapFromPathForStaticProcessingTests(
		cfg,
		cfg.Dist.PrivateFileMapGob(),
	)
	if err != nil {
		t.Fatalf(
			"loadFileMapFromPath after source dir removal returned error: %v",
			err,
		)
	}
	if len(updatedMap) != 0 {
		t.Fatalf(
			"expected empty private file map after source dir removal, got %#v",
			updatedMap,
		)
	}

	if _, statErr := os.Stat(initialDistPath); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected private dist artifact to be deleted after source dir removal, stat error: %v",
			statErr,
		)
	}
}

func TestProcessPrivateFilesOnly_ReturnsErrorWhenLogicalPathCollidesAcrossSourceLocations(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	defer builder.Close()

	privateDir := cfg.Core.StaticAssetDirs.Private
	prehashedDir := filepath.Join(privateDir, waveartifacts.PrehashedDirname)
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
	if !strings.Contains(
		err.Error(),
		`static source path collision for logical path "templates/home.html"`,
	) {
		t.Fatalf("expected collision error, got %v", err)
	}
}

func TestProcessPrivateFilesOnlyForChangedPaths_ReturnsErrorWhenCollisionExists(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	defer builder.Close()

	privateDir := cfg.Core.StaticAssetDirs.Private
	prehashedDir := filepath.Join(privateDir, waveartifacts.PrehashedDirname)
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

	err := builder.ProcessPrivateFilesOnlyForChangedPaths(
		[]string{prehashedPath},
	)
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(
		err.Error(),
		`static source path collision for logical path "templates/home.html"`,
	) {
		t.Fatalf("expected collision error, got %v", err)
	}
}

func TestProcessPrivateFilesOnlyForChangedPaths_MixedCreateDeleteAndRenameLikeBatch(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	defer builder.Close()

	privateDir := cfg.Core.StaticAssetDirs.Private
	if err := os.MkdirAll(privateDir, 0o755); err != nil {
		t.Fatalf("failed creating private dir: %v", err)
	}

	renamedFromPath := filepath.Join(privateDir, "templates", "old.html")
	renamedToPath := filepath.Join(privateDir, "templates", "new.html")
	deletedPath := filepath.Join(privateDir, "templates", "obsolete.html")
	createdPath := filepath.Join(privateDir, "templates", "added.html")

	if err := os.MkdirAll(filepath.Dir(renamedFromPath), 0o755); err != nil {
		t.Fatalf("failed creating private template dir: %v", err)
	}
	if err := os.WriteFile(renamedFromPath, []byte("<h1>old</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing renamed source file: %v", err)
	}
	if err := os.WriteFile(deletedPath, []byte("<h1>obsolete</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing deleted source file: %v", err)
	}

	if err := builder.ProcessPrivateFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPrivateFilesOnly returned error: %v", err)
	}

	initialMap, err := loadFileMapFromPathForStaticProcessingTests(
		cfg,
		cfg.Dist.PrivateFileMapGob(),
	)
	if err != nil {
		t.Fatalf(
			"loadFileMapFromPath after initial run returned error: %v",
			err,
		)
	}
	initialRenamedFromEntry := initialMap["templates/old.html"]
	initialDeletedEntry := initialMap["templates/obsolete.html"]
	initialRenamedFromDistPath := filepath.Join(
		cfg.Dist.StaticPrivate(),
		initialRenamedFromEntry.DistName,
	)
	initialDeletedDistPath := filepath.Join(
		cfg.Dist.StaticPrivate(),
		initialDeletedEntry.DistName,
	)

	if err := os.Rename(renamedFromPath, renamedToPath); err != nil {
		t.Fatalf("failed renaming source file: %v", err)
	}
	if err := os.Remove(deletedPath); err != nil {
		t.Fatalf("failed removing source file: %v", err)
	}
	if err := os.WriteFile(createdPath, []byte("<h1>added</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing created source file: %v", err)
	}

	if err := builder.ProcessPrivateFilesOnlyForChangedPaths([]string{
		renamedFromPath,
		renamedToPath,
		deletedPath,
		createdPath,
	}); err != nil {
		t.Fatalf(
			"processPrivateFilesOnlyForChangedPaths returned error: %v",
			err,
		)
	}

	updatedMap, err := loadFileMapFromPathForStaticProcessingTests(
		cfg,
		cfg.Dist.PrivateFileMapGob(),
	)
	if err != nil {
		t.Fatalf(
			"loadFileMapFromPath after changed-path batch returned error: %v",
			err,
		)
	}

	if _, exists := updatedMap["templates/old.html"]; exists {
		t.Fatalf(
			"expected renamed-from path to be removed, got %#v",
			updatedMap["templates/old.html"],
		)
	}
	if _, exists := updatedMap["templates/obsolete.html"]; exists {
		t.Fatalf(
			"expected deleted path to be removed, got %#v",
			updatedMap["templates/obsolete.html"],
		)
	}
	if _, exists := updatedMap["templates/new.html"]; !exists {
		t.Fatalf(
			"expected renamed-to path to exist in map, got %#v",
			updatedMap,
		)
	}
	if _, exists := updatedMap["templates/added.html"]; !exists {
		t.Fatalf("expected created path to exist in map, got %#v", updatedMap)
	}

	if _, statErr := os.Stat(initialRenamedFromDistPath); !os.IsNotExist(
		statErr,
	) {
		t.Fatalf(
			"expected old renamed dist artifact to be deleted, stat error: %v",
			statErr,
		)
	}
	if _, statErr := os.Stat(initialDeletedDistPath); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected deleted dist artifact to be deleted, stat error: %v",
			statErr,
		)
	}
}

func TestProcessPublicFilesOnlyForChangedPaths_OutsideStaticRootIsNoOp(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
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
	jsPath := filepath.Join(
		cfg.Dist.StaticPublic(),
		strings.TrimSpace(string(refDataBefore)),
	)
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

	if err := builder.ProcessPublicFilesOnlyForChangedPaths([]string{outsidePath}); err != nil {
		t.Fatalf(
			"processPublicFilesOnlyForChangedPaths returned error: %v",
			err,
		)
	}

	updatedMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap after no-op run returned error: %v", err)
	}
	updatedEntry := updatedMap["logo.png"]
	if updatedEntry != initialEntry {
		t.Fatalf(
			"expected map entry unchanged after outside-root change, before=%#v after=%#v",
			initialEntry,
			updatedEntry,
		)
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

func TestProcessPublicFilesOnlyForChangedPaths_OutsideStaticRootDoesNotBuildWhenMapMissing(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}

	publicFilePath := filepath.Join(publicDir, "logo.png")
	if err := os.WriteFile(publicFilePath, []byte("logo"), 0o644); err != nil {
		t.Fatalf("failed writing public file: %v", err)
	}

	outsidePath := filepath.Join(root, "not-static", "note.txt")
	if err := os.MkdirAll(filepath.Dir(outsidePath), 0o755); err != nil {
		t.Fatalf("failed creating outside path parent dir: %v", err)
	}
	if err := os.WriteFile(outsidePath, []byte("note"), 0o644); err != nil {
		t.Fatalf("failed writing outside path file: %v", err)
	}

	if err := builder.ProcessPublicFilesOnlyForChangedPaths([]string{outsidePath}); err != nil {
		t.Fatalf(
			"processPublicFilesOnlyForChangedPaths returned error: %v",
			err,
		)
	}

	if _, statErr := os.Stat(cfg.Dist.PublicFileMapGob()); !os.IsNotExist(
		statErr,
	) {
		t.Fatalf(
			"expected public file map gob to remain absent, stat error: %v",
			statErr,
		)
	}
	if _, statErr := os.Stat(cfg.Dist.PublicFileMapRef()); !os.IsNotExist(
		statErr,
	) {
		t.Fatalf(
			"expected public file map ref to remain absent, stat error: %v",
			statErr,
		)
	}

	publicDistEntries, readErr := os.ReadDir(cfg.Dist.StaticPublic())
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatalf("failed reading public dist dir: %v", readErr)
	}
	if readErr == nil && len(publicDistEntries) != 0 {
		t.Fatalf(
			"expected public dist dir to remain empty, got %d entries",
			len(publicDistEntries),
		)
	}
}

func TestProcessPrivateFilesOnlyForChangedPaths_RemovesDirectorySubtreeEntries(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	defer builder.Close()

	privateDir := cfg.Core.StaticAssetDirs.Private
	removedSubtreeDirectory := filepath.Join(privateDir, "templates", "removed")
	keptFilePath := filepath.Join(privateDir, "templates", "kept.html")
	removedFilePathA := filepath.Join(removedSubtreeDirectory, "a.html")
	removedFilePathB := filepath.Join(
		removedSubtreeDirectory,
		"nested",
		"b.html",
	)

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

	initialMap, err := loadFileMapFromPathForStaticProcessingTests(
		cfg,
		cfg.Dist.PrivateFileMapGob(),
	)
	if err != nil {
		t.Fatalf(
			"loadFileMapFromPath after initial run returned error: %v",
			err,
		)
	}
	removedEntryA := initialMap["templates/removed/a.html"]
	removedEntryB := initialMap["templates/removed/nested/b.html"]
	removedDistPathA := filepath.Join(
		cfg.Dist.StaticPrivate(),
		removedEntryA.DistName,
	)
	removedDistPathB := filepath.Join(
		cfg.Dist.StaticPrivate(),
		removedEntryB.DistName,
	)

	if err := os.RemoveAll(removedSubtreeDirectory); err != nil {
		t.Fatalf("failed removing private subtree directory: %v", err)
	}

	if err := builder.ProcessPrivateFilesOnlyForChangedPaths([]string{removedSubtreeDirectory}); err != nil {
		t.Fatalf(
			"processPrivateFilesOnlyForChangedPaths for subtree delete returned error: %v",
			err,
		)
	}

	updatedMap, err := loadFileMapFromPathForStaticProcessingTests(
		cfg,
		cfg.Dist.PrivateFileMapGob(),
	)
	if err != nil {
		t.Fatalf(
			"loadFileMapFromPath after subtree delete returned error: %v",
			err,
		)
	}
	if _, exists := updatedMap["templates/removed/a.html"]; exists {
		t.Fatalf(
			"expected templates/removed/a.html to be removed from map, got %#v",
			updatedMap["templates/removed/a.html"],
		)
	}
	if _, exists := updatedMap["templates/removed/nested/b.html"]; exists {
		t.Fatalf(
			"expected templates/removed/nested/b.html to be removed from map, got %#v",
			updatedMap["templates/removed/nested/b.html"],
		)
	}
	if _, exists := updatedMap["templates/kept.html"]; !exists {
		t.Fatalf(
			"expected templates/kept.html to remain in map, got %#v",
			updatedMap,
		)
	}

	if _, statErr := os.Stat(removedDistPathA); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected removed dist file a to be deleted, stat error: %v",
			statErr,
		)
	}
	if _, statErr := os.Stat(removedDistPathB); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected removed dist file b to be deleted, stat error: %v",
			statErr,
		)
	}
}

func TestProcessPrivateFilesOnlyForChangedPaths_DirectoryRenameWithoutChildFileEvents(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	defer builder.Close()

	privateDir := cfg.Core.StaticAssetDirs.Private
	oldDirectoryPath := filepath.Join(privateDir, "templates", "old")
	newDirectoryPath := filepath.Join(privateDir, "templates", "new")
	oldFilePathA := filepath.Join(oldDirectoryPath, "a.html")
	oldFilePathB := filepath.Join(oldDirectoryPath, "nested", "b.html")

	if err := os.MkdirAll(filepath.Dir(oldFilePathB), 0o755); err != nil {
		t.Fatalf("failed creating old subtree parent dir: %v", err)
	}
	if err := os.WriteFile(oldFilePathA, []byte("<h1>a</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing old file a: %v", err)
	}
	if err := os.WriteFile(oldFilePathB, []byte("<h1>b</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing old file b: %v", err)
	}

	if err := builder.ProcessPrivateFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPrivateFilesOnly returned error: %v", err)
	}

	initialMap, err := loadFileMapFromPathForStaticProcessingTests(
		cfg,
		cfg.Dist.PrivateFileMapGob(),
	)
	if err != nil {
		t.Fatalf(
			"loadFileMapFromPath after initial run returned error: %v",
			err,
		)
	}
	oldEntryA := initialMap["templates/old/a.html"]
	oldEntryB := initialMap["templates/old/nested/b.html"]
	oldDistPathA := filepath.Join(cfg.Dist.StaticPrivate(), oldEntryA.DistName)
	oldDistPathB := filepath.Join(cfg.Dist.StaticPrivate(), oldEntryB.DistName)

	if err := os.Rename(oldDirectoryPath, newDirectoryPath); err != nil {
		t.Fatalf("failed renaming private subtree directory: %v", err)
	}

	// Some platforms can emit only directory-level rename/create events for subtree moves.
	// The changed-path processor must still converge to the new subtree mapping.
	if err := builder.ProcessPrivateFilesOnlyForChangedPaths(
		[]string{oldDirectoryPath, newDirectoryPath},
	); err != nil {
		t.Fatalf(
			"processPrivateFilesOnlyForChangedPaths for subtree rename returned error: %v",
			err,
		)
	}

	updatedMap, err := loadFileMapFromPathForStaticProcessingTests(
		cfg,
		cfg.Dist.PrivateFileMapGob(),
	)
	if err != nil {
		t.Fatalf(
			"loadFileMapFromPath after subtree rename returned error: %v",
			err,
		)
	}
	if _, exists := updatedMap["templates/old/a.html"]; exists {
		t.Fatalf(
			"expected templates/old/a.html to be removed from map, got %#v",
			updatedMap["templates/old/a.html"],
		)
	}
	if _, exists := updatedMap["templates/old/nested/b.html"]; exists {
		t.Fatalf(
			"expected templates/old/nested/b.html to be removed from map, got %#v",
			updatedMap["templates/old/nested/b.html"],
		)
	}
	if _, exists := updatedMap["templates/new/a.html"]; !exists {
		t.Fatalf(
			"expected templates/new/a.html to exist in map, got %#v",
			updatedMap,
		)
	}
	if _, exists := updatedMap["templates/new/nested/b.html"]; !exists {
		t.Fatalf(
			"expected templates/new/nested/b.html to exist in map, got %#v",
			updatedMap,
		)
	}

	if _, statErr := os.Stat(oldDistPathA); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected old dist file a to be deleted, stat error: %v",
			statErr,
		)
	}
	if _, statErr := os.Stat(oldDistPathB); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected old dist file b to be deleted, stat error: %v",
			statErr,
		)
	}
}

func TestProcessPublicFilesOnly_RecopiesUnchangedFileWhenDistOutputIsMissing(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
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
		t.Fatalf(
			"failed removing dist output to simulate partial cleanup: %v",
			err,
		)
	}

	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("second ProcessPublicFilesOnly returned error: %v", err)
	}

	if _, statErr := os.Stat(distPath); statErr != nil {
		t.Fatalf(
			"expected missing dist output to be recopied, stat error: %v",
			statErr,
		)
	}
}

func TestProcessPublicFilesOnly_UnchangedInputsDoNotRewriteMapArtifacts(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForStaticProcessingTestsAtRoot(root)
	builder := builder.NewBuilder(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
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

	jsPath := filepath.Join(
		cfg.Dist.StaticPublic(),
		strings.TrimSpace(string(refBefore)),
	)
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

func newDiscardLoggerForStaticProcessingTests() *slog.Logger {
	return wavetest.NewDiscardLogger()
}

func newParsedConfigForStaticProcessingTestsAtRoot(
	root string,
) *waveconfig.ParsedConfig {
	return wavetest.NewParsedConfigAtRoot(root)
}

func loadFileMapFromPathForStaticProcessingTests(
	cfg *waveconfig.ParsedConfig,
	path string,
) (wavefilemap.FileMap, error) {
	staticProcessor := static.NewProcessor(
		cfg,
		newDiscardLoggerForStaticProcessingTests(),
	)
	return staticProcessor.LoadFileMapFromPath(path)
}

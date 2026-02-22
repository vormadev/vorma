package watch_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/internal/watch"
)

func TestNewWatcher_FrameworkIgnoredPatternsAreRelativeToWatchRoot(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.FrameworkIgnoredPatterns = []string{
		"generated/**",
		"generated_file.txt",
	}

	watcher, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	ignoredDir := filepath.Join(root, "generated")
	if !watcher.IsIgnoredDir(ignoredDir) {
		t.Fatalf("expected %q to be ignored as framework dir pattern", ignoredDir)
	}

	ignoredFile := filepath.Join(root, "generated_file.txt")
	if !watcher.IsIgnoredFile(ignoredFile) {
		t.Fatalf("expected %q to be ignored as framework file pattern", ignoredFile)
	}
}

func TestNewWatcher_FrameworkIgnoredLiteralDirectoryPatternIgnoresDirectoryTree(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.FrameworkIgnoredPatterns = []string{"generated"}

	watcher, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	ignoredDirectoryPath := filepath.Join(root, "generated")
	if !watcher.IsIgnoredDir(ignoredDirectoryPath) {
		t.Fatalf("expected %q to be ignored as framework literal directory pattern", ignoredDirectoryPath)
	}

	ignoredNestedDirectoryPath := filepath.Join(root, "generated", "nested")
	if !watcher.IsIgnoredDir(ignoredNestedDirectoryPath) {
		t.Fatalf(
			"expected nested directory %q to be ignored as framework literal directory pattern",
			ignoredNestedDirectoryPath,
		)
	}

	ignoredFilePath := filepath.Join(root, "generated", "asset.txt")
	if !watcher.IsIgnoredFile(ignoredFilePath) {
		t.Fatalf("expected file %q to be ignored as framework literal directory pattern", ignoredFilePath)
	}
}

func TestFindWatchedFile_MergesFrameworkAndUserMatches(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.FrameworkWatchPatterns = []wave.WatchedFile{
		{
			Pattern:           "**/*.txt",
			RecompileGoBinary: true,
			OnChangeHooks: []wave.OnChangeHook{
				{Cmd: "framework-pre", Timing: wave.OnChangeStrategyPre},
			},
		},
	}
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:    "**/*.txt",
			RestartApp: true,
			OnChangeHooks: []wave.OnChangeHook{
				{Cmd: "user-post", Timing: wave.OnChangeStrategyPost},
			},
		},
	}

	watcher, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	watchedFile := watcher.FindWatchedFile(filepath.Join(root, "notes.txt"))
	if watchedFile == nil {
		t.Fatal("expected watched file match, got nil")
	}

	if !watchedFile.RecompileGoBinary {
		t.Fatal("expected merged watched file to request Go recompilation")
	}
	if !watchedFile.RestartApp {
		t.Fatal("expected merged watched file to request app restart")
	}

	if len(watchedFile.OnChangeHooks) != 2 {
		t.Fatalf("expected 2 merged hooks, got %d", len(watchedFile.OnChangeHooks))
	}
	if watchedFile.OnChangeHooks[0].Cmd != "framework-pre" {
		t.Fatalf("expected first hook to be framework hook, got %q", watchedFile.OnChangeHooks[0].Cmd)
	}
	if watchedFile.OnChangeHooks[1].Cmd != "user-post" {
		t.Fatalf("expected second hook to be user hook, got %q", watchedFile.OnChangeHooks[1].Cmd)
	}
}

func TestNewWatcher_DoesNotMutateConfigWatchPatternsOrHookExcludes(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.FrameworkWatchPatterns = []wave.WatchedFile{
		{
			Pattern: "framework/**/*.txt",
			OnChangeHooks: []wave.OnChangeHook{
				{
					Cmd:     "framework-hook",
					Exclude: []string{"framework/exclude/**"},
				},
			},
		},
	}
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern: "user/**/*.txt",
			OnChangeHooks: []wave.OnChangeHook{
				{
					Cmd:     "user-hook",
					Exclude: []string{"user/exclude/**"},
				},
			},
		},
	}

	watcher, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	if cfg.FrameworkWatchPatterns[0].Pattern != "framework/**/*.txt" {
		t.Fatalf("expected framework pattern to remain relative, got %q", cfg.FrameworkWatchPatterns[0].Pattern)
	}
	if cfg.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0] != "framework/exclude/**" {
		t.Fatalf(
			"expected framework exclude to remain relative, got %q",
			cfg.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0],
		)
	}
	if cfg.Watch.Include[0].Pattern != "user/**/*.txt" {
		t.Fatalf("expected user pattern to remain relative, got %q", cfg.Watch.Include[0].Pattern)
	}
	if cfg.Watch.Include[0].OnChangeHooks[0].Exclude[0] != "user/exclude/**" {
		t.Fatalf(
			"expected user exclude to remain relative, got %q",
			cfg.Watch.Include[0].OnChangeHooks[0].Exclude[0],
		)
	}

	frameworkMatch := watcher.FindWatchedFile(
		filepath.Join(root, "framework", "file.txt"),
	)
	if frameworkMatch == nil {
		t.Fatal("expected framework pattern to match after watcher normalization")
	}

	userMatch := watcher.FindWatchedFile(filepath.Join(root, "user", "file.txt"))
	if userMatch == nil {
		t.Fatal("expected user pattern to match after watcher normalization")
	}
}

func TestWatcherStaticClassificationUsesDirectoryBoundaries(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)

	watcher, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	publicFile := filepath.Join(root, "static", "public", "app.js")
	if !watcher.IsPublicStaticFile(publicFile) {
		t.Fatalf("expected %q to be classified as public static file", publicFile)
	}

	publicPrefixCollision := filepath.Join(root, "static", "publicity", "app.js")
	if watcher.IsPublicStaticFile(publicPrefixCollision) {
		t.Fatalf("did not expect %q to be classified as public static file", publicPrefixCollision)
	}

	privateFile := filepath.Join(root, "static", "private", "template.html")
	if !watcher.IsPrivateStaticFile(privateFile) {
		t.Fatalf("expected %q to be classified as private static file", privateFile)
	}
}

func TestWatcherStaticClassificationRecognizesSymlinkAliasPaths(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)

	publicStaticDirectoryPath := filepath.Join(root, "static", "public")
	privateStaticDirectoryPath := filepath.Join(root, "static", "private")
	if makeDirectoryError := os.MkdirAll(publicStaticDirectoryPath, 0o755); makeDirectoryError != nil {
		t.Fatalf("failed creating public static directory: %v", makeDirectoryError)
	}
	if makeDirectoryError := os.MkdirAll(privateStaticDirectoryPath, 0o755); makeDirectoryError != nil {
		t.Fatalf("failed creating private static directory: %v", makeDirectoryError)
	}
	if writeError := os.WriteFile(
		filepath.Join(publicStaticDirectoryPath, "app.js"),
		[]byte("console.log('public');"),
		0o644,
	); writeError != nil {
		t.Fatalf("failed writing public static file: %v", writeError)
	}
	if writeError := os.WriteFile(
		filepath.Join(privateStaticDirectoryPath, "template.html"),
		[]byte("<html></html>"),
		0o644,
	); writeError != nil {
		t.Fatalf("failed writing private static file: %v", writeError)
	}

	watcher, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForWatchTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("newWatcher returned error: %v", watcherCreateError)
	}
	defer watcher.Close()

	aliasStaticDirectoryPath := filepath.Join(root, "static-alias")
	if symlinkError := os.Symlink(
		filepath.Join(root, "static"),
		aliasStaticDirectoryPath,
	); symlinkError != nil {
		t.Skipf("symlink creation is unavailable on this platform: %v", symlinkError)
	}

	aliasPublicStaticFilePath := filepath.Join(
		aliasStaticDirectoryPath,
		"public",
		"app.js",
	)
	if !watcher.IsPublicStaticFile(aliasPublicStaticFilePath) {
		t.Fatalf(
			"expected symlink alias path %q to classify as public static",
			aliasPublicStaticFilePath,
		)
	}

	aliasPrivateStaticFilePath := filepath.Join(
		aliasStaticDirectoryPath,
		"private",
		"template.html",
	)
	if !watcher.IsPrivateStaticFile(aliasPrivateStaticFilePath) {
		t.Fatalf(
			"expected symlink alias path %q to classify as private static",
			aliasPrivateStaticFilePath,
		)
	}
}

func TestNewWatcher_RejectsInvalidFrameworkWatchPattern(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.FrameworkWatchPatterns = []wave.WatchedFile{
		{
			Pattern: "[",
		},
	}

	_, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err == nil {
		t.Fatal("expected newWatcher to fail for invalid framework watch pattern")
	}
	if !strings.Contains(err.Error(), "FrameworkWatchPatterns[0].Pattern") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewWatcher_RejectsEmptyFrameworkWatchPattern(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.FrameworkWatchPatterns = []wave.WatchedFile{
		{
			Pattern: "   ",
		},
	}

	_, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err == nil {
		t.Fatal("expected newWatcher to fail for empty framework watch pattern")
	}
	if !strings.Contains(err.Error(), "FrameworkWatchPatterns[0].Pattern is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewWatcher_RejectsInvalidFrameworkHookExcludePattern(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.FrameworkWatchPatterns = []wave.WatchedFile{
		{
			Pattern: "**/*.txt",
			OnChangeHooks: []wave.OnChangeHook{
				{
					Cmd:     "echo hello",
					Exclude: []string{"["},
				},
			},
		},
	}

	_, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err == nil {
		t.Fatal("expected newWatcher to fail for invalid framework hook exclude pattern")
	}
	if !strings.Contains(err.Error(), "FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0]") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewWatcher_RejectsInvalidIgnoredPattern(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.FrameworkIgnoredPatterns = []string{"["}

	_, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err == nil {
		t.Fatal("expected newWatcher to fail for invalid framework ignored pattern")
	}
	if !strings.Contains(err.Error(), "FrameworkIgnoredPatterns[0]") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewWatcher_RejectsIgnoredPatternWithSurroundingWhitespace(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.FrameworkIgnoredPatterns = []string{" generated/** "}

	_, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err == nil {
		t.Fatal("expected newWatcher to fail for ignored pattern with surrounding whitespace")
	}
	if !strings.Contains(err.Error(), "FrameworkIgnoredPatterns[0] must not include surrounding whitespace") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewWatcher_RejectsInvalidWatchExcludePattern(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.Watch.Exclude.Files = []string{"["}

	_, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err == nil {
		t.Fatal("expected newWatcher to fail for invalid watch exclude pattern")
	}
	if !strings.Contains(err.Error(), "Watch.Exclude.Files[0]") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewWatcher_RelativePatternsMatchWhenWatchRootContainsGlobMetacharacters(t *testing.T) {
	root := t.TempDir()
	watchRoot := filepath.Join(root, "[watch-root]")
	if err := os.MkdirAll(watchRoot, 0o755); err != nil {
		t.Fatalf("failed creating watch root: %v", err)
	}

	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.Watch.WatchRoot = watchRoot
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern: "**/*.txt",
		},
	}
	cfg.FrameworkIgnoredPatterns = []string{"generated/**"}

	watcher, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	changedFilePath := filepath.Join(watchRoot, "notes.txt")
	matchedWatchedFile := watcher.FindWatchedFile(changedFilePath)
	if matchedWatchedFile == nil {
		t.Fatalf(
			"expected relative watch pattern to match changed file under watch root with glob metacharacters: %q",
			changedFilePath,
		)
	}

	ignoredGeneratedDirectoryPath := filepath.Join(watchRoot, "generated")
	if !watcher.IsIgnoredDir(ignoredGeneratedDirectoryPath) {
		t.Fatalf(
			"expected framework ignored dir pattern to match under watch root with glob metacharacters: %q",
			ignoredGeneratedDirectoryPath,
		)
	}
}

func TestWatcher_RelativeWatchRootMatchesAbsoluteEventPaths(t *testing.T) {
	root := t.TempDir()
	if makeDirectoryError := os.MkdirAll(
		filepath.Join(root, "static", "public"),
		0o755,
	); makeDirectoryError != nil {
		t.Fatalf("failed creating public static directory: %v", makeDirectoryError)
	}
	if makeDirectoryError := os.MkdirAll(
		filepath.Join(root, "static", "private"),
		0o755,
	); makeDirectoryError != nil {
		t.Fatalf("failed creating private static directory: %v", makeDirectoryError)
	}

	workingDirectoryBeforeTest, getWorkingDirectoryError := os.Getwd()
	if getWorkingDirectoryError != nil {
		t.Fatalf("failed to read working directory: %v", getWorkingDirectoryError)
	}
	if changeDirectoryError := os.Chdir(root); changeDirectoryError != nil {
		t.Fatalf("failed changing working directory: %v", changeDirectoryError)
	}
	t.Cleanup(func() {
		_ = os.Chdir(workingDirectoryBeforeTest)
	})

	cfg := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      "dist",
			StaticAssetDirs: staticAssetDirsForTests{
				Public:  filepath.Join("static", "public"),
				Private: filepath.Join("static", "private"),
			},
		},
		Watch: &wave.WatchConfig{
			WatchRoot: ".",
			Include: []wave.WatchedFile{
				{Pattern: "**/*.txt"},
			},
		},
	}
	cfg.Dist.Root = cfg.Core.DistDir

	watcher, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForWatchTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcher.Close()

	absoluteChangedFilePath, absoluteChangedPathError := filepath.Abs(
		filepath.Join("content", "notes.txt"),
	)
	if absoluteChangedPathError != nil {
		t.Fatalf("failed resolving absolute changed file path: %v", absoluteChangedPathError)
	}
	matchedWatchedFile := watcher.FindWatchedFile(absoluteChangedFilePath)
	if matchedWatchedFile == nil {
		t.Fatalf(
			"expected relative watch pattern to match absolute changed path %q",
			absoluteChangedFilePath,
		)
	}

	absolutePublicStaticPath, absolutePublicStaticPathError := filepath.Abs(
		filepath.Join("static", "public", "app.js"),
	)
	if absolutePublicStaticPathError != nil {
		t.Fatalf("failed resolving absolute public static path: %v", absolutePublicStaticPathError)
	}
	if !watcher.IsPublicStaticFile(absolutePublicStaticPath) {
		t.Fatalf(
			"expected absolute path %q to classify as public static",
			absolutePublicStaticPath,
		)
	}

	absoluteDistOutputPath, absoluteDistOutputPathError := filepath.Abs(
		filepath.Join("dist", "static", "assets", "public", "generated.js"),
	)
	if absoluteDistOutputPathError != nil {
		t.Fatalf("failed resolving absolute dist output path: %v", absoluteDistOutputPathError)
	}
	if !watcher.IsIgnoredFile(absoluteDistOutputPath) {
		t.Fatalf(
			"expected dist output path %q to be ignored",
			absoluteDistOutputPath,
		)
	}
}

func TestWatcher_AbsoluteGlobPatternsMatchAbsoluteEventPaths(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern: filepath.ToSlash(
				filepath.Join(root, "content", "**", "*.txt"),
			),
		},
	}
	cfg.FrameworkIgnoredPatterns = []string{
		filepath.ToSlash(filepath.Join(root, "generated", "**")),
	}

	watcher, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForWatchTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcher.Close()

	absoluteChangedFilePath, absoluteChangedPathError := filepath.Abs(
		filepath.Join(root, "content", "docs", "notes.txt"),
	)
	if absoluteChangedPathError != nil {
		t.Fatalf("failed resolving absolute changed file path: %v", absoluteChangedPathError)
	}
	matchedWatchedFile := watcher.FindWatchedFile(absoluteChangedFilePath)
	if matchedWatchedFile == nil {
		t.Fatalf(
			"expected absolute include glob to match absolute changed path %q",
			absoluteChangedFilePath,
		)
	}

	absoluteIgnoredFilePath, absoluteIgnoredPathError := filepath.Abs(
		filepath.Join(root, "generated", "assets", "file.txt"),
	)
	if absoluteIgnoredPathError != nil {
		t.Fatalf("failed resolving absolute ignored file path: %v", absoluteIgnoredPathError)
	}
	if !watcher.IsIgnoredFile(absoluteIgnoredFilePath) {
		t.Fatalf(
			"expected absolute framework ignored glob to match %q",
			absoluteIgnoredFilePath,
		)
	}
}

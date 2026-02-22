package eventpipeline_test

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/tooling/internal/watch"
)

func TestClassifyEventWithWatcherAndBuilder_GoFileCanBeTreatedAsNonGo(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:      "**/*.go",
			TreatAsNonGo: true,
			RestartApp:   true,
		},
	}

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	watcherEvent := fsnotify.Event{
		Name: filepath.Join(root, "pkg", "handler.go"),
		Op:   fsnotify.Write,
	}

	classifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		watcherEvent,
		watcherForTest,
		builderForTest,
	)
	if classifiedEvent.FileType != eventpipeline.FileTypeOther {
		t.Fatalf(
			"expected eventpipeline.FileTypeOther, got %v",
			classifiedEvent.FileType,
		)
	}
	if classifiedEvent.WatchedFile == nil {
		t.Fatal("expected matched watched file, got nil")
	}
	if classifiedEvent.Ignored {
		t.Fatal("expected event to be processed, got ignored=true")
	}
}

func TestClassifyEventWithWatcherAndBuilder_UnmatchedOtherFilesAreIgnored(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	watcherEvent := fsnotify.Event{
		Name: filepath.Join(root, "README.md"),
		Op:   fsnotify.Write,
	}

	classifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		watcherEvent,
		watcherForTest,
		builderForTest,
	)
	if classifiedEvent.FileType != eventpipeline.FileTypeOther {
		t.Fatalf(
			"expected eventpipeline.FileTypeOther, got %v",
			classifiedEvent.FileType,
		)
	}
	if !classifiedEvent.Ignored {
		t.Fatal("expected unmatched non-special file to be ignored")
	}
}

func TestIsConfigFileMatchesNormalizedPath(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "backend", "wave.config.json")

	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)
	cfg.Core.ConfigLocation = configPath
	equivalentPath := filepath.Join(root, "backend", ".", "wave.config.json")
	if !isConfigFileForEventPipelineTests(cfg, equivalentPath) {
		t.Fatalf(
			"expected isConfigFileForEventPipelineTests(%q) to match config path %q",
			equivalentPath,
			configPath,
		)
	}

	otherPath := filepath.Join(root, "backend", "different.config.json")
	if isConfigFileForEventPipelineTests(cfg, otherPath) {
		t.Fatalf("expected isConfigFileForEventPipelineTests(%q) to be false", otherPath)
	}

	cfg.Core.ConfigLocation = ""
	if isConfigFileForEventPipelineTests(cfg, configPath) {
		t.Fatal(
			"expected isConfigFileForEventPipelineTests to be false when config file path is empty",
		)
	}
}

func TestNeedsHardReload(t *testing.T) {
	if eventpipeline.NeedsHardReload(nil) {
		t.Fatal("eventpipeline.NeedsHardReload(nil) should be false")
	}
	if !eventpipeline.NeedsHardReload(
		&wave.WatchedFile{RecompileGoBinary: true},
	) {
		t.Fatal("expected RecompileGoBinary=true to require hard reload")
	}
	if !eventpipeline.NeedsHardReload(&wave.WatchedFile{RestartApp: true}) {
		t.Fatal("expected RestartApp=true to require hard reload")
	}
	if eventpipeline.NeedsHardReload(&wave.WatchedFile{}) {
		t.Fatal("expected empty watched file to not require hard reload")
	}
}

func TestClassifyEventWithWatcherAndBuilder_EmptyPathIsIgnored(t *testing.T) {
	cfg := newParsedConfigForEventClassificationTestsAtRoot(t.TempDir())

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	classifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{Name: "", Op: fsnotify.Write},
		watcherForTest,
		builderForTest,
	)

	if !classifiedEvent.Ignored {
		t.Fatal("expected empty-path event to be ignored")
	}
}

func TestClassifyEventWithWatcherAndBuilder_PublicAndPrivateStaticFiles(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	publicFilePath := filepath.Join(
		cfg.Core.StaticAssetDirs.Public,
		"img",
		"logo.png",
	)
	privateFilePath := filepath.Join(
		cfg.Core.StaticAssetDirs.Private,
		"tpl",
		"home.html",
	)

	publicClassifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{Name: publicFilePath, Op: fsnotify.Write},
		watcherForTest,
		builderForTest,
	)
	if publicClassifiedEvent.FileType != eventpipeline.FileTypePublicStatic {
		t.Fatalf(
			"expected public static file type, got %v",
			publicClassifiedEvent.FileType,
		)
	}

	privateClassifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{Name: privateFilePath, Op: fsnotify.Write},
		watcherForTest,
		builderForTest,
	)
	if privateClassifiedEvent.FileType != eventpipeline.FileTypePrivateStatic {
		t.Fatalf(
			"expected private static file type, got %v",
			privateClassifiedEvent.FileType,
		)
	}
}

func TestClassifyEventWithWatcherAndBuilder_CriticalAndNormalCSSFiles(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	criticalCSSFilePath := filepath.Join(root, "styles", "critical.css")
	normalCSSFilePath := filepath.Join(root, "styles", "normal.css")
	sharedCSSFilePath := filepath.Join(root, "styles", "shared.css")
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical:    criticalCSSFilePath,
		NonCritical: normalCSSFilePath,
	}
	if makeDirectoryError := os.MkdirAll(filepath.Dir(criticalCSSFilePath), 0o755); makeDirectoryError != nil {
		t.Fatalf("create css directory: %v", makeDirectoryError)
	}
	if writeCriticalError := os.WriteFile(
		criticalCSSFilePath,
		[]byte(`@import "./shared.css"; .critical { color: red; }`),
		0o644,
	); writeCriticalError != nil {
		t.Fatalf("write critical css: %v", writeCriticalError)
	}
	if writeNormalError := os.WriteFile(
		normalCSSFilePath,
		[]byte(`@import "./shared.css"; .normal { color: blue; }`),
		0o644,
	); writeNormalError != nil {
		t.Fatalf("write normal css: %v", writeNormalError)
	}
	if writeSharedError := os.WriteFile(
		sharedCSSFilePath,
		[]byte(`.shared { color: green; }`),
		0o644,
	); writeSharedError != nil {
		t.Fatalf("write shared css: %v", writeSharedError)
	}
	if cssBuildError := builderForTest.BuildCSS(
		builder.CSSBuildOptions{
			BuildCriticalCSS: true,
			BuildNormalCSS:   true,
		},
	); cssBuildError != nil {
		t.Fatalf("BuildCSS returned error: %v", cssBuildError)
	}

	criticalClassifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{Name: criticalCSSFilePath, Op: fsnotify.Write},
		watcherForTest,
		builderForTest,
	)
	if criticalClassifiedEvent.FileType != eventpipeline.FileTypeCriticalCSS {
		t.Fatalf(
			"expected critical css file type, got %v",
			criticalClassifiedEvent.FileType,
		)
	}

	normalClassifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{Name: normalCSSFilePath, Op: fsnotify.Write},
		watcherForTest,
		builderForTest,
	)
	if normalClassifiedEvent.FileType != eventpipeline.FileTypeNormalCSS {
		t.Fatalf(
			"expected normal css file type, got %v",
			normalClassifiedEvent.FileType,
		)
	}

	sharedClassifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{Name: sharedCSSFilePath, Op: fsnotify.Write},
		watcherForTest,
		builderForTest,
	)
	if sharedClassifiedEvent.FileType != eventpipeline.FileTypeOther {
		t.Fatalf(
			"expected shared css file type %v, got %v",
			eventpipeline.FileTypeOther,
			sharedClassifiedEvent.FileType,
		)
	}
}

func TestClassifyEventWithWatcherAndBuilder_RespectsIgnoredFiles(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForEventClassificationTestsAtRoot(root)
	cfg.Watch.Exclude.Files = []string{"ignored.tmp"}

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventClassificationTests(),
	)
	defer builderForTest.Close()

	classifiedEvent := classifyEventWithWatcherAndBuilderForEventPipelineTests(
		fsnotify.Event{
			Name: filepath.Join(root, "ignored.tmp"),
			Op:   fsnotify.Write,
		},
		watcherForTest,
		builderForTest,
	)

	if !classifiedEvent.Ignored {
		t.Fatal("expected ignored file to classify with ignored=true")
	}
}

func newDiscardLoggerForEventClassificationTests() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newParsedConfigForEventClassificationTestsAtRoot(
	root string,
) *wave.ParsedConfig {
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

package tooling

import (
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"github.com/vormadev/vorma/wave/tooling/watch"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
)

func TestClassifyEventWithWatcherAndBuilder_GoFileCanBeTreatedAsNonGo(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:      "**/*.go",
			TreatAsNonGo: true,
			RestartApp:   true,
		},
	}

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &devserver.Server{Cfg: cfg, Log: newDiscardLogger()}
	event := fsnotify.Event{
		Name: filepath.Join(root, "pkg", "handler.go"),
		Op:   fsnotify.Write,
	}

	classified := s.ClassifyEventWithWatcherAndBuilder(event, watcher, builder)
	if classified.FileType != devserver.FileTypeOther {
		t.Fatalf("expected devserver.FileTypeOther, got %v", classified.FileType)
	}
	if classified.WatchedFile == nil {
		t.Fatal("expected matched watched file, got nil")
	}
	if classified.Ignored {
		t.Fatal("expected event to be processed, got ignored=true")
	}
}

func TestClassifyEventWithWatcherAndBuilder_UnmatchedOtherFilesAreIgnored(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &devserver.Server{Cfg: cfg, Log: newDiscardLogger()}
	event := fsnotify.Event{
		Name: filepath.Join(root, "README.md"),
		Op:   fsnotify.Write,
	}

	classified := s.ClassifyEventWithWatcherAndBuilder(event, watcher, builder)
	if classified.FileType != devserver.FileTypeOther {
		t.Fatalf("expected devserver.FileTypeOther, got %v", classified.FileType)
	}
	if !classified.Ignored {
		t.Fatal("expected unmatched non-special file to be ignored")
	}
}

func TestIsConfigFileMatchesNormalizedPath(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "backend", "wave.config.json")

	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ConfigLocation = configPath
	s := &devserver.Server{Cfg: cfg}

	equivalentPath := filepath.Join(root, "backend", ".", "wave.config.json")
	if !s.IsConfigFile(equivalentPath) {
		t.Fatalf("expected isConfigFile(%q) to match config path %q", equivalentPath, configPath)
	}

	otherPath := filepath.Join(root, "backend", "different.config.json")
	if s.IsConfigFile(otherPath) {
		t.Fatalf("expected isConfigFile(%q) to be false", otherPath)
	}

	cfg.Core.ConfigLocation = ""
	if s.IsConfigFile(configPath) {
		t.Fatal("expected isConfigFile to be false when config file path is empty")
	}
}

func TestNeedsHardReload(t *testing.T) {
	if devserver.NeedsHardReload(nil) {
		t.Fatal("devserver.NeedsHardReload(nil) should be false")
	}
	if !devserver.NeedsHardReload(&wave.WatchedFile{RecompileGoBinary: true}) {
		t.Fatal("expected RecompileGoBinary=true to require hard reload")
	}
	if !devserver.NeedsHardReload(&wave.WatchedFile{RestartApp: true}) {
		t.Fatal("expected RestartApp=true to require hard reload")
	}
	if devserver.NeedsHardReload(&wave.WatchedFile{}) {
		t.Fatal("expected empty watched file to not require hard reload")
	}
}

func TestClassifyEventWithWatcherAndBuilder_EmptyPathIsIgnored(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &devserver.Server{Cfg: cfg, Log: newDiscardLogger()}
	classified := s.ClassifyEventWithWatcherAndBuilder(fsnotify.Event{Name: "", Op: fsnotify.Write}, watcher, builder)

	if !classified.Ignored {
		t.Fatal("expected empty-path event to be ignored")
	}
}

func TestClassifyEventWithWatcherAndBuilder_PublicAndPrivateStaticFiles(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &devserver.Server{Cfg: cfg, Log: newDiscardLogger()}

	publicFile := filepath.Join(cfg.Core.StaticAssetDirs.Public, "img", "logo.png")
	privateFile := filepath.Join(cfg.Core.StaticAssetDirs.Private, "tpl", "home.html")

	publicClassified := s.ClassifyEventWithWatcherAndBuilder(
		fsnotify.Event{Name: publicFile, Op: fsnotify.Write},
		watcher,
		builder,
	)
	if publicClassified.FileType != devserver.FileTypePublicStatic {
		t.Fatalf("expected public static file type, got %v", publicClassified.FileType)
	}

	privateClassified := s.ClassifyEventWithWatcherAndBuilder(
		fsnotify.Event{Name: privateFile, Op: fsnotify.Write},
		watcher,
		builder,
	)
	if privateClassified.FileType != devserver.FileTypePrivateStatic {
		t.Fatalf("expected private static file type, got %v", privateClassified.FileType)
	}
}

func TestClassifyEventWithWatcherAndBuilder_CriticalAndNormalCSSFiles(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	criticalCSSFile := filepath.Join(root, "styles", "critical.css")
	normalCSSFile := filepath.Join(root, "styles", "normal.css")
	sharedCSSFile := filepath.Join(root, "styles", "waveshared.css")
	criticalAbsPath, criticalErr := filepath.Abs(criticalCSSFile)
	if criticalErr != nil {
		t.Fatalf("filepath.Abs critical css failed: %v", criticalErr)
	}
	normalAbsPath, normalErr := filepath.Abs(normalCSSFile)
	if normalErr != nil {
		t.Fatalf("filepath.Abs normal css failed: %v", normalErr)
	}
	sharedAbsPath, sharedErr := filepath.Abs(sharedCSSFile)
	if sharedErr != nil {
		t.Fatalf("filepath.Abs shared css failed: %v", sharedErr)
	}

	builder.SetTrackedCriticalCSSImportPaths([]string{
		criticalAbsPath,
		sharedAbsPath,
	})
	builder.SetTrackedNormalCSSImportPaths([]string{
		normalAbsPath,
		sharedAbsPath,
	})

	s := &devserver.Server{Cfg: cfg, Log: newDiscardLogger()}

	criticalClassified := s.ClassifyEventWithWatcherAndBuilder(
		fsnotify.Event{Name: criticalCSSFile, Op: fsnotify.Write},
		watcher,
		builder,
	)
	if criticalClassified.FileType != devserver.FileTypeCriticalCSS {
		t.Fatalf("expected critical css file type, got %v", criticalClassified.FileType)
	}

	normalClassified := s.ClassifyEventWithWatcherAndBuilder(
		fsnotify.Event{Name: normalCSSFile, Op: fsnotify.Write},
		watcher,
		builder,
	)
	if normalClassified.FileType != devserver.FileTypeNormalCSS {
		t.Fatalf("expected normal css file type, got %v", normalClassified.FileType)
	}

	sharedClassified := s.ClassifyEventWithWatcherAndBuilder(
		fsnotify.Event{Name: sharedCSSFile, Op: fsnotify.Write},
		watcher,
		builder,
	)
	if sharedClassified.FileType != devserver.FileTypeCriticalAndNormalCSS {
		t.Fatalf(
			"expected shared css file type %v, got %v",
			devserver.FileTypeCriticalAndNormalCSS,
			sharedClassified.FileType,
		)
	}
}

func TestClassifyEventWithWatcherAndBuilder_RespectsIgnoredFiles(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Watch.Exclude.Files = []string{"ignored.tmp"}

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &devserver.Server{Cfg: cfg, Log: newDiscardLogger()}
	classified := s.ClassifyEventWithWatcherAndBuilder(
		fsnotify.Event{Name: filepath.Join(root, "ignored.tmp"), Op: fsnotify.Write},
		watcher,
		builder,
	)

	if !classified.Ignored {
		t.Fatal("expected ignored file to classify with ignored=true")
	}
}

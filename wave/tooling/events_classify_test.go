package tooling

import (
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

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{cfg: cfg, log: newDiscardLogger()}
	event := fsnotify.Event{
		Name: filepath.Join(root, "pkg", "handler.go"),
		Op:   fsnotify.Write,
	}

	classified := s.classifyEventWithWatcherAndBuilder(event, watcher, builder)
	if classified.fileType != fileTypeOther {
		t.Fatalf("expected fileTypeOther, got %v", classified.fileType)
	}
	if classified.watchedFile == nil {
		t.Fatal("expected matched watched file, got nil")
	}
	if classified.ignored {
		t.Fatal("expected event to be processed, got ignored=true")
	}
}

func TestClassifyEventWithWatcherAndBuilder_UnmatchedOtherFilesAreIgnored(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{cfg: cfg, log: newDiscardLogger()}
	event := fsnotify.Event{
		Name: filepath.Join(root, "README.md"),
		Op:   fsnotify.Write,
	}

	classified := s.classifyEventWithWatcherAndBuilder(event, watcher, builder)
	if classified.fileType != fileTypeOther {
		t.Fatalf("expected fileTypeOther, got %v", classified.fileType)
	}
	if !classified.ignored {
		t.Fatal("expected unmatched non-special file to be ignored")
	}
}

func TestIsConfigFileMatchesNormalizedPath(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "wave.config.json")

	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ConfigLocation = configPath
	s := &server{cfg: cfg}

	equivalentPath := filepath.Join(root, ".", "wave.config.json")
	if !s.isConfigFile(equivalentPath) {
		t.Fatalf("expected isConfigFile(%q) to match config path %q", equivalentPath, configPath)
	}

	otherPath := filepath.Join(root, "different.config.json")
	if s.isConfigFile(otherPath) {
		t.Fatalf("expected isConfigFile(%q) to be false", otherPath)
	}

	cfg.Core.ConfigLocation = ""
	if s.isConfigFile(configPath) {
		t.Fatal("expected isConfigFile to be false when ConfigLocation is empty")
	}
}

func TestNeedsHardReload(t *testing.T) {
	if needsHardReload(nil) {
		t.Fatal("needsHardReload(nil) should be false")
	}
	if !needsHardReload(&wave.WatchedFile{RecompileGoBinary: true}) {
		t.Fatal("expected RecompileGoBinary=true to require hard reload")
	}
	if !needsHardReload(&wave.WatchedFile{RestartApp: true}) {
		t.Fatal("expected RestartApp=true to require hard reload")
	}
	if needsHardReload(&wave.WatchedFile{}) {
		t.Fatal("expected empty watched file to not require hard reload")
	}
}

func TestClassifyEventWithWatcherAndBuilder_EmptyPathIsIgnored(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{cfg: cfg, log: newDiscardLogger()}
	classified := s.classifyEventWithWatcherAndBuilder(fsnotify.Event{Name: "", Op: fsnotify.Write}, watcher, builder)

	if !classified.ignored {
		t.Fatal("expected empty-path event to be ignored")
	}
}

func TestClassifyEventWithWatcherAndBuilder_PublicAndPrivateStaticFiles(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{cfg: cfg, log: newDiscardLogger()}

	publicFile := filepath.Join(cfg.Core.StaticAssetDirs.Public, "img", "logo.png")
	privateFile := filepath.Join(cfg.Core.StaticAssetDirs.Private, "tpl", "home.html")

	publicClassified := s.classifyEventWithWatcherAndBuilder(
		fsnotify.Event{Name: publicFile, Op: fsnotify.Write},
		watcher,
		builder,
	)
	if publicClassified.fileType != fileTypePublicStatic {
		t.Fatalf("expected public static file type, got %v", publicClassified.fileType)
	}

	privateClassified := s.classifyEventWithWatcherAndBuilder(
		fsnotify.Event{Name: privateFile, Op: fsnotify.Write},
		watcher,
		builder,
	)
	if privateClassified.fileType != fileTypePrivateStatic {
		t.Fatalf("expected private static file type, got %v", privateClassified.fileType)
	}
}

func TestClassifyEventWithWatcherAndBuilder_CriticalAndNormalCSSFiles(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	criticalCSSFile := filepath.Join(root, "styles", "critical.css")
	normalCSSFile := filepath.Join(root, "styles", "normal.css")
	criticalAbsPath, criticalErr := filepath.Abs(criticalCSSFile)
	if criticalErr != nil {
		t.Fatalf("filepath.Abs critical css failed: %v", criticalErr)
	}
	normalAbsPath, normalErr := filepath.Abs(normalCSSFile)
	if normalErr != nil {
		t.Fatalf("filepath.Abs normal css failed: %v", normalErr)
	}

	builder.css.mu.Lock()
	builder.css.criticalImports[criticalAbsPath] = struct{}{}
	builder.css.normalImports[normalAbsPath] = struct{}{}
	builder.css.mu.Unlock()

	s := &server{cfg: cfg, log: newDiscardLogger()}

	criticalClassified := s.classifyEventWithWatcherAndBuilder(
		fsnotify.Event{Name: criticalCSSFile, Op: fsnotify.Write},
		watcher,
		builder,
	)
	if criticalClassified.fileType != fileTypeCriticalCSS {
		t.Fatalf("expected critical css file type, got %v", criticalClassified.fileType)
	}

	normalClassified := s.classifyEventWithWatcherAndBuilder(
		fsnotify.Event{Name: normalCSSFile, Op: fsnotify.Write},
		watcher,
		builder,
	)
	if normalClassified.fileType != fileTypeNormalCSS {
		t.Fatalf("expected normal css file type, got %v", normalClassified.fileType)
	}
}

func TestClassifyEventWithWatcherAndBuilder_RespectsIgnoredFiles(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Watch.Exclude.Files = []string{"ignored.tmp"}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{cfg: cfg, log: newDiscardLogger()}
	classified := s.classifyEventWithWatcherAndBuilder(
		fsnotify.Event{Name: filepath.Join(root, "ignored.tmp"), Op: fsnotify.Write},
		watcher,
		builder,
	)

	if !classified.ignored {
		t.Fatal("expected ignored file to classify with ignored=true")
	}
}

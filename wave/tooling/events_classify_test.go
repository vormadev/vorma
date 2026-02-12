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

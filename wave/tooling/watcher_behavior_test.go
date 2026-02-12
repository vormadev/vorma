package tooling

import (
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestNewWatcher_FrameworkIgnoredPatternsAreRelativeToWatchRoot(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.FrameworkIgnoredPatterns = []string{
		"generated/**",
		"generated_file.txt",
	}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
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

func TestFindWatchedFile_MergesFrameworkAndUserMatches(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
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

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
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

func TestWatcherStaticClassificationUsesDirectoryBoundaries(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
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

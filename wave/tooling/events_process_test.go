package tooling

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
)

func TestProcessEvents_ConfigWriteTriggersConfigRestart(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	configFilePath := filepath.Join(root, "backend", "wave.config.json")
	cfg.ResolvedConfigFilePath = configFilePath
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	if err := os.MkdirAll(filepath.Dir(configFilePath), 0755); err != nil {
		t.Fatalf("failed creating config file directory: %v", err)
	}
	if err := os.WriteFile(configFilePath, []byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist"}}`), 0644); err != nil {
		t.Fatalf("failed writing config file: %v", err)
	}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		watcher:   watcher,
		builder:   builder,
		restartCh: make(chan restartRequest, 1),
	}

	s.processEvents([]fsnotify.Event{{
		Name: configFilePath,
		Op:   fsnotify.Write,
	}})

	select {
	case req := <-s.restartCh:
		if !req.isConfigRestart || !req.recompileGo {
			t.Fatalf("expected config restart with Go recompile, got %#v", req)
		}
	default:
		t.Fatal("expected config restart request, got none")
	}
}

func TestProcessEvents_DeduplicatesEventsByMatchedPattern(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         "**/*.txt",
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return nil, nil
					},
				},
			},
		},
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	var callbackCount int32
	cfg.Watch.Include[0].OnChangeHooks[0].Callback = func(*wave.HookContext) (*wave.RefreshAction, error) {
		atomic.AddInt32(&callbackCount, 1)
		return nil, nil
	}

	fileA := filepath.Join(root, "a.txt")
	fileB := filepath.Join(root, "b.txt")
	if err := os.WriteFile(fileA, []byte("a"), 0644); err != nil {
		t.Fatalf("failed writing %s: %v", fileA, err)
	}
	if err := os.WriteFile(fileB, []byte("b"), 0644); err != nil {
		t.Fatalf("failed writing %s: %v", fileB, err)
	}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		watcher:   watcher,
		builder:   builder,
		restartCh: make(chan restartRequest, 1),
	}

	s.processEvents([]fsnotify.Event{
		{Name: fileA, Op: fsnotify.Write},
		{Name: fileB, Op: fsnotify.Write},
	})

	if got := atomic.LoadInt32(&callbackCount); got != 1 {
		t.Fatalf("expected callback to run once after pattern dedupe, got %d", got)
	}
}

func TestProcessEvents_BatchHardReloadSetsAppStoppedForBatchOnHookContext(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	var goHookAppStopped atomic.Bool
	var txtHookAppStopped atomic.Bool

	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         "**/*.go",
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Callback: func(ctx *wave.HookContext) (*wave.RefreshAction, error) {
						goHookAppStopped.Store(ctx.AppStoppedForBatch)
						return nil, nil
					},
				},
			},
		},
		{
			Pattern:         "**/*.txt",
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Callback: func(ctx *wave.HookContext) (*wave.RefreshAction, error) {
						txtHookAppStopped.Store(ctx.AppStoppedForBatch)
						return nil, nil
					},
				},
			},
		},
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	goFile := filepath.Join(root, "main.go")
	txtFile := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(goFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed writing %s: %v", goFile, err)
	}
	if err := os.WriteFile(txtFile, []byte("notes"), 0644); err != nil {
		t.Fatalf("failed writing %s: %v", txtFile, err)
	}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		watcher:   watcher,
		builder:   builder,
		restartCh: make(chan restartRequest, 1),
	}

	s.processEvents([]fsnotify.Event{
		{Name: goFile, Op: fsnotify.Write},
		{Name: txtFile, Op: fsnotify.Write},
	})

	if !goHookAppStopped.Load() {
		t.Fatal("expected go hook to see AppStoppedForBatch=true in hard-reload batch")
	}
	if !txtHookAppStopped.Load() {
		t.Fatal("expected txt hook to see AppStoppedForBatch=true in hard-reload batch")
	}
}

func TestProcessEvents_IgnoresChmodOnNonEmptyFile(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	var callbackCount int32
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         "**/*.txt",
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						atomic.AddInt32(&callbackCount, 1)
						return nil, nil
					},
				},
			},
		},
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	filePath := filepath.Join(root, "chmod.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed writing %s: %v", filePath, err)
	}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		watcher:   watcher,
		builder:   builder,
		restartCh: make(chan restartRequest, 1),
	}

	s.processEvents([]fsnotify.Event{{Name: filePath, Op: fsnotify.Chmod}})

	if got := atomic.LoadInt32(&callbackCount); got != 0 {
		t.Fatalf("expected chmod-only non-empty event to be ignored, callback count=%d", got)
	}
}

func TestProcessEvents_NewDirectoryCreateEventAddsWatchDir(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		watcher:   watcher,
		builder:   builder,
		restartCh: make(chan restartRequest, 1),
	}

	newDirectory := filepath.Join(root, "new-child-dir")
	if err := os.MkdirAll(newDirectory, 0755); err != nil {
		t.Fatalf("failed creating new directory: %v", err)
	}

	s.processEvents([]fsnotify.Event{{
		Name: newDirectory,
		Op:   fsnotify.Create,
	}})

	directoryKey := watcher.norm(newDirectory)
	if _, ok := watcher.watchedDirs.Load(directoryKey); !ok {
		t.Fatalf("expected new directory to be added to watcher dirs: %s", directoryKey)
	}
}

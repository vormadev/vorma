package tooling

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
)

func TestProcessEvents_ConfigMutationsTriggerConfigRestart(t *testing.T) {
	runConfigMutationAndPathShapeMatrix(
		t,
		func(
			t *testing.T,
			configMutationCaseForRun configMutationCase,
			pathShapeCaseForRun configEventPathShapeCase,
		) {
			cfg, _, configFilePath := setupConfigEventTestConfig(t)
			if configMutationCaseForRun.prepareEvent != nil {
				configMutationCaseForRun.prepareEvent(t, configFilePath)
			}

			s := setupProcessEventsServerForToolingTests(t, cfg)

			configEventPath := pathShapeCaseForRun.buildPath(t, configFilePath)
			s.processEvents([]fsnotify.Event{{
				Name: configEventPath,
				Op:   configMutationCaseForRun.op,
			}})

			select {
			case req := <-s.restartCh:
				if !req.isConfigRestart || !req.recompileGo {
					t.Fatalf("expected config restart with Go recompile, got %#v", req)
				}
			default:
				t.Fatalf(
					"expected config restart request for op=%s pathShape=%s",
					configMutationCaseForRun.name,
					pathShapeCaseForRun.name,
				)
			}
		},
	)
}

func TestProcessEvents_ConfigChangeBatchSkipsNonConfigHookProcessing(t *testing.T) {
	runConfigMutationAndPathShapeMatrix(
		t,
		func(
			t *testing.T,
			configMutationCaseForRun configMutationCase,
			pathShapeCaseForRun configEventPathShapeCase,
		) {
			cfg, root, configFilePath := setupConfigEventTestConfig(t)

			var nonConfigHookCallCount int32
			cfg.Watch.Include = []wave.WatchedFile{
				{
					Pattern:         "**/*.txt",
					RunOnChangeOnly: true,
					OnChangeHooks: []wave.OnChangeHook{
						{
							Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
								atomic.AddInt32(&nonConfigHookCallCount, 1)
								return nil, nil
							},
						},
					},
				},
			}

			if err := os.MkdirAll(filepath.Dir(configFilePath), 0o755); err != nil {
				t.Fatalf("failed creating config file directory: %v", err)
			}
			if err := os.WriteFile(configFilePath, []byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist"}}`), 0o644); err != nil {
				t.Fatalf("failed writing config file: %v", err)
			}

			nonConfigFilePath := filepath.Join(root, "notes.txt")
			if err := os.WriteFile(nonConfigFilePath, []byte("notes"), 0o644); err != nil {
				t.Fatalf("failed writing non-config file: %v", err)
			}

			if configMutationCaseForRun.prepareEvent != nil {
				configMutationCaseForRun.prepareEvent(t, configFilePath)
			}

			s := setupProcessEventsServerForToolingTests(t, cfg)

			configEventPath := pathShapeCaseForRun.buildPath(t, configFilePath)
			s.processEvents([]fsnotify.Event{
				{
					Name: configEventPath,
					Op:   configMutationCaseForRun.op,
				},
				{
					Name: nonConfigFilePath,
					Op:   fsnotify.Write,
				},
			})

			select {
			case req := <-s.restartCh:
				if !req.isConfigRestart || !req.recompileGo {
					t.Fatalf("expected config restart with Go recompile, got %#v", req)
				}
			default:
				t.Fatalf(
					"expected config restart request for op=%s pathShape=%s",
					configMutationCaseForRun.name,
					pathShapeCaseForRun.name,
				)
			}

			if atomic.LoadInt32(&nonConfigHookCallCount) != 0 {
				t.Fatalf(
					"expected no non-config hook execution when config changes, got %d calls",
					atomic.LoadInt32(&nonConfigHookCallCount),
				)
			}
		},
	)
}

func TestProcessEvents_CreateForMissingFileStillRunsMatchingHooks(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	var hookCallCount int32
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         "**/*.txt",
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						atomic.AddInt32(&hookCallCount, 1)
						return nil, nil
					},
				},
			},
		},
	}

	s := setupProcessEventsServerForToolingTests(t, cfg)

	missingFilePath := filepath.Join(root, "new-file.txt")
	s.processEvents([]fsnotify.Event{{
		Name: missingFilePath,
		Op:   fsnotify.Create,
	}})

	if atomic.LoadInt32(&hookCallCount) != 1 {
		t.Fatalf("expected matching hook to run exactly once for missing-file create, got %d", atomic.LoadInt32(&hookCallCount))
	}

	select {
	case req := <-s.restartCh:
		t.Fatalf("did not expect restart request for missing-file create, got %#v", req)
	default:
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

	watcher, err := newWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
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

	watcher, err := newWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
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

	watcher, err := newWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
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

	watcher, err := newWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
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

func TestProcessEvents_PublicStaticMixedOpsBatchAppliesCreateDeleteAndRenameChanges(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	watcher, err := newWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}

	renamedFromPath := filepath.Join(publicDir, "old.png")
	renamedToPath := filepath.Join(publicDir, "new.png")
	deletedPath := filepath.Join(publicDir, "delete.txt")
	createdPath := filepath.Join(publicDir, "create.txt")

	if err := os.WriteFile(renamedFromPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("failed writing renamed-from file: %v", err)
	}
	if err := os.WriteFile(deletedPath, []byte("delete"), 0o644); err != nil {
		t.Fatalf("failed writing deleted file: %v", err)
	}
	if err := builder.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPublicFilesOnly returned error: %v", err)
	}

	initialMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap after initial run returned error: %v", err)
	}
	initialRenamedFromDistPath := filepath.Join(cfg.Dist.StaticPublic(), initialMap["old.png"].DistName)
	initialDeletedDistPath := filepath.Join(cfg.Dist.StaticPublic(), initialMap["delete.txt"].DistName)

	if err := os.Rename(renamedFromPath, renamedToPath); err != nil {
		t.Fatalf("failed renaming file: %v", err)
	}
	if err := os.Remove(deletedPath); err != nil {
		t.Fatalf("failed removing deleted file: %v", err)
	}
	if err := os.WriteFile(createdPath, []byte("create"), 0o644); err != nil {
		t.Fatalf("failed writing created file: %v", err)
	}

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		watcher:   watcher,
		builder:   builder,
		restartCh: make(chan restartRequest, 1),
	}

	s.processEvents([]fsnotify.Event{
		{Name: renamedFromPath, Op: fsnotify.Remove},
		{Name: renamedFromPath, Op: fsnotify.Rename},
		{Name: renamedToPath, Op: fsnotify.Create},
		{Name: renamedToPath, Op: fsnotify.Write},
		{Name: deletedPath, Op: fsnotify.Remove},
		{Name: createdPath, Op: fsnotify.Create},
	})

	updatedMap, err := builder.LoadPublicFileMap()
	if err != nil {
		t.Fatalf("LoadPublicFileMap after processEvents returned error: %v", err)
	}

	if _, exists := updatedMap["old.png"]; exists {
		t.Fatalf("expected old path removed from map, got %#v", updatedMap["old.png"])
	}
	if _, exists := updatedMap["delete.txt"]; exists {
		t.Fatalf("expected deleted path removed from map, got %#v", updatedMap["delete.txt"])
	}
	if _, exists := updatedMap["new.png"]; !exists {
		t.Fatalf("expected renamed path present in map, got %#v", updatedMap)
	}
	if _, exists := updatedMap["create.txt"]; !exists {
		t.Fatalf("expected created path present in map, got %#v", updatedMap)
	}

	if _, statErr := os.Stat(initialRenamedFromDistPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected old renamed dist artifact deleted, stat error: %v", statErr)
	}
	if _, statErr := os.Stat(initialDeletedDistPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected deleted dist artifact deleted, stat error: %v", statErr)
	}
}

func TestProcessEvents_CSSHotReloadSkipsFailedRebuildAndResumesAfterSuccessfulRebuild(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	criticalEntryPath := filepath.Join(root, "styles", "critical.css")
	cfg.Core.CSSEntryFiles = wave.CSSEntryFiles{
		Critical: criticalEntryPath,
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	if err := os.MkdirAll(filepath.Dir(criticalEntryPath), 0o755); err != nil {
		t.Fatalf("failed creating critical css directory: %v", err)
	}
	if err := os.WriteFile(criticalEntryPath, []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatalf("failed writing initial critical css file: %v", err)
	}

	watcher, err := newWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()
	if err := builder.BuildCriticalCSS(true); err != nil {
		t.Fatalf("initial BuildCriticalCSS returned error: %v", err)
	}

	criticalEventPath := ""
	builder.css.mu.RLock()
	for trackedCriticalImportPath := range builder.css.criticalImports {
		criticalEventPath = trackedCriticalImportPath
		break
	}
	builder.css.mu.RUnlock()
	if criticalEventPath == "" {
		t.Fatal("expected initial critical css build to track at least one import path")
	}

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		watcher:   watcher,
		builder:   builder,
		restartCh: make(chan restartRequest, 1),
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 2),
		},
		refreshMgrCtx: context.Background(),
	}

	cfg.Core.CSSEntryFiles.Critical = filepath.Join(root, "styles", "missing-critical.css")
	s.processEvents([]fsnotify.Event{
		{
			Name: criticalEventPath,
			Op:   fsnotify.Write,
		},
	})

	select {
	case payload := <-s.refreshMgr.broadcast:
		t.Fatalf("expected no css payload after failed rebuild, got %#v", payload)
	default:
	}

	cfg.Core.CSSEntryFiles.Critical = criticalEntryPath
	if err := os.WriteFile(criticalEntryPath, []byte("body { color: blue; }"), 0o644); err != nil {
		t.Fatalf("failed writing updated critical css file: %v", err)
	}

	s.processEvents([]fsnotify.Event{
		{
			Name: criticalEventPath,
			Op:   fsnotify.Write,
		},
	})

	select {
	case payload := <-s.refreshMgr.broadcast:
		if payload.ChangeType != changeTypeCriticalCSS {
			t.Fatalf("expected critical css payload after successful rebuild, got %#v", payload)
		}
		decodedCriticalCSS, decodeError := base64.StdEncoding.DecodeString(payload.CriticalCSS)
		if decodeError != nil {
			t.Fatalf("failed decoding critical css payload: %v", decodeError)
		}
		if !strings.Contains(string(decodedCriticalCSS), "blue") {
			t.Fatalf("expected decoded critical css payload to contain updated content, got %q", string(decodedCriticalCSS))
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for css payload after successful rebuild")
	}
}

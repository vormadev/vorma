package runloop_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/restartengine"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/runloop"
	"github.com/vormadev/vorma/wave/tooling/internal/broadcast"
	"github.com/vormadev/vorma/wave/tooling/internal/watch"
)

type configEventPathShapeCaseForRunloopProcessTests struct {
	Name      string
	BuildPath func(t *testing.T, configFilePath string) string
}

func configEventPathShapeCasesForRunloopProcessTests() []configEventPathShapeCaseForRunloopProcessTests {
	return []configEventPathShapeCaseForRunloopProcessTests{
		{
			Name: "exact",
			BuildPath: func(_ *testing.T, configFilePath string) string {
				return configFilePath
			},
		},
		{
			Name: "dot_alias",
			BuildPath: func(_ *testing.T, configFilePath string) string {
				return filepath.Join(
					filepath.Dir(configFilePath),
					".",
					filepath.Base(configFilePath),
				)
			},
		},
		{
			Name: "sibling_relative_alias",
			BuildPath: func(_ *testing.T, configFilePath string) string {
				return filepath.Join(
					filepath.Dir(configFilePath),
					"nested",
					"..",
					filepath.Base(configFilePath),
				)
			},
		},
		{
			Name: "symlink_alias",
			BuildPath: func(t *testing.T, configFilePath string) string {
				t.Helper()

				configDirectoryPath := filepath.Dir(configFilePath)
				aliasDirectoryPath := filepath.Join(
					configDirectoryPath,
					"config_alias_link",
				)
				if symlinkError := os.Symlink(
					configDirectoryPath,
					aliasDirectoryPath,
				); symlinkError != nil {
					t.Fatalf(
						"create config directory symlink alias: %v",
						symlinkError,
					)
				}

				return filepath.Join(
					aliasDirectoryPath,
					filepath.Base(configFilePath),
				)
			},
		},
	}
}

type configMutationCaseForRunloopProcessTests struct {
	Name         string
	Op           fsnotify.Op
	PrepareEvent func(t *testing.T, configFilePath string)
}

func configMutationCasesForRunloopProcessTests() []configMutationCaseForRunloopProcessTests {
	return []configMutationCaseForRunloopProcessTests{
		{
			Name: "write",
			Op:   fsnotify.Write,
		},
		{
			Name: "create",
			Op:   fsnotify.Create,
		},
		{
			Name: "remove",
			Op:   fsnotify.Remove,
			PrepareEvent: func(t *testing.T, configFilePath string) {
				t.Helper()
				if removeError := os.Remove(configFilePath); removeError != nil {
					t.Fatalf(
						"failed removing config file before remove Event: %v",
						removeError,
					)
				}
			},
		},
		{
			Name: "rename",
			Op:   fsnotify.Rename,
			PrepareEvent: func(t *testing.T, configFilePath string) {
				t.Helper()
				if renameError := os.Rename(
					configFilePath,
					configFilePath+".renamed",
				); renameError != nil {
					t.Fatalf(
						"failed renaming config file before rename Event: %v",
						renameError,
					)
				}
			},
		},
	}
}

func runConfigMutationAndPathShapeMatrixForRunloopProcessTests(
	t *testing.T,
	runCase func(
		t *testing.T,
		configMutationCaseForRun configMutationCaseForRunloopProcessTests,
		pathShapeCaseForRun configEventPathShapeCaseForRunloopProcessTests,
	),
) {
	t.Helper()

	for _, configMutationCaseForRun := range configMutationCasesForRunloopProcessTests() {
		for _, pathShapeCaseForRun := range configEventPathShapeCasesForRunloopProcessTests() {
			configMutationCaseForRun := configMutationCaseForRun
			pathShapeCaseForRun := pathShapeCaseForRun

			t.Run(
				configMutationCaseForRun.Name+"_"+pathShapeCaseForRun.Name,
				func(t *testing.T) {
					runCase(
						t,
						configMutationCaseForRun,
						pathShapeCaseForRun,
					)
				},
			)
		}
	}
}

func setupConfigEventTestConfigForRunloopProcessTests(
	t *testing.T,
) (*wave.ParsedConfig, string, string) {
	t.Helper()

	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	configFilePath := filepath.Join(root, "backend", "wave.config.json")
	cfg.Core.ConfigLocation = configFilePath
	cfg.Dist.Root = cfg.Core.DistDir

	if mkdirError := os.MkdirAll(filepath.Dir(configFilePath), 0o755); mkdirError != nil {
		t.Fatalf("failed creating config file directory: %v", mkdirError)
	}
	configPayload := map[string]any{
		"Core": map[string]any{
			"MainAppEntry":   "cmd/app",
			"DistDir":        cfg.Core.DistDir,
			"ServerOnlyMode": true,
			"StaticAssetDirs": map[string]any{
				"Public":  cfg.Core.StaticAssetDirs.Public,
				"Private": cfg.Core.StaticAssetDirs.Private,
			},
		},
		"Watch": map[string]any{
			"WatchRoot": root,
		},
	}
	configPayloadBytes, marshalError := json.Marshal(configPayload)
	if marshalError != nil {
		t.Fatalf("failed marshaling config file payload: %v", marshalError)
	}
	if writeError := os.WriteFile(
		configFilePath,
		configPayloadBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("failed writing config file: %v", writeError)
	}

	return cfg, root, configFilePath
}

func setupProcessEventsServerForRunloopTests(
	t *testing.T,
	cfg *wave.ParsedConfig,
) *runloopTestServer {
	t.Helper()

	watcherForTest, watcherError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	if watcherError != nil {
		t.Fatalf("watch.NewWatcher returned error: %v", watcherError)
	}
	t.Cleanup(func() {
		_ = watcherForTest.Close()
	})

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	t.Cleanup(func() {
		builderForTest.Close()
	})

	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	serverForTest.Watcher = watcherForTest
	serverForTest.Builder = builderForTest
	serverForTest.RestartIntents = restartengine.NewRestartIntentAccumulator(
		make(chan restartengine.RestartRequest, 1),
	)
	return serverForTest
}

func processEventsForRunloopTests(
	t *testing.T,
	serverForTest *runloopTestServer,
	events []fsnotify.Event,
) {
	t.Helper()
	engine := serverForTest.BuildRunloopEngine()
	engine.ProcessEvents(events)
}

func TestProcessEvents_ConfigMutationsTriggerConfigRestart(t *testing.T) {
	runConfigMutationAndPathShapeMatrixForRunloopProcessTests(
		t,
		func(
			t *testing.T,
			configMutationCaseForRun configMutationCaseForRunloopProcessTests,
			pathShapeCaseForRun configEventPathShapeCaseForRunloopProcessTests,
		) {
			cfg, _, configFilePath := setupConfigEventTestConfigForRunloopProcessTests(t)
			if configMutationCaseForRun.PrepareEvent != nil {
				configMutationCaseForRun.PrepareEvent(t, configFilePath)
			}

			serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)

			configEventPath := pathShapeCaseForRun.BuildPath(t, configFilePath)
			processEventsForRunloopTests(
				t,
				serverForTest,
				[]fsnotify.Event{{
					Name: configEventPath,
					Op:   configMutationCaseForRun.Op,
				}},
			)

			pendingRestartRequest := waitForPendingRestartRequestForRunloopTests(
				t,
				serverForTest.RestartIntents,
				200*time.Millisecond,
			)
			if !pendingRestartRequest.IsConfigRestart || !pendingRestartRequest.RecompileGo {
				t.Fatalf(
					"expected config restart with Go recompile, got %#v",
					pendingRestartRequest,
				)
			}
		},
	)
}

func TestProcessEvents_ConfigChangeBatchSkipsNonConfigHookProcessing(t *testing.T) {
	runConfigMutationAndPathShapeMatrixForRunloopProcessTests(
		t,
		func(
			t *testing.T,
			configMutationCaseForRun configMutationCaseForRunloopProcessTests,
			pathShapeCaseForRun configEventPathShapeCaseForRunloopProcessTests,
		) {
			cfg, root, configFilePath := setupConfigEventTestConfigForRunloopProcessTests(t)

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

			nonConfigFilePath := filepath.Join(root, "notes.txt")
			if err := os.WriteFile(nonConfigFilePath, []byte("notes"), 0o644); err != nil {
				t.Fatalf("failed writing non-config file: %v", err)
			}

			if configMutationCaseForRun.PrepareEvent != nil {
				configMutationCaseForRun.PrepareEvent(t, configFilePath)
			}

			serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)

			configEventPath := pathShapeCaseForRun.BuildPath(t, configFilePath)
			processEventsForRunloopTests(
				t,
				serverForTest,
				[]fsnotify.Event{
					{
						Name: configEventPath,
						Op:   configMutationCaseForRun.Op,
					},
					{
						Name: nonConfigFilePath,
						Op:   fsnotify.Write,
					},
				},
			)

			pendingRestartRequest := waitForPendingRestartRequestForRunloopTests(
				t,
				serverForTest.RestartIntents,
				200*time.Millisecond,
			)
			if !pendingRestartRequest.IsConfigRestart || !pendingRestartRequest.RecompileGo {
				t.Fatalf(
					"expected config restart with Go recompile, got %#v",
					pendingRestartRequest,
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
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist.Root = cfg.Core.DistDir

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

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)

	missingFilePath := filepath.Join(root, "new-file.txt")
	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{{
			Name: missingFilePath,
			Op:   fsnotify.Create,
		}},
	)

	if atomic.LoadInt32(&hookCallCount) != 1 {
		t.Fatalf("expected matching hook to run exactly once for missing-file create, got %d", atomic.LoadInt32(&hookCallCount))
	}

	assertNoPendingRestartRequestForRunloopTests(t, serverForTest.RestartIntents)
}

func TestProcessEvents_DeduplicatesEventsByMatchedPattern(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
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
	cfg.Dist.Root = cfg.Core.DistDir

	var callbackCount int32
	cfg.Watch.Include[0].OnChangeHooks[0].Callback = func(*wave.HookContext) (*wave.RefreshAction, error) {
		atomic.AddInt32(&callbackCount, 1)
		return nil, nil
	}

	fileA := filepath.Join(root, "a.txt")
	fileB := filepath.Join(root, "b.txt")
	if err := os.WriteFile(fileA, []byte("a"), 0o644); err != nil {
		t.Fatalf("failed writing %s: %v", fileA, err)
	}
	if err := os.WriteFile(fileB, []byte("b"), 0o644); err != nil {
		t.Fatalf("failed writing %s: %v", fileB, err)
	}

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)

	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{
			{Name: fileA, Op: fsnotify.Write},
			{Name: fileB, Op: fsnotify.Write},
		},
	)

	if got := atomic.LoadInt32(&callbackCount); got != 1 {
		t.Fatalf("expected callback to run once after pattern dedupe, got %d", got)
	}
}

func TestProcessEvents_BatchHardReloadSetsAppStoppedForBatchOnHookContext(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
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
	cfg.Dist.Root = cfg.Core.DistDir

	goFile := filepath.Join(root, "main.go")
	txtFile := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(goFile, []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed writing %s: %v", goFile, err)
	}
	if err := os.WriteFile(txtFile, []byte("notes"), 0o644); err != nil {
		t.Fatalf("failed writing %s: %v", txtFile, err)
	}

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)

	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{
			{Name: goFile, Op: fsnotify.Write},
			{Name: txtFile, Op: fsnotify.Write},
		},
	)

	if !goHookAppStopped.Load() {
		t.Fatal("expected go hook to see AppStoppedForBatch=true in hard-reload batch")
	}
	if !txtHookAppStopped.Load() {
		t.Fatal("expected txt hook to see AppStoppedForBatch=true in hard-reload batch")
	}
}

func TestProcessEvents_IgnoresChmodOnNonEmptyFile(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
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
	cfg.Dist.Root = cfg.Core.DistDir

	filePath := filepath.Join(root, "chmod.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed writing %s: %v", filePath, err)
	}

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)

	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{{Name: filePath, Op: fsnotify.Chmod}},
	)

	if got := atomic.LoadInt32(&callbackCount); got != 0 {
		t.Fatalf("expected chmod-only non-empty event to be ignored, callback count=%d", got)
	}
}

func TestProcessEvents_NewDirectoryCreateEventAddsWatchDir(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist.Root = cfg.Core.DistDir

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)

	newDirectory := filepath.Join(root, "new-child-dir")
	if err := os.MkdirAll(newDirectory, 0o755); err != nil {
		t.Fatalf("failed creating new directory: %v", err)
	}

	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{{
			Name: newDirectory,
			Op:   fsnotify.Create,
		}},
	)

	if !serverForTest.Watcher.IsWatchingDir(newDirectory) {
		t.Fatalf(
			"expected new directory to be added to watcher dirs: %s",
			serverForTest.Watcher.NormalizePath(newDirectory),
		)
	}
}

func TestProcessEvents_PublicStaticMixedOpsBatchAppliesCreateDeleteAndRenameChanges(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Dist.Root = cfg.Core.DistDir

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	builderForTest := serverForTest.Builder

	publicDirectoryPath := cfg.Core.StaticAssetDirs.Public
	if err := os.MkdirAll(publicDirectoryPath, 0o755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}

	renamedFromPath := filepath.Join(publicDirectoryPath, "old.png")
	renamedToPath := filepath.Join(publicDirectoryPath, "new.png")
	deletedPath := filepath.Join(publicDirectoryPath, "delete.txt")
	createdPath := filepath.Join(publicDirectoryPath, "create.txt")

	if err := os.WriteFile(renamedFromPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("failed writing renamed-from file: %v", err)
	}
	if err := os.WriteFile(deletedPath, []byte("delete"), 0o644); err != nil {
		t.Fatalf("failed writing deleted file: %v", err)
	}
	if err := builderForTest.ProcessPublicFilesOnly(); err != nil {
		t.Fatalf("initial ProcessPublicFilesOnly returned error: %v", err)
	}

	initialMap, err := builderForTest.LoadPublicFileMap()
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

	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{
			{Name: renamedFromPath, Op: fsnotify.Remove},
			{Name: renamedFromPath, Op: fsnotify.Rename},
			{Name: renamedToPath, Op: fsnotify.Create},
			{Name: renamedToPath, Op: fsnotify.Write},
			{Name: deletedPath, Op: fsnotify.Remove},
			{Name: createdPath, Op: fsnotify.Create},
		},
	)

	updatedMap, err := builderForTest.LoadPublicFileMap()
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

	if _, statError := os.Stat(initialRenamedFromDistPath); !os.IsNotExist(statError) {
		t.Fatalf("expected old renamed dist artifact deleted, stat error: %v", statError)
	}
	if _, statError := os.Stat(initialDeletedDistPath); !os.IsNotExist(statError) {
		t.Fatalf("expected deleted dist artifact deleted, stat error: %v", statError)
	}
}

func TestProcessEvents_CSSHotReloadSkipsFailedRebuildAndResumesAfterSuccessfulRebuild(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	criticalEntryPath := filepath.Join(root, "styles", "critical.css")
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical: criticalEntryPath,
	}
	cfg.Dist.Root = cfg.Core.DistDir

	if err := os.MkdirAll(filepath.Dir(criticalEntryPath), 0o755); err != nil {
		t.Fatalf("failed creating critical css directory: %v", err)
	}
	if err := os.WriteFile(criticalEntryPath, []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatalf("failed writing initial critical css file: %v", err)
	}

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	builderForTest := serverForTest.Builder
	if err := builderForTest.BuildCSS(
		builder.CSSBuildOptions{
			BuildCriticalCSS: true,
		},
	); err != nil {
		t.Fatalf("initial BuildCSS returned error: %v", err)
	}

	criticalEventPath := criticalEntryPath

	broadcastPayloads := make(chan broadcast.Payload, 2)
	engine := runloop.New(runloop.Dependencies{
		Log:                                serverForTest.Log,
		Config:                             serverForTest.Cfg,
		GetCurrentWatcher:                  serverForTest.WatcherInstance,
		GetCurrentBuilder:                  serverForTest.BuilderInstance,
		CurrentRunCycleContextOrBackground: serverForTest.CurrentRunCycleContextOrBackground,
		ExecuteBuildPhase:                  serverForTest.ExecuteBuildPhase,
		ExecuteBrowserPhase: func(work *eventpipeline.WorkSet) {
			if work == nil || work.Browser.Action != eventpipeline.BrowserPhaseActionHotReloadCSS {
				return
			}
			criticalCSS, readError := builderForTest.ReadCriticalCSSForHotReload(true)
			if readError != nil {
				return
			}
			payloads := eventpipeline.PlanHotReloadCSSPayloads(
				true,
				criticalCSS,
				true,
				false,
				"",
				false,
			)
			for _, payload := range payloads {
				broadcastPayloads <- payload
			}
		},
		StartApp:             serverForTest.StartApp,
		StopApp:              serverForTest.StopApp,
		TriggerRestart:       serverForTest.TriggerRestart,
		TriggerRestartNoGo:   serverForTest.TriggerRestartNoGo,
		TriggerConfigRestart: serverForTest.TriggerConfigRestart,
		BroadcastRebuilding:  serverForTest.BroadcastRebuilding,
		BuildEventExecutionPlan: func(
			events []fsnotify.Event,
			watcherForPlan *watch.Watcher,
			builderForPlan *builder.Builder,
		) eventpipeline.EventExecutionPlanningResult {
			return serverForTest.BuildEventExecutionPlan(events, watcherForPlan, builderForPlan)
		},
		DeriveWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
			traceContext := serverForTest.DeriveWatcherExecutionTraceContext()
			return runloop.WatcherExecutionTraceContext{
				CycleID: traceContext.CycleID,
				BatchID: traceContext.BatchID,
			}
		},
		SetCurrentWatcherExecutionTraceContext: func(traceContext runloop.WatcherExecutionTraceContext) {
			serverForTest.SetCurrentWatcherExecutionTraceContext(traceContext)
		},
		ClearCurrentWatcherExecutionTraceContext: serverForTest.ClearCurrentWatcherExecutionTraceContext,
		GetCurrentWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
			traceContext := serverForTest.CurrentWatcherExecutionTraceContextSnapshot()
			return runloop.WatcherExecutionTraceContext{
				CycleID: traceContext.CycleID,
				BatchID: traceContext.BatchID,
			}
		},
		RunNoWaitHookWithConcurrencyLimit:               serverForTest.RunNoWaitHookWithConcurrencyLimit,
		GetOrCreateConcurrentNoWaitHookLifecycleContext: serverForTest.GetOrCreateConcurrentNoWaitHookLifecycleContext,
		ResolveHookExecutionPlan:                        serverForTest.ResolveHookExecutionPlan,
	})

	cfg.Core.CSSEntryFiles.Critical = filepath.Join(root, "styles", "missing-critical.css")
	engine.ProcessEvents([]fsnotify.Event{
		{
			Name: criticalEventPath,
			Op:   fsnotify.Write,
		},
	})

	select {
	case payload := <-broadcastPayloads:
		t.Fatalf("expected no css payload after failed rebuild, got %#v", payload)
	default:
	}

	cfg.Core.CSSEntryFiles.Critical = criticalEntryPath
	if err := os.WriteFile(criticalEntryPath, []byte("body { color: blue; }"), 0o644); err != nil {
		t.Fatalf("failed writing updated critical css file: %v", err)
	}

	engine.ProcessEvents([]fsnotify.Event{
		{
			Name: criticalEventPath,
			Op:   fsnotify.Write,
		},
	})

	select {
	case payload := <-broadcastPayloads:
		if payload.ChangeType != broadcast.ChangeTypeCriticalCSS {
			t.Fatalf("expected critical css payload after successful rebuild, got %#v", payload)
		}
		decodedCriticalCSS, decodeError := base64.StdEncoding.DecodeString(payload.CriticalCSS)
		if decodeError != nil {
			t.Fatalf("failed decoding critical css payload: %v", decodeError)
		}
		decodedCriticalCSSString := string(decodedCriticalCSS)
		if strings.Contains(decodedCriticalCSSString, "red") {
			t.Fatalf("expected decoded critical css payload to drop stale content, got %q", decodedCriticalCSSString)
		}
		if !strings.Contains(decodedCriticalCSSString, "blue") &&
			!strings.Contains(decodedCriticalCSSString, "#00f") {
			t.Fatalf("expected decoded critical css payload to contain updated content, got %q", decodedCriticalCSSString)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for css payload after successful rebuild")
	}
}

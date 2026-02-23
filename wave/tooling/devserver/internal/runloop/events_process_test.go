package runloop_test

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
			cfg, _, configFilePath := setupConfigEventTestConfigForRunloopProcessTests(
				t,
			)
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
			if !pendingRestartRequest.IsConfigRestart ||
				!pendingRestartRequest.RecompileGo {
				t.Fatalf(
					"expected config restart with Go recompile, got %#v",
					pendingRestartRequest,
				)
			}
		},
	)
}

func TestProcessEvents_ConfigChmodDoesNotTriggerConfigRestart(t *testing.T) {
	cfg, _, configFilePath := setupConfigEventTestConfigForRunloopProcessTests(
		t,
	)
	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)

	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{{
			Name: configFilePath,
			Op:   fsnotify.Chmod,
		}},
	)

	assertNoPendingRestartRequestForRunloopTests(
		t,
		serverForTest.RestartIntents,
	)
}

func TestProcessEvents_ConfigChangeBatchSkipsNonConfigHookProcessing(
	t *testing.T,
) {
	runConfigMutationAndPathShapeMatrixForRunloopProcessTests(
		t,
		func(
			t *testing.T,
			configMutationCaseForRun configMutationCaseForRunloopProcessTests,
			pathShapeCaseForRun configEventPathShapeCaseForRunloopProcessTests,
		) {
			cfg, root, configFilePath := setupConfigEventTestConfigForRunloopProcessTests(
				t,
			)

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
			if !pendingRestartRequest.IsConfigRestart ||
				!pendingRestartRequest.RecompileGo {
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

func TestProcessEvents_CreateForMissingFileStillRunsMatchingHooks(
	t *testing.T,
) {
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
		t.Fatalf(
			"expected matching hook to run exactly once for missing-file create, got %d",
			atomic.LoadInt32(&hookCallCount),
		)
	}

	assertNoPendingRestartRequestForRunloopTests(
		t,
		serverForTest.RestartIntents,
	)
}

func TestProcessEvents_RenameForMissingFileStillRunsMatchingHooks(
	t *testing.T,
) {
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

	missingRenamePath := filepath.Join(root, "renamed-file.txt")
	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{{
			Name: missingRenamePath,
			Op:   fsnotify.Rename,
		}},
	)

	if atomic.LoadInt32(&hookCallCount) != 1 {
		t.Fatalf(
			"expected matching hook to run exactly once for missing-file rename, got %d",
			atomic.LoadInt32(&hookCallCount),
		)
	}

	assertNoPendingRestartRequestForRunloopTests(
		t,
		serverForTest.RestartIntents,
	)
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
		t.Fatalf(
			"expected callback to run once after pattern dedupe, got %d",
			got,
		)
	}
}

func TestProcessEvents_BatchHardReloadSetsAppStoppedForBatchOnHookContext(
	t *testing.T,
) {
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
		t.Fatal(
			"expected go hook to see AppStoppedForBatch=true in hard-reload batch",
		)
	}
	if !txtHookAppStopped.Load() {
		t.Fatal(
			"expected txt hook to see AppStoppedForBatch=true in hard-reload batch",
		)
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
		t.Fatalf(
			"expected chmod-only non-empty event to be ignored, callback count=%d",
			got,
		)
	}
}

func TestProcessEvents_LogsWatcherEventsWithCycleAndBatchTraceFields(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist.Root = cfg.Core.DistDir

	watchedTextFilePath := filepath.Join(root, "notes.txt")
	if writeError := os.WriteFile(watchedTextFilePath, []byte("notes"), 0o644); writeError != nil {
		t.Fatalf("write watched text file: %v", writeError)
	}

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

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	var logBuffer bytes.Buffer
	serverForTest.Log = slog.New(slog.NewTextHandler(&logBuffer, nil))

	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{
			{
				Name: watchedTextFilePath,
				Op:   fsnotify.Write,
			},
		},
	)

	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "watch event") {
		t.Fatalf("expected watcher event log entry, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "cycle_id=") {
		t.Fatalf(
			"expected watcher event log to include cycle_id, got %q",
			logOutput,
		)
	}
	if !strings.Contains(logOutput, "batch_id=") {
		t.Fatalf(
			"expected watcher event log to include batch_id, got %q",
			logOutput,
		)
	}
}

func TestProcessEvents_LogsWarningWhenBatchDurationExceedsThreshold(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist.Root = cfg.Core.DistDir

	watchedTextFilePath := filepath.Join(root, "slow-notes.txt")
	if writeError := os.WriteFile(
		watchedTextFilePath,
		[]byte("notes"),
		0o644,
	); writeError != nil {
		t.Fatalf("write watched text file: %v", writeError)
	}

	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         "**/*.txt",
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						time.Sleep(35 * time.Millisecond)
						return nil, nil
					},
				},
			},
		},
	}

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	serverForTest.watcherBatchDurationWarningThreshold = 5 * time.Millisecond
	var logBuffer bytes.Buffer
	serverForTest.Log = slog.New(slog.NewTextHandler(&logBuffer, nil))

	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{
			{
				Name: watchedTextFilePath,
				Op:   fsnotify.Write,
			},
		},
	)

	logOutput := logBuffer.String()
	if !strings.Contains(
		logOutput,
		"watcher batch processing exceeded duration threshold",
	) {
		t.Fatalf(
			"expected slow watcher-batch warning log entry, got %q",
			logOutput,
		)
	}
	if !strings.Contains(logOutput, "cycle_id=") {
		t.Fatalf(
			"expected slow watcher-batch warning to include cycle_id, got %q",
			logOutput,
		)
	}
	if !strings.Contains(logOutput, "batch_id=") {
		t.Fatalf(
			"expected slow watcher-batch warning to include batch_id, got %q",
			logOutput,
		)
	}
}

func TestProcessEvents_DoesNotLogBatchDurationWarningWhenBelowThreshold(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist.Root = cfg.Core.DistDir

	watchedTextFilePath := filepath.Join(root, "fast-notes.txt")
	if writeError := os.WriteFile(
		watchedTextFilePath,
		[]byte("notes"),
		0o644,
	); writeError != nil {
		t.Fatalf("write watched text file: %v", writeError)
	}

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

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	serverForTest.watcherBatchDurationWarningThreshold = 2 * time.Second
	var logBuffer bytes.Buffer
	serverForTest.Log = slog.New(slog.NewTextHandler(&logBuffer, nil))

	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{
			{
				Name: watchedTextFilePath,
				Op:   fsnotify.Write,
			},
		},
	)

	logOutput := logBuffer.String()
	if strings.Contains(
		logOutput,
		"watcher batch processing exceeded duration threshold",
	) {
		t.Fatalf(
			"did not expect slow watcher-batch warning when below threshold, got %q",
			logOutput,
		)
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
	assertNoPendingRestartRequestForRunloopTests(
		t,
		serverForTest.RestartIntents,
	)
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
	initialRenamedFromDistPath := filepath.Join(
		cfg.Dist.StaticPublic(),
		initialMap["old.png"].DistName,
	)
	initialDeletedDistPath := filepath.Join(
		cfg.Dist.StaticPublic(),
		initialMap["delete.txt"].DistName,
	)

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
		t.Fatalf(
			"LoadPublicFileMap after processEvents returned error: %v",
			err,
		)
	}

	if _, exists := updatedMap["old.png"]; exists {
		t.Fatalf(
			"expected old path removed from map, got %#v",
			updatedMap["old.png"],
		)
	}
	if _, exists := updatedMap["delete.txt"]; exists {
		t.Fatalf(
			"expected deleted path removed from map, got %#v",
			updatedMap["delete.txt"],
		)
	}
	if _, exists := updatedMap["new.png"]; !exists {
		t.Fatalf("expected renamed path present in map, got %#v", updatedMap)
	}
	if _, exists := updatedMap["create.txt"]; !exists {
		t.Fatalf("expected created path present in map, got %#v", updatedMap)
	}

	if _, statError := os.Stat(initialRenamedFromDistPath); !os.IsNotExist(
		statError,
	) {
		t.Fatalf(
			"expected old renamed dist artifact deleted, stat error: %v",
			statError,
		)
	}
	if _, statError := os.Stat(initialDeletedDistPath); !os.IsNotExist(
		statError,
	) {
		t.Fatalf(
			"expected deleted dist artifact deleted, stat error: %v",
			statError,
		)
	}
}

func TestProcessEvents_PublicStaticDirectoryRenameWithoutChildFileEvents(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Dist.Root = cfg.Core.DistDir

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	builderForTest := serverForTest.Builder

	publicDirectoryPath := cfg.Core.StaticAssetDirs.Public
	oldDirectoryPath := filepath.Join(publicDirectoryPath, "icons", "old")
	newDirectoryPath := filepath.Join(publicDirectoryPath, "icons", "new")
	oldFilePath := filepath.Join(oldDirectoryPath, "logo.svg")
	if mkdirError := os.MkdirAll(oldDirectoryPath, 0o755); mkdirError != nil {
		t.Fatalf("failed creating old subtree directory: %v", mkdirError)
	}
	if writeError := os.WriteFile(oldFilePath, []byte("<svg>old</svg>"), 0o644); writeError != nil {
		t.Fatalf("failed writing old subtree file: %v", writeError)
	}

	if processError := builderForTest.ProcessPublicFilesOnly(); processError != nil {
		t.Fatalf(
			"initial ProcessPublicFilesOnly returned error: %v",
			processError,
		)
	}

	initialMap, loadMapError := builderForTest.LoadPublicFileMap()
	if loadMapError != nil {
		t.Fatalf(
			"LoadPublicFileMap after initial run returned error: %v",
			loadMapError,
		)
	}
	oldEntry := initialMap["icons/old/logo.svg"]
	oldDistPath := filepath.Join(cfg.Dist.StaticPublic(), oldEntry.DistName)

	if renameError := os.Rename(oldDirectoryPath, newDirectoryPath); renameError != nil {
		t.Fatalf("failed renaming public subtree directory: %v", renameError)
	}

	// Some watcher backends can emit directory-level rename/create events without
	// child file events. The processor must still converge to the renamed subtree.
	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{
			{Name: oldDirectoryPath, Op: fsnotify.Rename},
			{Name: oldDirectoryPath, Op: fsnotify.Remove},
			{Name: newDirectoryPath, Op: fsnotify.Create},
		},
	)

	updatedMap, updatedLoadMapError := builderForTest.LoadPublicFileMap()
	if updatedLoadMapError != nil {
		t.Fatalf(
			"LoadPublicFileMap after processEvents returned error: %v",
			updatedLoadMapError,
		)
	}
	if _, exists := updatedMap["icons/old/logo.svg"]; exists {
		t.Fatalf(
			"expected old path removed from map after directory rename, got %#v",
			updatedMap["icons/old/logo.svg"],
		)
	}
	if _, exists := updatedMap["icons/new/logo.svg"]; !exists {
		t.Fatalf("expected renamed path present in map, got %#v", updatedMap)
	}
	if _, statError := os.Stat(oldDistPath); !os.IsNotExist(statError) {
		t.Fatalf(
			"expected old renamed dist artifact deleted, stat error: %v",
			statError,
		)
	}
}

func TestProcessEvents_PrivateStaticDirectoryRenameWithoutChildFileEvents(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Dist.Root = cfg.Core.DistDir

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	builderForTest := serverForTest.Builder

	privateDirectoryPath := cfg.Core.StaticAssetDirs.Private
	oldDirectoryPath := filepath.Join(privateDirectoryPath, "templates", "old")
	newDirectoryPath := filepath.Join(privateDirectoryPath, "templates", "new")
	oldFilePathA := filepath.Join(oldDirectoryPath, "a.html")
	oldFilePathB := filepath.Join(oldDirectoryPath, "nested", "b.html")
	if mkdirError := os.MkdirAll(filepath.Dir(oldFilePathB), 0o755); mkdirError != nil {
		t.Fatalf(
			"failed creating old private subtree directory: %v",
			mkdirError,
		)
	}
	if writeError := os.WriteFile(oldFilePathA, []byte("<h1>a</h1>"), 0o644); writeError != nil {
		t.Fatalf("failed writing old private subtree file a: %v", writeError)
	}
	if writeError := os.WriteFile(oldFilePathB, []byte("<h1>b</h1>"), 0o644); writeError != nil {
		t.Fatalf("failed writing old private subtree file b: %v", writeError)
	}

	if processError := builderForTest.ProcessPrivateFilesOnly(); processError != nil {
		t.Fatalf(
			"initial ProcessPrivateFilesOnly returned error: %v",
			processError,
		)
	}

	initialMap := loadStaticFileMapFromGobPathForRunloopProcessTests(
		t,
		cfg.Dist.PrivateFileMapGob(),
	)
	oldEntryA, hasOldEntryA := initialMap["templates/old/a.html"]
	if !hasOldEntryA {
		t.Fatalf(
			"expected templates/old/a.html in initial private map, got %#v",
			initialMap,
		)
	}
	oldEntryB, hasOldEntryB := initialMap["templates/old/nested/b.html"]
	if !hasOldEntryB {
		t.Fatalf(
			"expected templates/old/nested/b.html in initial private map, got %#v",
			initialMap,
		)
	}
	oldDistPathA := filepath.Join(cfg.Dist.StaticPrivate(), oldEntryA.DistName)
	oldDistPathB := filepath.Join(cfg.Dist.StaticPrivate(), oldEntryB.DistName)

	if renameError := os.Rename(oldDirectoryPath, newDirectoryPath); renameError != nil {
		t.Fatalf("failed renaming private subtree directory: %v", renameError)
	}

	// Some watcher backends can emit directory-level rename/create events without
	// child file events. The processor must still converge to the renamed subtree.
	processEventsForRunloopTests(
		t,
		serverForTest,
		[]fsnotify.Event{
			{Name: oldDirectoryPath, Op: fsnotify.Rename},
			{Name: oldDirectoryPath, Op: fsnotify.Remove},
			{Name: newDirectoryPath, Op: fsnotify.Create},
		},
	)

	updatedMap := loadStaticFileMapFromGobPathForRunloopProcessTests(
		t,
		cfg.Dist.PrivateFileMapGob(),
	)
	if _, exists := updatedMap["templates/old/a.html"]; exists {
		t.Fatalf(
			"expected old private path removed from map after directory rename, got %#v",
			updatedMap["templates/old/a.html"],
		)
	}
	if _, exists := updatedMap["templates/old/nested/b.html"]; exists {
		t.Fatalf(
			"expected old private nested path removed from map after directory rename, got %#v",
			updatedMap["templates/old/nested/b.html"],
		)
	}
	if _, exists := updatedMap["templates/new/a.html"]; !exists {
		t.Fatalf(
			"expected renamed private path present in map, got %#v",
			updatedMap,
		)
	}
	if _, exists := updatedMap["templates/new/nested/b.html"]; !exists {
		t.Fatalf(
			"expected renamed private nested path present in map, got %#v",
			updatedMap,
		)
	}

	if _, statError := os.Stat(oldDistPathA); !os.IsNotExist(statError) {
		t.Fatalf(
			"expected old renamed private dist artifact a deleted, stat error: %v",
			statError,
		)
	}
	if _, statError := os.Stat(oldDistPathB); !os.IsNotExist(statError) {
		t.Fatalf(
			"expected old renamed private dist artifact b deleted, stat error: %v",
			statError,
		)
	}

	assertNoPendingRestartRequestForRunloopTests(
		t,
		serverForTest.RestartIntents,
	)
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
			if work == nil ||
				work.Browser.Action != eventpipeline.BrowserPhaseActionHotReloadCSS {
				return
			}
			criticalCSS, readError := builderForTest.ReadCriticalCSSForHotReload(
				true,
			)
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
			return serverForTest.BuildEventExecutionPlan(
				events,
				watcherForPlan,
				builderForPlan,
			)
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

	cfg.Core.CSSEntryFiles.Critical = filepath.Join(
		root,
		"styles",
		"missing-critical.css",
	)
	engine.ProcessEvents([]fsnotify.Event{
		{
			Name: criticalEventPath,
			Op:   fsnotify.Write,
		},
	})

	select {
	case payload := <-broadcastPayloads:
		t.Fatalf(
			"expected no css payload after failed rebuild, got %#v",
			payload,
		)
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
			t.Fatalf(
				"expected critical css payload after successful rebuild, got %#v",
				payload,
			)
		}
		decodedCriticalCSS, decodeError := base64.StdEncoding.DecodeString(
			payload.CriticalCSS,
		)
		if decodeError != nil {
			t.Fatalf("failed decoding critical css payload: %v", decodeError)
		}
		decodedCriticalCSSString := string(decodedCriticalCSS)
		if strings.Contains(decodedCriticalCSSString, "red") {
			t.Fatalf(
				"expected decoded critical css payload to drop stale content, got %q",
				decodedCriticalCSSString,
			)
		}
		if !strings.Contains(decodedCriticalCSSString, "blue") &&
			!strings.Contains(decodedCriticalCSSString, "#00f") {
			t.Fatalf(
				"expected decoded critical css payload to contain updated content, got %q",
				decodedCriticalCSSString,
			)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for css payload after successful rebuild")
	}
}

type runloopWorkCaptureForProcessTests struct {
	mutex               sync.Mutex
	buildWorkSnapshots  []eventpipeline.WorkSet
	browserWorkSnapshot []eventpipeline.WorkSet
}

func (capture *runloopWorkCaptureForProcessTests) Reset() {
	if capture == nil {
		return
	}
	capture.mutex.Lock()
	defer capture.mutex.Unlock()
	capture.buildWorkSnapshots = nil
	capture.browserWorkSnapshot = nil
}

func (capture *runloopWorkCaptureForProcessTests) RecordBuildWork(
	work *eventpipeline.WorkSet,
) {
	if capture == nil {
		return
	}
	workSnapshot := cloneWorkSetForProcessTests(work)
	capture.mutex.Lock()
	capture.buildWorkSnapshots = append(
		capture.buildWorkSnapshots,
		workSnapshot,
	)
	capture.mutex.Unlock()
}

func (capture *runloopWorkCaptureForProcessTests) RecordBrowserWork(
	work *eventpipeline.WorkSet,
) {
	if capture == nil {
		return
	}
	workSnapshot := cloneWorkSetForProcessTests(work)
	capture.mutex.Lock()
	capture.browserWorkSnapshot = append(
		capture.browserWorkSnapshot,
		workSnapshot,
	)
	capture.mutex.Unlock()
}

func (capture *runloopWorkCaptureForProcessTests) BuildWorkSnapshots() []eventpipeline.WorkSet {
	if capture == nil {
		return nil
	}
	capture.mutex.Lock()
	defer capture.mutex.Unlock()
	snapshots := make(
		[]eventpipeline.WorkSet,
		0,
		len(capture.buildWorkSnapshots),
	)
	for index := range capture.buildWorkSnapshots {
		snapshot := cloneWorkSetForProcessTests(
			&capture.buildWorkSnapshots[index],
		)
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func (capture *runloopWorkCaptureForProcessTests) BrowserWorkSnapshots() []eventpipeline.WorkSet {
	if capture == nil {
		return nil
	}
	capture.mutex.Lock()
	defer capture.mutex.Unlock()
	snapshots := make(
		[]eventpipeline.WorkSet,
		0,
		len(capture.browserWorkSnapshot),
	)
	for index := range capture.browserWorkSnapshot {
		snapshot := cloneWorkSetForProcessTests(
			&capture.browserWorkSnapshot[index],
		)
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func cloneWorkSetForProcessTests(
	work *eventpipeline.WorkSet,
) eventpipeline.WorkSet {
	if work == nil {
		return eventpipeline.WorkSet{}
	}

	clone := *work
	clone.Build.PublicStaticChangedFilePaths = append(
		[]string(nil),
		work.Build.PublicStaticChangedFilePaths...,
	)
	clone.Build.PrivateStaticChangedFilePaths = append(
		[]string(nil),
		work.Build.PrivateStaticChangedFilePaths...,
	)
	clone.Build.PublicStaticChangedFilePathSet = cloneStringSetForProcessTests(
		work.Build.PublicStaticChangedFilePathSet,
	)
	clone.Build.PrivateStaticChangedFilePathSet = cloneStringSetForProcessTests(
		work.Build.PrivateStaticChangedFilePathSet,
	)

	return clone
}

func cloneStringSetForProcessTests(
	input map[string]struct{},
) map[string]struct{} {
	if len(input) == 0 {
		return nil
	}
	clone := make(map[string]struct{}, len(input))
	for key := range input {
		clone[key] = struct{}{}
	}
	return clone
}

func buildRunloopEngineWithWorkCaptureForProcessTests(
	serverForTest *runloopTestServer,
	capture *runloopWorkCaptureForProcessTests,
) *runloop.Engine {
	if serverForTest == nil {
		return runloop.New(runloop.Dependencies{})
	}

	return runloop.New(runloop.Dependencies{
		Log:                                serverForTest.Log,
		Config:                             serverForTest.Cfg,
		GetCurrentWatcher:                  serverForTest.WatcherInstance,
		GetCurrentBuilder:                  serverForTest.BuilderInstance,
		CurrentRunCycleContextOrBackground: serverForTest.CurrentRunCycleContextOrBackground,
		ExecuteBuildPhase: func(work *eventpipeline.WorkSet) error {
			if capture != nil {
				capture.RecordBuildWork(work)
			}
			return nil
		},
		ExecuteBrowserPhase: func(work *eventpipeline.WorkSet) {
			if capture != nil {
				capture.RecordBrowserWork(work)
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
			return serverForTest.BuildEventExecutionPlan(
				events,
				watcherForPlan,
				builderForPlan,
			)
		},
		DeriveWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
			traceContext := serverForTest.DeriveWatcherExecutionTraceContext()
			return runloop.WatcherExecutionTraceContext{
				CycleID: traceContext.CycleID,
				BatchID: traceContext.BatchID,
			}
		},
		SetCurrentWatcherExecutionTraceContext: func(
			traceContext runloop.WatcherExecutionTraceContext,
		) {
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
}

type siteStylePathMatrixForRunloopProcessTests struct {
	CriticalCSSEntryPath      string
	NormalCSSEntryPath        string
	CriticalCSSImportPath     string
	NormalCSSImportPath       string
	TemplatePath              string
	MarkdownPath              string
	RouteRegistryPath         string
	PublicStaticPath          string
	PrivateStaticPath         string
	TailwindCSSPath           string
	RandomFrontendTSPath      string
	VormaFrontendEntryTSXPath string
	GoSourcePath              string
}

func configureSiteStyleFixtureForRunloopProcessTests(
	t *testing.T,
	cfg *wave.ParsedConfig,
	root string,
) siteStylePathMatrixForRunloopProcessTests {
	t.Helper()

	cfg.Core.ServerOnlyMode = false
	cfg.Core.StaticAssetDirs.Public = filepath.Join(root, "frontend", "assets")
	cfg.Core.StaticAssetDirs.Private = filepath.Join(root, "backend", "assets")
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical: filepath.Join(
			root,
			"frontend",
			"src",
			"styles",
			"main.critical.css",
		),
		NonCritical: filepath.Join(
			root,
			"frontend",
			"src",
			"styles",
			"main.css",
		),
	}

	paths := siteStylePathMatrixForRunloopProcessTests{
		CriticalCSSEntryPath: cfg.Core.CSSEntryFiles.Critical,
		NormalCSSEntryPath:   cfg.Core.CSSEntryFiles.NonCritical,
		CriticalCSSImportPath: filepath.Join(
			root,
			"frontend",
			"src",
			"styles",
			"critical_import.css",
		),
		NormalCSSImportPath: filepath.Join(
			root,
			"frontend",
			"src",
			"styles",
			"fonts.css",
		),
		TemplatePath: filepath.Join(
			root,
			"backend",
			"assets",
			"entry.go.html",
		),
		MarkdownPath: filepath.Join(
			root,
			"backend",
			"assets",
			"markdown",
			"blog",
			"post.md",
		),
		RouteRegistryPath: filepath.Join(
			root,
			"frontend",
			"src",
			"routes",
			"core.vorma.routes.ts",
		),
		PublicStaticPath: filepath.Join(
			cfg.Core.StaticAssetDirs.Public,
			"logo.svg",
		),
		PrivateStaticPath: filepath.Join(
			cfg.Core.StaticAssetDirs.Private,
			"notes.txt",
		),
		TailwindCSSPath: filepath.Join(
			root,
			"frontend",
			"src",
			"styles",
			"tailwind.css",
		),
		RandomFrontendTSPath: filepath.Join(
			root,
			"frontend",
			"src",
			"lib",
			"client_util.ts",
		),
		VormaFrontendEntryTSXPath: filepath.Join(
			root,
			"frontend",
			"src",
			"vorma.entry.tsx",
		),
		GoSourcePath: filepath.Join(
			root,
			"backend",
			"handlers",
			"health.go",
		),
	}

	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:                            "backend/assets/markdown/**/*.md",
			OnlyRunClientDefinedRevalidateFunc: true,
			SkipRebuildingNotification:         true,
		},
		{
			Pattern:                    "frontend/src/**/*vorma.routes.ts",
			RunOnChangeOnly:            true,
			SkipRebuildingNotification: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Timing: wave.OnChangeStrategyPost,
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{
							ReloadBrowser: true,
							WaitForApp:    true,
							WaitForVite:   true,
						}, nil
					},
				},
			},
		},
		{
			Pattern:                    "backend/assets/entry.go.html",
			SkipRebuildingNotification: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Timing: wave.OnChangeStrategyPost,
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{
							ReloadBrowser: true,
							WaitForApp:    true,
							WaitForVite:   true,
						}, nil
					},
				},
			},
		},
	}

	directoriesToCreate := []string{
		filepath.Dir(paths.CriticalCSSEntryPath),
		filepath.Dir(paths.NormalCSSEntryPath),
		filepath.Dir(paths.TemplatePath),
		filepath.Dir(paths.MarkdownPath),
		filepath.Dir(paths.RouteRegistryPath),
		filepath.Dir(paths.PublicStaticPath),
		filepath.Dir(paths.PrivateStaticPath),
		filepath.Dir(paths.RandomFrontendTSPath),
		filepath.Dir(paths.VormaFrontendEntryTSXPath),
		filepath.Dir(paths.GoSourcePath),
	}
	for _, directoryPath := range directoriesToCreate {
		if mkdirError := os.MkdirAll(directoryPath, 0o755); mkdirError != nil {
			t.Fatalf("create directory %q: %v", directoryPath, mkdirError)
		}
	}

	filesToWrite := map[string]string{
		paths.CriticalCSSEntryPath:      `@import "./critical_import.css"; body { color: red; }`,
		paths.NormalCSSEntryPath:        `@import "./fonts.css"; body { color: blue; }`,
		paths.CriticalCSSImportPath:     `.critical-import { display: block; }`,
		paths.NormalCSSImportPath:       `.normal-import { font-size: 16px; }`,
		paths.TemplatePath:              `<!doctype html><html><body>{{.VormaBodyScripts}}</body></html>`,
		paths.MarkdownPath:              "# Post\n\nhello",
		paths.RouteRegistryPath:         "export const routes = []",
		paths.PublicStaticPath:          "<svg></svg>",
		paths.PrivateStaticPath:         "private note",
		paths.TailwindCSSPath:           "@tailwind utilities;",
		paths.RandomFrontendTSPath:      "export const clientUtil = () => 'ok'",
		paths.VormaFrontendEntryTSXPath: "export const App = () => null",
		paths.GoSourcePath:              "package handlers\n\nfunc Health() string { return \"ok\" }\n",
	}
	for filePath, fileContents := range filesToWrite {
		if writeError := os.WriteFile(filePath, []byte(fileContents), 0o644); writeError != nil {
			t.Fatalf("write file %q: %v", filePath, writeError)
		}
	}

	return paths
}

func loadStaticFileMapFromGobPathForRunloopProcessTests(
	t *testing.T,
	gobPath string,
) wave.FileMap {
	t.Helper()

	gobBytes, readError := os.ReadFile(gobPath)
	if readError != nil {
		t.Fatalf("failed reading file map gob %q: %v", gobPath, readError)
	}

	var decodedFileMap wave.FileMap
	if decodeError := gob.NewDecoder(bytes.NewReader(gobBytes)).Decode(&decodedFileMap); decodeError != nil {
		t.Fatalf("failed decoding file map gob %q: %v", gobPath, decodeError)
	}
	if decodedFileMap == nil {
		return make(wave.FileMap)
	}
	return decodedFileMap
}

func TestProcessEvents_SiteStyleMatrixUsesMinimumWorkByFileType(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Dist.Root = cfg.Core.DistDir
	paths := configureSiteStyleFixtureForRunloopProcessTests(t, cfg, root)

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	if buildCSSError := serverForTest.Builder.BuildCSS(
		builder.CSSBuildOptions{
			BuildCriticalCSS: true,
			BuildNormalCSS:   true,
		},
	); buildCSSError != nil {
		t.Fatalf("initial BuildCSS returned error: %v", buildCSSError)
	}

	capture := &runloopWorkCaptureForProcessTests{}
	engine := buildRunloopEngineWithWorkCaptureForProcessTests(
		serverForTest,
		capture,
	)

	resetStateForCase := func() {
		capture.Reset()
		serverForTest.RestartIntents = restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		)
	}

	assertNoBuildOrBrowserWork := func(t *testing.T) {
		t.Helper()
		buildWorkSnapshots := capture.BuildWorkSnapshots()
		browserWorkSnapshots := capture.BrowserWorkSnapshots()
		if len(buildWorkSnapshots) != 0 || len(browserWorkSnapshots) != 0 {
			t.Fatalf(
				"expected no build/browser work, build=%#v browser=%#v",
				buildWorkSnapshots,
				browserWorkSnapshots,
			)
		}
	}

	assertSingleBuildWorkSnapshot := func(t *testing.T) eventpipeline.WorkSet {
		t.Helper()
		buildWorkSnapshots := capture.BuildWorkSnapshots()
		if len(buildWorkSnapshots) != 1 {
			t.Fatalf(
				"expected one build work snapshot, got %#v",
				buildWorkSnapshots,
			)
		}
		return buildWorkSnapshots[0]
	}

	assertSingleBrowserWorkSnapshot := func(t *testing.T) eventpipeline.WorkSet {
		t.Helper()
		browserWorkSnapshots := capture.BrowserWorkSnapshots()
		if len(browserWorkSnapshots) != 1 {
			t.Fatalf(
				"expected one browser work snapshot, got %#v",
				browserWorkSnapshots,
			)
		}
		return browserWorkSnapshots[0]
	}

	testCases := []struct {
		Name     string
		FilePath string
		Op       fsnotify.Op
		Assert   func(*testing.T)
	}{
		{
			Name:     "critical_css_entry_builds_critical_css_only",
			FilePath: paths.CriticalCSSEntryPath,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				buildWork := assertSingleBuildWorkSnapshot(t)
				if !buildWork.Build.BuildCriticalCSS ||
					buildWork.Build.BuildNormalCSS ||
					buildWork.Build.ProcessPublicFiles ||
					buildWork.Build.ProcessPrivateFiles ||
					buildWork.Build.CompileGo {
					t.Fatalf(
						"critical css entry expected critical-css-only work, got %#v",
						buildWork.Build,
					)
				}
				browserWork := assertSingleBrowserWorkSnapshot(t)
				if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHotReloadCSS {
					t.Fatalf(
						"critical css entry browser action = %v, want %v",
						browserWork.Browser.Action,
						eventpipeline.BrowserPhaseActionHotReloadCSS,
					)
				}
			},
		},
		{
			Name:     "normal_css_import_builds_normal_css_only",
			FilePath: paths.NormalCSSImportPath,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				buildWork := assertSingleBuildWorkSnapshot(t)
				if buildWork.Build.BuildCriticalCSS ||
					!buildWork.Build.BuildNormalCSS ||
					buildWork.Build.ProcessPublicFiles ||
					buildWork.Build.ProcessPrivateFiles ||
					buildWork.Build.CompileGo {
					t.Fatalf(
						"normal css import expected normal-css-only work, got %#v",
						buildWork.Build,
					)
				}
				browserWork := assertSingleBrowserWorkSnapshot(t)
				if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHotReloadCSS {
					t.Fatalf(
						"normal css import browser action = %v, want %v",
						browserWork.Browser.Action,
						eventpipeline.BrowserPhaseActionHotReloadCSS,
					)
				}
			},
		},
		{
			Name:     "normal_css_entry_builds_normal_css_only",
			FilePath: paths.NormalCSSEntryPath,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				buildWork := assertSingleBuildWorkSnapshot(t)
				if buildWork.Build.BuildCriticalCSS ||
					!buildWork.Build.BuildNormalCSS ||
					buildWork.Build.ProcessPublicFiles ||
					buildWork.Build.ProcessPrivateFiles ||
					buildWork.Build.CompileGo {
					t.Fatalf(
						"normal css entry expected normal-css-only work, got %#v",
						buildWork.Build,
					)
				}
				browserWork := assertSingleBrowserWorkSnapshot(t)
				if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHotReloadCSS {
					t.Fatalf(
						"normal css entry browser action = %v, want %v",
						browserWork.Browser.Action,
						eventpipeline.BrowserPhaseActionHotReloadCSS,
					)
				}
			},
		},
		{
			Name:     "critical_css_import_builds_critical_css_only",
			FilePath: paths.CriticalCSSImportPath,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				buildWork := assertSingleBuildWorkSnapshot(t)
				if !buildWork.Build.BuildCriticalCSS ||
					buildWork.Build.BuildNormalCSS ||
					buildWork.Build.ProcessPublicFiles ||
					buildWork.Build.ProcessPrivateFiles ||
					buildWork.Build.CompileGo {
					t.Fatalf(
						"critical css import expected critical-css-only work, got %#v",
						buildWork.Build,
					)
				}
				browserWork := assertSingleBrowserWorkSnapshot(t)
				if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHotReloadCSS {
					t.Fatalf(
						"critical css import browser action = %v, want %v",
						browserWork.Browser.Action,
						eventpipeline.BrowserPhaseActionHotReloadCSS,
					)
				}
			},
		},
		{
			Name:     "template_write_processes_private_changed_path_and_fast_reload",
			FilePath: paths.TemplatePath,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				buildWork := assertSingleBuildWorkSnapshot(t)
				if !buildWork.Build.ProcessPrivateFiles ||
					len(buildWork.Build.PrivateStaticChangedFilePaths) != 1 ||
					buildWork.Build.PrivateStaticChangedFilePaths[0] != paths.TemplatePath {
					t.Fatalf(
						"template write expected one private changed path %q, got %#v",
						paths.TemplatePath,
						buildWork.Build,
					)
				}
				if buildWork.Build.ProcessPublicFiles ||
					buildWork.Build.BuildCriticalCSS ||
					buildWork.Build.BuildNormalCSS ||
					buildWork.Build.CompileGo {
					t.Fatalf(
						"template write expected no unrelated build work, got %#v",
						buildWork.Build,
					)
				}
				browserWork := assertSingleBrowserWorkSnapshot(t)
				if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
					!browserWork.Browser.WaitForApp ||
					!browserWork.Browser.WaitForVite {
					t.Fatalf(
						"template write browser decision = %#v, want hard reload waiting for app+vite",
						browserWork.Browser,
					)
				}
			},
		},
		{
			Name:     "markdown_write_revalidates_and_processes_private_changed_path",
			FilePath: paths.MarkdownPath,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				buildWork := assertSingleBuildWorkSnapshot(t)
				if !buildWork.Build.ProcessPrivateFiles ||
					len(buildWork.Build.PrivateStaticChangedFilePaths) != 1 ||
					buildWork.Build.PrivateStaticChangedFilePaths[0] != paths.MarkdownPath {
					t.Fatalf(
						"markdown write expected one private changed path %q, got %#v",
						paths.MarkdownPath,
						buildWork.Build,
					)
				}
				if !buildWork.PreferRevalidate {
					t.Fatalf(
						"markdown write expected PreferRevalidate=true, got %#v",
						buildWork,
					)
				}
				browserWork := assertSingleBrowserWorkSnapshot(t)
				if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionRevalidate ||
					!browserWork.Browser.WaitForApp ||
					browserWork.Browser.WaitForVite {
					t.Fatalf(
						"markdown write browser decision = %#v, want revalidate waiting for app only",
						browserWork.Browser,
					)
				}
			},
		},
		{
			Name:     "route_registry_write_runs_fast_reload_without_implicit_build",
			FilePath: paths.RouteRegistryPath,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				buildWorkSnapshots := capture.BuildWorkSnapshots()
				if len(buildWorkSnapshots) != 0 {
					t.Fatalf(
						"route registry write expected no build work, got %#v",
						buildWorkSnapshots,
					)
				}
				browserWork := assertSingleBrowserWorkSnapshot(t)
				if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
					!browserWork.Browser.WaitForApp ||
					!browserWork.Browser.WaitForVite {
					t.Fatalf(
						"route registry browser decision = %#v, want hard reload waiting for app+vite",
						browserWork.Browser,
					)
				}
			},
		},
		{
			Name:     "public_static_write_processes_changed_path_without_restart",
			FilePath: paths.PublicStaticPath,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				buildWork := assertSingleBuildWorkSnapshot(t)
				if !buildWork.Build.ProcessPublicFiles ||
					len(buildWork.Build.PublicStaticChangedFilePaths) != 1 ||
					buildWork.Build.PublicStaticChangedFilePaths[0] != paths.PublicStaticPath {
					t.Fatalf(
						"public static write expected one public changed path %q, got %#v",
						paths.PublicStaticPath,
						buildWork.Build,
					)
				}
				if buildWork.Build.ProcessPrivateFiles ||
					buildWork.Build.BuildCriticalCSS ||
					buildWork.Build.BuildNormalCSS ||
					buildWork.Build.CompileGo {
					t.Fatalf(
						"public static write expected no unrelated build work, got %#v",
						buildWork.Build,
					)
				}
				browserWork := assertSingleBrowserWorkSnapshot(t)
				if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionInvalidateVite {
					t.Fatalf(
						"public static write browser action = %v, want %v",
						browserWork.Browser.Action,
						eventpipeline.BrowserPhaseActionInvalidateVite,
					)
				}
			},
		},
		{
			Name:     "public_static_root_directory_event_is_noop",
			FilePath: cfg.Core.StaticAssetDirs.Public,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				assertNoBuildOrBrowserWork(t)
			},
		},
		{
			Name:     "private_static_root_directory_event_is_noop",
			FilePath: cfg.Core.StaticAssetDirs.Private,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				assertNoBuildOrBrowserWork(t)
			},
		},
		{
			Name:     "go_source_write_requests_compile_and_hard_reload",
			FilePath: paths.GoSourcePath,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				buildWork := assertSingleBuildWorkSnapshot(t)
				if !buildWork.Build.CompileGo {
					t.Fatalf(
						"go source write expected CompileGo=true, got %#v",
						buildWork.Build,
					)
				}
				if !buildWork.Restart.RestartApp {
					t.Fatalf(
						"go source write expected RestartApp=true, got %#v",
						buildWork.Restart,
					)
				}
				browserWork := assertSingleBrowserWorkSnapshot(t)
				if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
					!browserWork.Browser.WaitForApp ||
					browserWork.Browser.WaitForVite {
					t.Fatalf(
						"go source write browser decision = %#v, want hard reload waiting for app only",
						browserWork.Browser,
					)
				}
			},
		},
		{
			Name:     "tailwind_css_write_is_noop",
			FilePath: paths.TailwindCSSPath,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				assertNoBuildOrBrowserWork(t)
			},
		},
		{
			Name:     "frontend_ts_write_is_noop",
			FilePath: paths.RandomFrontendTSPath,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				assertNoBuildOrBrowserWork(t)
			},
		},
		{
			Name:     "vorma_entry_write_is_noop",
			FilePath: paths.VormaFrontendEntryTSXPath,
			Op:       fsnotify.Write,
			Assert: func(t *testing.T) {
				t.Helper()
				assertNoBuildOrBrowserWork(t)
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			resetStateForCase()
			engine.ProcessEvents([]fsnotify.Event{
				{
					Name: testCase.FilePath,
					Op:   testCase.Op,
				},
			})
			testCase.Assert(t)
			assertNoPendingRestartRequestForRunloopTests(
				t,
				serverForTest.RestartIntents,
			)
		})
	}
}

func TestProcessEvents_SiteStyleMixedBatchPublicAndPrivateStaticUsesHardReloadPrecedence(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Dist.Root = cfg.Core.DistDir
	paths := configureSiteStyleFixtureForRunloopProcessTests(t, cfg, root)

	privateStaticPath := filepath.Join(
		cfg.Core.StaticAssetDirs.Private,
		"notes.txt",
	)
	if writeError := os.WriteFile(
		privateStaticPath,
		[]byte("private note"),
		0o644,
	); writeError != nil {
		t.Fatalf("write private static file: %v", writeError)
	}

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	capture := &runloopWorkCaptureForProcessTests{}
	engine := buildRunloopEngineWithWorkCaptureForProcessTests(
		serverForTest,
		capture,
	)

	engine.ProcessEvents(
		[]fsnotify.Event{
			{
				Name: paths.PublicStaticPath,
				Op:   fsnotify.Write,
			},
			{
				Name: privateStaticPath,
				Op:   fsnotify.Write,
			},
		},
	)

	buildWorkSnapshots := capture.BuildWorkSnapshots()
	if len(buildWorkSnapshots) != 1 {
		t.Fatalf(
			"expected one build work snapshot, got %#v",
			buildWorkSnapshots,
		)
	}
	buildWork := buildWorkSnapshots[0]
	if !buildWork.Build.ProcessPublicFiles ||
		len(buildWork.Build.PublicStaticChangedFilePaths) != 1 ||
		buildWork.Build.PublicStaticChangedFilePaths[0] != paths.PublicStaticPath {
		t.Fatalf(
			"expected one public changed path %q, got %#v",
			paths.PublicStaticPath,
			buildWork.Build,
		)
	}
	if !buildWork.Build.ProcessPrivateFiles ||
		len(buildWork.Build.PrivateStaticChangedFilePaths) != 1 ||
		buildWork.Build.PrivateStaticChangedFilePaths[0] != privateStaticPath {
		t.Fatalf(
			"expected one private changed path %q, got %#v",
			privateStaticPath,
			buildWork.Build,
		)
	}
	if buildWork.Build.BuildCriticalCSS ||
		buildWork.Build.BuildNormalCSS ||
		buildWork.Build.CompileGo {
		t.Fatalf(
			"expected no unrelated mixed static build work, got %#v",
			buildWork.Build,
		)
	}

	browserWorkSnapshots := capture.BrowserWorkSnapshots()
	if len(browserWorkSnapshots) != 1 {
		t.Fatalf(
			"expected one browser work snapshot, got %#v",
			browserWorkSnapshots,
		)
	}
	browserWork := browserWorkSnapshots[0]
	if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
		!browserWork.Browser.WaitForApp ||
		browserWork.Browser.WaitForVite {
		t.Fatalf(
			"expected mixed public+private static browser hard reload waiting for app only, got %#v",
			browserWork.Browser,
		)
	}

	assertNoPendingRestartRequestForRunloopTests(
		t,
		serverForTest.RestartIntents,
	)
}

func TestProcessEvents_SiteStyleMixedBatchRouteRegistryAndMarkdownUsesHardReload(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Dist.Root = cfg.Core.DistDir
	paths := configureSiteStyleFixtureForRunloopProcessTests(t, cfg, root)

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	capture := &runloopWorkCaptureForProcessTests{}
	engine := buildRunloopEngineWithWorkCaptureForProcessTests(
		serverForTest,
		capture,
	)

	engine.ProcessEvents(
		[]fsnotify.Event{
			{
				Name: paths.MarkdownPath,
				Op:   fsnotify.Write,
			},
			{
				Name: paths.RouteRegistryPath,
				Op:   fsnotify.Write,
			},
		},
	)

	buildWorkSnapshots := capture.BuildWorkSnapshots()
	if len(buildWorkSnapshots) != 1 {
		t.Fatalf(
			"expected one build work snapshot, got %#v",
			buildWorkSnapshots,
		)
	}
	buildWork := buildWorkSnapshots[0]
	if !buildWork.Build.ProcessPrivateFiles ||
		len(buildWork.Build.PrivateStaticChangedFilePaths) != 1 ||
		buildWork.Build.PrivateStaticChangedFilePaths[0] != paths.MarkdownPath {
		t.Fatalf(
			"expected one private changed path %q, got %#v",
			paths.MarkdownPath,
			buildWork.Build,
		)
	}
	if !buildWork.PreferRevalidate {
		t.Fatalf(
			"expected markdown event to request revalidate preference, got %#v",
			buildWork,
		)
	}

	browserWorkSnapshots := capture.BrowserWorkSnapshots()
	if len(browserWorkSnapshots) != 1 {
		t.Fatalf(
			"expected one browser work snapshot, got %#v",
			browserWorkSnapshots,
		)
	}
	browserWork := browserWorkSnapshots[0]
	if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHardReload {
		t.Fatalf(
			"expected mixed markdown+route-registry batch to choose hard reload, got %#v",
			browserWork.Browser,
		)
	}
	if !browserWork.Browser.WaitForApp {
		t.Fatalf(
			"expected mixed markdown+route-registry hard reload to wait for app, got %#v",
			browserWork.Browser,
		)
	}

	assertNoPendingRestartRequestForRunloopTests(
		t,
		serverForTest.RestartIntents,
	)
}

func TestProcessEvents_SiteStyleMixedBatchGoAndNormalCSSUsesHardReload(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Dist.Root = cfg.Core.DistDir
	paths := configureSiteStyleFixtureForRunloopProcessTests(t, cfg, root)

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	capture := &runloopWorkCaptureForProcessTests{}
	engine := buildRunloopEngineWithWorkCaptureForProcessTests(
		serverForTest,
		capture,
	)

	engine.ProcessEvents(
		[]fsnotify.Event{
			{
				Name: paths.NormalCSSEntryPath,
				Op:   fsnotify.Write,
			},
			{
				Name: paths.GoSourcePath,
				Op:   fsnotify.Write,
			},
		},
	)

	buildWorkSnapshots := capture.BuildWorkSnapshots()
	if len(buildWorkSnapshots) != 1 {
		t.Fatalf(
			"expected one build work snapshot, got %#v",
			buildWorkSnapshots,
		)
	}
	buildWork := buildWorkSnapshots[0]
	if !buildWork.Build.CompileGo ||
		!buildWork.Restart.RestartApp {
		t.Fatalf(
			"expected mixed go+css batch to compile and restart app, got %#v",
			buildWork,
		)
	}
	if buildWork.Build.BuildCriticalCSS ||
		!buildWork.Build.BuildNormalCSS {
		t.Fatalf(
			"expected mixed go+css batch to build normal css only, got %#v",
			buildWork.Build,
		)
	}
	if buildWork.Build.ProcessPublicFiles ||
		buildWork.Build.ProcessPrivateFiles {
		t.Fatalf(
			"expected mixed go+css batch to avoid static processing, got %#v",
			buildWork.Build,
		)
	}

	browserWorkSnapshots := capture.BrowserWorkSnapshots()
	if len(browserWorkSnapshots) != 1 {
		t.Fatalf(
			"expected one browser work snapshot, got %#v",
			browserWorkSnapshots,
		)
	}
	browserWork := browserWorkSnapshots[0]
	if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
		!browserWork.Browser.WaitForApp ||
		browserWork.Browser.WaitForVite {
		t.Fatalf(
			"expected mixed go+css batch to hard reload waiting for app only, got %#v",
			browserWork.Browser,
		)
	}

	assertNoPendingRestartRequestForRunloopTests(
		t,
		serverForTest.RestartIntents,
	)
}

func TestProcessEvents_MixedBatchRunOnChangeOnlyAndImplicitBuildEvent(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist.Root = cfg.Core.DistDir

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

	watchedRunOnChangePath := filepath.Join(root, "notes.txt")
	goSourcePath := filepath.Join(root, "backend", "handlers", "health.go")
	if mkdirError := os.MkdirAll(filepath.Dir(goSourcePath), 0o755); mkdirError != nil {
		t.Fatalf("create go source directory: %v", mkdirError)
	}
	if writeError := os.WriteFile(watchedRunOnChangePath, []byte("notes"), 0o644); writeError != nil {
		t.Fatalf("write run-on-change file: %v", writeError)
	}
	if writeError := os.WriteFile(
		goSourcePath,
		[]byte("package handlers\n\nfunc Health() string { return \"ok\" }\n"),
		0o644,
	); writeError != nil {
		t.Fatalf("write go source file: %v", writeError)
	}

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	capture := &runloopWorkCaptureForProcessTests{}
	engine := buildRunloopEngineWithWorkCaptureForProcessTests(
		serverForTest,
		capture,
	)

	engine.ProcessEvents(
		[]fsnotify.Event{
			{
				Name: watchedRunOnChangePath,
				Op:   fsnotify.Write,
			},
			{
				Name: goSourcePath,
				Op:   fsnotify.Write,
			},
		},
	)

	if atomic.LoadInt32(&callbackCount) != 1 {
		t.Fatalf(
			"expected run-on-change callback to execute once in mixed batch, got %d",
			atomic.LoadInt32(&callbackCount),
		)
	}

	buildWorkSnapshots := capture.BuildWorkSnapshots()
	if len(buildWorkSnapshots) != 1 {
		t.Fatalf(
			"expected one build work snapshot, got %#v",
			buildWorkSnapshots,
		)
	}
	buildWork := buildWorkSnapshots[0]
	if !buildWork.Build.CompileGo || !buildWork.Restart.RestartApp {
		t.Fatalf(
			"expected mixed batch to preserve implicit go build+restart, got %#v",
			buildWork,
		)
	}
	if buildWork.Build.ProcessPublicFiles ||
		buildWork.Build.ProcessPrivateFiles ||
		buildWork.Build.BuildCriticalCSS ||
		buildWork.Build.BuildNormalCSS {
		t.Fatalf(
			"expected mixed batch to avoid unrelated build work, got %#v",
			buildWork.Build,
		)
	}

	browserWorkSnapshots := capture.BrowserWorkSnapshots()
	if len(browserWorkSnapshots) != 1 {
		t.Fatalf(
			"expected one browser work snapshot, got %#v",
			browserWorkSnapshots,
		)
	}
	browserWork := browserWorkSnapshots[0]
	if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
		!browserWork.Browser.WaitForApp ||
		browserWork.Browser.WaitForVite {
		t.Fatalf(
			"expected mixed batch browser hard reload waiting for app only, got %#v",
			browserWork.Browser,
		)
	}

	assertNoPendingRestartRequestForRunloopTests(
		t,
		serverForTest.RestartIntents,
	)
}

func TestProcessEvents_SiteStyleTemplateMutationOpsUseFastReloadWithoutRestart(
	t *testing.T,
) {
	testCases := []struct {
		Name    string
		Op      fsnotify.Op
		Prepare func(
			t *testing.T,
			paths siteStylePathMatrixForRunloopProcessTests,
		)
	}{
		{
			Name: "create",
			Op:   fsnotify.Create,
		},
		{
			Name: "remove",
			Op:   fsnotify.Remove,
			Prepare: func(
				t *testing.T,
				paths siteStylePathMatrixForRunloopProcessTests,
			) {
				t.Helper()
				if removeError := os.Remove(paths.TemplatePath); removeError != nil {
					t.Fatalf(
						"failed removing template before event: %v",
						removeError,
					)
				}
			},
		},
		{
			Name: "rename",
			Op:   fsnotify.Rename,
			Prepare: func(
				t *testing.T,
				paths siteStylePathMatrixForRunloopProcessTests,
			) {
				t.Helper()
				renamedTemplatePath := paths.TemplatePath + ".renamed"
				if renameError := os.Rename(
					paths.TemplatePath,
					renamedTemplatePath,
				); renameError != nil {
					t.Fatalf(
						"failed renaming template before event: %v",
						renameError,
					)
				}
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			root := t.TempDir()
			cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
			cfg.Dist.Root = cfg.Core.DistDir
			paths := configureSiteStyleFixtureForRunloopProcessTests(
				t,
				cfg,
				root,
			)

			serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
			capture := &runloopWorkCaptureForProcessTests{}
			engine := buildRunloopEngineWithWorkCaptureForProcessTests(
				serverForTest,
				capture,
			)

			if testCase.Prepare != nil {
				testCase.Prepare(t, paths)
			}

			engine.ProcessEvents(
				[]fsnotify.Event{
					{
						Name: paths.TemplatePath,
						Op:   testCase.Op,
					},
				},
			)

			buildWorkSnapshots := capture.BuildWorkSnapshots()
			if len(buildWorkSnapshots) != 1 {
				t.Fatalf(
					"expected one build work snapshot, got %#v",
					buildWorkSnapshots,
				)
			}
			buildWork := buildWorkSnapshots[0]
			if !buildWork.Build.ProcessPrivateFiles ||
				len(buildWork.Build.PrivateStaticChangedFilePaths) != 1 ||
				buildWork.Build.PrivateStaticChangedFilePaths[0] != paths.TemplatePath {
				t.Fatalf(
					"template %s expected one private changed path %q, got %#v",
					testCase.Name,
					paths.TemplatePath,
					buildWork.Build,
				)
			}
			if buildWork.Build.ProcessPublicFiles ||
				buildWork.Build.BuildCriticalCSS ||
				buildWork.Build.BuildNormalCSS ||
				buildWork.Build.CompileGo {
				t.Fatalf(
					"template %s expected no unrelated build work, got %#v",
					testCase.Name,
					buildWork.Build,
				)
			}

			browserWorkSnapshots := capture.BrowserWorkSnapshots()
			if len(browserWorkSnapshots) != 1 {
				t.Fatalf(
					"expected one browser work snapshot, got %#v",
					browserWorkSnapshots,
				)
			}
			browserWork := browserWorkSnapshots[0]
			if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
				!browserWork.Browser.WaitForApp ||
				!browserWork.Browser.WaitForVite {
				t.Fatalf(
					"template %s browser decision = %#v, want hard reload waiting for app+vite",
					testCase.Name,
					browserWork.Browser,
				)
			}

			assertNoPendingRestartRequestForRunloopTests(
				t,
				serverForTest.RestartIntents,
			)
		})
	}
}

func TestProcessEvents_SiteStyleRouteRegistryMutationOpsUseFastReloadWithoutRestart(
	t *testing.T,
) {
	testCases := []struct {
		Name    string
		Op      fsnotify.Op
		Prepare func(
			t *testing.T,
			paths siteStylePathMatrixForRunloopProcessTests,
		)
	}{
		{
			Name: "create",
			Op:   fsnotify.Create,
		},
		{
			Name: "remove",
			Op:   fsnotify.Remove,
			Prepare: func(
				t *testing.T,
				paths siteStylePathMatrixForRunloopProcessTests,
			) {
				t.Helper()
				if removeError := os.Remove(paths.RouteRegistryPath); removeError != nil {
					t.Fatalf(
						"failed removing route registry before event: %v",
						removeError,
					)
				}
			},
		},
		{
			Name: "rename",
			Op:   fsnotify.Rename,
			Prepare: func(
				t *testing.T,
				paths siteStylePathMatrixForRunloopProcessTests,
			) {
				t.Helper()
				renamedRouteRegistryPath := paths.RouteRegistryPath + ".renamed"
				if renameError := os.Rename(
					paths.RouteRegistryPath,
					renamedRouteRegistryPath,
				); renameError != nil {
					t.Fatalf(
						"failed renaming route registry before event: %v",
						renameError,
					)
				}
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			root := t.TempDir()
			cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
			cfg.Dist.Root = cfg.Core.DistDir
			paths := configureSiteStyleFixtureForRunloopProcessTests(
				t,
				cfg,
				root,
			)

			serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
			capture := &runloopWorkCaptureForProcessTests{}
			engine := buildRunloopEngineWithWorkCaptureForProcessTests(
				serverForTest,
				capture,
			)

			if testCase.Prepare != nil {
				testCase.Prepare(t, paths)
			}

			engine.ProcessEvents(
				[]fsnotify.Event{
					{
						Name: paths.RouteRegistryPath,
						Op:   testCase.Op,
					},
				},
			)

			buildWorkSnapshots := capture.BuildWorkSnapshots()
			if len(buildWorkSnapshots) != 0 {
				t.Fatalf(
					"route registry %s expected no build work, got %#v",
					testCase.Name,
					buildWorkSnapshots,
				)
			}

			browserWorkSnapshots := capture.BrowserWorkSnapshots()
			if len(browserWorkSnapshots) != 1 {
				t.Fatalf(
					"expected one browser work snapshot, got %#v",
					browserWorkSnapshots,
				)
			}
			browserWork := browserWorkSnapshots[0]
			if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
				!browserWork.Browser.WaitForApp ||
				!browserWork.Browser.WaitForVite {
				t.Fatalf(
					"route registry %s browser decision = %#v, want hard reload waiting for app+vite",
					testCase.Name,
					browserWork.Browser,
				)
			}

			assertNoPendingRestartRequestForRunloopTests(
				t,
				serverForTest.RestartIntents,
			)
		})
	}
}

func TestProcessEvents_SiteStyleMarkdownMutationOpsUseRevalidateWithoutRestart(
	t *testing.T,
) {
	testCases := []struct {
		Name    string
		Op      fsnotify.Op
		Prepare func(
			t *testing.T,
			paths siteStylePathMatrixForRunloopProcessTests,
		)
	}{
		{
			Name: "create",
			Op:   fsnotify.Create,
		},
		{
			Name: "remove",
			Op:   fsnotify.Remove,
			Prepare: func(
				t *testing.T,
				paths siteStylePathMatrixForRunloopProcessTests,
			) {
				t.Helper()
				if removeError := os.Remove(paths.MarkdownPath); removeError != nil {
					t.Fatalf(
						"failed removing markdown file before event: %v",
						removeError,
					)
				}
			},
		},
		{
			Name: "rename",
			Op:   fsnotify.Rename,
			Prepare: func(
				t *testing.T,
				paths siteStylePathMatrixForRunloopProcessTests,
			) {
				t.Helper()
				renamedMarkdownPath := paths.MarkdownPath + ".renamed"
				if renameError := os.Rename(
					paths.MarkdownPath,
					renamedMarkdownPath,
				); renameError != nil {
					t.Fatalf(
						"failed renaming markdown file before event: %v",
						renameError,
					)
				}
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			root := t.TempDir()
			cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
			cfg.Dist.Root = cfg.Core.DistDir
			paths := configureSiteStyleFixtureForRunloopProcessTests(
				t,
				cfg,
				root,
			)

			serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
			capture := &runloopWorkCaptureForProcessTests{}
			engine := buildRunloopEngineWithWorkCaptureForProcessTests(
				serverForTest,
				capture,
			)

			if testCase.Prepare != nil {
				testCase.Prepare(t, paths)
			}

			engine.ProcessEvents(
				[]fsnotify.Event{
					{
						Name: paths.MarkdownPath,
						Op:   testCase.Op,
					},
				},
			)

			buildWorkSnapshots := capture.BuildWorkSnapshots()
			if len(buildWorkSnapshots) != 1 {
				t.Fatalf(
					"expected one build work snapshot, got %#v",
					buildWorkSnapshots,
				)
			}
			buildWork := buildWorkSnapshots[0]
			if !buildWork.Build.ProcessPrivateFiles ||
				len(buildWork.Build.PrivateStaticChangedFilePaths) != 1 ||
				buildWork.Build.PrivateStaticChangedFilePaths[0] != paths.MarkdownPath {
				t.Fatalf(
					"markdown %s expected one private changed path %q, got %#v",
					testCase.Name,
					paths.MarkdownPath,
					buildWork.Build,
				)
			}
			if !buildWork.PreferRevalidate {
				t.Fatalf(
					"markdown %s expected PreferRevalidate=true, got %#v",
					testCase.Name,
					buildWork,
				)
			}
			if buildWork.Build.ProcessPublicFiles ||
				buildWork.Build.BuildCriticalCSS ||
				buildWork.Build.BuildNormalCSS ||
				buildWork.Build.CompileGo {
				t.Fatalf(
					"markdown %s expected no unrelated build work, got %#v",
					testCase.Name,
					buildWork.Build,
				)
			}

			browserWorkSnapshots := capture.BrowserWorkSnapshots()
			if len(browserWorkSnapshots) != 1 {
				t.Fatalf(
					"expected one browser work snapshot, got %#v",
					browserWorkSnapshots,
				)
			}
			browserWork := browserWorkSnapshots[0]
			if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionRevalidate ||
				!browserWork.Browser.WaitForApp ||
				browserWork.Browser.WaitForVite {
				t.Fatalf(
					"markdown %s browser decision = %#v, want revalidate waiting for app only",
					testCase.Name,
					browserWork.Browser,
				)
			}

			assertNoPendingRestartRequestForRunloopTests(
				t,
				serverForTest.RestartIntents,
			)
		})
	}
}

func TestProcessEvents_MarkdownWithoutWatchRuleUsesPrivateStaticReload(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Dist.Root = cfg.Core.DistDir
	cfg.Core.ServerOnlyMode = false

	markdownPath := filepath.Join(
		cfg.Core.StaticAssetDirs.Private,
		"markdown",
		"blog",
		"post.md",
	)
	if mkdirError := os.MkdirAll(filepath.Dir(markdownPath), 0o755); mkdirError != nil {
		t.Fatalf("create markdown directory: %v", mkdirError)
	}
	if writeError := os.WriteFile(
		markdownPath,
		[]byte("# Post\n\ncontent"),
		0o644,
	); writeError != nil {
		t.Fatalf("write markdown file: %v", writeError)
	}

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	capture := &runloopWorkCaptureForProcessTests{}
	engine := buildRunloopEngineWithWorkCaptureForProcessTests(
		serverForTest,
		capture,
	)

	engine.ProcessEvents(
		[]fsnotify.Event{
			{
				Name: markdownPath,
				Op:   fsnotify.Write,
			},
		},
	)

	buildWorkSnapshots := capture.BuildWorkSnapshots()
	if len(buildWorkSnapshots) != 1 {
		t.Fatalf(
			"expected one build work snapshot, got %#v",
			buildWorkSnapshots,
		)
	}
	buildWork := buildWorkSnapshots[0]
	if !buildWork.Build.ProcessPrivateFiles ||
		len(buildWork.Build.PrivateStaticChangedFilePaths) != 1 ||
		buildWork.Build.PrivateStaticChangedFilePaths[0] != markdownPath {
		t.Fatalf(
			"expected one private changed path %q, got %#v",
			markdownPath,
			buildWork.Build,
		)
	}
	if buildWork.PreferRevalidate {
		t.Fatalf(
			"expected markdown without watched-file override to avoid revalidate, got %#v",
			buildWork,
		)
	}
	if buildWork.Build.ProcessPublicFiles ||
		buildWork.Build.BuildCriticalCSS ||
		buildWork.Build.BuildNormalCSS ||
		buildWork.Build.CompileGo {
		t.Fatalf(
			"expected no unrelated build work, got %#v",
			buildWork.Build,
		)
	}

	browserWorkSnapshots := capture.BrowserWorkSnapshots()
	if len(browserWorkSnapshots) != 1 {
		t.Fatalf(
			"expected one browser work snapshot, got %#v",
			browserWorkSnapshots,
		)
	}
	browserWork := browserWorkSnapshots[0]
	if browserWork.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
		!browserWork.Browser.WaitForApp ||
		browserWork.Browser.WaitForVite {
		t.Fatalf(
			"markdown without watched-file override browser decision = %#v, want hard reload waiting for app only",
			browserWork.Browser,
		)
	}

	assertNoPendingRestartRequestForRunloopTests(
		t,
		serverForTest.RestartIntents,
	)
}

func TestProcessEvents_SiteStylePlainStaticAssetMutationOpsUseExpectedWorkWithoutRestart(
	t *testing.T,
) {
	testCases := []struct {
		Name                     string
		ResolvePath              func(siteStylePathMatrixForRunloopProcessTests) string
		Op                       fsnotify.Op
		Prepare                  func(*testing.T, string)
		ExpectProcessPublicFiles bool
		ExpectProcessPrivate     bool
		ExpectBrowserAction      eventpipeline.BrowserPhaseAction
		ExpectWaitForApp         bool
		ExpectWaitForVite        bool
	}{
		{
			Name: "public_static_create",
			ResolvePath: func(
				paths siteStylePathMatrixForRunloopProcessTests,
			) string {
				return paths.PublicStaticPath
			},
			Op: fsnotify.Create,
			Prepare: func(t *testing.T, targetPath string) {
				t.Helper()
				if removeError := os.Remove(targetPath); removeError != nil {
					t.Fatalf(
						"remove public static file before create event: %v",
						removeError,
					)
				}
				if writeError := os.WriteFile(
					targetPath,
					[]byte("<svg><!--public-create--></svg>"),
					0o644,
				); writeError != nil {
					t.Fatalf(
						"rewrite public static file before create event: %v",
						writeError,
					)
				}
			},
			ExpectProcessPublicFiles: true,
			ExpectProcessPrivate:     false,
			ExpectBrowserAction:      eventpipeline.BrowserPhaseActionInvalidateVite,
			ExpectWaitForApp:         false,
			ExpectWaitForVite:        false,
		},
		{
			Name: "public_static_remove",
			ResolvePath: func(
				paths siteStylePathMatrixForRunloopProcessTests,
			) string {
				return paths.PublicStaticPath
			},
			Op: fsnotify.Remove,
			Prepare: func(t *testing.T, targetPath string) {
				t.Helper()
				if removeError := os.Remove(targetPath); removeError != nil {
					t.Fatalf(
						"remove public static file before remove event: %v",
						removeError,
					)
				}
			},
			ExpectProcessPublicFiles: true,
			ExpectProcessPrivate:     false,
			ExpectBrowserAction:      eventpipeline.BrowserPhaseActionInvalidateVite,
			ExpectWaitForApp:         false,
			ExpectWaitForVite:        false,
		},
		{
			Name: "public_static_rename",
			ResolvePath: func(
				paths siteStylePathMatrixForRunloopProcessTests,
			) string {
				return paths.PublicStaticPath
			},
			Op: fsnotify.Rename,
			Prepare: func(t *testing.T, targetPath string) {
				t.Helper()
				renamedPath := targetPath + ".renamed"
				if renameError := os.Rename(targetPath, renamedPath); renameError != nil {
					t.Fatalf(
						"rename public static file before rename event: %v",
						renameError,
					)
				}
			},
			ExpectProcessPublicFiles: true,
			ExpectProcessPrivate:     false,
			ExpectBrowserAction:      eventpipeline.BrowserPhaseActionInvalidateVite,
			ExpectWaitForApp:         false,
			ExpectWaitForVite:        false,
		},
		{
			Name: "private_static_create",
			ResolvePath: func(
				paths siteStylePathMatrixForRunloopProcessTests,
			) string {
				return paths.PrivateStaticPath
			},
			Op: fsnotify.Create,
			Prepare: func(t *testing.T, targetPath string) {
				t.Helper()
				if removeError := os.Remove(targetPath); removeError != nil {
					t.Fatalf(
						"remove private static file before create event: %v",
						removeError,
					)
				}
				if writeError := os.WriteFile(
					targetPath,
					[]byte("private-create"),
					0o644,
				); writeError != nil {
					t.Fatalf(
						"rewrite private static file before create event: %v",
						writeError,
					)
				}
			},
			ExpectProcessPublicFiles: false,
			ExpectProcessPrivate:     true,
			ExpectBrowserAction:      eventpipeline.BrowserPhaseActionHardReload,
			ExpectWaitForApp:         true,
			ExpectWaitForVite:        false,
		},
		{
			Name: "private_static_remove",
			ResolvePath: func(
				paths siteStylePathMatrixForRunloopProcessTests,
			) string {
				return paths.PrivateStaticPath
			},
			Op: fsnotify.Remove,
			Prepare: func(t *testing.T, targetPath string) {
				t.Helper()
				if removeError := os.Remove(targetPath); removeError != nil {
					t.Fatalf(
						"remove private static file before remove event: %v",
						removeError,
					)
				}
			},
			ExpectProcessPublicFiles: false,
			ExpectProcessPrivate:     true,
			ExpectBrowserAction:      eventpipeline.BrowserPhaseActionHardReload,
			ExpectWaitForApp:         true,
			ExpectWaitForVite:        false,
		},
		{
			Name: "private_static_rename",
			ResolvePath: func(
				paths siteStylePathMatrixForRunloopProcessTests,
			) string {
				return paths.PrivateStaticPath
			},
			Op: fsnotify.Rename,
			Prepare: func(t *testing.T, targetPath string) {
				t.Helper()
				renamedPath := targetPath + ".renamed"
				if renameError := os.Rename(targetPath, renamedPath); renameError != nil {
					t.Fatalf(
						"rename private static file before rename event: %v",
						renameError,
					)
				}
			},
			ExpectProcessPublicFiles: false,
			ExpectProcessPrivate:     true,
			ExpectBrowserAction:      eventpipeline.BrowserPhaseActionHardReload,
			ExpectWaitForApp:         true,
			ExpectWaitForVite:        false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			root := t.TempDir()
			cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
			cfg.Dist.Root = cfg.Core.DistDir
			paths := configureSiteStyleFixtureForRunloopProcessTests(
				t,
				cfg,
				root,
			)

			serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
			capture := &runloopWorkCaptureForProcessTests{}
			engine := buildRunloopEngineWithWorkCaptureForProcessTests(
				serverForTest,
				capture,
			)

			targetPath := testCase.ResolvePath(paths)
			if testCase.Prepare != nil {
				testCase.Prepare(t, targetPath)
			}

			engine.ProcessEvents(
				[]fsnotify.Event{
					{
						Name: targetPath,
						Op:   testCase.Op,
					},
				},
			)

			buildWorkSnapshots := capture.BuildWorkSnapshots()
			if len(buildWorkSnapshots) != 1 {
				t.Fatalf(
					"expected one build work snapshot, got %#v",
					buildWorkSnapshots,
				)
			}
			buildWork := buildWorkSnapshots[0]
			if buildWork.Build.ProcessPublicFiles != testCase.ExpectProcessPublicFiles ||
				buildWork.Build.ProcessPrivateFiles != testCase.ExpectProcessPrivate {
				t.Fatalf(
					"static mutation expected public=%v private=%v processing, got %#v",
					testCase.ExpectProcessPublicFiles,
					testCase.ExpectProcessPrivate,
					buildWork.Build,
				)
			}
			if buildWork.Build.BuildCriticalCSS ||
				buildWork.Build.BuildNormalCSS ||
				buildWork.Build.CompileGo {
				t.Fatalf(
					"static mutation expected no css/go build work, got %#v",
					buildWork.Build,
				)
			}
			if testCase.ExpectProcessPublicFiles {
				if len(buildWork.Build.PublicStaticChangedFilePaths) != 1 ||
					buildWork.Build.PublicStaticChangedFilePaths[0] != targetPath {
					t.Fatalf(
						"expected one public changed path %q, got %#v",
						targetPath,
						buildWork.Build.PublicStaticChangedFilePaths,
					)
				}
				if len(buildWork.Build.PrivateStaticChangedFilePaths) != 0 {
					t.Fatalf(
						"expected no private changed paths for public static mutation, got %#v",
						buildWork.Build.PrivateStaticChangedFilePaths,
					)
				}
			}
			if testCase.ExpectProcessPrivate {
				if len(buildWork.Build.PrivateStaticChangedFilePaths) != 1 ||
					buildWork.Build.PrivateStaticChangedFilePaths[0] != targetPath {
					t.Fatalf(
						"expected one private changed path %q, got %#v",
						targetPath,
						buildWork.Build.PrivateStaticChangedFilePaths,
					)
				}
				if len(buildWork.Build.PublicStaticChangedFilePaths) != 0 {
					t.Fatalf(
						"expected no public changed paths for private static mutation, got %#v",
						buildWork.Build.PublicStaticChangedFilePaths,
					)
				}
			}

			browserWorkSnapshots := capture.BrowserWorkSnapshots()
			if len(browserWorkSnapshots) != 1 {
				t.Fatalf(
					"expected one browser work snapshot, got %#v",
					browserWorkSnapshots,
				)
			}
			browserWork := browserWorkSnapshots[0]
			if browserWork.Browser.Action != testCase.ExpectBrowserAction ||
				browserWork.Browser.WaitForApp != testCase.ExpectWaitForApp ||
				browserWork.Browser.WaitForVite != testCase.ExpectWaitForVite {
				t.Fatalf(
					"static mutation browser decision=%#v, want action=%v waitApp=%v waitVite=%v",
					browserWork.Browser,
					testCase.ExpectBrowserAction,
					testCase.ExpectWaitForApp,
					testCase.ExpectWaitForVite,
				)
			}

			assertNoPendingRestartRequestForRunloopTests(
				t,
				serverForTest.RestartIntents,
			)
		})
	}
}

func TestProcessEvents_SiteStyleNoopFilesMutationOpsRemainNoop(t *testing.T) {
	targetCases := []struct {
		Name        string
		ResolvePath func(paths siteStylePathMatrixForRunloopProcessTests) string
	}{
		{
			Name: "tailwind_css",
			ResolvePath: func(paths siteStylePathMatrixForRunloopProcessTests) string {
				return paths.TailwindCSSPath
			},
		},
		{
			Name: "frontend_ts",
			ResolvePath: func(paths siteStylePathMatrixForRunloopProcessTests) string {
				return paths.RandomFrontendTSPath
			},
		},
		{
			Name: "vorma_entry_tsx",
			ResolvePath: func(paths siteStylePathMatrixForRunloopProcessTests) string {
				return paths.VormaFrontendEntryTSXPath
			},
		},
	}
	opCases := []struct {
		Name    string
		Op      fsnotify.Op
		Prepare func(t *testing.T, targetPath string)
	}{
		{
			Name: "create",
			Op:   fsnotify.Create,
		},
		{
			Name: "remove",
			Op:   fsnotify.Remove,
			Prepare: func(t *testing.T, targetPath string) {
				t.Helper()
				if removeError := os.Remove(targetPath); removeError != nil {
					t.Fatalf(
						"failed removing target path before event: %v",
						removeError,
					)
				}
			},
		},
		{
			Name: "rename",
			Op:   fsnotify.Rename,
			Prepare: func(t *testing.T, targetPath string) {
				t.Helper()
				renamedTargetPath := targetPath + ".renamed"
				if renameError := os.Rename(
					targetPath,
					renamedTargetPath,
				); renameError != nil {
					t.Fatalf(
						"failed renaming target path before event: %v",
						renameError,
					)
				}
			},
		},
	}

	for _, targetCase := range targetCases {
		targetCase := targetCase
		for _, opCase := range opCases {
			opCase := opCase
			t.Run(targetCase.Name+"_"+opCase.Name, func(t *testing.T) {
				root := t.TempDir()
				cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
				cfg.Dist.Root = cfg.Core.DistDir
				paths := configureSiteStyleFixtureForRunloopProcessTests(
					t,
					cfg,
					root,
				)
				targetPath := targetCase.ResolvePath(paths)

				serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
				capture := &runloopWorkCaptureForProcessTests{}
				engine := buildRunloopEngineWithWorkCaptureForProcessTests(
					serverForTest,
					capture,
				)

				if opCase.Prepare != nil {
					opCase.Prepare(t, targetPath)
				}

				engine.ProcessEvents(
					[]fsnotify.Event{
						{
							Name: targetPath,
							Op:   opCase.Op,
						},
					},
				)

				if buildWorkSnapshots := capture.BuildWorkSnapshots(); len(
					buildWorkSnapshots,
				) != 0 {
					t.Fatalf(
						"%s %s expected no build work, got %#v",
						targetCase.Name,
						opCase.Name,
						buildWorkSnapshots,
					)
				}
				if browserWorkSnapshots := capture.BrowserWorkSnapshots(); len(
					browserWorkSnapshots,
				) != 0 {
					t.Fatalf(
						"%s %s expected no browser work, got %#v",
						targetCase.Name,
						opCase.Name,
						browserWorkSnapshots,
					)
				}

				assertNoPendingRestartRequestForRunloopTests(
					t,
					serverForTest.RestartIntents,
				)
			})
		}
	}
}

func TestProcessEvents_SiteStyleRouteRegistryFallbackRequestsNoGoRestart(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Dist.Root = cfg.Core.DistDir
	paths := configureSiteStyleFixtureForRunloopProcessTests(t, cfg, root)
	cfg.Watch.Include[1].OnChangeHooks = []wave.OnChangeHook{
		{
			Timing: wave.OnChangeStrategyPost,
			Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
				return &wave.RefreshAction{
					TriggerRestart: true,
					RecompileGo:    false,
				}, nil
			},
		},
	}

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	capture := &runloopWorkCaptureForProcessTests{}
	engine := buildRunloopEngineWithWorkCaptureForProcessTests(
		serverForTest,
		capture,
	)

	engine.ProcessEvents(
		[]fsnotify.Event{
			{
				Name: paths.RouteRegistryPath,
				Op:   fsnotify.Write,
			},
		},
	)

	buildWorkSnapshots := capture.BuildWorkSnapshots()
	if len(buildWorkSnapshots) != 0 {
		t.Fatalf(
			"expected route-registry restart fallback to skip implicit build work, got %#v",
			buildWorkSnapshots,
		)
	}

	browserWorkSnapshots := capture.BrowserWorkSnapshots()
	if len(browserWorkSnapshots) != 0 {
		t.Fatalf(
			"expected no browser execution after route-registry restart fallback, got %#v",
			browserWorkSnapshots,
		)
	}

	pendingRestartRequest := waitForPendingRestartRequestForRunloopTests(
		t,
		serverForTest.RestartIntents,
		200*time.Millisecond,
	)
	if pendingRestartRequest.RecompileGo ||
		pendingRestartRequest.IsConfigRestart {
		t.Fatalf(
			"expected non-config no-go restart request, got %#v",
			pendingRestartRequest,
		)
	}
}

func TestProcessEvents_SiteStyleTemplateFallbackRequestsNoGoRestart(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
	cfg.Dist.Root = cfg.Core.DistDir
	paths := configureSiteStyleFixtureForRunloopProcessTests(t, cfg, root)
	cfg.Watch.Include[2].OnChangeHooks = []wave.OnChangeHook{
		{
			Timing: wave.OnChangeStrategyPost,
			Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
				return &wave.RefreshAction{
					TriggerRestart: true,
					RecompileGo:    false,
				}, nil
			},
		},
	}

	serverForTest := setupProcessEventsServerForRunloopTests(t, cfg)
	capture := &runloopWorkCaptureForProcessTests{}
	engine := buildRunloopEngineWithWorkCaptureForProcessTests(
		serverForTest,
		capture,
	)

	engine.ProcessEvents(
		[]fsnotify.Event{
			{
				Name: paths.TemplatePath,
				Op:   fsnotify.Write,
			},
		},
	)

	buildWorkSnapshots := capture.BuildWorkSnapshots()
	if len(buildWorkSnapshots) != 1 {
		t.Fatalf(
			"expected one build work snapshot before template restart fallback, got %#v",
			buildWorkSnapshots,
		)
	}
	templateBuildWork := buildWorkSnapshots[0]
	if !templateBuildWork.Build.ProcessPrivateFiles ||
		len(templateBuildWork.Build.PrivateStaticChangedFilePaths) != 1 ||
		templateBuildWork.Build.PrivateStaticChangedFilePaths[0] != paths.TemplatePath {
		t.Fatalf(
			"expected template fallback build work to include private changed path %q, got %#v",
			paths.TemplatePath,
			templateBuildWork.Build,
		)
	}
	if templateBuildWork.Build.ProcessPublicFiles ||
		templateBuildWork.Build.BuildCriticalCSS ||
		templateBuildWork.Build.BuildNormalCSS ||
		templateBuildWork.Build.CompileGo {
		t.Fatalf(
			"expected template fallback build work to avoid unrelated work, got %#v",
			templateBuildWork.Build,
		)
	}

	browserWorkSnapshots := capture.BrowserWorkSnapshots()
	if len(browserWorkSnapshots) != 0 {
		t.Fatalf(
			"expected no browser execution after template restart fallback, got %#v",
			browserWorkSnapshots,
		)
	}

	pendingRestartRequest := waitForPendingRestartRequestForRunloopTests(
		t,
		serverForTest.RestartIntents,
		200*time.Millisecond,
	)
	if pendingRestartRequest.RecompileGo ||
		pendingRestartRequest.IsConfigRestart {
		t.Fatalf(
			"expected non-config no-go restart request, got %#v",
			pendingRestartRequest,
		)
	}
}

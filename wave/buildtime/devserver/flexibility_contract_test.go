package devserver

import (
	"bytes"
	"fmt"
	"github.com/vormadev/vorma/internal/wavetest"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave/wavewatch"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/buildtime/builder"
	"github.com/vormadev/vorma/wave/buildtime/internal/broadcast"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch"
)

const (
	flexibilityContractViteHelperProcessEnv = "WAVE_TOOLING_FLEXIBILITY_CONTRACT_VITE_HELPER_PROCESS"
	flexibilityContractViteHelperArgsEnv    = "WAVE_TOOLING_FLEXIBILITY_CONTRACT_VITE_HELPER_ARGS_FILE"
	flexibilityContractViteHelperCwdEnv     = "WAVE_TOOLING_FLEXIBILITY_CONTRACT_VITE_HELPER_CWD_FILE"
)

// TestWaveToolingFlexibilityContractViteHelperProcess runs in a subprocess and
// records CWD/args so tests can assert user-provided Vite command options are
// honored.
func TestWaveToolingFlexibilityContractViteHelperProcess(t *testing.T) {
	if os.Getenv(flexibilityContractViteHelperProcessEnv) != "1" {
		return
	}

	if err := runWaveToolingFlexibilityContractViteHelperProcess(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func TestFlexibilityContract_WatchExcludeDirsExcludesDirectoryTree(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetWatchExcludeDirs(cfg, []string{"generated"})

	generatedDir := filepath.Join(root, "generated")
	generatedNestedDir := filepath.Join(generatedDir, "nested")
	allowedDir := filepath.Join(root, "app")
	if err := os.MkdirAll(generatedNestedDir, 0o755); err != nil {
		t.Fatalf("failed creating excluded test directory: %v", err)
	}
	if err := os.MkdirAll(allowedDir, 0o755); err != nil {
		t.Fatalf("failed creating allowed test directory: %v", err)
	}

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	if err := watcher.AddDirectoryRecursively(root); err != nil {
		t.Fatalf("AddDir returned error: %v", err)
	}

	if !watcher.IsIgnoredDirectory(generatedDir) {
		t.Fatalf(
			"expected %q to be ignored by Watch.Exclude.Dirs",
			generatedDir,
		)
	}
	if !watcher.IsIgnoredDirectory(generatedNestedDir) {
		t.Fatalf(
			"expected nested directory %q to be ignored by Watch.Exclude.Dirs",
			generatedNestedDir,
		)
	}

	if watcher.IsWatchingDir(generatedDir) {
		t.Fatalf(
			"did not expect excluded directory to be watched: %q",
			generatedDir,
		)
	}
	if watcher.IsWatchingDir(generatedNestedDir) {
		t.Fatalf(
			"did not expect nested excluded directory to be watched: %q",
			generatedNestedDir,
		)
	}

	if watcher.IsIgnoredDirectory(allowedDir) {
		t.Fatalf(
			"did not expect non-excluded directory to be Ignored: %q",
			allowedDir,
		)
	}
	if !watcher.IsWatchingDir(allowedDir) {
		t.Fatalf(
			"expected non-excluded directory to be watched: %q",
			allowedDir,
		)
	}
}

func TestFlexibilityContract_AbsoluteWatchIncludePatternMatches(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)

	absolutePattern := filepath.Join(root, "custom-watch", "**", "*.txt")
	wavetest.SetWatchInclude(cfg, []wavewatch.WatchedFile{
		{
			Pattern:         absolutePattern,
			RunOnChangeOnly: true,
		},
	})

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	absoluteWatchedFile := filepath.Join(
		root,
		"custom-watch",
		"nested",
		"notes.txt",
	)
	if err := os.MkdirAll(filepath.Dir(absoluteWatchedFile), 0o755); err != nil {
		t.Fatalf("failed creating watched file parent dir: %v", err)
	}
	if err := os.WriteFile(absoluteWatchedFile, []byte("notes"), 0o644); err != nil {
		t.Fatalf("failed writing watched file: %v", err)
	}

	matchedWatchedFile := watcher.FindWatchedFile(absoluteWatchedFile)
	if matchedWatchedFile == nil {
		t.Fatal("expected absolute Watch.Include pattern to match watched file")
	}
	if !matchedWatchedFile.RunOnChangeOnly {
		t.Fatalf(
			"expected matched watched file to preserve RunOnChangeOnly=true, got %#v",
			matchedWatchedFile,
		)
	}
}

func TestFlexibilityContract_ConfigMutationsTriggerConfigRestart(t *testing.T) {
	runConfigMutationAndPathShapeMatrix(
		t,
		func(
			t *testing.T,
			configMutationCaseForRun configMutationCase,
			pathShapeCaseForRun configEventPathShapeCase,
		) {
			cfg, _, configFilePath := setupConfigEventTestConfig(t)
			if configMutationCaseForRun.PrepareEvent != nil {
				configMutationCaseForRun.PrepareEvent(t, configFilePath)
			}

			s := setupProcessEventsServerForToolingTests(t, cfg, configFilePath)

			configEventPath := pathShapeCaseForRun.BuildPath(t, configFilePath)
			s.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
				Name: configEventPath,
				Op:   configMutationCaseForRun.Op,
			}})

			pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
				t,
				s,
				200*time.Millisecond,
			)
			if !pendingRestartRequest.IsConfigRestart ||
				!pendingRestartRequest.RecompileGo {
				t.Fatalf(
					"expected config restart with Go recompile on config %s/%s, got %#v",
					configMutationCaseForRun.Name,
					pathShapeCaseForRun.Name,
					pendingRestartRequest,
				)
			}
		},
	)
}

func TestFlexibilityContract_NoOpConfigWriteDoesNotRestartOrBroadcast(
	t *testing.T,
) {
	cfg, _, configFilePath := setupConfigEventTestConfig(t)
	wavetest.SetCoreServerOnlyMode(cfg, false)
	serverForTest := setupProcessEventsServerForToolingTests(
		t,
		cfg,
		configFilePath,
	)

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()
	serverForTest.RefreshManager = refreshManager

	configBytes, readError := os.ReadFile(configFilePath)
	if readError != nil {
		t.Fatalf("failed reading config file for no-op write: %v", readError)
	}
	if writeError := os.WriteFile(
		configFilePath,
		configBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("failed writing no-op config bytes: %v", writeError)
	}

	serverForTest.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: configFilePath,
		Op:   fsnotify.Write,
	}})

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
			serverForTest,
		)
		if hasPendingRestartRequest {
			t.Fatalf(
				"expected no restart request for no-op config write, got %#v",
				pendingRestartRequest,
			)
		}
		time.Sleep(10 * time.Millisecond)
	}

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"expected no broadcast payload for no-op config write, got %#v",
			receivedPayload,
		)
	}
}

func TestFlexibilityContract_NoOpConfigWriteDoesNotRestart(
	t *testing.T,
) {
	cfg, _, configFilePath := setupConfigEventTestConfig(t)
	wavetest.SetCoreServerOnlyMode(cfg, false)

	serverForTest := setupProcessEventsServerForToolingTests(
		t,
		cfg,
		configFilePath,
	)

	configBytes, readError := os.ReadFile(configFilePath)
	if readError != nil {
		t.Fatalf("failed reading config file for no-op write: %v", readError)
	}
	if writeError := os.WriteFile(
		configFilePath,
		configBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("failed writing no-op config bytes: %v", writeError)
	}

	serverForTest.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: configFilePath,
		Op:   fsnotify.Write,
	}})

	if pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
		serverForTest,
	); hasPendingRestartRequest {
		t.Fatalf(
			"expected no restart request for no-op config write, got %#v",
			pendingRestartRequest,
		)
	}
}

func TestFlexibilityContract_NoOpConfigWriteAcrossPathAliasesIsNoOp(
	t *testing.T,
) {
	for _, pathShapeCaseForRun := range configEventPathShapeCases() {
		pathShapeCaseForRun := pathShapeCaseForRun
		t.Run(pathShapeCaseForRun.Name, func(t *testing.T) {
			cfg, _, configFilePath := setupConfigEventTestConfig(t)
			wavetest.SetCoreServerOnlyMode(cfg, false)

			serverForTest := setupProcessEventsServerForToolingTests(
				t,
				cfg,
				configFilePath,
			)

			configBytes, readError := os.ReadFile(configFilePath)
			if readError != nil {
				t.Fatalf(
					"failed reading config file for no-op write: %v",
					readError,
				)
			}
			if writeError := os.WriteFile(
				configFilePath,
				configBytes,
				0o644,
			); writeError != nil {
				t.Fatalf("failed writing no-op config bytes: %v", writeError)
			}

			configEventPath := pathShapeCaseForRun.BuildPath(t, configFilePath)
			serverForTest.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
				Name: configEventPath,
				Op:   fsnotify.Write,
			}})

			if pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
				serverForTest,
			); hasPendingRestartRequest {
				t.Fatalf(
					"expected no restart request for no-op config write via %s alias, got %#v",
					pathShapeCaseForRun.Name,
					pendingRestartRequest,
				)
			}
		})
	}
}

func TestFlexibilityContract_NoOpConfigMutationEventsDoNotRestartOrBroadcast(
	t *testing.T,
) {
	configMutationEventCases := []struct {
		Name string
		Op   fsnotify.Op
	}{
		{Name: "write", Op: fsnotify.Write},
		{Name: "create", Op: fsnotify.Create},
		{Name: "rename", Op: fsnotify.Rename},
	}

	for _, configMutationEventCaseForRun := range configMutationEventCases {
		configMutationEventCaseForRun := configMutationEventCaseForRun
		t.Run(configMutationEventCaseForRun.Name, func(t *testing.T) {
			cfg, _, configFilePath := setupConfigEventTestConfig(t)
			wavetest.SetCoreServerOnlyMode(cfg, false)

			var logOutputBuffer bytes.Buffer
			serverForTest := setupProcessEventsServerForToolingTests(
				t,
				cfg,
				configFilePath,
			)
			serverForTest.Log = slog.New(
				slog.NewTextHandler(&logOutputBuffer, nil),
			)

			refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
				t,
			)
			defer cleanup()
			serverForTest.RefreshManager = refreshManager

			configBytes, readError := os.ReadFile(configFilePath)
			if readError != nil {
				t.Fatalf(
					"failed reading config file for no-op mutation event: %v",
					readError,
				)
			}
			if writeError := os.WriteFile(
				configFilePath,
				configBytes,
				0o644,
			); writeError != nil {
				t.Fatalf(
					"failed writing config file for no-op mutation event: %v",
					writeError,
				)
			}

			serverForTest.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
				Name: configFilePath,
				Op:   configMutationEventCaseForRun.Op,
			}})

			if pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
				serverForTest,
			); hasPendingRestartRequest {
				t.Fatalf(
					"expected no restart request for no-op config %s event, got %#v",
					configMutationEventCaseForRun.Name,
					pendingRestartRequest,
				)
			}

			connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			var receivedPayload broadcast.Payload
			if readError := connection.ReadJSON(&receivedPayload); readError == nil {
				t.Fatalf(
					"expected no broadcast payload for no-op config %s event, got %#v",
					configMutationEventCaseForRun.Name,
					receivedPayload,
				)
			}

		})
	}
}

func TestFlexibilityContract_NoOpConfigAtomicSaveEventSequenceDoesNotRestartOrBroadcast(
	t *testing.T,
) {
	cfg, _, configFilePath := setupConfigEventTestConfig(t)
	wavetest.SetCoreServerOnlyMode(cfg, false)

	var logOutputBuffer bytes.Buffer
	serverForTest := setupProcessEventsServerForToolingTests(
		t,
		cfg,
		configFilePath,
	)
	serverForTest.Log = slog.New(
		slog.NewTextHandler(&logOutputBuffer, nil),
	)

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()
	serverForTest.RefreshManager = refreshManager

	configBytes, readError := os.ReadFile(configFilePath)
	if readError != nil {
		t.Fatalf(
			"failed reading config file for no-op atomic save sequence: %v",
			readError,
		)
	}
	if writeError := os.WriteFile(
		configFilePath,
		configBytes,
		0o644,
	); writeError != nil {
		t.Fatalf(
			"failed writing config file for no-op atomic save sequence: %v",
			writeError,
		)
	}

	serverForTest.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: configFilePath,
			Op:   fsnotify.Rename,
		},
		{
			Name: configFilePath,
			Op:   fsnotify.Create,
		},
		{
			Name: configFilePath,
			Op:   fsnotify.Write,
		},
	})

	if pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
		serverForTest,
	); hasPendingRestartRequest {
		t.Fatalf(
			"expected no restart request for no-op config atomic-save sequence, got %#v",
			pendingRestartRequest,
		)
	}

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"expected no broadcast payload for no-op config atomic-save sequence, got %#v",
			receivedPayload,
		)
	}

}

func TestFlexibilityContract_NoOpConfigMutationsAcrossPathAliasesAndEventTypes(
	t *testing.T,
) {
	configMutationEventCases := []struct {
		Name string
		Op   fsnotify.Op
	}{
		{Name: "write", Op: fsnotify.Write},
		{Name: "create", Op: fsnotify.Create},
		{Name: "rename", Op: fsnotify.Rename},
	}

	for _, pathShapeCaseForRun := range configEventPathShapeCases() {
		pathShapeCaseForRun := pathShapeCaseForRun
		for _, configMutationEventCaseForRun := range configMutationEventCases {
			configMutationEventCaseForRun := configMutationEventCaseForRun
			t.Run(
				pathShapeCaseForRun.Name+"_"+configMutationEventCaseForRun.Name,
				func(t *testing.T) {
					cfg, _, configFilePath := setupConfigEventTestConfig(t)
					wavetest.SetCoreServerOnlyMode(cfg, false)

					var logOutputBuffer bytes.Buffer
					serverForTest := setupProcessEventsServerForToolingTests(
						t,
						cfg,
						configFilePath,
					)
					serverForTest.Log = slog.New(
						slog.NewTextHandler(&logOutputBuffer, nil),
					)

					refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
						t,
					)
					defer cleanup()
					serverForTest.RefreshManager = refreshManager

					configBytes, readError := os.ReadFile(configFilePath)
					if readError != nil {
						t.Fatalf(
							"failed reading config file for no-op mutation matrix: %v",
							readError,
						)
					}
					if writeError := os.WriteFile(
						configFilePath,
						configBytes,
						0o644,
					); writeError != nil {
						t.Fatalf(
							"failed writing no-op config bytes for mutation matrix: %v",
							writeError,
						)
					}

					serverForTest.BuildRunloopEngine().
						ProcessEvents([]fsnotify.Event{{
							Name: pathShapeCaseForRun.BuildPath(
								t,
								configFilePath,
							),
							Op: configMutationEventCaseForRun.Op,
						}})

					if pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
						serverForTest,
					); hasPendingRestartRequest {
						t.Fatalf(
							"expected no restart request for no-op config %s via %s alias, got %#v",
							configMutationEventCaseForRun.Name,
							pathShapeCaseForRun.Name,
							pendingRestartRequest,
						)
					}

					connection.SetReadDeadline(
						time.Now().Add(200 * time.Millisecond),
					)
					var receivedPayload broadcast.Payload
					if readError := connection.ReadJSON(&receivedPayload); readError == nil {
						t.Fatalf(
							"expected no broadcast payload for no-op config %s via %s alias, got %#v",
							configMutationEventCaseForRun.Name,
							pathShapeCaseForRun.Name,
							receivedPayload,
						)
					}

				},
			)
		}
	}
}

func TestFlexibilityContract_ConfigSemanticMutationsAcrossPathAliasesAndEventTypes(
	t *testing.T,
) {
	configMutationEventCases := []struct {
		Name string
		Op   fsnotify.Op
	}{
		{Name: "write", Op: fsnotify.Write},
		{Name: "create", Op: fsnotify.Create},
		{Name: "rename", Op: fsnotify.Rename},
	}

	for _, pathShapeCaseForRun := range configEventPathShapeCases() {
		pathShapeCaseForRun := pathShapeCaseForRun
		for _, configMutationEventCaseForRun := range configMutationEventCases {
			configMutationEventCaseForRun := configMutationEventCaseForRun
			t.Run(
				pathShapeCaseForRun.Name+"_"+configMutationEventCaseForRun.Name,
				func(t *testing.T) {
					cfg, _, configFilePath := setupConfigEventTestConfig(t)
					wavetest.SetCoreServerOnlyMode(cfg, false)

					serverForTest := setupProcessEventsServerForToolingTests(
						t,
						cfg,
						configFilePath,
					)

					refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
						t,
					)
					defer cleanup()
					serverForTest.RefreshManager = refreshManager

					writeSemanticallyChangedConfigForToolingTests(
						t,
						configFilePath,
					)

					serverForTest.BuildRunloopEngine().
						ProcessEvents([]fsnotify.Event{{
							Name: pathShapeCaseForRun.BuildPath(
								t,
								configFilePath,
							),
							Op: configMutationEventCaseForRun.Op,
						}})

					pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
						t,
						serverForTest,
						200*time.Millisecond,
					)
					if !pendingRestartRequest.IsConfigRestart ||
						!pendingRestartRequest.RecompileGo {
						t.Fatalf(
							"expected config restart with Go recompile for semantic config %s via %s alias, got %#v",
							configMutationEventCaseForRun.Name,
							pathShapeCaseForRun.Name,
							pendingRestartRequest,
						)
					}

					connection.SetReadDeadline(
						time.Now().Add(500 * time.Millisecond),
					)
					var receivedPayload broadcast.Payload
					if readError := connection.ReadJSON(&receivedPayload); readError != nil {
						t.Fatalf(
							"expected rebuilding broadcast for semantic config %s via %s alias: %v",
							configMutationEventCaseForRun.Name,
							pathShapeCaseForRun.Name,
							readError,
						)
					}
					if receivedPayload.ChangeType != broadcast.ChangeTypeRebuilding {
						t.Fatalf(
							"expected rebuilding payload for semantic config %s via %s alias, got %#v",
							configMutationEventCaseForRun.Name,
							pathShapeCaseForRun.Name,
							receivedPayload,
						)
					}
				},
			)
		}
	}
}

func TestFlexibilityContract_ConfigWriteChangeBroadcastsAndRestarts(
	t *testing.T,
) {
	cfg, _, configFilePath := setupConfigEventTestConfig(t)
	wavetest.SetCoreServerOnlyMode(cfg, false)
	serverForTest := setupProcessEventsServerForToolingTests(
		t,
		cfg,
		configFilePath,
	)

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()
	serverForTest.RefreshManager = refreshManager

	writeSemanticallyChangedConfigForToolingTests(t, configFilePath)

	serverForTest.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: configFilePath,
		Op:   fsnotify.Write,
	}})

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		serverForTest,
		200*time.Millisecond,
	)
	if !pendingRestartRequest.IsConfigRestart ||
		!pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected config restart with Go recompile after semantic config write, got %#v",
			pendingRestartRequest,
		)
	}

	connection.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf(
			"expected rebuilding broadcast for semantic config write: %v",
			readError,
		)
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeRebuilding {
		t.Fatalf("expected rebuilding payload, got %#v", receivedPayload)
	}
}

func TestFlexibilityContract_ViteDevBuildHonorsCmdDirAndConfigFile(
	t *testing.T,
) {
	t.Setenv(flexibilityContractViteHelperProcessEnv, "1")

	root := t.TempDir()
	argsFilePath := filepath.Join(root, "helper.args")
	cwdFilePath := filepath.Join(root, "helper.cwd")
	t.Setenv(flexibilityContractViteHelperArgsEnv, argsFilePath)
	t.Setenv(flexibilityContractViteHelperCwdEnv, cwdFilePath)

	commandWorkingDirectory := filepath.Join(root, "frontend")
	if err := os.MkdirAll(commandWorkingDirectory, 0o755); err != nil {
		t.Fatalf("failed creating Vite command directory: %v", err)
	}

	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)
	ensureViteConfigForToolingTests(t, cfg)
	wavetest.SetViteJSPackageManagerBaseCmd(cfg, flexibilityContractViteHelperBaseCommand(
		t,
	))
	wavetest.SetViteJSPackageManagerCmdDir(cfg, commandWorkingDirectory)
	wavetest.SetViteConfigFile(cfg, "./vite.custom.config.ts")

	builder := builder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	viteContext, err := builder.NewViteDevContext()
	if err != nil {
		t.Fatalf("NewViteDevContext returned error: %v", err)
	}
	if viteContext == nil {
		t.Fatal("expected non-nil Vite context")
	}
	defer viteContext.Cleanup()
	viteContext.Wait()

	helperCwd, err := readTrimmedFile(cwdFilePath)
	if err != nil {
		t.Fatalf("failed reading helper cwd: %v", err)
	}
	expectedHelperCwd := filepath.Clean(commandWorkingDirectory)
	if helperCwd != expectedHelperCwd {
		t.Fatalf(
			"expected Vite command to run from %q, got %q",
			expectedHelperCwd,
			helperCwd,
		)
	}

	helperArgs, err := readHelperArgs(argsFilePath)
	if err != nil {
		t.Fatalf("failed reading helper args: %v", err)
	}

	if len(helperArgs) < 1 {
		t.Fatalf(
			"expected helper args to include vite invocation, got %v",
			helperArgs,
		)
	}
	if helperArgs[0] != "vite" {
		t.Fatalf("expected vite invocation, got args %v", helperArgs)
	}

	configArgValue, ok := helperFlagValue(helperArgs, "--config")
	if !ok {
		t.Fatalf(
			"expected helper args to include --config flag, got %v",
			helperArgs,
		)
	}
	if configArgValue != "./vite.custom.config.ts" {
		t.Fatalf(
			"expected --config value %q, got %q",
			"./vite.custom.config.ts",
			configArgValue,
		)
	}
}

func runWaveToolingFlexibilityContractViteHelperProcess() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := os.WriteFile(os.Getenv(flexibilityContractViteHelperCwdEnv), []byte(cwd), 0o644); err != nil {
		return err
	}

	helperArgs, err := helperArgsAfterDoubleDash(os.Args)
	if err != nil {
		return err
	}

	if err := os.WriteFile(
		os.Getenv(flexibilityContractViteHelperArgsEnv),
		[]byte(strings.Join(helperArgs, "\n")),
		0o644,
	); err != nil {
		return err
	}

	outDir, hasOutDir := helperFlagValue(helperArgs, "--outDir")
	manifestName, hasManifestName := helperFlagValue(helperArgs, "--manifest")
	if hasOutDir {
		resolvedOutDir := outDir
		if !filepath.IsAbs(resolvedOutDir) {
			resolvedOutDir = filepath.Join(cwd, resolvedOutDir)
		}
		if err := os.MkdirAll(resolvedOutDir, 0o755); err != nil {
			return err
		}

		manifestFileName := "__temp_viteutil_manifest__.json"
		if hasManifestName && manifestName != "" {
			manifestFileName = manifestName
		}

		manifestPath := filepath.Join(resolvedOutDir, manifestFileName)
		if err := os.WriteFile(manifestPath, []byte("{}"), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func flexibilityContractViteHelperBaseCommand(t *testing.T) string {
	t.Helper()

	executablePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error: %v", err)
	}

	if strings.ContainsAny(executablePath, " \t\n") {
		t.Skip(
			"test binary path contains whitespace; strings.Fields parser cannot represent this safely",
		)
	}

	return executablePath + " -test.run=^TestWaveToolingFlexibilityContractViteHelperProcess$ --"
}

func helperArgsAfterDoubleDash(args []string) ([]string, error) {
	for i, arg := range args {
		if arg == "--" {
			return append([]string(nil), args[i+1:]...), nil
		}
	}
	return nil, fmt.Errorf("helper process missing -- argument separator")
}

func readTrimmedFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func readHelperArgs(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}

func helperFlagValue(args []string, flag string) (string, bool) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

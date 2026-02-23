package devserver

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"github.com/vormadev/vorma/wave/tooling/internal/broadcast"
)

func processEventsWithTimeoutForSiteRegressionAdditionalTests(
	t *testing.T,
	serverForTest *Server,
	events []fsnotify.Event,
	timeout time.Duration,
	contextMessage string,
) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		serverForTest.BuildRunloopEngine().ProcessEvents(events)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf(
			"%s: ProcessEvents did not return within %s",
			contextMessage,
			timeout,
		)
	}
}

func assertNoReloadPayloadsForSiteRegressionAdditionalTests(
	t *testing.T,
	connection *websocket.Conn,
	contextMessage string,
) {
	t.Helper()

	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		connection.SetReadDeadline(time.Now().Add(75 * time.Millisecond))
		var receivedPayload broadcast.Payload
		if readError := connection.ReadJSON(&receivedPayload); readError != nil {
			return
		}
		if receivedPayload.ChangeType == broadcast.ChangeTypeRebuilding {
			continue
		}
		t.Fatalf(
			"%s: expected no reload payload while waiting for build retry, got %#v",
			contextMessage,
			receivedPayload,
		)
	}
}

func TestSiteRegression_WaitingForBuildRetry_GoSyntaxErrorThenFixDoesNotBlock(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.WatchRoot = root
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	goMainPath := filepath.Join(root, "cmd", "app", "main.go")
	if mkdirError := os.MkdirAll(filepath.Dir(goMainPath), 0o755); mkdirError != nil {
		t.Fatalf("create go main directory: %v", mkdirError)
	}
	if writeError := os.WriteFile(
		goMainPath,
		[]byte("package main\n\nfunc main() {\n\tthis is not valid go syntax\n}\n"),
		0o644,
	); writeError != nil {
		t.Fatalf("write broken go main file: %v", writeError)
	}
	cfg.Core.MainAppEntry = goMainPath

	serverForTest := setupProcessEventsServerForToolingTests(t, cfg)
	t.Cleanup(func() {
		serverForTest.CleanupForRebuild()
	})

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()
	serverForTest.RefreshManager = refreshManager
	serverForTest.SetWaitingForBuildRetry(true)

	processEventsWithTimeoutForSiteRegressionAdditionalTests(
		t,
		serverForTest,
		[]fsnotify.Event{{
			Name: goMainPath,
			Op:   fsnotify.Write,
		}},
		4*time.Second,
		"go syntax error write while waiting for build retry",
	)
	assertNoReloadPayloadsForSiteRegressionAdditionalTests(
		t,
		connection,
		"go syntax error write while waiting for build retry",
	)
	assertNoRestartRequestForSiteRegressionHarness(
		t,
		serverForTest,
		"go syntax error write while waiting for build retry",
	)

	if writeError := os.WriteFile(
		goMainPath,
		[]byte("package main\n\nfunc main() {\n\tselect {}\n}\n"),
		0o644,
	); writeError != nil {
		t.Fatalf("write fixed go main file: %v", writeError)
	}

	processEventsWithTimeoutForSiteRegressionAdditionalTests(
		t,
		serverForTest,
		[]fsnotify.Event{{
			Name: goMainPath,
			Op:   fsnotify.Write,
		}},
		4*time.Second,
		"go syntax fix write while waiting for build retry",
	)
	assertNoReloadPayloadsForSiteRegressionAdditionalTests(
		t,
		connection,
		"go syntax fix write while waiting for build retry",
	)
	assertNoRestartRequestForSiteRegressionHarness(
		t,
		serverForTest,
		"go syntax fix write while waiting for build retry",
	)

	if _, statError := os.Stat(cfg.Dist.Binary()); statError != nil {
		t.Fatalf(
			"expected fixed go syntax event to compile binary while waiting for build retry, stat error: %v",
			statError,
		)
	}
}

func TestSiteRegression_WaitingForBuildRetry_ErrorClassEventsDoNotBlock(
	t *testing.T,
) {
	testCases := []struct {
		name                  string
		prepareAndBuildEvents func(
			t *testing.T,
			harness siteRegressionHarnessForToolingTests,
		) []fsnotify.Event
		expectRebuildingPayload bool
		expectConfigRestart     bool
	}{
		{
			name: "public static typo write",
			prepareAndBuildEvents: func(
				t *testing.T,
				harness siteRegressionHarnessForToolingTests,
			) []fsnotify.Event {
				t.Helper()
				if writeError := os.WriteFile(
					harness.Paths.PublicStaticPath,
					[]byte("<svg><!--updated while waiting--></svg>"),
					0o644,
				); writeError != nil {
					t.Fatalf("write public static file: %v", writeError)
				}
				return []fsnotify.Event{{
					Name: harness.Paths.PublicStaticPath,
					Op:   fsnotify.Write,
				}}
			},
			expectRebuildingPayload: true,
			expectConfigRestart:     false,
		},
		{
			name: "framework route registry typo write",
			prepareAndBuildEvents: func(
				t *testing.T,
				harness siteRegressionHarnessForToolingTests,
			) []fsnotify.Event {
				t.Helper()
				if writeError := os.WriteFile(
					harness.Paths.RouteRegistryPath,
					[]byte("export const routes = [{ path: '/typo' }];"),
					0o644,
				); writeError != nil {
					t.Fatalf("write route registry file: %v", writeError)
				}
				return []fsnotify.Event{{
					Name: harness.Paths.RouteRegistryPath,
					Op:   fsnotify.Write,
				}}
			},
			expectRebuildingPayload: false,
			expectConfigRestart:     false,
		},
		{
			name: "framework template typo write",
			prepareAndBuildEvents: func(
				t *testing.T,
				harness siteRegressionHarnessForToolingTests,
			) []fsnotify.Event {
				t.Helper()
				if writeError := os.WriteFile(
					harness.Paths.TemplatePath,
					[]byte("<!doctype html><html><body><main>updated</main>{{.VormaBodyScripts}}</body></html>"),
					0o644,
				); writeError != nil {
					t.Fatalf("write template file: %v", writeError)
				}
				return []fsnotify.Event{{
					Name: harness.Paths.TemplatePath,
					Op:   fsnotify.Write,
				}}
			},
			expectRebuildingPayload: false,
			expectConfigRestart:     false,
		},
		{
			name: "markdown typo write",
			prepareAndBuildEvents: func(
				t *testing.T,
				harness siteRegressionHarnessForToolingTests,
			) []fsnotify.Event {
				t.Helper()
				if writeError := os.WriteFile(
					harness.Paths.MarkdownPath,
					[]byte("# Post\n\nUpdated while waiting"),
					0o644,
				); writeError != nil {
					t.Fatalf("write markdown file: %v", writeError)
				}
				return []fsnotify.Event{{
					Name: harness.Paths.MarkdownPath,
					Op:   fsnotify.Write,
				}}
			},
			expectRebuildingPayload: false,
			expectConfigRestart:     false,
		},
		{
			name: "config semantic error fix write",
			prepareAndBuildEvents: func(
				t *testing.T,
				harness siteRegressionHarnessForToolingTests,
			) []fsnotify.Event {
				t.Helper()
				writeSemanticallyChangedConfigForToolingTests(
					t,
					harness.Paths.ConfigFilePath,
				)
				return []fsnotify.Event{{
					Name: harness.Paths.ConfigFilePath,
					Op:   fsnotify.Write,
				}}
			},
			expectRebuildingPayload: true,
			expectConfigRestart:     true,
		},
		{
			name: "config syntax error write",
			prepareAndBuildEvents: func(
				t *testing.T,
				harness siteRegressionHarnessForToolingTests,
			) []fsnotify.Event {
				t.Helper()
				if writeError := os.WriteFile(
					harness.Paths.ConfigFilePath,
					[]byte("{ invalid json payload"),
					0o644,
				); writeError != nil {
					t.Fatalf("write invalid config payload: %v", writeError)
				}
				return []fsnotify.Event{{
					Name: harness.Paths.ConfigFilePath,
					Op:   fsnotify.Write,
				}}
			},
			expectRebuildingPayload: true,
			expectConfigRestart:     true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			harness := setupSiteRegressionHarnessWithoutAppHealthForToolingTests(
				t,
				false,
			)
			defer harness.Cleanup()
			harness.Server.SetWaitingForBuildRetry(true)

			events := testCase.prepareAndBuildEvents(t, harness)
			processEventsWithTimeoutForSiteRegressionAdditionalTests(
				t,
				harness.Server,
				events,
				1500*time.Millisecond,
				testCase.name,
			)

			if testCase.expectRebuildingPayload {
				assertExpectedBroadcastPayloadForSiteRegressionHarness(
					t,
					harness.Connection,
					broadcast.ChangeTypeRebuilding,
					testCase.name,
				)
				assertNoBroadcastPayloadForSiteRegressionHarness(
					t,
					harness.Connection,
					testCase.name,
				)
			} else {
				assertNoBroadcastPayloadForSiteRegressionHarness(
					t,
					harness.Connection,
					testCase.name,
				)
			}

			if testCase.expectConfigRestart {
				restartRequest := waitForPendingRestartRequestForToolingTests(
					t,
					harness.Server,
					500*time.Millisecond,
				)
				if !restartRequest.IsConfigRestart || !restartRequest.RecompileGo {
					t.Fatalf(
						"%s: expected config restart request while waiting for build retry, got %#v",
						testCase.name,
						restartRequest,
					)
				}
			} else {
				assertNoRestartRequestForSiteRegressionHarness(
					t,
					harness.Server,
					testCase.name,
				)
			}
		})
	}
}

func TestSiteRegression_FrameworkTemplateCreateTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if removeError := os.Remove(harness.Paths.TemplatePath); removeError != nil {
		t.Fatalf("remove template file before create event: %v", removeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.TemplatePath,
		[]byte("<!doctype html><html><body><main>created</main>{{.VormaBodyScripts}}</body></html>"),
		0o644,
	); writeError != nil {
		t.Fatalf("create template file before event: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.TemplatePath,
		Op:   fsnotify.Create,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"framework template create",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"framework template create",
	)
}

func TestSiteRegression_FrontendVormaEntryWriteIsNoOpWhenUnwatched(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	frontendVormaEntryPath := filepath.Join(
		harness.Paths.AppRoot,
		"frontend",
		"src",
		"vorma.entry.tsx",
	)
	if writeError := os.WriteFile(
		frontendVormaEntryPath,
		[]byte("export const VormaEntry = () => null;\n"),
		0o644,
	); writeError != nil {
		t.Fatalf("write frontend vorma entry file before event: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: frontendVormaEntryPath,
		Op:   fsnotify.Write,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"frontend vorma entry write",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"frontend vorma entry write",
	)
}

func TestSiteRegression_MixedBatchSemanticConfigAndRouteRegistryUsesConfigRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	writeSemanticallyChangedConfigForToolingTests(
		t,
		harness.Paths.ConfigFilePath,
	)
	if writeError := os.WriteFile(
		harness.Paths.RouteRegistryPath,
		[]byte("export const routes = [{ path: '/updated' }];"),
		0o644,
	); writeError != nil {
		t.Fatalf("write route registry file before mixed config batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: harness.Paths.ConfigFilePath,
			Op:   fsnotify.Write,
		},
		{
			Name: harness.Paths.RouteRegistryPath,
			Op:   fsnotify.Write,
		},
	})

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		harness.Server,
		500*time.Millisecond,
	)
	if !pendingRestartRequest.IsConfigRestart || !pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected mixed semantic config batch to queue config restart, got %#v",
			pendingRestartRequest,
		)
	}

	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRebuilding,
		"mixed semantic config + route-registry batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed semantic config + route-registry batch",
	)
}

func TestSiteRegression_MixedBatchNoOpConfigAndRouteRegistryProcessesRouteOnly(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, true)
	defer harness.Cleanup()

	configFileBytes, readError := os.ReadFile(harness.Paths.ConfigFilePath)
	if readError != nil {
		t.Fatalf("read config for no-op mixed batch: %v", readError)
	}
	if writeError := os.WriteFile(
		harness.Paths.ConfigFilePath,
		configFileBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write unchanged config for no-op mixed batch: %v", writeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.RouteRegistryPath,
		[]byte("export const routes = [{ path: '/docs' }];"),
		0o644,
	); writeError != nil {
		t.Fatalf("write route registry for no-op mixed batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: filepath.Join(
				filepath.Dir(harness.Paths.ConfigFilePath),
				".",
				filepath.Base(harness.Paths.ConfigFilePath),
			),
			Op: fsnotify.Write,
		},
		{
			Name: harness.Paths.RouteRegistryPath,
			Op:   fsnotify.Write,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"mixed no-op config + route-registry batch",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"mixed no-op config + route-registry batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed no-op config + route-registry batch",
	)

	if harness.LogBuffer == nil || !strings.Contains(
		harness.LogBuffer.String(),
		"no changes to wave.config.json; skipping restart",
	) {
		t.Fatalf(
			"expected mixed no-op config batch to log no-op config message, got logs: %s",
			harness.LogBuffer.String(),
		)
	}
}

func TestSiteRegression_MixedBatchNoOpConfigAndPublicStaticProcessesStaticOnly(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, true)
	defer harness.Cleanup()

	configFileBytes, readError := os.ReadFile(harness.Paths.ConfigFilePath)
	if readError != nil {
		t.Fatalf("read config for no-op mixed static batch: %v", readError)
	}
	if writeError := os.WriteFile(
		harness.Paths.ConfigFilePath,
		configFileBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write unchanged config for no-op mixed static batch: %v", writeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.PublicStaticPath,
		[]byte("<svg><!--updated--></svg>"),
		0o644,
	); writeError != nil {
		t.Fatalf("write public static file for no-op mixed static batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: filepath.Join(
				filepath.Dir(harness.Paths.ConfigFilePath),
				".",
				filepath.Base(harness.Paths.ConfigFilePath),
			),
			Op: fsnotify.Write,
		},
		{
			Name: harness.Paths.PublicStaticPath,
			Op:   fsnotify.Write,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"mixed no-op config + public-static batch",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed no-op config + public-static batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed no-op config + public-static batch",
	)

	if harness.LogBuffer == nil || !strings.Contains(
		harness.LogBuffer.String(),
		"no changes to wave.config.json; skipping restart",
	) {
		t.Fatalf(
			"expected mixed no-op config + public-static batch to log no-op config message, got logs: %s",
			harness.LogBuffer.String(),
		)
	}
}

func TestSiteRegression_MixedBatchNoOpConfigAtomicSaveAndRouteRegistryProcessesRouteOnly(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, true)
	defer harness.Cleanup()

	configFileBytes, readError := os.ReadFile(harness.Paths.ConfigFilePath)
	if readError != nil {
		t.Fatalf("read config for no-op mixed atomic-save batch: %v", readError)
	}
	if writeError := os.WriteFile(
		harness.Paths.ConfigFilePath,
		configFileBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write unchanged config for no-op mixed atomic-save batch: %v", writeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.RouteRegistryPath,
		[]byte("export const routes = [{ path: '/atomic-noop' }];"),
		0o644,
	); writeError != nil {
		t.Fatalf("write route registry for no-op mixed atomic-save batch: %v", writeError)
	}

	configAliasPath := filepath.Join(
		filepath.Dir(harness.Paths.ConfigFilePath),
		".",
		filepath.Base(harness.Paths.ConfigFilePath),
	)
	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: configAliasPath,
			Op:   fsnotify.Remove,
		},
		{
			Name: configAliasPath,
			Op:   fsnotify.Create,
		},
		{
			Name: harness.Paths.RouteRegistryPath,
			Op:   fsnotify.Write,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"mixed no-op config atomic-save + route-registry batch",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"mixed no-op config atomic-save + route-registry batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed no-op config atomic-save + route-registry batch",
	)

	if harness.LogBuffer == nil || !strings.Contains(
		harness.LogBuffer.String(),
		"no changes to wave.config.json; skipping restart",
	) {
		t.Fatalf(
			"expected mixed no-op config atomic-save batch to log no-op config message, got logs: %s",
			harness.LogBuffer.String(),
		)
	}
}

func TestSiteRegression_MixedBatchSemanticConfigAtomicSaveAndRouteRegistryUsesConfigRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	writeSemanticallyChangedConfigForToolingTests(
		t,
		harness.Paths.ConfigFilePath,
	)
	if writeError := os.WriteFile(
		harness.Paths.RouteRegistryPath,
		[]byte("export const routes = [{ path: '/atomic-semantic' }];"),
		0o644,
	); writeError != nil {
		t.Fatalf("write route registry for semantic mixed atomic-save batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: harness.Paths.ConfigFilePath,
			Op:   fsnotify.Remove,
		},
		{
			Name: harness.Paths.ConfigFilePath,
			Op:   fsnotify.Create,
		},
		{
			Name: harness.Paths.RouteRegistryPath,
			Op:   fsnotify.Write,
		},
	})

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		harness.Server,
		500*time.Millisecond,
	)
	if !pendingRestartRequest.IsConfigRestart || !pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected mixed semantic config atomic-save batch to queue config restart, got %#v",
			pendingRestartRequest,
		)
	}

	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRebuilding,
		"mixed semantic config atomic-save + route-registry batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed semantic config atomic-save + route-registry batch",
	)
}

func TestSiteRegression_MixedBatchNoOpConfigAtomicSaveAndPublicStaticProcessesStaticOnly(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, true)
	defer harness.Cleanup()

	configFileBytes, readError := os.ReadFile(harness.Paths.ConfigFilePath)
	if readError != nil {
		t.Fatalf("read config for no-op mixed atomic-save static batch: %v", readError)
	}
	if writeError := os.WriteFile(
		harness.Paths.ConfigFilePath,
		configFileBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write unchanged config for no-op mixed atomic-save static batch: %v", writeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.PublicStaticPath,
		[]byte("<svg><!--atomic-static-noop--></svg>"),
		0o644,
	); writeError != nil {
		t.Fatalf("write public static file for no-op mixed atomic-save static batch: %v", writeError)
	}

	configAliasPath := filepath.Join(
		filepath.Dir(harness.Paths.ConfigFilePath),
		".",
		filepath.Base(harness.Paths.ConfigFilePath),
	)
	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: configAliasPath,
			Op:   fsnotify.Remove,
		},
		{
			Name: configAliasPath,
			Op:   fsnotify.Create,
		},
		{
			Name: harness.Paths.PublicStaticPath,
			Op:   fsnotify.Write,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"mixed no-op config atomic-save + public-static batch",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed no-op config atomic-save + public-static batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed no-op config atomic-save + public-static batch",
	)

	if harness.LogBuffer == nil || !strings.Contains(
		harness.LogBuffer.String(),
		"no changes to wave.config.json; skipping restart",
	) {
		t.Fatalf(
			"expected mixed no-op config atomic-save static batch to log no-op config message, got logs: %s",
			harness.LogBuffer.String(),
		)
	}
}

func TestSiteRegression_MixedBatchSemanticConfigAtomicSaveAndPublicStaticUsesConfigRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	writeSemanticallyChangedConfigForToolingTests(
		t,
		harness.Paths.ConfigFilePath,
	)
	if writeError := os.WriteFile(
		harness.Paths.PublicStaticPath,
		[]byte("<svg><!--atomic-static-semantic--></svg>"),
		0o644,
	); writeError != nil {
		t.Fatalf("write public static file for semantic mixed atomic-save static batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: harness.Paths.ConfigFilePath,
			Op:   fsnotify.Remove,
		},
		{
			Name: harness.Paths.ConfigFilePath,
			Op:   fsnotify.Create,
		},
		{
			Name: harness.Paths.PublicStaticPath,
			Op:   fsnotify.Write,
		},
	})

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		harness.Server,
		500*time.Millisecond,
	)
	if !pendingRestartRequest.IsConfigRestart || !pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected mixed semantic config atomic-save static batch to queue config restart, got %#v",
			pendingRestartRequest,
		)
	}

	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRebuilding,
		"mixed semantic config atomic-save + public-static batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed semantic config atomic-save + public-static batch",
	)
}

func TestSiteRegression_MixedBatchNoOpConfigAtomicSaveAndMarkdownWithOverrideProcessesRevalidate(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, true)
	defer harness.Cleanup()

	configFileBytes, readError := os.ReadFile(harness.Paths.ConfigFilePath)
	if readError != nil {
		t.Fatalf("read config for no-op mixed atomic-save markdown batch: %v", readError)
	}
	if writeError := os.WriteFile(
		harness.Paths.ConfigFilePath,
		configFileBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write unchanged config for no-op mixed atomic-save markdown batch: %v", writeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.MarkdownPath,
		[]byte("# Post\n\nAtomic Save No-Op"),
		0o644,
	); writeError != nil {
		t.Fatalf("write markdown file for no-op mixed atomic-save markdown batch: %v", writeError)
	}

	configAliasPath := filepath.Join(
		filepath.Dir(harness.Paths.ConfigFilePath),
		".",
		filepath.Base(harness.Paths.ConfigFilePath),
	)
	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: configAliasPath,
			Op:   fsnotify.Remove,
		},
		{
			Name: configAliasPath,
			Op:   fsnotify.Create,
		},
		{
			Name: harness.Paths.MarkdownPath,
			Op:   fsnotify.Write,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"mixed no-op config atomic-save + markdown-with-override batch",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRevalidate,
		"mixed no-op config atomic-save + markdown-with-override batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed no-op config atomic-save + markdown-with-override batch",
	)

	if harness.LogBuffer == nil || !strings.Contains(
		harness.LogBuffer.String(),
		"no changes to wave.config.json; skipping restart",
	) {
		t.Fatalf(
			"expected mixed no-op config atomic-save markdown batch to log no-op config message, got logs: %s",
			harness.LogBuffer.String(),
		)
	}
}

func TestSiteRegression_MixedBatchNoOpConfigAtomicSaveAndMarkdownWithoutOverrideProcessesHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessWithoutMarkdownIncludeForToolingTests(
		t,
		true,
	)
	defer harness.Cleanup()

	configFileBytes, readError := os.ReadFile(harness.Paths.ConfigFilePath)
	if readError != nil {
		t.Fatalf("read config for no-op mixed atomic-save markdown-no-override batch: %v", readError)
	}
	if writeError := os.WriteFile(
		harness.Paths.ConfigFilePath,
		configFileBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write unchanged config for no-op mixed atomic-save markdown-no-override batch: %v", writeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.MarkdownPath,
		[]byte("# Post\n\nAtomic Save No-Override"),
		0o644,
	); writeError != nil {
		t.Fatalf("write markdown file for no-op mixed atomic-save markdown-no-override batch: %v", writeError)
	}

	configAliasPath := filepath.Join(
		filepath.Dir(harness.Paths.ConfigFilePath),
		".",
		filepath.Base(harness.Paths.ConfigFilePath),
	)
	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: configAliasPath,
			Op:   fsnotify.Remove,
		},
		{
			Name: configAliasPath,
			Op:   fsnotify.Create,
		},
		{
			Name: harness.Paths.MarkdownPath,
			Op:   fsnotify.Write,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"mixed no-op config atomic-save + markdown-without-override batch",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed no-op config atomic-save + markdown-without-override batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed no-op config atomic-save + markdown-without-override batch",
	)

	if harness.LogBuffer == nil || !strings.Contains(
		harness.LogBuffer.String(),
		"no changes to wave.config.json; skipping restart",
	) {
		t.Fatalf(
			"expected mixed no-op config atomic-save markdown-no-override batch to log no-op config message, got logs: %s",
			harness.LogBuffer.String(),
		)
	}
}

func TestSiteRegression_MixedBatchSemanticConfigAtomicSaveAndMarkdownWithOverrideUsesConfigRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	writeSemanticallyChangedConfigForToolingTests(
		t,
		harness.Paths.ConfigFilePath,
	)
	if writeError := os.WriteFile(
		harness.Paths.MarkdownPath,
		[]byte("# Post\n\nAtomic Save Semantic"),
		0o644,
	); writeError != nil {
		t.Fatalf("write markdown file for semantic mixed atomic-save markdown batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: harness.Paths.ConfigFilePath,
			Op:   fsnotify.Remove,
		},
		{
			Name: harness.Paths.ConfigFilePath,
			Op:   fsnotify.Create,
		},
		{
			Name: harness.Paths.MarkdownPath,
			Op:   fsnotify.Write,
		},
	})

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		harness.Server,
		500*time.Millisecond,
	)
	if !pendingRestartRequest.IsConfigRestart || !pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected mixed semantic config atomic-save markdown batch to queue config restart, got %#v",
			pendingRestartRequest,
		)
	}

	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRebuilding,
		"mixed semantic config atomic-save + markdown-with-override batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed semantic config atomic-save + markdown-with-override batch",
	)
}

func TestSiteRegression_MixedBatchConfigChmodAndRouteRegistryProcessesRouteOnly(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.RouteRegistryPath,
		[]byte("export const routes = [{ path: '/chmod-route' }];"),
		0o644,
	); writeError != nil {
		t.Fatalf("write route registry for mixed config-chmod batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: harness.Paths.ConfigFilePath,
			Op:   fsnotify.Chmod,
		},
		{
			Name: harness.Paths.RouteRegistryPath,
			Op:   fsnotify.Write,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"mixed config chmod + route-registry batch",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"mixed config chmod + route-registry batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed config chmod + route-registry batch",
	)
}

func TestSiteRegression_MixedBatchPublicAndPrivateStaticTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.PublicStaticPath,
		[]byte("<svg><!--public-updated--></svg>"),
		0o644,
	); writeError != nil {
		t.Fatalf("write public static file for mixed static batch: %v", writeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.PrivateStaticPath,
		[]byte("private note updated"),
		0o644,
	); writeError != nil {
		t.Fatalf("write private static file for mixed static batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: harness.Paths.PublicStaticPath,
			Op:   fsnotify.Write,
		},
		{
			Name: harness.Paths.PrivateStaticPath,
			Op:   fsnotify.Write,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"mixed public+private static batch",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed public+private static batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed public+private static batch",
	)
}

func TestSiteRegression_MixedBatchPublicStaticAndRouteRegistryTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.PublicStaticPath,
		[]byte("<svg><!--updated--></svg>"),
		0o644,
	); writeError != nil {
		t.Fatalf("write public static file for mixed public+route batch: %v", writeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.RouteRegistryPath,
		[]byte("export const routes = [{ path: '/docs' }];"),
		0o644,
	); writeError != nil {
		t.Fatalf("write route registry file for mixed public+route batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: harness.Paths.PublicStaticPath,
			Op:   fsnotify.Write,
		},
		{
			Name: harness.Paths.RouteRegistryPath,
			Op:   fsnotify.Write,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"mixed public-static+route-registry batch",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed public-static+route-registry batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed public-static+route-registry batch",
	)
}

func TestSiteRegression_MixedBatchPublicStaticAndMarkdownTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.PublicStaticPath,
		[]byte("<svg><!--updated--></svg>"),
		0o644,
	); writeError != nil {
		t.Fatalf("write public static file for mixed public+markdown batch: %v", writeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.MarkdownPath,
		[]byte("# Post\n\nUpdated"),
		0o644,
	); writeError != nil {
		t.Fatalf("write markdown file for mixed public+markdown batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: harness.Paths.PublicStaticPath,
			Op:   fsnotify.Write,
		},
		{
			Name: harness.Paths.MarkdownPath,
			Op:   fsnotify.Write,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"mixed public-static+markdown batch",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed public-static+markdown batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed public-static+markdown batch",
	)
}

func TestSiteRegression_MixedBatchPublicStaticAndNormalCSSUsesHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.PublicStaticPath,
		[]byte("<svg><!--updated--></svg>"),
		0o644,
	); writeError != nil {
		t.Fatalf("write public static file for mixed public+normal-css batch: %v", writeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.NormalCSSEntryPath,
		[]byte("@import \"./fonts.css\";\nbody { color: black; }"),
		0o644,
	); writeError != nil {
		t.Fatalf("write normal css file for mixed public+normal-css batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: harness.Paths.PublicStaticPath,
			Op:   fsnotify.Write,
		},
		{
			Name: harness.Paths.NormalCSSEntryPath,
			Op:   fsnotify.Write,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"mixed public-static+normal-css batch",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed public-static+normal-css batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed public-static+normal-css batch",
	)
}

func TestSiteRegression_MixedBatchMarkdownAndRouteRegistryTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.MarkdownPath,
		[]byte("# Post\n\nUpdated"),
		0o644,
	); writeError != nil {
		t.Fatalf("write markdown file for mixed markdown+route batch: %v", writeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.RouteRegistryPath,
		[]byte("export const routes = [{ path: '/blog' }];"),
		0o644,
	); writeError != nil {
		t.Fatalf("write route registry file for mixed markdown+route batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: harness.Paths.MarkdownPath,
			Op:   fsnotify.Write,
		},
		{
			Name: harness.Paths.RouteRegistryPath,
			Op:   fsnotify.Write,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"mixed markdown+route-registry batch",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"mixed markdown+route-registry batch",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"mixed markdown+route-registry batch",
	)
}

func TestSiteRegression_CriticalCSSAtomicSaveSequenceHotReloadsWithoutRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if removeError := os.Remove(harness.Paths.CriticalCSSEntryPath); removeError != nil {
		t.Fatalf("remove critical css entry before atomic-save event batch: %v", removeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.CriticalCSSEntryPath,
		[]byte("@import \"./critical_import.css\";\n.critical_atomic_save { color: purple; }"),
		0o644,
	); writeError != nil {
		t.Fatalf("rewrite critical css entry before atomic-save event batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: harness.Paths.CriticalCSSEntryPath,
			Op:   fsnotify.Remove,
		},
		{
			Name: harness.Paths.CriticalCSSEntryPath,
			Op:   fsnotify.Create,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"critical css atomic-save batch",
	)

	payload := assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeCriticalCSS,
		"critical css atomic-save batch",
	)
	if strings.TrimSpace(payload.CriticalCSS) == "" {
		t.Fatalf("expected non-empty critical css base64 payload, got %#v", payload)
	}
	decodedCriticalCSSBytes, decodeError := base64.StdEncoding.DecodeString(
		payload.CriticalCSS,
	)
	if decodeError != nil {
		t.Fatalf("expected valid critical css base64 payload, got decode error: %v", decodeError)
	}
	decodedCriticalCSSString := string(decodedCriticalCSSBytes)
	if !strings.Contains(decodedCriticalCSSString, "critical_atomic_save") {
		t.Fatalf(
			"expected decoded critical css payload to include updated selector, got %q",
			decodedCriticalCSSString,
		)
	}

	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"critical css atomic-save batch",
	)
}

func TestSiteRegression_NormalCSSAtomicSaveSequenceHotReloadsWithoutRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if removeError := os.Remove(harness.Paths.NormalCSSEntryPath); removeError != nil {
		t.Fatalf("remove normal css entry before atomic-save event batch: %v", removeError)
	}
	if writeError := os.WriteFile(
		harness.Paths.NormalCSSEntryPath,
		[]byte("@import \"./fonts.css\";\n.normal_atomic_save { color: teal; }"),
		0o644,
	); writeError != nil {
		t.Fatalf("rewrite normal css entry before atomic-save event batch: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{
		{
			Name: harness.Paths.NormalCSSEntryPath,
			Op:   fsnotify.Remove,
		},
		{
			Name: harness.Paths.NormalCSSEntryPath,
			Op:   fsnotify.Create,
		},
	})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"normal css atomic-save batch",
	)

	payload := assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeNormalCSS,
		"normal css atomic-save batch",
	)
	if strings.TrimSpace(payload.NormalCSSURL) == "" {
		t.Fatalf("expected non-empty normal css url payload, got %#v", payload)
	}
	if strings.TrimSpace(payload.CriticalCSS) != "" {
		t.Fatalf(
			"expected normal css payload not to carry critical css content, got %#v",
			payload,
		)
	}

	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"normal css atomic-save batch",
	)
}

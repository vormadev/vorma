package devserver

import (
	"encoding/json"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveconfig"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/internal/testpath"
	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/wave/buildtime/builder"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/restartengine"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch"
)

func newDiscardLogger() *slog.Logger {
	return wavetest.NewDiscardLogger()
}

func newParsedConfigForToolingTestsAtRoot(
	tb testing.TB,
	root string,
) waveconfig.ParsedConfig {
	tb.Helper()
	return wavetest.NewParsedConfigAtRoot(tb, root)
}

func newParsedConfigForToolingTestsWithoutViteAtRoot(
	tb testing.TB,
	root string,
) waveconfig.ParsedConfig {
	tb.Helper()

	baseConfig := wavetest.NewParsedConfigAtRoot(tb, root)
	baseConfigJSON, marshalError := wavetest.MarshalParsedConfigToRawJSON(
		baseConfig,
	)
	if marshalError != nil {
		tb.Fatalf(
			"marshal base tooling test config for no-vite fixture: %v",
			marshalError,
		)
	}

	var configDocument map[string]any
	if unmarshalError := json.Unmarshal(baseConfigJSON, &configDocument); unmarshalError != nil {
		tb.Fatalf(
			"parse base tooling test config for no-vite fixture: %v",
			unmarshalError,
		)
	}
	delete(configDocument, "Vite")

	noViteConfigJSON, remarshalError := json.Marshal(configDocument)
	if remarshalError != nil {
		tb.Fatalf(
			"re-marshal no-vite tooling test config: %v",
			remarshalError,
		)
	}

	parsedConfig, parseError := waveconfig.ParseConfigJSONWithConfigPath(
		noViteConfigJSON,
		wavetest.MustCWDRelativePath(filepath.Join(root, "wave.config.json")),
	)
	if parseError != nil {
		tb.Fatalf("parse no-vite tooling test config: %v", parseError)
	}
	return parsedConfig
}

func ensureViteConfigForToolingTests(
	t *testing.T,
	cfg waveconfig.ParsedConfig,
) {
	wavetest.EnsureViteConfig(t, cfg)
}

func pathRelativeToCurrentWorkingDirectoryForToolingConfigJSON(
	t *testing.T,
	configuredPath string,
) string {
	return testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		configuredPath,
	)
}

func pathRelativeToConfigFileDirectoryForToolingConfigJSON(
	t *testing.T,
	configFilePath string,
	configuredPath string,
) string {
	return testpath.MustPathRelativeToConfiguredWorkingDirectory(
		t,
		filepath.Dir(configFilePath),
		configuredPath,
	)
}

type configEventPathShapeCase struct {
	Name      string
	BuildPath func(t *testing.T, configFilePath string) string
}

func configEventPathShapeCases() []configEventPathShapeCase {
	return []configEventPathShapeCase{
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

type configMutationCase struct {
	Name         string
	Op           fsnotify.Op
	PrepareEvent func(t *testing.T, configFilePath string)
}

func configMutationCases() []configMutationCase {
	return []configMutationCase{
		{
			Name: "write",
			Op:   fsnotify.Write,
			PrepareEvent: func(t *testing.T, configFilePath string) {
				t.Helper()
				writeSemanticallyChangedConfigForToolingTests(
					t,
					configFilePath,
				)
			},
		},
		{
			Name: "create",
			Op:   fsnotify.Create,
			PrepareEvent: func(t *testing.T, configFilePath string) {
				t.Helper()
				if removeError := os.Remove(configFilePath); removeError != nil {
					t.Fatalf(
						"failed removing config file before create Event: %v",
						removeError,
					)
				}
				writeSemanticallyChangedConfigForToolingTests(
					t,
					configFilePath,
				)
			},
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

func writeSemanticallyChangedConfigForToolingTests(
	t *testing.T,
	configFilePath string,
) {
	t.Helper()

	resolveRootBasePath := filepath.Dir(filepath.Dir(configFilePath))

	configBytes, readError := os.ReadFile(configFilePath)
	if os.IsNotExist(readError) {
		configPayload := map[string]any{
			"Core": map[string]any{
				"ProjectID": "test-project",
				"MainAppEntry": pathRelativeToConfigFileDirectoryForToolingConfigJSON(
					t,
					configFilePath,
					filepath.Join(resolveRootBasePath, "cmd", "app_changed"),
				),
				"StaticAssetDirs": map[string]any{
					"Public": pathRelativeToConfigFileDirectoryForToolingConfigJSON(
						t,
						configFilePath,
						filepath.Join(
							resolveRootBasePath,
							"static",
							waveartifacts.PublicDirname,
						),
					),
					"Private": pathRelativeToConfigFileDirectoryForToolingConfigJSON(
						t,
						configFilePath,
						filepath.Join(
							resolveRootBasePath,
							"static",
							waveartifacts.PrivateDirname,
						),
					),
				},
			},
		}
		updatedConfigBytes, marshalError := json.Marshal(configPayload)
		if marshalError != nil {
			t.Fatalf(
				"failed marshaling semantically changed config: %v",
				marshalError,
			)
		}
		if writeError := os.WriteFile(
			configFilePath,
			updatedConfigBytes,
			0o644,
		); writeError != nil {
			t.Fatalf(
				"failed writing semantically changed config: %v",
				writeError,
			)
		}
		return
	}
	if readError != nil {
		t.Fatalf(
			"failed reading config file for semantic mutation: %v",
			readError,
		)
	}

	var configPayload map[string]any
	if unmarshalError := json.Unmarshal(configBytes, &configPayload); unmarshalError != nil {
		t.Fatalf(
			"failed unmarshaling config file for semantic mutation: %v",
			unmarshalError,
		)
	}

	corePayload, hasCorePayload := configPayload["Core"].(map[string]any)
	if !hasCorePayload {
		corePayload = map[string]any{}
		configPayload["Core"] = corePayload
	}
	corePayload["MainAppEntry"] = "cmd/app_changed"

	updatedConfigBytes, marshalError := json.Marshal(configPayload)
	if marshalError != nil {
		t.Fatalf(
			"failed marshaling semantically changed config: %v",
			marshalError,
		)
	}
	if writeError := os.WriteFile(
		configFilePath,
		updatedConfigBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("failed writing semantically changed config: %v", writeError)
	}
}

func runConfigMutationAndPathShapeMatrix(
	t *testing.T,
	runCase func(
		t *testing.T,
		configMutationCaseForRun configMutationCase,
		pathShapeCaseForRun configEventPathShapeCase,
	),
) {
	t.Helper()

	for _, configMutationCaseForRun := range configMutationCases() {
		for _, pathShapeCaseForRun := range configEventPathShapeCases() {
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

func setupConfigEventTestConfig(
	t *testing.T,
) (waveconfig.ParsedConfig, string, string) {
	t.Helper()

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)
	configFilePath := filepath.Join(root, "backend", "wave.config.json")

	if mkdirError := os.MkdirAll(filepath.Dir(configFilePath), 0o755); mkdirError != nil {
		t.Fatalf("failed creating config file directory: %v", mkdirError)
	}
	configPayload := map[string]any{
		"Core": map[string]any{
			"ProjectID": "test-project",
			"MainAppEntry": pathRelativeToConfigFileDirectoryForToolingConfigJSON(
				t,
				configFilePath,
				cfg.Core().MainAppEntry(),
			),
			"ServerOnlyMode": false,
			"StaticAssetDirs": map[string]any{
				"Public": pathRelativeToConfigFileDirectoryForToolingConfigJSON(
					t,
					configFilePath,
					cfg.Core().StaticAssetDirsPublic(),
				),
				"Private": pathRelativeToConfigFileDirectoryForToolingConfigJSON(
					t,
					configFilePath,
					cfg.Core().StaticAssetDirsPrivate(),
				),
			},
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

	parsedConfig, parseError := waveconfig.ParseConfigFile(configFilePath)
	if parseError != nil {
		t.Fatalf("failed parsing config file payload: %v", parseError)
	}
	return parsedConfig, root, configFilePath
}

func setupWatcherAndBuilderForToolingTests(
	t *testing.T,
	cfg waveconfig.ParsedConfig,
) (*watch.Watcher, *builder.Builder) {
	t.Helper()

	watcher, watcherError := watch.NewWatcher(cfg, newDiscardLogger())
	if watcherError != nil {
		t.Fatalf("newWatcher returned error: %v", watcherError)
	}
	t.Cleanup(func() {
		_ = watcher.Close()
	})

	builderForTest := builder.NewBuilder(cfg, newDiscardLogger())
	t.Cleanup(func() {
		builderForTest.Close()
	})

	return watcher, builderForTest
}

func setupProcessEventsServerForToolingTests(
	t *testing.T,
	cfg waveconfig.ParsedConfig,
	configFilePath ...string,
) *Server {
	t.Helper()

	watcher, builderForTest := setupWatcherAndBuilderForToolingTests(t, cfg)
	resolvedConfigFilePath := ""
	if len(configFilePath) > 0 {
		resolvedConfigFilePath = configFilePath[0]
	}
	return &Server{
		Cfg:            cfg,
		ConfigFilePath: resolvedConfigFilePath,
		Log:            newDiscardLogger(),
		Watcher:        watcher,
		Builder:        builderForTest,
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}
}

func consumePendingRestartRequestForToolingTests(
	serverForTest *Server,
) (restartengine.RestartRequest, bool) {
	if serverForTest == nil {
		return restartengine.RestartRequest{}, false
	}
	return serverForTest.ConsumePendingRestartRequest()
}

func waitForPendingRestartRequestForToolingTests(
	t *testing.T,
	serverForTest *Server,
	timeout time.Duration,
) restartengine.RestartRequest {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		restartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
			serverForTest,
		)
		if hasPendingRestartRequest {
			return restartRequest
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for pending restart request after %s", timeout)
	return restartengine.RestartRequest{}
}

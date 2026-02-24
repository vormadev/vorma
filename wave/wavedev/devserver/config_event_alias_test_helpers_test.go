package devserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavebuild/builder"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/restartengine"
	"github.com/vormadev/vorma/wave/wavedev/internal/watch"
)

type staticAssetDirsForTests = struct {
	Private string `json:"Private"`
	Public  string `json:"Public"`
}

type cssEntryFilesForTests = struct {
	Critical    string `json:"Critical,omitempty"`
	NonCritical string `json:"NonCritical,omitempty"`
}

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newParsedConfigForToolingTestsAtRoot(root string) *wave.ParsedConfig {
	cfg := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
			StaticAssetDirs: staticAssetDirsForTests{
				Public:  filepath.Join(root, "static", "public"),
				Private: filepath.Join(root, "static", "private"),
			},
		},
		Watch: &wave.WatchConfig{
			WatchRoot: root,
		},
	}
	cfg.Dist.Root = cfg.Core.DistDir
	return cfg
}

func ensureViteConfigForToolingTests(
	t *testing.T,
	cfg *wave.ParsedConfig,
) {
	t.Helper()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Vite != nil {
		return
	}

	cfgValue := reflect.ValueOf(cfg).Elem()
	viteField := cfgValue.FieldByName("Vite")
	if !viteField.IsValid() || !viteField.CanSet() || !viteField.IsNil() {
		t.Fatal("expected settable nil Vite field")
	}
	viteField.Set(reflect.New(viteField.Type().Elem()))
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

	configBytes, readError := os.ReadFile(configFilePath)
	if os.IsNotExist(readError) {
		configPayload := map[string]any{
			"Core": map[string]any{
				"MainAppEntry": "cmd/app_changed",
				"DistDir": filepath.Join(
					filepath.Dir(filepath.Dir(configFilePath)),
					"dist",
				),
				"StaticAssetDirs": map[string]any{
					"Public": filepath.Join(
						filepath.Dir(filepath.Dir(configFilePath)),
						"static",
						"public",
					),
					"Private": filepath.Join(
						filepath.Dir(filepath.Dir(configFilePath)),
						"static",
						"private",
					),
				},
			},
			"Watch": map[string]any{
				"WatchRoot": filepath.Dir(filepath.Dir(configFilePath)),
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
) (*wave.ParsedConfig, string, string) {
	t.Helper()

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	configFilePath := filepath.Join(root, "backend", "wave.config.json")
	cfg.Core.ConfigLocation = configFilePath
	cfg.Dist.Root = cfg.Core.DistDir

	if mkdirError := os.MkdirAll(filepath.Dir(configFilePath), 0o755); mkdirError != nil {
		t.Fatalf("failed creating config file directory: %v", mkdirError)
	}
	configPayload := map[string]any{
		"Core": map[string]any{
			"MainAppEntry":   cfg.Core.MainAppEntry,
			"DistDir":        cfg.Core.DistDir,
			"ServerOnlyMode": false,
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

	parsedConfig, parseError := wave.ParseConfigFile(configFilePath)
	if parseError != nil {
		t.Fatalf("failed parsing config file payload: %v", parseError)
	}
	return parsedConfig, root, configFilePath
}

func setupWatcherAndBuilderForToolingTests(
	t *testing.T,
	cfg *wave.ParsedConfig,
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
	cfg *wave.ParsedConfig,
) *Server {
	t.Helper()

	watcher, builderForTest := setupWatcherAndBuilderForToolingTests(t, cfg)
	return &Server{
		Cfg:     cfg,
		Log:     newDiscardLogger(),
		Watcher: watcher,
		Builder: builderForTest,
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

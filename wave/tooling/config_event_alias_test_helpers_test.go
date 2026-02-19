package tooling

import (
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"github.com/vormadev/vorma/wave/tooling/devserver/devserverengine"
	"github.com/vormadev/vorma/wave/tooling/watch"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
)

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
				aliasDirectoryPath := filepath.Join(configDirectoryPath, "config_alias_link")
				if err := os.Symlink(configDirectoryPath, aliasDirectoryPath); err != nil {
					t.Fatalf("create config directory symlink alias: %v", err)
				}

				return filepath.Join(aliasDirectoryPath, filepath.Base(configFilePath))
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
				if err := os.Remove(configFilePath); err != nil {
					t.Fatalf("failed removing config file before remove Event: %v", err)
				}
			},
		},
		{
			Name: "rename",
			Op:   fsnotify.Rename,
			PrepareEvent: func(t *testing.T, configFilePath string) {
				t.Helper()
				if err := os.Rename(configFilePath, configFilePath+".renamed"); err != nil {
					t.Fatalf("failed renaming config file before rename Event: %v", err)
				}
			},
		},
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

			t.Run(configMutationCaseForRun.Name+"_"+pathShapeCaseForRun.Name, func(t *testing.T) {
				runCase(t, configMutationCaseForRun, pathShapeCaseForRun)
			})
		}
	}
}

func setupConfigEventTestConfig(t *testing.T) (*wave.ParsedConfig, string, string) {
	t.Helper()

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	configFilePath := filepath.Join(root, "backend", "wave.config.json")
	cfg.Core.ConfigLocation = configFilePath
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	if err := os.MkdirAll(filepath.Dir(configFilePath), 0o755); err != nil {
		t.Fatalf("failed creating config file directory: %v", err)
	}
	if err := os.WriteFile(configFilePath, []byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist"}}`), 0o644); err != nil {
		t.Fatalf("failed writing config file: %v", err)
	}

	return cfg, root, configFilePath
}

func setupWatcherAndBuilderForToolingTests(
	t *testing.T,
	cfg *wave.ParsedConfig,
) (*watch.Watcher, *toolingbuilder.Builder) {
	t.Helper()

	watcher, watcherError := watch.NewWatcher(cfg, newDiscardLogger())
	if watcherError != nil {
		t.Fatalf("newWatcher returned error: %v", watcherError)
	}
	t.Cleanup(func() {
		_ = watcher.Close()
	})

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	t.Cleanup(func() {
		builder.Close()
	})

	return watcher, builder
}

func setupProcessEventsServerForToolingTests(
	t *testing.T,
	cfg *wave.ParsedConfig,
) *devserver.Server {
	t.Helper()

	watcher, builder := setupWatcherAndBuilderForToolingTests(t, cfg)
	return &devserver.Server{
		Cfg:            cfg,
		Log:            newDiscardLogger(),
		Watcher:        watcher,
		Builder:        builder,
		RestartIntents: devserverengine.NewRestartIntentAccumulator(make(chan devserverengine.RestartRequest, 1)),
	}
}

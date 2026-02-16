package tooling

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
)

type configEventPathShapeCase struct {
	name      string
	buildPath func(t *testing.T, configFilePath string) string
}

func configEventPathShapeCases() []configEventPathShapeCase {
	return []configEventPathShapeCase{
		{
			name: "exact",
			buildPath: func(_ *testing.T, configFilePath string) string {
				return configFilePath
			},
		},
		{
			name: "dot_alias",
			buildPath: func(_ *testing.T, configFilePath string) string {
				return filepath.Join(
					filepath.Dir(configFilePath),
					".",
					filepath.Base(configFilePath),
				)
			},
		},
		{
			name: "sibling_relative_alias",
			buildPath: func(_ *testing.T, configFilePath string) string {
				return filepath.Join(
					filepath.Dir(configFilePath),
					"nested",
					"..",
					filepath.Base(configFilePath),
				)
			},
		},
		{
			name: "symlink_alias",
			buildPath: func(t *testing.T, configFilePath string) string {
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
	name         string
	op           fsnotify.Op
	prepareEvent func(t *testing.T, configFilePath string)
}

func configMutationCases() []configMutationCase {
	return []configMutationCase{
		{
			name: "write",
			op:   fsnotify.Write,
		},
		{
			name: "create",
			op:   fsnotify.Create,
		},
		{
			name: "remove",
			op:   fsnotify.Remove,
			prepareEvent: func(t *testing.T, configFilePath string) {
				t.Helper()
				if err := os.Remove(configFilePath); err != nil {
					t.Fatalf("failed removing config file before remove event: %v", err)
				}
			},
		},
		{
			name: "rename",
			op:   fsnotify.Rename,
			prepareEvent: func(t *testing.T, configFilePath string) {
				t.Helper()
				if err := os.Rename(configFilePath, configFilePath+".renamed"); err != nil {
					t.Fatalf("failed renaming config file before rename event: %v", err)
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

			t.Run(configMutationCaseForRun.name+"_"+pathShapeCaseForRun.name, func(t *testing.T) {
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
) (*watcher, *Builder) {
	t.Helper()

	watcher, watcherError := newWatcher(cfg, newDiscardLogger())
	if watcherError != nil {
		t.Fatalf("newWatcher returned error: %v", watcherError)
	}
	t.Cleanup(func() {
		_ = watcher.Close()
	})

	builder := NewBuilder(cfg, newDiscardLogger())
	t.Cleanup(func() {
		builder.Close()
	})

	return watcher, builder
}

func setupProcessEventsServerForToolingTests(
	t *testing.T,
	cfg *wave.ParsedConfig,
) *server {
	t.Helper()

	watcher, builder := setupWatcherAndBuilderForToolingTests(t, cfg)
	return &server{
		cfg:            cfg,
		log:            newDiscardLogger(),
		watcher:        watcher,
		builder:        builder,
		restartIntents: newRestartIntentAccumulator(make(chan restartRequest, 1)),
	}
}

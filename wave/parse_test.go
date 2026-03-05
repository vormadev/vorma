package wave

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/waveconfig"
)

func TestParseConfigJSONRequiresConfigPath(t *testing.T) {
	_, err := waveconfig.ParseConfigJSON(
		[]byte(`{"Core":{"ProjectID":"test-project","MainAppEntry":"cmd/app"}}`),
	)
	if err == nil {
		t.Fatal("expected ParseConfigJSON to fail without config path")
	}
	if !strings.Contains(err.Error(), "ParseConfigJSONWithConfigPath") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseConfigRejectsInvalidJSON(t *testing.T) {
	_, err := waveconfig.ParseConfigJSONWithConfigPath(
		[]byte("{"),
		"backend/wave.config.json",
	)
	if err == nil {
		t.Fatal("expected parse failure for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseConfigRequiresCoreSection(t *testing.T) {
	_, err := waveconfig.ParseConfigJSONWithConfigPath(
		[]byte(`{"Vite":{"DefaultPort":5173}}`),
		"backend/wave.config.json",
	)
	if err == nil {
		t.Fatal("expected parse failure when Core section is missing")
	}
	if !strings.Contains(err.Error(), "Core section is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseConfigRequiresProjectID(t *testing.T) {
	_, err := waveconfig.ParseConfigJSONWithConfigPath(
		[]byte(`{"Core":{"MainAppEntry":"cmd/app"}}`),
		"backend/wave.config.json",
	)
	if err == nil {
		t.Fatal("expected parse failure when Core.ProjectID is missing")
	}
	if !strings.Contains(err.Error(), "Core.ProjectID is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseConfigRequiresMainAppEntry(t *testing.T) {
	_, err := waveconfig.ParseConfigJSONWithConfigPath(
		[]byte(`{"Core":{"ProjectID":"test-project"}}`),
		"backend/wave.config.json",
	)
	if err == nil {
		t.Fatal("expected parse failure when Core.MainAppEntry is missing")
	}
	if !strings.Contains(err.Error(), "Core.MainAppEntry is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseConfigResolvesPathsAndDerivesDistFromConfigSibling(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	cfg, err := waveconfig.ParseConfigJSONWithConfigPath(
		[]byte(`{
			"Core":{
				"ProjectID":"test-project",
				"ResolveRoot":"../",
				"MainAppEntry":"backend/cmd/app",
				"StaticAssetDirs":{
					"Public":"frontend/assets",
					"Private":"backend/assets/private"
				},
				"CSSEntryFiles":{
					"Critical":"frontend/src/styles/main.critical.css",
					"NonCritical":"frontend/src/styles/main.css"
				}
			},
			"Vite":{
				"JSPackageManagerCmdDir":"frontend",
				"ViteConfigFile":"frontend/vite.config.ts"
			},
			"Watch":{
				"Exclude":{
					"Dirs":["tmp/**"],
					"Files":["tmp/**/*.tmp"]
				},
				"Include":[
					{
						"Pattern":"backend/src/**/*.go",
						"OnChangeHooks":[{"Exclude":["backend/src/generated/**"]}]
					}
				]
			}
		}`),
		filepath.Join("backend", "wave.config.json"),
	)
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}

	if cfg.Core().ProjectID() != "test-project" {
		t.Fatalf("unexpected ProjectID: %q", cfg.Core().ProjectID())
	}

	expectedResolveRoot := filepath.Clean(".")
	if cfg.ResolveRoot() != expectedResolveRoot {
		t.Fatalf("expected ResolveRoot %q, got %q", expectedResolveRoot, cfg.ResolveRoot())
	}

	expectedDistRoot := filepath.Join("backend", ".wavedist")
	if cfg.Dist().Root() != expectedDistRoot {
		t.Fatalf("expected Dist.Root %q, got %q", expectedDistRoot, cfg.Dist().Root())
	}

	if cfg.Core().MainAppEntry() != filepath.Join("backend", "cmd", "app") {
		t.Fatalf("unexpected MainAppEntry: %q", cfg.Core().MainAppEntry())
	}
	if cfg.Core().StaticAssetDirsPublic() != filepath.Join("frontend", "assets") {
		t.Fatalf("unexpected StaticAssetDirs.Public: %q", cfg.Core().StaticAssetDirsPublic())
	}
	if cfg.Core().StaticAssetDirsPrivate() != filepath.Join("backend", "assets", "private") {
		t.Fatalf("unexpected StaticAssetDirs.Private: %q", cfg.Core().StaticAssetDirsPrivate())
	}
	if cfg.Core().CriticalCSSEntryFile() != filepath.Join("frontend", "src", "styles", "main.critical.css") {
		t.Fatalf("unexpected CSSEntryFiles.Critical: %q", cfg.Core().CriticalCSSEntryFile())
	}
	if cfg.Core().NonCriticalCSSEntryFile() != filepath.Join("frontend", "src", "styles", "main.css") {
		t.Fatalf("unexpected CSSEntryFiles.NonCritical: %q", cfg.Core().NonCriticalCSSEntryFile())
	}
	if cfg.Vite().JSPackageManagerCmdDir() != filepath.Join("frontend") {
		t.Fatalf("unexpected Vite.JSPackageManagerCmdDir: %q", cfg.Vite().JSPackageManagerCmdDir())
	}
	if cfg.Vite().ViteConfigFile() != filepath.Join("frontend", "vite.config.ts") {
		t.Fatalf("unexpected Vite.ViteConfigFile: %q", cfg.Vite().ViteConfigFile())
	}
	if cfg.Watch().ExcludeDirs()[0] != filepath.Join("tmp", "**") {
		t.Fatalf("unexpected Watch.Exclude.Dirs[0]: %q", cfg.Watch().ExcludeDirs()[0])
	}
	if cfg.Watch().ExcludeFiles()[0] != filepath.Join("tmp", "**", "*.tmp") {
		t.Fatalf("unexpected Watch.Exclude.Files[0]: %q", cfg.Watch().ExcludeFiles()[0])
	}
	if cfg.Watch().Include()[0].Pattern != filepath.Join("backend", "src", "**", "*.go") {
		t.Fatalf("unexpected Watch.Include[0].Pattern: %q", cfg.Watch().Include()[0].Pattern)
	}
	if cfg.Watch().Include()[0].OnChangeHooks[0].Exclude[0] !=
		filepath.Join("backend", "src", "generated", "**") {
		t.Fatalf(
			"unexpected Watch.Include[0].OnChangeHooks[0].Exclude[0]: %q",
			cfg.Watch().Include()[0].OnChangeHooks[0].Exclude[0],
		)
	}
}

func TestParseConfigOmittedResolveRootDefaultsToConfigDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	cfg, err := waveconfig.ParseConfigJSONWithConfigPath(
		[]byte(`{
			"Core":{
				"ProjectID":"test-project",
				"MainAppEntry":"cmd/app"
			}
		}`),
		filepath.Join("backend", "wave.config.json"),
	)
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}

	expectedResolveRoot := filepath.Join("backend")
	if cfg.ResolveRoot() != expectedResolveRoot {
		t.Fatalf("expected ResolveRoot %q, got %q", expectedResolveRoot, cfg.ResolveRoot())
	}
	if cfg.Core().MainAppEntry() != filepath.Join("backend", "cmd", "app") {
		t.Fatalf("unexpected MainAppEntry: %q", cfg.Core().MainAppEntry())
	}

	expectedDistRoot := filepath.Join("backend", ".wavedist")
	if cfg.Dist().Root() != expectedDistRoot {
		t.Fatalf("expected Dist.Root %q, got %q", expectedDistRoot, cfg.Dist().Root())
	}
}

func TestParseConfigWithMachineAbsoluteConfigPathKeepsRelativePathBasis(
	t *testing.T,
) {
	root := t.TempDir()
	t.Chdir(root)

	configPathMachineAbsolute := filepath.Join(
		root,
		"backend",
		"wave.config.json",
	)
	cfg, err := waveconfig.ParseConfigJSONWithConfigPath(
		[]byte(`{
			"Core":{
				"ProjectID":"test-project",
				"MainAppEntry":"cmd/app"
			}
		}`),
		configPathMachineAbsolute,
	)
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}

	expectedResolveRoot := filepath.Join("backend")
	if cfg.ResolveRoot() != expectedResolveRoot {
		t.Fatalf("expected ResolveRoot %q, got %q", expectedResolveRoot, cfg.ResolveRoot())
	}
	if filepath.IsAbs(cfg.ResolveRoot()) {
		t.Fatalf("expected ResolveRoot to remain parser-relative, got absolute %q", cfg.ResolveRoot())
	}

	expectedDistRoot := filepath.Join("backend", ".wavedist")
	if cfg.Dist().Root() != expectedDistRoot {
		t.Fatalf("expected Dist.Root %q, got %q", expectedDistRoot, cfg.Dist().Root())
	}
	if filepath.IsAbs(cfg.Dist().Root()) {
		t.Fatalf("expected Dist.Root to remain parser-relative, got absolute %q", cfg.Dist().Root())
	}

	if cfg.Core().MainAppEntry() != filepath.Join("backend", "cmd", "app") {
		t.Fatalf("unexpected MainAppEntry: %q", cfg.Core().MainAppEntry())
	}
}

func TestParseConfigPanicsOnMachineAbsoluteFilesystemPaths(t *testing.T) {
	testCases := []struct {
		name              string
		rawConfigJSON     string
		expectedFieldPath string
	}{
		{
			name: "Core.ResolveRoot",
			rawConfigJSON: `{
				"Core":{
					"ProjectID":"test-project",
					"ResolveRoot":"/absolute/root",
					"MainAppEntry":"cmd/app"
				}
			}`,
			expectedFieldPath: "Core.ResolveRoot",
		},
		{
			name: "Core.MainAppEntry",
			rawConfigJSON: `{
				"Core":{
					"ProjectID":"test-project",
					"MainAppEntry":"/cmd/app"
				}
			}`,
			expectedFieldPath: "Core.MainAppEntry",
		},
		{
			name: "Core.StaticAssetDirs.Private",
			rawConfigJSON: `{
				"Core":{
					"ProjectID":"test-project",
					"MainAppEntry":"cmd/app",
					"StaticAssetDirs":{"Private":"/static/private","Public":"static/public"}
				}
			}`,
			expectedFieldPath: "Core.StaticAssetDirs.Private",
		},
		{
			name: "Core.StaticAssetDirs.Public",
			rawConfigJSON: `{
				"Core":{
					"ProjectID":"test-project",
					"MainAppEntry":"cmd/app",
					"StaticAssetDirs":{"Private":"static/private","Public":"/static/public"}
				}
			}`,
			expectedFieldPath: "Core.StaticAssetDirs.Public",
		},
		{
			name: "Core.CSSEntryFiles.Critical",
			rawConfigJSON: `{
				"Core":{
					"ProjectID":"test-project",
					"MainAppEntry":"cmd/app",
					"CSSEntryFiles":{"Critical":"/styles/critical.css"}
				}
			}`,
			expectedFieldPath: "Core.CSSEntryFiles.Critical",
		},
		{
			name: "Core.CSSEntryFiles.NonCritical",
			rawConfigJSON: `{
				"Core":{
					"ProjectID":"test-project",
					"MainAppEntry":"cmd/app",
					"CSSEntryFiles":{"NonCritical":"/styles/app.css"}
				}
			}`,
			expectedFieldPath: "Core.CSSEntryFiles.NonCritical",
		},
		{
			name: "Vite.JSPackageManagerCmdDir",
			rawConfigJSON: `{
				"Core":{
					"ProjectID":"test-project",
					"MainAppEntry":"cmd/app"
				},
				"Vite":{"JSPackageManagerCmdDir":"/frontend"}
			}`,
			expectedFieldPath: "Vite.JSPackageManagerCmdDir",
		},
		{
			name: "Vite.ViteConfigFile",
			rawConfigJSON: `{
				"Core":{
					"ProjectID":"test-project",
					"MainAppEntry":"cmd/app"
				},
				"Vite":{"ViteConfigFile":"/frontend/vite.config.ts"}
			}`,
			expectedFieldPath: "Vite.ViteConfigFile",
		},
		{
			name: "Watch.Exclude.Dirs[0]",
			rawConfigJSON: `{
				"Core":{
					"ProjectID":"test-project",
					"MainAppEntry":"cmd/app"
				},
				"Watch":{"Exclude":{"Dirs":["/tmp/**"]}}
			}`,
			expectedFieldPath: "Watch.Exclude.Dirs[0]",
		},
		{
			name: "Watch.Exclude.Files[0]",
			rawConfigJSON: `{
				"Core":{
					"ProjectID":"test-project",
					"MainAppEntry":"cmd/app"
				},
				"Watch":{"Exclude":{"Files":["/**/*.tmp"]}}
			}`,
			expectedFieldPath: "Watch.Exclude.Files[0]",
		},
		{
			name: "Watch.Include[0].Pattern",
			rawConfigJSON: `{
				"Core":{
					"ProjectID":"test-project",
					"MainAppEntry":"cmd/app"
				},
				"Watch":{"Include":[{"Pattern":"/backend/**/*.go"}]}
			}`,
			expectedFieldPath: "Watch.Include[0].Pattern",
		},
		{
			name: "Watch.Include[0].OnChangeHooks[0].Exclude[0]",
			rawConfigJSON: `{
				"Core":{
					"ProjectID":"test-project",
					"MainAppEntry":"cmd/app"
				},
				"Watch":{"Include":[{"Pattern":"backend/**/*.go","OnChangeHooks":[{"Exclude":["/backend/generated/**"]}]}]}
			}`,
			expectedFieldPath: "Watch.Include[0].OnChangeHooks[0].Exclude[0]",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			defer func() {
				recoveredPanic := recover()
				if recoveredPanic == nil {
					t.Fatalf("expected parse to panic for %s", testCase.expectedFieldPath)
				}
				panicMessage := fmt.Sprint(recoveredPanic)
				if !strings.Contains(panicMessage, testCase.expectedFieldPath) {
					t.Fatalf(
						"expected panic message to include %q, got %q",
						testCase.expectedFieldPath,
						panicMessage,
					)
				}
				if !strings.Contains(panicMessage, "must not be a machine-absolute filesystem path") {
					t.Fatalf("unexpected panic message: %q", panicMessage)
				}
			}()

			_, _ = waveconfig.ParseConfigJSONWithConfigPath(
				[]byte(testCase.rawConfigJSON),
				"backend/wave.config.json",
			)
		})
	}
}

func TestParseConfigAllowsSlashRootedURLPathFields(t *testing.T) {
	cfg, err := waveconfig.ParseConfigJSONWithConfigPath(
		[]byte(`{
			"Core":{
				"ProjectID":"test-project",
				"MainAppEntry":"cmd/app",
				"PublicPathPrefix":"/assets/"
			},
			"Watch":{"HealthcheckEndpoint":"/healthz"}
		}`),
		"backend/wave.config.json",
	)
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestParseConfigFileResolvesPathsRelativeToConfigDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	configPath := filepath.Join("backend", "wave.config.json")

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create config parent directory: %v", err)
	}
	if err := os.WriteFile(
		configPath,
		[]byte(`{
			"Core":{
				"ProjectID":"test-project",
				"MainAppEntry":"cmd/app"
			}
		}`),
		0o644,
	); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := waveconfig.ParseConfigFile(configPath)
	if err != nil {
		t.Fatalf("ParseConfigFile returned error: %v", err)
	}
	if cfg.Core().MainAppEntry() != filepath.Join("backend", "cmd", "app") {
		t.Fatalf("unexpected MainAppEntry: %q", cfg.Core().MainAppEntry())
	}
	if cfg.Dist().Root() != filepath.Join("backend", ".wavedist") {
		t.Fatalf("unexpected Dist.Root: %q", cfg.Dist().Root())
	}
}

func TestParseConfigFileRejectsEmptyPath(t *testing.T) {
	_, err := waveconfig.ParseConfigFile("   ")
	if err == nil {
		t.Fatal("expected ParseConfigFile to fail for empty path")
	}
	if !strings.Contains(err.Error(), "config file path is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

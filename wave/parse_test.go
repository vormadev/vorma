package wave

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/waveconfig"
)

func TestParseConfigRejectsInvalidJSON(t *testing.T) {
	_, err := waveconfig.ParseConfigJSON([]byte("{"))
	if err == nil {
		t.Fatal("expected parseConfig to fail for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("expected parse config error prefix, got %q", err)
	}
}

func TestParseConfigRequiresCoreSection(t *testing.T) {
	_, err := waveconfig.ParseConfigJSON([]byte(`{"Vite":{"DefaultPort":5173}}`))
	if err == nil {
		t.Fatal("expected parseConfig to fail when Core section is missing")
	}
	if !strings.Contains(err.Error(), "Core section is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseConfigSetsCleanDistRoot(t *testing.T) {
	raw := []byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"./dist/../dist/."}}`)

	cfg, err := waveconfig.ParseConfigJSON(raw)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	expectedDist := filepath.Clean("./dist/../dist/.")
	expectedAbsoluteDist, expectedAbsoluteDistError := filepath.Abs(expectedDist)
	if expectedAbsoluteDistError != nil {
		t.Fatalf("resolve absolute expected dist path: %v", expectedAbsoluteDistError)
	}
	if cfg.Dist.Root != expectedAbsoluteDist {
		t.Fatalf(
			"expected cleaned absolute Dist root %q, got %q",
			expectedAbsoluteDist,
			cfg.Dist.Root,
		)
	}
}

func TestParseConfigPanicsOnMachineAbsoluteFilesystemConfigPaths(t *testing.T) {
	testCases := []struct {
		name              string
		rawConfigJSON     string
		expectedFieldPath string
	}{
		{
			name: "Core.MainAppEntry",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"/cmd/app",
					"DistDir":"dist"
				}
			}`,
			expectedFieldPath: "Core.MainAppEntry",
		},
		{
			name: "Core.DistDir",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"cmd/app",
					"DistDir":"/dist"
				}
			}`,
			expectedFieldPath: "Core.DistDir",
		},
		{
			name: "Core.StaticAssetDirs.Private",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"cmd/app",
					"DistDir":"dist",
					"StaticAssetDirs":{"Private":"/static/private","Public":"static/public"}
				}
			}`,
			expectedFieldPath: "Core.StaticAssetDirs.Private",
		},
		{
			name: "Core.StaticAssetDirs.Public",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"cmd/app",
					"DistDir":"dist",
					"StaticAssetDirs":{"Private":"static/private","Public":"/static/public"}
				}
			}`,
			expectedFieldPath: "Core.StaticAssetDirs.Public",
		},
		{
			name: "Core.CSSEntryFiles.Critical",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"cmd/app",
					"DistDir":"dist",
					"CSSEntryFiles":{"Critical":"/styles/critical.css"}
				}
			}`,
			expectedFieldPath: "Core.CSSEntryFiles.Critical",
		},
		{
			name: "Core.CSSEntryFiles.NonCritical",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"cmd/app",
					"DistDir":"dist",
					"CSSEntryFiles":{"NonCritical":"/styles/app.css"}
				}
			}`,
			expectedFieldPath: "Core.CSSEntryFiles.NonCritical",
		},
		{
			name: "Vite.JSPackageManagerCmdDir",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"cmd/app",
					"DistDir":"dist"
				},
				"Vite":{"JSPackageManagerCmdDir":"/frontend"}
			}`,
			expectedFieldPath: "Vite.JSPackageManagerCmdDir",
		},
		{
			name: "Vite.ViteConfigFile",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"cmd/app",
					"DistDir":"dist"
				},
				"Vite":{"ViteConfigFile":"/frontend/vite.config.ts"}
			}`,
			expectedFieldPath: "Vite.ViteConfigFile",
		},
		{
			name: "Watch.WatchRoot",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"cmd/app",
					"DistDir":"dist"
				},
				"Watch":{"WatchRoot":"/workspace"}
			}`,
			expectedFieldPath: "Watch.WatchRoot",
		},
		{
			name: "Watch.Exclude.Dirs[0]",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"cmd/app",
					"DistDir":"dist"
				},
				"Watch":{"Exclude":{"Dirs":["/tmp/**"]}}
			}`,
			expectedFieldPath: "Watch.Exclude.Dirs[0]",
		},
		{
			name: "Watch.Exclude.Files[0]",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"cmd/app",
					"DistDir":"dist"
				},
				"Watch":{"Exclude":{"Files":["/**/*.tmp"]}}
			}`,
			expectedFieldPath: "Watch.Exclude.Files[0]",
		},
		{
			name: "Watch.Include[0].Pattern",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"cmd/app",
					"DistDir":"dist"
				},
				"Watch":{"Include":[{"Pattern":"/backend/**/*.go"}]}
			}`,
			expectedFieldPath: "Watch.Include[0].Pattern",
		},
		{
			name: "Watch.Include[0].OnChangeHooks[0].Exclude[0]",
			rawConfigJSON: `{
				"Core":{
					"MainAppEntry":"cmd/app",
					"DistDir":"dist"
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
					t.Fatalf("expected parseConfig to panic for %s", testCase.expectedFieldPath)
				}
				panicMessage := fmt.Sprint(recoveredPanic)
				if !strings.Contains(panicMessage, testCase.expectedFieldPath) {
					t.Fatalf(
						"expected panic message to include %q, got %q",
						testCase.expectedFieldPath,
						panicMessage,
					)
				}
				if !strings.Contains(
					panicMessage,
					"must not be a machine-absolute filesystem path",
				) {
					t.Fatalf("unexpected panic message: %q", panicMessage)
				}
				if !strings.Contains(panicMessage, "\"/dist\"") {
					t.Fatalf(
						"expected panic message to include slash-prefixed-path example, got %q",
						panicMessage,
					)
				}
			}()

			_, _ = waveconfig.ParseConfigJSON([]byte(testCase.rawConfigJSON))
		})
	}
}

func TestParseConfigAllowsSlashRootedURLPathFields(t *testing.T) {
	raw := []byte(`{
		"Core":{
			"MainAppEntry":"cmd/app",
			"DistDir":"dist",
			"PublicPathPrefix":"/assets/"
		},
		"Watch":{"HealthcheckEndpoint":"/healthz"}
	}`)

	cfg, err := waveconfig.ParseConfigJSON(raw)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestParseConfigFileSetsConfigLocationToAbsolutePath(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "backend", "wave.config.json")

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create config parent directory: %v", err)
	}
	if err := os.WriteFile(
		configPath,
		[]byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist"}}`),
		0o644,
	); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := waveconfig.ParseConfigFile(configPath)
	if err != nil {
		t.Fatalf("ParseConfigFile returned error: %v", err)
	}

	expectedConfigLocation := filepath.Clean(configPath)
	if cfg.Core.ConfigLocation != expectedConfigLocation {
		t.Fatalf(
			"expected config location %q, got %q",
			expectedConfigLocation,
			cfg.Core.ConfigLocation,
		)
	}
}

func TestParseConfigFileRejectsEmptyPath(t *testing.T) {
	_, err := waveconfig.ParseConfigFile("   ")
	if err == nil {
		t.Fatal("expected ParseConfigFile to fail for empty config path")
	}
	if !strings.Contains(err.Error(), "config file path is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

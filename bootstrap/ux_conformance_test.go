package bootstrap

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

func TestBootstrapRouteTemplateUsesTopLevelRegistrations(t *testing.T) {
	exampleRoutesTemplate, err := tmplsFS.ReadFile("tmpls/backend_src_router_example_routes_go_tmpl.txt")
	if err != nil {
		t.Fatalf("read example routes template: %v", err)
	}
	content := string(exampleRoutesTemplate)

	expectedSubstrings := []string{
		"var _ = NewLoader(",
		"var _ = NewAction(",
	}
	for _, expectedSubstring := range expectedSubstrings {
		if !strings.Contains(content, expectedSubstring) {
			t.Fatalf("expected template to contain %q", expectedSubstring)
		}
	}

	forbiddenSubstrings := []string{
		"func registerExampleLoaders(",
		"func registerExampleActions(",
		"registerAllRoutes",
	}
	for _, forbiddenSubstring := range forbiddenSubstrings {
		if strings.Contains(content, forbiddenSubstring) {
			t.Fatalf("did not expect template to contain %q", forbiddenSubstring)
		}
	}
}

func TestBootstrapTemplateSetDoesNotIncludeRegistrationWrapperFile(t *testing.T) {
	_, err := tmplsFS.ReadFile("tmpls/backend_src_router_registration_go_tmpl.txt")
	if err == nil {
		t.Fatal("did not expect registration wrapper template file to exist")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unexpected error reading registration wrapper template: %v", err)
	}
}

func TestBootstrapBuildTemplateUsesSingleAppVariable(t *testing.T) {
	buildTemplate, err := tmplsFS.ReadFile("tmpls/cmd_build_main_go_tmpl.txt")
	if err != nil {
		t.Fatalf("read build command template: %v", err)
	}
	if !strings.Contains(string(buildTemplate), "vormabuild.Build(router.App)") {
		t.Fatalf("expected build template to call vormabuild.Build(router.App)")
	}
}

func TestBootstrapServeTemplateHandlesListenAndServeErrors(t *testing.T) {
	serveTemplate, err := tmplsFS.ReadFile("tmpls/cmd_app_main_go_tmpl.txt")
	if err != nil {
		t.Fatalf("read serve command template: %v", err)
	}

	content := string(serveTemplate)
	if !strings.Contains(content, "if err := http.ListenAndServe") {
		t.Fatalf("expected serve template to handle ListenAndServe errors")
	}
	if !strings.Contains(content, "panic(err)") {
		t.Fatalf("expected serve template to panic on ListenAndServe error")
	}
}

func TestBootstrapExampleRouteTemplateUsesAtomicCounter(t *testing.T) {
	exampleRoutesTemplate, err := tmplsFS.ReadFile("tmpls/backend_src_router_example_routes_go_tmpl.txt")
	if err != nil {
		t.Fatalf("read example routes template: %v", err)
	}
	content := string(exampleRoutesTemplate)

	requiredSubstrings := []string{
		`"sync/atomic"`,
		"var requestCount atomic.Int64",
		"requestCount.Load()",
		"requestCount.Add(1)",
	}
	for _, requiredSubstring := range requiredSubstrings {
		if !strings.Contains(content, requiredSubstring) {
			t.Fatalf("expected template to contain %q", requiredSubstring)
		}
	}

	forbiddenSubstrings := []string{
		"var count = 0",
		"count++",
	}
	for _, forbiddenSubstring := range forbiddenSubstrings {
		if strings.Contains(content, forbiddenSubstring) {
			t.Fatalf("did not expect template to contain %q", forbiddenSubstring)
		}
	}
}

func TestBootstrapResolveJSDevDependencyInstallCommand(t *testing.T) {
	testCases := []struct {
		name               string
		jsPackageManager   string
		packages           []string
		expectedCommand    string
		expectedCommandArg []string
	}{
		{
			name:               "npm",
			jsPackageManager:   "npm",
			packages:           []string{"vite", "typescript"},
			expectedCommand:    "npm",
			expectedCommandArg: []string{"i", "-D", "vite", "typescript"},
		},
		{
			name:               "pnpm",
			jsPackageManager:   "pnpm",
			packages:           []string{"vite", "typescript"},
			expectedCommand:    "pnpm",
			expectedCommandArg: []string{"add", "-D", "vite", "typescript"},
		},
		{
			name:               "yarn",
			jsPackageManager:   "yarn",
			packages:           []string{"vite", "typescript"},
			expectedCommand:    "yarn",
			expectedCommandArg: []string{"add", "-D", "vite", "typescript"},
		},
		{
			name:               "bun",
			jsPackageManager:   "bun",
			packages:           []string{"vite", "typescript"},
			expectedCommand:    "bun",
			expectedCommandArg: []string{"add", "-d", "vite", "typescript"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotCommand, gotCommandArgs := resolveJSDevDependencyInstallCommand(
				testCase.jsPackageManager,
				testCase.packages,
			)
			if gotCommand != testCase.expectedCommand {
				t.Fatalf("command = %q, want %q", gotCommand, testCase.expectedCommand)
			}
			if !reflect.DeepEqual(gotCommandArgs, testCase.expectedCommandArg) {
				t.Fatalf("args = %#v, want %#v", gotCommandArgs, testCase.expectedCommandArg)
			}
		})
	}
}

func TestBootstrapDerived_UnknownJSPackageManagerPanics(t *testing.T) {
	defer func() {
		recoveredValue := recover()
		if recoveredValue == nil {
			t.Fatal("expected panic for unknown JSPackageManager")
		}
		panicText := recoveredValue.(string)
		if !strings.Contains(panicText, "unknown JSPackageManager") {
			t.Fatalf("panic text = %q, expected unknown JSPackageManager guidance", panicText)
		}
	}()

	_ = Options{
		GoImportBase:     "example.com/app",
		UIVariant:        "react",
		JSPackageManager: "not-a-manager",
		DeploymentTarget: "none",
		GoVersion:        "go1.24.0",
	}.derived()
}

func TestBootstrapDerived_DockerTargetRequiresNodeMajorVersion(t *testing.T) {
	t.Run("panics when NodeMajorVersion is empty", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic when NodeMajorVersion is empty for docker target")
			}
		}()

		Options{
			GoImportBase:     "example.com/app",
			DeploymentTarget: "docker",
			JSPackageManager: "npm",
			NodeMajorVersion: "",
			GoVersion:        "go1.24.0",
			HasParentModule:  false,
			IncludeTailwind:  false,
			CreatedInDir:     "",
			ModuleRoot:       "",
			CurrentDir:       "",
			UIVariant:        "react",
		}.derived()
	})

	t.Run("panics when NodeMajorVersion has non-digit runes", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for non-digit NodeMajorVersion")
			}
		}()

		Options{
			GoImportBase:     "example.com/app",
			DeploymentTarget: "docker",
			JSPackageManager: "npm",
			NodeMajorVersion: "22.x",
			GoVersion:        "go1.24.0",
			HasParentModule:  false,
			IncludeTailwind:  false,
			CreatedInDir:     "",
			ModuleRoot:       "",
			CurrentDir:       "",
			UIVariant:        "react",
		}.derived()
	})

	t.Run("does not panic for numeric NodeMajorVersion", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("unexpected panic for numeric NodeMajorVersion: %v", r)
			}
		}()

		_ = Options{
			GoImportBase:     "example.com/app",
			DeploymentTarget: "docker",
			JSPackageManager: "npm",
			NodeMajorVersion: "22",
			GoVersion:        "go1.24.0",
			HasParentModule:  false,
			IncludeTailwind:  false,
			CreatedInDir:     "",
			ModuleRoot:       "",
			CurrentDir:       "",
			UIVariant:        "react",
		}.derived()
	})
}

func TestBootstrapWaveTemplateSetUsesDevProdSplitFiles(t *testing.T) {
	devTemplate, err := tmplsFS.ReadFile("tmpls/backend_wave_dev_go_str.txt")
	if err != nil {
		t.Fatalf("read dev wave template: %v", err)
	}
	prodTemplate, err := tmplsFS.ReadFile("tmpls/backend_wave_prod_go_str.txt")
	if err != nil {
		t.Fatalf("read prod wave template: %v", err)
	}

	for _, requiredFragment := range []string{
		"//go:build !prod",
		"os.DirFS(\"backend\")",
		"WaveConfigJSON:",
		"DistStaticFS:",
	} {
		if !strings.Contains(string(devTemplate), requiredFragment) {
			t.Fatalf("expected dev wave template to contain %q", requiredFragment)
		}
	}

	for _, requiredFragment := range []string{
		"//go:build prod",
		"//go:embed all:dist/static wave.config.json",
		"WaveConfigJSON:",
		"DistStaticFS:",
	} {
		if !strings.Contains(string(prodTemplate), requiredFragment) {
			t.Fatalf("expected prod wave template to contain %q", requiredFragment)
		}
	}
}

func TestBootstrapWaveTemplateSetDoesNotIncludeSingleWaveTemplate(t *testing.T) {
	_, err := tmplsFS.ReadFile("tmpls/backend_wave_go_tmpl.txt")
	if err == nil {
		t.Fatal("did not expect single wave template file to exist")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unexpected error reading single wave template: %v", err)
	}
}

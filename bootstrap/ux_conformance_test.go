package bootstrap

import (
	"errors"
	"io/fs"
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

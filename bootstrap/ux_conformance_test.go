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

func TestBootstrapWaveTemplateUsesSingleJSONConfigPath(t *testing.T) {
	waveTemplate, err := tmplsFS.ReadFile("tmpls/backend_wave_go_tmpl.txt")
	if err != nil {
		t.Fatalf("read wave template: %v", err)
	}

	waveTemplateContent := string(waveTemplate)
	requiredFragments := []string{
		"wave.config.json",
		"WaveConfigJSON:",
		"DistStaticFS:",
	}
	for _, requiredFragment := range requiredFragments {
		if !strings.Contains(waveTemplateContent, requiredFragment) {
			t.Fatalf("expected wave template to contain %q", requiredFragment)
		}
	}
}

func TestBootstrapWaveTemplateSetDoesNotIncludeWaveOptionsSplitFiles(t *testing.T) {
	waveOptionTemplatePaths := []string{
		"tmpls/backend_wave_options_dev_go_tmpl.txt",
		"tmpls/backend_wave_options_prod_embed_go_tmpl.txt",
		"tmpls/backend_wave_options_prod_filesystem_go_tmpl.txt",
	}

	for _, waveOptionTemplatePath := range waveOptionTemplatePaths {
		_, err := tmplsFS.ReadFile(waveOptionTemplatePath)
		if err == nil {
			t.Fatalf("did not expect wave options split template file to exist: %s", waveOptionTemplatePath)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf(
				"unexpected error reading %s: %v",
				waveOptionTemplatePath,
				err,
			)
		}
	}
}

package runtimeconfig

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/waveconfig"
)

func TestResolveDefaultsAndTrimmedValues(t *testing.T) {
	if got := ResolveDevReloadRoutesEndpointPath("   "); got != DefaultDevReloadRoutesEndpointPath {
		t.Fatalf(
			"ResolveDevReloadRoutesEndpointPath(blank) = %q, want %q",
			got,
			DefaultDevReloadRoutesEndpointPath,
		)
	}
	if got := ResolveDevReloadTemplateEndpointPath("  /reload-template "); got != "/reload-template" {
		t.Fatalf(
			"ResolveDevReloadTemplateEndpointPath(trimmed) = %q, want %q",
			got,
			"/reload-template",
		)
	}
	if got := ResolveDevReloadPublicFileMapEndpointPath("  /reload-public-filemap "); got != "/reload-public-filemap" {
		t.Fatalf(
			"ResolveDevReloadPublicFileMapEndpointPath(trimmed) = %q, want %q",
			got,
			"/reload-public-filemap",
		)
	}
	if got := ResolveTemplateDataKeyHeadElements("  HeadKey  "); got != "HeadKey" {
		t.Fatalf(
			"ResolveTemplateDataKeyHeadElements(trimmed) = %q, want %q",
			got,
			"HeadKey",
		)
	}
	if got := ResolveTemplateDataKeyBodyScripts(""); got != DefaultTemplateDataKeyBodyScripts {
		t.Fatalf(
			"ResolveTemplateDataKeyBodyScripts(blank) = %q, want %q",
			got,
			DefaultTemplateDataKeyBodyScripts,
		)
	}
	if got := ResolveTemplateDataKeySSRScript(""); got != DefaultTemplateDataKeySSRScript {
		t.Fatalf(
			"ResolveTemplateDataKeySSRScript(blank) = %q, want %q",
			got,
			DefaultTemplateDataKeySSRScript,
		)
	}
	if got := ResolveTemplateDataKeySSRScriptHash(""); got != DefaultTemplateDataKeySSRScriptHash {
		t.Fatalf(
			"ResolveTemplateDataKeySSRScriptHash(blank) = %q, want %q",
			got,
			DefaultTemplateDataKeySSRScriptHash,
		)
	}
	if got := ResolveTemplateDataKeyRootElementID(""); got != DefaultTemplateDataKeyRootElementID {
		t.Fatalf(
			"ResolveTemplateDataKeyRootElementID(blank) = %q, want %q",
			got,
			DefaultTemplateDataKeyRootElementID,
		)
	}
	if got := ResolveClientRootElementID("  app-root "); got != "app-root" {
		t.Fatalf(
			"ResolveClientRootElementID(trimmed) = %q, want %q",
			got,
			"app-root",
		)
	}
}

func TestParseAndValidateVormaConfig_ResolvesPathsRelativeToConfigFileDirectory(
	t *testing.T,
) {
	root := t.TempDir()
	t.Chdir(root)
	configPath := filepath.Join("backend", "wave.config.json")

	rawConfigJSON := []byte(`{
		"Core":{
			"ProjectID":"test-project",
			"MainAppEntry":"../backend/cmd/app_main.go",
			"StaticAssetDirs":{
				"Public":"../static/public",
				"Private":"../static/private"
			}
		},
		"Vorma":{
			"MainBuildEntry":"../backend/cmd/build",
			"UIVariant":"react",
			"HTMLTemplateLocation":"entry.go.html",
			"ClientEntry":"../frontend/src/vorma.entry.tsx",
			"ClientRouteDefinitionPatterns":["../frontend/src/**/*vorma.routes.ts"],
			"ServerRouteDefinitionPatterns":["../backend/src/**/*.go"],
			"TSGenOutDir":"../frontend/src/vorma.gen"
		}
	}`)

	parsedWaveConfig, waveParseError := waveconfig.ParseConfigJSONWithConfigPath(
		rawConfigJSON,
		configPath,
	)
	if waveParseError != nil {
		t.Fatalf(
			"ParseConfigJSONWithConfigPath returned error: %v",
			waveParseError,
		)
	}

	config, parseError := ParseVormaConfigJSON(
		rawConfigJSON,
		parsedWaveConfig,
	)
	if parseError != nil {
		t.Fatalf("ParseAndValidateVormaConfig returned error: %v", parseError)
	}

	if got, want := config.MainBuildEntry(), filepath.Join("backend", "cmd", "build"); got != want {
		t.Fatalf("MainBuildEntry=%q, want %q", got, want)
	}
	if got, want := config.ClientEntry(), filepath.Join("frontend", "src", "vorma.entry.tsx"); got != want {
		t.Fatalf("ClientEntry=%q, want %q", got, want)
	}
	if got, want := config.TSGenOutDir(), filepath.Join("frontend", "src", "vorma.gen"); got != want {
		t.Fatalf("TSGenOutDir=%q, want %q", got, want)
	}
	if got, want := config.ServerRouteDefinitionPatterns()[0], filepath.Join("backend", "src", "**", "*.go"); got != want {
		t.Fatalf("ServerRouteDefinitionPatterns[0]=%q, want %q", got, want)
	}
	if got, want := config.ClientRouteDefinitionPatterns()[0], filepath.Join("frontend", "src", "**", "*vorma.routes.ts"); got != want {
		t.Fatalf("ClientRouteDefinitionPatterns[0]=%q, want %q", got, want)
	}
}

func TestParseAndValidateVormaConfig_RequiresParsedWaveConfig(
	t *testing.T,
) {
	rawConfigJSON := []byte(`{
		"Vorma":{
			"MainBuildEntry":"backend/cmd/build",
			"UIVariant":"react",
			"HTMLTemplateLocation":"entry.go.html",
			"ClientEntry":"frontend/src/vorma.entry.tsx",
			"ClientRouteDefinitionPatterns":["frontend/src/**/*vorma.routes.ts"],
			"TSGenOutDir":"frontend/src/vorma.gen"
		}
	}`)

	_, parseError := ParseVormaConfigJSON(
		rawConfigJSON,
		nil,
	)
	if parseError == nil {
		t.Fatal("expected ParseAndValidateVormaConfig to reject nil parsed wave config")
	}
	if !strings.Contains(
		parseError.Error(),
		"parsed wave config",
	) {
		t.Fatalf("unexpected parse error: %v", parseError)
	}
}

func TestNormalizeAndValidateMutableConfig_AppliesDefaultsAndValidation(
	t *testing.T,
) {
	mainBuildEntry := "cmd/app"
	uiVariant := "react"
	htmlTemplateLocation := "index.html"
	clientEntry := "frontend/src/vorma.entry.tsx"
	clientRouteDefinitionPatterns := []string{"frontend/src/routes/**/*.tsx"}
	tsGenOutDir := "frontend/src/generated"
	buildtimePublicURLFuncName := ""
	unresolvedRoutePolicy := " WARN "
	devReloadRoutesEndpointPath := ""
	devReloadTemplateEndpointPath := ""
	devReloadPublicFileMapEndpointPath := ""
	templateDataKeyHeadElements := ""
	templateDataKeyBodyScripts := ""
	templateDataKeySSRScript := ""
	templateDataKeySSRScriptHash := ""
	templateDataKeyRootElementID := ""
	clientRootElementID := ""

	config := MutableValidationConfig{
		MainBuildEntry:                     &mainBuildEntry,
		UIVariant:                          &uiVariant,
		HTMLTemplateLocation:               &htmlTemplateLocation,
		ClientEntry:                        &clientEntry,
		ClientRouteDefinitionPatterns:      &clientRouteDefinitionPatterns,
		TSGenOutDir:                        &tsGenOutDir,
		BuildtimePublicURLFuncName:         &buildtimePublicURLFuncName,
		UnresolvedRoutePolicy:              &unresolvedRoutePolicy,
		DevReloadRoutesEndpointPath:        &devReloadRoutesEndpointPath,
		DevReloadTemplateEndpointPath:      &devReloadTemplateEndpointPath,
		DevReloadPublicFileMapEndpointPath: &devReloadPublicFileMapEndpointPath,
		TemplateDataKeyHeadElements:        &templateDataKeyHeadElements,
		TemplateDataKeyBodyScripts:         &templateDataKeyBodyScripts,
		TemplateDataKeySSRScript:           &templateDataKeySSRScript,
		TemplateDataKeySSRScriptHash:       &templateDataKeySSRScriptHash,
		TemplateDataKeyRootElementID:       &templateDataKeyRootElementID,
		ClientRootElementID:                &clientRootElementID,
	}

	if err := NormalizeAndValidateMutableConfig(config); err != nil {
		t.Fatalf("NormalizeAndValidateMutableConfig returned error: %v", err)
	}

	if buildtimePublicURLFuncName != DefaultBuildtimePublicURLFuncName {
		t.Fatalf(
			"BuildtimePublicURLFuncName = %q, want %q",
			buildtimePublicURLFuncName,
			DefaultBuildtimePublicURLFuncName,
		)
	}
	if unresolvedRoutePolicy != UnresolvedRoutePolicyWarn {
		t.Fatalf(
			"UnresolvedRoutePolicy = %q, want %q",
			unresolvedRoutePolicy,
			UnresolvedRoutePolicyWarn,
		)
	}
	if devReloadRoutesEndpointPath != DefaultDevReloadRoutesEndpointPath {
		t.Fatalf(
			"DevReloadRoutesEndpointPath = %q, want %q",
			devReloadRoutesEndpointPath,
			DefaultDevReloadRoutesEndpointPath,
		)
	}
	if devReloadTemplateEndpointPath != DefaultDevReloadTemplateEndpointPath {
		t.Fatalf(
			"DevReloadTemplateEndpointPath = %q, want %q",
			devReloadTemplateEndpointPath,
			DefaultDevReloadTemplateEndpointPath,
		)
	}
	if devReloadPublicFileMapEndpointPath != DefaultDevReloadPublicFileMapEndpointPath {
		t.Fatalf(
			"DevReloadPublicFileMapEndpointPath = %q, want %q",
			devReloadPublicFileMapEndpointPath,
			DefaultDevReloadPublicFileMapEndpointPath,
		)
	}
	if templateDataKeyHeadElements != DefaultTemplateDataKeyHeadElements {
		t.Fatalf(
			"TemplateDataKeyHeadElements = %q, want %q",
			templateDataKeyHeadElements,
			DefaultTemplateDataKeyHeadElements,
		)
	}
	if templateDataKeyBodyScripts != DefaultTemplateDataKeyBodyScripts {
		t.Fatalf(
			"TemplateDataKeyBodyScripts = %q, want %q",
			templateDataKeyBodyScripts,
			DefaultTemplateDataKeyBodyScripts,
		)
	}
	if templateDataKeySSRScript != DefaultTemplateDataKeySSRScript {
		t.Fatalf(
			"TemplateDataKeySSRScript = %q, want %q",
			templateDataKeySSRScript,
			DefaultTemplateDataKeySSRScript,
		)
	}
	if templateDataKeySSRScriptHash != DefaultTemplateDataKeySSRScriptHash {
		t.Fatalf(
			"TemplateDataKeySSRScriptHash = %q, want %q",
			templateDataKeySSRScriptHash,
			DefaultTemplateDataKeySSRScriptHash,
		)
	}
	if templateDataKeyRootElementID != DefaultTemplateDataKeyRootElementID {
		t.Fatalf(
			"TemplateDataKeyRootElementID = %q, want %q",
			templateDataKeyRootElementID,
			DefaultTemplateDataKeyRootElementID,
		)
	}
	if clientRootElementID != DefaultClientRootElementID {
		t.Fatalf(
			"ClientRootElementID = %q, want %q",
			clientRootElementID,
			DefaultClientRootElementID,
		)
	}
}

func TestNormalizeAndValidateMutableConfig_ReturnsValidationErrors(
	t *testing.T,
) {
	t.Run("missing pointer fails fast", func(t *testing.T) {
		config := validMutableValidationConfig()
		config.TemplateDataKeyBodyScripts = nil

		if err := NormalizeAndValidateMutableConfig(config); err == nil {
			t.Fatal(
				"expected NormalizeAndValidateMutableConfig to return pointer validation error",
			)
		}
	})

	t.Run("duplicate template keys rejected", func(t *testing.T) {
		config := validMutableValidationConfig()
		*config.TemplateDataKeyHeadElements = "SharedTemplateKey"
		*config.TemplateDataKeyBodyScripts = "SharedTemplateKey"

		if err := NormalizeAndValidateMutableConfig(config); err == nil {
			t.Fatal(
				"expected NormalizeAndValidateMutableConfig to return duplicate-template-key error",
			)
		}
	})

	t.Run("reload endpoints must differ", func(t *testing.T) {
		config := validMutableValidationConfig()
		*config.DevReloadRoutesEndpointPath = "/__same"
		*config.DevReloadTemplateEndpointPath = "/__same"

		if err := NormalizeAndValidateMutableConfig(config); err == nil {
			t.Fatal(
				"expected NormalizeAndValidateMutableConfig to reject equal reload endpoints",
			)
		}
	})

	t.Run("public filemap reload endpoint needs leading slash", func(t *testing.T) {
		config := validMutableValidationConfig()
		*config.DevReloadPublicFileMapEndpointPath = "reload-public-filemap"

		if err := NormalizeAndValidateMutableConfig(config); err == nil {
			t.Fatal(
				"expected NormalizeAndValidateMutableConfig to reject non-absolute public filemap reload endpoint",
			)
		}
	})

	t.Run(
		"duplicate client route definition patterns rejected",
		func(t *testing.T) {
			config := validMutableValidationConfig()
			*config.ClientRouteDefinitionPatterns = []string{
				"frontend/src/routes/core.vorma.routes.ts",
				"frontend/src/routes/core.vorma.routes.ts",
			}

			err := NormalizeAndValidateMutableConfig(config)
			if err == nil {
				t.Fatal(
					"expected NormalizeAndValidateMutableConfig to reject duplicate client route definition patterns",
				)
			}
			if !strings.Contains(err.Error(), "duplicates an earlier pattern") {
				t.Fatalf(
					"error = %q, expected duplicate-pattern validation message",
					err,
				)
			}
		},
	)

	t.Run(
		"client route definition patterns with surrounding whitespace rejected",
		func(t *testing.T) {
			config := validMutableValidationConfig()
			*config.ClientRouteDefinitionPatterns = []string{
				" frontend/src/routes/core.vorma.routes.ts ",
			}

			err := NormalizeAndValidateMutableConfig(config)
			if err == nil {
				t.Fatal(
					"expected NormalizeAndValidateMutableConfig to reject client route definition patterns with surrounding whitespace",
				)
			}
			if !strings.Contains(
				err.Error(),
				"must not contain surrounding whitespace",
			) {
				t.Fatalf(
					"error = %q, expected surrounding-whitespace validation message",
					err,
				)
			}
		},
	)
}

func validMutableValidationConfig() MutableValidationConfig {
	mainBuildEntry := "cmd/app"
	uiVariant := "react"
	htmlTemplateLocation := "index.html"
	clientEntry := "frontend/src/vorma.entry.tsx"
	clientRouteDefinitionPatterns := []string{"frontend/src/routes/**/*.tsx"}
	tsGenOutDir := "frontend/src/generated"
	buildtimePublicURLFuncName := DefaultBuildtimePublicURLFuncName
	unresolvedRoutePolicy := UnresolvedRoutePolicyWarn
	devReloadRoutesEndpointPath := DefaultDevReloadRoutesEndpointPath
	devReloadTemplateEndpointPath := DefaultDevReloadTemplateEndpointPath
	devReloadPublicFileMapEndpointPath := DefaultDevReloadPublicFileMapEndpointPath
	templateDataKeyHeadElements := DefaultTemplateDataKeyHeadElements
	templateDataKeyBodyScripts := DefaultTemplateDataKeyBodyScripts
	templateDataKeySSRScript := DefaultTemplateDataKeySSRScript
	templateDataKeySSRScriptHash := DefaultTemplateDataKeySSRScriptHash
	templateDataKeyRootElementID := DefaultTemplateDataKeyRootElementID
	clientRootElementID := DefaultClientRootElementID

	return MutableValidationConfig{
		MainBuildEntry:                     &mainBuildEntry,
		UIVariant:                          &uiVariant,
		HTMLTemplateLocation:               &htmlTemplateLocation,
		ClientEntry:                        &clientEntry,
		ClientRouteDefinitionPatterns:      &clientRouteDefinitionPatterns,
		TSGenOutDir:                        &tsGenOutDir,
		BuildtimePublicURLFuncName:         &buildtimePublicURLFuncName,
		UnresolvedRoutePolicy:              &unresolvedRoutePolicy,
		DevReloadRoutesEndpointPath:        &devReloadRoutesEndpointPath,
		DevReloadTemplateEndpointPath:      &devReloadTemplateEndpointPath,
		DevReloadPublicFileMapEndpointPath: &devReloadPublicFileMapEndpointPath,
		TemplateDataKeyHeadElements:        &templateDataKeyHeadElements,
		TemplateDataKeyBodyScripts:         &templateDataKeyBodyScripts,
		TemplateDataKeySSRScript:           &templateDataKeySSRScript,
		TemplateDataKeySSRScriptHash:       &templateDataKeySSRScriptHash,
		TemplateDataKeyRootElementID:       &templateDataKeyRootElementID,
		ClientRootElementID:                &clientRootElementID,
	}
}

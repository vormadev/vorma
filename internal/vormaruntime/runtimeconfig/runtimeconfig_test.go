package runtimeconfig

import "testing"

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
	templateDataKeyHeadElements := ""
	templateDataKeyBodyScripts := ""
	templateDataKeySSRScript := ""
	templateDataKeySSRScriptHash := ""
	templateDataKeyRootElementID := ""
	clientRootElementID := ""

	config := MutableValidationConfig{
		MainBuildEntry:                &mainBuildEntry,
		UIVariant:                     &uiVariant,
		HTMLTemplateLocation:          &htmlTemplateLocation,
		ClientEntry:                   &clientEntry,
		ClientRouteDefinitionPatterns: &clientRouteDefinitionPatterns,
		TSGenOutDir:                   &tsGenOutDir,
		BuildtimePublicURLFuncName:    &buildtimePublicURLFuncName,
		UnresolvedRoutePolicy:         &unresolvedRoutePolicy,
		DevReloadRoutesEndpointPath:   &devReloadRoutesEndpointPath,
		DevReloadTemplateEndpointPath: &devReloadTemplateEndpointPath,
		TemplateDataKeyHeadElements:   &templateDataKeyHeadElements,
		TemplateDataKeyBodyScripts:    &templateDataKeyBodyScripts,
		TemplateDataKeySSRScript:      &templateDataKeySSRScript,
		TemplateDataKeySSRScriptHash:  &templateDataKeySSRScriptHash,
		TemplateDataKeyRootElementID:  &templateDataKeyRootElementID,
		ClientRootElementID:           &clientRootElementID,
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
	templateDataKeyHeadElements := DefaultTemplateDataKeyHeadElements
	templateDataKeyBodyScripts := DefaultTemplateDataKeyBodyScripts
	templateDataKeySSRScript := DefaultTemplateDataKeySSRScript
	templateDataKeySSRScriptHash := DefaultTemplateDataKeySSRScriptHash
	templateDataKeyRootElementID := DefaultTemplateDataKeyRootElementID
	clientRootElementID := DefaultClientRootElementID

	return MutableValidationConfig{
		MainBuildEntry:                &mainBuildEntry,
		UIVariant:                     &uiVariant,
		HTMLTemplateLocation:          &htmlTemplateLocation,
		ClientEntry:                   &clientEntry,
		ClientRouteDefinitionPatterns: &clientRouteDefinitionPatterns,
		TSGenOutDir:                   &tsGenOutDir,
		BuildtimePublicURLFuncName:    &buildtimePublicURLFuncName,
		UnresolvedRoutePolicy:         &unresolvedRoutePolicy,
		DevReloadRoutesEndpointPath:   &devReloadRoutesEndpointPath,
		DevReloadTemplateEndpointPath: &devReloadTemplateEndpointPath,
		TemplateDataKeyHeadElements:   &templateDataKeyHeadElements,
		TemplateDataKeyBodyScripts:    &templateDataKeyBodyScripts,
		TemplateDataKeySSRScript:      &templateDataKeySSRScript,
		TemplateDataKeySSRScriptHash:  &templateDataKeySSRScriptHash,
		TemplateDataKeyRootElementID:  &templateDataKeyRootElementID,
		ClientRootElementID:           &clientRootElementID,
	}
}

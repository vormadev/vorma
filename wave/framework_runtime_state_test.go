package wave

import (
	"context"
	"testing"

	"github.com/vormadev/vorma/lab/jsonschema"
)

func TestCopyFrameworkRuntimeFieldsFrom(
	t *testing.T,
) {
	previousParsedConfig := &ParsedConfig{
		FrameworkSchemaExtensions: map[string]jsonschema.Entry{
			"Vorma": {
				Type: jsonschema.TypeObject,
			},
		},
		FrameworkWatchPatterns: []WatchedFile{
			{
				Pattern: "**/*.route",
				OnChangeHooks: []OnChangeHook{
					{
						Cmd:     "go run ./cmd/generate_routes",
						Exclude: []string{"generated/**"},
					},
				},
				SortedHooks: &SortedHooks{
					Pre: []OnChangeHook{
						{
							Cmd:     "go run ./cmd/pre_hook",
							Exclude: []string{"skip-pre/**"},
						},
					},
				},
			},
		},
		FrameworkIgnoredPatterns:     []string{"generated/**"},
		FrameworkPublicFileMapOutDir: "backend/dist/public_filemap",
		FrameworkDevBuildHook:        "go run ./cmd/build --dev",
		FrameworkProdBuildHook:       "go run ./cmd/build --prod",
		FrameworkRunBuildHook: func(
			_ context.Context,
			_ bool,
		) error {
			return nil
		},
		FrameworkPrepareGoBuildOverlay: func() (*GoBuildOverlay, error) {
			return &GoBuildOverlay{
				OverlayConfigPath: "/tmp/test-overlay.json",
			}, nil
		},
		FrameworkBrowserRuntimeNamespace:              "__vorma_runtime",
		FrameworkBrowserPublicURLResolverFunctionName: "resolvePublicURL",
		FrameworkBrowserRevalidateFunctionName:        "__vorma_revalidate",
		FrameworkRefreshRebuildingOverlayElementID:    "vorma-refresh-overlay",
		FrameworkCriticalCSSStyleElementID:            "vorma-critical-css",
		FrameworkNonCriticalCSSLinkElementID:          "vorma-noncritical-css",
	}

	parsedConfig := &ParsedConfig{}
	parsedConfig.CopyFrameworkRuntimeFieldsFrom(previousParsedConfig)

	previousParsedConfig.FrameworkWatchPatterns[0].Pattern = "**/*.changed"
	previousParsedConfig.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0] = "changed-hook/**"
	previousParsedConfig.FrameworkWatchPatterns[0].SortedHooks.Pre[0].Exclude[0] = "changed-pre/**"
	previousParsedConfig.FrameworkIgnoredPatterns[0] = "changed-ignored/**"
	previousParsedConfig.FrameworkPublicFileMapOutDir = "backend/dist/changed_filemap"
	previousParsedConfig.FrameworkDevBuildHook = "go run ./cmd/build --changed-dev"
	previousParsedConfig.FrameworkProdBuildHook = "go run ./cmd/build --changed-prod"
	previousParsedConfig.FrameworkSchemaExtensions["Vorma"] = jsonschema.Entry{
		Type: jsonschema.TypeString,
	}
	previousParsedConfig.FrameworkBrowserRuntimeNamespace = "__changed_runtime"
	previousParsedConfig.FrameworkBrowserPublicURLResolverFunctionName = "changedPublicURLResolver"
	previousParsedConfig.FrameworkBrowserRevalidateFunctionName = "__changed_revalidate"
	previousParsedConfig.FrameworkRefreshRebuildingOverlayElementID = "changed-refresh-overlay"
	previousParsedConfig.FrameworkCriticalCSSStyleElementID = "changed-critical-css"
	previousParsedConfig.FrameworkNonCriticalCSSLinkElementID = "changed-noncritical-css"

	if got := parsedConfig.FrameworkWatchPatterns[0].Pattern; got != "**/*.route" {
		t.Fatalf("framework watch pattern = %q, want **/*.route", got)
	}
	if got := parsedConfig.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0]; got != "generated/**" {
		t.Fatalf("framework watch hook exclude = %q, want generated/**", got)
	}
	if got := parsedConfig.FrameworkWatchPatterns[0].SortedHooks.Pre[0].Exclude[0]; got != "skip-pre/**" {
		t.Fatalf("framework sorted pre-hook exclude = %q, want skip-pre/**", got)
	}
	if got := parsedConfig.FrameworkIgnoredPatterns[0]; got != "generated/**" {
		t.Fatalf("framework ignored pattern = %q, want generated/**", got)
	}
	if got := parsedConfig.FrameworkPublicFileMapOutDir; got != "backend/dist/public_filemap" {
		t.Fatalf("framework public file map out dir = %q, want backend/dist/public_filemap", got)
	}
	if got := parsedConfig.FrameworkDevBuildHook; got != "go run ./cmd/build --dev" {
		t.Fatalf("framework dev build hook = %q, want go run ./cmd/build --dev", got)
	}
	if got := parsedConfig.FrameworkProdBuildHook; got != "go run ./cmd/build --prod" {
		t.Fatalf("framework prod build hook = %q, want go run ./cmd/build --prod", got)
	}
	if got := parsedConfig.FrameworkSchemaExtensions["Vorma"].Type; got != jsonschema.TypeObject {
		t.Fatalf("framework schema extension type = %v, want %v", got, jsonschema.TypeObject)
	}
	if parsedConfig.FrameworkRunBuildHook == nil {
		t.Fatal("framework run build hook was not preserved")
	}
	if err := parsedConfig.FrameworkRunBuildHook(context.Background(), true); err != nil {
		t.Fatalf("framework run build hook returned error: %v", err)
	}
	if parsedConfig.FrameworkPrepareGoBuildOverlay == nil {
		t.Fatal("framework go build overlay preparer was not preserved")
	}
	overlay, err := parsedConfig.FrameworkPrepareGoBuildOverlay()
	if err != nil {
		t.Fatalf("framework go build overlay preparer returned error: %v", err)
	}
	if overlay == nil || overlay.OverlayConfigPath != "/tmp/test-overlay.json" {
		t.Fatalf("framework go build overlay = %#v, want overlay config path /tmp/test-overlay.json", overlay)
	}
	if got := parsedConfig.FrameworkBrowserRuntimeNamespace; got != "__vorma_runtime" {
		t.Fatalf("framework browser runtime namespace = %q, want __vorma_runtime", got)
	}
	if got := parsedConfig.FrameworkBrowserPublicURLResolverFunctionName; got != "resolvePublicURL" {
		t.Fatalf(
			"framework public URL resolver function name = %q, want resolvePublicURL",
			got,
		)
	}
	if got := parsedConfig.FrameworkBrowserRevalidateFunctionName; got != "__vorma_revalidate" {
		t.Fatalf("framework browser revalidate function name = %q, want __vorma_revalidate", got)
	}
	if got := parsedConfig.FrameworkRefreshRebuildingOverlayElementID; got != "vorma-refresh-overlay" {
		t.Fatalf("framework refresh rebuilding overlay element ID = %q, want vorma-refresh-overlay", got)
	}
	if got := parsedConfig.FrameworkCriticalCSSStyleElementID; got != "vorma-critical-css" {
		t.Fatalf("framework critical CSS style element ID = %q, want vorma-critical-css", got)
	}
	if got := parsedConfig.FrameworkNonCriticalCSSLinkElementID; got != "vorma-noncritical-css" {
		t.Fatalf("framework non-critical CSS link element ID = %q, want vorma-noncritical-css", got)
	}
}

func TestCopyFrameworkRuntimeFieldsFromNilSourceIsNoOp(
	t *testing.T,
) {
	parsedConfig := &ParsedConfig{
		FrameworkDevBuildHook: "keep",
	}

	parsedConfig.CopyFrameworkRuntimeFieldsFrom(nil)

	if parsedConfig.FrameworkDevBuildHook != "keep" {
		t.Fatalf(
			"expected framework fields to be unchanged when source config is nil, got %q",
			parsedConfig.FrameworkDevBuildHook,
		)
	}
}

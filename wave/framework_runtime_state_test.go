package wave

import (
	"testing"
)

func TestCopyFrameworkRuntimeFieldsFrom(
	t *testing.T,
) {
	previousParsedConfig := &ParsedConfig{
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

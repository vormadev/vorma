package vormabuild

import (
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/globset"
)

func TestVormaOutWatchExcludePatternRemovesOutputFromWatchPatterns(t *testing.T) {
	cfg := vorma_cfg{C: &vorma.Config{
		DistDir: "fun/platform",
		DevWatchConfig: vorma.DevWatchConfig{
			WatchPatterns: []string{"."},
		},
	}}
	watch_set, err := globset.Compile(cfg.watch_patterns())
	if err != nil {
		t.Fatalf("error compiling watch patterns: %v", err)
	}

	rel_path := cfg.watch_relative_path(
		filepath.Join("fun", "platform", ".vorma", "main"),
	)
	if watch_set.Match(rel_path) {
		t.Fatalf(
			"expected effective watch patterns %v to exclude %q",
			cfg.watch_patterns(),
			rel_path,
		)
	}
}

func TestUIVariantAcceptsRemix(t *testing.T) {
	cfg := vorma_cfg{C: &vorma.Config{
		FrontendConfig: vorma.FrontendConfig{
			UIVariant: ui_variant_remix,
		},
	}}

	got, err := cfg.__validate_ui_variant()
	if err != nil {
		t.Fatalf("error validating UI variant: %v", err)
	}
	if got != ui_variant_remix {
		t.Fatalf("expected %q, got %q", ui_variant_remix, got)
	}
}

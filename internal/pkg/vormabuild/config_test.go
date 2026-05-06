package vormabuild

import (
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/globset"
)

func TestVormaOutWatchIgnorePatternMatchesWatchRootRelativePaths(t *testing.T) {
	tests := []struct {
		name       string
		watch_root string
		out_dir    string
		event_path string
	}{
		{
			name:       "cwd watch root",
			watch_root: ".",
			out_dir:    "fun/platform",
			event_path: filepath.Join("fun", "platform", ".vorma", "main"),
		},
		{
			name:       "app subdirectory watch root",
			watch_root: "fun",
			out_dir:    "fun/platform",
			event_path: filepath.Join("fun", "platform", ".vorma", "main"),
		},
		{
			name:       "output directory watch root",
			watch_root: filepath.Join("fun", "platform"),
			out_dir:    "fun/platform",
			event_path: filepath.Join("fun", "platform", ".vorma", "main"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := vorma_cfg{C: &vorma.Config{
				DistDir: tt.out_dir,
				DevWatchConfig: vorma.DevWatchConfig{
					WatchRoot: tt.watch_root,
				},
			}}
			ignore_set, err := globset.Compile(
				[]string{cfg.vorma_out_watch_ignore_pattern()},
			)
			if err != nil {
				t.Fatalf("error compiling ignore pattern: %v", err)
			}

			rel_path := cfg.watch_relative_path(tt.event_path)
			if !ignore_set.Match(rel_path) {
				t.Fatalf(
					"expected pattern %q to match watch-relative path %q",
					cfg.vorma_out_watch_ignore_pattern(),
					rel_path,
				)
			}
		})
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

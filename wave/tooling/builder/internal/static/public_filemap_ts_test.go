package static_test

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/builder/internal/static"
)

type staticAssetDirsForTests = struct {
	Private string `json:"Private"`
	Public  string `json:"Public"`
}

func TestWritePublicFileMapTS_WritesSortedTSAndJSONOutputs(t *testing.T) {
	root := t.TempDir()
	outDir := filepath.Join(root, "generated")

	cfg := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
			StaticAssetDirs: staticAssetDirsForTests{
				Public:  filepath.Join(root, "static", "public"),
				Private: filepath.Join(root, "static", "private"),
			},
		},
		FrameworkPublicFileMapOutDir: outDir,
	}
	cfg.Dist.Root = cfg.Core.DistDir

	processor := static.NewProcessor(
		cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	input := wave.FileMap{
		"z.js": {
			DistName:    "vorma_out_z_deadbeef.js",
			ContentHash: "vorma_out_z_deadbeef.js",
		},
		"a.js": {
			DistName:    "vorma_out_a_deadbeef.js",
			ContentHash: "vorma_out_a_deadbeef.js",
		},
	}
	if err := processor.SaveFileMap(input, cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("SaveFileMap returned error: %v", err)
	}

	if err := processor.WriteFrameworkPublicFileMapTS(); err != nil {
		t.Fatalf("WriteFrameworkPublicFileMapTS returned error: %v", err)
	}

	tsPath := filepath.Join(outDir, wave.RelPaths.PublicFileMapTSName())
	tsData, err := os.ReadFile(tsPath)
	if err != nil {
		t.Fatalf("failed reading generated TS file: %v", err)
	}

	tsContent := string(tsData)
	if !strings.Contains(tsContent, "\"a.js\": \"vorma_out_a_deadbeef.js\"") {
		t.Fatalf("expected TS output to contain a.js entry, got:\n%s", tsContent)
	}
	if !strings.Contains(tsContent, "\"z.js\": \"vorma_out_z_deadbeef.js\"") {
		t.Fatalf("expected TS output to contain z.js entry, got:\n%s", tsContent)
	}
	if strings.Index(tsContent, "\"a.js\"") > strings.Index(tsContent, "\"z.js\"") {
		t.Fatalf("expected TS entries to be sorted by key, got:\n%s", tsContent)
	}

	jsonPath := filepath.Join(outDir, wave.RelPaths.PublicFileMapJSONName())
	if _, statErr := os.Stat(jsonPath); statErr != nil {
		t.Fatalf("expected JSON file to be generated at %s: %v", jsonPath, statErr)
	}
}

func TestWriteFrameworkPublicFileMapTS_ServerOnlyModeWithoutFileMap(t *testing.T) {
	root := t.TempDir()
	outDir := filepath.Join(root, "generated")

	cfg := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry:   "cmd/app",
			ServerOnlyMode: true,
			DistDir:        filepath.Join(root, "dist"),
			StaticAssetDirs: staticAssetDirsForTests{
				Public:  filepath.Join(root, "static", "public"),
				Private: filepath.Join(root, "static", "private"),
			},
		},
		FrameworkPublicFileMapOutDir: outDir,
	}
	cfg.Dist.Root = cfg.Core.DistDir

	processor := static.NewProcessor(
		cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	if err := processor.WriteFrameworkPublicFileMapTS(); err != nil {
		t.Fatalf("WriteFrameworkPublicFileMapTS returned error: %v", err)
	}

	tsPath := filepath.Join(outDir, wave.RelPaths.PublicFileMapTSName())
	tsData, err := os.ReadFile(tsPath)
	if err != nil {
		t.Fatalf("failed reading generated TS file: %v", err)
	}
	if !strings.Contains(string(tsData), "staticPublicAssetMap = {") {
		t.Fatalf("expected generated TS output to include staticPublicAssetMap, got:\n%s", string(tsData))
	}
}

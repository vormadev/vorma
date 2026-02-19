package tooling

import (
	"github.com/vormadev/vorma/wave/tooling/builder"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestWritePublicFileMapTS_WritesSortedTSAndJSONOutputs(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

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
	if err := builder.SaveFileMap(input, cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("saveFileMap returned error: %v", err)
	}

	outDir := filepath.Join(root, "generated")
	if err := builder.WritePublicFileMapTS(outDir); err != nil {
		t.Fatalf("WritePublicFileMapTS returned error: %v", err)
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

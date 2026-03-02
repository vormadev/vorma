package static_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/wave/buildtime/builder/internal/static"
	"github.com/vormadev/vorma/wave/internal/wavefilemap"
	"github.com/vormadev/vorma/wave/waveartifacts"
)

type canonicalPublicFileMapValue struct {
	Dist      string `json:"dist"`
	Hash      string `json:"hash"`
	Prehashed bool   `json:"prehashed"`
}

func TestWriteCanonicalPublicFileMapJSONAndRef_WritesCanonicalHashedJSONAndRef(
	t *testing.T,
) {
	root := t.TempDir()

	cfg := wavetest.NewParsedConfigAtRoot(root)
	processor := static.NewProcessor(
		cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	zFileName := waveartifacts.ApplyWaveFileOutputPrefix("z_deadbeef.js")
	aFileName := waveartifacts.ApplyWaveFileOutputPrefix("a_deadbeef.js")
	input := wavefilemap.FileMap{
		"z.js": {
			DistName:    zFileName,
			ContentHash: zFileName,
		},
		"a.js": {
			DistName:    aFileName,
			ContentHash: aFileName,
		},
	}
	if err := processor.SaveFileMap(input, cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("SaveFileMap returned error: %v", err)
	}

	if err := processor.WriteCanonicalPublicFileMapJSONAndRef(); err != nil {
		t.Fatalf("WriteCanonicalPublicFileMapJSONAndRef returned error: %v", err)
	}

	refBytes, err := os.ReadFile(cfg.Dist.PublicFileMapRef())
	if err != nil {
		t.Fatalf("read public filemap ref: %v", err)
	}
	refTarget := strings.TrimSpace(string(refBytes))
	if refTarget == "" {
		t.Fatal("expected non-empty public filemap ref")
	}
	if !strings.HasPrefix(
		refTarget,
		waveartifacts.ApplyWaveFileOutputPrefix(
			waveartifacts.ApplyWaveOwnedFileOutputPrefix("public_filemap_"),
		),
	) {
		t.Fatalf("unexpected ref target %q", refTarget)
	}
	if !strings.HasSuffix(refTarget, ".json") {
		t.Fatalf("expected ref target to end with .json, got %q", refTarget)
	}

	canonicalJSONPath := filepath.Join(cfg.Dist.StaticPublic(), refTarget)
	canonicalJSONBytes, err := os.ReadFile(canonicalJSONPath)
	if err != nil {
		t.Fatalf("read canonical public filemap JSON: %v", err)
	}

	var parsed map[string]canonicalPublicFileMapValue
	if err := json.Unmarshal(canonicalJSONBytes, &parsed); err != nil {
		t.Fatalf("parse canonical public filemap JSON: %v", err)
	}
	if parsed["a.js"].Dist != aFileName {
		t.Fatalf(
			"a.js dist = %q, want %q",
			parsed["a.js"].Dist,
			aFileName,
		)
	}
	if parsed["z.js"].Dist != zFileName {
		t.Fatalf(
			"z.js dist = %q, want %q",
			parsed["z.js"].Dist,
			zFileName,
		)
	}
}

func TestWriteCanonicalPublicFileMapJSONAndRef_ServerOnlyModeWithoutFileMap(
	t *testing.T,
) {
	root := t.TempDir()

	cfg := wavetest.NewParsedConfigAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	processor := static.NewProcessor(
		cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	if err := processor.WriteCanonicalPublicFileMapJSONAndRef(); err != nil {
		t.Fatalf("WriteCanonicalPublicFileMapJSONAndRef returned error: %v", err)
	}

	refBytes, err := os.ReadFile(cfg.Dist.PublicFileMapRef())
	if err != nil {
		t.Fatalf("read public filemap ref: %v", err)
	}
	refTarget := strings.TrimSpace(string(refBytes))
	if refTarget == "" {
		t.Fatal("expected non-empty public filemap ref")
	}

	canonicalJSONPath := filepath.Join(cfg.Dist.StaticPublic(), refTarget)
	canonicalJSONBytes, err := os.ReadFile(canonicalJSONPath)
	if err != nil {
		t.Fatalf("read canonical public filemap JSON: %v", err)
	}

	var parsed map[string]canonicalPublicFileMapValue
	if err := json.Unmarshal(canonicalJSONBytes, &parsed); err != nil {
		t.Fatalf("parse canonical public filemap JSON: %v", err)
	}
	if len(parsed) != 0 {
		t.Fatalf("expected empty canonical public filemap, got %#v", parsed)
	}
}

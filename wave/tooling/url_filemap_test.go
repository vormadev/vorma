package tooling

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestPublicURLBuildtime_ReturnsErrorWhenMapIsMissing(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	url, err := builder.PublicURLBuildtime("images/logo.png")
	if err == nil {
		t.Fatal("expected error when public file map does not exist")
	}
	if url != "" {
		t.Fatalf("expected empty URL when file map is missing, got %q", url)
	}
}

func TestPublicURLBuildtime_ReturnsErrorWhenLookupMisses(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	fileMap := wave.FileMap{
		"images/logo.png": {
			DistName:    "vorma_out_images_logo_deadbeef.png",
			ContentHash: "vorma_out_images_logo_deadbeef.png",
		},
	}
	if err := builder.saveFileMap(fileMap, cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("saveFileMap returned error: %v", err)
	}

	missingURL, missingError := builder.PublicURLBuildtime("images/other.png")
	if missingError == nil {
		t.Fatal("expected lookup miss to return an error")
	}
	if missingURL != "" {
		t.Fatalf("expected empty URL for lookup miss, got %q", missingURL)
	}
}

func TestPublicURLBuildtime_ResolvesMappedPath(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	fileMap := wave.FileMap{
		"images/logo.png": {
			DistName:    "vorma_out_images_logo_deadbeef.png",
			ContentHash: "vorma_out_images_logo_deadbeef.png",
		},
	}
	if err := builder.saveFileMap(fileMap, cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("saveFileMap returned error: %v", err)
	}

	mappedURL, mappedErr := builder.PublicURLBuildtime("images/logo.png")
	if mappedErr != nil {
		t.Fatalf(
			"PublicURLBuildtime for mapped file returned error: %v",
			mappedErr,
		)
	}
	if mappedURL != "/vorma_out_images_logo_deadbeef.png" {
		t.Fatalf("unexpected mapped URL: %q", mappedURL)
	}
}

func TestMustPublicURLBuildtime_PanicsWhenMapIsMissing(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	defer func() {
		if recover() == nil {
			t.Fatal(
				"expected MustPublicURLBuildtime to panic when map is missing",
			)
		}
	}()

	_ = builder.MustPublicURLBuildtime("images/logo.png")
}

func TestMustPublicURLBuildtime_PanicsWhenLookupMisses(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	fileMap := wave.FileMap{
		"images/logo.png": {
			DistName:    "vorma_out_images_logo_deadbeef.png",
			ContentHash: "vorma_out_images_logo_deadbeef.png",
		},
	}
	if err := builder.saveFileMap(fileMap, cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("saveFileMap returned error: %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal(
				"expected MustPublicURLBuildtime to panic on lookup miss",
			)
		}
	}()

	_ = builder.MustPublicURLBuildtime("images/missing.png")
}

func TestMustPublicURLBuildtime_ResolvesMappedPath(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	fileMap := wave.FileMap{
		"images/logo.png": {
			DistName:    "vorma_out_images_logo_deadbeef.png",
			ContentHash: "vorma_out_images_logo_deadbeef.png",
		},
	}
	if err := builder.saveFileMap(fileMap, cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("saveFileMap returned error: %v", err)
	}

	mapped := builder.MustPublicURLBuildtime("images/logo.png")
	if mapped != "/vorma_out_images_logo_deadbeef.png" {
		t.Fatalf("unexpected mapped URL: %q", mapped)
	}
}

func TestPublicURLBuildtimeCached_PanicsWhenMapIsMissing(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	defer func() {
		if recover() == nil {
			t.Fatal(
				"expected getPublicURLBuildtimeCached to panic when map is missing",
			)
		}
	}()

	_ = builder.getPublicURLBuildtimeCached("images/logo.png")
}

func TestPublicURLBuildtimeCached_PanicsWhenLookupMisses(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	fileMap := wave.FileMap{
		"images/logo.png": {
			DistName:    "vorma_out_images_logo_deadbeef.png",
			ContentHash: "vorma_out_images_logo_deadbeef.png",
		},
	}
	if err := builder.saveFileMap(fileMap, cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("saveFileMap returned error: %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal(
				"expected getPublicURLBuildtimeCached to panic on lookup miss",
			)
		}
	}()

	_ = builder.getPublicURLBuildtimeCached("images/missing.png")
}

func TestPublicFileMapViews_ExcludePrehashedEntries(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	fileMap := wave.FileMap{
		"b.css": {
			DistName:    "vorma_out_b_deadbeef.css",
			ContentHash: "vorma_out_b_deadbeef.css",
		},
		"a.css": {
			DistName:    "vorma_out_a_deadbeef.css",
			ContentHash: "vorma_out_a_deadbeef.css",
		},
		"vendor/prehashed.js": {
			DistName:    "vendor/prehashed.js",
			ContentHash: "vorma_out_vendor_prehashed_deadbeef.js",
			IsPrehashed: true,
		},
	}
	if err := builder.saveFileMap(fileMap, cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("saveFileMap returned error: %v", err)
	}

	keys, err := builder.PublicFileMapKeys()
	if err != nil {
		t.Fatalf("PublicFileMapKeys returned error: %v", err)
	}
	expectedKeys := []string{"a.css", "b.css"}
	if !reflect.DeepEqual(keys, expectedKeys) {
		t.Fatalf("PublicFileMapKeys=%#v, want %#v", keys, expectedKeys)
	}

	simpleMap, simpleErr := builder.SimplePublicFileMap()
	if simpleErr != nil {
		t.Fatalf("SimplePublicFileMap returned error: %v", simpleErr)
	}
	expectedSimple := map[string]string{
		"a.css": "vorma_out_a_deadbeef.css",
		"b.css": "vorma_out_b_deadbeef.css",
	}
	if !reflect.DeepEqual(simpleMap, expectedSimple) {
		t.Fatalf("SimplePublicFileMap=%#v, want %#v", simpleMap, expectedSimple)
	}
}

func TestPublicFileMapKeys_BuildsFileMapWhenMissing(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	publicDir := cfg.Core.StaticAssetDirs.Public
	privateDir := cfg.Core.StaticAssetDirs.Private
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}
	if err := os.MkdirAll(privateDir, 0755); err != nil {
		t.Fatalf("failed creating private dir: %v", err)
	}

	assetPath := filepath.Join(publicDir, "images", "logo.png")
	if err := os.MkdirAll(filepath.Dir(assetPath), 0755); err != nil {
		t.Fatalf("failed creating asset parent dir: %v", err)
	}
	if err := os.WriteFile(assetPath, []byte("logo"), 0644); err != nil {
		t.Fatalf("failed writing asset file: %v", err)
	}

	keys, err := builder.PublicFileMapKeys()
	if err != nil {
		t.Fatalf("PublicFileMapKeys returned error: %v", err)
	}

	expected := []string{"images/logo.png"}
	if !reflect.DeepEqual(keys, expected) {
		t.Fatalf("PublicFileMapKeys=%#v, want %#v", keys, expected)
	}
}

func TestAddPublicAssetKeys_EmitsTypedAssetKeyDefinitions(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	fileMap := wave.FileMap{
		"images/logo.png": {
			DistName:    "vorma_out_images_logo_deadbeef.png",
			ContentHash: "vorma_out_images_logo_deadbeef.png",
		},
		"scripts/app.js": {
			DistName:    "vorma_out_scripts_app_deadbeef.js",
			ContentHash: "vorma_out_scripts_app_deadbeef.js",
		},
	}
	if err := builder.saveFileMap(fileMap, cfg.Dist.PublicFileMapGob()); err != nil {
		t.Fatalf("saveFileMap returned error: %v", err)
	}

	statements, err := builder.AddPublicAssetKeys(nil)
	if err != nil {
		t.Fatalf("AddPublicAssetKeys returned error: %v", err)
	}
	generated := statements.BuildString()

	if !strings.Contains(generated, "const WAVE_PUBLIC_ASSETS") {
		t.Fatalf(
			"expected generated output to define WAVE_PUBLIC_ASSETS, got:\n%s",
			generated,
		)
	}
	if !strings.Contains(generated, "export type WavePublicAsset") {
		t.Fatalf(
			"expected generated output to define WavePublicAsset type, got:\n%s",
			generated,
		)
	}
	if !strings.Contains(generated, "images/logo.png") ||
		!strings.Contains(generated, "scripts/app.js") {
		t.Fatalf(
			"expected generated output to include public asset keys, got:\n%s",
			generated,
		)
	}
}

func TestAddPublicAssetKeys_ReturnsErrorWhenFileMapIsMissing(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	_, err := builder.AddPublicAssetKeys(nil)
	if err == nil {
		t.Fatal(
			"expected AddPublicAssetKeys to return error when file map is missing",
		)
	}
	if !strings.Contains(err.Error(), "public file map keys") {
		t.Fatalf("unexpected AddPublicAssetKeys error: %v", err)
	}
}

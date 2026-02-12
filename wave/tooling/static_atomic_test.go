package tooling

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestWriteFileAtomicBytes_CreatesParentDirectoriesAndReplacesContent(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nested", "dir", "file.txt")

	if err := writeFileAtomicBytes(target, []byte("first")); err != nil {
		t.Fatalf("writeFileAtomicBytes first write returned error: %v", err)
	}
	if err := writeFileAtomicBytes(target, []byte("second")); err != nil {
		t.Fatalf("writeFileAtomicBytes second write returned error: %v", err)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed reading target file: %v", err)
	}
	if string(content) != "second" {
		t.Fatalf("expected final content %q, got %q", "second", string(content))
	}
}

func TestWriteFileAtomic_CleansUpTempFileOnWriteError(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")

	if err := os.WriteFile(target, []byte("original"), 0644); err != nil {
		t.Fatalf("failed to seed target file: %v", err)
	}

	injectedErr := errors.New("injected write failure")
	err := writeFileAtomic(target, func(_ *os.File) error {
		return injectedErr
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected injected error, got %v", err)
	}

	content, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("failed reading target file: %v", readErr)
	}
	if string(content) != "original" {
		t.Fatalf("target file was modified on failed atomic write, got %q", string(content))
	}

	tmpFiles, globErr := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	if globErr != nil {
		t.Fatalf("failed globbing temp files: %v", globErr)
	}
	if len(tmpFiles) != 0 {
		t.Fatalf("expected no leftover temp files, found: %s", strings.Join(tmpFiles, ", "))
	}
}

func TestFileMapSaveAndLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	fileMapPath := filepath.Join(root, "maps", "public.gob")
	input := wave.FileMap{
		"styles.css": {
			DistName:    "vorma_out_styles_abcd1234.css",
			ContentHash: "vorma_out_styles_abcd1234.css",
		},
		"vendor/prehashed.js": {
			DistName:    "vendor/prehashed.js",
			ContentHash: "vorma_out_vendor_prehashed_abcd1234.js",
			IsPrehashed: true,
		},
	}

	if err := builder.saveFileMap(input, fileMapPath); err != nil {
		t.Fatalf("saveFileMap returned error: %v", err)
	}

	output, err := builder.loadFileMapFromPath(fileMapPath)
	if err != nil {
		t.Fatalf("loadFileMapFromPath returned error: %v", err)
	}

	if !reflect.DeepEqual(output, input) {
		t.Fatalf("loaded file map mismatch\noutput=%#v\ninput=%#v", output, input)
	}
}

func TestSetupDistDir_CreatesExpectedStructure(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	if err := SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}

	requiredPaths := []string{
		cfg.Dist.Internal(),
		cfg.Dist.StaticPublic(),
		cfg.Dist.StaticPrivate(),
		cfg.Dist.KeepFile(),
	}

	for _, path := range requiredPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected path to exist: %s (error: %v)", path, err)
		}
	}
}

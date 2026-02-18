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

func TestWriteFileAtomicBytes_CreatesParentDirectoriesAndReplacesContent(
	t *testing.T,
) {
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
		t.Fatalf(
			"target file was modified on failed atomic write, got %q",
			string(content),
		)
	}

	tmpFiles, globErr := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	if globErr != nil {
		t.Fatalf("failed globbing temp files: %v", globErr)
	}
	if len(tmpFiles) != 0 {
		t.Fatalf(
			"expected no leftover temp files, found: %s",
			strings.Join(tmpFiles, ", "),
		)
	}
}

func TestWriteFileAtomic_RetriesRenameByReplacingExistingTargetOnPermissionDenied(
	t *testing.T,
) {
	target := filepath.Join(t.TempDir(), "target.txt")
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatalf("failed seeding existing target: %v", err)
	}

	renameCallCount := 0
	removeCallCount := 0
	err := writeFileAtomicWithDependencies(
		target,
		func(file *os.File) error {
			_, writeError := file.Write([]byte("updated"))
			return writeError
		},
		atomicFileWriteDependencies{
			renameTempFile: func(oldPath string, newPath string) error {
				renameCallCount++
				if renameCallCount == 1 {
					return os.ErrPermission
				}
				return os.Rename(oldPath, newPath)
			},
			removeExistingTarget: func(path string) error {
				removeCallCount++
				return os.Remove(path)
			},
			statTarget: os.Stat,
		},
	)
	if err != nil {
		t.Fatalf("writeFileAtomicWithDependencies returned error: %v", err)
	}
	if renameCallCount != 2 {
		t.Fatalf(
			"expected rename to be retried once, got %d calls",
			renameCallCount,
		)
	}
	if removeCallCount != 1 {
		t.Fatalf(
			"expected existing target to be removed once, got %d calls",
			removeCallCount,
		)
	}

	content, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("failed reading updated target file: %v", readErr)
	}
	if string(content) != "updated" {
		t.Fatalf(
			"expected updated target file contents, got %q",
			string(content),
		)
	}
}

func TestWriteFileAtomic_DoesNotReplaceTargetWhenPermissionDeniedAndTargetMissing(
	t *testing.T,
) {
	target := filepath.Join(t.TempDir(), "target.txt")

	removeCallCount := 0
	err := writeFileAtomicWithDependencies(
		target,
		func(file *os.File) error {
			_, writeError := file.Write([]byte("updated"))
			return writeError
		},
		atomicFileWriteDependencies{
			renameTempFile: func(_, _ string) error {
				return os.ErrPermission
			},
			removeExistingTarget: func(path string) error {
				removeCallCount++
				return os.Remove(path)
			},
			statTarget: func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
		},
	)
	if err == nil {
		t.Fatal("expected rename permission error for missing target")
	}
	if removeCallCount != 0 {
		t.Fatalf(
			"expected missing target not to be removed, got %d remove calls",
			removeCallCount,
		)
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
		t.Fatalf(
			"loaded file map mismatch\noutput=%#v\ninput=%#v",
			output,
			input,
		)
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

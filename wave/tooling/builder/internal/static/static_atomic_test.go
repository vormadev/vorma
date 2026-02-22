package static_test

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/builder/internal/static"
)

func TestWriteFileAtomicBytes_CreatesParentDirectoriesAndReplacesContent(
	t *testing.T,
) {
	target := filepath.Join(t.TempDir(), "nested", "dir", "file.txt")

	if err := static.WriteFileAtomicBytes(target, []byte("first")); err != nil {
		t.Fatalf("writeFileAtomicBytes first write returned error: %v", err)
	}
	if err := static.WriteFileAtomicBytes(target, []byte("second")); err != nil {
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
	err := static.WriteFileAtomic(target, func(_ *os.File) error {
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
	err := static.WriteFileAtomicWithDependencies(
		target,
		func(file *os.File) error {
			_, writeError := file.Write([]byte("updated"))
			return writeError
		},
		static.AtomicFileWriteDependencies{
			RenameTempFile: func(oldPath string, newPath string) error {
				renameCallCount++
				if renameCallCount == 1 {
					return os.ErrPermission
				}
				return os.Rename(oldPath, newPath)
			},
			RemoveExistingTarget: func(path string) error {
				removeCallCount++
				return os.Remove(path)
			},
			StatTarget: os.Stat,
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
	err := static.WriteFileAtomicWithDependencies(
		target,
		func(file *os.File) error {
			_, writeError := file.Write([]byte("updated"))
			return writeError
		},
		static.AtomicFileWriteDependencies{
			RenameTempFile: func(_, _ string) error {
				return os.ErrPermission
			},
			RemoveExistingTarget: func(path string) error {
				removeCallCount++
				return os.Remove(path)
			},
			StatTarget: func(path string) (os.FileInfo, error) {
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
	cfg := newParsedConfigForStaticAtomicTestsAtRoot(root)
	staticProcessor := static.NewProcessor(
		cfg,
		newDiscardLoggerForStaticAtomicTests(),
	)

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

	if err := staticProcessor.SaveFileMap(input, fileMapPath); err != nil {
		t.Fatalf("saveFileMap returned error: %v", err)
	}

	output, err := staticProcessor.LoadFileMapFromPath(fileMapPath)
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
	cfg := newParsedConfigForStaticAtomicTestsAtRoot(root)

	if err := builder.SetupDistDir(cfg); err != nil {
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

func newDiscardLoggerForStaticAtomicTests() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newParsedConfigForStaticAtomicTestsAtRoot(root string) *wave.ParsedConfig {
	cfg := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
			StaticAssetDirs: staticAssetDirsForTests{
				Public:  filepath.Join(root, "static", "public"),
				Private: filepath.Join(root, "static", "private"),
			},
		},
		Watch: &wave.WatchConfig{
			WatchRoot: root,
		},
	}
	cfg.Dist.Root = cfg.Core.DistDir
	return cfg
}

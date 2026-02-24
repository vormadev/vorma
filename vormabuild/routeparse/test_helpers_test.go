package routeparse

import (
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

type staticFileInfo struct {
	name  string
	size  int64
	mode  fs.FileMode
	isDir bool
}

func (info staticFileInfo) Name() string {
	return info.name
}

func (info staticFileInfo) Size() int64 {
	return info.size
}

func (info staticFileInfo) Mode() fs.FileMode {
	return info.mode
}

func (info staticFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (info staticFileInfo) IsDir() bool {
	return info.isDir
}

func (info staticFileInfo) Sys() any {
	return nil
}

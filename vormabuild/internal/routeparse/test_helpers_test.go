package routeparse

import (
	"io/fs"
	"testing"
	"time"

	"log/slog"

	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func testLogger() *slog.Logger {
	return testkit.TestLogger()
}

func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	testkit.MustWriteFile(t, path, content)
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

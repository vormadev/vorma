package vormabuild

import (
	"bytes"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"
)

func TestGetFSSummaryHash_ChangesWhenFileContentChangesAtSamePathAndSize(t *testing.T) {
	fsA := fstest.MapFS{
		"alpha.txt": {Data: []byte("ab")},
		"beta.txt":  {Data: []byte("xyz")},
	}
	fsB := fstest.MapFS{
		"alpha.txt": {Data: []byte("cd")},
		"beta.txt":  {Data: []byte("uvw")},
	}

	hashA, err := getFSSummaryHash(fsA)
	if err != nil {
		t.Fatalf("getFSSummaryHash(fsA) failed: %v", err)
	}
	hashB, err := getFSSummaryHash(fsB)
	if err != nil {
		t.Fatalf("getFSSummaryHash(fsB) failed: %v", err)
	}

	if bytes.Equal(hashA, hashB) {
		t.Fatalf(
			"expected different hashes when file content changes at same path/size, got %x",
			hashA,
		)
	}
}

func TestGetFSSummaryHash_ChangesWhenSummaryChanges(t *testing.T) {
	t.Run("size change changes hash", func(t *testing.T) {
		before := fstest.MapFS{
			"alpha.txt": {Data: []byte("ab")},
		}
		after := fstest.MapFS{
			"alpha.txt": {Data: []byte("abc")},
		}

		beforeHash, err := getFSSummaryHash(before)
		if err != nil {
			t.Fatalf("getFSSummaryHash(before) failed: %v", err)
		}
		afterHash, err := getFSSummaryHash(after)
		if err != nil {
			t.Fatalf("getFSSummaryHash(after) failed: %v", err)
		}

		if bytes.Equal(beforeHash, afterHash) {
			t.Fatalf("expected different hashes when size changes, got %x", beforeHash)
		}
	})

	t.Run("path change changes hash", func(t *testing.T) {
		before := fstest.MapFS{
			"alpha.txt": {Data: []byte("ab")},
		}
		after := fstest.MapFS{
			"beta.txt": {Data: []byte("ab")},
		}

		beforeHash, err := getFSSummaryHash(before)
		if err != nil {
			t.Fatalf("getFSSummaryHash(before) failed: %v", err)
		}
		afterHash, err := getFSSummaryHash(after)
		if err != nil {
			t.Fatalf("getFSSummaryHash(after) failed: %v", err)
		}

		if bytes.Equal(beforeHash, afterHash) {
			t.Fatalf("expected different hashes when path changes, got %x", beforeHash)
		}
	})
}

func TestGetFSSummaryHash_ReturnsErrorsFromFilesystemWalk(t *testing.T) {
	t.Run("returns error when opening root fails", func(t *testing.T) {
		expectedErr := errors.New("cannot open root")
		_, err := getFSSummaryHash(openErrorFS{err: expectedErr})
		if err == nil {
			t.Fatal("expected getFSSummaryHash to return filesystem open error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped open error", err)
		}
	})

	t.Run("returns error when DirEntry.Info fails", func(t *testing.T) {
		expectedErr := errors.New("cannot stat entry")
		_, err := getFSSummaryHash(dirEntryInfoErrorFS{infoErr: expectedErr})
		if err == nil {
			t.Fatal("expected getFSSummaryHash to return dir entry info error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped dir entry info error", err)
		}
	})
}

type openErrorFS struct {
	err error
}

func (fsys openErrorFS) Open(name string) (fs.File, error) {
	return nil, fsys.err
}

type dirEntryInfoErrorFS struct {
	infoErr error
}

func (fsys dirEntryInfoErrorFS) Stat(name string) (fs.FileInfo, error) {
	if name != "." {
		return nil, fs.ErrNotExist
	}
	return staticFileInfo{
		name:  ".",
		size:  0,
		mode:  fs.ModeDir | 0o755,
		isDir: true,
	}, nil
}

func (fsys dirEntryInfoErrorFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func (fsys dirEntryInfoErrorFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "." {
		return nil, fs.ErrNotExist
	}
	return []fs.DirEntry{dirEntryWithInfoError{err: fsys.infoErr}}, nil
}

type dirEntryWithInfoError struct {
	err error
}

func (entry dirEntryWithInfoError) Name() string {
	return "broken.txt"
}

func (entry dirEntryWithInfoError) IsDir() bool {
	return false
}

func (entry dirEntryWithInfoError) Type() fs.FileMode {
	return 0
}

func (entry dirEntryWithInfoError) Info() (fs.FileInfo, error) {
	return nil, entry.err
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

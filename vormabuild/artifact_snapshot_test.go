package vormabuild

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestCaptureBuildArtifactFile(t *testing.T) {
	t.Run("returns existing snapshot with content", func(t *testing.T) {
		var readPath string
		snapshot, err := captureBuildArtifactFile(
			"/tmp/artifact.json",
			func(path string) ([]byte, error) {
				readPath = path
				return []byte("artifact-content"), nil
			},
		)
		if err != nil {
			t.Fatalf("captureBuildArtifactFile returned error: %v", err)
		}
		if readPath != "/tmp/artifact.json" {
			t.Fatalf("read path = %q, want %q", readPath, "/tmp/artifact.json")
		}
		if !snapshot.existed {
			t.Fatal("snapshot.existed = false, want true")
		}
		if string(snapshot.content) != "artifact-content" {
			t.Fatalf("snapshot.content = %q, want %q", string(snapshot.content), "artifact-content")
		}
	})

	t.Run("returns zero snapshot for missing artifact", func(t *testing.T) {
		snapshot, err := captureBuildArtifactFile(
			"/tmp/missing-artifact.json",
			func(string) ([]byte, error) {
				return nil, &os.PathError{Op: "open", Path: "missing", Err: os.ErrNotExist}
			},
		)
		if err != nil {
			t.Fatalf("captureBuildArtifactFile returned error: %v", err)
		}
		if snapshot.existed {
			t.Fatalf("snapshot.existed = %v, want false", snapshot.existed)
		}
		if snapshot.content != nil {
			t.Fatalf("snapshot.content = %#v, want nil", snapshot.content)
		}
	})

	t.Run("returns zero snapshot for ENOTDIR", func(t *testing.T) {
		snapshot, err := captureBuildArtifactFile(
			"/tmp/not-a-directory/artifact.json",
			func(string) ([]byte, error) {
				return nil, &os.PathError{Op: "open", Path: "not-a-directory", Err: syscall.ENOTDIR}
			},
		)
		if err != nil {
			t.Fatalf("captureBuildArtifactFile returned error: %v", err)
		}
		if snapshot.existed {
			t.Fatalf("snapshot.existed = %v, want false", snapshot.existed)
		}
		if snapshot.content != nil {
			t.Fatalf("snapshot.content = %#v, want nil", snapshot.content)
		}
	})

	t.Run("returns read error", func(t *testing.T) {
		expectedErr := errors.New("read failed")
		_, err := captureBuildArtifactFile(
			"/tmp/artifact.json",
			func(string) ([]byte, error) {
				return nil, expectedErr
			},
		)
		if err == nil {
			t.Fatal("expected captureBuildArtifactFile to return error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped read error", err)
		}
	})
}

func TestRestoreBuildArtifactFile(t *testing.T) {
	t.Run("writes snapshot content when artifact existed", func(t *testing.T) {
		var writePath string
		var writtenContent []byte
		var writeMode os.FileMode
		removeCalled := false

		err := restoreBuildArtifactFile(
			"/tmp/artifact.json",
			buildArtifactFileSnapshot{existed: true, content: []byte("restore-content")},
			func(path string, content []byte, mode os.FileMode) error {
				writePath = path
				writtenContent = append([]byte(nil), content...)
				writeMode = mode
				return nil
			},
			func(string) error {
				removeCalled = true
				return nil
			},
		)
		if err != nil {
			t.Fatalf("restoreBuildArtifactFile returned error: %v", err)
		}
		if writePath != "/tmp/artifact.json" {
			t.Fatalf("write path = %q, want %q", writePath, "/tmp/artifact.json")
		}
		if string(writtenContent) != "restore-content" {
			t.Fatalf("written content = %q, want %q", string(writtenContent), "restore-content")
		}
		if writeMode != buildArtifactFileMode {
			t.Fatalf("write mode = %v, want %v", writeMode, buildArtifactFileMode)
		}
		if removeCalled {
			t.Fatal("did not expect remove callback when snapshot existed")
		}
	})

	t.Run("returns write error when restoring existing artifact", func(t *testing.T) {
		expectedErr := errors.New("write failed")
		err := restoreBuildArtifactFile(
			"/tmp/artifact.json",
			buildArtifactFileSnapshot{existed: true, content: []byte("restore-content")},
			func(string, []byte, os.FileMode) error {
				return expectedErr
			},
			func(string) error {
				t.Fatal("did not expect remove callback when snapshot existed")
				return nil
			},
		)
		if err == nil {
			t.Fatal("expected restoreBuildArtifactFile to return write error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped write error", err)
		}
	})

	t.Run("removes artifact when snapshot did not exist", func(t *testing.T) {
		var removePath string
		err := restoreBuildArtifactFile(
			"/tmp/artifact.json",
			buildArtifactFileSnapshot{},
			func(string, []byte, os.FileMode) error {
				t.Fatal("did not expect write callback when snapshot did not exist")
				return nil
			},
			func(path string) error {
				removePath = path
				return nil
			},
		)
		if err != nil {
			t.Fatalf("restoreBuildArtifactFile returned error: %v", err)
		}
		if removePath != "/tmp/artifact.json" {
			t.Fatalf("remove path = %q, want %q", removePath, "/tmp/artifact.json")
		}
	})

	t.Run("ignores missing artifact during remove", func(t *testing.T) {
		err := restoreBuildArtifactFile(
			"/tmp/missing-artifact.json",
			buildArtifactFileSnapshot{},
			func(string, []byte, os.FileMode) error {
				t.Fatal("did not expect write callback when snapshot did not exist")
				return nil
			},
			func(string) error {
				return &os.PathError{Op: "remove", Path: "missing", Err: os.ErrNotExist}
			},
		)
		if err != nil {
			t.Fatalf("restoreBuildArtifactFile returned error: %v", err)
		}
	})

	t.Run("ignores ENOTDIR during remove", func(t *testing.T) {
		err := restoreBuildArtifactFile(
			"/tmp/not-a-directory/artifact.json",
			buildArtifactFileSnapshot{},
			func(string, []byte, os.FileMode) error {
				t.Fatal("did not expect write callback when snapshot did not exist")
				return nil
			},
			func(string) error {
				return &os.PathError{Op: "remove", Path: "not-a-directory", Err: syscall.ENOTDIR}
			},
		)
		if err != nil {
			t.Fatalf("restoreBuildArtifactFile returned error: %v", err)
		}
	})

	t.Run("returns remove error", func(t *testing.T) {
		expectedErr := errors.New("remove failed")
		err := restoreBuildArtifactFile(
			"/tmp/artifact.json",
			buildArtifactFileSnapshot{},
			func(string, []byte, os.FileMode) error {
				t.Fatal("did not expect write callback when snapshot did not exist")
				return nil
			},
			func(string) error {
				return expectedErr
			},
		)
		if err == nil {
			t.Fatal("expected restoreBuildArtifactFile to return remove error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped remove error", err)
		}
		if strings.Contains(err.Error(), "write") {
			t.Fatalf("error = %q, did not expect write-related context", err)
		}
	})
}

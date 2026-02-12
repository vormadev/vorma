package vormabuild

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomically(t *testing.T) {
	t.Run("replaces target file contents without leaving temp files", func(t *testing.T) {
		outputDirectory := t.TempDir()
		targetPath := filepath.Join(outputDirectory, "artifact.json")
		mustWriteFile(t, targetPath, []byte(`{"old":true}`))

		if err := writeFileAtomically(targetPath, []byte(`{"new":true}`), 0o644); err != nil {
			t.Fatalf("writeFileAtomically returned error: %v", err)
		}

		rawBytes, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("read target file: %v", err)
		}
		if string(rawBytes) != `{"new":true}` {
			t.Fatalf("target file contents = %q, want %q", string(rawBytes), `{"new":true}`)
		}

		tempFileMatches, err := filepath.Glob(filepath.Join(outputDirectory, atomicFileWriteTempFilePattern))
		if err != nil {
			t.Fatalf("glob temp files: %v", err)
		}
		if len(tempFileMatches) != 0 {
			t.Fatalf("expected no temp files after atomic write, got %#v", tempFileMatches)
		}
	})

	t.Run("wraps create-temp-file errors", func(t *testing.T) {
		targetPath := filepath.Join(t.TempDir(), "missing-directory", "artifact.json")
		err := writeFileAtomically(targetPath, []byte(`{}`), 0o644)
		if err == nil {
			t.Fatal("expected writeFileAtomically to return create-temp-file error")
		}
		if !strings.Contains(err.Error(), "create temp file") {
			t.Fatalf("error = %q, expected create-temp-file context", err)
		}
	})

	t.Run("removes temp file when rename step fails", func(t *testing.T) {
		originalAtomicFileWriteDependencies := atomicFileWriteDeps
		t.Cleanup(func() {
			atomicFileWriteDeps = originalAtomicFileWriteDependencies
		})

		renameErr := errors.New("rename failed")
		var removedTempPath string
		atomicFileWriteDeps.renameTempFile = func(string, string) error {
			return renameErr
		}
		atomicFileWriteDeps.removeTempFile = func(path string) error {
			removedTempPath = path
			return os.Remove(path)
		}

		targetPath := filepath.Join(t.TempDir(), "artifact.json")
		err := writeFileAtomically(targetPath, []byte(`{"value":1}`), 0o644)
		if err == nil {
			t.Fatal("expected writeFileAtomically to return rename error")
		}
		if !strings.Contains(err.Error(), "rename temp file") {
			t.Fatalf("error = %q, expected rename-temp-file context", err)
		}
		if !errors.Is(err, renameErr) {
			t.Fatalf("error = %v, expected wrapped rename error", err)
		}
		if removedTempPath == "" {
			t.Fatal("expected temp file removal after rename error")
		}
		if _, statErr := os.Stat(removedTempPath); !os.IsNotExist(statErr) {
			t.Fatalf("expected removed temp file to not exist, stat err=%v", statErr)
		}
	})

	t.Run("closes and removes temp file when write step fails", func(t *testing.T) {
		originalAtomicFileWriteDependencies := atomicFileWriteDeps
		t.Cleanup(func() {
			atomicFileWriteDeps = originalAtomicFileWriteDependencies
		})

		writeErr := errors.New("write failed")
		closeCalled := false
		renameCalled := false
		removeCalled := false
		atomicFileWriteDeps.writeAllBytesToTempFile = func(*os.File, []byte) (int, error) {
			return 0, writeErr
		}
		atomicFileWriteDeps.closeTempFile = func(file *os.File) error {
			closeCalled = true
			return file.Close()
		}
		atomicFileWriteDeps.renameTempFile = func(string, string) error {
			renameCalled = true
			return nil
		}
		atomicFileWriteDeps.removeTempFile = func(path string) error {
			removeCalled = true
			return os.Remove(path)
		}

		targetPath := filepath.Join(t.TempDir(), "artifact.json")
		err := writeFileAtomically(targetPath, []byte(`{"value":1}`), 0o644)
		if err == nil {
			t.Fatal("expected writeFileAtomically to return write error")
		}
		if !strings.Contains(err.Error(), "write temp file") {
			t.Fatalf("error = %q, expected write-temp-file context", err)
		}
		if !errors.Is(err, writeErr) {
			t.Fatalf("error = %v, expected wrapped write error", err)
		}
		if !closeCalled {
			t.Fatal("expected temp file close on write error")
		}
		if renameCalled {
			t.Fatal("did not expect rename to be called after write error")
		}
		if !removeCalled {
			t.Fatal("expected temp file remove after write error")
		}
	})

	t.Run("treats partial writes as short-write errors", func(t *testing.T) {
		originalAtomicFileWriteDependencies := atomicFileWriteDeps
		t.Cleanup(func() {
			atomicFileWriteDeps = originalAtomicFileWriteDependencies
		})

		closeCalled := false
		renameCalled := false
		removeCalled := false
		atomicFileWriteDeps.writeAllBytesToTempFile = func(_ *os.File, fileContents []byte) (int, error) {
			return len(fileContents) - 1, nil
		}
		atomicFileWriteDeps.closeTempFile = func(file *os.File) error {
			closeCalled = true
			return file.Close()
		}
		atomicFileWriteDeps.renameTempFile = func(string, string) error {
			renameCalled = true
			return nil
		}
		atomicFileWriteDeps.removeTempFile = func(path string) error {
			removeCalled = true
			return os.Remove(path)
		}

		targetPath := filepath.Join(t.TempDir(), "artifact.json")
		err := writeFileAtomically(targetPath, []byte(`{"value":1}`), 0o644)
		if err == nil {
			t.Fatal("expected writeFileAtomically to return short-write error")
		}
		if !strings.Contains(err.Error(), "write temp file") {
			t.Fatalf("error = %q, expected write-temp-file context", err)
		}
		if !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("error = %v, expected wrapped short-write error", err)
		}
		if !closeCalled {
			t.Fatal("expected temp file close on short-write error")
		}
		if renameCalled {
			t.Fatal("did not expect rename after short-write error")
		}
		if !removeCalled {
			t.Fatal("expected temp file removal after short-write error")
		}
	})

	t.Run("closes and removes temp file when set-mode fails", func(t *testing.T) {
		originalAtomicFileWriteDependencies := atomicFileWriteDeps
		t.Cleanup(func() {
			atomicFileWriteDeps = originalAtomicFileWriteDependencies
		})

		modeErr := errors.New("chmod failed")
		closeCalled := false
		renameCalled := false
		removeCalled := false
		atomicFileWriteDeps.setTempFileMode = func(*os.File, os.FileMode) error {
			return modeErr
		}
		atomicFileWriteDeps.closeTempFile = func(file *os.File) error {
			closeCalled = true
			return file.Close()
		}
		atomicFileWriteDeps.renameTempFile = func(string, string) error {
			renameCalled = true
			return nil
		}
		atomicFileWriteDeps.removeTempFile = func(path string) error {
			removeCalled = true
			return os.Remove(path)
		}

		targetPath := filepath.Join(t.TempDir(), "artifact.json")
		err := writeFileAtomically(targetPath, []byte(`{"value":1}`), 0o644)
		if err == nil {
			t.Fatal("expected writeFileAtomically to return set-mode error")
		}
		if !strings.Contains(err.Error(), "set temp file mode") {
			t.Fatalf("error = %q, expected set-temp-file-mode context", err)
		}
		if !errors.Is(err, modeErr) {
			t.Fatalf("error = %v, expected wrapped set-mode error", err)
		}
		if !closeCalled {
			t.Fatal("expected close on set-mode error")
		}
		if renameCalled {
			t.Fatal("did not expect rename after set-mode error")
		}
		if !removeCalled {
			t.Fatal("expected temp file removal on set-mode error")
		}
	})

	t.Run("closes and removes temp file when sync fails", func(t *testing.T) {
		originalAtomicFileWriteDependencies := atomicFileWriteDeps
		t.Cleanup(func() {
			atomicFileWriteDeps = originalAtomicFileWriteDependencies
		})

		syncErr := errors.New("sync failed")
		closeCalled := false
		renameCalled := false
		removeCalled := false
		atomicFileWriteDeps.syncTempFileToDisk = func(*os.File) error {
			return syncErr
		}
		atomicFileWriteDeps.closeTempFile = func(file *os.File) error {
			closeCalled = true
			return file.Close()
		}
		atomicFileWriteDeps.renameTempFile = func(string, string) error {
			renameCalled = true
			return nil
		}
		atomicFileWriteDeps.removeTempFile = func(path string) error {
			removeCalled = true
			return os.Remove(path)
		}

		targetPath := filepath.Join(t.TempDir(), "artifact.json")
		err := writeFileAtomically(targetPath, []byte(`{"value":1}`), 0o644)
		if err == nil {
			t.Fatal("expected writeFileAtomically to return sync error")
		}
		if !strings.Contains(err.Error(), "sync temp file") {
			t.Fatalf("error = %q, expected sync-temp-file context", err)
		}
		if !errors.Is(err, syncErr) {
			t.Fatalf("error = %v, expected wrapped sync error", err)
		}
		if !closeCalled {
			t.Fatal("expected close on sync error")
		}
		if renameCalled {
			t.Fatal("did not expect rename after sync error")
		}
		if !removeCalled {
			t.Fatal("expected temp file removal on sync error")
		}
	})

	t.Run("returns close error before rename and removes temp file", func(t *testing.T) {
		originalAtomicFileWriteDependencies := atomicFileWriteDeps
		t.Cleanup(func() {
			atomicFileWriteDeps = originalAtomicFileWriteDependencies
		})

		closeErr := errors.New("close failed")
		renameCalled := false
		removeCalled := false
		atomicFileWriteDeps.closeTempFile = func(file *os.File) error {
			return errors.Join(closeErr, file.Close())
		}
		atomicFileWriteDeps.renameTempFile = func(string, string) error {
			renameCalled = true
			return nil
		}
		atomicFileWriteDeps.removeTempFile = func(path string) error {
			removeCalled = true
			return os.Remove(path)
		}

		targetPath := filepath.Join(t.TempDir(), "artifact.json")
		err := writeFileAtomically(targetPath, []byte(`{"value":1}`), 0o644)
		if err == nil {
			t.Fatal("expected writeFileAtomically to return close error")
		}
		if !strings.Contains(err.Error(), "close temp file") {
			t.Fatalf("error = %q, expected close-temp-file context", err)
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("error = %v, expected wrapped close error", err)
		}
		if renameCalled {
			t.Fatal("did not expect rename after close error")
		}
		if !removeCalled {
			t.Fatal("expected temp file removal after close error")
		}
	})

	t.Run("joins operation and close errors when cleanup close fails", func(t *testing.T) {
		originalAtomicFileWriteDependencies := atomicFileWriteDeps
		t.Cleanup(func() {
			atomicFileWriteDeps = originalAtomicFileWriteDependencies
		})

		writeErr := errors.New("write failed")
		closeErr := errors.New("close failed")
		removeCalled := false
		atomicFileWriteDeps.writeAllBytesToTempFile = func(*os.File, []byte) (int, error) {
			return 0, writeErr
		}
		atomicFileWriteDeps.closeTempFile = func(file *os.File) error {
			_ = file.Close()
			return closeErr
		}
		atomicFileWriteDeps.removeTempFile = func(path string) error {
			removeCalled = true
			return os.Remove(path)
		}

		targetPath := filepath.Join(t.TempDir(), "artifact.json")
		err := writeFileAtomically(targetPath, []byte(`{"value":1}`), 0o644)
		if err == nil {
			t.Fatal("expected writeFileAtomically to return joined write+close error")
		}
		if !strings.Contains(err.Error(), "write temp file") {
			t.Fatalf("error = %q, expected write-temp-file context", err)
		}
		if !errors.Is(err, writeErr) {
			t.Fatalf("error = %v, expected write error in joined chain", err)
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("error = %v, expected close error in joined chain", err)
		}
		if !removeCalled {
			t.Fatal("expected temp file removal when close-after-failure returns error")
		}
	})

	t.Run("retries rename after removing existing target when rename reports target exists", func(t *testing.T) {
		originalAtomicFileWriteDependencies := atomicFileWriteDeps
		t.Cleanup(func() {
			atomicFileWriteDeps = originalAtomicFileWriteDependencies
		})

		outputDirectory := t.TempDir()
		targetPath := filepath.Join(outputDirectory, "artifact.json")
		mustWriteFile(t, targetPath, []byte(`{"old":true}`))

		renameCalls := 0
		removeTargetCalls := 0
		atomicFileWriteDeps.renameTempFile = func(oldPath string, newPath string) error {
			renameCalls++
			if renameCalls == 1 {
				return os.ErrExist
			}
			return os.Rename(oldPath, newPath)
		}
		atomicFileWriteDeps.removeExistingTargetFile = func(path string) error {
			removeTargetCalls++
			return os.Remove(path)
		}

		if err := writeFileAtomically(targetPath, []byte(`{"new":true}`), 0o644); err != nil {
			t.Fatalf("writeFileAtomically returned error: %v", err)
		}
		if renameCalls != 2 {
			t.Fatalf("rename calls = %d, want %d", renameCalls, 2)
		}
		if removeTargetCalls != 1 {
			t.Fatalf("remove target calls = %d, want %d", removeTargetCalls, 1)
		}

		contents, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("read target file: %v", err)
		}
		if string(contents) != `{"new":true}` {
			t.Fatalf("target contents = %q, want %q", string(contents), `{"new":true}`)
		}
	})

	t.Run("returns remove-existing-target-file error when replace step fails", func(t *testing.T) {
		originalAtomicFileWriteDependencies := atomicFileWriteDeps
		t.Cleanup(func() {
			atomicFileWriteDeps = originalAtomicFileWriteDependencies
		})

		removeErr := errors.New("remove existing failed")
		atomicFileWriteDeps.renameTempFile = func(string, string) error {
			return os.ErrExist
		}
		atomicFileWriteDeps.removeExistingTargetFile = func(string) error {
			return removeErr
		}

		targetPath := filepath.Join(t.TempDir(), "artifact.json")
		err := writeFileAtomically(targetPath, []byte(`{"value":1}`), 0o644)
		if err == nil {
			t.Fatal("expected writeFileAtomically to return remove-existing-target error")
		}
		if !strings.Contains(err.Error(), "remove existing target file before rename") {
			t.Fatalf("error = %q, expected remove-existing-target context", err)
		}
		if !errors.Is(err, removeErr) {
			t.Fatalf("error = %v, expected wrapped remove-existing-target error", err)
		}
	})

	t.Run("returns second rename error after removing existing target", func(t *testing.T) {
		originalAtomicFileWriteDependencies := atomicFileWriteDeps
		t.Cleanup(func() {
			atomicFileWriteDeps = originalAtomicFileWriteDependencies
		})

		secondRenameErr := errors.New("second rename failed")
		renameCalls := 0
		atomicFileWriteDeps.renameTempFile = func(string, string) error {
			renameCalls++
			if renameCalls == 1 {
				return os.ErrExist
			}
			return secondRenameErr
		}
		atomicFileWriteDeps.removeExistingTargetFile = func(string) error {
			return nil
		}

		targetPath := filepath.Join(t.TempDir(), "artifact.json")
		err := writeFileAtomically(targetPath, []byte(`{"value":1}`), 0o644)
		if err == nil {
			t.Fatal("expected writeFileAtomically to return second rename error")
		}
		if !strings.Contains(err.Error(), "rename temp file after replacing existing target") {
			t.Fatalf("error = %q, expected second-rename context", err)
		}
		if !errors.Is(err, secondRenameErr) {
			t.Fatalf("error = %v, expected wrapped second rename error", err)
		}
	})
}

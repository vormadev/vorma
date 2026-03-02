package artifactio

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomically(t *testing.T) {
	t.Run(
		"replaces target file contents without leaving temp files",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			outputDirectory := t.TempDir()
			targetPath := filepath.Join(outputDirectory, "artifact.json")
			if writeErr := os.WriteFile(targetPath, []byte(`{"old":true}`), 0o644); writeErr != nil {
				t.Fatalf("write target file: %v", writeErr)
			}

			if err := writeFileAtomicallyWithDependencies(targetPath, []byte(`{"new":true}`), 0o644, atomicWriteDependencies); err != nil {
				t.Fatalf("writeFileAtomically returned error: %v", err)
			}

			rawBytes, err := os.ReadFile(targetPath)
			if err != nil {
				t.Fatalf("read target file: %v", err)
			}
			if string(rawBytes) != `{"new":true}` {
				t.Fatalf(
					"target file contents = %q, want %q",
					string(rawBytes),
					`{"new":true}`,
				)
			}

			tempFileMatches, err := filepath.Glob(
				filepath.Join(outputDirectory, AtomicFileWriteTempFilePattern),
			)
			if err != nil {
				t.Fatalf("glob temp files: %v", err)
			}
			if len(tempFileMatches) != 0 {
				t.Fatalf(
					"expected no temp files after atomic write, got %#v",
					tempFileMatches,
				)
			}
		},
	)

	t.Run("syncs parent directory after rename", func(t *testing.T) {
		atomicWriteDependencies := defaultAtomicFileWriteDependencies()
		originalAtomicFileWriteDependencies := atomicWriteDependencies
		t.Cleanup(func() {
			atomicWriteDependencies = originalAtomicFileWriteDependencies
		})

		outputDirectory := t.TempDir()
		targetPath := filepath.Join(outputDirectory, "artifact.json")

		openedParentDirectoryPath := ""
		syncParentDirectoryCalled := false
		closeParentDirectoryCalled := false
		atomicWriteDependencies.openParentDirectory = func(path string) (*os.File, error) {
			openedParentDirectoryPath = path
			return os.Open(path)
		}
		atomicWriteDependencies.syncParentDirectory = func(dir *os.File) error {
			syncParentDirectoryCalled = true
			return dir.Sync()
		}
		atomicWriteDependencies.closeParentDirectory = func(dir *os.File) error {
			closeParentDirectoryCalled = true
			return dir.Close()
		}

		if err := writeFileAtomicallyWithDependencies(targetPath, []byte(`{"new":true}`), 0o644, atomicWriteDependencies); err != nil {
			t.Fatalf("writeFileAtomically returned error: %v", err)
		}
		if openedParentDirectoryPath != outputDirectory {
			t.Fatalf(
				"opened parent directory path = %q, want %q",
				openedParentDirectoryPath,
				outputDirectory,
			)
		}
		if !syncParentDirectoryCalled {
			t.Fatal("expected parent directory sync after rename")
		}
		if !closeParentDirectoryCalled {
			t.Fatal("expected parent directory close after sync")
		}
	})

	t.Run(
		"returns open-parent-directory error after rename",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			outputDirectory := t.TempDir()
			targetPath := filepath.Join(outputDirectory, "artifact.json")
			if writeErr := os.WriteFile(targetPath, []byte(`{"old":true}`), 0o644); writeErr != nil {
				t.Fatalf("write target file: %v", writeErr)
			}

			openParentDirectoryErr := errors.New("open parent directory failed")
			atomicWriteDependencies.openParentDirectory = func(string) (*os.File, error) {
				return nil, openParentDirectoryErr
			}

			err := writeFileAtomicallyWithDependencies(
				targetPath,
				[]byte(`{"new":true}`),
				0o644,
				atomicWriteDependencies,
			)
			if err == nil {
				t.Fatal(
					"expected writeFileAtomically to return open-parent-directory error",
				)
			}
			if !strings.Contains(
				err.Error(),
				"open parent directory for sync",
			) {
				t.Fatalf(
					"error = %q, expected open-parent-directory context",
					err,
				)
			}
			if !errors.Is(err, openParentDirectoryErr) {
				t.Fatalf(
					"error = %v, expected wrapped open-parent-directory error",
					err,
				)
			}

			rawBytes, readErr := os.ReadFile(targetPath)
			if readErr != nil {
				t.Fatalf("read target file: %v", readErr)
			}
			if string(rawBytes) != `{"new":true}` {
				t.Fatalf(
					"target file contents = %q, want %q",
					string(rawBytes),
					`{"new":true}`,
				)
			}
		},
	)

	t.Run(
		"returns sync-parent-directory error and closes directory",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			outputDirectory := t.TempDir()
			targetPath := filepath.Join(outputDirectory, "artifact.json")

			syncParentDirectoryErr := errors.New("sync parent directory failed")
			closeParentDirectoryCalled := false
			atomicWriteDependencies.openParentDirectory = func(path string) (*os.File, error) {
				return os.Open(path)
			}
			atomicWriteDependencies.syncParentDirectory = func(*os.File) error {
				return syncParentDirectoryErr
			}
			atomicWriteDependencies.closeParentDirectory = func(dir *os.File) error {
				closeParentDirectoryCalled = true
				return dir.Close()
			}

			err := writeFileAtomicallyWithDependencies(
				targetPath,
				[]byte(`{"value":1}`),
				0o644,
				atomicWriteDependencies,
			)
			if err == nil {
				t.Fatal(
					"expected writeFileAtomically to return sync-parent-directory error",
				)
			}
			if !strings.Contains(err.Error(), "sync parent directory") {
				t.Fatalf(
					"error = %q, expected sync-parent-directory context",
					err,
				)
			}
			if !errors.Is(err, syncParentDirectoryErr) {
				t.Fatalf(
					"error = %v, expected wrapped sync-parent-directory error",
					err,
				)
			}
			if !closeParentDirectoryCalled {
				t.Fatal(
					"expected close parent directory call after sync failure",
				)
			}
		},
	)

	t.Run(
		"joins sync and close errors for parent directory",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			outputDirectory := t.TempDir()
			targetPath := filepath.Join(outputDirectory, "artifact.json")

			syncParentDirectoryErr := errors.New("sync parent directory failed")
			closeParentDirectoryErr := errors.New(
				"close parent directory failed",
			)
			atomicWriteDependencies.openParentDirectory = func(path string) (*os.File, error) {
				return os.Open(path)
			}
			atomicWriteDependencies.syncParentDirectory = func(*os.File) error {
				return syncParentDirectoryErr
			}
			atomicWriteDependencies.closeParentDirectory = func(dir *os.File) error {
				_ = dir.Close()
				return closeParentDirectoryErr
			}

			err := writeFileAtomicallyWithDependencies(
				targetPath,
				[]byte(`{"value":1}`),
				0o644,
				atomicWriteDependencies,
			)
			if err == nil {
				t.Fatal(
					"expected writeFileAtomically to return joined sync+close parent-directory errors",
				)
			}
			if !strings.Contains(err.Error(), "sync parent directory") {
				t.Fatalf(
					"error = %q, expected sync-parent-directory context",
					err,
				)
			}
			if !errors.Is(err, syncParentDirectoryErr) {
				t.Fatalf(
					"error = %v, expected sync-parent-directory error in joined chain",
					err,
				)
			}
			if !errors.Is(err, closeParentDirectoryErr) {
				t.Fatalf(
					"error = %v, expected close-parent-directory error in joined chain",
					err,
				)
			}
		},
	)

	t.Run(
		"returns close-parent-directory error when sync succeeds",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			outputDirectory := t.TempDir()
			targetPath := filepath.Join(outputDirectory, "artifact.json")

			closeParentDirectoryErr := errors.New(
				"close parent directory failed",
			)
			atomicWriteDependencies.openParentDirectory = func(path string) (*os.File, error) {
				return os.Open(path)
			}
			atomicWriteDependencies.syncParentDirectory = func(*os.File) error {
				return nil
			}
			atomicWriteDependencies.closeParentDirectory = func(dir *os.File) error {
				_ = dir.Close()
				return closeParentDirectoryErr
			}

			err := writeFileAtomicallyWithDependencies(
				targetPath,
				[]byte(`{"value":1}`),
				0o644,
				atomicWriteDependencies,
			)
			if err == nil {
				t.Fatal(
					"expected writeFileAtomically to return close-parent-directory error",
				)
			}
			if !strings.Contains(err.Error(), "close parent directory") {
				t.Fatalf(
					"error = %q, expected close-parent-directory context",
					err,
				)
			}
			if !errors.Is(err, closeParentDirectoryErr) {
				t.Fatalf(
					"error = %v, expected wrapped close-parent-directory error",
					err,
				)
			}
		},
	)

	t.Run("wraps create-target-directory errors", func(t *testing.T) {
		atomicWriteDependencies := defaultAtomicFileWriteDependencies()
		outputDirectory := t.TempDir()
		parentPath := filepath.Join(outputDirectory, "not-a-directory")
		if writeErr := os.WriteFile(parentPath, []byte("x"), 0o644); writeErr != nil {
			t.Fatalf("write parent marker file: %v", writeErr)
		}
		targetPath := filepath.Join(parentPath, "artifact.json")
		err := writeFileAtomicallyWithDependencies(
			targetPath,
			[]byte(`{}`),
			0o644,
			atomicWriteDependencies,
		)
		if err == nil {
			t.Fatal(
				"expected writeFileAtomically to return create-target-directory error",
			)
		}
		if !strings.Contains(err.Error(), "create target directory") {
			t.Fatalf("error = %q, expected create-target-directory context", err)
		}
	})

	t.Run("removes temp file when rename step fails", func(t *testing.T) {
		atomicWriteDependencies := defaultAtomicFileWriteDependencies()
		originalAtomicFileWriteDependencies := atomicWriteDependencies
		t.Cleanup(func() {
			atomicWriteDependencies = originalAtomicFileWriteDependencies
		})

		renameErr := errors.New("rename failed")
		var removedTempPath string
		atomicWriteDependencies.renameTempFile = func(string, string) error {
			return renameErr
		}
		atomicWriteDependencies.removeTempFile = func(path string) error {
			removedTempPath = path
			return os.Remove(path)
		}

		targetPath := filepath.Join(t.TempDir(), "artifact.json")
		err := writeFileAtomicallyWithDependencies(
			targetPath,
			[]byte(`{"value":1}`),
			0o644,
			atomicWriteDependencies,
		)
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
			t.Fatalf(
				"expected removed temp file to not exist, stat err=%v",
				statErr,
			)
		}
	})

	t.Run(
		"closes and removes temp file when write step fails",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			writeErr := errors.New("write failed")
			closeCalled := false
			renameCalled := false
			removeCalled := false
			atomicWriteDependencies.writeAllBytesToTempFile = func(*os.File, []byte) (int, error) {
				return 0, writeErr
			}
			atomicWriteDependencies.closeTempFile = func(file *os.File) error {
				closeCalled = true
				return file.Close()
			}
			atomicWriteDependencies.renameTempFile = func(string, string) error {
				renameCalled = true
				return nil
			}
			atomicWriteDependencies.removeTempFile = func(path string) error {
				removeCalled = true
				return os.Remove(path)
			}

			targetPath := filepath.Join(t.TempDir(), "artifact.json")
			err := writeFileAtomicallyWithDependencies(
				targetPath,
				[]byte(`{"value":1}`),
				0o644,
				atomicWriteDependencies,
			)
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
		},
	)

	t.Run("treats partial writes as short-write errors", func(t *testing.T) {
		atomicWriteDependencies := defaultAtomicFileWriteDependencies()
		originalAtomicFileWriteDependencies := atomicWriteDependencies
		t.Cleanup(func() {
			atomicWriteDependencies = originalAtomicFileWriteDependencies
		})

		closeCalled := false
		renameCalled := false
		removeCalled := false
		atomicWriteDependencies.writeAllBytesToTempFile = func(_ *os.File, fileContents []byte) (int, error) {
			return len(fileContents) - 1, nil
		}
		atomicWriteDependencies.closeTempFile = func(file *os.File) error {
			closeCalled = true
			return file.Close()
		}
		atomicWriteDependencies.renameTempFile = func(string, string) error {
			renameCalled = true
			return nil
		}
		atomicWriteDependencies.removeTempFile = func(path string) error {
			removeCalled = true
			return os.Remove(path)
		}

		targetPath := filepath.Join(t.TempDir(), "artifact.json")
		err := writeFileAtomicallyWithDependencies(
			targetPath,
			[]byte(`{"value":1}`),
			0o644,
			atomicWriteDependencies,
		)
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

	t.Run(
		"closes and removes temp file when set-mode fails",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			modeErr := errors.New("chmod failed")
			closeCalled := false
			renameCalled := false
			removeCalled := false
			atomicWriteDependencies.setTempFileMode = func(*os.File, os.FileMode) error {
				return modeErr
			}
			atomicWriteDependencies.closeTempFile = func(file *os.File) error {
				closeCalled = true
				return file.Close()
			}
			atomicWriteDependencies.renameTempFile = func(string, string) error {
				renameCalled = true
				return nil
			}
			atomicWriteDependencies.removeTempFile = func(path string) error {
				removeCalled = true
				return os.Remove(path)
			}

			targetPath := filepath.Join(t.TempDir(), "artifact.json")
			err := writeFileAtomicallyWithDependencies(
				targetPath,
				[]byte(`{"value":1}`),
				0o644,
				atomicWriteDependencies,
			)
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
		},
	)

	t.Run("closes and removes temp file when sync fails", func(t *testing.T) {
		atomicWriteDependencies := defaultAtomicFileWriteDependencies()
		originalAtomicFileWriteDependencies := atomicWriteDependencies
		t.Cleanup(func() {
			atomicWriteDependencies = originalAtomicFileWriteDependencies
		})

		syncErr := errors.New("sync failed")
		closeCalled := false
		renameCalled := false
		removeCalled := false
		atomicWriteDependencies.syncTempFileToDisk = func(*os.File) error {
			return syncErr
		}
		atomicWriteDependencies.closeTempFile = func(file *os.File) error {
			closeCalled = true
			return file.Close()
		}
		atomicWriteDependencies.renameTempFile = func(string, string) error {
			renameCalled = true
			return nil
		}
		atomicWriteDependencies.removeTempFile = func(path string) error {
			removeCalled = true
			return os.Remove(path)
		}

		targetPath := filepath.Join(t.TempDir(), "artifact.json")
		err := writeFileAtomicallyWithDependencies(
			targetPath,
			[]byte(`{"value":1}`),
			0o644,
			atomicWriteDependencies,
		)
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

	t.Run(
		"returns close error before rename and removes temp file",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			closeErr := errors.New("close failed")
			renameCalled := false
			removeCalled := false
			atomicWriteDependencies.closeTempFile = func(file *os.File) error {
				return errors.Join(closeErr, file.Close())
			}
			atomicWriteDependencies.renameTempFile = func(string, string) error {
				renameCalled = true
				return nil
			}
			atomicWriteDependencies.removeTempFile = func(path string) error {
				removeCalled = true
				return os.Remove(path)
			}

			targetPath := filepath.Join(t.TempDir(), "artifact.json")
			err := writeFileAtomicallyWithDependencies(
				targetPath,
				[]byte(`{"value":1}`),
				0o644,
				atomicWriteDependencies,
			)
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
		},
	)

	t.Run(
		"joins operation and close errors when cleanup close fails",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			writeErr := errors.New("write failed")
			closeErr := errors.New("close failed")
			removeCalled := false
			atomicWriteDependencies.writeAllBytesToTempFile = func(*os.File, []byte) (int, error) {
				return 0, writeErr
			}
			atomicWriteDependencies.closeTempFile = func(file *os.File) error {
				_ = file.Close()
				return closeErr
			}
			atomicWriteDependencies.removeTempFile = func(path string) error {
				removeCalled = true
				return os.Remove(path)
			}

			targetPath := filepath.Join(t.TempDir(), "artifact.json")
			err := writeFileAtomicallyWithDependencies(
				targetPath,
				[]byte(`{"value":1}`),
				0o644,
				atomicWriteDependencies,
			)
			if err == nil {
				t.Fatal(
					"expected writeFileAtomically to return joined write+close error",
				)
			}
			if !strings.Contains(err.Error(), "write temp file") {
				t.Fatalf("error = %q, expected write-temp-file context", err)
			}
			if !errors.Is(err, writeErr) {
				t.Fatalf(
					"error = %v, expected write error in joined chain",
					err,
				)
			}
			if !errors.Is(err, closeErr) {
				t.Fatalf(
					"error = %v, expected close error in joined chain",
					err,
				)
			}
			if !removeCalled {
				t.Fatal(
					"expected temp file removal when close-after-failure returns error",
				)
			}
		},
	)

	t.Run(
		"retries rename after removing existing target when rename reports target exists",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			outputDirectory := t.TempDir()
			targetPath := filepath.Join(outputDirectory, "artifact.json")
			if writeErr := os.WriteFile(targetPath, []byte(`{"old":true}`), 0o644); writeErr != nil {
				t.Fatalf("write target file: %v", writeErr)
			}

			renameCalls := 0
			removeTargetCalls := 0
			atomicWriteDependencies.renameTempFile = func(oldPath string, newPath string) error {
				renameCalls++
				if renameCalls == 1 {
					return os.ErrExist
				}
				return os.Rename(oldPath, newPath)
			}
			atomicWriteDependencies.removeExistingTargetFile = func(path string) error {
				removeTargetCalls++
				return os.Remove(path)
			}

			if err := writeFileAtomicallyWithDependencies(targetPath, []byte(`{"new":true}`), 0o644, atomicWriteDependencies); err != nil {
				t.Fatalf("writeFileAtomically returned error: %v", err)
			}
			if renameCalls != 2 {
				t.Fatalf("rename calls = %d, want %d", renameCalls, 2)
			}
			if removeTargetCalls != 1 {
				t.Fatalf(
					"remove target calls = %d, want %d",
					removeTargetCalls,
					1,
				)
			}

			contents, err := os.ReadFile(targetPath)
			if err != nil {
				t.Fatalf("read target file: %v", err)
			}
			if string(contents) != `{"new":true}` {
				t.Fatalf(
					"target contents = %q, want %q",
					string(contents),
					`{"new":true}`,
				)
			}
		},
	)

	t.Run(
		"retries rename after removing existing target when rename reports permission denied",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			outputDirectory := t.TempDir()
			targetPath := filepath.Join(outputDirectory, "artifact.json")
			if writeErr := os.WriteFile(targetPath, []byte(`{"old":true}`), 0o644); writeErr != nil {
				t.Fatalf("write target file: %v", writeErr)
			}

			renameCalls := 0
			removeTargetCalls := 0
			atomicWriteDependencies.renameTempFile = func(oldPath string, newPath string) error {
				renameCalls++
				if renameCalls == 1 {
					return os.ErrPermission
				}
				return os.Rename(oldPath, newPath)
			}
			atomicWriteDependencies.removeExistingTargetFile = func(path string) error {
				removeTargetCalls++
				return os.Remove(path)
			}

			if err := writeFileAtomicallyWithDependencies(targetPath, []byte(`{"new":true}`), 0o644, atomicWriteDependencies); err != nil {
				t.Fatalf("writeFileAtomically returned error: %v", err)
			}
			if renameCalls != 2 {
				t.Fatalf("rename calls = %d, want %d", renameCalls, 2)
			}
			if removeTargetCalls != 1 {
				t.Fatalf(
					"remove target calls = %d, want %d",
					removeTargetCalls,
					1,
				)
			}

			contents, err := os.ReadFile(targetPath)
			if err != nil {
				t.Fatalf("read target file: %v", err)
			}
			if string(contents) != `{"new":true}` {
				t.Fatalf(
					"target contents = %q, want %q",
					string(contents),
					`{"new":true}`,
				)
			}
		},
	)

	t.Run(
		"does not replace target when rename reports permission denied and target is absent",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			removeTargetCalls := 0
			renameCalls := 0
			atomicWriteDependencies.renameTempFile = func(string, string) error {
				renameCalls++
				return os.ErrPermission
			}
			atomicWriteDependencies.removeExistingTargetFile = func(string) error {
				removeTargetCalls++
				return nil
			}

			targetPath := filepath.Join(t.TempDir(), "missing-target.json")
			err := writeFileAtomicallyWithDependencies(
				targetPath,
				[]byte(`{"value":1}`),
				0o644,
				atomicWriteDependencies,
			)
			if err == nil {
				t.Fatal(
					"expected permission-denied rename error when target is absent",
				)
			}
			if !strings.Contains(err.Error(), "rename temp file") {
				t.Fatalf("error = %q, expected rename context", err)
			}
			if !errors.Is(err, os.ErrPermission) {
				t.Fatalf("error = %v, expected wrapped permission error", err)
			}
			if removeTargetCalls != 0 {
				t.Fatalf("remove target calls = %d, want 0", removeTargetCalls)
			}
			if renameCalls != 1 {
				t.Fatalf("rename calls = %d, want 1", renameCalls)
			}
		},
	)

	t.Run(
		"returns remove-existing-target-file error when replace step fails",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			removeErr := errors.New("remove existing failed")
			atomicWriteDependencies.renameTempFile = func(string, string) error {
				return os.ErrExist
			}
			atomicWriteDependencies.removeExistingTargetFile = func(string) error {
				return removeErr
			}

			targetPath := filepath.Join(t.TempDir(), "artifact.json")
			err := writeFileAtomicallyWithDependencies(
				targetPath,
				[]byte(`{"value":1}`),
				0o644,
				atomicWriteDependencies,
			)
			if err == nil {
				t.Fatal(
					"expected writeFileAtomically to return remove-existing-target error",
				)
			}
			if !strings.Contains(
				err.Error(),
				"remove existing target file before rename",
			) {
				t.Fatalf(
					"error = %q, expected remove-existing-target context",
					err,
				)
			}
			if !errors.Is(err, removeErr) {
				t.Fatalf(
					"error = %v, expected wrapped remove-existing-target error",
					err,
				)
			}
		},
	)

	t.Run(
		"returns second rename error after removing existing target",
		func(t *testing.T) {
			atomicWriteDependencies := defaultAtomicFileWriteDependencies()
			originalAtomicFileWriteDependencies := atomicWriteDependencies
			t.Cleanup(func() {
				atomicWriteDependencies = originalAtomicFileWriteDependencies
			})

			secondRenameErr := errors.New("second rename failed")
			renameCalls := 0
			atomicWriteDependencies.renameTempFile = func(string, string) error {
				renameCalls++
				if renameCalls == 1 {
					return os.ErrExist
				}
				return secondRenameErr
			}
			atomicWriteDependencies.removeExistingTargetFile = func(string) error {
				return nil
			}

			targetPath := filepath.Join(t.TempDir(), "artifact.json")
			err := writeFileAtomicallyWithDependencies(
				targetPath,
				[]byte(`{"value":1}`),
				0o644,
				atomicWriteDependencies,
			)
			if err == nil {
				t.Fatal(
					"expected writeFileAtomically to return second rename error",
				)
			}
			if !strings.Contains(
				err.Error(),
				"rename temp file after replacing existing target",
			) {
				t.Fatalf("error = %q, expected second-rename context", err)
			}
			if !errors.Is(err, secondRenameErr) {
				t.Fatalf(
					"error = %v, expected wrapped second rename error",
					err,
				)
			}
		},
	)
}

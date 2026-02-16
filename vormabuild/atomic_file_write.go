package vormabuild

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type atomicFileWriteDependencies struct {
	createTempFileInDirectory func(string, string) (*os.File, error)
	writeAllBytesToTempFile   func(*os.File, []byte) (int, error)
	setTempFileMode           func(*os.File, fs.FileMode) error
	syncTempFileToDisk        func(*os.File) error
	closeTempFile             func(*os.File) error
	renameTempFile            func(string, string) error
	removeExistingTargetFile  func(string) error
	removeTempFile            func(string) error
	openParentDirectory       func(string) (*os.File, error)
	syncParentDirectory       func(*os.File) error
	closeParentDirectory      func(*os.File) error
}

var atomicFileWriteDeps = atomicFileWriteDependencies{
	createTempFileInDirectory: os.CreateTemp,
	writeAllBytesToTempFile: func(file *os.File, fileContents []byte) (int, error) {
		return file.Write(fileContents)
	},
	setTempFileMode: func(file *os.File, fileMode fs.FileMode) error {
		return file.Chmod(fileMode)
	},
	syncTempFileToDisk: func(file *os.File) error {
		return file.Sync()
	},
	closeTempFile:            func(file *os.File) error { return file.Close() },
	renameTempFile:           os.Rename,
	removeExistingTargetFile: os.Remove,
	removeTempFile:           os.Remove,
	openParentDirectory:      os.Open,
	syncParentDirectory: func(dir *os.File) error {
		return dir.Sync()
	},
	closeParentDirectory: func(dir *os.File) error {
		return dir.Close()
	},
}

const atomicFileWriteTempFilePattern = ".vorma-atomic-write-*"

func writeFileAtomically(
	targetPath string,
	fileContents []byte,
	fileMode fs.FileMode,
) error {
	tempFile, err := atomicFileWriteDeps.createTempFileInDirectory(
		filepath.Dir(targetPath),
		atomicFileWriteTempFilePattern,
	)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tempPath := tempFile.Name()
	shouldRemoveTempPath := true
	defer func() {
		if shouldRemoveTempPath {
			_ = atomicFileWriteDeps.removeTempFile(tempPath)
		}
	}()

	bytesWritten, err := atomicFileWriteDeps.writeAllBytesToTempFile(tempFile, fileContents)
	if err != nil {
		return closeTempFileAfterAtomicWriteFailure(tempFile, err, "write temp file")
	}
	if bytesWritten != len(fileContents) {
		return closeTempFileAfterAtomicWriteFailure(tempFile, io.ErrShortWrite, "write temp file")
	}

	if err := atomicFileWriteDeps.setTempFileMode(tempFile, fileMode); err != nil {
		return closeTempFileAfterAtomicWriteFailure(tempFile, err, "set temp file mode")
	}

	if err := atomicFileWriteDeps.syncTempFileToDisk(tempFile); err != nil {
		return closeTempFileAfterAtomicWriteFailure(tempFile, err, "sync temp file")
	}

	if err := atomicFileWriteDeps.closeTempFile(tempFile); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := renameAtomicWriteTempPath(tempPath, targetPath); err != nil {
		return err
	}
	if err := syncParentDirectoryAfterAtomicRename(targetPath); err != nil {
		return err
	}

	shouldRemoveTempPath = false
	return nil
}

func renameAtomicWriteTempPath(tempPath string, targetPath string) error {
	renameErr := atomicFileWriteDeps.renameTempFile(tempPath, targetPath)
	if renameErr == nil {
		return nil
	}

	if !errors.Is(renameErr, fs.ErrExist) && !errors.Is(renameErr, os.ErrExist) {
		return fmt.Errorf("rename temp file: %w", renameErr)
	}

	removeExistingFileErr := atomicFileWriteDeps.removeExistingTargetFile(targetPath)
	if removeExistingFileErr != nil && !os.IsNotExist(removeExistingFileErr) {
		return fmt.Errorf("remove existing target file before rename: %w", removeExistingFileErr)
	}

	renameAfterRemoveErr := atomicFileWriteDeps.renameTempFile(tempPath, targetPath)
	if renameAfterRemoveErr != nil {
		return fmt.Errorf("rename temp file after replacing existing target: %w", renameAfterRemoveErr)
	}

	return nil
}

func closeTempFileAfterAtomicWriteFailure(
	tempFile *os.File,
	operationError error,
	operationContext string,
) error {
	closeErr := atomicFileWriteDeps.closeTempFile(tempFile)
	if closeErr != nil {
		return fmt.Errorf(
			"%s: %w",
			operationContext,
			errors.Join(operationError, fmt.Errorf("close temp file: %w", closeErr)),
		)
	}

	return fmt.Errorf("%s: %w", operationContext, operationError)
}

func syncParentDirectoryAfterAtomicRename(targetPath string) error {
	parentDirectoryPath := filepath.Dir(targetPath)
	parentDirectory, err := atomicFileWriteDeps.openParentDirectory(parentDirectoryPath)
	if err != nil {
		return fmt.Errorf("open parent directory for sync: %w", err)
	}

	syncErr := atomicFileWriteDeps.syncParentDirectory(parentDirectory)
	closeErr := atomicFileWriteDeps.closeParentDirectory(parentDirectory)
	if syncErr == nil && closeErr == nil {
		return nil
	}

	if syncErr != nil && closeErr != nil {
		return fmt.Errorf(
			"sync parent directory: %w",
			errors.Join(syncErr, fmt.Errorf("close parent directory: %w", closeErr)),
		)
	}
	if syncErr != nil {
		return fmt.Errorf("sync parent directory: %w", syncErr)
	}
	return fmt.Errorf("close parent directory: %w", closeErr)
}

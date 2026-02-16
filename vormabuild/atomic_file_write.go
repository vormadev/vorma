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

type atomicFileWriteExecutor struct {
	dependencies atomicFileWriteDependencies
}

const atomicFileWriteTempFilePattern = ".vorma-atomic-write-*"

func defaultAtomicFileWriteDependencies() atomicFileWriteDependencies {
	return atomicFileWriteDependencies{
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
}

func normalizeAtomicFileWriteDependencies(
	dependencies atomicFileWriteDependencies,
) atomicFileWriteDependencies {
	defaultDependencies := defaultAtomicFileWriteDependencies()
	if dependencies.createTempFileInDirectory == nil {
		dependencies.createTempFileInDirectory = defaultDependencies.createTempFileInDirectory
	}
	if dependencies.writeAllBytesToTempFile == nil {
		dependencies.writeAllBytesToTempFile = defaultDependencies.writeAllBytesToTempFile
	}
	if dependencies.setTempFileMode == nil {
		dependencies.setTempFileMode = defaultDependencies.setTempFileMode
	}
	if dependencies.syncTempFileToDisk == nil {
		dependencies.syncTempFileToDisk = defaultDependencies.syncTempFileToDisk
	}
	if dependencies.closeTempFile == nil {
		dependencies.closeTempFile = defaultDependencies.closeTempFile
	}
	if dependencies.renameTempFile == nil {
		dependencies.renameTempFile = defaultDependencies.renameTempFile
	}
	if dependencies.removeExistingTargetFile == nil {
		dependencies.removeExistingTargetFile = defaultDependencies.removeExistingTargetFile
	}
	if dependencies.removeTempFile == nil {
		dependencies.removeTempFile = defaultDependencies.removeTempFile
	}
	if dependencies.openParentDirectory == nil {
		dependencies.openParentDirectory = defaultDependencies.openParentDirectory
	}
	if dependencies.syncParentDirectory == nil {
		dependencies.syncParentDirectory = defaultDependencies.syncParentDirectory
	}
	if dependencies.closeParentDirectory == nil {
		dependencies.closeParentDirectory = defaultDependencies.closeParentDirectory
	}
	return dependencies
}

func newAtomicFileWriteExecutor(
	dependencies atomicFileWriteDependencies,
) atomicFileWriteExecutor {
	return atomicFileWriteExecutor{
		dependencies: normalizeAtomicFileWriteDependencies(dependencies),
	}
}

var defaultAtomicFileWriteExecutor = newAtomicFileWriteExecutor(atomicFileWriteDependencies{})

func writeFileAtomically(
	targetPath string,
	fileContents []byte,
	fileMode fs.FileMode,
) error {
	return defaultAtomicFileWriteExecutor.writeFileAtomically(targetPath, fileContents, fileMode)
}

func writeFileAtomicallyWithDependencies(
	targetPath string,
	fileContents []byte,
	fileMode fs.FileMode,
	dependencies atomicFileWriteDependencies,
) error {
	return newAtomicFileWriteExecutor(dependencies).writeFileAtomically(targetPath, fileContents, fileMode)
}

func (executor atomicFileWriteExecutor) writeFileAtomically(
	targetPath string,
	fileContents []byte,
	fileMode fs.FileMode,
) error {
	tempFile, err := executor.dependencies.createTempFileInDirectory(
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
			_ = executor.dependencies.removeTempFile(tempPath)
		}
	}()

	bytesWritten, err := executor.dependencies.writeAllBytesToTempFile(tempFile, fileContents)
	if err != nil {
		return executor.closeTempFileAfterAtomicWriteFailure(tempFile, err, "write temp file")
	}
	if bytesWritten != len(fileContents) {
		return executor.closeTempFileAfterAtomicWriteFailure(tempFile, io.ErrShortWrite, "write temp file")
	}

	if err := executor.dependencies.setTempFileMode(tempFile, fileMode); err != nil {
		return executor.closeTempFileAfterAtomicWriteFailure(tempFile, err, "set temp file mode")
	}

	if err := executor.dependencies.syncTempFileToDisk(tempFile); err != nil {
		return executor.closeTempFileAfterAtomicWriteFailure(tempFile, err, "sync temp file")
	}

	if err := executor.dependencies.closeTempFile(tempFile); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := executor.renameAtomicWriteTempPath(tempPath, targetPath); err != nil {
		return err
	}
	if err := executor.syncParentDirectoryAfterAtomicRename(targetPath); err != nil {
		return err
	}

	shouldRemoveTempPath = false
	return nil
}

func (executor atomicFileWriteExecutor) renameAtomicWriteTempPath(
	tempPath string,
	targetPath string,
) error {
	renameErr := executor.dependencies.renameTempFile(tempPath, targetPath)
	if renameErr == nil {
		return nil
	}

	if !errors.Is(renameErr, fs.ErrExist) && !errors.Is(renameErr, os.ErrExist) {
		return fmt.Errorf("rename temp file: %w", renameErr)
	}

	removeExistingFileErr := executor.dependencies.removeExistingTargetFile(targetPath)
	if removeExistingFileErr != nil && !os.IsNotExist(removeExistingFileErr) {
		return fmt.Errorf("remove existing target file before rename: %w", removeExistingFileErr)
	}

	renameAfterRemoveErr := executor.dependencies.renameTempFile(tempPath, targetPath)
	if renameAfterRemoveErr != nil {
		return fmt.Errorf("rename temp file after replacing existing target: %w", renameAfterRemoveErr)
	}

	return nil
}

func (executor atomicFileWriteExecutor) closeTempFileAfterAtomicWriteFailure(
	tempFile *os.File,
	operationError error,
	operationContext string,
) error {
	closeErr := executor.dependencies.closeTempFile(tempFile)
	if closeErr != nil {
		return fmt.Errorf(
			"%s: %w",
			operationContext,
			errors.Join(operationError, fmt.Errorf("close temp file: %w", closeErr)),
		)
	}

	return fmt.Errorf("%s: %w", operationContext, operationError)
}

func (executor atomicFileWriteExecutor) syncParentDirectoryAfterAtomicRename(
	targetPath string,
) error {
	parentDirectoryPath := filepath.Dir(targetPath)
	parentDirectory, err := executor.dependencies.openParentDirectory(parentDirectoryPath)
	if err != nil {
		return fmt.Errorf("open parent directory for sync: %w", err)
	}

	syncErr := executor.dependencies.syncParentDirectory(parentDirectory)
	closeErr := executor.dependencies.closeParentDirectory(parentDirectory)
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

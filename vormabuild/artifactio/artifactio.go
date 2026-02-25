// Package artifactio centralizes low-level build artifact I/O primitives.
//
// Keeping atomic writes, artifact snapshot/restore behavior, filesystem summary
// hashing, and close-aware resource execution in one place avoids duplicating
// tricky filesystem edge-case handling across build orchestration code.
package artifactio

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

const (
	// AtomicFileWriteTempFilePattern is the temp filename glob used for atomic
	// write scratch files.
	AtomicFileWriteTempFilePattern = ".vorma-atomic-write-*"

	// BuildArtifactFileMode is the file mode used for generated build artifacts.
	BuildArtifactFileMode fs.FileMode = 0o644
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

var defaultAtomicFileWriteExecutor = newAtomicFileWriteExecutor(
	atomicFileWriteDependencies{},
)

// WriteFileAtomically writes file content through a temp file then renames
// into place and fsyncs the parent directory.
func WriteFileAtomically(
	targetPath string,
	fileContents []byte,
	fileMode fs.FileMode,
) error {
	return defaultAtomicFileWriteExecutor.writeFileAtomically(
		targetPath,
		fileContents,
		fileMode,
	)
}

func writeFileAtomicallyWithDependencies(
	targetPath string,
	fileContents []byte,
	fileMode fs.FileMode,
	dependencies atomicFileWriteDependencies,
) error {
	return newAtomicFileWriteExecutor(
		dependencies,
	).writeFileAtomically(targetPath, fileContents, fileMode)
}

func (executor atomicFileWriteExecutor) writeFileAtomically(
	targetPath string,
	fileContents []byte,
	fileMode fs.FileMode,
) error {
	tempFile, err := executor.dependencies.createTempFileInDirectory(
		filepath.Dir(targetPath),
		AtomicFileWriteTempFilePattern,
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

	bytesWritten, err := executor.dependencies.writeAllBytesToTempFile(
		tempFile,
		fileContents,
	)
	if err != nil {
		return executor.closeTempFileAfterAtomicWriteFailure(
			tempFile,
			err,
			"write temp file",
		)
	}
	if bytesWritten != len(fileContents) {
		return executor.closeTempFileAfterAtomicWriteFailure(
			tempFile,
			io.ErrShortWrite,
			"write temp file",
		)
	}

	if err := executor.dependencies.setTempFileMode(tempFile, fileMode); err != nil {
		return executor.closeTempFileAfterAtomicWriteFailure(
			tempFile,
			err,
			"set temp file mode",
		)
	}

	if err := executor.dependencies.syncTempFileToDisk(tempFile); err != nil {
		return executor.closeTempFileAfterAtomicWriteFailure(
			tempFile,
			err,
			"sync temp file",
		)
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

	if !shouldRetryRenameByReplacingTarget(renameErr, targetPath) {
		return fmt.Errorf("rename temp file: %w", renameErr)
	}

	removeExistingFileErr := executor.dependencies.removeExistingTargetFile(
		targetPath,
	)
	if removeExistingFileErr != nil && !os.IsNotExist(removeExistingFileErr) {
		return fmt.Errorf(
			"remove existing target file before rename: %w",
			removeExistingFileErr,
		)
	}

	renameAfterRemoveErr := executor.dependencies.renameTempFile(
		tempPath,
		targetPath,
	)
	if renameAfterRemoveErr != nil {
		return fmt.Errorf(
			"rename temp file after replacing existing target: %w",
			renameAfterRemoveErr,
		)
	}

	return nil
}

func shouldRetryRenameByReplacingTarget(
	renameErr error,
	targetPath string,
) bool {
	if errors.Is(renameErr, fs.ErrExist) || errors.Is(renameErr, os.ErrExist) {
		return true
	}

	if errors.Is(renameErr, fs.ErrPermission) ||
		errors.Is(renameErr, os.ErrPermission) {
		_, statErr := os.Stat(targetPath)
		return statErr == nil
	}

	return false
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
			errors.Join(
				operationError,
				fmt.Errorf("close temp file: %w", closeErr),
			),
		)
	}

	return fmt.Errorf("%s: %w", operationContext, operationError)
}

func (executor atomicFileWriteExecutor) syncParentDirectoryAfterAtomicRename(
	targetPath string,
) error {
	parentDirectoryPath := filepath.Dir(targetPath)
	parentDirectory, err := executor.dependencies.openParentDirectory(
		parentDirectoryPath,
	)
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
			errors.Join(
				syncErr,
				fmt.Errorf("close parent directory: %w", closeErr),
			),
		)
	}
	if syncErr != nil {
		return fmt.Errorf("sync parent directory: %w", syncErr)
	}
	return fmt.Errorf("close parent directory: %w", closeErr)
}

type closableResource interface {
	Close() error
}

// RunWithClosableResource executes runWithResource and joins any close failure.
func RunWithClosableResource[T closableResource](
	resource T,
	closeErrorContext string,
	runWithResource func(T) error,
) (operationErr error) {
	defer func() {
		operationErr = joinOperationErrorWithCloseError(
			operationErr,
			closeErrorContext,
			resource.Close(),
		)
	}()

	return runWithResource(resource)
}

func joinOperationErrorWithCloseError(
	operationErr error,
	closeErrorContext string,
	closeErr error,
) error {
	if closeErr == nil {
		return operationErr
	}

	closeErrWithContext := fmt.Errorf("%s: %w", closeErrorContext, closeErr)
	if operationErr == nil {
		return closeErrWithContext
	}
	return errors.Join(operationErr, closeErrWithContext)
}

// BuildArtifactFileSnapshot captures whether a prior artifact existed and what
// content must be restored on rollback.
type BuildArtifactFileSnapshot struct {
	existed bool
	content []byte
}

// Existed reports whether the artifact existed at snapshot capture time.
func (snapshot BuildArtifactFileSnapshot) Existed() bool {
	return snapshot.existed
}

// Content returns a defensive copy of captured artifact content.
func (snapshot BuildArtifactFileSnapshot) Content() []byte {
	return append([]byte(nil), snapshot.content...)
}

// CaptureBuildArtifactFile snapshots one artifact file for possible rollback.
func CaptureBuildArtifactFile(
	artifactPath string,
	readArtifactFile func(string) ([]byte, error),
) (BuildArtifactFileSnapshot, error) {
	artifactContent, err := readArtifactFile(artifactPath)
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, syscall.ENOTDIR) {
			return BuildArtifactFileSnapshot{}, nil
		}
		return BuildArtifactFileSnapshot{}, err
	}

	return BuildArtifactFileSnapshot{
		existed: true,
		content: artifactContent,
	}, nil
}

// RestoreBuildArtifactFile restores one previously captured artifact snapshot.
func RestoreBuildArtifactFile(
	artifactPath string,
	snapshot BuildArtifactFileSnapshot,
	writeArtifactFile func(string, []byte, os.FileMode) error,
	removeArtifactFile func(string) error,
) error {
	if snapshot.existed {
		return writeArtifactFile(
			artifactPath,
			snapshot.content,
			BuildArtifactFileMode,
		)
	}

	err := removeArtifactFile(artifactPath)
	if err == nil || os.IsNotExist(err) || errors.Is(err, syscall.ENOTDIR) {
		return nil
	}
	return err
}

type fsFileSummary struct {
	path string
	size int64
}

// GetFSSummaryHash returns a content+metadata hash for an fs.FS tree.
func GetFSSummaryHash(fsys fs.FS) ([]byte, error) {
	var fileSummaries []fsFileSummary
	err := fs.WalkDir(
		fsys,
		".",
		func(path string, dirEntry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == "." || dirEntry.IsDir() {
				return nil
			}
			info, err := dirEntry.Info()
			if err != nil {
				return err
			}
			fileSummaries = append(fileSummaries, fsFileSummary{
				path: path,
				size: info.Size(),
			})
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	sort.Slice(fileSummaries, func(i int, j int) bool {
		return fileSummaries[i].path < fileSummaries[j].path
	})

	hash := sha256.New()
	for _, fileSummary := range fileSummaries {
		hash.Write([]byte(fileSummary.path))
		hash.Write([]byte{0})

		sizeBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(sizeBytes, uint64(fileSummary.size))
		hash.Write(sizeBytes)
		hash.Write([]byte{0})

		fileContents, err := fs.ReadFile(fsys, fileSummary.path)
		if err != nil {
			return nil, fmt.Errorf(
				"read %s for FS summary hash: %w",
				fileSummary.path,
				err,
			)
		}
		hash.Write(fileContents)
		hash.Write([]byte{0})
	}
	return hash.Sum(nil), nil
}

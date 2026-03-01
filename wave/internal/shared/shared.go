// Package shared contains Wave-internal filesystem and process helpers used by
// both runtime and tooling paths.
//
// Centralizing these helpers avoids duplicating low-level lock, IO, and command
// behavior across higher-level packages.
package shared

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/vormadev/vorma/kit/lockfile"
)

// LockFileName is the canonical lock file used by wave dev orchestration.
const LockFileName = ".wave-dev.lock"

const (
	lockAcquireRetryLimit                 = 12
	lockAcquireRetryDelay                 = 20 * time.Millisecond
	invalidLockStaleThreshold             = 1200 * time.Millisecond
	fileWriteMode             fs.FileMode = 0o644
	directoryWriteMode        fs.FileMode = 0o755
)

// ErrLockHeld reports that another process currently owns the project lock.
var ErrLockHeld = errors.New("another wave dev process is already running")

// DevLock manages exclusive per-project ownership for development mode.
//
// It uses lease-based lock files so stale or suspended owners can be reclaimed.
type DevLock struct {
	lock *lockfile.PIDLock
}

// NewDevLock creates a lock rooted in the project's dist static directory.
func NewDevLock(distStaticDirectoryPath string) *DevLock {
	lockPath := filepath.Join(distStaticDirectoryPath, LockFileName)
	return &DevLock{
		lock: lockfile.NewPIDLockWithOptions(
			lockPath,
			lockfile.Options{
				HeldError:                    ErrLockHeld,
				AcquireRetryLimit:            lockAcquireRetryLimit,
				AcquireRetryDelay:            lockAcquireRetryDelay,
				InvalidPIDLockStaleThreshold: invalidLockStaleThreshold,
				FileWriteMode:                fileWriteMode,
				DirectoryWriteMode:           directoryWriteMode,
			},
		),
	}
}

// Path returns the full lock file path.
func (devLock *DevLock) Path() string {
	if devLock == nil || devLock.lock == nil {
		return ""
	}
	return devLock.lock.Path()
}

// Acquire obtains lock ownership or returns ErrLockHeld.
func (devLock *DevLock) Acquire() error {
	if devLock == nil || devLock.lock == nil {
		return errors.New("lock is nil")
	}
	return devLock.lock.Acquire()
}

// Release drops lock ownership by removing the lock file.
func (devLock *DevLock) Release() error {
	if devLock == nil || devLock.lock == nil {
		return nil
	}
	return devLock.lock.Release()
}

// Held reports whether Acquire has succeeded in-process and has not been released.
func (devLock *DevLock) Held() bool {
	if devLock == nil || devLock.lock == nil {
		return false
	}
	return devLock.lock.Held()
}

// IsLockFileName reports whether a filename is the Wave dev lock file.
func IsLockFileName(name string) bool {
	return strings.TrimSpace(name) == LockFileName
}

// IsLockFile reports whether a filename is the Wave dev lock file.
func IsLockFile(name string) bool {
	return IsLockFileName(name)
}

// ValidateNamedGlobPatternInput validates one labeled glob field.
func ValidateNamedGlobPatternInput(
	validationContextLabel string,
	fieldPath string,
	globPattern string,
) error {
	if strings.TrimSpace(globPattern) == "" {
		return fmt.Errorf(
			"%s: %s is required",
			validationContextLabel,
			fieldPath,
		)
	}
	if strings.TrimSpace(globPattern) != globPattern {
		return fmt.Errorf(
			"%s: %s must not include surrounding whitespace",
			validationContextLabel,
			fieldPath,
		)
	}

	normalizedGlobPattern := strings.ReplaceAll(globPattern, "\\", "/")
	if !doublestar.ValidatePattern(normalizedGlobPattern) {
		return fmt.Errorf(
			"%s: %s must be a valid glob pattern; got %q",
			validationContextLabel,
			fieldPath,
			globPattern,
		)
	}
	return nil
}

// NormalizePathForMatching normalizes filesystem paths for glob matching.
func NormalizePathForMatching(pathForNormalization string) string {
	if strings.TrimSpace(pathForNormalization) == "" {
		return ""
	}
	normalizedPath := filepath.Clean(pathForNormalization)
	return strings.ReplaceAll(normalizedPath, "\\", "/")
}

// NormalizeGlobPatternForMatching normalizes glob patterns for doublestar matching.
func NormalizeGlobPatternForMatching(patternForNormalization string) string {
	trimmedPattern := strings.TrimSpace(patternForNormalization)
	if trimmedPattern == "" {
		return ""
	}

	var normalizedPatternBuilder strings.Builder
	normalizedPatternBuilder.Grow(len(trimmedPattern))

	for index := 0; index < len(trimmedPattern); index++ {
		currentByte := trimmedPattern[index]
		if currentByte != '\\' {
			normalizedPatternBuilder.WriteByte(currentByte)
			continue
		}

		if index+1 < len(trimmedPattern) {
			nextByte := trimmedPattern[index+1]
			switch nextByte {
			case '\\', '*', '?', '[', ']', '{', '}':
				normalizedPatternBuilder.WriteByte('\\')
				normalizedPatternBuilder.WriteByte(nextByte)
				index++
				continue
			}
		}

		normalizedPatternBuilder.WriteByte('/')
	}

	return normalizedPatternBuilder.String()
}

// MatchPathAgainstGlob matches a normalized path against a normalized pattern.
func MatchPathAgainstGlob(pathForMatching string, globPattern string) bool {
	normalizedPath := NormalizePathForMatching(pathForMatching)
	normalizedPattern := NormalizeGlobPatternForMatching(globPattern)
	if normalizedPath == "" || normalizedPattern == "" {
		return false
	}
	matched, matchError := doublestar.PathMatch(
		normalizedPattern,
		normalizedPath,
	)
	if matchError != nil {
		return false
	}
	return matched
}

// IsPathWithinDirectory reports whether path is equal to or nested under directory.
func IsPathWithinDirectory(pathForCheck string, directoryPath string) bool {
	absolutePath, pathError := filepath.Abs(pathForCheck)
	if pathError != nil {
		return false
	}
	absoluteDirectory, directoryError := filepath.Abs(directoryPath)
	if directoryError != nil {
		return false
	}

	relativePath, relativeError := filepath.Rel(absoluteDirectory, absolutePath)
	if relativeError != nil {
		return false
	}
	if relativePath == "." {
		return true
	}
	return !strings.HasPrefix(relativePath, "..")
}

// EnsureDirectoryForFile ensures the parent directory exists for filePath.
func EnsureDirectoryForFile(filePath string) error {
	if strings.TrimSpace(filePath) == "" {
		return errors.New("file path is empty")
	}
	parentDirectory := filepath.Dir(filePath)
	if mkdirError := os.MkdirAll(parentDirectory, directoryWriteMode); mkdirError != nil {
		return fmt.Errorf(
			"create directory for file %q: %w",
			filePath,
			mkdirError,
		)
	}
	return nil
}

// WriteFileAtomically writes a file by temp-write + rename to avoid partial reads.
func WriteFileAtomically(path string, content []byte, mode fs.FileMode) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is empty")
	}
	if mode == 0 {
		mode = fileWriteMode
	}
	if ensureDirectoryError := EnsureDirectoryForFile(path); ensureDirectoryError != nil {
		return ensureDirectoryError
	}

	tempFile, createTempError := os.CreateTemp(
		filepath.Dir(path),
		filepath.Base(path)+".tmp-*",
	)
	if createTempError != nil {
		return fmt.Errorf(
			"create temporary file for %q: %w",
			path,
			createTempError,
		)
	}
	tempPath := tempFile.Name()
	cleanupTempFilePath := true
	defer func() {
		if cleanupTempFilePath {
			_ = os.Remove(tempPath)
		}
	}()

	if _, writeError := tempFile.Write(content); writeError != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temporary file %q: %w", tempPath, writeError)
	}
	if chmodError := tempFile.Chmod(mode); chmodError != nil {
		_ = tempFile.Close()
		return fmt.Errorf("chmod temporary file %q: %w", tempPath, chmodError)
	}
	if closeError := tempFile.Close(); closeError != nil {
		return fmt.Errorf("close temporary file %q: %w", tempPath, closeError)
	}
	if renameError := os.Rename(tempPath, path); renameError != nil {
		return fmt.Errorf(
			"rename temporary file %q to %q: %w",
			tempPath,
			path,
			renameError,
		)
	}
	cleanupTempFilePath = false
	return nil
}

// CopyFileAtomically copies src to dst through a temporary file and rename.
func CopyFileAtomically(sourcePath string, destinationPath string) error {
	sourceBytes, readError := os.ReadFile(sourcePath)
	if readError != nil {
		return fmt.Errorf("read source file %q: %w", sourcePath, readError)
	}

	sourceInfo, statError := os.Stat(sourcePath)
	if statError != nil {
		return fmt.Errorf("stat source file %q: %w", sourcePath, statError)
	}

	return WriteFileAtomically(destinationPath, sourceBytes, sourceInfo.Mode())
}

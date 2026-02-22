package shared

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bmatcuk/doublestar/v4"
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
// The lock file contains only the owner PID as text, and stale lock
// reclamation is guarded by compare-before-delete checks.
type DevLock struct {
	path string
	mu   sync.Mutex
	held bool
}

// lockFileSnapshot captures one stable read of lock file state.
type lockFileSnapshot struct {
	rawData   []byte
	pid       int
	pidParsed bool
	modified  time.Time
}

// NewDevLock creates a lock rooted in the project's dist static directory.
func NewDevLock(distStaticDirectoryPath string) *DevLock {
	return &DevLock{
		path: filepath.Join(distStaticDirectoryPath, LockFileName),
	}
}

// Path returns the full lock file path.
func (devLock *DevLock) Path() string {
	if devLock == nil {
		return ""
	}
	return devLock.path
}

// Acquire obtains lock ownership or returns ErrLockHeld.
func (devLock *DevLock) Acquire() error {
	if devLock == nil {
		return errors.New("lock is nil")
	}

	devLock.mu.Lock()
	defer devLock.mu.Unlock()

	if strings.TrimSpace(devLock.path) == "" {
		return errors.New("lock path is empty")
	}

	if ensureDirectoryError := os.MkdirAll(filepath.Dir(devLock.path), directoryWriteMode); ensureDirectoryError != nil {
		return fmt.Errorf("create lock directory: %w", ensureDirectoryError)
	}

	for attemptIndex := 0; attemptIndex < lockAcquireRetryLimit; attemptIndex++ {
		acquired, lockHeldError, acquireAttemptError := devLock.tryAcquireSingleAttempt()
		if acquireAttemptError != nil {
			return acquireAttemptError
		}
		if lockHeldError != nil {
			return lockHeldError
		}
		if acquired {
			devLock.held = true
			return nil
		}
		time.Sleep(lockAcquireRetryDelay)
	}

	return fmt.Errorf("acquire lock: contention exceeded retry budget")
}

// Release drops lock ownership by removing the lock file.
func (devLock *DevLock) Release() error {
	if devLock == nil {
		return nil
	}

	devLock.mu.Lock()
	defer devLock.mu.Unlock()

	removeError := os.Remove(devLock.path)
	if removeError != nil && !os.IsNotExist(removeError) {
		return fmt.Errorf("release lock: %w", removeError)
	}
	devLock.held = false
	return nil
}

// Held reports whether Acquire has succeeded in-process and has not been released.
func (devLock *DevLock) Held() bool {
	if devLock == nil {
		return false
	}
	devLock.mu.Lock()
	defer devLock.mu.Unlock()
	return devLock.held
}

// tryAcquireSingleAttempt tries O_EXCL create, then stale lock reclamation.
func (devLock *DevLock) tryAcquireSingleAttempt() (bool, error, error) {
	created, createError := devLock.tryCreateLockFileForCurrentProcess()
	if createError != nil {
		return false, nil, createError
	}
	if created {
		return true, nil, nil
	}

	snapshot, snapshotFound, snapshotError := devLock.readLockFileSnapshot()
	if snapshotError != nil {
		return false, nil, snapshotError
	}
	if !snapshotFound {
		return false, nil, nil
	}

	if snapshot.pidParsed {
		if processAppearsAlive(snapshot.pid) {
			return false, fmt.Errorf(
				"%w (pid %d)",
				ErrLockHeld,
				snapshot.pid,
			), nil
		}
	} else if !isInvalidPIDLockStale(snapshot.modified) {
		return false, fmt.Errorf("%w (pid unavailable)", ErrLockHeld), nil
	}

	removed, removeError := devLock.tryRemoveSnapshotIfUnchanged(snapshot)
	if removeError != nil {
		return false, nil, removeError
	}
	if !removed {
		return false, nil, nil
	}

	return false, nil, nil
}

// tryCreateLockFileForCurrentProcess performs one atomic lock-file create.
func (devLock *DevLock) tryCreateLockFileForCurrentProcess() (bool, error) {
	file, openError := os.OpenFile(
		devLock.path,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		fileWriteMode,
	)
	if openError != nil {
		if os.IsExist(openError) {
			return false, nil
		}
		return false, fmt.Errorf("create lock file: %w", openError)
	}

	written := false
	defer func() {
		if !written {
			_ = file.Close()
			_ = os.Remove(devLock.path)
		}
	}()

	if _, writeError := file.WriteString(strconv.Itoa(os.Getpid())); writeError != nil {
		return false, fmt.Errorf("write lock file pid: %w", writeError)
	}
	if closeError := file.Close(); closeError != nil {
		return false, fmt.Errorf("close lock file: %w", closeError)
	}
	written = true
	return true, nil
}

// readLockFileSnapshot loads lock content and metadata.
func (devLock *DevLock) readLockFileSnapshot() (lockFileSnapshot, bool, error) {
	lockRawData, readError := os.ReadFile(devLock.path)
	if readError != nil {
		if os.IsNotExist(readError) {
			return lockFileSnapshot{}, false, nil
		}
		return lockFileSnapshot{}, false, fmt.Errorf(
			"read lock file: %w",
			readError,
		)
	}

	lockFileInfo, statError := os.Stat(devLock.path)
	if statError != nil {
		if os.IsNotExist(statError) {
			return lockFileSnapshot{}, false, nil
		}
		return lockFileSnapshot{}, false, fmt.Errorf(
			"stat lock file: %w",
			statError,
		)
	}

	lockPID, lockPIDParsed := parsePIDFromLockData(lockRawData)
	return lockFileSnapshot{
		rawData:   lockRawData,
		pid:       lockPID,
		pidParsed: lockPIDParsed,
		modified:  lockFileInfo.ModTime(),
	}, true, nil
}

// tryRemoveSnapshotIfUnchanged removes the lock file only if content matches.
func (devLock *DevLock) tryRemoveSnapshotIfUnchanged(
	snapshot lockFileSnapshot,
) (bool, error) {
	currentRawData, readError := os.ReadFile(devLock.path)
	if readError != nil {
		if os.IsNotExist(readError) {
			return false, nil
		}
		return false, fmt.Errorf("re-read lock file: %w", readError)
	}

	if !bytes.Equal(currentRawData, snapshot.rawData) {
		return false, nil
	}

	if removeError := os.Remove(devLock.path); removeError != nil {
		if os.IsNotExist(removeError) {
			return false, nil
		}
		return false, fmt.Errorf("remove stale lock file: %w", removeError)
	}
	return true, nil
}

// parsePIDFromLockData parses pid text from lock file bytes.
func parsePIDFromLockData(lockRawData []byte) (int, bool) {
	lockPID, parseError := strconv.Atoi(strings.TrimSpace(string(lockRawData)))
	if parseError != nil || lockPID <= 0 {
		return 0, false
	}
	return lockPID, true
}

// isInvalidPIDLockStale reports whether invalid-pid lock data is old enough to reclaim.
func isInvalidPIDLockStale(lockModifiedAt time.Time) bool {
	return time.Since(lockModifiedAt) >= invalidLockStaleThreshold
}

// processAppearsAlive reports whether a process should be treated as active.
func processAppearsAlive(processID int) bool {
	process, findError := os.FindProcess(processID)
	if findError != nil {
		return false
	}

	signalError := process.Signal(syscall.Signal(0))
	if signalError == nil {
		return true
	}
	if errors.Is(signalError, os.ErrProcessDone) {
		return false
	}

	errorString := strings.ToLower(signalError.Error())
	if runtime.GOOS == "windows" {
		if strings.Contains(errorString, "access is denied") {
			return true
		}
		if strings.Contains(errorString, "operation completed successfully") {
			return true
		}
	}
	if strings.Contains(errorString, "operation not permitted") {
		return true
	}

	return false
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

	tempPath := path + ".tmp"
	if writeError := os.WriteFile(tempPath, content, mode); writeError != nil {
		return fmt.Errorf("write temporary file %q: %w", tempPath, writeError)
	}
	if renameError := os.Rename(tempPath, path); renameError != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf(
			"rename temporary file %q to %q: %w",
			tempPath,
			path,
			renameError,
		)
	}
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

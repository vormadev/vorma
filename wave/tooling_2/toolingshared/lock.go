package toolingshared

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

const lockFileName = ".wave-dev.lock"
const maxLockAcquireAttempts = 8
const invalidLockFileStaleThreshold = 1 * time.Second

// ErrLockHeld is returned when another wave dev instance is running on this project.
var ErrLockHeld = errors.New(
	"another wave dev instance is running on this project",
)

// DevLock manages a project-level lock to prevent multiple Wave dev instances.
type DevLock struct {
	path string
}

func NewDevLock(distStaticDir string) *DevLock {
	return &DevLock{
		path: filepath.Join(distStaticDir, lockFileName),
	}
}

// Acquire attempts to acquire the dev lock.
// Returns ErrLockHeld (wrapped with PID info) if another instance is running.
// Handles stale locks from crashed processes.
func (l *DevLock) Acquire() error {
	// Ensure parent directory exists (handles fresh clones)
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create lock directory: %w", err)
	}

	for attempt := 0; attempt < maxLockAcquireAttempts; attempt++ {
		created, err := l.tryCreateLockFileWithCurrentPID()
		if err != nil {
			return err
		}
		if created {
			return nil
		}

		_, lockHeldErr, err := l.tryRecoverStaleLockFile()
		if err != nil {
			return err
		}
		if lockHeldErr != nil {
			return lockHeldErr
		}
	}

	return fmt.Errorf(
		"acquire lock file: lock contention exceeded retry budget",
	)
}

func (l *DevLock) tryCreateLockFileWithCurrentPID() (bool, error) {
	file, err := os.OpenFile(l.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("create lock file: %w", err)
	}

	createdSuccessfully := false
	defer func() {
		if !createdSuccessfully {
			_ = file.Close()
			_ = os.Remove(l.path)
		}
	}()

	if _, err := file.WriteString(strconv.Itoa(os.Getpid())); err != nil {
		return false, fmt.Errorf("write lock file: %w", err)
	}
	if err := file.Close(); err != nil {
		return false, fmt.Errorf("close lock file: %w", err)
	}

	createdSuccessfully = true
	return true, nil
}

func (l *DevLock) tryRecoverStaleLockFile() (bool, error, error) {
	lockData, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("read lock file: %w", err)
	}

	lockFileInfo, err := os.Stat(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("stat lock file: %w", err)
	}

	lockPID, lockPIDParsed := parseLockPID(lockData)
	if lockPIDParsed && isProcessRunning(lockPID) {
		return false, fmt.Errorf("%w (PID %d)", ErrLockHeld, lockPID), nil
	}
	if !lockPIDParsed && !isStaleInvalidLockFile(lockFileInfo.ModTime()) {
		return false, fmt.Errorf("%w (PID unavailable)", ErrLockHeld), nil
	}

	lockDataStillUnchanged, err := l.lockFileDataMatches(lockData)
	if err != nil {
		return false, nil, err
	}
	if !lockDataStillUnchanged {
		return false, nil, nil
	}

	if err := os.Remove(l.path); err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("remove stale lock file: %w", err)
	}

	return true, nil, nil
}

func (l *DevLock) lockFileDataMatches(expected []byte) (bool, error) {
	lockData, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("re-read lock file: %w", err)
	}

	return bytes.Equal(lockData, expected), nil
}

func parseLockPID(lockData []byte) (int, bool) {
	lockPID, err := strconv.Atoi(strings.TrimSpace(string(lockData)))
	if err != nil || lockPID <= 0 {
		return 0, false
	}
	return lockPID, true
}

func isStaleInvalidLockFile(lockFileModificationTime time.Time) bool {
	return time.Since(lockFileModificationTime) >= invalidLockFileStaleThreshold
}

// isProcessRunning reports whether pid appears to still be alive.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

// Release removes the lock file.
func (l *DevLock) Release() error {
	err := os.Remove(l.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Path returns the lock file path.
func (l *DevLock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// IsLockFile returns true if the filename is a wave lock file.
func IsLockFile(name string) bool {
	return name == lockFileName
}

func ValidateNamedGlobPatternInput(
	validationContextLabel string,
	fieldPath string,
	globPattern string,
) error {
	if strings.TrimSpace(globPattern) == "" {
		return fmt.Errorf("%s: %s is required", validationContextLabel, fieldPath)
	}
	if strings.TrimSpace(globPattern) != globPattern {
		return fmt.Errorf(
			"%s: %s must not include surrounding whitespace",
			validationContextLabel,
			fieldPath,
		)
	}

	normalizedPattern := strings.ReplaceAll(globPattern, "\\", "/")
	if !doublestar.ValidatePattern(normalizedPattern) {
		return fmt.Errorf(
			"%s: %s must be a valid glob pattern; got %q",
			validationContextLabel,
			fieldPath,
			globPattern,
		)
	}

	return nil
}

package tooling

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const lockFileName = ".wave-dev.lock"

// ErrLockHeld is returned when another wave dev instance is running on this project.
var ErrLockHeld = errors.New("another wave dev instance is running on this project")

// devLock manages a project-level lock to prevent multiple wave dev instances.
type devLock struct {
	path string
}

func newDevLock(distStaticDir string) *devLock {
	return &devLock{
		path: filepath.Join(distStaticDir, lockFileName),
	}
}

// acquire attempts to acquire the dev lock.
// Returns ErrLockHeld (wrapped with PID info) if another instance is running.
// Handles stale locks from crashed processes.
func (l *devLock) acquire() error {
	// Ensure parent directory exists (handles fresh clones)
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create lock directory: %w", err)
	}

	// Check for existing lock
	data, err := os.ReadFile(l.path)
	if err == nil {
		// Lock file exists, check if process is still running
		pidStr := strings.TrimSpace(string(data))
		pid, parseErr := strconv.Atoi(pidStr)
		if parseErr == nil && pid > 0 {
			if isProcessRunning(pid) {
				return fmt.Errorf("%w (PID %d)", ErrLockHeld, pid)
			}
			// Process is dead, stale lock - safe to take over
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read lock file: %w", err)
	}

	// Write our PID
	pid := os.Getpid()
	if err := os.WriteFile(l.path, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("write lock file: %w", err)
	}

	return nil
}

// release removes the lock file.
func (l *devLock) release() error {
	return os.Remove(l.path)
}

// isLockFile returns true if the filename is a wave lock file.
func isLockFile(name string) bool {
	return strings.HasPrefix(name, ".wave-")
}

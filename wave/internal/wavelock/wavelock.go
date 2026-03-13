// Package wavelock owns Wave's project-level lock lifecycle for build and dev
// orchestration.
//
// Keeping lock behavior in one package avoids divergent lock semantics between
// devserver and builder flows.
package wavelock

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/lockfile"
)

// LockFileName is the canonical lock file used by wave dev orchestration.
const LockFileName = ".wave-dev.lock"

// BuildLockFileName is the canonical lock file used by full build
// orchestration.
const BuildLockFileName = ".wave-build.lock"

const (
	lockAcquireRetryLimit                 = 12
	lockAcquireRetryDelay                 = 20 * time.Millisecond
	invalidLockStaleThreshold             = 1200 * time.Millisecond
	fileWriteMode             fs.FileMode = 0o644
	directoryWriteMode        fs.FileMode = 0o755
)

// ErrLockHeld reports that another process currently owns the project lock.
var ErrLockHeld = errors.New("another wave dev process is already running")

// DevLock manages exclusive per-project ownership for development/build mode.
//
// It uses lease-based lock files so stale or suspended owners can be
// reclaimed.
type DevLock struct {
	lock *lockfile.PIDLock
}

func newProjectLock(
	distStaticDirectoryPath string,
	lockFileName string,
) *DevLock {
	lockPath := filepath.Join(distStaticDirectoryPath, lockFileName)
	return &DevLock{
		lock: lockfile.NewPIDLockWithOptions(
			lockPath,
			lockfile.Options{
				HeldError: ErrLockHeld,
				// AcquireRetryLimit:            lockAcquireRetryLimit,
				// AcquireRetryDelay:            lockAcquireRetryDelay,
				// InvalidPIDLockStaleThreshold: invalidLockStaleThreshold,
				// FileWriteMode:                fileWriteMode,
				// DirectoryWriteMode:           directoryWriteMode,
			},
		),
	}
}

// NewDevLock creates a lock rooted in the project's dist static directory.
func NewDevLock(distStaticDirectoryPath string) *DevLock {
	return newProjectLock(distStaticDirectoryPath, LockFileName)
}

// NewBuildLock creates a build lock rooted in the dist static directory.
func NewBuildLock(distStaticDirectoryPath string) *DevLock {
	return newProjectLock(distStaticDirectoryPath, BuildLockFileName)
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

// Held reports whether Acquire has succeeded in-process and has not been
// released.
func (devLock *DevLock) Held() bool {
	if devLock == nil || devLock.lock == nil {
		return false
	}
	return devLock.lock.Held()
}

// IsLockFileName reports whether a filename is a Wave project lock file.
func IsLockFileName(name string) bool {
	switch strings.TrimSpace(name) {
	case LockFileName, BuildLockFileName:
		return true
	default:
		return false
	}
}

// IsLockFile reports whether a filename is a Wave project lock file.
func IsLockFile(name string) bool {
	return IsLockFileName(name)
}

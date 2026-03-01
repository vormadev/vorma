// Package lockfile provides cross-process file locks using lease heartbeats and
// stale-lock reclamation semantics for command and daemon orchestration.
package lockfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	lockFileLeaseSchemaVersion = 1

	defaultAcquireRetryLimit                     = 12
	defaultAcquireRetryDelay                     = 20 * time.Millisecond
	defaultInvalidLockStaleThreshold             = 1200 * time.Millisecond
	defaultLeaseHeartbeatInterval                = 250 * time.Millisecond
	defaultLeaseStaleThreshold                   = 3 * time.Second
	defaultFileWriteMode             fs.FileMode = 0o644
	defaultDirectoryWriteMode        fs.FileMode = 0o755
)

// ErrLockHeld reports that another process currently owns the lock file.
var ErrLockHeld = errors.New("lock file is already held by another process")

var leaseOwnerIdentifierSequence uint64

// Options controls lock-file behavior.
type Options struct {
	// HeldError is wrapped when an active owner is detected.
	// Defaults to ErrLockHeld when unset.
	HeldError error
	// AcquireRetryLimit is the number of retry attempts after a stale-lock remove
	// race before giving up.
	AcquireRetryLimit int
	// AcquireRetryDelay is the delay between lock acquisition retries.
	AcquireRetryDelay time.Duration
	// InvalidPIDLockStaleThreshold is the age threshold before reclaiming lock
	// files that do not contain a parseable lease record.
	InvalidPIDLockStaleThreshold time.Duration
	// FileWriteMode is used when creating lock files.
	FileWriteMode fs.FileMode
	// DirectoryWriteMode is used when creating parent directories.
	DirectoryWriteMode fs.FileMode
	// ProcessAppearsAlive is ignored and retained only for API stability.
	ProcessAppearsAlive func(processID int) bool
	// LeaseHeartbeatInterval controls heartbeat write cadence while lock is held.
	// Defaults to 250ms when unset.
	LeaseHeartbeatInterval time.Duration
	// LeaseStaleThreshold is the stale lease threshold before takeover.
	// Defaults to 3s when unset.
	LeaseStaleThreshold time.Duration
	// OnLeaseLost is called when ownership is lost while the lock is held.
	OnLeaseLost func()
}

// PIDLock manages exclusive ownership for one lock-file path using lease files.
type PIDLock struct {
	path         string
	options      resolvedOptions
	mu           sync.Mutex
	held         bool
	leaseOwnerID string
	leaseStopCh  chan struct{}
	leaseDoneCh  chan struct{}
}

// lockFileSnapshot captures one stable read of lock-file state.
type lockFileSnapshot struct {
	rawData   []byte
	modified  time.Time
	lease     lockFileLeaseRecord
	leaseMode bool
}

type lockFileLeaseRecord struct {
	Version               int    `json:"version"`
	OwnerID               string `json:"ownerID"`
	PID                   int    `json:"pid"`
	LastHeartbeatUnixNano int64  `json:"lastHeartbeatUnixNano"`
}

// resolvedOptions stores normalized runtime lock options.
type resolvedOptions struct {
	heldError                 error
	acquireRetryLimit         int
	acquireRetryDelay         time.Duration
	invalidLockStaleThreshold time.Duration
	fileWriteMode             fs.FileMode
	directoryWriteMode        fs.FileMode
	processAppearsAlive       func(processID int) bool
	leaseHeartbeatInterval    time.Duration
	leaseStaleThreshold       time.Duration
	onLeaseLost               func()
}

// NewPIDLock creates a lock using default lock options.
func NewPIDLock(lockFilePath string) *PIDLock {
	return NewPIDLockWithOptions(lockFilePath, Options{})
}

// NewPIDLockWithOptions creates a lock with explicit options.
func NewPIDLockWithOptions(lockFilePath string, options Options) *PIDLock {
	return &PIDLock{
		path:    lockFilePath,
		options: resolveOptions(options),
	}
}

// Path returns the full lock-file path.
func (pidLock *PIDLock) Path() string {
	if pidLock == nil {
		return ""
	}
	return pidLock.path
}

// Acquire obtains lock ownership or returns a held error.
func (pidLock *PIDLock) Acquire() error {
	if pidLock == nil {
		return errors.New("lock is nil")
	}

	pidLock.mu.Lock()
	defer pidLock.mu.Unlock()

	if strings.TrimSpace(pidLock.path) == "" {
		return errors.New("lock path is empty")
	}
	if pidLock.held {
		return nil
	}

	if ensureDirectoryError := os.MkdirAll(
		filepath.Dir(pidLock.path),
		pidLock.options.directoryWriteMode,
	); ensureDirectoryError != nil {
		return fmt.Errorf("create lock directory: %w", ensureDirectoryError)
	}

	leaseOwnerID := buildLeaseOwnerIDForCurrentProcess()
	for attemptIndex := 0; attemptIndex < pidLock.options.acquireRetryLimit; attemptIndex++ {
		acquired, lockHeldError, acquireAttemptError :=
			pidLock.tryAcquireSingleAttempt(leaseOwnerID)
		if acquireAttemptError != nil {
			return acquireAttemptError
		}
		if lockHeldError != nil {
			return lockHeldError
		}
		if acquired {
			pidLock.held = true
			pidLock.leaseOwnerID = leaseOwnerID
			pidLock.startLeaseHeartbeatLoopLocked(leaseOwnerID)
			return nil
		}
		time.Sleep(pidLock.options.acquireRetryDelay)
	}

	return fmt.Errorf("acquire lock: contention exceeded retry budget")
}

// Release drops lock ownership by removing the lock file.
func (pidLock *PIDLock) Release() error {
	if pidLock == nil {
		return nil
	}

	pidLock.mu.Lock()
	if !pidLock.held {
		pidLock.mu.Unlock()
		return nil
	}
	leaseOwnerID := pidLock.leaseOwnerID
	leaseStopCh := pidLock.leaseStopCh
	leaseDoneCh := pidLock.leaseDoneCh
	pidLock.held = false
	pidLock.leaseOwnerID = ""
	pidLock.leaseStopCh = nil
	pidLock.leaseDoneCh = nil
	pidLock.mu.Unlock()

	if leaseStopCh != nil {
		close(leaseStopCh)
	}
	if leaseDoneCh != nil {
		<-leaseDoneCh
	}

	_, removeError := pidLock.tryRemoveLockIfOwned(leaseOwnerID)
	if removeError != nil {
		return removeError
	}
	return nil
}

// Held reports whether Acquire has succeeded in-process and has not been released.
func (pidLock *PIDLock) Held() bool {
	if pidLock == nil {
		return false
	}
	pidLock.mu.Lock()
	defer pidLock.mu.Unlock()
	return pidLock.held
}

// tryAcquireSingleAttempt tries O_EXCL create, then stale lock reclamation.
func (pidLock *PIDLock) tryAcquireSingleAttempt(
	leaseOwnerID string,
) (bool, error, error) {
	created, createError := pidLock.tryCreateLockFileForOwner(leaseOwnerID)
	if createError != nil {
		return false, nil, createError
	}
	if created {
		return true, nil, nil
	}

	snapshot, snapshotFound, snapshotError := pidLock.readLockFileSnapshot()
	if snapshotError != nil {
		return false, nil, snapshotError
	}
	if !snapshotFound {
		return false, nil, nil
	}

	if snapshot.leaseMode {
		ownerProcessAppearsAlive := true
		if pidLock.options.processAppearsAlive != nil {
			ownerProcessAppearsAlive = pidLock.options.processAppearsAlive(
				snapshot.lease.PID,
			)
		}
		if ownerProcessAppearsAlive && !isLeaseRecordStale(
			snapshot.lease,
			pidLock.options.leaseStaleThreshold,
		) {
			return false, fmt.Errorf(
				"%w (pid %d)",
				pidLock.options.heldError,
				snapshot.lease.PID,
			), nil
		}
	} else if !isInvalidLockSnapshotStale(
		snapshot.modified,
		pidLock.options.invalidLockStaleThreshold,
	) {
		return false, fmt.Errorf(
			"%w (owner unavailable)",
			pidLock.options.heldError,
		), nil
	}

	removed, removeError := pidLock.tryRemoveSnapshotIfUnchanged(snapshot)
	if removeError != nil {
		return false, nil, removeError
	}
	if !removed {
		return false, nil, nil
	}

	return false, nil, nil
}

// tryCreateLockFileForOwner performs one atomic lock-file create.
func (pidLock *PIDLock) tryCreateLockFileForOwner(
	leaseOwnerID string,
) (bool, error) {
	file, openError := os.OpenFile(
		pidLock.path,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		pidLock.options.fileWriteMode,
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
			_ = os.Remove(pidLock.path)
		}
	}()

	lockFileLeaseRecord := buildLeaseRecordForOwner(leaseOwnerID)
	encodedLockData, encodeError := encodeLeaseRecord(lockFileLeaseRecord)
	if encodeError != nil {
		return false, encodeError
	}

	if _, writeError := file.Write(encodedLockData); writeError != nil {
		return false, fmt.Errorf("write lock file lease: %w", writeError)
	}
	if closeError := file.Close(); closeError != nil {
		return false, fmt.Errorf("close lock file: %w", closeError)
	}
	written = true
	return true, nil
}

// readLockFileSnapshot loads lock content and metadata.
func (pidLock *PIDLock) readLockFileSnapshot() (lockFileSnapshot, bool, error) {
	lockRawData, readError := os.ReadFile(pidLock.path)
	if readError != nil {
		if os.IsNotExist(readError) {
			return lockFileSnapshot{}, false, nil
		}
		return lockFileSnapshot{}, false, fmt.Errorf(
			"read lock file: %w",
			readError,
		)
	}

	lockFileInfo, statError := os.Stat(pidLock.path)
	if statError != nil {
		if os.IsNotExist(statError) {
			return lockFileSnapshot{}, false, nil
		}
		return lockFileSnapshot{}, false, fmt.Errorf(
			"stat lock file: %w",
			statError,
		)
	}

	lockFileLeaseRecord, leaseRecordParsed := parseLeaseRecordFromLockData(
		lockRawData,
	)
	return lockFileSnapshot{
		rawData:   lockRawData,
		modified:  lockFileInfo.ModTime(),
		lease:     lockFileLeaseRecord,
		leaseMode: leaseRecordParsed,
	}, true, nil
}

// tryRemoveSnapshotIfUnchanged removes the lock file only if content matches.
func (pidLock *PIDLock) tryRemoveSnapshotIfUnchanged(
	snapshot lockFileSnapshot,
) (bool, error) {
	currentRawData, readError := os.ReadFile(pidLock.path)
	if readError != nil {
		if os.IsNotExist(readError) {
			return false, nil
		}
		return false, fmt.Errorf("re-read lock file: %w", readError)
	}

	if !bytes.Equal(currentRawData, snapshot.rawData) {
		return false, nil
	}

	if removeError := os.Remove(pidLock.path); removeError != nil {
		if os.IsNotExist(removeError) {
			return false, nil
		}
		return false, fmt.Errorf("remove stale lock file: %w", removeError)
	}
	return true, nil
}

// tryRemoveLockIfOwned removes the lock file only when owner id matches.
func (pidLock *PIDLock) tryRemoveLockIfOwned(
	leaseOwnerID string,
) (bool, error) {
	snapshot, snapshotFound, snapshotError := pidLock.readLockFileSnapshot()
	if snapshotError != nil {
		return false, snapshotError
	}
	if !snapshotFound {
		return false, nil
	}
	if !snapshot.leaseMode {
		return false, nil
	}
	if snapshot.lease.OwnerID != leaseOwnerID {
		return false, nil
	}
	removed, removeError := pidLock.tryRemoveSnapshotIfUnchanged(snapshot)
	if removeError != nil {
		return false, fmt.Errorf("release lock: %w", removeError)
	}
	return removed, nil
}

// parseLeaseRecordFromLockData parses lease JSON from lock-file bytes.
func parseLeaseRecordFromLockData(
	lockRawData []byte,
) (lockFileLeaseRecord, bool) {
	var lockFileLeaseRecord lockFileLeaseRecord
	if decodeError := json.Unmarshal(
		lockRawData,
		&lockFileLeaseRecord,
	); decodeError != nil {
		return lockFileLeaseRecord, false
	}
	if lockFileLeaseRecord.Version != lockFileLeaseSchemaVersion {
		return lockFileLeaseRecord, false
	}
	if strings.TrimSpace(lockFileLeaseRecord.OwnerID) == "" {
		return lockFileLeaseRecord, false
	}
	if lockFileLeaseRecord.PID <= 0 {
		return lockFileLeaseRecord, false
	}
	if lockFileLeaseRecord.LastHeartbeatUnixNano <= 0 {
		return lockFileLeaseRecord, false
	}
	return lockFileLeaseRecord, true
}

// isLeaseRecordStale reports whether the lease heartbeat is older than threshold.
func isLeaseRecordStale(
	lockFileLeaseRecord lockFileLeaseRecord,
	leaseStaleThreshold time.Duration,
) bool {
	return time.Since(
		time.Unix(0, lockFileLeaseRecord.LastHeartbeatUnixNano),
	) >= leaseStaleThreshold
}

// isInvalidLockSnapshotStale reports whether invalid lock data is old enough to reclaim.
func isInvalidLockSnapshotStale(
	lockModifiedAt time.Time,
	invalidLockStaleThreshold time.Duration,
) bool {
	return time.Since(lockModifiedAt) >= invalidLockStaleThreshold
}

// defaultProcessAppearsAlive reports whether processID likely still exists.
func defaultProcessAppearsAlive(processID int) bool {
	if processID <= 0 {
		return false
	}

	process, findProcessError := os.FindProcess(processID)
	if findProcessError != nil {
		return false
	}

	signalProbeError := process.Signal(syscall.Signal(0))
	if signalProbeError == nil {
		return true
	}
	if errors.Is(signalProbeError, os.ErrProcessDone) {
		return false
	}

	signalProbeErrorMessage := strings.ToLower(signalProbeError.Error())
	if strings.Contains(signalProbeErrorMessage, "no such process") ||
		strings.Contains(signalProbeErrorMessage, "process already finished") ||
		strings.Contains(signalProbeErrorMessage, "process has already exited") {
		return false
	}
	if strings.Contains(signalProbeErrorMessage, "operation not permitted") ||
		strings.Contains(signalProbeErrorMessage, "permission denied") ||
		strings.Contains(signalProbeErrorMessage, "access is denied") ||
		strings.Contains(signalProbeErrorMessage, "not supported") {
		return true
	}

	// Unknown probe failures are treated as alive to avoid unsafe takeover.
	return true
}

// buildLeaseRecordForOwner returns one lease record snapshot for lock ownership.
func buildLeaseRecordForOwner(leaseOwnerID string) lockFileLeaseRecord {
	return lockFileLeaseRecord{
		Version:               lockFileLeaseSchemaVersion,
		OwnerID:               leaseOwnerID,
		PID:                   os.Getpid(),
		LastHeartbeatUnixNano: time.Now().UTC().UnixNano(),
	}
}

// encodeLeaseRecord encodes a lease record as compact JSON with trailing newline.
func encodeLeaseRecord(
	lockFileLeaseRecord lockFileLeaseRecord,
) ([]byte, error) {
	encodedLockData, encodeError := json.Marshal(lockFileLeaseRecord)
	if encodeError != nil {
		return nil, fmt.Errorf("encode lock file lease: %w", encodeError)
	}
	encodedLockData = append(encodedLockData, '\n')
	return encodedLockData, nil
}

// buildLeaseOwnerIDForCurrentProcess allocates one unique owner id.
func buildLeaseOwnerIDForCurrentProcess() string {
	ownerSequence := atomic.AddUint64(
		&leaseOwnerIdentifierSequence,
		1,
	)
	return fmt.Sprintf(
		"%d-%d-%d",
		os.Getpid(),
		time.Now().UTC().UnixNano(),
		ownerSequence,
	)
}

// startLeaseHeartbeatLoopLocked launches heartbeat writes for one owner id.
func (pidLock *PIDLock) startLeaseHeartbeatLoopLocked(leaseOwnerID string) {
	if pidLock.options.leaseHeartbeatInterval <= 0 {
		return
	}

	pidLock.leaseStopCh = make(chan struct{})
	pidLock.leaseDoneCh = make(chan struct{})
	go pidLock.runLeaseHeartbeatLoop(
		leaseOwnerID,
		pidLock.leaseStopCh,
		pidLock.leaseDoneCh,
	)
}

// runLeaseHeartbeatLoop periodically refreshes lock heartbeat while held.
func (pidLock *PIDLock) runLeaseHeartbeatLoop(
	leaseOwnerID string,
	leaseStopCh <-chan struct{},
	leaseDoneCh chan<- struct{},
) {
	defer close(leaseDoneCh)
	leaseHeartbeatTicker := time.NewTicker(
		pidLock.options.leaseHeartbeatInterval,
	)
	defer leaseHeartbeatTicker.Stop()

	for {
		select {
		case <-leaseStopCh:
			return
		case <-leaseHeartbeatTicker.C:
			ownershipLost, refreshError := pidLock.refreshLeaseHeartbeatForOwner(
				leaseOwnerID,
			)
			if refreshError != nil || ownershipLost {
				pidLock.handleLeaseLoss(leaseOwnerID)
				return
			}
		}
	}
}

// handleLeaseLoss marks in-process ownership as lost and invokes callback.
func (pidLock *PIDLock) handleLeaseLoss(leaseOwnerID string) {
	if pidLock == nil {
		return
	}

	pidLock.mu.Lock()
	if !pidLock.held || pidLock.leaseOwnerID != leaseOwnerID {
		pidLock.mu.Unlock()
		return
	}
	pidLock.held = false
	pidLock.leaseOwnerID = ""
	pidLock.leaseStopCh = nil
	pidLock.leaseDoneCh = nil
	onLeaseLost := pidLock.options.onLeaseLost
	pidLock.mu.Unlock()

	if onLeaseLost != nil {
		onLeaseLost()
	}
}

// refreshLeaseHeartbeatForOwner updates heartbeat when owner still controls file.
func (pidLock *PIDLock) refreshLeaseHeartbeatForOwner(
	leaseOwnerID string,
) (bool, error) {
	lockFile, openError := os.OpenFile(
		pidLock.path,
		os.O_RDWR,
		pidLock.options.fileWriteMode,
	)
	if openError != nil {
		if os.IsNotExist(openError) {
			return true, nil
		}
		return false, fmt.Errorf(
			"open lock file for heartbeat: %w",
			openError,
		)
	}
	defer lockFile.Close()

	lockFileInfoAtOpen, statAtOpenError := lockFile.Stat()
	if statAtOpenError != nil {
		return false, fmt.Errorf(
			"stat lock file descriptor for heartbeat: %w",
			statAtOpenError,
		)
	}

	lockRawData, readError := io.ReadAll(lockFile)
	if readError != nil {
		return false, fmt.Errorf("read lock file for heartbeat: %w", readError)
	}

	lockLeaseRecord, leaseRecordParsed := parseLeaseRecordFromLockData(lockRawData)
	if !leaseRecordParsed {
		return true, nil
	}
	if lockLeaseRecord.OwnerID != leaseOwnerID {
		return true, nil
	}

	lockFileInfoAtPath, statPathError := os.Stat(pidLock.path)
	if statPathError != nil {
		if os.IsNotExist(statPathError) {
			return true, nil
		}
		return false, fmt.Errorf(
			"stat lock file path for heartbeat: %w",
			statPathError,
		)
	}
	if !os.SameFile(lockFileInfoAtOpen, lockFileInfoAtPath) {
		return true, nil
	}

	lockLeaseRecord.LastHeartbeatUnixNano = time.Now().UTC().UnixNano()
	encodedLeaseRecord, encodeError := encodeLeaseRecord(lockLeaseRecord)
	if encodeError != nil {
		return false, encodeError
	}

	if truncateError := lockFile.Truncate(0); truncateError != nil {
		return false, fmt.Errorf(
			"truncate lock file for heartbeat: %w",
			truncateError,
		)
	}
	if _, seekError := lockFile.Seek(0, 0); seekError != nil {
		return false, fmt.Errorf("seek lock file for heartbeat: %w", seekError)
	}
	if _, writeError := lockFile.Write(encodedLeaseRecord); writeError != nil {
		return false, fmt.Errorf("write lock file heartbeat: %w", writeError)
	}

	lockFileInfoAfterWrite, statAfterWriteError := os.Stat(pidLock.path)
	if statAfterWriteError != nil {
		if os.IsNotExist(statAfterWriteError) {
			return true, nil
		}
		return false, fmt.Errorf(
			"stat lock file path after heartbeat write: %w",
			statAfterWriteError,
		)
	}
	if !os.SameFile(lockFileInfoAtOpen, lockFileInfoAfterWrite) {
		return true, nil
	}

	return false, nil
}

// resolveOptions applies defaults and validation to lock options.
func resolveOptions(options Options) resolvedOptions {
	resolved := resolvedOptions{
		heldError:                 options.HeldError,
		acquireRetryLimit:         options.AcquireRetryLimit,
		acquireRetryDelay:         options.AcquireRetryDelay,
		invalidLockStaleThreshold: options.InvalidPIDLockStaleThreshold,
		fileWriteMode:             options.FileWriteMode,
		directoryWriteMode:        options.DirectoryWriteMode,
		processAppearsAlive:       options.ProcessAppearsAlive,
		leaseHeartbeatInterval:    options.LeaseHeartbeatInterval,
		leaseStaleThreshold:       options.LeaseStaleThreshold,
		onLeaseLost:               options.OnLeaseLost,
	}
	if resolved.heldError == nil {
		resolved.heldError = ErrLockHeld
	}
	if resolved.acquireRetryLimit <= 0 {
		resolved.acquireRetryLimit = defaultAcquireRetryLimit
	}
	if resolved.acquireRetryDelay <= 0 {
		resolved.acquireRetryDelay = defaultAcquireRetryDelay
	}
	if resolved.invalidLockStaleThreshold <= 0 {
		resolved.invalidLockStaleThreshold = defaultInvalidLockStaleThreshold
	}
	if resolved.fileWriteMode == 0 {
		resolved.fileWriteMode = defaultFileWriteMode
	}
	if resolved.directoryWriteMode == 0 {
		resolved.directoryWriteMode = defaultDirectoryWriteMode
	}
	if resolved.processAppearsAlive == nil {
		resolved.processAppearsAlive = defaultProcessAppearsAlive
	}
	if resolved.leaseHeartbeatInterval <= 0 {
		resolved.leaseHeartbeatInterval = defaultLeaseHeartbeatInterval
	}
	if resolved.leaseStaleThreshold <= 0 {
		resolved.leaseStaleThreshold = defaultLeaseStaleThreshold
	}
	return resolved
}

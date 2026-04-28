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
	lease_schema_version = 1

	acquire_retry_limit                    = 3
	acquire_retry_delay                    = 20 * time.Millisecond
	invalid_lock_stale_thresh              = 1200 * time.Millisecond
	default_heartbeat_interval             = 250 * time.Millisecond
	default_stale_threshold                = 3 * time.Second
	file_write_mode            fs.FileMode = 0o644
	directory_write_mode       fs.FileMode = 0o755
)

// ErrLockHeld reports that another process currently owns the lock file.
var ErrLockHeld = errors.New("lock file is already held by another process")

var owner_id_sequence uint64

// Options controls lock-file behavior.
type Options struct {
	// HeldError is wrapped when an active owner is detected.
	// Defaults to ErrLockHeld when unset.
	HeldError error
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
	path        string
	options     resolved_options
	mu          sync.Mutex
	held        bool
	lease_owner string
	lease_stop  chan struct{}
	lease_done  chan struct{}
}

// lease_snapshot captures one stable read of lock-file state.
type lease_snapshot struct {
	raw_data   []byte
	modified   time.Time
	record     lease_record
	lease_mode bool
}

// lease_record is the on-disk JSON structure inside a lock file.
// Fields are exported solely for json.Marshal/Unmarshal.
type lease_record struct {
	Version               int    `json:"version"`
	OwnerID               string `json:"ownerID"`
	PID                   int    `json:"pid"`
	LastHeartbeatUnixNano int64  `json:"lastHeartbeatUnixNano"`
}

// resolved_options stores normalized runtime lock options.
type resolved_options struct {
	held_error         error
	heartbeat_interval time.Duration
	stale_threshold    time.Duration
	on_lease_lost      func()
}

// NewPIDLock creates a lock using default lock options.
func NewPIDLock(lock_file_path string) *PIDLock {
	return NewPIDLockWithOptions(lock_file_path, Options{})
}

// NewPIDLockWithOptions creates a lock with explicit options.
func NewPIDLockWithOptions(lock_file_path string, options Options) *PIDLock {
	return &PIDLock{
		path:    lock_file_path,
		options: resolve_options(options),
	}
}

// Path returns the full lock-file path.
func (lock *PIDLock) Path() string {
	if lock == nil {
		return ""
	}
	return lock.path
}

// Acquire obtains lock ownership or returns a held error.
func (lock *PIDLock) Acquire() error {
	if lock == nil {
		return errors.New("lock is nil")
	}

	lock.mu.Lock()
	defer lock.mu.Unlock()

	if strings.TrimSpace(lock.path) == "" {
		return errors.New("lock path is empty")
	}
	if lock.held {
		return nil
	}

	if err := os.MkdirAll(
		filepath.Dir(lock.path),
		directory_write_mode,
	); err != nil {
		return fmt.Errorf("create lock directory: %w", err)
	}

	owner_id := build_owner_id()
	for range acquire_retry_limit {
		acquired, held_err, attempt_err := lock.try_acquire(owner_id)
		if attempt_err != nil {
			return attempt_err
		}
		if held_err != nil {
			return held_err
		}
		if acquired {
			lock.held = true
			lock.lease_owner = owner_id
			lock.start_heartbeat_locked(owner_id)
			return nil
		}
		time.Sleep(acquire_retry_delay)
	}

	return fmt.Errorf("acquire lock: contention exceeded retry budget")
}

// Release drops lock ownership by removing the lock file.
func (lock *PIDLock) Release() error {
	if lock == nil {
		return nil
	}

	lock.mu.Lock()
	if !lock.held {
		lock.mu.Unlock()
		return nil
	}
	owner_id := lock.lease_owner
	stop_ch := lock.lease_stop
	done_ch := lock.lease_done
	lock.held = false
	lock.lease_owner = ""
	lock.lease_stop = nil
	lock.lease_done = nil
	lock.mu.Unlock()

	if stop_ch != nil {
		close(stop_ch)
	}
	if done_ch != nil {
		<-done_ch
	}

	_, err := lock.try_remove_if_owned(owner_id)
	if err != nil {
		return err
	}
	return nil
}

// Held reports whether Acquire has succeeded in-process and has not been released.
func (lock *PIDLock) Held() bool {
	if lock == nil {
		return false
	}
	lock.mu.Lock()
	defer lock.mu.Unlock()
	return lock.held
}

// try_acquire tries O_EXCL create, then stale lock reclamation.
func (lock *PIDLock) try_acquire(owner_id string) (bool, error, error) {
	created, err := lock.try_create(owner_id)
	if err != nil {
		return false, nil, err
	}
	if created {
		return true, nil, nil
	}

	snap, found, err := lock.read_snapshot()
	if err != nil {
		return false, nil, err
	}
	if !found {
		return false, nil, nil
	}

	if snap.lease_mode {
		alive := process_appears_alive(snap.record.PID)
		stale := is_lease_stale(snap.record, lock.options.stale_threshold)
		if alive && !stale {
			return false, fmt.Errorf(
				"%w (pid %d)",
				lock.options.held_error,
				snap.record.PID,
			), nil
		}
	} else if !is_invalid_lock_stale(snap.modified) {
		return false, fmt.Errorf(
			"%w (owner unavailable)",
			lock.options.held_error,
		), nil
	}

	removed, err := lock.try_remove_if_unchanged(snap)
	if err != nil {
		return false, nil, err
	}
	if !removed {
		return false, nil, nil
	}

	return false, nil, nil
}

// try_create performs one atomic lock-file create.
func (lock *PIDLock) try_create(owner_id string) (bool, error) {
	file, err := os.OpenFile(
		lock.path,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		file_write_mode,
	)
	if err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("create lock file: %w", err)
	}

	written := false
	defer func() {
		if !written {
			_ = file.Close()
			_ = os.Remove(lock.path)
		}
	}()

	rec := build_lease_record(owner_id)
	encoded, err := encode_lease_record(rec)
	if err != nil {
		return false, err
	}

	if _, err := file.Write(encoded); err != nil {
		return false, fmt.Errorf("write lock file lease: %w", err)
	}
	if err := file.Close(); err != nil {
		return false, fmt.Errorf("close lock file: %w", err)
	}
	written = true
	return true, nil
}

// read_snapshot loads lock content and metadata.
func (lock *PIDLock) read_snapshot() (lease_snapshot, bool, error) {
	raw_data, err := os.ReadFile(lock.path)
	if err != nil {
		if os.IsNotExist(err) {
			return lease_snapshot{}, false, nil
		}
		return lease_snapshot{}, false, fmt.Errorf("read lock file: %w", err)
	}

	info, err := os.Stat(lock.path)
	if err != nil {
		if os.IsNotExist(err) {
			return lease_snapshot{}, false, nil
		}
		return lease_snapshot{}, false, fmt.Errorf("stat lock file: %w", err)
	}

	rec, parsed := parse_lease_record(raw_data)
	return lease_snapshot{
		raw_data:   raw_data,
		modified:   info.ModTime(),
		record:     rec,
		lease_mode: parsed,
	}, true, nil
}

// try_remove_if_unchanged removes the lock file only if content matches.
func (lock *PIDLock) try_remove_if_unchanged(
	snap lease_snapshot,
) (bool, error) {
	current_data, err := os.ReadFile(lock.path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("re-read lock file: %w", err)
	}

	if !bytes.Equal(current_data, snap.raw_data) {
		return false, nil
	}

	if err := os.Remove(lock.path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("remove stale lock file: %w", err)
	}
	return true, nil
}

// try_remove_if_owned removes the lock file only when owner id matches.
func (lock *PIDLock) try_remove_if_owned(owner_id string) (bool, error) {
	snap, found, err := lock.read_snapshot()
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	if !snap.lease_mode {
		return false, nil
	}
	if snap.record.OwnerID != owner_id {
		return false, nil
	}
	removed, err := lock.try_remove_if_unchanged(snap)
	if err != nil {
		return false, fmt.Errorf("release lock: %w", err)
	}
	return removed, nil
}

// parse_lease_record parses lease JSON from lock-file bytes.
func parse_lease_record(raw_data []byte) (lease_record, bool) {
	var rec lease_record
	if err := json.Unmarshal(raw_data, &rec); err != nil {
		return rec, false
	}
	if rec.Version != lease_schema_version {
		return rec, false
	}
	if strings.TrimSpace(rec.OwnerID) == "" {
		return rec, false
	}
	if rec.PID <= 0 {
		return rec, false
	}
	if rec.LastHeartbeatUnixNano <= 0 {
		return rec, false
	}
	return rec, true
}

// is_lease_stale reports whether the lease heartbeat is older than threshold.
func is_lease_stale(rec lease_record, threshold time.Duration) bool {
	return time.Since(time.Unix(0, rec.LastHeartbeatUnixNano)) >= threshold
}

// is_invalid_lock_stale reports whether invalid lock data is old enough to reclaim.
func is_invalid_lock_stale(modified_at time.Time) bool {
	return time.Since(modified_at) >= invalid_lock_stale_thresh
}

// process_appears_alive reports whether the given pid likely still exists.
func process_appears_alive(pid int) bool {
	if pid <= 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	sig_err := proc.Signal(syscall.Signal(0))
	if sig_err == nil {
		return true
	}
	if errors.Is(sig_err, os.ErrProcessDone) {
		return false
	}
	if errors.Is(sig_err, syscall.ESRCH) {
		return false
	}
	if errors.Is(sig_err, syscall.EPERM) {
		return true
	}

	// Unknown probe failures are treated as alive to avoid unsafe takeover.
	return true
}

// build_lease_record returns one lease record snapshot for lock ownership.
func build_lease_record(owner_id string) lease_record {
	return lease_record{
		Version:               lease_schema_version,
		OwnerID:               owner_id,
		PID:                   os.Getpid(),
		LastHeartbeatUnixNano: time.Now().UTC().UnixNano(),
	}
}

// encode_lease_record encodes a lease record as compact JSON with trailing newline.
func encode_lease_record(rec lease_record) ([]byte, error) {
	data, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("encode lock file lease: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// build_owner_id allocates one unique owner id for the current process.
func build_owner_id() string {
	seq := atomic.AddUint64(&owner_id_sequence, 1)
	return fmt.Sprintf(
		"%d-%d-%d",
		os.Getpid(),
		time.Now().UTC().UnixNano(),
		seq,
	)
}

// start_heartbeat_locked launches heartbeat writes for one owner id.
func (lock *PIDLock) start_heartbeat_locked(owner_id string) {
	if lock.options.heartbeat_interval <= 0 {
		return
	}

	lock.lease_stop = make(chan struct{})
	lock.lease_done = make(chan struct{})
	go lock.run_heartbeat_loop(owner_id, lock.lease_stop, lock.lease_done)
}

// run_heartbeat_loop periodically refreshes lock heartbeat while held.
func (lock *PIDLock) run_heartbeat_loop(
	owner_id string,
	stop_ch <-chan struct{},
	done_ch chan<- struct{},
) {
	defer close(done_ch)
	ticker := time.NewTicker(lock.options.heartbeat_interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop_ch:
			return
		case <-ticker.C:
			lost, err := lock.refresh_heartbeat(owner_id)
			if err != nil || lost {
				lock.handle_lease_loss(owner_id)
				return
			}
		}
	}
}

// handle_lease_loss marks in-process ownership as lost and invokes callback.
func (lock *PIDLock) handle_lease_loss(owner_id string) {
	if lock == nil {
		return
	}

	lock.mu.Lock()
	if !lock.held || lock.lease_owner != owner_id {
		lock.mu.Unlock()
		return
	}
	lock.held = false
	lock.lease_owner = ""
	lock.lease_stop = nil
	lock.lease_done = nil
	callback := lock.options.on_lease_lost
	lock.mu.Unlock()

	if callback != nil {
		callback()
	}
}

// refresh_heartbeat updates heartbeat when owner still controls the file.
func (lock *PIDLock) refresh_heartbeat(owner_id string) (bool, error) {
	file, err := os.OpenFile(lock.path, os.O_RDWR, file_write_mode)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("open lock file for heartbeat: %w", err)
	}
	defer file.Close()

	info_at_open, err := file.Stat()
	if err != nil {
		return false, fmt.Errorf(
			"stat lock file descriptor for heartbeat: %w",
			err,
		)
	}

	raw_data, err := io.ReadAll(file)
	if err != nil {
		return false, fmt.Errorf("read lock file for heartbeat: %w", err)
	}

	rec, parsed := parse_lease_record(raw_data)
	if !parsed {
		return true, nil
	}
	if rec.OwnerID != owner_id {
		return true, nil
	}

	info_at_path, err := os.Stat(lock.path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf(
			"stat lock file path for heartbeat: %w",
			err,
		)
	}
	if !os.SameFile(info_at_open, info_at_path) {
		return true, nil
	}

	rec.LastHeartbeatUnixNano = time.Now().UTC().UnixNano()
	encoded, err := encode_lease_record(rec)
	if err != nil {
		return false, err
	}

	if err := file.Truncate(0); err != nil {
		return false, fmt.Errorf(
			"truncate lock file for heartbeat: %w",
			err,
		)
	}
	if _, err := file.Seek(0, 0); err != nil {
		return false, fmt.Errorf("seek lock file for heartbeat: %w", err)
	}
	if _, err := file.Write(encoded); err != nil {
		return false, fmt.Errorf("write lock file heartbeat: %w", err)
	}

	info_after_write, err := os.Stat(lock.path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf(
			"stat lock file path after heartbeat write: %w",
			err,
		)
	}
	if !os.SameFile(info_at_open, info_after_write) {
		return true, nil
	}

	return false, nil
}

// resolve_options applies defaults to lock options.
func resolve_options(options Options) resolved_options {
	resolved := resolved_options{
		held_error:         options.HeldError,
		heartbeat_interval: options.LeaseHeartbeatInterval,
		stale_threshold:    options.LeaseStaleThreshold,
		on_lease_lost:      options.OnLeaseLost,
	}
	if resolved.held_error == nil {
		resolved.held_error = ErrLockHeld
	}
	if resolved.heartbeat_interval <= 0 {
		resolved.heartbeat_interval = default_heartbeat_interval
	}
	if resolved.stale_threshold <= 0 {
		resolved.stale_threshold = default_stale_threshold
	}
	return resolved
}

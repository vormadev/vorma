package shared_test

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave/internal/shared"
)

func TestDevLock_AcquireBlocksSecondAcquireUntilRelease(t *testing.T) {
	staticDir := t.TempDir()

	first := shared.NewDevLock(staticDir)
	if err := first.Acquire(); err != nil {
		t.Fatalf("first acquire returned error: %v", err)
	}
	defer first.Release()

	second := shared.NewDevLock(staticDir)
	err := second.Acquire()
	if err == nil {
		t.Fatal("expected second acquire to fail while first lock is held")
	}
	if !errors.Is(err, shared.ErrLockHeld) {
		t.Fatalf("expected ErrLockHeld, got %v", err)
	}

	if err := first.Release(); err != nil {
		t.Fatalf("release returned error: %v", err)
	}

	if err := second.Acquire(); err != nil {
		t.Fatalf("expected acquire to succeed after release, got: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatalf("second release returned error: %v", err)
	}
}

func TestIsLockFile(t *testing.T) {
	if !shared.IsLockFile(".wave-dev.lock") {
		t.Fatal("expected .wave-dev.lock to be recognized as lock file")
	}
	if shared.IsLockFile(".wave-custom") {
		t.Fatal(
			"expected non-lock .wave-* file to not be recognized as lock file",
		)
	}
	if shared.IsLockFile("wave-dev.lock") {
		t.Fatal("expected wave-dev.lock to not be recognized as lock file")
	}
}

func TestDevLock_AcquireRecoversStaleLockFile(t *testing.T) {
	staticDir := t.TempDir()
	lock := shared.NewDevLock(staticDir)

	writeLeaseRecordForDevLockTest(
		t,
		lock.Path(),
		"stale-owner",
		os.Getpid(),
		time.Now().UTC().Add(-10*time.Second),
	)

	if err := lock.Acquire(); err != nil {
		t.Fatalf("expected stale lock recovery to acquire lock, got: %v", err)
	}
	defer lock.Release()

	lockData, err := os.ReadFile(lock.Path())
	if err != nil {
		t.Fatalf("failed reading lock file after acquire: %v", err)
	}

	var leaseRecord struct {
		PID int `json:"pid"`
	}
	if decodeError := json.Unmarshal(lockData, &leaseRecord); decodeError != nil {
		t.Fatalf("failed decoding lock file json: %v", decodeError)
	}
	if leaseRecord.PID != os.Getpid() {
		t.Fatalf("lock file pid = %d, want %d", leaseRecord.PID, os.Getpid())
	}
}

func TestDevLock_AcquireRecoversInvalidLockFileContents(t *testing.T) {
	staticDir := t.TempDir()
	lock := shared.NewDevLock(staticDir)

	if err := os.WriteFile(lock.Path(), []byte("not-a-pid"), 0o644); err != nil {
		t.Fatalf("failed writing invalid lock file contents: %v", err)
	}
	staleTimestamp := time.Now().Add(-2 * time.Second)
	if err := os.Chtimes(lock.Path(), staleTimestamp, staleTimestamp); err != nil {
		t.Fatalf("failed to age invalid lock file contents: %v", err)
	}

	if err := lock.Acquire(); err != nil {
		t.Fatalf(
			"expected invalid lock contents to be recoverable, got: %v",
			err,
		)
	}
	defer lock.Release()
}

func TestDevLock_AcquireTreatsFreshInvalidLockContentsAsHeld(t *testing.T) {
	staticDir := t.TempDir()
	lock := shared.NewDevLock(staticDir)

	if err := os.WriteFile(lock.Path(), []byte("not-a-pid"), 0o644); err != nil {
		t.Fatalf("failed writing invalid lock file contents: %v", err)
	}

	err := lock.Acquire()
	if err == nil {
		t.Fatal("expected fresh invalid lock contents to be treated as held")
	}
	if !errors.Is(err, shared.ErrLockHeld) {
		t.Fatalf(
			"expected ErrLockHeld for fresh invalid lock contents, got: %v",
			err,
		)
	}
}

func TestDevLock_AtomicCreateAllowsOnlyOneConcurrentAcquire(t *testing.T) {
	staticDir := t.TempDir()
	const contenderCount = 12

	var waitGroup sync.WaitGroup
	resultChannel := make(chan error, contenderCount)
	releaseChannel := make(chan struct{})

	for i := 0; i < contenderCount; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			lock := shared.NewDevLock(staticDir)
			err := lock.Acquire()
			if err != nil {
				resultChannel <- err
				return
			}

			resultChannel <- nil
			<-releaseChannel
			_ = lock.Release()
		}()
	}

	successCount := 0
	lockHeldCount := 0

	for i := 0; i < contenderCount; i++ {
		select {
		case err := <-resultChannel:
			if err == nil {
				successCount++
				continue
			}
			if !errors.Is(err, shared.ErrLockHeld) {
				t.Fatalf(
					"expected ErrLockHeld for contended acquire, got: %v",
					err,
				)
			}
			lockHeldCount++
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for contended acquire results")
		}
	}

	if successCount != 1 {
		t.Fatalf(
			"expected exactly one successful concurrent acquire, got %d",
			successCount,
		)
	}
	if lockHeldCount != contenderCount-1 {
		t.Fatalf(
			"expected %d ErrLockHeld results, got %d",
			contenderCount-1,
			lockHeldCount,
		)
	}

	close(releaseChannel)
	waitGroup.Wait()
}

func writeLeaseRecordForDevLockTest(
	t *testing.T,
	lockFilePath string,
	ownerID string,
	pid int,
	heartbeatAt time.Time,
) {
	t.Helper()
	leaseRecordBytes, encodeError := json.Marshal(struct {
		Version               int    `json:"version"`
		OwnerID               string `json:"ownerID"`
		PID                   int    `json:"pid"`
		LastHeartbeatUnixNano int64  `json:"lastHeartbeatUnixNano"`
	}{
		Version:               1,
		OwnerID:               ownerID,
		PID:                   pid,
		LastHeartbeatUnixNano: heartbeatAt.UTC().UnixNano(),
	})
	if encodeError != nil {
		t.Fatalf("encode lease record: %v", encodeError)
	}
	leaseRecordBytes = append(leaseRecordBytes, '\n')
	if writeError := os.WriteFile(lockFilePath, leaseRecordBytes, 0o644); writeError != nil {
		t.Fatalf("write lease record: %v", writeError)
	}
}

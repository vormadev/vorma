package lockfile_test

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/kit/lockfile"
)

type leaseRecordForTest struct {
	Version               int    `json:"version"`
	OwnerID               string `json:"ownerID"`
	PID                   int    `json:"pid"`
	LastHeartbeatUnixNano int64  `json:"lastHeartbeatUnixNano"`
}

func TestPIDLockAcquireBlocksSecondAcquireUntilRelease(t *testing.T) {
	lockDirectory := t.TempDir()
	lockFilePath := lockDirectory + "/example.lock"

	first := lockfile.NewPIDLock(lockFilePath)
	if firstAcquireError := first.Acquire(); firstAcquireError != nil {
		t.Fatalf("first acquire returned error: %v", firstAcquireError)
	}
	defer first.Release()

	second := lockfile.NewPIDLock(lockFilePath)
	secondAcquireError := second.Acquire()
	if secondAcquireError == nil {
		t.Fatal("expected second acquire to fail while first lock is held")
	}
	if !errors.Is(secondAcquireError, lockfile.ErrLockHeld) {
		t.Fatalf("expected ErrLockHeld, got %v", secondAcquireError)
	}

	if firstReleaseError := first.Release(); firstReleaseError != nil {
		t.Fatalf("release returned error: %v", firstReleaseError)
	}

	if secondAcquireRetryError := second.Acquire(); secondAcquireRetryError != nil {
		t.Fatalf(
			"expected acquire to succeed after release, got: %v",
			secondAcquireRetryError,
		)
	}
	if secondReleaseError := second.Release(); secondReleaseError != nil {
		t.Fatalf("second release returned error: %v", secondReleaseError)
	}
}

func TestPIDLockAcquireRecoversStaleLeaseLockFile(t *testing.T) {
	lockDirectory := t.TempDir()
	lockFilePath := lockDirectory + "/example.lock"
	lock := lockfile.NewPIDLock(lockFilePath)

	writeLeaseRecordForTest(
		t,
		lock.Path(),
		"stale-owner",
		os.Getpid(),
		time.Now().UTC().Add(-10*time.Second),
	)

	if acquireError := lock.Acquire(); acquireError != nil {
		t.Fatalf(
			"expected stale lock recovery to acquire lock, got: %v",
			acquireError,
		)
	}
	defer lock.Release()

	lockData, readLockError := os.ReadFile(lock.Path())
	if readLockError != nil {
		t.Fatalf("failed reading lock file after acquire: %v", readLockError)
	}

	var leaseRecord leaseRecordForTest
	if decodeError := json.Unmarshal(lockData, &leaseRecord); decodeError != nil {
		t.Fatalf("failed decoding lock file after acquire: %v", decodeError)
	}
	if leaseRecord.OwnerID == "stale-owner" {
		t.Fatalf(
			"expected lock ownership to rotate away from stale owner, got %q",
			leaseRecord.OwnerID,
		)
	}
	if leaseRecord.PID != os.Getpid() {
		t.Fatalf("lock file pid = %d, want %d", leaseRecord.PID, os.Getpid())
	}
}

func TestPIDLockAcquireRecoversFreshLeaseWhenOwnerProcessIsDead(t *testing.T) {
	lockDirectory := t.TempDir()
	lockFilePath := lockDirectory + "/example.lock"
	const deadOwnerPID = 42_424

	writeLeaseRecordForTest(
		t,
		lockFilePath,
		"dead-owner",
		deadOwnerPID,
		time.Now().UTC(),
	)

	lock := lockfile.NewPIDLockWithOptions(
		lockFilePath,
		lockfile.Options{
			ProcessAppearsAlive: func(processID int) bool {
				return processID != deadOwnerPID
			},
			LeaseStaleThreshold: 30 * time.Second,
		},
	)
	if acquireError := lock.Acquire(); acquireError != nil {
		t.Fatalf(
			"expected dead-owner lease takeover to acquire lock, got: %v",
			acquireError,
		)
	}
	defer lock.Release()
}

func TestPIDLockAcquireRecoversInvalidLockFileContents(t *testing.T) {
	lockDirectory := t.TempDir()
	lockFilePath := lockDirectory + "/example.lock"
	lock := lockfile.NewPIDLock(lockFilePath)

	if writeLockError := os.WriteFile(
		lock.Path(),
		[]byte("not-a-lease-json"),
		0o644,
	); writeLockError != nil {
		t.Fatalf(
			"failed writing invalid lock file contents: %v",
			writeLockError,
		)
	}
	staleTimestamp := time.Now().Add(-2 * time.Second)
	if setTimesError := os.Chtimes(
		lock.Path(),
		staleTimestamp,
		staleTimestamp,
	); setTimesError != nil {
		t.Fatalf("failed to age invalid lock file contents: %v", setTimesError)
	}

	if acquireError := lock.Acquire(); acquireError != nil {
		t.Fatalf(
			"expected invalid lock contents to be recoverable, got: %v",
			acquireError,
		)
	}
	defer lock.Release()
}

func TestPIDLockAcquireTreatsFreshInvalidLockContentsAsHeld(t *testing.T) {
	lockDirectory := t.TempDir()
	lockFilePath := lockDirectory + "/example.lock"
	lock := lockfile.NewPIDLock(lockFilePath)

	if writeLockError := os.WriteFile(
		lock.Path(),
		[]byte("not-a-lease-json"),
		0o644,
	); writeLockError != nil {
		t.Fatalf(
			"failed writing invalid lock file contents: %v",
			writeLockError,
		)
	}

	acquireError := lock.Acquire()
	if acquireError == nil {
		t.Fatal("expected fresh invalid lock contents to be treated as held")
	}
	if !errors.Is(acquireError, lockfile.ErrLockHeld) {
		t.Fatalf(
			"expected ErrLockHeld for fresh invalid lock contents, got: %v",
			acquireError,
		)
	}
}

func TestPIDLockAtomicCreateAllowsOnlyOneConcurrentAcquire(t *testing.T) {
	lockDirectory := t.TempDir()
	lockFilePath := lockDirectory + "/example.lock"
	const contenderCount = 12

	var waitGroup sync.WaitGroup
	resultChannel := make(chan error, contenderCount)
	releaseChannel := make(chan struct{})

	for index := 0; index < contenderCount; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			lock := lockfile.NewPIDLock(lockFilePath)
			acquireError := lock.Acquire()
			if acquireError != nil {
				resultChannel <- acquireError
				return
			}

			resultChannel <- nil
			<-releaseChannel
			_ = lock.Release()
		}()
	}

	successCount := 0
	lockHeldCount := 0

	for index := 0; index < contenderCount; index++ {
		select {
		case acquireError := <-resultChannel:
			if acquireError == nil {
				successCount++
				continue
			}
			if !errors.Is(acquireError, lockfile.ErrLockHeld) {
				t.Fatalf(
					"expected ErrLockHeld for contended acquire, got: %v",
					acquireError,
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

func TestPIDLockUsesCustomHeldError(t *testing.T) {
	lockDirectory := t.TempDir()
	lockFilePath := lockDirectory + "/example.lock"
	customHeldError := errors.New("custom lock held")

	first := lockfile.NewPIDLockWithOptions(
		lockFilePath,
		lockfile.Options{
			HeldError: customHeldError,
		},
	)
	if acquireError := first.Acquire(); acquireError != nil {
		t.Fatalf("first acquire returned error: %v", acquireError)
	}
	defer first.Release()

	second := lockfile.NewPIDLockWithOptions(
		lockFilePath,
		lockfile.Options{
			HeldError: customHeldError,
		},
	)
	secondAcquireError := second.Acquire()
	if secondAcquireError == nil {
		t.Fatal("expected second acquire to fail while first lock is held")
	}
	if !errors.Is(secondAcquireError, customHeldError) {
		t.Fatalf("expected custom held error, got %v", secondAcquireError)
	}
}

func TestPIDLockReleaseDoesNotRemoveForeignOwnerLockFile(t *testing.T) {
	lockDirectory := t.TempDir()
	lockFilePath := lockDirectory + "/example.lock"

	lock := lockfile.NewPIDLockWithOptions(
		lockFilePath,
		lockfile.Options{
			LeaseHeartbeatInterval: 10 * time.Second,
		},
	)
	if acquireError := lock.Acquire(); acquireError != nil {
		t.Fatalf("acquire returned error: %v", acquireError)
	}

	writeLeaseRecordForTest(
		t,
		lock.Path(),
		"foreign-owner",
		os.Getpid(),
		time.Now().UTC(),
	)

	if releaseError := lock.Release(); releaseError != nil {
		t.Fatalf("release returned error: %v", releaseError)
	}

	if _, statError := os.Stat(lock.Path()); statError != nil {
		t.Fatalf("expected foreign lock file to remain, stat error: %v", statError)
	}
}

func TestPIDLockOnLeaseLostCallbackRunsWhenOwnershipChanges(t *testing.T) {
	lockDirectory := t.TempDir()
	lockFilePath := lockDirectory + "/example.lock"

	var callbackCount int32
	leaseLostCallbackSignal := make(chan struct{}, 1)
	lock := lockfile.NewPIDLockWithOptions(
		lockFilePath,
		lockfile.Options{
			LeaseHeartbeatInterval: 15 * time.Millisecond,
			OnLeaseLost: func() {
				atomic.AddInt32(&callbackCount, 1)
				select {
				case leaseLostCallbackSignal <- struct{}{}:
				default:
				}
			},
		},
	)
	if acquireError := lock.Acquire(); acquireError != nil {
		t.Fatalf("acquire returned error: %v", acquireError)
	}
	defer func() {
		_ = os.Remove(lock.Path())
		_ = lock.Release()
	}()

	writeLeaseRecordForTest(
		t,
		lock.Path(),
		"foreign-owner",
		os.Getpid(),
		time.Now().UTC(),
	)

	select {
	case <-leaseLostCallbackSignal:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnLeaseLost callback")
	}

	if lock.Held() {
		t.Fatal("expected lock to report not-held after lease ownership loss")
	}
	if callbackCount != 1 {
		t.Fatalf("expected OnLeaseLost callback once, got %d", callbackCount)
	}
}

func writeLeaseRecordForTest(
	t *testing.T,
	lockFilePath string,
	ownerID string,
	pid int,
	heartbeatAt time.Time,
) {
	t.Helper()
	leaseRecordBytes, encodeError := json.Marshal(leaseRecordForTest{
		Version:               1,
		OwnerID:               ownerID,
		PID:                   pid,
		LastHeartbeatUnixNano: heartbeatAt.UTC().UnixNano(),
	})
	if encodeError != nil {
		t.Fatalf("encode lease record: %v", encodeError)
	}
	leaseRecordBytes = append(leaseRecordBytes, '\n')
	if writeError := os.WriteFile(
		lockFilePath,
		leaseRecordBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write lease record: %v", writeError)
	}
}

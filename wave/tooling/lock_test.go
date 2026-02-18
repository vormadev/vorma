package tooling

import (
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestDevLock_AcquireBlocksSecondAcquireUntilRelease(t *testing.T) {
	staticDir := t.TempDir()

	first := newDevLock(staticDir)
	if err := first.acquire(); err != nil {
		t.Fatalf("first acquire returned error: %v", err)
	}
	defer first.release()

	second := newDevLock(staticDir)
	err := second.acquire()
	if err == nil {
		t.Fatal("expected second acquire to fail while first lock is held")
	}
	if !errors.Is(err, ErrLockHeld) {
		t.Fatalf("expected ErrLockHeld, got %v", err)
	}

	if err := first.release(); err != nil {
		t.Fatalf("release returned error: %v", err)
	}

	if err := second.acquire(); err != nil {
		t.Fatalf("expected acquire to succeed after release, got: %v", err)
	}
	if err := second.release(); err != nil {
		t.Fatalf("second release returned error: %v", err)
	}
}

func TestIsLockFile(t *testing.T) {
	if !isLockFile(".wave-dev.lock") {
		t.Fatal("expected .wave-dev.lock to be recognized as lock file")
	}
	if isLockFile(".wave-custom") {
		t.Fatal(
			"expected non-lock .wave-* file to not be recognized as lock file",
		)
	}
	if isLockFile("wave-dev.lock") {
		t.Fatal("expected wave-dev.lock to not be recognized as lock file")
	}
}

func TestDevLock_AcquireRecoversStaleLockFile(t *testing.T) {
	staticDir := t.TempDir()
	lock := newDevLock(staticDir)

	if err := os.WriteFile(lock.path, []byte("99999999"), 0644); err != nil {
		t.Fatalf("failed writing stale lock file: %v", err)
	}

	if err := lock.acquire(); err != nil {
		t.Fatalf("expected stale lock recovery to acquire lock, got: %v", err)
	}
	defer lock.release()

	lockData, err := os.ReadFile(lock.path)
	if err != nil {
		t.Fatalf("failed reading lock file after acquire: %v", err)
	}
	if got := string(lockData); got != strconv.Itoa(os.Getpid()) {
		t.Fatalf("lock file PID = %q, want %q", got, strconv.Itoa(os.Getpid()))
	}
}

func TestDevLock_AcquireRecoversInvalidLockFileContents(t *testing.T) {
	staticDir := t.TempDir()
	lock := newDevLock(staticDir)

	if err := os.WriteFile(lock.path, []byte("not-a-pid"), 0644); err != nil {
		t.Fatalf("failed writing invalid lock file contents: %v", err)
	}
	staleTimestamp := time.Now().Add(-2 * time.Second)
	if err := os.Chtimes(lock.path, staleTimestamp, staleTimestamp); err != nil {
		t.Fatalf("failed to age invalid lock file contents: %v", err)
	}

	if err := lock.acquire(); err != nil {
		t.Fatalf(
			"expected invalid lock contents to be recoverable, got: %v",
			err,
		)
	}
	defer lock.release()
}

func TestDevLock_AcquireTreatsFreshInvalidLockContentsAsHeld(t *testing.T) {
	staticDir := t.TempDir()
	lock := newDevLock(staticDir)

	if err := os.WriteFile(lock.path, []byte("not-a-pid"), 0644); err != nil {
		t.Fatalf("failed writing invalid lock file contents: %v", err)
	}

	err := lock.acquire()
	if err == nil {
		t.Fatal("expected fresh invalid lock contents to be treated as held")
	}
	if !errors.Is(err, ErrLockHeld) {
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
			lock := newDevLock(staticDir)
			err := lock.acquire()
			if err != nil {
				resultChannel <- err
				return
			}

			resultChannel <- nil
			<-releaseChannel
			_ = lock.release()
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
			if !errors.Is(err, ErrLockHeld) {
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

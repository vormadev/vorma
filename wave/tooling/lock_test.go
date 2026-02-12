package tooling

import (
	"errors"
	"testing"
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
	if isLockFile("wave-dev.lock") {
		t.Fatal("expected wave-dev.lock to not be recognized as lock file")
	}
}

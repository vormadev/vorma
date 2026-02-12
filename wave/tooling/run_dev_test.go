package tooling

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDev_ReturnsValidationErrorForInvalidConfig(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.MainAppEntry = ""

	err := RunDev(cfg, newDiscardLogger())
	if err == nil {
		t.Fatal("expected RunDev to fail validation for missing MainAppEntry")
	}
	if !strings.Contains(err.Error(), "config validation failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDev_ReturnsLockHeldErrorWhenProjectIsAlreadyLocked(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	lock := newDevLock(cfg.Dist.Static())
	if err := lock.acquire(); err != nil {
		t.Fatalf("failed to acquire initial lock: %v", err)
	}
	defer lock.release()

	err := RunDev(cfg, newDiscardLogger())
	if err == nil {
		t.Fatal("expected RunDev to fail when lock is already held")
	}
	if !errors.Is(err, ErrLockHeld) {
		t.Fatalf("expected ErrLockHeld, got %v", err)
	}
}

func TestRunDev_WithNilLoggerReleasesLockWhenRunReturnsError(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Watch.WatchRoot = filepath.Join(root, "missing-watch-root")

	err := RunDev(cfg, nil)
	if err == nil {
		t.Fatal("expected RunDev to fail when watch root does not exist")
	}
	if !strings.Contains(err.Error(), "init watcher") {
		t.Fatalf("expected init watcher error, got: %v", err)
	}

	lock := newDevLock(cfg.Dist.Static())
	if lockErr := lock.acquire(); lockErr != nil {
		t.Fatalf("expected lock to be released after RunDev error, acquire failed: %v", lockErr)
	}
	defer lock.release()
}

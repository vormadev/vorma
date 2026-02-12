package tooling

import (
	"errors"
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

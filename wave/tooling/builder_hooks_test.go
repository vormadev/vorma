package tooling

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/kit/executil"
)

func TestRunHooks_DevRunsUserThenFramework(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	orderPath := filepath.Join(root, "hook-order.log")
	cfg.Core.DevBuildHook = "printf 'user\\n' >> " + strconv.Quote(orderPath)
	cfg.FrameworkDevBuildHook = "printf 'framework\\n' >> " + strconv.Quote(orderPath)

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	if err := builder.runHooks(true); err != nil {
		t.Fatalf("runHooks(true) returned error: %v", err)
	}

	content, err := os.ReadFile(orderPath)
	if err != nil {
		t.Fatalf("failed reading hook order file: %v", err)
	}

	if string(content) != "user\nframework\n" {
		t.Fatalf("unexpected hook order:\n%s", string(content))
	}
}

func TestRunHooks_ProdUsesProdHooks(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	outPath := filepath.Join(root, "prod-hook.log")
	cfg.Core.ProdBuildHook = "printf 'prod-user\\n' >> " + strconv.Quote(outPath)
	cfg.FrameworkProdBuildHook = "printf 'prod-framework\\n' >> " + strconv.Quote(outPath)

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	if err := builder.runHooks(false); err != nil {
		t.Fatalf("runHooks(false) returned error: %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed reading hook output: %v", err)
	}

	if string(content) != "prod-user\nprod-framework\n" {
		t.Fatalf("unexpected hook output:\n%s", string(content))
	}
}

func TestRunHooks_FailFastOnUserHookError(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	outPath := filepath.Join(root, "should-not-exist.log")
	cfg.Core.DevBuildHook = "false"
	cfg.FrameworkDevBuildHook = "printf 'framework\\n' >> " + strconv.Quote(outPath)

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.runHooks(true)
	if err == nil {
		t.Fatal("expected runHooks to fail on user hook error, got nil")
	}
	if !strings.Contains(err.Error(), "user build hook failed") {
		t.Fatalf("unexpected error message: %v", err)
	}

	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected framework hook not to run, stat error: %v", statErr)
	}
}

func TestRunHooks_ReportsFrameworkHookErrorAfterUserHookRuns(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	outPath := filepath.Join(root, "hook-state.log")
	cfg.Core.DevBuildHook = "printf 'user\\n' >> " + strconv.Quote(outPath)
	cfg.FrameworkDevBuildHook = "false"

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.runHooks(true)
	if err == nil {
		t.Fatal("expected runHooks to fail on framework hook error, got nil")
	}
	if !strings.Contains(err.Error(), "framework build hook failed") {
		t.Fatalf("unexpected error message: %v", err)
	}

	content, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("expected user hook output to be written before framework failure: %v", readErr)
	}
	if string(content) != "user\n" {
		t.Fatalf("unexpected content after framework failure:\n%s", string(content))
	}
}

func TestRunHooks_DevHookTimeoutStopsUserHookQuickly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.DevBuildHookTimeoutMilliseconds = 100

	frameworkMarkerPath := filepath.Join(root, "framework-should-not-run.log")
	cfg.Core.DevBuildHook = "sleep 2"
	cfg.FrameworkDevBuildHook = "printf 'framework\\n' >> " + strconv.Quote(frameworkMarkerPath)

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	hookStartTime := time.Now()
	err := builder.runHooks(true)
	hookElapsedTime := time.Since(hookStartTime)
	if err == nil {
		t.Fatal("expected runHooks(true) to fail when dev hook times out")
	}
	if !errors.Is(err, executil.ErrCommandExecutionTimedOut) {
		t.Fatalf("expected timed-out command classification, got %v", err)
	}
	if !strings.Contains(err.Error(), "user build hook failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if hookElapsedTime > 1*time.Second {
		t.Fatalf("expected timed-out dev hook to stop quickly, elapsed=%s", hookElapsedTime)
	}
	if _, statErr := os.Stat(frameworkMarkerPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected framework hook not to run after timed-out dev hook, stat error: %v", statErr)
	}
}

func TestRunHooks_ProdHookTimeoutStopsFrameworkHookQuickly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ProdBuildHookTimeoutMilliseconds = 100

	userMarkerPath := filepath.Join(root, "prod-user-ran.log")
	cfg.Core.ProdBuildHook = "printf 'prod-user\\n' >> " + strconv.Quote(userMarkerPath)
	cfg.FrameworkProdBuildHook = "sleep 2"

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	hookStartTime := time.Now()
	err := builder.runHooks(false)
	hookElapsedTime := time.Since(hookStartTime)
	if err == nil {
		t.Fatal("expected runHooks(false) to fail when prod framework hook times out")
	}
	if !errors.Is(err, executil.ErrCommandExecutionTimedOut) {
		t.Fatalf("expected timed-out command classification, got %v", err)
	}
	if !strings.Contains(err.Error(), "framework build hook failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if hookElapsedTime > 1*time.Second {
		t.Fatalf("expected timed-out prod framework hook to stop quickly, elapsed=%s", hookElapsedTime)
	}

	userOutput, readUserOutputError := os.ReadFile(userMarkerPath)
	if readUserOutputError != nil {
		t.Fatalf("expected user prod hook to run before framework timeout: %v", readUserOutputError)
	}
	if string(userOutput) != "prod-user\n" {
		t.Fatalf("unexpected user prod hook output before framework timeout:\n%s", string(userOutput))
	}
}

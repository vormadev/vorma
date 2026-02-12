package tooling

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
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

package builder

import (
	"context"
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

func TestRunBuildHooks_DevRunsUserThenFramework(t *testing.T) {
	root := t.TempDir()
	config := newParsedConfigForBuilderBasicTestsAtRoot(root)

	orderPath := filepath.Join(root, "hook-order.log")
	config.Core.DevBuildHook = "printf 'user\\n' >> " + strconv.Quote(orderPath)
	config.FrameworkDevBuildHook = "printf 'framework\\n' >> " + strconv.Quote(
		orderPath,
	)

	builderForTest := NewBuilder(config, newDiscardLoggerForBuilderBasicTests())
	defer builderForTest.Close()

	if runHooksError := builderForTest.runBuildHooks(true); runHooksError != nil {
		t.Fatalf("runBuildHooks(true) returned error: %v", runHooksError)
	}

	content, readError := os.ReadFile(orderPath)
	if readError != nil {
		t.Fatalf("failed reading hook order file: %v", readError)
	}

	if string(content) != "user\nframework\n" {
		t.Fatalf("unexpected hook order:\n%s", string(content))
	}
}

func TestRunBuildHooks_ProdUsesProdHooks(t *testing.T) {
	root := t.TempDir()
	config := newParsedConfigForBuilderBasicTestsAtRoot(root)

	outPath := filepath.Join(root, "prod-hook.log")
	config.Core.ProdBuildHook = "printf 'prod-user\\n' >> " + strconv.Quote(
		outPath,
	)
	config.FrameworkProdBuildHook = "printf 'prod-framework\\n' >> " + strconv.Quote(
		outPath,
	)

	builderForTest := NewBuilder(config, newDiscardLoggerForBuilderBasicTests())
	defer builderForTest.Close()

	if runHooksError := builderForTest.runBuildHooks(false); runHooksError != nil {
		t.Fatalf("runBuildHooks(false) returned error: %v", runHooksError)
	}

	content, readError := os.ReadFile(outPath)
	if readError != nil {
		t.Fatalf("failed reading hook output: %v", readError)
	}

	if string(content) != "prod-user\nprod-framework\n" {
		t.Fatalf("unexpected hook output:\n%s", string(content))
	}
}

func TestRunBuildHooks_FailFastOnUserHookError(t *testing.T) {
	root := t.TempDir()
	config := newParsedConfigForBuilderBasicTestsAtRoot(root)

	outPath := filepath.Join(root, "should-not-exist.log")
	config.Core.DevBuildHook = "false"
	config.FrameworkDevBuildHook = "printf 'framework\\n' >> " + strconv.Quote(
		outPath,
	)

	builderForTest := NewBuilder(config, newDiscardLoggerForBuilderBasicTests())
	defer builderForTest.Close()

	runHooksError := builderForTest.runBuildHooks(true)
	if runHooksError == nil {
		t.Fatal("expected runHooks to fail on user hook error, got nil")
	}
	if !strings.Contains(runHooksError.Error(), "user build hook failed") {
		t.Fatalf("unexpected error message: %v", runHooksError)
	}

	if _, statError := os.Stat(outPath); !os.IsNotExist(statError) {
		t.Fatalf(
			"expected framework hook not to run, stat error: %v",
			statError,
		)
	}
}

func TestRunBuildHooks_ReportsFrameworkHookErrorAfterUserHookRuns(
	t *testing.T,
) {
	root := t.TempDir()
	config := newParsedConfigForBuilderBasicTestsAtRoot(root)

	outPath := filepath.Join(root, "hook-state.log")
	config.Core.DevBuildHook = "printf 'user\\n' >> " + strconv.Quote(outPath)
	config.FrameworkDevBuildHook = "false"

	builderForTest := NewBuilder(config, newDiscardLoggerForBuilderBasicTests())
	defer builderForTest.Close()

	runHooksError := builderForTest.runBuildHooks(true)
	if runHooksError == nil {
		t.Fatal("expected runHooks to fail on framework hook error, got nil")
	}
	if !strings.Contains(runHooksError.Error(), "framework build hook failed") {
		t.Fatalf("unexpected error message: %v", runHooksError)
	}

	content, readError := os.ReadFile(outPath)
	if readError != nil {
		t.Fatalf(
			"expected user hook output to be written before framework failure: %v",
			readError,
		)
	}
	if string(content) != "user\n" {
		t.Fatalf(
			"unexpected content after framework failure:\n%s",
			string(content),
		)
	}
}

func TestRunBuildHooks_UsesFrameworkBuildHookRunnerWhenConfigured(
	t *testing.T,
) {
	root := t.TempDir()
	config := newParsedConfigForBuilderBasicTestsAtRoot(root)

	outPath := filepath.Join(root, "hook-state.log")
	config.Core.DevBuildHook = "printf 'user\\n' >> " + strconv.Quote(outPath)
	config.FrameworkDevBuildHook = "false"

	frameworkRunnerCalled := false
	config.FrameworkRunBuildHook = func(commandExecutionContext context.Context, runInDevelopmentMode bool) error {
		frameworkRunnerCalled = true
		if commandExecutionContext == nil {
			t.Fatal("expected non-nil framework runner context")
		}
		if !runInDevelopmentMode {
			t.Fatal(
				"expected framework runner to receive runInDevelopmentMode=true",
			)
		}
		frameworkOutputFile, openFileError := os.OpenFile(
			outPath,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0o644,
		)
		if openFileError != nil {
			return openFileError
		}
		defer frameworkOutputFile.Close()
		if _, writeStringError := frameworkOutputFile.WriteString("framework-runner\n"); writeStringError != nil {
			return writeStringError
		}
		return nil
	}

	builderForTest := NewBuilder(config, newDiscardLoggerForBuilderBasicTests())
	defer builderForTest.Close()

	if runHooksError := builderForTest.runBuildHooks(true); runHooksError != nil {
		t.Fatalf("runBuildHooks(true) returned error: %v", runHooksError)
	}
	if !frameworkRunnerCalled {
		t.Fatal("expected configured framework build hook runner to be called")
	}

	content, readError := os.ReadFile(outPath)
	if readError != nil {
		t.Fatalf("failed reading hook output: %v", readError)
	}
	if string(content) != "user\nframework-runner\n" {
		t.Fatalf("unexpected hook output:\n%s", string(content))
	}
}

func TestRunBuildHooks_DevHookTimeoutStopsUserHookQuickly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	root := t.TempDir()
	config := newParsedConfigForBuilderBasicTestsAtRoot(root)
	config.Core.DevBuildHookTimeoutMilliseconds = 100

	frameworkMarkerPath := filepath.Join(root, "framework-should-not-run.log")
	config.Core.DevBuildHook = "sleep 2"
	config.FrameworkDevBuildHook = "printf 'framework\\n' >> " + strconv.Quote(
		frameworkMarkerPath,
	)

	builderForTest := NewBuilder(config, newDiscardLoggerForBuilderBasicTests())
	defer builderForTest.Close()

	hookStartTime := time.Now()
	runHooksError := builderForTest.runBuildHooks(true)
	hookElapsedTime := time.Since(hookStartTime)
	if runHooksError == nil {
		t.Fatal("expected runBuildHooks(true) to fail when dev hook times out")
	}
	if !errors.Is(runHooksError, executil.ErrCommandExecutionTimedOut) {
		t.Fatalf(
			"expected timed-out command classification, got %v",
			runHooksError,
		)
	}
	if !strings.Contains(runHooksError.Error(), "user build hook failed") {
		t.Fatalf("unexpected error message: %v", runHooksError)
	}
	if hookElapsedTime > 1*time.Second {
		t.Fatalf(
			"expected timed-out dev hook to stop quickly, elapsed=%s",
			hookElapsedTime,
		)
	}
	if _, statError := os.Stat(frameworkMarkerPath); !os.IsNotExist(statError) {
		t.Fatalf(
			"expected framework hook not to run after timed-out dev hook, stat error: %v",
			statError,
		)
	}
}

func TestRunBuildHooks_ProdHookTimeoutStopsFrameworkHookQuickly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	root := t.TempDir()
	config := newParsedConfigForBuilderBasicTestsAtRoot(root)
	config.Core.ProdBuildHookTimeoutMilliseconds = 100

	userMarkerPath := filepath.Join(root, "prod-user-ran.log")
	config.Core.ProdBuildHook = "printf 'prod-user\\n' >> " + strconv.Quote(
		userMarkerPath,
	)
	config.FrameworkProdBuildHook = "sleep 2"

	builderForTest := NewBuilder(config, newDiscardLoggerForBuilderBasicTests())
	defer builderForTest.Close()

	hookStartTime := time.Now()
	runHooksError := builderForTest.runBuildHooks(false)
	hookElapsedTime := time.Since(hookStartTime)
	if runHooksError == nil {
		t.Fatal(
			"expected runBuildHooks(false) to fail when prod framework hook times out",
		)
	}
	if !errors.Is(runHooksError, executil.ErrCommandExecutionTimedOut) {
		t.Fatalf(
			"expected timed-out command classification, got %v",
			runHooksError,
		)
	}
	if !strings.Contains(runHooksError.Error(), "framework build hook failed") {
		t.Fatalf("unexpected error message: %v", runHooksError)
	}
	if hookElapsedTime > 1*time.Second {
		t.Fatalf(
			"expected timed-out prod framework hook to stop quickly, elapsed=%s",
			hookElapsedTime,
		)
	}

	userOutput, readUserOutputError := os.ReadFile(userMarkerPath)
	if readUserOutputError != nil {
		t.Fatalf(
			"expected user prod hook to run before framework timeout: %v",
			readUserOutputError,
		)
	}
	if string(userOutput) != "prod-user\n" {
		t.Fatalf(
			"unexpected user prod hook output before framework timeout:\n%s",
			string(userOutput),
		)
	}
}

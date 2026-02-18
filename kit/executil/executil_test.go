package executil

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMakeCmdRunner(t *testing.T) {
	// Test running a simple command (e.g., echo)
	runner := MakeCmdRunner("echo", "hello, world")
	if err := runner(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Test running a command that fails
	runner = MakeCmdRunner("false") // "false" is a command that always exits with status 1
	if err := runner(); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestGetExecutableDir(t *testing.T) {
	// Get the current executable's directory
	execDir, err := GetExecutableDir()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify the returned directory is not empty
	if execDir == "" {
		t.Fatalf("expected non-empty executable directory")
	}

	// Verify that the returned path is a directory
	info, err := os.Stat(execDir)
	if err != nil {
		t.Fatalf("failed to stat the directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected a directory, got a non-directory")
	}
}

func TestEdgeCases(t *testing.T) {
	// Test MakeCmdRunner with a non-existent command
	runner := MakeCmdRunner("non_existent_command")
	if err := runner(); err == nil {
		t.Fatalf("expected error for non-existent command, got nil")
	}
}

func TestRunShellWithContext_CancelStopsCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	commandExecutionContext, cancelCommandExecutionContext := context.WithTimeout(
		context.Background(),
		100*time.Millisecond,
	)
	defer cancelCommandExecutionContext()

	commandStartTime := time.Now()
	err := RunShellWithContext(commandExecutionContext, "sleep 2")
	commandElapsedTime := time.Since(commandStartTime)

	if err == nil {
		t.Fatal("expected canceled shell command to return error")
	}
	if !errors.Is(err, ErrCommandExecutionTimedOut) {
		t.Fatalf("expected timed-out command classification, got %v", err)
	}
	if commandElapsedTime > 1*time.Second {
		t.Fatalf(
			"expected canceled shell command to stop quickly, elapsed=%s",
			commandElapsedTime,
		)
	}
}

func TestRunShellWithContext_CancelClassifiesCanceledCommand(t *testing.T) {
	commandExecutionContext, cancelCommandExecutionContext := context.WithCancel(
		context.Background(),
	)
	cancelCommandExecutionContext()

	err := RunShellWithContext(commandExecutionContext, "echo should-not-run")
	if err == nil {
		t.Fatal("expected canceled shell command to return error")
	}
	if !errors.Is(err, ErrCommandExecutionCanceled) {
		t.Fatalf("expected canceled command classification, got %v", err)
	}
}

func TestRunCmdCapture_ReturnsErrorWhenNoCommandProvided(t *testing.T) {
	_, err := RunCmdCapture()
	if err == nil {
		t.Fatal("expected error for empty command list")
	}
}

func TestRunCmd_ReturnsErrorWhenNoCommandProvided(t *testing.T) {
	err := RunCmd()
	if err == nil {
		t.Fatal("expected error for empty command list")
	}
}

func TestRunCmdCapture_CapturesOutputFromSuccessfulCommand(t *testing.T) {
	output, err := RunCmdCapture("echo", "hello")
	if err != nil {
		t.Fatalf("RunCmdCapture() unexpected error: %v", err)
	}
	if !strings.Contains(output, "hello") {
		t.Fatalf("expected output to contain %q, got %q", "hello", output)
	}
}

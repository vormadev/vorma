package cliutil

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestExit_WithNilError_UsesNonZeroExitCode(t *testing.T) {
	exitCode, output := runExitAndCaptureOutput(
		t,
		"aborted",
		nil,
	)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(output, "aborted") {
		t.Fatalf("expected output to include abort message, got %q", output)
	}
}

func TestExit_WithError_EmitsErrorPrefix(t *testing.T) {
	exitCode, output := runExitAndCaptureOutput(
		t,
		"failed to run command",
		errors.New("boom"),
	)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(output, "ERROR: failed to run command: ") {
		t.Fatalf("expected ERROR prefix in output, got %q", output)
	}
	if !strings.Contains(output, "boom") {
		t.Fatalf("expected wrapped error message in output, got %q", output)
	}
}

func runExitAndCaptureOutput(
	t *testing.T,
	message string,
	err error,
) (int, string) {
	t.Helper()

	originalProcessExit := processExit
	originalStdout := os.Stdout
	readPipe, writePipe, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe() error = %v", pipeErr)
	}

	capturedExitCode := -1
	processExit = func(code int) {
		capturedExitCode = code
	}

	os.Stdout = writePipe

	t.Cleanup(func() {
		processExit = originalProcessExit
		os.Stdout = originalStdout
	})

	Exit(message, err)

	if closeErr := writePipe.Close(); closeErr != nil {
		t.Fatalf("writePipe.Close() error = %v", closeErr)
	}

	outputBytes, readErr := io.ReadAll(readPipe)
	if readErr != nil {
		t.Fatalf("io.ReadAll() error = %v", readErr)
	}
	if closeErr := readPipe.Close(); closeErr != nil {
		t.Fatalf("readPipe.Close() error = %v", closeErr)
	}

	return capturedExitCode, string(outputBytes)
}

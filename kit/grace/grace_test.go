package grace

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// testLogger creates a logger that writes to a buffer for testing
func testLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	return logger, &buf
}

func TestOrchestrate_Defaults(t *testing.T) {
	logger, _ := testLogger()
	finished := make(chan struct{})

	go func() {
		opts := OrchestrateOptions{
			Logger: logger,
		}
		go func() {
			time.Sleep(100 * time.Millisecond)
			p, _ := os.FindProcess(os.Getpid())
			p.Signal(syscall.SIGTERM)
		}()
		Orchestrate(opts)
		close(finished)
	}()

	select {
	case <-finished:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Orchestrate did not shut down within expected timeframe")
	}
}

func TestOrchestrate_CustomOptions(t *testing.T) {
	logger, _ := testLogger()
	startupCalled := false
	cleanupCalled := false

	options := OrchestrateOptions{
		ShutdownTimeout: 2 * time.Second,
		Signals:         []os.Signal{syscall.SIGTERM},
		Logger:          logger,
		StartupCallback: func() error {
			startupCalled = true
			// Trigger shutdown after startup completes
			p, _ := os.FindProcess(os.Getpid())
			p.Signal(syscall.SIGTERM)
			return nil
		},
		ShutdownCallback: func(ctx context.Context) error {
			cleanupCalled = true
			return nil
		},
	}

	finished := make(chan struct{})
	go func() {
		Orchestrate(options)
		close(finished)
	}()

	select {
	case <-finished:
		if !startupCalled {
			t.Error("Startup callback was not called")
		}
		if !cleanupCalled {
			t.Error("Cleanup callback was not called")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Orchestrate did not shut down within expected timeframe")
	}
}

func TestOrchestrate_StartupError(t *testing.T) {
	logger, _ := testLogger()
	cleanupCalled := false
	expectedErr := errors.New("startup failure")

	options := OrchestrateOptions{
		Logger:          logger,
		ShutdownTimeout: time.Second,
		StartupCallback: func() error {
			return expectedErr
		},
		ShutdownCallback: func(ctx context.Context) error {
			cleanupCalled = true
			return nil
		},
	}

	finished := make(chan struct{})
	go func() {
		Orchestrate(options)
		close(finished)
	}()

	select {
	case <-finished:
		if !cleanupCalled {
			t.Error("Cleanup callback should be called even when startup fails")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Orchestrate did not shut down after startup error")
	}
}

func TestOrchestrate_CleanupTimeout(t *testing.T) {
	logger, logBuf := testLogger()
	cleanupStarted := make(chan struct{})

	options := OrchestrateOptions{
		Logger:          logger,
		ShutdownTimeout: 100 * time.Millisecond,
		StartupCallback: func() error {
			p, _ := os.FindProcess(os.Getpid())
			p.Signal(syscall.SIGTERM)
			return nil
		},
		ShutdownCallback: func(ctx context.Context) error {
			close(cleanupStarted)
			// Simulate slow cleanup
			time.Sleep(200 * time.Millisecond)
			return nil
		},
	}

	finished := make(chan struct{})
	go func() {
		Orchestrate(options)
		close(finished)
	}()

	select {
	case <-cleanupStarted:
		// Cleanup started
	case <-time.After(time.Second):
		t.Fatal("Cleanup was not triggered")
	}

	select {
	case <-finished:
		if !logContains(logBuf, "timed out") {
			t.Error("Expected timeout warning in logs")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Orchestrate did not shut down after cleanup timeout")
	}
}

func TestOrchestrate_CleanupTimeout_NonCooperativeCallback(t *testing.T) {
	logger, logBuf := testLogger()
	cleanupStarted := make(chan struct{})
	blockForever := make(chan struct{})

	options := OrchestrateOptions{
		Logger:          logger,
		ShutdownTimeout: 100 * time.Millisecond,
		StartupCallback: func() error {
			p, _ := os.FindProcess(os.Getpid())
			p.Signal(syscall.SIGTERM)
			return nil
		},
		ShutdownCallback: func(ctx context.Context) error {
			close(cleanupStarted)
			<-blockForever
			return nil
		},
	}

	finished := make(chan struct{})
	go func() {
		Orchestrate(options)
		close(finished)
	}()

	select {
	case <-cleanupStarted:
	case <-time.After(time.Second):
		t.Fatal("Cleanup was not triggered")
	}

	select {
	case <-finished:
		if !logContains(logBuf, "timed out") {
			t.Error("Expected timeout warning in logs")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Orchestrate did not return after timeout for non-cooperative callback")
	}
}

func TestTerminateProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	logger, _ := testLogger()

	tests := []struct {
		name      string
		timeout   time.Duration
		wantError bool
	}{
		{
			name:      "Graceful Termination",
			timeout:   time.Second,
			wantError: false,
		},
		{
			name:      "Force Kill",
			timeout:   100 * time.Millisecond,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("sleep", "10")
			if err := cmd.Start(); err != nil {
				t.Fatalf("Failed to start test process: %v", err)
			}

			err := TerminateProcess(cmd.Process, tt.timeout, logger)
			if (err != nil) != tt.wantError {
				t.Errorf("TerminateProcess() error = %v, wantError = %v", err, tt.wantError)
			}

			// Verify process is no longer running
			if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
				cmd.Process.Kill() // Cleanup
				t.Error("Process is still running after termination")
			}
		})
	}
}

func TestTerminateProcess_InvalidProcess(t *testing.T) {
	logger, _ := testLogger()
	// Try to terminate a non-existent process
	nonExistentPID := 99999999
	process, _ := os.FindProcess(nonExistentPID)
	err := TerminateProcess(process, time.Second, logger)
	if err == nil {
		t.Error("Expected error when terminating non-existent process")
	}
}

func TestNewDefaultLogger(t *testing.T) {
	logger := newDefaultLogger()
	if logger == nil {
		t.Error("newDefaultLogger() returned nil")
	}
}

// Helper function to check if a log buffer contains a string
func logContains(buf *bytes.Buffer, s string) bool {
	return bytes.Contains(buf.Bytes(), []byte(s))
}

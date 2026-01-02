package grace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/vormadev/vorma/kit/colorlog"
)

func defaultSignals() []os.Signal {
	if runtime.GOOS == "windows" {
		return []os.Signal{os.Interrupt}
	}
	return []os.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}
}

type OrchestrateOptions struct {
	ShutdownTimeout  time.Duration // Default: 30 seconds
	Signals          []os.Signal   // Default: SIGHUP, SIGINT, SIGTERM, SIGQUIT
	Logger           *slog.Logger  // Default: os.Stdout
	StartupCallback  func() error
	ShutdownCallback func(context.Context) error
}

// Orchestrate manages the core lifecycle of an application, including startup, shutdown, and os signal handling.
func Orchestrate(options OrchestrateOptions) {
	// Set defaults
	if options.Logger == nil {
		options.Logger = newDefaultLogger()
	}
	if options.ShutdownTimeout == 0 {
		options.ShutdownTimeout = 30 * time.Second
	}
	if len(options.Signals) == 0 {
		options.Signals = defaultSignals()
	}

	// Context for orchestrating shutdown
	ctx, stopCtx := context.WithCancel(context.Background())
	defer stopCtx()

	// Signal handling
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, options.Signals...)
	defer signal.Stop(sig)

	// Create a channel to coordinate cleanup
	cleanup := make(chan struct{})

	// Handle cleanup in a separate goroutine
	go func() {
		select {
		case receivedSignal := <-sig:
			options.Logger.Info(fmt.Sprintf("[shutdown] Signal received: %v. Initiating graceful shutdown.", receivedSignal))
		case <-ctx.Done():
			options.Logger.Info("[shutdown] Initiating graceful shutdown due to startup failure")
		}

		shutdownCtx, cancelCtx := context.WithTimeout(context.Background(), options.ShutdownTimeout)
		defer cancelCtx()

		// Execute shutdown logic (cleanup tasks)
		if options.ShutdownCallback != nil {
			if err := options.ShutdownCallback(shutdownCtx); err != nil {
				options.Logger.Error(fmt.Sprintf("[shutdown] Cleanup error: %v", err))
			}
		}

		if shutdownCtx.Err() == context.DeadlineExceeded {
			options.Logger.Warn("[shutdown] Graceful shutdown timed out. Forcing exit.")
		}

		close(cleanup)
	}()

	// Execute startup logic
	if options.StartupCallback != nil {
		if err := options.StartupCallback(); err != nil {
			options.Logger.Error(fmt.Sprintf("[startup] Error: %v", err))
			stopCtx() // This will now trigger cleanup via ctx.Done()
		}
	}

	// Wait for cleanup to complete
	<-cleanup
}

// TerminateProcess attempts to gracefully terminate a process, falling back to force kill after timeout.
// If logger is nil, defaults to stdout.
func TerminateProcess(process *os.Process, timeToWait time.Duration, logger *slog.Logger) error {
	if logger == nil {
		logger = newDefaultLogger()
	}

	var err error
	if runtime.GOOS == "windows" {
		err = process.Kill()
	} else {
		err = process.Signal(syscall.SIGTERM)
	}

	if err != nil {
		return fmt.Errorf("failed to send termination signal: %w", err)
	}

	done := make(chan error)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("process exited with error: %w", err)
		}
		return nil
	case <-time.After(timeToWait):
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process after timeout: %w", err)
		}
		pid := process.Pid
		logger.Warn(fmt.Sprintf("process %d killed after timeout of %v", pid, timeToWait))
		return nil
	}
}

func newDefaultLogger() *slog.Logger {
	return colorlog.New("grace")
}

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
	ShutdownTimeout time.Duration // Default: 30 seconds
	Signals         []os.Signal   // Default: SIGHUP, SIGINT, SIGTERM, SIGQUIT
	Logger          *slog.Logger  // Default: os.Stdout

	// StartupCallback runs your main application logic (e.g., server.ListenAndServe).
	// This callback should block until the application is ready to shut down.
	// Do not call os.Exit or log.Fatal here; return an error instead.
	StartupCallback func() error

	// ShutdownCallback runs cleanup logic (e.g., server.Shutdown, closing DB connections).
	// The context has a timeout based on ShutdownTimeout.
	// Do not call os.Exit or log.Fatal here; return an error instead.
	ShutdownCallback func(context.Context) error
}

// Orchestrate manages the core lifecycle of an application, including startup, shutdown, and os signal handling.
// StartupCallback is expected to block (e.g., http.Server.ListenAndServe). If it returns immediately,
// Orchestrate will wait for a shutdown signal before exiting.
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
			options.Logger.Info("[shutdown] Signal received, initiating graceful shutdown", "signal", receivedSignal)
		case <-ctx.Done():
			options.Logger.Info("[shutdown] Initiating graceful shutdown due to startup failure")
		}

		shutdownCtx, cancelCtx := context.WithTimeout(context.Background(), options.ShutdownTimeout)
		defer cancelCtx()

		// Execute shutdown logic (cleanup tasks)
		if options.ShutdownCallback != nil {
			if err := options.ShutdownCallback(shutdownCtx); err != nil {
				options.Logger.Error("[shutdown] Cleanup error", "error", err)
			}
		}

		if shutdownCtx.Err() == context.DeadlineExceeded {
			options.Logger.Warn("[shutdown] Graceful shutdown timed out, forcing exit")
		}

		close(cleanup)
	}()

	// Execute startup logic
	if options.StartupCallback != nil {
		if err := options.StartupCallback(); err != nil {
			options.Logger.Error("[startup] Error", "error", err)
			stopCtx() // This will trigger cleanup via ctx.Done()
			<-cleanup
			return
		}
	}

	// Wait for signal and cleanup to complete
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
		logger.Warn("Process killed after timeout", "pid", process.Pid, "timeout", timeToWait)
		return nil
	}
}

func newDefaultLogger() *slog.Logger {
	return colorlog.New("grace")
}

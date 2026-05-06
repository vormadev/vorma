package grace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/vormadev/vorma/kit/colorlog"
)

var errProcessIsNil = errors.New("process is nil")

func defaultSignals() []os.Signal {
	if runtime.GOOS == "windows" {
		return []os.Signal{os.Interrupt}
	}
	return []os.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}
}

type Lifecycle struct {
	ShutdownTimeout time.Duration // Default: 30 seconds
	Signals         []os.Signal   // Default: SIGHUP, SIGINT, SIGTERM, SIGQUIT
	Logger          *slog.Logger  // Default: os.Stdout

	// Startup runs your main application logic (e.g., server.ListenAndServe).
	// This callback should block until the application is ready to shut down.
	// Do not call os.Exit or log.Fatal here; return an error instead.
	Startup func() error

	// Shutdown runs cleanup logic (e.g., server.Shutdown, closing DB connections).
	// The context has a timeout based on ShutdownTimeout.
	// Do not call os.Exit or log.Fatal here; return an error instead.
	Shutdown func(context.Context) error
}

// Run manages the core lifecycle of an application, including startup, shutdown, and os signal handling.
// Lifecycle.Startup is expected to block (e.g., http.Server.ListenAndServe). If it returns immediately,
// Run will wait for a shutdown signal before exiting.
func (l Lifecycle) Run() { Run(l) }

// Run manages the core lifecycle of an application, including startup, shutdown, and os signal handling.
// Lifecycle.Startup is expected to block (e.g., http.Server.ListenAndServe). If it returns immediately,
// Run will wait for a shutdown signal before exiting.
func Run(l Lifecycle) {
	// Set defaults
	if l.Logger == nil {
		l.Logger = newDefaultLogger()
	}
	if l.ShutdownTimeout == 0 {
		l.ShutdownTimeout = 30 * time.Second
	}
	if len(l.Signals) == 0 {
		l.Signals = defaultSignals()
	}

	// Context for orchestrating shutdown
	ctx, stopCtx := context.WithCancel(context.Background())
	defer stopCtx()

	// Signal handling
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, l.Signals...)
	defer signal.Stop(sig)

	// Create a channel to coordinate cleanup
	cleanup := make(chan struct{})

	// Handle cleanup in a separate goroutine
	go func() {
		select {
		case receivedSignal := <-sig:
			l.Logger.Info(
				"[shutdown] Signal received, initiating graceful shutdown",
				"signal",
				receivedSignal,
			)
		case <-ctx.Done():
			l.Logger.Info("[shutdown] Initiating graceful shutdown due to startup failure")
		}

		shutdownCtx, cancelCtx := context.WithTimeout(context.Background(), l.ShutdownTimeout)
		defer cancelCtx()

		// Execute shutdown logic (cleanup tasks)
		timedOut := false
		if l.Shutdown != nil {
			done := make(chan error, 1)
			go func() {
				done <- l.Shutdown(shutdownCtx)
			}()

			select {
			case err := <-done:
				if err != nil {
					l.Logger.Error("[shutdown] Cleanup error", "error", err)
				}
			case <-shutdownCtx.Done():
				// Allow Orchestrate to continue even if callback ignores context.
				l.Logger.Warn("[shutdown] Graceful shutdown timed out, forcing exit")
				timedOut = true
			}
		}

		if !timedOut && shutdownCtx.Err() == context.DeadlineExceeded {
			l.Logger.Warn("[shutdown] Graceful shutdown timed out, forcing exit")
		}

		close(cleanup)
	}()

	// Execute startup logic
	if l.Startup != nil {
		if err := l.Startup(); err != nil {
			l.Logger.Error("[startup] Error", "error", err)
			stopCtx() // This will trigger cleanup via ctx.Done()
			<-cleanup
			return
		}
	}

	// Wait for signal and cleanup to complete
	<-cleanup
}

// Terminate attempts to gracefully terminate a process, falling back to force kill after timeout.
// If logger is nil, defaults to stdout.
func Terminate(process *os.Process, waitFor time.Duration, logger *slog.Logger) error {
	if process == nil {
		return errProcessIsNil
	}

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
	case <-time.After(waitFor):
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process after timeout: %w", err)
		}
		logger.Warn("Process killed after timeout", "pid", process.Pid, "timeout", waitFor)
		return nil
	}
}

func newDefaultLogger() *slog.Logger {
	return colorlog.New("grace")
}

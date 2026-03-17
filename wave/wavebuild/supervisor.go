package wavebuild

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/vormadev/vorma/kit/procutil"
	"github.com/vormadev/vorma/kit/strict"
)

const (
	supervisor_grace_period  = 2 * time.Second
	supervisor_ready_timeout = 10 * time.Second
)

type supervisor struct {
	bin_out_path_abs strict.MachAbsPath
	port             int
	health           string   // endpoint path, e.g. "/"
	env              []string // extra env vars appended to os.Environ()
	pid_file         string
	logger           *slog.Logger

	mu       sync.Mutex
	cmd      *exec.Cmd
	done     chan struct{} // closed when process exits
	exit_err error         // written before done is closed; safe to read after <-done
}

// update refreshes the supervisor's binary path and env vars.
// Called before restart when config may have changed.
func (sv *supervisor) update(
	bin_out_path_abs strict.MachAbsPath,
	health string,
	env []string,
) {
	sv.mu.Lock()
	sv.bin_out_path_abs = bin_out_path_abs
	sv.health = health
	sv.env = env
	sv.mu.Unlock()
}

func (sv *supervisor) start() error {
	sv.mu.Lock()

	if sv.cmd != nil {
		sv.mu.Unlock()
		return fmt.Errorf("supervisor: process already running")
	}

	cmd := exec.Command(sv.bin_out_path_abs.Str())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), sv.env...)
	cmd.SysProcAttr = procutil.SysProcAttr()

	if err := cmd.Start(); err != nil {
		sv.mu.Unlock()
		return fmt.Errorf("Failed to start app server: %w", err)
	}

	if err := write_pid_file(sv.pid_file, cmd.Process.Pid); err != nil {
		sv.logger.Warn("Failed to write app pid file", "err", err)
	}

	sv.logger.Info("Starting app server",
		"binary", sv.bin_out_path_abs,
		"pid", cmd.Process.Pid,
	)

	sv.cmd = cmd
	done := make(chan struct{})
	sv.done = done

	go func() {
		// exit_err is written before close(done), so any reader
		// that observes <-done is guaranteed to see the value.
		sv.exit_err = cmd.Wait()
		close(done)
	}()

	sv.mu.Unlock()

	// poll healthcheck without holding the lock — this allows
	// stop() to acquire the lock if wave is shutting down.
	if err := poll_http_ready_endpoint(
		done,
		&sv.exit_err,
		fmt.Sprintf("http://localhost:%d%s", sv.port, sv.health),
		"app server",
		supervisor_ready_timeout,
	); err != nil {
		sv.stop()
		return err
	}

	sv.logger.Info(
		"⟶ App server ready",
		"url",
		fmt.Sprintf("http://localhost:%d", sv.port),
	)
	return nil
}

func (sv *supervisor) stop() {
	sv.mu.Lock()
	if sv.cmd == nil || sv.cmd.Process == nil {
		sv.mu.Unlock()
		return
	}

	pid := sv.cmd.Process.Pid
	done := sv.done
	sv.mu.Unlock()

	sv.logger.Info("Stopping app server")

	// graceful shutdown of the process group (child + any subprocesses)
	procutil.RequestStop(pid)

	select {
	case <-done:
		sv.logger.Info("App server stopped")
	case <-time.After(supervisor_grace_period):
		sv.logger.Info(
			"App server did not stop within grace period. Killing instead.",
		)
		procutil.ForceKill(pid)
		<-done
	}

	remove_pid_file(sv.pid_file)

	sv.mu.Lock()
	sv.cmd = nil
	sv.done = nil
	sv.exit_err = nil
	sv.mu.Unlock()
}

func poll_http_ready_endpoint(
	done chan struct{},
	exit_err *error,
	url string,
	process_label string,
	timeout time.Duration,
) error {
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(timeout)

	current_delay := 0 * time.Millisecond
	delay_inc_amt := 20 * time.Millisecond
	max_delay := 200 * time.Millisecond

	for time.Now().Before(deadline) {
		select {
		case <-done:
			if *exit_err != nil {
				return fmt.Errorf(
					"%s exited during startup: %w",
					process_label,
					*exit_err,
				)
			}
			return fmt.Errorf(
				"%s exited during startup (exit 0)",
				process_label,
			)
		default:
		}

		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		current_delay = min(current_delay+delay_inc_amt, max_delay)
		time.Sleep(current_delay)
	}

	return fmt.Errorf(
		"%s not ready after %s (endpoint: %s)",
		process_label,
		timeout,
		url,
	)
}

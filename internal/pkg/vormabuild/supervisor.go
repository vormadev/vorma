package vormabuild

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/vormadev/vorma/internal/pkg/vormarun"
	"github.com/vormadev/vorma/kit/netutil"
	"github.com/vormadev/vorma/kit/procutil"
)

/////////////////////////////////////////////////////////////////////
/////// APP SERVER -- START
/////////////////////////////////////////////////////////////////////

// Acquires lock.
func (rs *run_state) start_app_server() error {
	cfg, err := rs.get_config()
	if err != nil {
		return fmt.Errorf("error getting config: %w", err)
	}

	rs.mu.Lock()
	sv := rs.app_server_sv
	if sv != nil && sv.is_running() {
		rs.mu.Unlock()
		return nil // already running
	}
	build_ctx := rs.build_ctx
	rs.app_server_sv = &supervisor{}
	sv = rs.app_server_sv
	rs.mu.Unlock()

	opts := sv_start_opts{
		ctx:            build_ctx,
		preferred_port: 8080,
		make_cmd: func(ctx context.Context, port int) *exec.Cmd {
			env := []string{
				env_item_int("PORT", port),
				env_item_str(vormarun.Env_Key_Is_Dev, fmt.Sprintf("%t", rs.is_dev)),
			}
			return cfg.run_app_server_cmd(ctx, env)
		},
		ready_endpoint: "/.vorma/healthz",
	}

	if err := sv.start(opts); err != nil {
		return fmt.Errorf("error starting app server: %w", err)
	}

	rs.log.Info(fmt.Sprintf(
		"App server ready: http://localhost:%d",
		sv.port(),
	))

	return nil
}

/////////////////////////////////////////////////////////////////////
/////// VITE SERVER -- START
/////////////////////////////////////////////////////////////////////

// Acquires lock.
func (rs *run_state) start_vite_server() error {
	cfg, err := rs.get_config()
	if err != nil {
		return fmt.Errorf("error getting config: %w", err)
	}

	rs.mu.Lock()
	sv := rs.vite_server_sv
	if sv != nil && sv.is_running() {
		rs.mu.Unlock()
		return nil // already running
	}
	build_ctx := rs.build_ctx
	rs.vite_server_sv = &supervisor{}
	sv = rs.vite_server_sv
	dev_mux_port := rs.dev_mux_port
	rs.mu.Unlock()

	opts := sv_start_opts{
		ctx:            build_ctx,
		preferred_port: 5173,
		make_cmd: func(ctx context.Context, port int) *exec.Cmd {
			return cfg.run_vite_server_cmd(ctx, port, dev_mux_port)
		},
		ready_endpoint: "/@vite/client",
	}

	if err := sv.start(opts); err != nil {
		return fmt.Errorf("error starting server: %w", err)
	}

	if err := rs.write_manifest(); err != nil {
		return fmt.Errorf("error writing manifest: %w", err)
	}

	return nil
}

/////////////////////////////////////////////////////////////////////
/////// LOW-LEVEL SUPERVISOR
/////////////////////////////////////////////////////////////////////

type supervisor struct {
	mu         sync.Mutex
	_port      int
	process    *os.Process
	done       chan struct{}
	exit_err   error
	start_opts sv_start_opts
}

type sv_start_opts struct {
	ctx            context.Context
	preferred_port int
	make_cmd       func(ctx context.Context, port int) *exec.Cmd
	ready_endpoint string
}

// Acquires lock.
func (sv *supervisor) is_running() bool {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	return sv.process != nil
}

// Acquires lock.
func (sv *supervisor) port() int {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	return sv._port
}

// Acquires lock.
func (sv *supervisor) start(opts sv_start_opts) error {
	sv.mu.Lock()

	if sv.process != nil {
		sv.mu.Unlock()
		return nil // already running
	}

	sv.start_opts = opts
	sv.mu.Unlock()

	port, err := netutil.GetFreePort(opts.preferred_port)
	if err != nil {
		return fmt.Errorf("error getting free port: %w", err)
	}

	cmd := opts.make_cmd(opts.ctx, port)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting process: %w", err)
	}

	sv.mu.Lock()
	sv._port = port
	sv.process = cmd.Process
	sv.done = make(chan struct{})

	go func() {
		sv.exit_err = cmd.Wait()
		close(sv.done)
	}()

	done := sv.done
	exit_err := &sv.exit_err
	sv.mu.Unlock()

	if err := poll_http_ready_endpoint(
		opts.ctx,
		done,
		exit_err,
		fmt.Sprintf("http://localhost:%d%s", port, opts.ready_endpoint),
	); err != nil {
		sv.stop(true)
		return err
	}

	return nil
}

// Acquires lock.
func (sv *supervisor) stop(force bool) {
	sv.mu.Lock()

	if sv.process == nil {
		sv.mu.Unlock()
		return
	}

	pid := sv.process.Pid
	done := sv.done
	sv.mu.Unlock()

	if force {
		procutil.ForceKill(pid)
		<-done
	} else {
		procutil.RequestStop(pid)

		select {
		case <-done:
		case <-time.After(supervisor_shutdown_grace_period):
			procutil.ForceKill(pid)
			<-done
		}
	}

	sv.mu.Lock()
	sv.process = nil
	sv.done = nil
	sv.exit_err = nil
	sv.mu.Unlock()
}

/////////////////////////////////////////////////////////////////////
/////// POLL HTTP READY ENDPOINT
/////////////////////////////////////////////////////////////////////

func poll_http_ready_endpoint(
	ctx context.Context,
	done chan struct{},
	exit_err *error,
	url string,
) error {
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(supervisor_ready_timeout)

	current_delay := 0 * time.Millisecond
	delay_inc_amt := 20 * time.Millisecond
	max_delay := 200 * time.Millisecond

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return fmt.Errorf(
				"process exited before becoming ready: %w",
				*exit_err,
			)
		default:
			resp, err := client.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil // ready!
				}
			}
			current_delay = min(current_delay+delay_inc_amt, max_delay)
			time.Sleep(current_delay)
		}
	}

	return fmt.Errorf("process did not become ready in time")
}

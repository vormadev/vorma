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
	"github.com/vormadev/vorma/kit/envutil"
	"github.com/vormadev/vorma/kit/netutil"
	"github.com/vormadev/vorma/kit/procutil"
)

const supervisor_app_server_name = "app server"
const supervisor_vite_server_name = "Vite server"

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
		preferred_port: envutil.GetInt(dev_app_server_preferred_port_env_key, 8080),
		make_cmd: func(ctx context.Context, port int) *exec.Cmd {
			env := []string{
				env_item_int("PORT", port),
				env_item_str(vormarun.Env_Key_Is_Dev, fmt.Sprintf("%t", rs.is_dev)),
				env_item_str(vormarun.Env_Key_Is_Build, ""),
			}
			return cfg.run_app_server_cmd(ctx, env)
		},
		ready_endpoint: "/.vorma/healthz",
		on_unexpected_exit: func(err error) {
			rs.on_child_process_exit(supervisor_app_server_name, err)
		},
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
		preferred_port: envutil.GetInt(dev_vite_server_preferred_port_env_key, 5173),
		make_cmd: func(ctx context.Context, port int) *exec.Cmd {
			return cfg.run_vite_server_cmd(ctx, port, dev_mux_port)
		},
		ready_endpoint: "/@vite/client",
		on_unexpected_exit: func(err error) {
			rs.on_child_process_exit(supervisor_vite_server_name, err)
		},
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
	stopping   bool
	start_opts sv_start_opts
}

type sv_start_opts struct {
	ctx                context.Context
	preferred_port     int
	make_cmd           func(ctx context.Context, port int) *exec.Cmd
	ready_endpoint     string
	on_unexpected_exit func(error)
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
	sv.exit_err = nil
	sv.stopping = false

	done := sv.done
	on_unexpected_exit := opts.on_unexpected_exit
	go func() {
		err := cmd.Wait()

		sv.mu.Lock()
		sv.exit_err = err
		should_notify := !sv.stopping && opts.ctx.Err() == nil
		close(done)
		sv.mu.Unlock()

		if should_notify && on_unexpected_exit != nil {
			on_unexpected_exit(err)
		}
	}()

	sv.mu.Unlock()

	if err := poll_http_ready_endpoint(
		opts.ctx,
		done,
		sv.exit_error,
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
	sv.stopping = true
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
	sv.stopping = false
	sv.mu.Unlock()
}

func (sv *supervisor) exit_error() error {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	return sv.exit_err
}

/////////////////////////////////////////////////////////////////////
/////// POLL HTTP READY ENDPOINT
/////////////////////////////////////////////////////////////////////

func poll_http_ready_endpoint(
	ctx context.Context,
	done chan struct{},
	exit_err func() error,
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
				exit_err(),
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

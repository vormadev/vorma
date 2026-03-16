package wavebuild

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vormadev/vorma/kit/procutil"
)

const vite_grace_period = 2 * time.Second

/////////////////////////////////////////////////////////////////////
/////// Vite build config (shared by dev supervisor and prod build)
/////////////////////////////////////////////////////////////////////

type vite_build_config struct {
	base_cmd    []string // e.g. ["pnpm"] or ["npx", "--yes"]
	cmd_dir     string   // working directory for the command
	config_file string   // optional vite config file path
}

func new_vite_build_config(
	pkg_manager_base_cmd string,
	cmd_dir string,
	config_file string,
) *vite_build_config {
	return &vite_build_config{
		base_cmd:    strings.Fields(pkg_manager_base_cmd),
		cmd_dir:     cmd_dir,
		config_file: config_file,
	}
}

/////////////////////////////////////////////////////////////////////
/////// Vite dev server supervisor
/////////////////////////////////////////////////////////////////////

type vite_supervisor struct {
	cfg      *vite_build_config
	port     int
	env      []string
	pid_file string
	logger   *slog.Logger

	mu       sync.Mutex
	cmd      *exec.Cmd
	done     chan struct{}
	exit_err error
	stopping bool // true when stop() is in progress
}

func new_vite_supervisor(
	cfg *vite_build_config,
	port int,
	env []string,
	pid_file string,
	logger *slog.Logger,
) *vite_supervisor {
	return &vite_supervisor{
		cfg:      cfg,
		port:     port,
		env:      env,
		pid_file: pid_file,
		logger:   logger,
	}
}

func (vs *vite_supervisor) start() error {
	vs.mu.Lock()
	vs.stopping = false

	if vs.cmd != nil {
		vs.mu.Unlock()
		return fmt.Errorf("vite_supervisor: process already running")
	}

	args := make([]string, len(vs.cfg.base_cmd))
	copy(args, vs.cfg.base_cmd)
	args = append(args, "vite",
		"--port", fmt.Sprintf("%d", vs.port),
		"--clearScreen", "false",
		"--strictPort", "true",
	)
	if vs.cfg.config_file != "" {
		args = append(args, "--config", vs.cfg.config_file)
	}

	vs.logger.Info("Starting vite dev server",
		"command", strings.Join(args, " "),
		"port", vs.port,
	)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), vs.env...)
	cmd.SysProcAttr = procutil.SysProcAttr()

	if vs.cfg.cmd_dir != "" {
		cmd.Dir = vs.cfg.cmd_dir
	}

	if err := cmd.Start(); err != nil {
		vs.mu.Unlock()
		return fmt.Errorf("failed to start vite dev server: %w", err)
	}

	if err := write_pid_file(vs.pid_file, cmd.Process.Pid); err != nil {
		vs.logger.Warn("failed to write vite pid file", "err", err)
	}

	vs.cmd = cmd
	done := make(chan struct{})
	vs.done = done

	go func() {
		vs.exit_err = cmd.Wait()
		close(done)
	}()

	vs.mu.Unlock()

	vs.logger.Info(fmt.Sprintf(
		"vite dev server started → http://localhost:%d",
		vs.port,
	))

	// watch for unexpected vite exit
	go vs.watch_for_crash()

	return nil
}

// watch_for_crash monitors the vite process and logs a warning if
// it exits unexpectedly. The developer sees vite errors in the
// terminal; this ensures Wave also surfaces the problem.
func (vs *vite_supervisor) watch_for_crash() {
	vs.mu.Lock()
	done := vs.done
	vs.mu.Unlock()
	if done == nil {
		return
	}

	<-done

	vs.mu.Lock()
	was_intentional := vs.stopping
	vs.mu.Unlock()

	if !was_intentional {
		vs.logger.Warn(
			"vite dev server exited unexpectedly — " +
				"restart wave to recover vite",
		)
	}
}

func (vs *vite_supervisor) stop() {
	vs.mu.Lock()
	vs.stopping = true

	if vs.cmd == nil || vs.cmd.Process == nil {
		vs.mu.Unlock()
		return
	}

	pid := vs.cmd.Process.Pid
	done := vs.done
	vs.mu.Unlock()

	vs.logger.Info("Stopping vite dev server")

	procutil.RequestStop(pid)

	select {
	case <-done:
		vs.logger.Info("vite dev server stopped")
	case <-time.After(vite_grace_period):
		vs.logger.Info(
			"Vite did not stop within grace period, killing",
		)
		procutil.ForceKill(pid)
		<-done
	}

	remove_pid_file(vs.pid_file)

	vs.mu.Lock()
	vs.cmd = nil
	vs.done = nil
	vs.exit_err = nil
	vs.mu.Unlock()
}

/////////////////////////////////////////////////////////////////////
/////// Vite prod build (synchronous, called by plugins)
/////////////////////////////////////////////////////////////////////

// ViteProdBuildOpts are provided by the plugin (framework) to
// control where vite writes its output.
type ViteProdBuildOpts struct {
	OutDir      string // vite --outDir
	ManifestOut string // final location for the manifest file
}

func run_vite_prod_build(
	cfg *vite_build_config,
	opts ViteProdBuildOpts,
	env []string,
	logger *slog.Logger,
) error {
	args := make([]string, len(cfg.base_cmd))
	copy(args, cfg.base_cmd)

	// Vite's `--manifest` flag always writes relative to `--outDir`, so
	// we write to a temp file in the outDir and then move it to the
	// caller's desired location.
	tmp_vite_manifest_filename := "__WAVE_INTERNAL_TMP_VITE_MANIFEST__.json"

	args = append(args, "vite", "build",
		"--outDir", filepath.Join(".", opts.OutDir),
		"--assetsDir", filepath.Join("."),
		"--manifest", tmp_vite_manifest_filename,
	)
	if cfg.config_file != "" {
		args = append(args, "--config", cfg.config_file)
	}

	logger.Info("Running vite build (prod)",
		"command", strings.Join(args, " "),
	)

	var stderr_buf bytes.Buffer
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr_buf)
	cmd.Env = append(os.Environ(), env...)

	if cfg.cmd_dir != "" {
		cmd.Dir = cfg.cmd_dir
	}

	if err := cmd.Run(); err != nil {
		captured := strings.TrimSpace(stderr_buf.String())
		if captured != "" {
			return fmt.Errorf("vite build (prod) failed: %w: %s", err, captured)
		}
		return fmt.Errorf("vite build (prod) failed: %w", err)
	}

	// move the temp manifest to the caller's desired location
	temp_manifest := filepath.Join(
		".", opts.OutDir, tmp_vite_manifest_filename,
	)
	if err := os.Rename(temp_manifest, opts.ManifestOut); err != nil {
		return fmt.Errorf(
			"failed to move vite manifest from %s to %s: %w",
			temp_manifest, opts.ManifestOut, err,
		)
	}

	logger.Info("vite build (prod) complete",
		"manifest", opts.ManifestOut,
		"outDir", opts.OutDir,
	)
	return nil
}

package vormabuild

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vormadev/vorma/internal/pkg/vormarun"
	"github.com/vormadev/vorma/kit/procutil"
)

const bombadil_test_variant_env_key = "VORMA_BOMBADIL_VARIANT"
const bombadil_test_deployment_env_key = "VORMA_BOMBADIL_DEPLOYMENT"
const bombadil_test_mode_env_key = "VORMA_BOMBADIL_MODE"
const bombadil_test_react_variant = "react"
const bombadil_test_deployment_a = "A"
const bombadil_test_mode_dev = "dev"
const bombadil_test_panic_route_env_key = "VORMA_BOMBADIL_ENABLE_PANIC_ROUTE"
const bombadil_test_panic_path = "/__bombadil/panic"
const bombadil_test_exit_route_env_key = "VORMA_BOMBADIL_ENABLE_EXIT_ROUTE"
const bombadil_test_exit_path = "/__bombadil/exit"

type dev_cleanup_harness struct {
	t             *testing.T
	bombadil_root string
	build_bin     string
}

type dev_test_output struct {
	t    *testing.T
	path string
	file *os.File
}

type dev_test_command struct {
	pid  int
	done chan struct{}
	err  error
}

func (o *dev_test_output) Close() {
	if err := o.file.Close(); err != nil {
		o.t.Fatalf("error closing dev test output: %v", err)
	}
}

func (o *dev_test_output) String() string {
	if err := o.file.Sync(); err != nil {
		o.t.Fatalf("error syncing dev test output: %v", err)
	}
	b, err := os.ReadFile(o.path)
	if err != nil {
		o.t.Fatalf("error reading dev test output: %v", err)
	}
	return string(b)
}

func TestDevPanicCleanupStopsVite(t *testing.T) {
	h := dev_cleanup_harness{t: t}
	h.init()

	tests := []struct {
		name             string
		stage            string
		triggers_refresh bool
	}{
		{
			name:  "initial Vite start",
			stage: __test_panic_stage_after_initial_vite_start,
		},
		{
			name:             "during Go refresh",
			stage:            __test_panic_stage_during_go_refresh,
			triggers_refresh: true,
		},
		{
			name:             "before Vite restart",
			stage:            __test_panic_stage_before_vite_restart,
			triggers_refresh: true,
		},
		{
			name:             "after Vite restart",
			stage:            __test_panic_stage_after_vite_restart,
			triggers_refresh: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := dev_cleanup_harness{
				t:             t,
				bombadil_root: h.bombadil_root,
				build_bin:     h.build_bin,
			}
			h.run_panic_cleanup_case(tt.stage, tt.triggers_refresh)
		})
	}
}

func TestDevAsyncPanicCleanupStopsVite(t *testing.T) {
	h := dev_cleanup_harness{t: t}
	h.init()

	dist_dir := filepath.Join(h.bombadil_root, ".dist.react.dev.a")
	if err := os.RemoveAll(dist_dir); err != nil {
		t.Fatalf("error removing old dev dist dir: %v", err)
	}

	cmd, output := h.start_dev_command(__test_panic_stage_async_after_initial_vite_start)
	defer h.stop_command(cmd)

	manifest := h.wait_for_vite(dist_dir, cmd)
	err := h.wait_for_exit(cmd, 5*time.Second)
	h.assert_test_async_panic(err, output, __test_panic_stage_async_after_initial_vite_start)
	if h.vite_serving(manifest.Dev_ViteServerPort) {
		h.stop_vite_on_port(manifest.Dev_ViteServerPort)
		t.Fatalf(
			"Vite was still serving after async dev panic at %s",
			h.vite_url(manifest.Dev_ViteServerPort),
		)
	}
}

func TestDevAppHandlerPanicLeavesViteRunning(t *testing.T) {
	h := dev_cleanup_harness{t: t}
	h.init()

	dist_dir := filepath.Join(h.bombadil_root, ".dist.react.dev.a")
	if err := os.RemoveAll(dist_dir); err != nil {
		t.Fatalf("error removing old dev dist dir: %v", err)
	}

	cmd, output := h.start_dev_command("", bombadil_test_panic_route_env_key+"=1")
	defer h.stop_command(cmd)

	manifest := h.wait_for_vite(dist_dir, cmd)
	app_url := h.wait_for_app_url(cmd, output)
	client := &http.Client{Timeout: 1 * time.Second}
	_, _ = client.Get(app_url + bombadil_test_panic_path)
	time.Sleep(250 * time.Millisecond)

	if err, ok := cmd.exit_err(); ok {
		t.Fatalf("dev build helper exited after app handler panic: %v", err)
	}
	if !h.vite_serving(manifest.Dev_ViteServerPort) {
		t.Fatalf("expected Vite to keep serving after recovered app handler panic")
	}
}

func TestDevAppProcessExitStopsVite(t *testing.T) {
	h := dev_cleanup_harness{t: t}
	h.init()

	dist_dir := filepath.Join(h.bombadil_root, ".dist.react.dev.a")
	if err := os.RemoveAll(dist_dir); err != nil {
		t.Fatalf("error removing old dev dist dir: %v", err)
	}

	cmd, output := h.start_dev_command("", bombadil_test_exit_route_env_key+"=1")
	defer h.stop_command(cmd)

	manifest := h.wait_for_vite(dist_dir, cmd)
	app_url := h.wait_for_app_url(cmd, output)
	client := &http.Client{Timeout: 1 * time.Second}
	_, _ = client.Get(app_url + bombadil_test_exit_path)

	select {
	case <-cmd.done:
		err := cmd.err
		if err == nil {
			t.Fatalf("expected dev build helper to exit with an error after app process exited")
		}
	case <-time.After(5 * time.Second):
		if h.vite_serving(manifest.Dev_ViteServerPort) {
			t.Fatalf(
				"dev build helper stayed running and Vite was still serving after app process exited",
			)
		}
		t.Fatalf("dev build helper stayed running after app process exited")
	}
	h.assert_vite_stopped(manifest.Dev_ViteServerPort)
}

func (h *dev_cleanup_harness) init() {
	bombadil_root, err := filepath.Abs("../../integration_tests")
	if err != nil {
		h.t.Fatalf("error resolving Bombadil root: %v", err)
	}
	h.bombadil_root = bombadil_root
	h.ensure_bombadil_deps()

	temp_dir := h.t.TempDir()
	h.build_bin = filepath.Join(temp_dir, "bombadil-build")
	build_cmd := exec.Command("go", "build", "-o", h.build_bin, "./cmd/build")
	build_cmd.Dir = h.bombadil_root
	if out, err := build_cmd.CombinedOutput(); err != nil {
		h.t.Fatalf("error building Bombadil build helper: %v\n%s", err, out)
	}
}

func (h dev_cleanup_harness) ensure_bombadil_deps() {
	if _, err := os.Stat(filepath.Join(h.bombadil_root, "node_modules", ".bin", "vite")); err == nil {
		return
	}
	install_cmd := exec.Command("pnpm", "i")
	install_cmd.Dir = h.bombadil_root
	if out, err := install_cmd.CombinedOutput(); err != nil {
		h.t.Fatalf("error installing Bombadil fixture dependencies: %v\n%s", err, out)
	}
}

func (h dev_cleanup_harness) run_panic_cleanup_case(
	stage string,
	triggers_refresh bool,
) {
	dist_dir := filepath.Join(h.bombadil_root, ".dist.react.dev.a")
	if err := os.RemoveAll(dist_dir); err != nil {
		h.t.Fatalf("error removing old dev dist dir: %v", err)
	}

	cmd, output := h.start_dev_command(stage)
	defer h.stop_command(cmd)

	var manifest vormarun.Manifest
	if triggers_refresh {
		manifest = h.wait_for_vite(dist_dir, cmd)
		h.trigger_go_refresh(stage)
		err := h.wait_for_exit(cmd, 20*time.Second)
		h.assert_test_panic(err, output, stage)
	} else {
		err := h.wait_for_exit(cmd, 20*time.Second)
		h.assert_test_panic(err, output, stage)
		manifest = h.wait_for_manifest(dist_dir)
	}

	if manifest.Dev_ViteServerPort == 0 {
		h.t.Fatalf("expected dev manifest to record Vite port")
	}
	h.assert_vite_stopped(manifest.Dev_ViteServerPort)
}

func (h dev_cleanup_harness) start_dev_command(
	stage string,
	extra_env ...string,
) (*dev_test_command, *dev_test_output) {
	output_file, err := os.CreateTemp(h.t.TempDir(), "dev-output-*.log")
	if err != nil {
		h.t.Fatalf("error creating dev test output file: %v", err)
	}
	output := &dev_test_output{
		t:    h.t,
		path: output_file.Name(),
		file: output_file,
	}
	h.t.Cleanup(output.Close)

	cmd := exec.Command(h.build_bin, "--dev")
	cmd.Dir = h.bombadil_root
	cmd.SysProcAttr = procutil.SysProcAttr()
	cmd.Env = append(
		os.Environ(),
		bombadil_test_variant_env_key+"="+bombadil_test_react_variant,
		bombadil_test_deployment_env_key+"="+bombadil_test_deployment_a,
		bombadil_test_mode_env_key+"="+bombadil_test_mode_dev,
	)
	if stage != "" {
		cmd.Env = append(cmd.Env, __test_panic_stage_env_key+"="+stage)
	}
	cmd.Env = append(cmd.Env, extra_env...)
	cmd.Stdout = output_file
	cmd.Stderr = output_file
	if err := cmd.Start(); err != nil {
		h.t.Fatalf("error starting dev build helper: %v", err)
	}

	test_cmd := &dev_test_command{
		pid:  cmd.Process.Pid,
		done: make(chan struct{}),
	}
	go func() {
		test_cmd.err = cmd.Wait()
		close(test_cmd.done)
	}()
	return test_cmd, output
}

func (h dev_cleanup_harness) stop_command(cmd *dev_test_command) {
	select {
	case <-cmd.done:
		return
	default:
	}
	if cmd.pid == 0 {
		return
	}
	_ = syscall.Kill(-cmd.pid, syscall.SIGINT)
	select {
	case <-cmd.done:
		return
	case <-time.After(15 * time.Second):
	}
	_ = syscall.Kill(-cmd.pid, syscall.SIGKILL)
	<-cmd.done
}

func (h dev_cleanup_harness) wait_for_vite(
	dist_dir string,
	cmd *dev_test_command,
) vormarun.Manifest {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-cmd.done:
			h.t.Fatalf("dev build helper exited before Vite was ready: %v", cmd.err)
		default:
		}

		manifest, ok := h.read_dev_manifest(dist_dir)
		if ok && manifest.Dev_ViteServerPort != 0 && h.vite_serving(manifest.Dev_ViteServerPort) {
			return manifest
		}
		time.Sleep(100 * time.Millisecond)
	}
	h.t.Fatalf("timed out waiting for Vite to serve")
	return vormarun.Manifest{}
}

func (h dev_cleanup_harness) wait_for_app_url(
	cmd *dev_test_command,
	output *dev_test_output,
) string {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-cmd.done:
			h.t.Fatalf("dev build helper exited before app server was ready: %v", cmd.err)
		default:
		}

		manifest := h.wait_for_manifest(filepath.Join(h.bombadil_root, ".dist.react.dev.a"))
		if manifest.Dev_MuxPort == 0 {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		port, ok := h.find_app_port(output)
		if ok {
			return "http://127.0.0.1:" + strconv.Itoa(port)
		}
		time.Sleep(100 * time.Millisecond)
	}
	h.t.Fatalf("timed out waiting for app server URL")
	return ""
}

func (h dev_cleanup_harness) find_app_port(output *dev_test_output) (int, bool) {
	output_text := output.String()
	idx := strings.LastIndex(output_text, "App server ready: http://localhost:")
	if idx == -1 {
		return 0, false
	}
	port_text := output_text[idx+len("App server ready: http://localhost:"):]
	end := strings.IndexFunc(port_text, func(r rune) bool {
		return r < '0' || r > '9'
	})
	if end != -1 {
		port_text = port_text[:end]
	}
	port, err := strconv.Atoi(port_text)
	return port, err == nil
}

func (h dev_cleanup_harness) wait_for_manifest(dist_dir string) vormarun.Manifest {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		manifest, ok := h.read_dev_manifest(dist_dir)
		if ok {
			return manifest
		}
		time.Sleep(50 * time.Millisecond)
	}
	h.t.Fatalf("timed out waiting for dev manifest")
	return vormarun.Manifest{}
}

func (h dev_cleanup_harness) read_dev_manifest(dist_dir string) (vormarun.Manifest, bool) {
	manifest_path := filepath.Join(
		dist_dir,
		".vorma",
		"static",
		vormarun.ManifestStaticOutDev,
	)
	manifest_bytes, err := os.ReadFile(manifest_path)
	if err != nil {
		return vormarun.Manifest{}, false
	}
	var manifest vormarun.Manifest
	if err := json.Unmarshal(manifest_bytes, &manifest); err != nil {
		h.t.Fatalf("error parsing dev manifest: %v", err)
	}
	return manifest, true
}

func (h dev_cleanup_harness) trigger_go_refresh(stage string) {
	probe_path := filepath.Join(
		h.bombadil_root,
		"scenario",
		"__vorma_cleanup_probe_test.go",
	)
	h.t.Cleanup(func() {
		if err := os.Remove(probe_path); err != nil && !os.IsNotExist(err) {
			h.t.Fatalf("error removing cleanup probe: %v", err)
		}
	})
	src := fmt.Sprintf(
		"package scenario\n\nvar cleanup_probe_%d = %q\n",
		time.Now().UnixNano(),
		stage,
	)
	if err := os.WriteFile(probe_path, []byte(src), 0644); err != nil {
		h.t.Fatalf("error writing cleanup probe: %v", err)
	}
}

func (h dev_cleanup_harness) assert_test_panic(
	err error,
	output *dev_test_output,
	stage string,
) {
	if err == nil {
		h.t.Fatalf("expected dev build helper to panic")
	}
	expected := "test panic: " + stage
	if !strings.Contains(output.String(), expected) {
		h.t.Fatalf("expected %q in output, got:\n%s", expected, output.String())
	}
}

func (h dev_cleanup_harness) assert_test_async_panic(
	err error,
	output *dev_test_output,
	stage string,
) {
	if err == nil {
		h.t.Fatalf("expected dev build helper to panic")
	}
	expected := "test async panic: " + stage
	if !strings.Contains(output.String(), expected) {
		h.t.Fatalf("expected %q in output, got:\n%s", expected, output.String())
	}
}

func (cmd *dev_test_command) exit_err() (error, bool) {
	select {
	case <-cmd.done:
		return cmd.err, true
	default:
		return nil, false
	}
}

func (h dev_cleanup_harness) wait_for_exit(
	cmd *dev_test_command,
	timeout time.Duration,
) error {
	select {
	case <-cmd.done:
		return cmd.err
	case <-time.After(timeout):
		h.t.Fatalf("timed out waiting for dev build helper to exit")
		return nil
	}
}

func (h dev_cleanup_harness) assert_vite_stopped(port int) {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !h.vite_serving(port) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	h.t.Fatalf("Vite was still serving after dev panic cleanup at %s", h.vite_url(port))
}

func (h dev_cleanup_harness) stop_vite_on_port(port int) {
	cmd := exec.Command("lsof", "-ti", "tcp:"+strconv.Itoa(port))
	out, err := cmd.Output()
	if err != nil {
		return
	}
	for pid_text := range strings.FieldsSeq(string(out)) {
		pid, err := strconv.Atoi(pid_text)
		if err == nil {
			procutil.ForceKill(pid)
		}
	}
}

func (h dev_cleanup_harness) vite_serving(port int) bool {
	client := &http.Client{Timeout: 200 * time.Millisecond}
	resp, err := client.Get(h.vite_url(port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (h dev_cleanup_harness) vite_url(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/@vite/client", port)
}

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const dev_server_ready_prefix = "App server ready: "
const dev_server_ready_poll_interval = 100 * time.Millisecond
const dev_server_ready_timeout = 90 * time.Second
const dev_server_shutdown_timeout = 10 * time.Second
const localhost_url_prefix = "http://localhost:"
const loopback_url_prefix = "http://127.0.0.1:"
const chrome_local_permissions = "local-network-access"
const bombadil_artifacts_dir = ".bombadil"
const bombadil_logs_dir = ".bombadil/logs"
const dev_manifest_filename = "vorma.manifest.dev.json"
const bombadil_variant_env_key = "VORMA_BOMBADIL_VARIANT"
const bombadil_deployment_env_key = "VORMA_BOMBADIL_DEPLOYMENT"
const bombadil_mode_env_key = "VORMA_BOMBADIL_MODE"
const vorma_dev_app_server_preferred_port_env_key = "__VORMA_DEV_APP_SERVER_PREFERRED_PORT"
const vorma_dev_vite_server_preferred_port_env_key = "__VORMA_DEV_VITE_SERVER_PREFERRED_PORT"
const bombadil_mode_dev = "dev"
const bombadil_mode_prod = "prod"

type variant_config struct {
	name          string
	port          int
	dev_app_port  int
	dev_vite_port int
}

type run_config struct {
	intensity int
	variants  []variant_config
}

type variant_runner struct {
	config  run_config
	variant variant_config
}

type dev_manifest struct {
	Dev_ViteServerPort int                          `json:"Dev_ViteServerPort"`
	ClientEntry        dev_client_module            `json:"ClientEntry"`
	ClientRoutes       map[string]dev_client_module `json:"ClientRoutes"`
}

type dev_client_module struct {
	URL string `json:"URL"`
}

func main() {
	config := run_config{
		variants: []variant_config{
			{name: "react", port: 18080, dev_app_port: 8080, dev_vite_port: 5173},
			{name: "preact", port: 18081, dev_app_port: 8081, dev_vite_port: 5174},
			{name: "solid", port: 18082, dev_app_port: 8082, dev_vite_port: 5175},
		},
	}

	if err := config.run_command(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (config run_config) run_command(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: bombadil <build|test-prod|test-dev|serve-dev|inspect>")
	}

	switch args[0] {
	case "build":
		return config.build()
	case "test-prod":
		flags := flag.NewFlagSet("test-prod", flag.ContinueOnError)
		intensity := flags.Int("intensity", 1, "time-limit intensity")
		variant_name := flags.String("variant", "", "optional variant to test")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		config.intensity = *intensity
		return config.test_prod(*variant_name)
	case "serve-dev":
		if len(args) != 2 {
			return errors.New("usage: bombadil serve-dev <react|preact|solid>")
		}
		return config.serve_dev(args[1])
	case "test-dev":
		return config.test_dev(args[1:])
	case "inspect":
		inspect_path := bombadil_artifacts_dir
		if len(args) > 1 {
			inspect_path = args[1]
		}
		cmd := exec.Command("pnpm", "exec", "bombadil", "inspect", inspect_path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (config run_config) build() error {
	return config.for_each_variant(func(runner variant_runner) error {
		return runner.build()
	})
}

func (config run_config) test_prod(variant_name string) error {
	return config.for_each_selected_variant(variant_name, func(runner variant_runner) error {
		return runner.test_prod()
	})
}

func (config run_config) test_dev(args []string) error {
	flags := flag.NewFlagSet("test-dev", flag.ContinueOnError)
	intensity := flags.Int("intensity", 1, "time-limit intensity")
	variant_name := flags.String("variant", "", "optional variant to test")
	if err := flags.Parse(args); err != nil {
		return err
	}
	config.intensity = *intensity

	return config.for_each_selected_variant(*variant_name, func(runner variant_runner) error {
		return runner.test_dev()
	})
}

func (config run_config) for_each_selected_variant(
	variant_name string,
	callback func(variant_runner) error,
) error {
	if variant_name != "" {
		variant, ok := config.find_variant(variant_name)
		if !ok {
			return fmt.Errorf("unknown variant %q", variant_name)
		}
		return callback(variant_runner{
			config:  config,
			variant: variant,
		})
	}
	return config.for_each_variant(callback)
}

func (config run_config) serve_dev(name string) error {
	variant, ok := config.find_variant(name)
	if !ok {
		return fmt.Errorf("unknown variant %q", name)
	}

	runner := variant_runner{
		config:  config,
		variant: variant,
	}
	build_binary, cleanup, err := runner.build_dev_binary()
	if err != nil {
		return err
	}
	defer cleanup()

	cmd := exec.Command(build_binary, "--dev")
	runner.set_process_group(cmd)
	cmd.Env = append(
		os.Environ(),
		bombadil_variant_env_key+"="+variant.name,
		bombadil_deployment_env_key+"=A",
		bombadil_mode_env_key+"="+bombadil_mode_dev,
		vorma_dev_app_server_preferred_port_env_key+"="+strconv.Itoa(variant.dev_app_port),
		vorma_dev_vite_server_preferred_port_env_key+"="+strconv.Itoa(variant.dev_vite_port),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	server_done := make(chan error, 1)
	go func() {
		server_done <- cmd.Wait()
	}()
	stop_signal_cleanup := runner.stop_dev_server_on_signal(cmd)
	defer stop_signal_cleanup()
	return <-server_done
}

func (config run_config) find_variant(name string) (variant_config, bool) {
	for _, variant := range config.variants {
		if variant.name == name {
			return variant, true
		}
	}
	return variant_config{}, false
}

func (config run_config) for_each_variant(callback func(variant_runner) error) error {
	var wait_group sync.WaitGroup
	errs := make(chan error, len(config.variants))

	for _, variant := range config.variants {
		runner := variant_runner{
			config:  config,
			variant: variant,
		}
		wait_group.Go(func() {
			if err := callback(runner); err != nil {
				errs <- err
			}
		})
	}

	wait_group.Wait()
	close(errs)

	var err error
	for variant_err := range errs {
		err = errors.Join(err, variant_err)
	}
	return err
}

func (runner variant_runner) test_prod() error {
	if err := runner.build(); err != nil {
		return err
	}
	return runner.test()
}

func (runner variant_runner) test_dev() error {
	runner.log("starting dev fixture server")
	log_path, err := runner.log_path("dev")
	if err != nil {
		return err
	}
	log_file, err := os.Create(log_path)
	if err != nil {
		return err
	}
	defer log_file.Close()

	build_binary, cleanup, err := runner.build_dev_binary()
	if err != nil {
		return err
	}
	defer cleanup()

	server := exec.Command(build_binary, "--dev")
	runner.set_process_group(server)
	server.Env = append(
		os.Environ(),
		bombadil_variant_env_key+"="+runner.variant.name,
		bombadil_deployment_env_key+"=A",
		bombadil_mode_env_key+"="+bombadil_mode_dev,
		vorma_dev_app_server_preferred_port_env_key+"="+strconv.Itoa(runner.variant.dev_app_port),
		vorma_dev_vite_server_preferred_port_env_key+"="+strconv.Itoa(runner.variant.dev_vite_port),
	)
	server.Stdout = log_file
	server.Stderr = log_file

	if err := server.Start(); err != nil {
		return err
	}

	server_done := make(chan error, 1)
	go func() {
		server_done <- server.Wait()
	}()
	stop_signal_cleanup := runner.stop_dev_server_on_signal(server)
	defer stop_signal_cleanup()
	defer runner.stop_dev_server(server, server_done)

	base_url, err := runner.wait_for_dev_server_ready(log_path, server_done)
	if err != nil {
		log_file.Seek(0, 0)
		io.Copy(os.Stderr, log_file)
		return err
	}
	base_url = strings.Replace(base_url, localhost_url_prefix, loopback_url_prefix, 1)

	runner.log("dev fixture server ready at " + base_url)
	return runner.run_bombadil_suite(base_url, "dev-"+runner.variant.name, "inline")
}

func (runner variant_runner) build_dev_binary() (string, func(), error) {
	temp_dir, err := os.MkdirTemp("", "vorma-bombadil-dev-build-*")
	if err != nil {
		return "", nil, err
	}

	cleanup := func() {
		os.RemoveAll(temp_dir)
	}
	build_binary := filepath.Join(temp_dir, "build")
	build_cmd := exec.Command("go", "build", "-o", build_binary, "./cmd/build")
	log_path := filepath.Join(bombadil_logs_dir, "build-dev-"+runner.variant.name+".log")
	if err := runner.run_command_to_log(build_cmd, log_path); err != nil {
		cleanup()
		return "", nil, err
	}
	return build_binary, cleanup, nil
}

func (runner variant_runner) build() error {
	runner.log("building deployment A")
	if err := runner.build_client_deployment("A"); err != nil {
		return err
	}

	runner.log("building deployment B")
	if err := runner.build_client_deployment("B"); err != nil {
		return err
	}

	if err := os.MkdirAll("./.dist."+runner.variant.name+"/.vorma", 0755); err != nil {
		return err
	}

	runner.log("building server binary")
	cmd := exec.Command(
		"go",
		"build",
		"-o",
		"./.dist."+runner.variant.name+"/.vorma/main",
		"-tags=prod",
		"./cmd/serve",
	)
	cmd.Env = append(
		os.Environ(),
		bombadil_variant_env_key+"="+runner.variant.name,
		bombadil_mode_env_key+"="+bombadil_mode_prod,
	)
	log_path := filepath.Join(bombadil_logs_dir, "build-prod-"+runner.variant.name+"-server.log")
	return runner.run_command_to_log(cmd, log_path)
}

func (runner variant_runner) build_client_deployment(deployment string) error {
	cmd := exec.Command("go", "run", "./cmd/build")
	cmd.Env = append(
		os.Environ(),
		bombadil_variant_env_key+"="+runner.variant.name,
		bombadil_deployment_env_key+"="+deployment,
		bombadil_mode_env_key+"="+bombadil_mode_prod,
	)
	log_path := filepath.Join(
		bombadil_logs_dir,
		"build-prod-"+runner.variant.name+"-"+strings.ToLower(deployment)+".log",
	)
	return runner.run_command_to_log(cmd, log_path)
}

func (runner variant_runner) test() error {
	runner.log("starting fixture server")
	log_path, err := runner.log_path("prod")
	if err != nil {
		return err
	}
	log_file, err := os.Create(log_path)
	if err != nil {
		return err
	}
	defer log_file.Close()

	server := exec.Command("./.dist." + runner.variant.name + "/.vorma/main")
	runner.set_process_group(server)
	server.Env = append(
		os.Environ(),
		"PORT="+strconv.Itoa(runner.variant.port),
		bombadil_variant_env_key+"="+runner.variant.name,
		bombadil_mode_env_key+"="+bombadil_mode_prod,
	)
	server.Stdout = log_file
	server.Stderr = log_file

	if err := server.Start(); err != nil {
		return err
	}

	server_done := make(chan error, 1)
	go func() {
		server_done <- server.Wait()
	}()
	stop_signal_cleanup := runner.stop_server_on_signal(server)
	defer stop_signal_cleanup()
	defer runner.stop_server(server, server_done)

	if err := runner.wait_until_ready(); err != nil {
		log_file.Seek(0, 0)
		io.Copy(os.Stderr, log_file)
		return err
	}

	runner.log("fixture server ready")
	base_url := "http://127.0.0.1:" + strconv.Itoa(runner.variant.port)
	return runner.run_bombadil_suite(base_url, runner.variant.name, "files,inline")
}

func (runner variant_runner) log_path(mode string) (string, error) {
	if err := os.MkdirAll(bombadil_logs_dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(bombadil_logs_dir, mode+"-"+runner.variant.name+".log"), nil
}

func (runner variant_runner) wait_until_ready() error {
	url := "http://127.0.0.1:" + strconv.Itoa(runner.variant.port) + "/"

	for range 50 {
		res, err := http.Get(url)
		if err == nil {
			res.Body.Close()
			if res.StatusCode >= 200 && res.StatusCode < 500 {
				return nil
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("%s server did not become ready", runner.variant.name)
}

func (runner variant_runner) run_bombadil_suite(
	base_url string,
	output_prefix string,
	instrument_javascript string,
) error {
	if err := runner.run_bombadil_test(base_url, "", 30, output_prefix, instrument_javascript); err != nil {
		return err
	}
	if err := runner.run_bombadil_test(base_url, "/nested/alpha/details", 15, output_prefix+"-nested", instrument_javascript); err != nil {
		return err
	}
	return runner.run_bombadil_test(
		base_url,
		"/counter?n=0",
		10,
		output_prefix+"-counter",
		instrument_javascript,
	)
}

func (runner variant_runner) run_bombadil_test(
	base_url string,
	path string,
	seconds int,
	output_path string,
	instrument_javascript string,
) error {
	time_limit := strconv.Itoa(seconds*runner.config.intensity) + "s"
	artifact_path := filepath.Join(bombadil_artifacts_dir, output_path)
	runner.log("testing " + base_url + path + " for " + time_limit)
	runner.log("writing Bombadil artifacts to " + artifact_path)
	if err := os.RemoveAll(artifact_path); err != nil {
		return fmt.Errorf("error removing stale Bombadil artifacts at %s: %w", artifact_path, err)
	}
	if err := os.MkdirAll(bombadil_logs_dir, 0755); err != nil {
		return err
	}
	test_log_path := filepath.Join(bombadil_logs_dir, "test-"+output_path+".log")

	cmd := exec.Command(
		"pnpm",
		"exec",
		"bombadil",
		"test",
		base_url+path,
		"./specs/vorma.spec.ts",
		"--headless",
		"--exit-on-violation",
		"--instrument-javascript",
		instrument_javascript,
		"--chrome-grant-permissions",
		chrome_local_permissions,
		"--time-limit",
		time_limit,
		"--output-path",
		artifact_path,
	)
	if err := runner.run_command_to_log(cmd, test_log_path); err != nil {
		return fmt.Errorf(
			"inspect artifacts with `go run ../../internal/cmd/maint inspect-framework-artifacts --artifact %s`; read log at %s: %w",
			artifact_path,
			test_log_path,
			err,
		)
	}
	return nil
}

func (runner variant_runner) run_command_to_log(cmd *exec.Cmd, log_path string) error {
	if err := os.MkdirAll(filepath.Dir(log_path), 0755); err != nil {
		return err
	}
	log_file, err := os.Create(log_path)
	if err != nil {
		return err
	}
	defer log_file.Close()

	runner.log("writing command log to " + log_path)
	cmd.Stdout = log_file
	cmd.Stderr = log_file
	if err := runner.run_command(cmd); err != nil {
		return fmt.Errorf("read log at %s: %w", log_path, err)
	}
	return nil
}

func (runner variant_runner) run_command(cmd *exec.Cmd) error {
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", runner.variant.name, err)
	}
	return nil
}

func (runner variant_runner) wait_for_dev_server_ready(
	log_path string,
	server_done <-chan error,
) (string, error) {
	deadline := time.Now().Add(dev_server_ready_timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-server_done:
			return "", fmt.Errorf("%s dev server exited before ready: %w", runner.variant.name, err)
		default:
		}

		ready_url, ok, err := runner.read_dev_server_ready_url(log_path)
		if err != nil {
			return "", err
		}
		if ok && runner.dev_client_modules_serving() {
			return ready_url, nil
		}
		time.Sleep(dev_server_ready_poll_interval)
	}
	return "", fmt.Errorf("%s dev server did not become ready", runner.variant.name)
}

func (runner variant_runner) read_dev_server_ready_url(log_path string) (string, bool, error) {
	data, err := os.ReadFile(log_path)
	if err != nil {
		return "", false, err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		_, after, ok := strings.Cut(line, dev_server_ready_prefix)
		if !ok {
			continue
		}
		return strings.TrimSpace(after), true, nil
	}
	return "", false, nil
}

func (runner variant_runner) dev_client_modules_serving() bool {
	manifest_path := filepath.Join(
		".dist."+runner.variant.name+".dev.a",
		".vorma",
		"static",
		dev_manifest_filename,
	)
	data, err := os.ReadFile(manifest_path)
	if err != nil {
		return false
	}

	var manifest dev_manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return false
	}
	if manifest.Dev_ViteServerPort == 0 {
		return false
	}

	urls := []string{
		"http://127.0.0.1:" + strconv.Itoa(manifest.Dev_ViteServerPort) + "/@vite/client",
		manifest.ClientEntry.URL,
	}
	if root_route, ok := manifest.ClientRoutes["/"]; ok {
		urls = append(urls, root_route.URL)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	for _, url := range urls {
		if url == "" {
			return false
		}
		res, err := client.Get(url)
		if err != nil {
			return false
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return false
		}
	}
	return true
}

func (runner variant_runner) log(message string) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", runner.variant.name, message)
}

func (runner variant_runner) stop_server(server *exec.Cmd, server_done <-chan error) {
	select {
	case <-server_done:
		return
	default:
	}
	if server.ProcessState != nil {
		return
	}

	runner.signal_process_group(server, syscall.SIGKILL)
	<-server_done
}

func (runner variant_runner) stop_server_on_signal(server *exec.Cmd) func() {
	sig_ch := make(chan os.Signal, 1)
	stop_ch := make(chan struct{})
	done := make(chan struct{})
	signal.Notify(sig_ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer close(done)
		select {
		case <-stop_ch:
		case <-sig_ch:
			runner.signal_process_group(server, syscall.SIGKILL)
		}
	}()
	return func() {
		signal.Stop(sig_ch)
		close(stop_ch)
		close(sig_ch)
		<-done
	}
}

func (runner variant_runner) stop_dev_server(server *exec.Cmd, server_done <-chan error) {
	select {
	case <-server_done:
		return
	default:
	}
	if server.ProcessState != nil {
		return
	}

	runner.signal_process_group(server, syscall.SIGINT)
	select {
	case <-server_done:
	case <-time.After(dev_server_shutdown_timeout):
		runner.signal_process_group(server, syscall.SIGKILL)
		<-server_done
	}
}

func (runner variant_runner) stop_dev_server_on_signal(server *exec.Cmd) func() {
	sig_ch := make(chan os.Signal, 1)
	stop_ch := make(chan struct{})
	done := make(chan struct{})
	signal.Notify(sig_ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer close(done)
		select {
		case <-stop_ch:
		case <-sig_ch:
			runner.signal_process_group(server, syscall.SIGINT)
			select {
			case <-stop_ch:
			case <-time.After(dev_server_shutdown_timeout):
				runner.signal_process_group(server, syscall.SIGKILL)
			}
		}
	}()
	return func() {
		signal.Stop(sig_ch)
		close(stop_ch)
		close(sig_ch)
		<-done
	}
}

func (runner variant_runner) set_process_group(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func (runner variant_runner) signal_process_group(server *exec.Cmd, signal syscall.Signal) {
	if err := syscall.Kill(-server.Process.Pid, signal); err == nil {
		return
	}
	server.Process.Signal(signal)
}

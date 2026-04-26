package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type variant_config struct {
	name string
	port int
}

type run_config struct {
	multiplier int
	variants   []variant_config
}

type variant_runner struct {
	config  run_config
	variant variant_config
}

func main() {
	config := run_config{
		variants: []variant_config{
			{name: "react", port: 18080},
			{name: "preact", port: 18081},
			{name: "solid", port: 18082},
		},
	}

	if err := config.run_command(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (config run_config) run_command(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: bombadil <build|run|dev|inspect>")
	}

	switch args[0] {
	case "build":
		return config.build()
	case "run":
		flags := flag.NewFlagSet("run", flag.ContinueOnError)
		multiplier := flags.Int("multiplier", 1, "time-limit multiplier")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		config.multiplier = *multiplier
		return config.run()
	case "dev":
		if len(args) != 2 {
			return errors.New("usage: bombadil dev <react|preact|solid>")
		}
		return config.dev(args[1])
	case "inspect":
		cmd := exec.Command("pnpm", "exec", "bombadil", "inspect", "./.bombadil")
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

func (config run_config) run() error {
	return config.for_each_variant(func(runner variant_runner) error {
		return runner.run()
	})
}

func (config run_config) dev(name string) error {
	variant, ok := config.find_variant(name)
	if !ok {
		return fmt.Errorf("unknown variant %q", name)
	}

	cmd := exec.Command("go", "run", "./cmd/build", "--dev")
	cmd.Env = append(
		os.Environ(),
		"VORMA_BOMBADIL_VARIANT="+variant.name,
		"VORMA_BOMBADIL_DEPLOYMENT=A",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
		wait_group.Add(1)

		go func() {
			defer wait_group.Done()

			runner := variant_runner{
				config:  config,
				variant: variant,
			}
			errs <- callback(runner)
		}()
	}

	wait_group.Wait()
	close(errs)

	var joined error
	for err := range errs {
		joined = errors.Join(joined, err)
	}
	return joined
}

func (runner variant_runner) run() error {
	if err := runner.build(); err != nil {
		return err
	}
	return runner.test()
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
		"VORMA_BOMBADIL_VARIANT="+runner.variant.name,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return runner.run_command(cmd)
}

func (runner variant_runner) build_client_deployment(deployment string) error {
	cmd := exec.Command("go", "run", "./cmd/build")
	cmd.Env = append(
		os.Environ(),
		"VORMA_BOMBADIL_VARIANT="+runner.variant.name,
		"VORMA_BOMBADIL_DEPLOYMENT="+deployment,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return runner.run_command(cmd)
}

func (runner variant_runner) test() error {
	runner.log("starting fixture server")
	log_path := filepath.Join(os.TempDir(), "vorma-bombadil-"+runner.variant.name+".log")
	log_file, err := os.Create(log_path)
	if err != nil {
		return err
	}
	defer log_file.Close()

	server := exec.Command("./.dist." + runner.variant.name + "/.vorma/main")
	server.Env = append(
		os.Environ(),
		"PORT="+strconv.Itoa(runner.variant.port),
		"VORMA_BOMBADIL_VARIANT="+runner.variant.name,
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
	defer runner.stop_server(server, server_done)

	if err := runner.wait_until_ready(); err != nil {
		log_file.Seek(0, 0)
		io.Copy(os.Stderr, log_file)
		return err
	}

	runner.log("fixture server ready")
	if err := runner.run_bombadil_test("", 30, runner.variant.name); err != nil {
		return err
	}
	if err := runner.run_bombadil_test("/nested/alpha/details", 15, runner.variant.name+"-nested"); err != nil {
		return err
	}
	return runner.run_bombadil_test("/counter?n=0", 10, runner.variant.name+"-counter")
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

func (runner variant_runner) run_bombadil_test(path string, seconds int, output_path string) error {
	base_url := "http://127.0.0.1:" + strconv.Itoa(runner.variant.port)
	time_limit := strconv.Itoa(seconds*runner.config.multiplier) + "s"
	runner.log("testing " + base_url + path + " for " + time_limit)

	cmd := exec.Command(
		"pnpm",
		"exec",
		"bombadil",
		"test",
		base_url+path,
		"./specs/vorma.spec.ts",
		"--headless",
		"--exit-on-violation",
		"--chrome-grant-permissions",
		"",
		"--time-limit",
		time_limit,
		"--output-path",
		"./.bombadil/"+output_path,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return runner.run_command(cmd)
}

func (runner variant_runner) run_command(cmd *exec.Cmd) error {
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", runner.variant.name, err)
	}
	return nil
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

	server.Process.Kill()
	<-server_done
}

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

type stress_runner struct {
	multiplier   int
	hegel_log    string
	bombadil_log string
}

type stress_result struct {
	name string
	err  error
}

func main() {
	multiplier := flag.Int("multiplier", 1, "stress multiplier")
	flag.Parse()

	runner := stress_runner{
		multiplier:   *multiplier,
		hegel_log:    "/tmp/vorma-hegel-stress.log",
		bombadil_log: "/tmp/vorma-bombadil-stress.log",
	}

	if err := runner.run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (runner stress_runner) run() error {
	fmt.Println("Running Hegel stress...")
	fmt.Println("Hegel log:", runner.hegel_log)
	fmt.Println("Running Bombadil stress...")
	fmt.Println("Bombadil log:", runner.bombadil_log)

	var wait_group sync.WaitGroup
	results := make(chan stress_result, 2)

	wait_group.Add(2)
	go func() {
		defer wait_group.Done()
		results <- stress_result{
			name: "Hegel",
			err:  runner.run_hegel(),
		}
	}()
	go func() {
		defer wait_group.Done()
		results <- stress_result{
			name: "Bombadil",
			err:  runner.run_bombadil(),
		}
	}()

	var joined error
	for range 2 {
		result := <-results
		if result.err != nil {
			fmt.Println(result.name + ": FAILED")
			joined = errors.Join(joined, result.err)
			continue
		}
		fmt.Println(result.name + ": OK")
	}
	wait_group.Wait()
	if joined != nil {
		fmt.Println("Full run: FAILED")
		return joined
	}

	fmt.Println("Full run: OK")
	return nil
}

func (runner stress_runner) run_hegel() error {
	cmd := exec.Command(
		"go",
		"test",
		"-race",
		"-v",
		"./kit/matcher",
		"./kit/schema",
		"./kit/searchparams",
		"./kit/tsgen",
		"-count="+strconv.Itoa(runner.multiplier),
	)
	if err := runner.run_logged(cmd, runner.hegel_log); err != nil {
		return fmt.Errorf("Hegel stress failed. Log: %s", runner.hegel_log)
	}
	return nil
}

func (runner stress_runner) run_bombadil() error {
	log_file, err := os.OpenFile(runner.bombadil_log, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer log_file.Close()

	install := exec.Command("pnpm", "i")
	install.Dir = "./internal/apps/bombadil"
	install.Stdout = log_file
	install.Stderr = log_file
	if err := install.Run(); err != nil {
		return fmt.Errorf("Bombadil dependency install failed. Log: %s", runner.bombadil_log)
	}

	cmd := exec.Command(
		"go",
		"run",
		"./cmd/bombadil",
		"run",
		"-multiplier",
		strconv.Itoa(runner.multiplier),
	)
	cmd.Dir = "./internal/apps/bombadil"
	cmd.Stdout = log_file
	cmd.Stderr = log_file
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Bombadil stress failed. Log: %s", runner.bombadil_log)
	}
	return nil
}

func (runner stress_runner) run_logged(cmd *exec.Cmd, log_path string) error {
	log_file, err := os.OpenFile(log_path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer log_file.Close()

	cmd.Stdout = log_file
	cmd.Stderr = log_file
	return cmd.Run()
}

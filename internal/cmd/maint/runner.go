package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const framework_tests_dir = "internal/framework_tests"
const docs_dir = "internal/apps/docs"
const npm_dir = "internal/pkg/npm"
const create_npm_dir = "internal/pkg/npm/vorma/create"
const local_output_dir = "__.local"

type command_step struct {
	name     string
	dir      string
	command  string
	args     []string
	env      []string
	log_path string
}

func (app maint_app) find_repo_root() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := wd
	for {
		go_mod := filepath.Join(dir, "go.mod")
		data, read_err := os.ReadFile(go_mod)
		if read_err == nil && strings.Contains(string(data), "module github.com/vormadev/vorma") {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find Vorma repo root")
		}
		dir = parent
	}
}

func (app maint_app) run_steps(steps []command_step) error {
	for _, step := range steps {
		if err := app.run_step(step); err != nil {
			return err
		}
	}
	return nil
}

func (app maint_app) run_step(step command_step) error {
	step_dir := app.step_dir(step)

	fmt.Println("==>", step.name)
	if app.verbose || app.dry_run {
		fmt.Println("   ", app.format_step(step, step_dir))
	}
	if app.dry_run {
		return nil
	}

	if err := app.execute_step(step, step_dir); err != nil {
		return app.command_error(step.name, err)
	}
	return nil
}

func (app maint_app) execute_step(step command_step, step_dir string) error {
	cmd := exec.Command(step.command, step.args...)
	cmd.Dir = step_dir
	cmd.Env = append(os.Environ(), step.env...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (app maint_app) execute_logged_step(
	step command_step,
	step_dir string,
	log_path string,
) error {
	if err := os.MkdirAll(filepath.Dir(log_path), 0755); err != nil {
		return err
	}
	log_file, err := os.OpenFile(log_path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer log_file.Close()

	cmd := exec.Command(step.command, step.args...)
	cmd.Dir = step_dir
	cmd.Env = append(os.Environ(), step.env...)
	cmd.Stdout = log_file
	cmd.Stderr = log_file
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed; read log at %s: %w", step.name, log_path, err)
	}
	return nil
}

func (app maint_app) format_step(step command_step, step_dir string) string {
	parts := []string{"cd", step_dir, "&&", step.command}
	parts = append(parts, step.args...)
	if len(step.env) > 0 {
		parts = append(parts, "env:", strings.Join(step.env, " "))
	}
	return strings.Join(parts, " ")
}

func (app maint_app) step_dir(step command_step) string {
	if step.dir == "" {
		return app.root
	}
	return filepath.Join(app.root, step.dir)
}

func (app maint_app) confirm(prompt string) error {
	if app.yes || app.dry_run {
		return nil
	}

	fmt.Print(prompt + " (y/n) ")
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		return errors.New("aborted")
	}
	return nil
}

func (app maint_app) require_clean_worktree() error {
	if app.dry_run {
		return nil
	}

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = app.root
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(output)) > 0 {
		return errors.New("worktree is not clean")
	}
	return nil
}

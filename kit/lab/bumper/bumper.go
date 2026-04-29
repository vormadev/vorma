package bumper

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	t "github.com/vormadev/vorma/kit/lab/cliutil"
)

// Config controls a Go module tag/proxy publish run.
type Config struct {
	RootDir    string
	ModulePath string
	Version    string
	DryRun     bool
	Yes        bool
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
}

// Publish creates and pushes a Go module tag, then asks the Go proxy for it.
func (config Config) Publish() error {
	config = config.with_defaults()

	if config.ModulePath == "" {
		return errors.New("module path is empty")
	}

	current_tag, err := config.current_tag()
	if err != nil {
		return err
	}

	next_version := strings.TrimSpace(config.Version)
	if next_version == "" {
		fmt.Fprintln(config.Stdout, "current Go module tag:", current_tag)
		next_version, err = config.prompt_line("New Go module version")
		if err != nil {
			return err
		}
	}

	next_tag := strings.TrimSpace(next_version)
	if !strings.HasPrefix(next_tag, "v") {
		next_tag = "v" + next_tag
	}
	if next_tag == "v" {
		return errors.New("version is empty")
	}

	fmt.Fprintf(config.Stdout, "Go module tag: %s -> %s\n", current_tag, next_tag)
	if err := config.confirm("Create and push Go module tag " + next_tag + "?"); err != nil {
		return err
	}

	proxy_query := config.ModulePath + "@" + next_tag
	if config.ModulePath == "all" {
		proxy_query = "all"
	}

	steps := []command_step{
		{name: "create Go module tag", command: "git", args: []string{"tag", next_tag}},
		{
			name:    "push Go module tag",
			command: "git",
			args:    []string{"push", "origin", "refs/tags/" + next_tag},
		},
		{
			name:    "publish Go module to proxy",
			command: "go",
			args:    []string{"list", "-m", proxy_query},
			env:     []string{"GOPROXY=proxy.golang.org"},
		},
	}
	for _, step := range steps {
		if err := config.run_step(step); err != nil {
			return err
		}
	}
	return nil
}

// Run starts the original interactive Go module tag/proxy publish flow.
func Run() {
	config := Config{
		ModulePath: "all",
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}

	t.Blue("have you pushed your code? ")
	t.RequireYes("aborted. go commit and push your changes, then come back")

	if err := config.Publish(); err != nil {
		t.Exit("go module publish failed", err)
	}

	t.Plain("done")
	t.NewLine()
}

type command_step struct {
	name    string
	command string
	args    []string
	env     []string
}

func (config Config) with_defaults() Config {
	if config.RootDir == "" {
		config.RootDir = "."
	}
	if config.Stdin == nil {
		config.Stdin = os.Stdin
	}
	if config.Stdout == nil {
		config.Stdout = os.Stdout
	}
	if config.Stderr == nil {
		config.Stderr = os.Stderr
	}
	return config
}

func (config Config) current_tag() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = config.RootDir
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(config.Stdout, "No existing tags found. Get started by running:")
		fmt.Fprintln(config.Stdout)
		fmt.Fprintln(config.Stdout, "git tag v0.0.1")
		fmt.Fprintln(config.Stdout, "git push origin v0.0.1")
		fmt.Fprintln(config.Stdout, "GOPROXY=proxy.golang.org go list -m all")
		return "", fmt.Errorf("could not resolve current Go module tag: %w", err)
	}

	current_tag := strings.TrimSpace(string(output))
	if current_tag == "" {
		return "", errors.New("current tag is empty")
	}
	return current_tag, nil
}

func (config Config) prompt_line(prompt string) (string, error) {
	fmt.Fprint(config.Stdout, prompt+": ")
	reader := bufio.NewReader(config.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (config Config) confirm(prompt string) error {
	if config.Yes || config.DryRun {
		return nil
	}

	fmt.Fprint(config.Stdout, prompt+" (y/n) ")
	reader := bufio.NewReader(config.Stdin)
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

func (config Config) run_step(step command_step) error {
	fmt.Fprintln(config.Stdout, "==>", step.name)
	if config.DryRun {
		fmt.Fprintln(config.Stdout, "   ", config.format_step(step))
		return nil
	}

	cmd := exec.Command(step.command, step.args...)
	cmd.Dir = config.RootDir
	cmd.Env = append(os.Environ(), step.env...)
	cmd.Stdin = config.Stdin
	cmd.Stdout = config.Stdout
	cmd.Stderr = config.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", step.name, err)
	}
	return nil
}

func (config Config) format_step(step command_step) string {
	parts := []string{"cd", config.RootDir, "&&", step.command}
	parts = append(parts, step.args...)
	if len(step.env) > 0 {
		parts = append(parts, "env:", strings.Join(step.env, " "))
	}
	return strings.Join(parts, " ")
}

package tooling

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const LocalOutputDir = "__.local"

type App struct {
	Root    string
	Yes     bool
	DryRun  bool
	Verbose bool
}

type Step struct {
	Name     string
	Dir      string
	Command  string
	Args     []string
	Env      []string
	UnsetEnv []string
	LogPath  string
}

type Args []string

func (args Args) NewApp() (App, string, []string, error) {
	app := App{}
	command, rest := app.ParseGlobalFlags([]string(args))
	root, err := FindRepoRoot()
	if err != nil {
		return app, "", nil, err
	}
	app.Root = root
	if err := os.Chdir(root); err != nil {
		return app, "", nil, err
	}
	return app, command, rest, nil
}

func (app *App) ParseGlobalFlags(args []string) (string, []string) {
	for len(args) > 0 {
		switch args[0] {
		case "--yes", "-y":
			app.Yes = true
			args = args[1:]
		case "--dry-run":
			app.DryRun = true
			args = args[1:]
		case "--verbose", "-v":
			app.Verbose = true
			args = args[1:]
		default:
			return args[0], args[1:]
		}
	}
	return "", nil
}

func FindRepoRoot() (string, error) {
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

func (app App) RunStep(step Step) error {
	started_at := time.Now()
	step_dir := app.StepDir(step)

	fmt.Println("==>", step.Name)
	if app.Verbose || app.DryRun {
		fmt.Println("   ", app.FormatStep(step, step_dir))
	}
	if step.LogPath != "" {
		fmt.Println("    log:", step.LogPath)
	}
	if app.DryRun {
		fmt.Printf(
			"<== %s passed (%s)\n",
			step.Name,
			time.Since(started_at).Round(time.Millisecond),
		)
		return nil
	}

	var err error
	if step.LogPath != "" {
		err = app.ExecuteLoggedStep(step, step_dir, step.LogPath)
	} else {
		err = app.ExecuteStep(step, step_dir)
	}
	if err != nil {
		fmt.Printf(
			"<== %s failed (%s)\n",
			step.Name,
			time.Since(started_at).Round(time.Millisecond),
		)
		if step.LogPath != "" {
			return err
		}
		return app.CommandError(step.Name, err)
	}
	fmt.Printf("<== %s passed (%s)\n", step.Name, time.Since(started_at).Round(time.Millisecond))
	return nil
}

func (app App) ExecuteStep(step Step, step_dir string) error {
	cmd := exec.Command(step.Command, step.Args...)
	cmd.Dir = step_dir
	cmd.Env = app.CommandEnv(step, step_dir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (app App) ExecuteLoggedStep(step Step, step_dir string, log_path string) error {
	if err := os.MkdirAll(filepath.Dir(log_path), 0755); err != nil {
		return err
	}
	log_file, err := os.OpenFile(log_path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer log_file.Close()

	cmd := exec.Command(step.Command, step.Args...)
	cmd.Dir = step_dir
	cmd.Env = app.CommandEnv(step, step_dir)
	cmd.Stdout = log_file
	cmd.Stderr = log_file
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed; read log at %s: %w", step.Name, log_path, err)
	}
	return nil
}

func (app App) FormatStep(step Step, step_dir string) string {
	parts := []string{"cd", step_dir, "&&", step.Command}
	parts = append(parts, step.Args...)
	if len(step.Env) > 0 {
		parts = append(parts, "env:", strings.Join(step.Env, " "))
	}
	return strings.Join(parts, " ")
}

func (app App) CommandEnv(step Step, step_dir string) []string {
	unset := make(map[string]bool, len(step.UnsetEnv))
	for _, key := range step.UnsetEnv {
		unset[key] = true
	}
	env := make([]string, 0, len(os.Environ())+len(step.Env)+1)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !unset[key] {
			env = append(env, entry)
		}
	}
	env = append(env, step.Env...)
	return append(env, "PWD="+step_dir)
}

func (app App) StepDir(step Step) string {
	if step.Dir == "" {
		return app.Root
	}
	return filepath.Join(app.Root, step.Dir)
}

func (app App) Confirm(prompt string) error {
	if app.Yes || app.DryRun {
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

func (app App) RequireCleanWorktree() error {
	if app.DryRun {
		return nil
	}

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = app.Root
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(output)) > 0 {
		return errors.New("worktree is not clean")
	}
	return nil
}

func (app App) CommandError(command string, err error) error {
	if err == nil {
		return nil
	}
	if strings.TrimSpace(command) == "" {
		return err
	}
	return errors.Join(fmt.Errorf("%s failed", command), err)
}

func (app App) LogPath(scope string, filename string) string {
	return filepath.Join(app.Root, LocalOutputDir, "logs", scope, filename)
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/internal/cmd/internal/tooling"
	"github.com/vormadev/vorma/kit/lab/bumper"
	t "github.com/vormadev/vorma/kit/lab/cliutil"
)

const npm_package_json_path = "internal/pkg/npm/package.json"
const create_package_json_path = "internal/pkg/npm/vorma/create/package.json"
const npm_package_name = "vorma"
const create_package_name = "create-vorma"

type package_json_file struct {
	path string
}

type package_json_content struct {
	lines       []string
	version     string
	version_idx int
}

type release_version string

type release_package struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (app release_app) prepare_release(args []string) error {
	if err := app.require_no_args("prepare", args); err != nil {
		return err
	}

	npm_package := package_json_file{path: npm_package_json_path}
	create_package := package_json_file{path: create_package_json_path}

	npm_content, create_content, err := app.read_npm_release_packages()
	if err != nil {
		return err
	}

	t.Plain("current version: ")
	t.Green(npm_content.version)
	t.NewLine()
	prompted, err := app.prompt_line("what is the new version?")
	if err != nil {
		return err
	}
	next := release_version(prompted)
	if next.npm_version() == "" {
		return fmt.Errorf("version is empty")
	}

	t.Plain("Result: ")
	t.Red(npm_content.version)
	t.Plain("  -->  ")
	t.Green(next.npm_version())
	t.NewLine()
	t.Plain("npm release tag: ")
	t.Green(next.npm_tag())
	t.NewLine()
	t.Plain("go module tag: ")
	t.Green(next.go_tag())
	t.NewLine()
	if !app.Yes && !app.DryRun {
		t.Blue("is this correct?")
	}
	if err := app.Confirm(""); err != nil {
		return err
	}
	if !app.Yes && !app.DryRun {
		t.Blue("write release version ")
		t.Green(next.npm_version())
		t.Blue(" to package files and run release verification?")
	}
	if err := app.Confirm(""); err != nil {
		return err
	}

	if app.DryRun {
		fmt.Printf("would write %s to %s\n", next.npm_version(), npm_package.path)
		fmt.Printf("would write %s to %s\n", next.npm_version(), create_package.path)
	} else {
		if err := npm_package.write_version(npm_content, next.npm_version()); err != nil {
			return err
		}
		if err := create_package.write_version(create_content, next.npm_version()); err != nil {
			return err
		}
	}

	if err := app.run_enforcer_gate(); err != nil {
		return err
	}

	next.print_npm_publish_instructions(app)
	return nil
}

func (app release_app) publish_go(args []string) error {
	if err := app.require_no_args("publish-go", args); err != nil {
		return err
	}

	npm_content, _, err := app.read_npm_release_packages()
	if err != nil {
		return err
	}
	version := release_version(npm_content.version)
	if err := version.verify_go_tag_available(app); err != nil {
		return err
	}
	if err := version.verify_npm_registry(app); err != nil {
		return err
	}
	if err := version.commit_prepared_release(app); err != nil {
		return err
	}
	if err := app.RequireCleanWorktree(); err != nil {
		return err
	}
	return bumper.Config{
		RootDir:    app.Root,
		ModulePath: "github.com/vormadev/vorma",
		Version:    version.go_tag(),
		DryRun:     app.DryRun,
		Yes:        app.Yes,
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}.Publish()
}

func (app release_app) run_enforcer_gate() error {
	return app.RunStep(tooling.Step{
		Name:            "run enforcer gate",
		Command:         "go",
		Args:            []string{"run", "./internal/cmd/enforcer", "gate"},
		HighlightResult: true,
	})
}

func (app release_app) prompt_line(prompt string) (string, error) {
	t.Blue(prompt + " ")
	value, err := t.NewReader().ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (file package_json_file) read() (package_json_content, error) {
	data, err := os.ReadFile(file.path)
	if err != nil {
		return package_json_content{}, err
	}

	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	version_idx := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), `"version":`) {
			version_idx = i
			break
		}
	}
	if version_idx == -1 {
		return package_json_content{}, fmt.Errorf("%s has no version line", file.path)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return package_json_content{}, err
	}
	version, ok := decoded["version"].(string)
	if !ok || version == "" {
		return package_json_content{}, fmt.Errorf("%s has no version value", file.path)
	}

	return package_json_content{
		lines:       lines,
		version:     version,
		version_idx: version_idx,
	}, nil
}

func (file package_json_file) write_version(content package_json_content, version string) error {
	content.lines[content.version_idx] = strings.Replace(
		content.lines[content.version_idx],
		content.version,
		version,
		1,
	)
	return os.WriteFile(file.path, []byte(strings.Join(content.lines, "\n")+"\n"), 0644)
}

func (app release_app) read_npm_release_packages() (package_json_content, package_json_content, error) {
	npm_content, err := (package_json_file{path: npm_package_json_path}).read()
	if err != nil {
		return package_json_content{}, package_json_content{}, err
	}
	create_content, err := (package_json_file{path: create_package_json_path}).read()
	if err != nil {
		return package_json_content{}, package_json_content{}, err
	}
	if npm_content.version != create_content.version {
		return package_json_content{}, package_json_content{}, fmt.Errorf(
			"package versions differ: vorma=%s create-vorma=%s",
			npm_content.version,
			create_content.version,
		)
	}
	return npm_content, create_content, nil
}

func (version release_version) npm_version() string {
	return strings.TrimPrefix(strings.TrimSpace(string(version)), "v")
}

func (version release_version) go_tag() string {
	npm_version := version.npm_version()
	if npm_version == "" {
		return ""
	}
	return "v" + npm_version
}

func (version release_version) npm_tag() string {
	if strings.Contains(version.npm_version(), "pre") {
		return "pre"
	}
	return "latest"
}

func (version release_version) verify_go_tag_available(app release_app) error {
	if app.DryRun {
		return app.RunStep(tooling.Step{
			Name:    "verify Go module tag is available",
			Command: "git",
			Args: []string{
				"ls-remote",
				"--exit-code",
				"--tags",
				"origin",
				"refs/tags/" + version.go_tag(),
			},
		})
	}

	cmd := exec.Command(
		"git",
		"ls-remote",
		"--exit-code",
		"--tags",
		"origin",
		"refs/tags/"+version.go_tag(),
	)
	cmd.Dir = app.Root
	if err := cmd.Run(); err == nil {
		return fmt.Errorf("Go module tag %s already exists on origin", version.go_tag())
	} else if exit_err, ok := err.(*exec.ExitError); ok && exit_err.ExitCode() == 2 {
		return nil
	} else {
		return fmt.Errorf("error checking Go module tag %s on origin: %w", version.go_tag(), err)
	}
}

func (version release_version) verify_npm_registry(app release_app) error {
	for _, pkg := range app.release_packages() {
		step := tooling.Step{
			Name:    "verify npm package " + pkg.Name + "@" + version.npm_version(),
			Command: "npm",
			Args:    []string{"view", pkg.Name + "@" + version.npm_version(), "version"},
		}
		if err := app.RunStep(step); err != nil {
			return err
		}
	}
	return nil
}

func (version release_version) commit_prepared_release(app release_app) error {
	if err := app.RunStep(tooling.Step{
		Name:    "show prepared release status",
		Command: "git",
		Args:    []string{"status", "--short"},
	}); err != nil {
		return err
	}
	if err := app.Confirm(
		"Stage every current worktree change with `git add .`, commit " + version.go_tag() + ", and push?",
	); err != nil {
		return err
	}

	steps := []tooling.Step{
		{
			Name:    "stage prepared release",
			Command: "git",
			Args:    []string{"add", "."},
		},
		{
			Name:    "commit prepared release",
			Command: "git",
			Args:    []string{"commit", "-m", version.go_tag(), "--no-verify"},
		},
		{
			Name:    "push prepared release commit",
			Command: "git",
			Args:    []string{"push"},
		},
	}
	for _, step := range steps {
		if err := app.RunStep(step); err != nil {
			return err
		}
	}
	return nil
}

func (version release_version) print_npm_publish_instructions(app release_app) {
	fmt.Println()
	fmt.Println("Run these npm publish commands directly from your terminal:")
	fmt.Println()
	for _, pkg := range app.release_packages() {
		pkg.print_publish_command(app, version)
	}
	fmt.Println("cd " + app.Root)
	fmt.Println()
	fmt.Println("After npm publish succeeds, run:")
	fmt.Println()
	fmt.Println("make publish-go")
}

func (pkg release_package) print_publish_command(app release_app, version release_version) {
	fmt.Println("cd " + filepath.Join(app.Root, filepath.Dir(pkg.Path)))
	fmt.Println("npm publish --access public --tag " + version.npm_tag())
	fmt.Println()
}

func (app release_app) release_packages() []release_package {
	return []release_package{
		{Name: npm_package_name, Path: npm_package_json_path},
		{Name: create_package_name, Path: create_package_json_path},
	}
}

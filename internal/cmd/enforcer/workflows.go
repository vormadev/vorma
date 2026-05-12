package main

import (
	"context"
	"fmt"

	"github.com/vormadev/vorma/internal/cmd/internal/tooling"
	"github.com/vormadev/vorma/kit/tasks"
)

const docs_dir = "internal/apps/docs"
const framework_tests_dir = "internal/framework_tests"
const matcher_tests_dir = "internal/matcher_tests"
const npm_dir = "internal/pkg/npm"
const create_npm_dir = "internal/pkg/npm/vorma/create"

type enforcer_input struct {
	app       enforcer_app
	intensity int
}

type go_module_group []string

type ts_path_group []string

type ts_project struct {
	label   string
	dir     string
	project string
	json    bool
}

type ts_project_group []ts_project

type action_matrix struct {
	go_fw      *tasks.Task[enforcer_input, struct{}]
	go_matcher *tasks.Task[enforcer_input, struct{}]
	go_other   *tasks.Task[enforcer_input, struct{}]
	ts_fw      *tasks.Task[enforcer_input, struct{}]
	ts_matcher *tasks.Task[enforcer_input, struct{}]
	ts_other   *tasks.Task[enforcer_input, struct{}]
}

var (
	go_fw_modules      = go_module_group{framework_tests_dir}
	go_matcher_modules = go_module_group{matcher_tests_dir}
	go_other_modules   = go_module_group{"", docs_dir}
)

var ts_fw_paths = ts_path_group{
	"internal/pkg/npm/vorma/core",
	"internal/pkg/npm/vorma/tests",
	"internal/pkg/npm/vorma/tsx",
	"internal/pkg/npm/vorma/vite",
	framework_tests_dir,
}

var ts_other_paths = ts_path_group{
	"package.json",
	"internal/pkg/npm/package.json",
	"internal/pkg/npm/tsconfig.base.json",
	"internal/pkg/npm/tsdown.config.ts",
	"internal/pkg/npm/vitest.config.ts",
	"internal/pkg/npm/kit",
	"internal/pkg/npm/vorma/create",
	docs_dir,
}

var ts_matcher_paths = ts_path_group{
	matcher_tests_dir,
}

var typecheck_other_projects = ts_project_group{
	{label: "kit", dir: npm_dir, project: "./kit"},
	{label: "create-vorma", dir: npm_dir, project: "./vorma/create"},
	{label: "docs", dir: docs_dir, project: "tsconfig.json", json: true},
}

var typecheck_matcher_projects = ts_project_group{
	{
		label:   "matcher tests",
		dir:     npm_dir,
		project: "../../matcher_tests/tsconfig.json",
		json:    true,
	},
}

var typecheck_fw_source_projects = ts_project_group{
	{label: "framework core", dir: npm_dir, project: "./vorma/core"},
	{label: "framework Preact adapter", dir: npm_dir, project: "./vorma/tsx/preact"},
	{label: "framework Remix adapter", dir: npm_dir, project: "./vorma/tsx/remix"},
	{label: "framework React adapter", dir: npm_dir, project: "./vorma/tsx/react"},
	{label: "framework Solid adapter", dir: npm_dir, project: "./vorma/tsx/solid"},
	{label: "framework Vite integration", dir: npm_dir, project: "./vorma/vite"},
	{
		label:   "framework TypeScript tests",
		dir:     npm_dir,
		project: "./vorma/tests/tsconfig.json",
		json:    true,
	},
}

var typecheck_fw_fixture_projects = ts_project_group{
	{label: "framework fixture", dir: framework_tests_dir, project: "tsconfig.json", json: true},
}

var task_verify_repo_shape = tasks.NewTask(
	func(_ *tasks.Cache, input enforcer_input) (struct{}, error) {
		if err := input.app.check_repo_shape(); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	},
)

var (
	task_install_go_fw      = go_fw_modules.tidy_task("install Go fw")
	task_install_go_matcher = go_matcher_modules.tidy_task("install Go matcher")
	task_install_go_other   = go_other_modules.tidy_task("install Go other")
	task_format_go_fw       = go_fw_modules.fmt_task("format Go fw")
	task_format_go_matcher  = go_matcher_modules.fmt_task("format Go matcher")
	task_format_go_other    = go_other_modules.fmt_task("format Go other")
	task_format_ts_fw       = ts_fw_paths.oxfmt_task("format TypeScript fw")
	task_format_ts_matcher  = ts_matcher_paths.oxfmt_task("format TypeScript matcher")
	task_format_ts_other    = ts_other_paths.oxfmt_task("format TypeScript other")
	task_lint_go_fw         = go_fw_modules.lint_task()
	task_lint_go_matcher    = go_matcher_modules.lint_task()
	task_lint_go_other      = go_other_modules.lint_task()
	task_lint_ts_fw         = ts_fw_paths.oxlint_task("lint TypeScript fw", nil)
	task_lint_ts_matcher    = ts_matcher_paths.oxlint_task("lint TypeScript matcher", nil)
	task_lint_ts_other      = ts_other_paths.oxlint_task("lint TypeScript other", nil)
	task_fix_go_fw          = go_fw_modules.fix_task("fix Go fw")
	task_fix_go_matcher     = go_matcher_modules.fix_task("fix Go matcher")
	task_fix_go_other       = go_other_modules.fix_task("fix Go other")
	task_fix_ts_fw          = ts_fw_paths.oxlint_task("fix TypeScript fw", []string{"--fix"})
)

var task_fix_ts_matcher = ts_matcher_paths.oxlint_task(
	"fix TypeScript matcher",
	[]string{"--fix"},
)

var task_fix_ts_other = ts_other_paths.oxlint_task(
	"fix TypeScript other",
	[]string{"--fix"},
)

var task_install_root_ts = enforcer_task(tooling.Step{
	Name:    "install root TypeScript dependencies",
	Command: "pnpm",
	Args:    []string{"i", "--config.confirmModulesPurge=false"},
})

var task_install_npm_ts = enforcer_task(tooling.Step{
	Name:    "install npm package TypeScript dependencies",
	Dir:     npm_dir,
	Command: "pnpm",
	Args:    []string{"i", "--config.confirmModulesPurge=false"},
})

var task_install_create_ts = enforcer_task(tooling.Step{
	Name:    "install create-vorma TypeScript dependencies",
	Dir:     create_npm_dir,
	Command: "pnpm",
	Args:    []string{"i", "--config.confirmModulesPurge=false"},
})

var task_install_fw_ts = enforcer_task(tooling.Step{
	Name:    "install framework TypeScript dependencies",
	Dir:     framework_tests_dir,
	Command: "pnpm",
	Args:    []string{"i", "--config.confirmModulesPurge=false"},
})

var task_install_docs_ts = enforcer_task(tooling.Step{
	Name:    "install docs TypeScript dependencies",
	Dir:     docs_dir,
	Command: "pnpm",
	Args:    []string{"i", "--config.confirmModulesPurge=false"},
})

var task_install_ts_fw = tasks.NewTask(
	func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		return input.run_parallel(ctx, task_install_npm_ts, task_install_fw_ts)
	},
)

var task_install_ts_other = tasks.NewTask(
	func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		return input.run_parallel(
			ctx,
			task_install_root_ts,
			task_install_npm_ts,
			task_install_create_ts,
			task_install_docs_ts,
		)
	},
)

var task_install_ts_matcher = tasks.NewTask(
	func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		return input.run_parallel(
			ctx,
			task_install_npm_ts,
			task_install_matcher_tests_ts,
		)
	},
)

var task_install_matcher_tests_ts = enforcer_task(tooling.Step{
	Name:    "install matcher TypeScript test dependencies",
	Dir:     matcher_tests_dir,
	Command: "pnpm",
	Args:    []string{"i", "--config.confirmModulesPurge=false"},
})

var (
	task_compile_check_go_fw      = go_fw_modules.compile_check_task("compile check Go fw")
	task_compile_check_go_matcher = go_matcher_modules.compile_check_task(
		"compile check Go matcher",
	)
	task_compile_check_go_other = go_other_modules.compile_check_task("compile check Go other")
	task_test_go_fw             = go_fw_modules.test_task("test Go fw")
	task_test_go_matcher_base   = go_matcher_modules.test_task("test Go matcher")
	task_test_go_other          = go_other_modules.test_task("test Go other")
)

var task_test_ts_fw = tasks.NewTask(
	func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		if _, err := input.run_parallel(ctx, task_build_ts, task_install_fw_ts); err != nil {
			return struct{}{}, err
		}
		if _, err := input.run_step(tooling.Step{
			Name:    "test TypeScript fw",
			Dir:     npm_dir,
			Command: "pnpm",
			Args:    []string{"vitest", "run", "--reporter=dot", "vorma"},
		}); err != nil {
			return struct{}{}, err
		}
		if _, err := input.run_step(tooling.Step{
			Name:    "test framework production runtime",
			Dir:     framework_tests_dir,
			Command: "go",
			Args:    []string{"run", "./cmd/bombadil", "test-prod"},
		}); err != nil {
			return struct{}{}, err
		}
		return input.run_step(tooling.Step{
			Name:    "test framework development runtime",
			Dir:     framework_tests_dir,
			Command: "go",
			Args:    []string{"run", "./cmd/bombadil", "test-dev"},
		})
	},
)

var task_test_ts_other = tasks.NewTask(
	func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		if _, err := task_build_ts.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return input.run_step(tooling.Step{
			Name:    "test TypeScript other",
			Dir:     npm_dir,
			Command: "pnpm",
			Args:    []string{"vitest", "run", "--reporter=dot", "--exclude", "vorma/**"},
		})
	},
)

var task_test_matcher = tasks.NewTask(
	func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		if _, err := input.run_parallel(
			ctx,
			task_build_ts,
			task_install_matcher_tests_ts,
		); err != nil {
			return struct{}{}, err
		}
		return task_test_go_matcher_base.Run(ctx, input)
	},
)

var task_build_ts = tasks.NewTask(
	func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		if _, err := task_install_npm_ts.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return input.run_step(tooling.Step{
			Name:    "build TypeScript packages",
			Dir:     npm_dir,
			Command: "pnpm",
			Args:    []string{"tsdown"},
		})
	},
)

var task_build_fw_fixture = enforcer_task(tooling.Step{
	Name:    "build framework fixture",
	Dir:     framework_tests_dir,
	Command: "go",
	Args:    []string{"run", "./cmd/bombadil", "build"},
})

var task_typecheck_ts_fw_source = typecheck_fw_source_projects.typecheck_task(
	"typecheck TypeScript fw",
	task_build_ts,
	task_install_fw_ts,
)

var task_typecheck_ts_fw_fixture = typecheck_fw_fixture_projects.typecheck_task(
	"typecheck TypeScript fw",
)

var task_typecheck_ts_fw = tasks.NewTask(
	func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		if _, err := input.run_parallel(ctx, task_build_ts, task_install_fw_ts); err != nil {
			return struct{}{}, err
		}
		if _, err := task_build_fw_fixture.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return input.run_parallel(
			ctx,
			task_typecheck_ts_fw_source,
			task_typecheck_ts_fw_fixture,
		)
	},
)

var task_typecheck_ts_other = typecheck_other_projects.typecheck_task(
	"typecheck TypeScript other",
	task_build_ts,
	task_install_create_ts,
	task_install_docs_ts,
)

var task_typecheck_ts_matcher = typecheck_matcher_projects.typecheck_task(
	"typecheck TypeScript matcher",
	task_build_ts,
	task_install_matcher_tests_ts,
)

var (
	task_stress_go_fw           = go_fw_modules.stress_task("stress Go fw", "stress-go-fw.log")
	task_stress_go_matcher_base = go_matcher_modules.stress_task(
		"stress Go matcher",
		"stress-go-matcher.log",
	)
	task_stress_go_other = go_other_modules.stress_task(
		"stress Go other",
		"stress-go-other.log",
	)
)

var task_stress_matcher = tasks.NewTask(
	func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		if _, err := input.run_parallel(
			ctx,
			task_build_ts,
			task_install_matcher_tests_ts,
		); err != nil {
			return struct{}{}, err
		}
		return task_stress_go_matcher_base.Run(ctx, input)
	},
)

var task_stress_ts_fw = tasks.NewTask(
	func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		if _, err := input.run_parallel(
			ctx,
			task_build_ts,
			task_typecheck_ts_fw,
			task_test_ts_fw,
		); err != nil {
			return struct{}{}, err
		}
		if _, err := input.run_step(tooling.Step{
			Name:    "stress framework production runtime",
			Dir:     framework_tests_dir,
			Command: "go",
			Args: []string{
				"run",
				"./cmd/bombadil",
				"test-prod",
				"-intensity",
				fmt.Sprint(input.intensity),
			},
			LogPath: input.app.LogPath("enforcer", "stress-ts-fw-prod.log"),
		}); err != nil {
			return struct{}{}, err
		}
		return input.run_step(tooling.Step{
			Name:    "stress framework development runtime",
			Dir:     framework_tests_dir,
			Command: "go",
			Args: []string{
				"run",
				"./cmd/bombadil",
				"test-dev",
				"-intensity",
				fmt.Sprint(input.intensity),
			},
			LogPath: input.app.LogPath("enforcer", "stress-ts-fw-dev.log"),
		})
	},
)

var task_stress_ts_other = tasks.NewTask(
	func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		if _, err := task_build_ts.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		for range input.intensity {
			if _, err := input.run_step(tooling.Step{
				Name:    "stress TypeScript other",
				Dir:     npm_dir,
				Command: "pnpm",
				Args:    []string{"vitest", "run", "--reporter=dot", "--exclude", "vorma/**"},
			}); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, nil
	},
)

var action_matrices = map[action_name]action_matrix{
	action_vals.Install: {
		go_fw:      task_install_go_fw,
		go_matcher: task_install_go_matcher,
		go_other:   task_install_go_other,
		ts_fw:      task_install_ts_fw,
		ts_matcher: task_install_ts_matcher,
		ts_other:   task_install_ts_other,
	},
	action_vals.Format: {
		go_fw:      task_format_go_fw,
		go_matcher: task_format_go_matcher,
		go_other:   task_format_go_other,
		ts_fw:      task_format_ts_fw,
		ts_matcher: task_format_ts_matcher,
		ts_other:   task_format_ts_other,
	},
	action_vals.Lint: {
		go_fw:      task_lint_go_fw,
		go_matcher: task_lint_go_matcher,
		go_other:   task_lint_go_other,
		ts_fw:      task_lint_ts_fw,
		ts_matcher: task_lint_ts_matcher,
		ts_other:   task_lint_ts_other,
	},
	action_vals.Fix: {
		go_fw:      task_fix_go_fw,
		go_matcher: task_fix_go_matcher,
		go_other:   task_fix_go_other,
		ts_fw:      task_fix_ts_fw,
		ts_matcher: task_fix_ts_matcher,
		ts_other:   task_fix_ts_other,
	},
	action_vals.Typecheck: {
		go_fw:      task_compile_check_go_fw,
		go_matcher: task_compile_check_go_matcher,
		go_other:   task_compile_check_go_other,
		ts_fw:      task_typecheck_ts_fw,
		ts_matcher: task_typecheck_ts_matcher,
		ts_other:   task_typecheck_ts_other,
	},
	action_vals.Test: {
		go_fw:      task_test_go_fw,
		go_matcher: task_test_matcher,
		go_other:   task_test_go_other,
		ts_fw:      task_test_ts_fw,
		ts_matcher: task_test_matcher,
		ts_other:   task_test_ts_other,
	},
	action_vals.Build: {
		go_fw:      task_compile_check_go_fw,
		go_matcher: task_compile_check_go_matcher,
		go_other:   task_compile_check_go_other,
		ts_fw:      task_build_ts,
		ts_matcher: task_build_ts,
		ts_other:   task_build_ts,
	},
}

var stress_action_matrix = action_matrix{
	go_fw:      task_stress_go_fw,
	go_matcher: task_stress_matcher,
	go_other:   task_stress_go_other,
	ts_fw:      task_stress_ts_fw,
	ts_matcher: task_stress_matcher,
	ts_other:   task_stress_ts_other,
}

var gate_actions = []action_name{
	action_vals.Install,
	action_vals.Fix,
	action_vals.Format,
	action_vals.Lint,
	action_vals.Typecheck,
	action_vals.Build,
	action_vals.Test,
}

var action_phases = [][]action_name{
	{action_vals.Install},
	{action_vals.Fix},
	{action_vals.Format},
	{action_vals.Lint, action_vals.Typecheck, action_vals.Build},
	{action_vals.Test, action_vals.Stress},
}

func (app enforcer_app) run_request(req enforcer_request) error {
	ctx := tasks.NewCache(context.Background())
	input := enforcer_input{app: app, intensity: req.intensity}
	action_set := make(map[action_name]bool, len(req.actions))
	run_shape_check := false
	for _, action := range req.actions {
		if action == action_vals.Gate {
			run_shape_check = true
			for _, gate_action := range gate_actions {
				action_set[gate_action] = true
			}
		} else {
			action_set[action] = true
		}
	}
	if run_shape_check {
		if _, err := task_verify_repo_shape.Run(ctx, input); err != nil {
			return err
		}
	}

	for _, phase := range action_phases {
		task_list := []*tasks.Task[enforcer_input, struct{}]{}
		for _, action := range phase {
			if !action_set[action] {
				continue
			}
			tasks_for_action, err := app.action_tasks(action, req)
			if err != nil {
				return err
			}
			task_list = append(task_list, tasks_for_action...)
		}
		if len(task_list) == 0 {
			continue
		}
		if _, err := input.run_parallel(ctx, task_list...); err != nil {
			return err
		}
	}

	return nil
}

func (app enforcer_app) action_tasks(
	action action_name,
	req enforcer_request,
) ([]*tasks.Task[enforcer_input, struct{}], error) {
	if action == action_vals.Stress {
		if req.intensity <= 0 {
			return nil, fmt.Errorf("stress requires --intensity with a positive integer")
		}
		return stress_action_matrix.tasks(req), nil
	}
	matrix, ok := action_matrices[action]
	if !ok {
		return nil, fmt.Errorf("unsupported action %q", action)
	}
	return matrix.tasks(req), nil
}

func (matrix action_matrix) tasks(req enforcer_request) []*tasks.Task[enforcer_input, struct{}] {
	task_list := []*tasks.Task[enforcer_input, struct{}]{}
	if req.lang == lang_scope_vals.All || req.lang == lang_scope_vals.Go {
		if req.scope == target_scope_vals.All || req.scope == target_scope_vals.Fw {
			task_list = append(task_list, matrix.go_fw)
		}
		if req.scope == target_scope_vals.All || req.scope == target_scope_vals.Matcher {
			task_list = append(task_list, matrix.go_matcher)
		}
		if req.scope == target_scope_vals.All || req.scope == target_scope_vals.Other {
			task_list = append(task_list, matrix.go_other)
		}
	}
	if req.lang == lang_scope_vals.All || req.lang == lang_scope_vals.Ts {
		if req.scope == target_scope_vals.All || req.scope == target_scope_vals.Fw {
			task_list = append(task_list, matrix.ts_fw)
		}
		if req.scope == target_scope_vals.All || req.scope == target_scope_vals.Matcher {
			task_list = append(task_list, matrix.ts_matcher)
		}
		if req.scope == target_scope_vals.All || req.scope == target_scope_vals.Other {
			task_list = append(task_list, matrix.ts_other)
		}
	}
	return task_list
}

func (app enforcer_app) clean_js() error {
	if err := app.Confirm("Remove every node_modules directory?"); err != nil {
		return err
	}
	if err := app.RunStep(tooling.Step{Name: "remove root node_modules", Command: "rm", Args: []string{"-rf", "node_modules"}}); err != nil {
		return err
	}
	return app.RunStep(tooling.Step{
		Name:    "remove nested node_modules",
		Command: "find",
		Args: []string{
			".",
			"-path",
			"*/node_modules",
			"-type",
			"d",
			"-prune",
			"-exec",
			"rm",
			"-rf",
			"{}",
			"+",
		},
	})
}

func (group go_module_group) tidy_task(name string) *tasks.Task[enforcer_input, struct{}] {
	return group.command_task(name, "go", []string{"mod", "tidy"}, "")
}

func (group go_module_group) fmt_task(name string) *tasks.Task[enforcer_input, struct{}] {
	return group.command_task(name, "go", []string{"fmt", "./..."}, "")
}

func (group go_module_group) fix_task(name string) *tasks.Task[enforcer_input, struct{}] {
	return group.command_task(name, "go", []string{"fix", "./..."}, "")
}

func (group go_module_group) compile_check_task(name string) *tasks.Task[enforcer_input, struct{}] {
	return group.command_task(name, "go", []string{"test", "./...", "-run", "^$", "-vet=off"}, "")
}

func (group go_module_group) test_task(name string) *tasks.Task[enforcer_input, struct{}] {
	return group.command_task(name, "go", []string{"test", "-race", "./..."}, "")
}

func (group go_module_group) stress_task(
	name string,
	log_file string,
) *tasks.Task[enforcer_input, struct{}] {
	return tasks.NewTask(func(_ *tasks.Cache, input enforcer_input) (struct{}, error) {
		for _, dir := range group {
			if _, err := input.run_step(tooling.Step{
				Name:    name + " module " + module_label(dir),
				Dir:     dir,
				Command: "go",
				Args: []string{
					"test",
					"-race",
					fmt.Sprintf("-count=%d", input.intensity),
					"./...",
				},
				LogPath: input.app.LogPath("enforcer", log_file),
			}); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, nil
	})
}

func (group go_module_group) lint_task() *tasks.Task[enforcer_input, struct{}] {
	return tasks.NewTask(func(_ *tasks.Cache, input enforcer_input) (struct{}, error) {
		for _, dir := range group {
			for _, step := range []tooling.Step{
				{
					Name:    "vet Go module " + module_label(dir),
					Dir:     dir,
					Command: "go",
					Args:    []string{"vet", "./..."},
				},
				{
					Name:     "staticcheck Go module " + module_label(dir),
					Dir:      dir,
					Command:  "staticcheck",
					Args:     []string{"./..."},
					UnsetEnv: []string{"GO111MODULE"},
				},
			} {
				if _, err := input.run_step(step); err != nil {
					return struct{}{}, err
				}
			}
		}
		return struct{}{}, nil
	})
}

func (group go_module_group) command_task(
	name string,
	command string,
	args []string,
	log_path string,
) *tasks.Task[enforcer_input, struct{}] {
	return tasks.NewTask(func(_ *tasks.Cache, input enforcer_input) (struct{}, error) {
		for _, dir := range group {
			step := tooling.Step{
				Name:    name + " module " + module_label(dir),
				Dir:     dir,
				Command: command,
				Args:    args,
				LogPath: log_path,
			}
			if _, err := input.run_step(step); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, nil
	})
}

func (paths ts_path_group) oxfmt_task(name string) *tasks.Task[enforcer_input, struct{}] {
	return tasks.NewTask(func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		if _, err := task_install_root_ts.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		args := append([]string{"oxfmt"}, paths...)
		return input.run_step(tooling.Step{Name: name, Command: "pnpm", Args: args})
	})
}

func (paths ts_path_group) oxlint_task(
	name string,
	extra_args []string,
) *tasks.Task[enforcer_input, struct{}] {
	return tasks.NewTask(func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		if _, err := task_install_root_ts.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		args := []string{"oxlint"}
		args = append(args, extra_args...)
		args = append(args, paths...)
		return input.run_step(tooling.Step{Name: name, Command: "pnpm", Args: args})
	})
}

func (projects ts_project_group) typecheck_task(
	name string,
	prerequisites ...*tasks.Task[enforcer_input, struct{}],
) *tasks.Task[enforcer_input, struct{}] {
	return tasks.NewTask(func(ctx *tasks.Cache, input enforcer_input) (struct{}, error) {
		if _, err := input.run_parallel(ctx, prerequisites...); err != nil {
			return struct{}{}, err
		}
		for _, project := range projects {
			args := []string{"tsgo", "--noEmit", "--project", project.project}
			if project.json {
				args = []string{"tsgo", "-p", project.project, "--noEmit"}
			}
			if _, err := input.run_step(tooling.Step{
				Name:    name + " project " + project.label,
				Dir:     project.dir,
				Command: "pnpm",
				Args:    args,
			}); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, nil
	})
}

func enforcer_task(step tooling.Step) *tasks.Task[enforcer_input, struct{}] {
	return tasks.NewTask(func(_ *tasks.Cache, input enforcer_input) (struct{}, error) {
		return input.run_step(step)
	})
}

func (input enforcer_input) run_step(step tooling.Step) (struct{}, error) {
	if err := input.app.RunStep(step); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
}

func (input enforcer_input) run_parallel(
	ctx *tasks.Cache,
	task_list ...*tasks.Task[enforcer_input, struct{}],
) (struct{}, error) {
	bound := make([]tasks.Prepared, 0, len(task_list))
	for _, task := range task_list {
		if task != nil {
			bound = append(bound, task.BindInput(input))
		}
	}
	return struct{}{}, ctx.RunParallel(bound...)
}

func module_label(dir string) string {
	if dir == "" {
		return "repo"
	}
	return dir
}

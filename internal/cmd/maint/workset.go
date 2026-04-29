package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/vormadev/vorma/kit/tasks"
)

type work_id string

const (
	work_verify_repo_shape work_id = "verify-repo-shape"

	work_install_js           work_id = "install-js"
	work_install_js_root      work_id = "install-js-root"
	work_install_js_npm       work_id = "install-js-npm"
	work_install_js_create    work_id = "install-js-create"
	work_install_js_framework work_id = "install-js-framework"
	work_install_js_docs      work_id = "install-js-docs"

	work_build_ts                     work_id = "build-ts"
	work_fmt_ts                       work_id = "fmt-ts"
	work_lint_ts                      work_id = "lint-ts"
	work_typecheck_ts                 work_id = "typecheck-ts"
	work_typecheck_ts_other           work_id = "typecheck-ts-other"
	work_typecheck_ts_framework       work_id = "typecheck-ts-framework"
	work_typecheck_ts_kit             work_id = "typecheck-ts-kit"
	work_typecheck_ts_core            work_id = "typecheck-ts-core"
	work_typecheck_ts_preact          work_id = "typecheck-ts-preact"
	work_typecheck_ts_react           work_id = "typecheck-ts-react"
	work_typecheck_ts_solid           work_id = "typecheck-ts-solid"
	work_typecheck_ts_vite            work_id = "typecheck-ts-vite"
	work_typecheck_ts_create          work_id = "typecheck-ts-create"
	work_typecheck_ts_framework_tests work_id = "typecheck-ts-framework-tests"

	work_test_ts           work_id = "test-ts"
	work_test_ts_other     work_id = "test-ts-other"
	work_test_ts_framework work_id = "test-ts-framework"

	work_test_go           work_id = "test-go"
	work_test_go_other     work_id = "test-go-other"
	work_test_root_go      work_id = "test-root-go"
	work_test_internal_go  work_id = "test-internal-go"
	work_test_kit          work_id = "test-kit"
	work_test_lab          work_id = "test-lab"
	work_test_docs         work_id = "test-docs"
	work_test_go_framework work_id = "test-go-framework"

	work_typecheck_framework_fixture_ts work_id = "typecheck-framework-fixture-ts"
	work_framework_prereqs              work_id = "framework-prereqs"
	work_test_framework                 work_id = "test-framework"
	work_test_framework_prod            work_id = "test-framework-prod"
	work_test_framework_dev             work_id = "test-framework-dev"

	work_stress                work_id = "stress"
	work_stress_other          work_id = "stress-other"
	work_stress_root_go        work_id = "stress-root-go"
	work_stress_docs_go        work_id = "stress-docs-go"
	work_stress_go_framework   work_id = "stress-go-framework"
	work_stress_framework      work_id = "stress-framework"
	work_stress_framework_prod work_id = "stress-framework-prod"
	work_stress_framework_dev  work_id = "stress-framework-dev"

	work_gate work_id = "gate"
)

type maint_task_input struct {
	app       maint_app
	framework framework_options
	stress    stress_options
}

var maint_output_mu sync.Mutex

var task_verify_repo_shape = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		action := input.app.check_repo_shape
		return input.run_action("verify repo maintenance shape", action)
	},
)

var task_install_js = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_parallel(
			ctx,
			task_install_js_root,
			task_install_js_npm,
			task_install_js_create,
			task_install_js_framework,
			task_install_js_docs,
		)
	},
)

var task_install_js_root = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_step(
			command_step{
				name:    "install root JavaScript dependencies",
				command: "pnpm",
				args:    []string{"i", "--config.confirmModulesPurge=false"},
			},
		)
	},
)

var task_install_js_npm = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_step(
			command_step{
				name:    "install npm package dependencies",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"i", "--config.confirmModulesPurge=false"},
			},
		)
	},
)

var task_install_js_create = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_step(
			command_step{
				name:    "install create-vorma dependencies",
				dir:     create_npm_dir,
				command: "pnpm",
				args:    []string{"i", "--config.confirmModulesPurge=false"},
			},
		)
	},
)

var task_install_js_framework = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_step(
			command_step{
				name:    "install framework test dependencies",
				dir:     framework_tests_dir,
				command: "pnpm",
				args:    []string{"i", "--config.confirmModulesPurge=false"},
			},
		)
	},
)

var task_install_js_docs = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_step(
			command_step{
				name:    "install docs dependencies",
				dir:     docs_dir,
				command: "pnpm",
				args:    []string{"i", "--config.confirmModulesPurge=false"},
			},
		)
	},
)

var task_build_ts = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_install_js_npm.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return input.run_step(
			command_step{
				name:    "build TypeScript packages",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsdown"},
			},
		)
	},
)

var task_fmt_ts = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_install_js_root.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return input.run_step(
			command_step{name: "format TypeScript", command: "pnpm", args: []string{"oxfmt"}},
		)
	},
)

var task_lint_ts = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_install_js_root.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return input.run_step(
			command_step{name: "lint TypeScript", command: "pnpm", args: []string{"oxlint"}},
		)
	},
)

var task_typecheck_ts = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_parallel(ctx, task_typecheck_ts_other, task_typecheck_ts_framework)
	},
)

var task_typecheck_ts_other = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_parallel(ctx, task_typecheck_ts_kit, task_typecheck_ts_create)
	},
)

var task_typecheck_ts_framework = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_parallel(
			ctx,
			task_typecheck_ts_core,
			task_typecheck_ts_preact,
			task_typecheck_ts_react,
			task_typecheck_ts_solid,
			task_typecheck_ts_vite,
			task_typecheck_ts_framework_tests,
			task_typecheck_framework_fixture_ts,
		)
	},
)

var task_typecheck_ts_kit = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_install_js_npm.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return input.run_step(
			command_step{
				name:    "typecheck kit",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsgo", "--noEmit", "--project", "./kit"},
			},
		)
	},
)

var task_typecheck_ts_core = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_npm_typecheck(ctx, "typecheck framework core", "./vorma/core")
	},
)

var task_typecheck_ts_preact = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_npm_typecheck(
			ctx,
			"typecheck framework Preact adapter",
			"./vorma/tsx/preact",
		)
	},
)

var task_typecheck_ts_react = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_npm_typecheck(
			ctx,
			"typecheck framework React adapter",
			"./vorma/tsx/react",
		)
	},
)

var task_typecheck_ts_solid = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_npm_typecheck(
			ctx,
			"typecheck framework Solid adapter",
			"./vorma/tsx/solid",
		)
	},
)

var task_typecheck_ts_vite = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_npm_typecheck(ctx, "typecheck framework Vite integration", "./vorma/vite")
	},
)

var task_typecheck_ts_create = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := input.run_parallel(ctx, task_install_js_npm, task_install_js_create); err != nil {
			return struct{}{}, err
		}
		return input.run_step(
			command_step{
				name:    "typecheck create-vorma",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsgo", "--noEmit", "--project", "./vorma/create"},
			},
		)
	},
)

var task_typecheck_ts_framework_tests = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_install_js_npm.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return input.run_step(
			command_step{
				name:    "typecheck framework TypeScript tests",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsgo", "-p", "./vorma/tests/tsconfig.json", "--noEmit"},
			},
		)
	},
)

var task_test_ts = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_parallel(ctx, task_test_ts_other, task_test_ts_framework)
	},
)

var task_test_ts_other = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_install_js_npm.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return input.run_step(
			command_step{
				name:    "test non-framework TypeScript packages",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"vitest", "run", "--reporter=dot", "--exclude", "vorma/**"},
			},
		)
	},
)

var task_test_ts_framework = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_build_ts.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return input.run_step(
			command_step{
				name:    "test framework TypeScript packages",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"vitest", "run", "--reporter=dot", "vorma"},
			},
		)
	},
)

var task_test_go = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_parallel(ctx, task_test_go_other, task_test_go_framework)
	},
)

var task_test_go_other = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_parallel(ctx, task_test_root_go, task_test_docs)
	},
)

var task_test_root_go = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_step(
			command_step{
				name:    "test root Go module",
				command: "go",
				args:    []string{"test", "-race", "./..."},
			},
		)
	},
)

var task_test_internal_go = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_step(
			command_step{
				name:    "test internal Go packages",
				command: "go",
				args:    []string{"test", "-race", "./internal/..."},
			},
		)
	},
)

var task_test_kit = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_step(
			command_step{
				name:    "test kit Go packages",
				command: "go",
				args:    []string{"test", "-race", "./kit/..."},
			},
		)
	},
)

var task_test_lab = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_step(
			command_step{
				name:    "test lab Go packages",
				command: "go",
				args:    []string{"test", "-race", "./kit/lab/..."},
			},
		)
	},
)

var task_test_docs = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_step(
			command_step{
				name:    "test docs Go module",
				dir:     docs_dir,
				command: "go",
				args:    []string{"test", "-race", "./..."},
			},
		)
	},
)

var task_test_go_framework = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_step(
			command_step{
				name:    "test framework Go module",
				dir:     framework_tests_dir,
				command: "go",
				args:    []string{"test", "-race", "./...", "-count=1"},
			},
		)
	},
)

var task_typecheck_framework_fixture_ts = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_install_js_framework.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return input.run_step(
			command_step{
				name:    "typecheck framework fixture TypeScript",
				dir:     framework_tests_dir,
				command: "pnpm",
				args:    []string{"tsgo", "-p", "tsconfig.json", "--noEmit"},
			},
		)
	},
)

var task_framework_prereqs = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_parallel(
			ctx,
			task_build_ts,
			task_install_js_framework,
			task_test_go_framework,
			task_typecheck_ts_framework,
			task_test_ts_framework,
		)
	},
)

var task_test_framework = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_test_framework_prod.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return task_test_framework_dev.Run(ctx, input)
	},
)

var task_test_framework_prod = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_framework_prereqs.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		step := input.app.bombadil_step("test framework production", "test-prod", input.framework)
		return input.run_step(step)
	},
)

var task_test_framework_dev = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_framework_prereqs.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		step := input.app.bombadil_step("test framework development", "test-dev", input.framework)
		return input.run_step(step)
	},
)

var task_stress = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_parallel(ctx, task_stress_other, task_stress_framework)
	},
)

var task_stress_other = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		return input.run_parallel(ctx, task_stress_root_go, task_stress_docs_go)
	},
)

var task_stress_root_go = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		step := command_step{
			name:    "stress root Go module",
			command: "go",
			args: []string{
				"test",
				"-race",
				fmt.Sprintf("-count=%d", input.stress.intensity),
				"./...",
			},
			log_path: input.maint_log_path("stress-root-go.log"),
		}
		return input.run_step(step)
	},
)

var task_stress_docs_go = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		step := command_step{
			name:    "stress docs Go module",
			dir:     docs_dir,
			command: "go",
			args: []string{
				"test",
				"-race",
				fmt.Sprintf("-count=%d", input.stress.intensity),
				"./...",
			},
			log_path: input.maint_log_path("stress-docs-go.log"),
		}
		return input.run_step(step)
	},
)

var task_stress_framework = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := input.run_parallel(
			ctx,
			task_build_ts,
			task_install_js_framework,
			task_stress_go_framework,
			task_typecheck_ts_framework,
			task_test_ts_framework,
		); err != nil {
			return struct{}{}, err
		}
		if _, err := task_stress_framework_prod.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		return task_stress_framework_dev.Run(ctx, input)
	},
)

var task_stress_go_framework = tasks.NewTask(
	func(_ *tasks.Ctx, input maint_task_input) (struct{}, error) {
		step := command_step{
			name:    "stress framework Go module",
			dir:     framework_tests_dir,
			command: "go",
			args: []string{
				"test",
				"-race",
				fmt.Sprintf("-count=%d", input.stress.intensity),
				"./...",
			},
			log_path: input.maint_log_path("stress-framework-go.log"),
		}
		return input.run_step(step)
	},
)

var task_stress_framework_prod = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_build_ts.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		step := command_step{
			name:    "stress framework production",
			dir:     framework_tests_dir,
			command: "go",
			args: input.app.bombadil_args(
				"test-prod",
				framework_options{intensity: fmt.Sprint(input.stress.intensity)},
			),
			log_path: input.maint_log_path("stress-framework-prod.log"),
		}
		return input.run_step(step)
	},
)

var task_stress_framework_dev = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := task_build_ts.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		step := command_step{
			name:    "stress framework development",
			dir:     framework_tests_dir,
			command: "go",
			args: input.app.bombadil_args(
				"test-dev",
				framework_options{intensity: fmt.Sprint(input.stress.intensity)},
			),
			log_path: input.maint_log_path("stress-framework-dev.log"),
		}
		return input.run_step(step)
	},
)

var task_gate = tasks.NewTask(
	func(ctx *tasks.Ctx, input maint_task_input) (struct{}, error) {
		if _, err := input.run_parallel(ctx, task_verify_repo_shape, task_install_js); err != nil {
			return struct{}{}, err
		}
		if _, err := task_build_ts.Run(ctx, input); err != nil {
			return struct{}{}, err
		}
		if _, err := input.run_parallel(ctx, task_fmt_ts, task_lint_ts, task_typecheck_ts); err != nil {
			return struct{}{}, err
		}
		if _, err := input.run_parallel(ctx, task_test_go, task_test_ts); err != nil {
			return struct{}{}, err
		}
		return task_test_framework.Run(ctx, input)
	},
)

var maint_task_by_id = map[work_id]*tasks.Task[maint_task_input, struct{}]{
	work_install_js:                     task_install_js,
	work_install_js_root:                task_install_js_root,
	work_install_js_npm:                 task_install_js_npm,
	work_install_js_create:              task_install_js_create,
	work_install_js_framework:           task_install_js_framework,
	work_install_js_docs:                task_install_js_docs,
	work_build_ts:                       task_build_ts,
	work_fmt_ts:                         task_fmt_ts,
	work_lint_ts:                        task_lint_ts,
	work_typecheck_ts:                   task_typecheck_ts,
	work_typecheck_ts_other:             task_typecheck_ts_other,
	work_typecheck_ts_framework:         task_typecheck_ts_framework,
	work_typecheck_ts_kit:               task_typecheck_ts_kit,
	work_typecheck_ts_core:              task_typecheck_ts_core,
	work_typecheck_ts_preact:            task_typecheck_ts_preact,
	work_typecheck_ts_react:             task_typecheck_ts_react,
	work_typecheck_ts_solid:             task_typecheck_ts_solid,
	work_typecheck_ts_vite:              task_typecheck_ts_vite,
	work_typecheck_ts_create:            task_typecheck_ts_create,
	work_typecheck_ts_framework_tests:   task_typecheck_ts_framework_tests,
	work_test_ts:                        task_test_ts,
	work_test_ts_other:                  task_test_ts_other,
	work_test_ts_framework:              task_test_ts_framework,
	work_test_go:                        task_test_go,
	work_test_go_other:                  task_test_go_other,
	work_test_root_go:                   task_test_root_go,
	work_test_internal_go:               task_test_internal_go,
	work_test_kit:                       task_test_kit,
	work_test_lab:                       task_test_lab,
	work_test_docs:                      task_test_docs,
	work_test_go_framework:              task_test_go_framework,
	work_typecheck_framework_fixture_ts: task_typecheck_framework_fixture_ts,
	work_framework_prereqs:              task_framework_prereqs,
	work_test_framework:                 task_test_framework,
	work_test_framework_prod:            task_test_framework_prod,
	work_test_framework_dev:             task_test_framework_dev,
	work_stress:                         task_stress,
	work_stress_other:                   task_stress_other,
	work_stress_root_go:                 task_stress_root_go,
	work_stress_docs_go:                 task_stress_docs_go,
	work_stress_go_framework:            task_stress_go_framework,
	work_stress_framework:               task_stress_framework,
	work_stress_framework_prod:          task_stress_framework_prod,
	work_stress_framework_dev:           task_stress_framework_dev,
	work_gate:                           task_gate,
	work_verify_repo_shape:              task_verify_repo_shape,
}

func (app maint_app) run_work(id work_id) error {
	return app.run_maint_task(id, maint_task_input{app: app})
}

func (app maint_app) run_framework_work(options framework_options, id work_id) error {
	return app.run_maint_task(id, maint_task_input{app: app, framework: options})
}

func (app maint_app) run_stress_work(options stress_options, id work_id) error {
	return app.run_maint_task(id, maint_task_input{app: app, stress: options})
}

func (app maint_app) run_maint_task(id work_id, input maint_task_input) error {
	task, ok := maint_task_by_id[id]
	if !ok {
		return fmt.Errorf("unknown work item %q", id)
	}

	started_at := time.Now()
	ctx := tasks.NewCtx(context.Background())
	_, err := task.Run(ctx, input)
	duration := time.Since(started_at).Round(time.Millisecond)
	if err != nil {
		fmt.Printf("%s failed (%s)\n", id, duration)
		return err
	}
	if app.dry_run {
		fmt.Printf("%s dry run complete (%s)\n", id, duration)
		return nil
	}
	fmt.Printf("%s passed (%s)\n", id, duration)
	return nil
}

func (input maint_task_input) run_parallel(
	ctx *tasks.Ctx,
	task_list ...*tasks.Task[maint_task_input, struct{}],
) (struct{}, error) {
	bound := make([]tasks.BoundTask, 0, len(task_list))
	for _, task := range task_list {
		bound = append(bound, task.Bind(input))
	}
	return struct{}{}, ctx.RunParallel(bound...)
}

func (input maint_task_input) run_npm_typecheck(
	ctx *tasks.Ctx,
	name, project string,
) (struct{}, error) {
	if _, err := task_install_js_npm.Run(ctx, input); err != nil {
		return struct{}{}, err
	}
	return input.run_step(
		command_step{
			name:    name,
			dir:     npm_dir,
			command: "pnpm",
			args:    []string{"tsgo", "--noEmit", "--project", project},
		},
	)
}

func (input maint_task_input) run_action(name string, action func() error) (struct{}, error) {
	started_at := time.Now()
	input.print_task_start(name, "", "")
	if !input.app.dry_run {
		if err := action(); err != nil {
			input.print_task_finish("failed", name, started_at)
			return struct{}{}, fmt.Errorf("%s failed: %w", name, err)
		}
	}
	input.print_task_finish("passed", name, started_at)
	return struct{}{}, nil
}

func (input maint_task_input) run_step(step command_step) (struct{}, error) {
	started_at := time.Now()
	detail := ""
	if input.app.verbose || input.app.dry_run {
		step_dir := input.app.step_dir(step)
		detail = input.app.format_step(step, step_dir)
	}
	input.print_task_start(step.name, step.log_path, detail)
	if input.app.dry_run {
		input.print_task_finish("passed", step.name, started_at)
		return struct{}{}, nil
	}

	var err error
	if step.log_path != "" {
		err = input.app.execute_logged_step(step, input.app.step_dir(step), step.log_path)
	} else {
		err = input.app.execute_step(step, input.app.step_dir(step))
	}
	if err != nil {
		input.print_task_finish("failed", step.name, started_at)
		if step.log_path != "" {
			return struct{}{}, err
		}
		return struct{}{}, input.app.command_error(step.name, err)
	}

	input.print_task_finish("passed", step.name, started_at)
	return struct{}{}, nil
}

func (input maint_task_input) print_task_start(
	name string,
	log_path string,
	detail string,
) {
	maint_output_mu.Lock()
	defer maint_output_mu.Unlock()
	fmt.Println("starting " + name)
	if log_path != "" {
		fmt.Println("  log: " + log_path)
	}
	if detail != "" {
		fmt.Println("  " + detail)
	}
}

func (input maint_task_input) print_task_finish(
	status string,
	name string,
	started_at time.Time,
) {
	maint_output_mu.Lock()
	defer maint_output_mu.Unlock()
	fmt.Printf("%s %s (%s)\n", status, name, time.Since(started_at).Round(time.Millisecond))
}

func (input maint_task_input) maint_log_path(filename string) string {
	return filepath.Join(input.app.root, local_output_dir, "logs", "maint", filename)
}

func (app maint_app) bombadil_step(
	name string,
	mode string,
	options framework_options,
) command_step {
	return command_step{
		name:    name,
		dir:     framework_tests_dir,
		command: "go",
		args:    app.bombadil_args(mode, options),
	}
}

func (app maint_app) bombadil_args(
	mode string,
	options framework_options,
) []string {
	args := []string{"run", "./cmd/bombadil", mode}
	if options.variant != "" {
		args = append(args, "-variant", options.variant)
	}
	if options.intensity != "" {
		args = append(args, "-intensity", options.intensity)
	}
	return args
}

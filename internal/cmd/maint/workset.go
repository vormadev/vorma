package main

import (
	"fmt"
	"path/filepath"
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

type work_item struct {
	id       work_id
	deps     []work_id
	covers   []work_id
	step     command_step
	action   func() error
	log_path string
}

type work_catalog struct {
	items map[work_id]work_item
	order []work_id
}

type work_plan []work_item

func (app maint_app) run_work(requests ...work_id) error {
	plan, err := app.work_catalog(framework_options{}, stress_options{}).resolve(requests...)
	if err != nil {
		return err
	}
	return app.run_plan(plan)
}

func (app maint_app) run_framework_work(options framework_options, requests ...work_id) error {
	plan, err := app.work_catalog(options, stress_options{}).resolve(requests...)
	if err != nil {
		return err
	}
	return app.run_plan(plan)
}

func (app maint_app) run_stress_work(options stress_options, requests ...work_id) error {
	plan, err := app.work_catalog(framework_options{}, options).resolve(requests...)
	if err != nil {
		return err
	}
	return app.run_plan(plan)
}

func (app maint_app) run_plan(plan work_plan) error {
	for _, item := range plan {
		if item.step.command == "" {
			if item.action != nil {
				fmt.Println("==>", item.step.name)
				if !app.dry_run {
					if err := item.action(); err != nil {
						return fmt.Errorf("%s failed: %w", item.step.name, err)
					}
				}
			}
			continue
		}
		if item.log_path != "" {
			if err := app.run_logged_step(item.step, item.log_path); err != nil {
				return err
			}
			continue
		}
		if err := app.run_step(item.step); err != nil {
			return err
		}
	}
	return nil
}

func (app maint_app) work_catalog(framework framework_options, stress stress_options) work_catalog {
	items := map[work_id]work_item{
		work_verify_repo_shape: {
			id: work_verify_repo_shape,
			step: command_step{
				name: "verify repo maintenance shape",
			},
			action: app.check_repo_shape,
		},
		work_install_js: {
			id: work_install_js,
			deps: []work_id{
				work_install_js_root,
				work_install_js_npm,
				work_install_js_create,
				work_install_js_framework,
				work_install_js_docs,
			},
		},
		work_install_js_root: {
			id: work_install_js_root,
			step: command_step{
				name:    "install root JavaScript dependencies",
				command: "pnpm",
				args:    []string{"i"},
			},
		},
		work_install_js_npm: {
			id: work_install_js_npm,
			step: command_step{
				name:    "install npm package dependencies",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"i"},
			},
		},
		work_install_js_create: {
			id: work_install_js_create,
			step: command_step{
				name:    "install create-vorma dependencies",
				dir:     create_npm_dir,
				command: "pnpm",
				args:    []string{"i"},
			},
		},
		work_install_js_framework: {
			id: work_install_js_framework,
			step: command_step{
				name:    "install framework test dependencies",
				dir:     framework_tests_dir,
				command: "pnpm",
				args:    []string{"i"},
			},
		},
		work_install_js_docs: {
			id: work_install_js_docs,
			step: command_step{
				name:    "install docs dependencies",
				dir:     docs_dir,
				command: "pnpm",
				args:    []string{"i"},
			},
		},
		work_build_ts: {
			id: work_build_ts,
			step: command_step{
				name:    "build TypeScript packages",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsdown"},
			},
		},
		work_fmt_ts: {
			id:   work_fmt_ts,
			step: command_step{name: "format TypeScript", command: "pnpm", args: []string{"oxfmt"}},
		},
		work_lint_ts: {
			id:   work_lint_ts,
			step: command_step{name: "lint TypeScript", command: "pnpm", args: []string{"oxlint"}},
		},
		work_typecheck_ts: {
			id:   work_typecheck_ts,
			deps: []work_id{work_typecheck_ts_other, work_typecheck_ts_framework},
		},
		work_typecheck_ts_other: {
			id: work_typecheck_ts_other,
			deps: []work_id{
				work_typecheck_ts_kit,
				work_typecheck_ts_create,
			},
		},
		work_typecheck_ts_framework: {
			id: work_typecheck_ts_framework,
			deps: []work_id{
				work_typecheck_ts_core,
				work_typecheck_ts_preact,
				work_typecheck_ts_react,
				work_typecheck_ts_solid,
				work_typecheck_ts_vite,
				work_typecheck_ts_framework_tests,
				work_typecheck_framework_fixture_ts,
			},
		},
		work_typecheck_ts_kit: {
			id: work_typecheck_ts_kit,
			step: command_step{
				name:    "typecheck kit",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsgo", "--noEmit", "--project", "./kit"},
			},
		},
		work_typecheck_ts_core: {
			id: work_typecheck_ts_core,
			step: command_step{
				name:    "typecheck framework core",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsgo", "--noEmit", "--project", "./vorma/core"},
			},
		},
		work_typecheck_ts_preact: {
			id: work_typecheck_ts_preact,
			step: command_step{
				name:    "typecheck framework Preact adapter",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsgo", "--noEmit", "--project", "./vorma/tsx/preact"},
			},
		},
		work_typecheck_ts_react: {
			id: work_typecheck_ts_react,
			step: command_step{
				name:    "typecheck framework React adapter",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsgo", "--noEmit", "--project", "./vorma/tsx/react"},
			},
		},
		work_typecheck_ts_solid: {
			id: work_typecheck_ts_solid,
			step: command_step{
				name:    "typecheck framework Solid adapter",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsgo", "--noEmit", "--project", "./vorma/tsx/solid"},
			},
		},
		work_typecheck_ts_vite: {
			id: work_typecheck_ts_vite,
			step: command_step{
				name:    "typecheck framework Vite integration",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsgo", "--noEmit", "--project", "./vorma/vite"},
			},
		},
		work_typecheck_ts_create: {
			id: work_typecheck_ts_create,
			step: command_step{
				name:    "typecheck create-vorma",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsgo", "--noEmit", "--project", "./vorma/create"},
			},
		},
		work_typecheck_ts_framework_tests: {
			id: work_typecheck_ts_framework_tests,
			step: command_step{
				name:    "typecheck framework TypeScript tests",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"tsgo", "-p", "./vorma/tests/tsconfig.json", "--noEmit"},
			},
		},
		work_test_ts: {
			id:   work_test_ts,
			deps: []work_id{work_test_ts_other, work_test_ts_framework},
		},
		work_test_ts_other: {
			id: work_test_ts_other,
			step: command_step{
				name:    "test non-framework TypeScript packages",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"vitest", "run", "--reporter=dot", "--exclude", "vorma/**"},
			},
		},
		work_test_ts_framework: {
			id: work_test_ts_framework,
			step: command_step{
				name:    "test framework TypeScript packages",
				dir:     npm_dir,
				command: "pnpm",
				args:    []string{"vitest", "run", "--reporter=dot", "vorma"},
			},
		},
		work_test_go: {
			id:   work_test_go,
			deps: []work_id{work_test_go_other, work_test_go_framework},
		},
		work_test_go_other: {
			id: work_test_go_other,
			deps: []work_id{
				work_test_root_go,
				work_test_docs,
			},
		},
		work_test_root_go: {
			id: work_test_root_go,
			covers: []work_id{
				work_test_internal_go,
				work_test_kit,
				work_test_lab,
			},
			step: command_step{
				name:    "test root Go module",
				command: "go",
				args:    []string{"test", "-race", "./..."},
			},
		},
		work_test_internal_go: {
			id: work_test_internal_go,
			step: command_step{
				name:    "test internal Go packages",
				command: "go",
				args:    []string{"test", "-race", "./internal/..."},
			},
		},
		work_test_kit: {
			id:     work_test_kit,
			covers: []work_id{work_test_lab},
			step: command_step{
				name:    "test kit Go packages",
				command: "go",
				args:    []string{"test", "-race", "./kit/..."},
			},
		},
		work_test_lab: {
			id: work_test_lab,
			step: command_step{
				name:    "test lab Go packages",
				command: "go",
				args:    []string{"test", "-race", "./kit/lab/..."},
			},
		},
		work_test_docs: {
			id: work_test_docs,
			step: command_step{
				name:    "test docs Go module",
				dir:     docs_dir,
				command: "go",
				args:    []string{"test", "-race", "./..."},
			},
		},
		work_test_go_framework: {
			id: work_test_go_framework,
			step: command_step{
				name:    "test framework Go module",
				dir:     framework_tests_dir,
				command: "go",
				args:    []string{"test", "-race", "./...", "-count=1"},
			},
		},
		work_typecheck_framework_fixture_ts: {
			id: work_typecheck_framework_fixture_ts,
			step: command_step{
				name:    "typecheck framework fixture TypeScript",
				dir:     framework_tests_dir,
				command: "pnpm",
				args:    []string{"tsgo", "-p", "tsconfig.json", "--noEmit"},
			},
		},
		work_test_framework: {
			id:   work_test_framework,
			deps: []work_id{work_test_framework_prod, work_test_framework_dev},
		},
		work_test_framework_prod: {
			id: work_test_framework_prod,
			deps: []work_id{
				work_install_js_framework,
				work_test_go_framework,
				work_typecheck_ts_framework,
				work_test_ts_framework,
			},
			step: app.bombadil_step("test framework production", "test-prod", framework),
		},
		work_test_framework_dev: {
			id: work_test_framework_dev,
			deps: []work_id{
				work_install_js_framework,
				work_test_go_framework,
				work_typecheck_ts_framework,
				work_test_ts_framework,
			},
			step: app.bombadil_step("test framework development", "test-dev", framework),
		},
		work_stress: {
			id:   work_stress,
			deps: []work_id{work_stress_other, work_stress_framework},
		},
		work_stress_other: {
			id:   work_stress_other,
			deps: []work_id{work_stress_root_go, work_stress_docs_go},
		},
		work_stress_root_go: {
			id: work_stress_root_go,
			step: command_step{
				name:    "stress root Go module",
				command: "go",
				args: []string{
					"test",
					"-race",
					fmt.Sprintf("-count=%d", stress.intensity),
					"./...",
				},
			},
			log_path: filepath.Join(
				app.root,
				local_output_dir,
				"logs",
				"maint",
				"stress-root-go.log",
			),
		},
		work_stress_docs_go: {
			id: work_stress_docs_go,
			step: command_step{
				name:    "stress docs Go module",
				dir:     docs_dir,
				command: "go",
				args: []string{
					"test",
					"-race",
					fmt.Sprintf("-count=%d", stress.intensity),
					"./...",
				},
			},
			log_path: filepath.Join(
				app.root,
				local_output_dir,
				"logs",
				"maint",
				"stress-docs-go.log",
			),
		},
		work_stress_framework: {
			id: work_stress_framework,
			deps: []work_id{
				work_install_js_framework,
				work_stress_go_framework,
				work_typecheck_ts_framework,
				work_test_ts_framework,
				work_stress_framework_prod,
				work_stress_framework_dev,
			},
		},
		work_stress_go_framework: {
			id: work_stress_go_framework,
			step: command_step{
				name:    "stress framework Go module",
				dir:     framework_tests_dir,
				command: "go",
				args: []string{
					"test",
					"-race",
					fmt.Sprintf("-count=%d", stress.intensity),
					"./...",
				},
			},
			log_path: filepath.Join(
				app.root,
				local_output_dir,
				"logs",
				"maint",
				"stress-framework-go.log",
			),
		},
		work_stress_framework_prod: {
			id: work_stress_framework_prod,
			step: app.bombadil_step(
				"stress framework production",
				"test-prod",
				framework_options{intensity: fmt.Sprint(stress.intensity)},
			),
			log_path: filepath.Join(
				app.root,
				local_output_dir,
				"logs",
				"maint",
				"stress-framework-prod.log",
			),
		},
		work_stress_framework_dev: {
			id: work_stress_framework_dev,
			step: app.bombadil_step(
				"stress framework development",
				"test-dev",
				framework_options{intensity: fmt.Sprint(stress.intensity)},
			),
			log_path: filepath.Join(
				app.root,
				local_output_dir,
				"logs",
				"maint",
				"stress-framework-dev.log",
			),
		},
		work_gate: {
			id: work_gate,
			deps: []work_id{
				work_install_js,
				work_verify_repo_shape,
				work_build_ts,
				work_fmt_ts,
				work_lint_ts,
				work_typecheck_ts,
				work_test_go,
				work_test_ts,
				work_test_framework,
			},
		},
	}

	return work_catalog{
		items: items,
		order: []work_id{
			work_verify_repo_shape,
			work_install_js_root,
			work_install_js_npm,
			work_install_js_create,
			work_install_js_framework,
			work_install_js_docs,
			work_build_ts,
			work_fmt_ts,
			work_lint_ts,
			work_typecheck_ts_kit,
			work_typecheck_ts_core,
			work_typecheck_ts_preact,
			work_typecheck_ts_react,
			work_typecheck_ts_solid,
			work_typecheck_ts_vite,
			work_typecheck_ts_create,
			work_typecheck_ts_framework_tests,
			work_test_root_go,
			work_test_internal_go,
			work_test_kit,
			work_test_lab,
			work_test_docs,
			work_test_go_framework,
			work_test_ts_other,
			work_test_ts_framework,
			work_typecheck_framework_fixture_ts,
			work_test_framework_prod,
			work_test_framework_dev,
			work_stress_root_go,
			work_stress_docs_go,
			work_stress_go_framework,
			work_stress_framework_prod,
			work_stress_framework_dev,
		},
	}
}

func (catalog work_catalog) resolve(requests ...work_id) (work_plan, error) {
	selected := map[work_id]bool{}
	visiting := map[work_id]bool{}

	for _, request := range requests {
		if err := catalog.select_work(request, selected, visiting); err != nil {
			return nil, err
		}
	}
	catalog.apply_coverage(selected)

	plan := work_plan{}
	for _, id := range catalog.order {
		if selected[id] {
			plan = append(plan, catalog.items[id])
		}
	}
	return plan, nil
}

func (catalog work_catalog) select_work(
	id work_id,
	selected map[work_id]bool,
	visiting map[work_id]bool,
) error {
	item, ok := catalog.items[id]
	if !ok {
		return fmt.Errorf("unknown work item %q", id)
	}
	if selected[id] {
		return nil
	}
	if visiting[id] {
		return fmt.Errorf("work dependency cycle at %q", id)
	}

	visiting[id] = true
	for _, dep := range item.deps {
		if err := catalog.select_work(dep, selected, visiting); err != nil {
			return err
		}
	}
	visiting[id] = false
	selected[id] = true
	return nil
}

func (catalog work_catalog) apply_coverage(selected map[work_id]bool) {
	for id := range selected {
		item := catalog.items[id]
		for _, covered := range item.covers {
			delete(selected, covered)
		}
	}
}

func (app maint_app) bombadil_step(
	name string,
	mode string,
	options framework_options,
) command_step {
	args := []string{"run", "./cmd/bombadil", mode}
	if options.variant != "" {
		args = append(args, "-variant", options.variant)
	}
	if options.intensity != "" {
		args = append(args, "-intensity", options.intensity)
	}
	return command_step{name: name, dir: framework_tests_dir, command: "go", args: args}
}

package main

import (
	"flag"
)

type framework_options struct {
	variant   string
	intensity string
}

func (app maint_app) test_framework(args []string) error {
	options, err := app.parse_framework_options("test-framework", args)
	if err != nil {
		return err
	}
	return app.run_framework_work(options, work_test_framework)
}

func (app maint_app) test_framework_prod(args []string) error {
	options, err := app.parse_framework_options("test-framework-prod", args)
	if err != nil {
		return err
	}
	return app.run_framework_work(options, work_test_framework_prod)
}

func (app maint_app) test_framework_dev(args []string) error {
	options, err := app.parse_framework_options("test-framework-dev", args)
	if err != nil {
		return err
	}
	return app.run_framework_work(options, work_test_framework_dev)
}

func (app maint_app) stress_framework(args []string) error {
	options, err := app.parse_stress_options("stress-framework", args)
	if err != nil {
		return err
	}
	return app.run_stress_work(options, work_stress_framework)
}

func (app maint_app) build_framework() error {
	return app.run_step(
		command_step{
			name:    "build framework fixture",
			dir:     framework_tests_dir,
			command: "go",
			args:    []string{"run", "./cmd/bombadil", "build"},
		},
	)
}

func (app maint_app) serve_framework_dev(args []string) error {
	flags := flag.NewFlagSet("serve-framework-dev", flag.ContinueOnError)
	variant := flags.String("variant", "react", "variant to serve")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return app.run_step(
		command_step{
			name:    "serve framework development fixture",
			dir:     framework_tests_dir,
			command: "go",
			args:    []string{"run", "./cmd/bombadil", "serve-dev", *variant},
		},
	)
}

func (app maint_app) inspect_framework(args []string) error {
	flags := flag.NewFlagSet("inspect-framework-artifacts", flag.ContinueOnError)
	artifact := flags.String("artifact", "", "artifact path to inspect")
	if err := flags.Parse(args); err != nil {
		return err
	}

	bombadil_args := []string{"run", "./cmd/bombadil", "inspect"}
	if *artifact != "" {
		bombadil_args = append(bombadil_args, *artifact)
	}
	return app.run_step(
		command_step{
			name:    "inspect framework artifact",
			dir:     framework_tests_dir,
			command: "go",
			args:    bombadil_args,
		},
	)
}

func (app maint_app) clean_framework() error {
	if err := app.confirm("Remove framework artifacts?"); err != nil {
		return err
	}
	return app.run_step(
		command_step{
			name:    "remove framework artifacts",
			dir:     framework_tests_dir,
			command: "rm",
			args:    []string{"-rf", ".bombadil"},
		},
	)
}

func (app maint_app) parse_framework_options(
	command string,
	args []string,
) (framework_options, error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	variant := flags.String("variant", "", "optional variant")
	intensity := flags.String("intensity", "", "optional intensity")
	if err := flags.Parse(args); err != nil {
		return framework_options{}, err
	}
	return framework_options{
		variant:   *variant,
		intensity: *intensity,
	}, nil
}

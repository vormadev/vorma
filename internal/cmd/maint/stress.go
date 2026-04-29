package main

import (
	"flag"
	"fmt"
)

type stress_options struct {
	intensity int
}

func (app maint_app) stress(args []string) error {
	options, err := app.parse_stress_options("stress", args)
	if err != nil {
		return err
	}
	return app.run_stress_work(options, work_stress)
}

func (app maint_app) stress_other(args []string) error {
	options, err := app.parse_stress_options("stress-other", args)
	if err != nil {
		return err
	}
	return app.run_stress_work(options, work_stress_other)
}

func (app maint_app) parse_stress_options(command string, args []string) (stress_options, error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	intensity := flags.Int("intensity", 0, "positive stress intensity")
	if err := flags.Parse(args); err != nil {
		return stress_options{}, err
	}
	if *intensity <= 0 {
		return stress_options{}, fmt.Errorf(
			"%s requires --intensity with a positive integer",
			command,
		)
	}
	return stress_options{intensity: *intensity}, nil
}

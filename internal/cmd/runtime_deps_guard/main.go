package main

import (
	"fmt"
	"os"

	"github.com/vormadev/vorma/internal/runtimeguard"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(
			os.Stderr,
			"runtime dependency guard does not take arguments",
		)
		os.Exit(2)
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	report, evaluateError := runtimeguard.EvaluateDefaultRuntimeDependencyBoundaries()
	if evaluateError != nil {
		return evaluateError
	}

	if !report.HasViolations() {
		fmt.Println("runtime dependency guard: PASS")
		return nil
	}

	return fmt.Errorf(
		"runtime dependency guard: FAIL\n%s",
		report.FormatViolationReport(),
	)
}

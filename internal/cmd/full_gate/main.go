package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

var releaseGateTargetNames = []string{
	"gotest",
	"tsreset",
	"tstest-source",
	"tslint",
	"tscheck",
	"npmbuild",
	"tstest-dist",
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "release gate command does not take arguments")
		os.Exit(2)
	}

	for _, targetName := range releaseGateTargetNames {
		executionErr := runMakeTarget(targetName)
		if executionErr != nil {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "RELEASE GATE: FAIL")
			fmt.Fprintf(os.Stderr, "failed target: %s\n", targetName)
			os.Exit(exitCodeFromCommandError(executionErr))
		}
	}

	fmt.Println()
	fmt.Println("RELEASE GATE: PASS")
}

func runMakeTarget(targetName string) error {
	command := exec.Command("make", targetName)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func exitCodeFromCommandError(commandErr error) int {
	var exitErr *exec.ExitError
	if errors.As(commandErr, &exitErr) {
		commandExitCode := exitErr.ExitCode()
		if commandExitCode != 0 {
			return commandExitCode
		}
	}
	return 1
}

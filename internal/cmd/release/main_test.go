package main

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

const (
	releaseMainTestHelperProcessEnv = "RELEASE_MAIN_TEST_HELPER_PROCESS"
	releaseMainTestHelperModeEnv    = "RELEASE_MAIN_TEST_HELPER_MODE"
)

func TestReleaseMainHelperProcess(t *testing.T) {
	if os.Getenv(releaseMainTestHelperProcessEnv) != "1" {
		return
	}

	if os.Getenv(releaseMainTestHelperModeEnv) != "invalid_args" {
		os.Exit(2)
	}

	os.Args = []string{"release", "unexpected"}
	main()
	os.Exit(0)
}

func TestMain_ExitsNonZeroWhenArgumentsAreProvided(t *testing.T) {
	command := exec.Command(
		os.Args[0],
		"-test.run=^TestReleaseMainHelperProcess$",
	)
	command.Env = append(
		os.Environ(),
		releaseMainTestHelperProcessEnv+"=1",
		releaseMainTestHelperModeEnv+"=invalid_args",
	)

	err := command.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for invalid args")
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected exec.ExitError, got %T", err)
	}
	if exitErr.ExitCode() == 0 {
		t.Fatalf("expected non-zero exit code, got %d", exitErr.ExitCode())
	}
}

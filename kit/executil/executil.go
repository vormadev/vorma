package executil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var (
	ErrCommandExecutionTimedOut = errors.New("command execution timed out")
	ErrCommandExecutionCanceled = errors.New("command execution canceled")
)

func MakeCmdRunner(commands ...string) func() error {
	return func() error {
		if len(commands) == 0 {
			return fmt.Errorf("no commands provided")
		}
		cmd := exec.Command(commands[0], commands[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
}

func GetExecutableDir() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("error getting executable path: %w", err)
	}
	return filepath.Dir(execPath), nil
}

func RunCmd(commands ...string) error {
	return MakeCmdRunner(commands...)()
}

func RunCmdCapture(commands ...string) (string, error) {
	if len(commands) == 0 {
		return "", fmt.Errorf("no commands provided")
	}
	cmd := exec.Command(commands[0], commands[1:]...)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return output, err
	}
	return output, nil
}

func RunShell(command string) error {
	return RunShellWithContext(context.Background(), command)
}

func RunShellWithContext(
	commandExecutionContext context.Context,
	command string,
) error {
	if commandExecutionContext == nil {
		commandExecutionContext = context.Background()
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(commandExecutionContext, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(commandExecutionContext, "sh", "-c", command)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runCommandError := cmd.Run()
	if runCommandError == nil {
		return nil
	}

	if commandExecutionContext == nil {
		return runCommandError
	}

	commandExecutionContextError := commandExecutionContext.Err()
	if errors.Is(commandExecutionContextError, context.DeadlineExceeded) {
		return fmt.Errorf(
			"%w: %w",
			ErrCommandExecutionTimedOut,
			runCommandError,
		)
	}
	if errors.Is(commandExecutionContextError, context.Canceled) {
		return fmt.Errorf(
			"%w: %w",
			ErrCommandExecutionCanceled,
			runCommandError,
		)
	}

	return runCommandError
}

package executil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	return cmd.Run()
}

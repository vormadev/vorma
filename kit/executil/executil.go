package executil

import (
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

func RunShell(command string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

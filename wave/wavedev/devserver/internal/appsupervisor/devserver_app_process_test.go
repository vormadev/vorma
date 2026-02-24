package appsupervisor

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestStopApp_ReturnsProcessTerminationErrors(t *testing.T) {
	manager := NewAppProcessManager()
	manager.GracefulStopTimeout = 20 * time.Millisecond

	stopError := manager.StopApp(&exec.Cmd{Process: &os.Process{Pid: 99999999}})
	if stopError == nil {
		t.Fatal("expected StopApp to return process termination error")
	}
}

func TestAppProcessManager_StopApp_GracefulTimeoutFallsBackToKill(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip("shell signal handling expectations are unix-specific")
	}

	command := exec.Command("sh", "-c", "trap '' INT; sleep 5")
	if startError := command.Start(); startError != nil {
		t.Fatalf("failed to start test process: %v", startError)
	}

	manager := &AppProcessManager{
		GracefulStopTimeout: 20 * time.Millisecond,
	}
	if stopError := manager.StopApp(command); stopError != nil {
		t.Fatalf(
			"expected kill fallback to terminate process without error, got %v",
			stopError,
		)
	}
	if command.ProcessState == nil {
		t.Fatalf(
			"expected process state to be populated after StopApp, got %#v",
			command.ProcessState,
		)
	}
}

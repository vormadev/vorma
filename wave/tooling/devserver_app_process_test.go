package tooling

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestStopApp_ReturnsProcessTerminationErrors(t *testing.T) {
	s := &server{
		log: newDiscardLogger(),
		appCmd: &exec.Cmd{
			Process: &os.Process{Pid: 99999999},
		},
	}

	if err := s.stopApp(); err == nil {
		t.Fatal("expected stopApp to return process termination error")
	}
}

func TestAppProcessManager_StopApp_GracefulTimeoutFallsBackToKill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell signal handling expectations are unix-specific")
	}

	cmd := exec.Command("sh", "-c", "trap '' INT; sleep 5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}

	manager := &appProcessManager{
		gracefulStopTimeout: 20 * time.Millisecond,
	}
	if err := manager.stopApp(cmd); err != nil {
		t.Fatalf("expected kill fallback to terminate process without error, got %v", err)
	}
	if cmd.ProcessState == nil {
		t.Fatalf("expected process state to be populated after stopApp, got %#v", cmd.ProcessState)
	}
}

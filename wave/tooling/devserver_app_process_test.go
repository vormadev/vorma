package tooling

import (
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"github.com/vormadev/vorma/wave/tooling/devserver/devserverruntime"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestStopApp_ReturnsProcessTerminationErrors(t *testing.T) {
	s := &devserver.Server{
		Log: newDiscardLogger(),
		AppCmd: &exec.Cmd{
			Process: &os.Process{Pid: 99999999},
		},
	}

	if err := s.StopApp(); err == nil {
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

	manager := &devserverruntime.AppProcessManager{
		GracefulStopTimeout: 20 * time.Millisecond,
	}
	if err := manager.StopApp(cmd); err != nil {
		t.Fatalf("expected kill fallback to terminate process without error, got %v", err)
	}
	if cmd.ProcessState == nil {
		t.Fatalf("expected process state to be populated after stopApp, got %#v", cmd.ProcessState)
	}
}

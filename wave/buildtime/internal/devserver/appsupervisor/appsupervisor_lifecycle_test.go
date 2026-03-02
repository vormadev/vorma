package appsupervisor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAppProcessManager_StartAppAndStopCurrentApp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script process lifecycle test is unix-specific")
	}

	manager := NewAppProcessManager()
	manager.GracefulStopTimeout = 100 * time.Millisecond

	scriptPath := writeLongRunningTestScript(t)
	command, startError := manager.StartApp(scriptPath)
	if startError != nil {
		t.Fatalf("StartApp returned error: %v", startError)
	}
	if command == nil || command.Process == nil {
		t.Fatalf("StartApp returned invalid command: %#v", command)
	}

	currentCommand := manager.CurrentCommand()
	if currentCommand == nil || currentCommand.Process == nil {
		t.Fatalf("CurrentCommand returned invalid command: %#v", currentCommand)
	}
	if currentCommand.Process.Pid != command.Process.Pid {
		t.Fatalf(
			"CurrentCommand pid=%d, expected started pid=%d",
			currentCommand.Process.Pid,
			command.Process.Pid,
		)
	}

	if stopError := manager.StopCurrentApp(); stopError != nil {
		t.Fatalf("StopCurrentApp returned error: %v", stopError)
	}
	if manager.CurrentCommand() != nil {
		t.Fatalf(
			"CurrentCommand after StopCurrentApp=%#v, expected nil",
			manager.CurrentCommand(),
		)
	}
	if manager.waitDone != nil || manager.waitRunning {
		t.Fatalf(
			"manager wait state was not cleared: waitDone=%v waitRunning=%v",
			manager.waitDone,
			manager.waitRunning,
		)
	}

	// StopCurrentApp should be idempotent after process has already been cleared.
	if stopError := manager.StopCurrentApp(); stopError != nil {
		t.Fatalf("second StopCurrentApp returned error: %v", stopError)
	}
}

func TestAppProcessManager_StartApp_ReplacesRunningCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script process lifecycle test is unix-specific")
	}

	manager := NewAppProcessManager()
	manager.GracefulStopTimeout = 100 * time.Millisecond
	scriptPath := writeLongRunningTestScript(t)

	firstCommand, firstStartError := manager.StartApp(scriptPath)
	if firstStartError != nil {
		t.Fatalf("first StartApp returned error: %v", firstStartError)
	}
	if firstCommand == nil || firstCommand.Process == nil {
		t.Fatalf("first StartApp returned invalid command: %#v", firstCommand)
	}
	defer firstCommand.Process.Kill()

	secondCommand, secondStartError := manager.StartApp(scriptPath)
	if secondStartError != nil {
		t.Fatalf("second StartApp returned error: %v", secondStartError)
	}
	if secondCommand == nil || secondCommand.Process == nil {
		t.Fatalf("second StartApp returned invalid command: %#v", secondCommand)
	}
	defer secondCommand.Process.Kill()

	currentCommand := manager.CurrentCommand()
	if currentCommand == nil || currentCommand.Process == nil {
		t.Fatalf("CurrentCommand returned invalid command: %#v", currentCommand)
	}
	if currentCommand.Process.Pid != secondCommand.Process.Pid {
		t.Fatalf(
			"CurrentCommand pid=%d, expected second pid=%d",
			currentCommand.Process.Pid,
			secondCommand.Process.Pid,
		)
	}

	if stopError := manager.StopCurrentApp(); stopError != nil {
		t.Fatalf("StopCurrentApp returned error: %v", stopError)
	}
}

func TestAppProcessManager_StartApp_ValidatesInput(t *testing.T) {
	manager := NewAppProcessManager()

	if _, startError := manager.StartApp(" "); startError == nil {
		t.Fatal("expected StartApp to reject blank binary path")
	}

	var nilManager *AppProcessManager
	if _, startError := nilManager.StartApp("/bin/sh"); startError == nil {
		t.Fatal("expected nil process manager StartApp to fail")
	}
}

func TestAppProcessManager_StopCurrentApp_DoesNotReturnDoubleWaitError(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script process lifecycle test is unix-specific")
	}

	manager := NewAppProcessManager()
	manager.GracefulStopTimeout = 100 * time.Millisecond
	scriptPath := writeLongRunningTestScript(t)

	for iteration := 0; iteration < 20; iteration++ {
		_, startError := manager.StartApp(scriptPath)
		if startError != nil {
			t.Fatalf(
				"StartApp iteration %d returned error: %v",
				iteration,
				startError,
			)
		}

		stopError := manager.StopCurrentApp()
		if stopError != nil &&
			strings.Contains(stopError.Error(), "Wait was already called") {
			t.Fatalf(
				"StopCurrentApp iteration %d returned double-wait error: %v",
				iteration,
				stopError,
			)
		}
		if stopError != nil {
			t.Fatalf(
				"StopCurrentApp iteration %d returned error: %v",
				iteration,
				stopError,
			)
		}
	}
}

func writeLongRunningTestScript(t *testing.T) string {
	t.Helper()

	scriptPath := filepath.Join(t.TempDir(), "appsupervisor_lifecycle.sh")
	scriptSource := "#!/bin/sh\ntrap 'exit 0' TERM INT\nwhile :; do sleep 1; done\n"
	if writeError := os.WriteFile(scriptPath, []byte(scriptSource), 0o755); writeError != nil {
		t.Fatalf("write test script: %v", writeError)
	}
	return scriptPath
}

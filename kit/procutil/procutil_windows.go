//go:build windows

package procutil

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// SysProcAttr returns platform-specific process attributes that
// place the new process in its own process group, enabling
// group-level signal delivery via RequestStop and ForceKill.
func SysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// IsAlive reports whether the process with the given PID exists
// and is running.
func IsAlive(pid int) bool {
	out, err := exec.Command(
		"tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH",
	).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), strconv.Itoa(pid))
}

// RequestStop sends a graceful shutdown signal to the process and
// its children. On Windows this uses taskkill without /F, which
// sends WM_CLOSE to GUI apps. Console apps may not respond.
func RequestStop(pid int) {
	_ = exec.Command("taskkill", "/T", "/PID", strconv.Itoa(pid)).Run()
}

// ForceKill immediately terminates the process and its children.
func ForceKill(pid int) {
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}

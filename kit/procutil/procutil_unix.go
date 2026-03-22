//go:build !windows

package procutil

import "syscall"

// SysProcAttr returns platform-specific process attributes that
// place the new process in its own process group, enabling
// group-level signal delivery via RequestStop and ForceKill.
func SysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// IsAlive reports whether the process with the given PID exists
// and is running.
func IsAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// RequestStop sends a graceful shutdown signal to the process and
// its children. The process may ignore the signal.
func RequestStop(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGTERM)
}

// ForceKill immediately terminates the process and its children.
func ForceKill(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

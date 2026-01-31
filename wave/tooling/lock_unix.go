//go:build !windows

package tooling

import (
	"os"
	"syscall"
)

// isProcessRunning checks if a process with the given PID is still running.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 doesn't send anything but checks if the process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

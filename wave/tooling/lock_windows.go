//go:build windows

package tooling

import "syscall"

// PROCESS_QUERY_LIMITED_INFORMATION allows querying limited process info.
// Available on Windows Vista and later.
const processQueryLimitedInformation = 0x1000

// isProcessRunning checks if a process with the given PID is still running.
func isProcessRunning(pid int) bool {
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	syscall.CloseHandle(handle)
	return true
}

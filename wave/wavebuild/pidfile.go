package wavebuild

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/vormadev/vorma/kit/procutil"
)

// write_pid_file writes a PID to the given path.
func write_pid_file(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

// remove_pid_file removes a PID file, ignoring not-found errors.
func remove_pid_file(path string) {
	_ = os.Remove(path)
}

// Reads a PID file, checks whether the process is still alive, and
// kills it if so. Removes the PID file afterward no matter what.
// Does nothing if the file does not exist.
func kill_stale_pid(path string, label string, logger *slog.Logger) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // file doesn't exist or unreadable — nothing to do
	}
	defer remove_pid_file(path)

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return // corrupt file — remove and move on
	}

	if !procutil.IsAlive(pid) {
		return // process already dead
	}

	logger.Info(fmt.Sprintf(
		"Found stale %s process (pid %d), killing", label, pid,
	))

	procutil.ForceKill(pid)
}

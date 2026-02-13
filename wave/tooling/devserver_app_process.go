package tooling

import (
	"os"
	"os/exec"
)

func (s *server) startApp() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := exec.Command(s.cfg.Dist.Binary())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		s.log.Error("start app failed", "error", err)
		return
	}

	s.appCmd = cmd
	s.log.Info("Started app", "pid", cmd.Process.Pid)
}

func (s *server) stopApp() error {
	s.mu.Lock()
	cmd := s.appCmd
	s.appCmd = nil
	s.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	s.log.Info("Stopping app", "pid", cmd.Process.Pid)
	cmd.Process.Kill()
	cmd.Wait() // reap zombie, ensure port is released
	return nil
}

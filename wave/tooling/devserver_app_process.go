package tooling

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

const defaultAppProcessGracefulStopTimeout = 2 * time.Second

type appProcessManager struct {
	gracefulStopTimeout time.Duration
}

func newAppProcessManager() *appProcessManager {
	return &appProcessManager{
		gracefulStopTimeout: defaultAppProcessGracefulStopTimeout,
	}
}

func (s *server) ensureAppProcessManager() *appProcessManager {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.appProcessManager == nil {
		s.appProcessManager = newAppProcessManager()
	}
	return s.appProcessManager
}

func (s *server) startApp() {
	manager := s.ensureAppProcessManager()
	if manager == nil {
		return
	}

	cmd, err := manager.startApp(s.cfg.Dist.Binary())
	if err != nil {
		s.log.Error("start app failed", "error", err)
		return
	}

	s.mu.Lock()
	s.appCmd = cmd
	s.mu.Unlock()
	s.log.Info("Started app", "pid", cmd.Process.Pid)
}

func (manager *appProcessManager) startApp(
	binaryPath string,
) (*exec.Cmd, error) {
	cmd := exec.Command(binaryPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func (s *server) stopApp() error {
	manager := s.ensureAppProcessManager()
	if manager == nil {
		return nil
	}

	s.mu.Lock()
	cmd := s.appCmd
	s.appCmd = nil
	s.mu.Unlock()

	return manager.stopApp(cmd)
}

func (manager *appProcessManager) stopApp(
	cmd *exec.Cmd,
) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	processInterruptError := cmd.Process.Signal(os.Interrupt)
	if shouldIgnoreProcessTerminationError(processInterruptError) {
		processInterruptError = nil
	}

	processWaitErrorChannel := make(chan error, 1)
	go func() {
		processWaitErrorChannel <- cmd.Wait()
	}()

	gracefulStopTimeout := manager.gracefulStopTimeout
	if gracefulStopTimeout <= 0 {
		gracefulStopTimeout = defaultAppProcessGracefulStopTimeout
	}

	select {
	case processWaitError := <-processWaitErrorChannel:
		if shouldIgnoreProcessWaitError(processWaitError) {
			processWaitError = nil
		}
		return errors.Join(processInterruptError, processWaitError)
	case <-time.After(gracefulStopTimeout):
		processTerminationError := cmd.Process.Kill()
		if shouldIgnoreProcessTerminationError(processTerminationError) {
			processTerminationError = nil
		}

		processWaitError := <-processWaitErrorChannel
		if shouldIgnoreProcessWaitError(processWaitError) {
			processWaitError = nil
		}

		return errors.Join(processInterruptError, processTerminationError, processWaitError)
	}
}

func shouldIgnoreProcessTerminationError(
	processTerminationError error,
) bool {
	if processTerminationError == nil {
		return false
	}
	if errors.Is(processTerminationError, os.ErrProcessDone) {
		return true
	}
	return strings.Contains(strings.ToLower(processTerminationError.Error()), "process already finished")
}

func shouldIgnoreProcessWaitError(processWaitError error) bool {
	if processWaitError == nil {
		return false
	}

	processExitError, isProcessExitError := processWaitError.(*exec.ExitError)
	if !isProcessExitError || processExitError.ProcessState == nil {
		return false
	}

	processStateDescription := strings.ToLower(processExitError.ProcessState.String())
	return strings.Contains(processStateDescription, "signal: killed") ||
		strings.Contains(processStateDescription, "signal: terminated") ||
		strings.Contains(processStateDescription, "signal: interrupt")
}

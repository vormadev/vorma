package runtimeprocess

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DefaultAppProcessGracefulStopTimeout is the fallback stop timeout for the
// app process.
const DefaultAppProcessGracefulStopTimeout = 2 * time.Second

// AppProcessManager controls app-process start/stop behavior.
type AppProcessManager struct {
	GracefulStopTimeout time.Duration
}

// NewAppProcessManager constructs an AppProcessManager with default timeout.
func NewAppProcessManager() *AppProcessManager {
	return &AppProcessManager{
		GracefulStopTimeout: DefaultAppProcessGracefulStopTimeout,
	}
}

// StartApp starts binaryPath and wires process output to the current process.
func (manager *AppProcessManager) StartApp(
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

// StopApp stops cmd with interrupt-then-kill fallback semantics.
func (manager *AppProcessManager) StopApp(
	cmd *exec.Cmd,
) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	processInterruptError := cmd.Process.Signal(os.Interrupt)
	if ShouldIgnoreProcessTerminationError(processInterruptError) {
		processInterruptError = nil
	}

	processWaitErrorChannel := make(chan error, 1)
	go func() {
		processWaitErrorChannel <- cmd.Wait()
	}()

	gracefulStopTimeout := DefaultAppProcessGracefulStopTimeout
	if manager != nil && manager.GracefulStopTimeout > 0 {
		gracefulStopTimeout = manager.GracefulStopTimeout
	}

	select {
	case processWaitError := <-processWaitErrorChannel:
		if ShouldIgnoreProcessWaitError(processWaitError) {
			processWaitError = nil
		}
		return errors.Join(processInterruptError, processWaitError)
	case <-time.After(gracefulStopTimeout):
		processTerminationError := cmd.Process.Kill()
		if ShouldIgnoreProcessTerminationError(processTerminationError) {
			processTerminationError = nil
		}

		processWaitError := <-processWaitErrorChannel
		if ShouldIgnoreProcessWaitError(processWaitError) {
			processWaitError = nil
		}

		return errors.Join(processInterruptError, processTerminationError, processWaitError)
	}
}

// ShouldIgnoreProcessTerminationError returns true when processTerminationError
// means the process is already gone.
func ShouldIgnoreProcessTerminationError(
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

// ShouldIgnoreProcessWaitError returns true when processWaitError represents an
// expected signal termination.
func ShouldIgnoreProcessWaitError(processWaitError error) bool {
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

// ReadinessWaitPolicy defines how readiness polling is performed.
type ReadinessWaitPolicy struct {
	MaxAttempts    int
	BaseDelay      time.Duration
	MaxTotal       time.Duration
	RequestTimeout time.Duration
}

// LocalReadinessProbeHostIPv4 is the default app readiness probe host.
const LocalReadinessProbeHostIPv4 = "127.0.0.1"

// LocalReadinessProbeHostLocalhost is the alternative readiness probe host.
const LocalReadinessProbeHostLocalhost = "localhost"

// DefaultReadinessWaitPolicy returns the readiness polling defaults.
func DefaultReadinessWaitPolicy() ReadinessWaitPolicy {
	return ReadinessWaitPolicy{
		MaxAttempts:    100,
		BaseDelay:      20 * time.Millisecond,
		MaxTotal:       10 * time.Second,
		RequestTimeout: 500 * time.Millisecond,
	}
}

// ResolveAppReadyURL resolves the app readiness URL for appPort and endpoint.
func ResolveAppReadyURL(appPort int, healthcheckEndpoint string) string {
	return ResolveReadinessProbeURL(
		LocalReadinessProbeHostIPv4,
		appPort,
		healthcheckEndpoint,
	)
}

// ResolveReadinessProbeURL builds a readiness probe URL from host, port, and
// endpoint.
func ResolveReadinessProbeURL(
	host string,
	port int,
	endpoint string,
) string {
	return fmt.Sprintf("http://%s:%d%s", host, port, endpoint)
}

// DeriveReadinessWaitDelay computes attempt backoff delay.
func DeriveReadinessWaitDelay(
	attemptIndex int,
	baseDelay time.Duration,
) time.Duration {
	return baseDelay + time.Duration(attemptIndex)*baseDelay
}

// ShouldContinueReadinessWait reports whether polling should continue.
func ShouldContinueReadinessWait(
	total time.Duration,
	maxTotal time.Duration,
) bool {
	return total <= maxTotal
}

// WaitForReady polls one URL for readiness with policy.
func WaitForReady(url string, policy ReadinessWaitPolicy) bool {
	return WaitForAnyReady([]string{url}, policy)
}

// WaitForAnyReady polls URLs until one returns HTTP 200 or policy is exhausted.
func WaitForAnyReady(
	urls []string,
	policy ReadinessWaitPolicy,
) bool {
	client := &http.Client{Timeout: policy.RequestTimeout}
	var total time.Duration

	uniqueURLs := make([]string, 0, len(urls))
	seenURLs := make(map[string]struct{}, len(urls))
	for _, url := range urls {
		if url == "" {
			continue
		}
		if _, exists := seenURLs[url]; exists {
			continue
		}
		seenURLs[url] = struct{}{}
		uniqueURLs = append(uniqueURLs, url)
	}
	if len(uniqueURLs) == 0 {
		return false
	}

	for attemptIndex := range policy.MaxAttempts {
		for _, url := range uniqueURLs {
			resp, err := client.Get(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return true
			}
			if resp != nil {
				resp.Body.Close()
			}
		}

		delay := DeriveReadinessWaitDelay(attemptIndex, policy.BaseDelay)
		total += delay
		if !ShouldContinueReadinessWait(total, policy.MaxTotal) {
			return false
		}

		time.Sleep(delay)
	}

	return false
}

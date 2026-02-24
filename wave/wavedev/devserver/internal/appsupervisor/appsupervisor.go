package appsupervisor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// DefaultAppProcessGracefulStopTimeout is the default timeout for process stop.
	DefaultAppProcessGracefulStopTimeout = 6 * time.Second
	// LocalReadinessProbeHostIPv4 is the default host used for probe URL generation.
	LocalReadinessProbeHostIPv4 = "127.0.0.1"
	// LocalReadinessProbeHostLocalhost is alternate host used for probe fallback.
	LocalReadinessProbeHostLocalhost = "localhost"
)

// AppProcessManager owns lifecycle operations for the app runtime process.
type AppProcessManager struct {
	mu          sync.Mutex
	appCommand  *exec.Cmd
	waitDone    chan struct{}
	waitRunning bool

	GracefulStopTimeout time.Duration
}

// ReadinessWaitPolicy defines polling behavior for readiness checks.
type ReadinessWaitPolicy struct {
	HTTPClientTimeout          time.Duration
	InitialDelay               time.Duration
	MaximumDelay               time.Duration
	MaximumTotalWait           time.Duration
	TreatHTTPStatusCodeAsReady func(int) bool
}

// DefaultReadinessWaitPolicy returns robust defaults for local dev readiness polling.
func DefaultReadinessWaitPolicy() ReadinessWaitPolicy {
	return ReadinessWaitPolicy{
		HTTPClientTimeout: 900 * time.Millisecond,
		InitialDelay:      50 * time.Millisecond,
		MaximumDelay:      500 * time.Millisecond,
		MaximumTotalWait:  25 * time.Second,
		TreatHTTPStatusCodeAsReady: func(statusCode int) bool {
			return statusCode >= 200 && statusCode < 400
		},
	}
}

// NewAppProcessManager creates a new process manager.
func NewAppProcessManager() *AppProcessManager {
	return &AppProcessManager{
		GracefulStopTimeout: DefaultAppProcessGracefulStopTimeout,
	}
}

// StartApp starts a new app process and records it as current command.
func (processManager *AppProcessManager) StartApp(
	binaryPath string,
) (*exec.Cmd, error) {
	if processManager == nil {
		return nil, errors.New("app process manager is nil")
	}
	if strings.TrimSpace(binaryPath) == "" {
		return nil, errors.New("binary path is empty")
	}

	processManager.mu.Lock()
	defer processManager.mu.Unlock()

	if stopError := processManager.stopCurrentAppLocked(
		processManager.deriveGracefulStopTimeout(),
	); stopError != nil {
		return nil, stopError
	}

	command := exec.Command(binaryPath)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = os.Environ()

	if startError := command.Start(); startError != nil {
		return nil, fmt.Errorf(
			"start app process %q: %w",
			binaryPath,
			startError,
		)
	}

	processManager.appCommand = command
	processManager.waitDone = make(chan struct{})
	processManager.waitRunning = true
	go processManager.waitForCommand(command, processManager.waitDone)

	return command, nil
}

// StopApp stops the provided command when it matches current running app.
func (processManager *AppProcessManager) StopApp(command *exec.Cmd) error {
	if processManager == nil {
		return nil
	}

	processManager.mu.Lock()
	defer processManager.mu.Unlock()

	if processManager.appCommand == nil && command != nil {
		processManager.appCommand = command
	}

	if command != nil && processManager.appCommand != nil &&
		command.Process != nil &&
		processManager.appCommand.Process != nil {
		if command.Process.Pid != processManager.appCommand.Process.Pid {
			return nil
		}
	}

	return processManager.stopCurrentAppLocked(
		processManager.deriveGracefulStopTimeout(),
	)
}

// StopCurrentApp stops the currently tracked app process.
func (processManager *AppProcessManager) StopCurrentApp() error {
	if processManager == nil {
		return nil
	}
	processManager.mu.Lock()
	defer processManager.mu.Unlock()
	return processManager.stopCurrentAppLocked(
		processManager.deriveGracefulStopTimeout(),
	)
}

// CurrentCommand returns the current tracked command pointer.
func (processManager *AppProcessManager) CurrentCommand() *exec.Cmd {
	if processManager == nil {
		return nil
	}
	processManager.mu.Lock()
	defer processManager.mu.Unlock()
	return processManager.appCommand
}

// stopCurrentAppLocked stops app process using graceful then hard stop strategy.
func (processManager *AppProcessManager) stopCurrentAppLocked(
	gracefulStopTimeout time.Duration,
) error {
	if processManager.appCommand == nil ||
		processManager.appCommand.Process == nil {
		processManager.appCommand = nil
		processManager.waitDone = nil
		processManager.waitRunning = false
		return nil
	}

	stopError := stopCommandWithTimeout(
		processManager.appCommand,
		processManager.waitDone,
		gracefulStopTimeout,
	)
	waitDone := processManager.waitDone

	if waitDone != nil {
		select {
		case <-waitDone:
		case <-time.After(gracefulStopTimeout):
		}
	}

	processManager.appCommand = nil
	processManager.waitDone = nil
	processManager.waitRunning = false

	if stopError != nil && !ShouldIgnoreProcessTerminationError(stopError) {
		return stopError
	}
	return nil
}

func (processManager *AppProcessManager) deriveGracefulStopTimeout() time.Duration {
	if processManager == nil || processManager.GracefulStopTimeout <= 0 {
		return DefaultAppProcessGracefulStopTimeout
	}
	return processManager.GracefulStopTimeout
}

// waitForCommand waits for command completion and marks wait state closed.
func (processManager *AppProcessManager) waitForCommand(
	command *exec.Cmd,
	done chan struct{},
) {
	defer close(done)
	if command == nil {
		return
	}
	if waitError := command.Wait(); waitError != nil &&
		!ShouldIgnoreProcessWaitError(waitError) {
		// Wait errors are intentionally ignored by caller; this background loop is
		// only responsible for signaling completion.
	}
}

// stopCommandWithTimeout sends SIGTERM then SIGKILL on timeout.
func stopCommandWithTimeout(
	command *exec.Cmd,
	waitDone chan struct{},
	gracefulStopTimeout time.Duration,
) error {
	if command == nil || command.Process == nil {
		return nil
	}
	if gracefulStopTimeout <= 0 {
		gracefulStopTimeout = DefaultAppProcessGracefulStopTimeout
	}

	if signalError := command.Process.Signal(syscall.SIGTERM); signalError != nil {
		if !ShouldIgnoreProcessTerminationError(signalError) {
			return fmt.Errorf("send SIGTERM: %w", signalError)
		}
	}

	waitForExitWithTimeout := createWaitForExitWithTimeout(command, waitDone)

	waitError, exited := waitForExitWithTimeout(gracefulStopTimeout)
	if exited {
		if waitError != nil &&
			!shouldIgnoreProcessWaitErrorForCommand(waitError, command) {
			return waitError
		}
		return nil
	}

	if killError := command.Process.Kill(); killError != nil &&
		!ShouldIgnoreProcessTerminationError(killError) {
		return fmt.Errorf(
			"kill process after graceful timeout: %w",
			killError,
		)
	}

	waitError, exited = waitForExitWithTimeout(gracefulStopTimeout)
	if exited {
		if waitError != nil &&
			!shouldIgnoreProcessWaitErrorForCommand(waitError, command) {
			return waitError
		}
		return nil
	}

	return errors.New(
		"timed out waiting for process exit after SIGKILL",
	)
}

func createWaitForExitWithTimeout(
	command *exec.Cmd,
	waitDone chan struct{},
) func(time.Duration) (error, bool) {
	if waitDone == nil {
		waitResult := make(chan error, 1)
		go func() {
			waitResult <- command.Wait()
		}()
		return func(timeout time.Duration) (error, bool) {
			select {
			case waitError := <-waitResult:
				return waitError, true
			case <-time.After(timeout):
				return nil, false
			}
		}
	}

	return func(timeout time.Duration) (error, bool) {
		select {
		case <-waitDone:
			return nil, true
		case <-time.After(timeout):
			return nil, false
		}
	}
}

func shouldIgnoreProcessWaitErrorForCommand(
	processWaitError error,
	command *exec.Cmd,
) bool {
	if ShouldIgnoreProcessWaitError(processWaitError) {
		return true
	}
	if processWaitError == nil {
		return false
	}
	errorString := strings.ToLower(processWaitError.Error())
	if strings.Contains(errorString, "wait") &&
		strings.Contains(errorString, "no child processes") &&
		command != nil &&
		strings.TrimSpace(command.Path) != "" {
		return true
	}
	return false
}

// ShouldIgnoreProcessTerminationError reports expected stop outcomes.
func ShouldIgnoreProcessTerminationError(processTerminationError error) bool {
	if processTerminationError == nil {
		return false
	}
	if errors.Is(processTerminationError, os.ErrProcessDone) {
		return true
	}
	errorString := strings.ToLower(processTerminationError.Error())
	return strings.Contains(errorString, "process already finished")
}

// ShouldIgnoreProcessWaitError reports expected wait outcomes from termination.
func ShouldIgnoreProcessWaitError(processWaitError error) bool {
	if processWaitError == nil {
		return false
	}
	errorString := strings.ToLower(processWaitError.Error())
	if strings.Contains(errorString, "signal: terminated") {
		return true
	}
	if strings.Contains(errorString, "signal: killed") {
		return true
	}
	return false
}

// ResolveReadinessProbeURL builds a readiness URL from host/port/endpoint.
func ResolveReadinessProbeURL(host string, port int, endpoint string) string {
	resolvedHost := strings.TrimSpace(host)
	if resolvedHost == "" {
		resolvedHost = LocalReadinessProbeHostIPv4
	}
	resolvedEndpoint := strings.TrimSpace(endpoint)
	if resolvedEndpoint == "" {
		resolvedEndpoint = "/"
	}
	if !strings.HasPrefix(resolvedEndpoint, "/") {
		resolvedEndpoint = "/" + resolvedEndpoint
	}
	return fmt.Sprintf("http://%s:%d%s", resolvedHost, port, resolvedEndpoint)
}

// ResolveAppReadyURL resolves app healthcheck URL for given runtime port.
func ResolveAppReadyURL(appPort int, healthcheckEndpoint string) string {
	return ResolveReadinessProbeURL(
		LocalReadinessProbeHostIPv4,
		appPort,
		healthcheckEndpoint,
	)
}

// DeriveReadinessWaitDelay computes delay using capped doubling backoff.
func DeriveReadinessWaitDelay(
	attemptIndex int,
	baseDelay time.Duration,
	maximumDelay time.Duration,
) time.Duration {
	if baseDelay <= 0 {
		baseDelay = 50 * time.Millisecond
	}
	if maximumDelay <= 0 {
		maximumDelay = 500 * time.Millisecond
	}
	if attemptIndex <= 0 {
		return baseDelay
	}

	computedDelay := baseDelay
	for index := 0; index < attemptIndex; index++ {
		computedDelay *= 2
		if computedDelay >= maximumDelay {
			return maximumDelay
		}
	}
	return computedDelay
}

// ShouldContinueReadinessWait reports whether elapsed time is still within budget.
func ShouldContinueReadinessWait(
	totalElapsed time.Duration,
	maximumTotal time.Duration,
) bool {
	if maximumTotal <= 0 {
		maximumTotal = 25 * time.Second
	}
	return totalElapsed <= maximumTotal
}

// WaitForAnyReady polls URLs until one is healthy or timeout budget expires.
func WaitForAnyReady(urls []string, policy ReadinessWaitPolicy) bool {
	return WaitForAnyReadyWithContext(
		context.Background(),
		urls,
		policy,
	)
}

// WaitForAnyReadyWithContext polls URLs until one is healthy, timeout budget
// expires, or context cancellation is observed.
func WaitForAnyReadyWithContext(
	readinessContext context.Context,
	urls []string,
	policy ReadinessWaitPolicy,
) bool {
	if readinessContext == nil {
		readinessContext = context.Background()
	}
	if len(urls) == 0 {
		return false
	}

	resolvedPolicy := normalizeReadinessWaitPolicy(policy)
	readinessBudgetContext, cancelReadinessBudgetContext := context.WithTimeout(
		readinessContext,
		resolvedPolicy.MaximumTotalWait,
	)
	defer cancelReadinessBudgetContext()

	httpClient := &http.Client{Timeout: resolvedPolicy.HTTPClientTimeout}
	startTime := time.Now()

	attemptIndex := 0
	for {
		select {
		case <-readinessBudgetContext.Done():
			return false
		default:
		}

		for _, targetURL := range urls {
			if probeURLWithClient(
				readinessBudgetContext,
				httpClient,
				targetURL,
				resolvedPolicy.TreatHTTPStatusCodeAsReady,
			) {
				return true
			}
		}

		elapsed := time.Since(startTime)
		if !ShouldContinueReadinessWait(
			elapsed,
			resolvedPolicy.MaximumTotalWait,
		) {
			return false
		}

		delay := DeriveReadinessWaitDelay(
			attemptIndex,
			resolvedPolicy.InitialDelay,
			resolvedPolicy.MaximumDelay,
		)
		attemptIndex++

		delayTimer := time.NewTimer(delay)
		select {
		case <-readinessBudgetContext.Done():
			if !delayTimer.Stop() {
				<-delayTimer.C
			}
			return false
		case <-delayTimer.C:
		}
	}
}

func normalizeReadinessWaitPolicy(
	policy ReadinessWaitPolicy,
) ReadinessWaitPolicy {
	defaultPolicy := DefaultReadinessWaitPolicy()
	if policy.HTTPClientTimeout <= 0 {
		policy.HTTPClientTimeout = defaultPolicy.HTTPClientTimeout
	}
	if policy.InitialDelay <= 0 {
		policy.InitialDelay = defaultPolicy.InitialDelay
	}
	if policy.MaximumDelay <= 0 {
		policy.MaximumDelay = defaultPolicy.MaximumDelay
	}
	if policy.MaximumTotalWait <= 0 {
		policy.MaximumTotalWait = defaultPolicy.MaximumTotalWait
	}
	if policy.TreatHTTPStatusCodeAsReady == nil {
		policy.TreatHTTPStatusCodeAsReady = defaultPolicy.TreatHTTPStatusCodeAsReady
	}
	return policy
}

// probeURLWithClient performs one readiness probe request.
func probeURLWithClient(
	readinessContext context.Context,
	httpClient *http.Client,
	targetURL string,
	statusCodePredicate func(int) bool,
) bool {
	request, requestCreateError := http.NewRequestWithContext(
		readinessContext,
		http.MethodGet,
		targetURL,
		nil,
	)
	if requestCreateError != nil {
		return false
	}

	response, requestError := httpClient.Do(request)
	if requestError != nil {
		return false
	}
	defer response.Body.Close()
	return statusCodePredicate(response.StatusCode)
}

package tooling

import (
	"fmt"
	"net/http"
	"time"

	"github.com/vormadev/vorma/wave"
)

type readinessWaitPolicy struct {
	maxAttempts    int
	baseDelay      time.Duration
	maxTotal       time.Duration
	requestTimeout time.Duration
}

const localReadinessProbeHostIPv4 = "127.0.0.1"

func defaultReadinessWaitPolicy() readinessWaitPolicy {
	return readinessWaitPolicy{
		maxAttempts:    100,
		baseDelay:      20 * time.Millisecond,
		maxTotal:       10 * time.Second,
		requestTimeout: 500 * time.Millisecond,
	}
}

func (s *server) waitForApp() bool {
	url := resolveAppReadyURL(s.mustGetPort(), s.cfg.HealthcheckEndpoint())
	ok := s.waitForReady(url)
	if !ok {
		s.log.Warn("App did not become ready in time", "url", url)
	}
	return ok
}

func resolveAppReadyURL(appPort int, healthcheckEndpoint string) string {
	return fmt.Sprintf(
		"http://%s:%d%s",
		localReadinessProbeHostIPv4,
		appPort,
		healthcheckEndpoint,
	)
}

func (s *server) waitForReady(url string) bool {
	policy := defaultReadinessWaitPolicy()
	client := &http.Client{Timeout: policy.requestTimeout}
	var total time.Duration

	for attemptIndex := range policy.maxAttempts {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return true
		}
		if resp != nil {
			resp.Body.Close()
		}

		delay := deriveReadinessWaitDelay(attemptIndex, policy.baseDelay)
		total += delay
		if !shouldContinueReadinessWait(total, policy.maxTotal) {
			return false
		}

		time.Sleep(delay)
	}

	return false
}

func deriveReadinessWaitDelay(
	attemptIndex int,
	baseDelay time.Duration,
) time.Duration {
	return baseDelay + time.Duration(attemptIndex)*baseDelay
}

func shouldContinueReadinessWait(
	total time.Duration,
	maxTotal time.Duration,
) bool {
	return total <= maxTotal
}

func (s *server) mustGetPort() int {
	if s == nil || s.portResolver == nil {
		return wave.MustGetPort()
	}
	return s.portResolver.MustGetPort()
}

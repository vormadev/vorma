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
const localReadinessProbeHostLocalhost = "localhost"

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
	return resolveReadinessProbeURL(
		localReadinessProbeHostIPv4,
		appPort,
		healthcheckEndpoint,
	)
}

func (s *server) waitForReady(url string) bool {
	return s.waitForAnyReady([]string{url})
}

func (s *server) waitForAnyReady(urls []string) bool {
	policy := defaultReadinessWaitPolicy()
	client := &http.Client{Timeout: policy.requestTimeout}
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

	for attemptIndex := range policy.maxAttempts {
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

		delay := deriveReadinessWaitDelay(attemptIndex, policy.baseDelay)
		total += delay
		if !shouldContinueReadinessWait(total, policy.maxTotal) {
			return false
		}

		time.Sleep(delay)
	}

	return false
}

func resolveReadinessProbeURL(
	host string,
	port int,
	endpoint string,
) string {
	return fmt.Sprintf("http://%s:%d%s", host, port, endpoint)
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

// mustGetPort returns the app runtime port for devserver orchestration.
// It panics in dev mode if a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func (s *server) mustGetPort() int {
	if s == nil || s.portResolver == nil {
		return wave.MustGetPort()
	}
	return s.portResolver.MustGetPort()
}

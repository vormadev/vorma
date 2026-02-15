package viteutil

import (
	"fmt"
	"testing"

	"github.com/vormadev/vorma/kit/netutil"
)

func TestInitPort_UsesCallerDefaultPortWhenAvailable(
	t *testing.T,
) {
	t.Setenv(PortEnvName, "")

	candidatePort := findAvailablePortInRangeForInitPortTest(
		t,
		20000,
		24000,
	)

	vitePort, err := InitPort(candidatePort)
	if err != nil {
		t.Fatalf("InitPort() returned error: %v", err)
	}
	if vitePort != candidatePort {
		t.Fatalf("expected InitPort() to use caller default port %d, got %d", candidatePort, vitePort)
	}

	if got := GetVitePortStr(); got != fmt.Sprintf("%d", candidatePort) {
		t.Fatalf("GetVitePortStr() = %q, want %d", got, candidatePort)
	}
}

func findAvailablePortInRangeForInitPortTest(
	t *testing.T,
	rangeStart int,
	rangeEnd int,
) int {
	t.Helper()

	for candidatePort := rangeStart; candidatePort <= rangeEnd; candidatePort++ {
		if netutil.CheckAvailability(candidatePort) {
			return candidatePort
		}
	}

	t.Fatalf(
		"could not find an available port in range [%d, %d]",
		rangeStart,
		rangeEnd,
	)
	return 0
}
